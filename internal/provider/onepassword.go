package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	onepassword "github.com/1password/onepassword-sdk-go"

	"github.com/tiagocborg/jasm/internal/metrics"
)

const (
	onePasswordProviderName = "1password"
	opServiceAccountEnv     = "OP_SERVICE_ACCOUNT_TOKEN"
	integrationName         = "JASM"
	integrationVersion      = "1.0.0"
)

// OnePasswordProvider implements SecretProvider for 1Password.
type OnePasswordProvider struct {
	client     *onepassword.Client
	retryQueue *RetryQueue
	metrics    *metrics.ProviderMetrics
}

// NewOnePasswordProvider creates a new 1Password provider.
func NewOnePasswordProvider(ctx context.Context) (*OnePasswordProvider, error) {
	token := os.Getenv(opServiceAccountEnv)
	if token == "" {
		return nil, fmt.Errorf("1Password service account token not configured (set %s)", opServiceAccountEnv)
	}

	client, err := onepassword.NewClient(ctx,
		onepassword.WithServiceAccountToken(token),
		onepassword.WithIntegrationInfo(integrationName, integrationVersion),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create 1Password client: %w", err)
	}

	providerMetrics, err := metrics.NewProviderMetrics(onePasswordProviderName)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics: %w", err)
	}

	retryQueue := NewRetryQueue(5*time.Minute, func(delta int64) {
		providerMetrics.UpdateQueueDepth(context.Background(), delta)
	})

	return &OnePasswordProvider{
		client:     client,
		retryQueue: retryQueue,
		metrics:    providerMetrics,
	}, nil
}

// Name returns the provider identifier.
func (p *OnePasswordProvider) Name() string {
	return onePasswordProviderName
}

// FetchSecret retrieves a secret from 1Password.
// Supported path formats:
//   - op://vault/item (fetches all fields)
//   - op://vault/item/field
//   - op://vault/item/section/field
func (p *OnePasswordProvider) FetchSecret(ctx context.Context, path string) (map[string]string, error) {
	// Parse the secret reference
	ref, err := ParseSecretReference(path)
	if err != nil {
		return nil, fmt.Errorf("invalid 1Password secret reference: %w", err)
	}

	if err := ref.Validate(); err != nil {
		return nil, fmt.Errorf("invalid 1Password secret reference: %w", err)
	}

	start := time.Now()

	// If no field is specified, fetch all fields from the item
	if !ref.HasField() {
		secrets, err := p.fetchAllFields(ctx, ref)
		duration := time.Since(start)

		if err != nil {
			p.metrics.RecordRequest(ctx, "error", ref.Vault)
			p.metrics.RecordDuration(ctx, duration, "error")

			if isRateLimited(err) {
				p.metrics.RecordRateLimit(ctx)
				return nil, fmt.Errorf("rate limited by 1Password API")
			}

			return nil, p.wrapError(err, ref)
		}

		p.metrics.RecordRequest(ctx, "success", ref.Vault)
		p.metrics.RecordDuration(ctx, duration, "success")
		return secrets, nil
	}

	// Fetch a single field
	secret, err := p.resolveSecret(ctx, ref.String())
	duration := time.Since(start)

	if err != nil {
		p.metrics.RecordRequest(ctx, "error", ref.Vault)
		p.metrics.RecordDuration(ctx, duration, "error")

		if isRateLimited(err) {
			p.metrics.RecordRateLimit(ctx)
			return nil, fmt.Errorf("rate limited by 1Password API")
		}

		return nil, p.wrapError(err, ref)
	}

	p.metrics.RecordRequest(ctx, "success", ref.Vault)
	p.metrics.RecordDuration(ctx, duration, "success")

	// Return the secret as a single key-value pair using the field name as key
	return map[string]string{
		ref.Field: secret,
	}, nil
}

// fetchAllFields retrieves all fields from a 1Password item.
func (p *OnePasswordProvider) fetchAllFields(ctx context.Context, ref *SecretReference) (map[string]string, error) {
	// Look up vault ID by name
	vaultID, err := p.lookupVaultID(ctx, ref.Vault)
	if err != nil {
		return nil, err
	}

	// Look up item ID by title
	itemID, err := p.lookupItemID(ctx, vaultID, ref.Item)
	if err != nil {
		return nil, err
	}

	// Get the full item with all fields
	item, err := p.client.Items().Get(ctx, vaultID, itemID)
	if err != nil {
		return nil, err
	}

	secrets := make(map[string]string)
	for _, field := range item.Fields {
		// Skip fields without a title or value
		if field.Title == "" || field.Value == "" {
			continue
		}
		secrets[field.Title] = field.Value
	}

	if len(secrets) == 0 {
		return nil, fmt.Errorf("no fields found in item %q", ref.Item)
	}

	return secrets, nil
}

// lookupVaultID finds a vault's UUID by its name.
func (p *OnePasswordProvider) lookupVaultID(ctx context.Context, vaultName string) (string, error) {
	vaults, err := p.client.Vaults().List(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list vaults: %w", err)
	}

	for _, vault := range vaults {
		if vault.Title == vaultName {
			return vault.ID, nil
		}
	}

	return "", fmt.Errorf("vault %q not found", vaultName)
}

// lookupItemID finds an item's UUID by its title within a vault.
func (p *OnePasswordProvider) lookupItemID(ctx context.Context, vaultID, itemTitle string) (string, error) {
	items, err := p.client.Items().List(ctx, vaultID)
	if err != nil {
		return "", fmt.Errorf("failed to list items: %w", err)
	}

	for _, item := range items {
		if item.Title == itemTitle {
			return item.ID, nil
		}
	}

	return "", fmt.Errorf("item %q not found in vault", itemTitle)
}

// Start begins the retry queue processor.
func (p *OnePasswordProvider) Start(ctx context.Context) {
	p.retryQueue.Start(ctx)
}

// Stop stops the retry queue processor.
func (p *OnePasswordProvider) Stop() {
	p.retryQueue.Stop()
}

func (p *OnePasswordProvider) resolveSecret(ctx context.Context, reference string) (string, error) {
	return p.client.Secrets().Resolve(ctx, reference)
}

func (p *OnePasswordProvider) wrapError(err error, ref *SecretReference) error {
	errStr := err.Error()

	// Check for common error patterns and provide actionable messages
	switch {
	case strings.Contains(errStr, "vault") && strings.Contains(errStr, "not found"):
		return fmt.Errorf("vault %q not found or access denied", ref.Vault)
	case strings.Contains(errStr, "item") && strings.Contains(errStr, "not found"):
		return fmt.Errorf("item %q not found in vault %q", ref.Item, ref.Vault)
	case strings.Contains(errStr, "field") && strings.Contains(errStr, "not found"):
		return fmt.Errorf("field %q not found in item %q", ref.Field, ref.Item)
	case strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "authentication"):
		return fmt.Errorf("authentication failed: %w", err)
	case strings.Contains(errStr, "network") || strings.Contains(errStr, "connection"):
		return fmt.Errorf("failed to connect to 1Password: %w", err)
	default:
		return fmt.Errorf("failed to resolve secret from 1Password: %w", err)
	}
}

func isRateLimited(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "too many requests")
}

// ErrRateLimited is returned when 1Password API rate limits requests.
var ErrRateLimited = errors.New("rate limited by 1Password API")
