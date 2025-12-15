package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tiagocborg/jasm/internal/metrics"
)

const (
	bitwardenProviderName = "bitwarden"
	enableBitwardenEnv    = "ENABLE_BITWARDEN"
	bwServerURLEnv        = "BW_SERVER_URL"
	bwEmailEnv            = "BW_EMAIL"
	bwPasswordEnv         = "BW_PASSWORD"
)

// BitwardenProvider implements SecretProvider for Bitwarden/Vaultwarden.
// It uses direct API calls with client-side decryption.
type BitwardenProvider struct {
	client     *BitwardenClient
	password   string
	retryQueue *RetryQueue
	metrics    *metrics.ProviderMetrics
	mu         sync.Mutex
}

// NewBitwardenProvider creates a new Bitwarden provider.
// Required environment variables:
//   - ENABLE_BITWARDEN: Set to "true" to enable the provider
//   - BW_EMAIL: Bitwarden account email
//   - BW_PASSWORD: Master password
//   - BW_SERVER_URL: (Optional) Vaultwarden server URL, defaults to https://vault.bitwarden.com
func NewBitwardenProvider(ctx context.Context) (*BitwardenProvider, error) {
	enabled := os.Getenv(enableBitwardenEnv)
	if enabled != "true" {
		return nil, fmt.Errorf("Bitwarden provider not enabled (set %s=true)", enableBitwardenEnv)
	}

	email := os.Getenv(bwEmailEnv)
	if email == "" {
		return nil, fmt.Errorf("%s is required", bwEmailEnv)
	}

	password := os.Getenv(bwPasswordEnv)
	if password == "" {
		return nil, fmt.Errorf("%s is required", bwPasswordEnv)
	}

	// Server URL is optional, defaults to Bitwarden cloud
	serverURL := os.Getenv(bwServerURLEnv)

	providerMetrics, err := metrics.NewProviderMetrics(bitwardenProviderName)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics: %w", err)
	}

	retryQueue := NewRetryQueue(5*time.Minute, func(delta int64) {
		providerMetrics.UpdateQueueDepth(context.Background(), delta)
	})

	client := NewBitwardenClient(serverURL, email)

	// Authenticate with Bitwarden
	if err := client.Authenticate(ctx, password); err != nil {
		return nil, fmt.Errorf("failed to authenticate with Bitwarden: %w", err)
	}

	return &BitwardenProvider{
		client:     client,
		password:   password,
		retryQueue: retryQueue,
		metrics:    providerMetrics,
	}, nil
}

// Name returns the provider identifier.
func (p *BitwardenProvider) Name() string {
	return bitwardenProviderName
}

// Start begins the retry queue processor.
func (p *BitwardenProvider) Start(ctx context.Context) {
	p.retryQueue.Start(ctx)
}

// Stop stops the retry queue processor.
func (p *BitwardenProvider) Stop() {
	p.retryQueue.Stop()
}

// FetchSecret retrieves a secret from Bitwarden/Vaultwarden.
// Supported path format: bw://<folder>/<item>[/<field>]
func (p *BitwardenProvider) FetchSecret(ctx context.Context, path string) (map[string]string, error) {
	// Parse the secret reference
	ref, err := ParseBitwardenReference(path)
	if err != nil {
		return nil, fmt.Errorf("invalid Bitwarden secret reference: %w", err)
	}

	if err := ref.Validate(); err != nil {
		return nil, fmt.Errorf("invalid Bitwarden secret reference: %w", err)
	}

	start := time.Now()

	// Ensure we have a valid cache
	p.mu.Lock()
	if !p.client.IsCacheValid() || !p.client.IsAuthenticated() {
		if err := p.client.Authenticate(ctx, p.password); err != nil {
			p.mu.Unlock()
			p.metrics.RecordRequest(ctx, "error", ref.Folder)
			p.metrics.RecordDuration(ctx, time.Since(start), "error")
			return nil, fmt.Errorf("failed to re-authenticate with Bitwarden: %w", err)
		}
	}
	p.mu.Unlock()

	// Find the item
	item, err := p.client.FindItem(ref.Folder, ref.ItemName)
	duration := time.Since(start)

	if err != nil {
		p.metrics.RecordRequest(ctx, "error", ref.Folder)
		p.metrics.RecordDuration(ctx, duration, "error")

		if isBWRateLimited(err) {
			p.metrics.RecordRateLimit(ctx)
			return nil, fmt.Errorf("rate limited by Bitwarden API")
		}

		return nil, p.wrapError(err, ref)
	}

	// Extract fields from the item
	secrets := item.ToSecretMap()
	if len(secrets) == 0 {
		p.metrics.RecordRequest(ctx, "error", ref.Folder)
		p.metrics.RecordDuration(ctx, duration, "error")
		return nil, fmt.Errorf("no fields found in item %q", ref.ItemName)
	}

	// If a specific field is requested, filter to just that field
	if ref.HasField() {
		value, ok := secrets[ref.Field]
		if !ok {
			p.metrics.RecordRequest(ctx, "error", ref.Folder)
			p.metrics.RecordDuration(ctx, duration, "error")
			return nil, fmt.Errorf("field %q not found in item %q", ref.Field, ref.ItemName)
		}
		secrets = map[string]string{ref.Field: value}
	}

	p.metrics.RecordRequest(ctx, "success", ref.Folder)
	p.metrics.RecordDuration(ctx, duration, "success")

	return secrets, nil
}

// wrapError provides actionable error messages.
func (p *BitwardenProvider) wrapError(err error, ref *BitwardenSecretReference) error {
	errStr := err.Error()

	switch {
	case strings.Contains(errStr, "folder") && strings.Contains(errStr, "not found"):
		return fmt.Errorf("folder %q not found", ref.Folder)
	case strings.Contains(errStr, "item") && strings.Contains(errStr, "not found"):
		return fmt.Errorf("item %q not found in folder %q", ref.ItemName, ref.Folder)
	case strings.Contains(errStr, "field") && strings.Contains(errStr, "not found"):
		return fmt.Errorf("field %q not found in item %q", ref.Field, ref.ItemName)
	case strings.Contains(errStr, "authentication") || strings.Contains(errStr, "login failed"):
		return fmt.Errorf("authentication failed: invalid email or password")
	case strings.Contains(errStr, "network") || strings.Contains(errStr, "connection"):
		return fmt.Errorf("failed to connect to Bitwarden server: %w", err)
	default:
		return fmt.Errorf("failed to fetch secret from Bitwarden: %w", err)
	}
}

// isBWRateLimited checks if the error indicates rate limiting.
func isBWRateLimited(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrBWRateLimited) {
		return true
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "too many requests")
}

// ErrBWRateLimited is returned when Bitwarden/Vaultwarden rate limits requests.
var ErrBWRateLimited = errors.New("rate limited by Bitwarden API")
