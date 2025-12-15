// Package provider defines interfaces and implementations for fetching secrets
// from external secret management systems like AWS Secrets Manager, HashiCorp Vault,
// Azure Key Vault, and 1Password.
package provider

import (
	"context"
	"fmt"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// SecretProvider is the interface for external secret sources.
// Implementations include AWS Secrets Manager, HashiCorp Vault, Azure Key Vault.
type SecretProvider interface {
	// FetchSecret retrieves a secret from the provider at the given path.
	// Returns a map of key-value pairs representing the secret data.
	// Returns an error if the secret cannot be fetched.
	FetchSecret(ctx context.Context, path string) (map[string]string, error)

	// Name returns the provider identifier (e.g., "aws-secretsmanager").
	Name() string
}

// ProviderRegistry manages available secret providers.
type ProviderRegistry struct {
	providers map[string]SecretProvider
}

// NewProviderRegistry creates a new provider registry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]SecretProvider),
	}
}

// Register adds a provider to the registry.
func (r *ProviderRegistry) Register(provider SecretProvider) {
	r.providers[provider.Name()] = provider
}

// Get retrieves a provider by name.
// Returns nil if the provider is not found.
func (r *ProviderRegistry) Get(name string) SecretProvider {
	return r.providers[name]
}

// List returns all registered provider names.
func (r *ProviderRegistry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// DefaultProviderRegistry creates a registry with all available providers.
// This is the main entry point for initializing providers in the controller.
func DefaultProviderRegistry(ctx context.Context) (*ProviderRegistry, error) {
	logger := log.FromContext(ctx)
	registry := NewProviderRegistry()

	// Register AWS Secrets Manager provider
	awsProvider, err := NewAWSSecretsManagerProvider(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS provider: %w", err)
	}
	registry.Register(awsProvider)
	logger.Info("AWS Secrets Manager provider registered")

	// Register 1Password provider (only if token is configured)
	if os.Getenv("OP_SERVICE_ACCOUNT_TOKEN") != "" {
		opProvider, err := NewOnePasswordProvider(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create 1Password provider: %w", err)
		}
		registry.Register(opProvider)
		opProvider.Start(ctx)
		logger.Info("1Password provider registered")
	} else {
		logger.Info("1Password provider not configured (OP_SERVICE_ACCOUNT_TOKEN not set)")
	}

	// Register Bitwarden provider (only if enabled)
	if os.Getenv(enableBitwardenEnv) == "true" {
		bwProvider, err := NewBitwardenProvider(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create Bitwarden provider: %w", err)
		}
		registry.Register(bwProvider)
		bwProvider.Start(ctx)
		logger.Info("Bitwarden provider registered", "serverURL", os.Getenv(bwServerURLEnv))
	} else {
		logger.Info("Bitwarden provider not enabled (set ENABLE_BITWARDEN=true)")
	}

	return registry, nil
}
