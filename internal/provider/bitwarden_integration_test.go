package provider

import (
	"context"
	"os"
	"testing"
	"time"
)

func skipIfBitwardenNotConfigured(t *testing.T) {
	t.Helper()
	if os.Getenv("ENABLE_BITWARDEN") != "true" {
		t.Skip("skipping integration test: ENABLE_BITWARDEN not set to true")
	}
}

func TestBitwardenProvider_Integration(t *testing.T) {
	skipIfBitwardenNotConfigured(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	p, err := NewBitwardenProvider(ctx)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer p.Stop()

	t.Run("provider name", func(t *testing.T) {
		if p.Name() != "bitwarden" {
			t.Errorf("Name() = %q, want %q", p.Name(), "bitwarden")
		}
	})

	t.Run("fetch secret with invalid reference", func(t *testing.T) {
		_, err := p.FetchSecret(ctx, "invalid-reference")
		if err == nil {
			t.Error("expected error for invalid reference")
		}
	})

	t.Run("fetch secret with missing folder", func(t *testing.T) {
		_, err := p.FetchSecret(ctx, "bw://folder/NonExistentFolder12345/item")
		if err == nil {
			t.Error("expected error for non-existent folder")
		}
	})
}

func TestBitwardenProvider_RetryQueueIntegration(t *testing.T) {
	skipIfBitwardenNotConfigured(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	p, err := NewBitwardenProvider(ctx)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	if p.retryQueue == nil {
		t.Error("retry queue not initialized")
	}

	p.Start(ctx)
	defer p.Stop()

	if depth := p.retryQueue.Depth(); depth != 0 {
		t.Errorf("initial queue depth = %d, want 0", depth)
	}
}

func TestBitwardenProvider_MetricsIntegration(t *testing.T) {
	skipIfBitwardenNotConfigured(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	p, err := NewBitwardenProvider(ctx)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	if p.metrics == nil {
		t.Error("metrics not initialized")
	}
	if p.metrics.RequestsTotal == nil {
		t.Error("RequestsTotal metric not initialized")
	}
	if p.metrics.RateLimitsTotal == nil {
		t.Error("RateLimitsTotal metric not initialized")
	}
	if p.metrics.RequestDuration == nil {
		t.Error("RequestDuration metric not initialized")
	}
	if p.metrics.RetryQueueDepth == nil {
		t.Error("RetryQueueDepth metric not initialized")
	}
}

func TestBitwardenProvider_FetchSecretIntegration(t *testing.T) {
	skipIfBitwardenNotConfigured(t)

	testPath := os.Getenv("BW_TEST_SECRET_PATH")
	if testPath == "" {
		t.Skip("skipping fetch test: BW_TEST_SECRET_PATH not set (e.g., bw://folder/codnod-k8s/jasm-test)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	p, err := NewBitwardenProvider(ctx)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer p.Stop()

	secrets, err := p.FetchSecret(ctx, testPath)
	if err != nil {
		t.Fatalf("FetchSecret() error = %v", err)
	}

	if len(secrets) == 0 {
		t.Error("expected at least one secret field")
	}

	t.Logf("fetched %d fields from %s", len(secrets), testPath)
}
