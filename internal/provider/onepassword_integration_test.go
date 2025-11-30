package provider

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestOnePasswordProvider_Integration(t *testing.T) {
	token := os.Getenv("OP_SERVICE_ACCOUNT_TOKEN")
	if token == "" {
		t.Skip("skipping integration test: OP_SERVICE_ACCOUNT_TOKEN not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	p, err := NewOnePasswordProvider(ctx)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer p.Stop()

	t.Run("provider name", func(t *testing.T) {
		if p.Name() != "1password" {
			t.Errorf("Name() = %q, want %q", p.Name(), "1password")
		}
	})

	t.Run("fetch secret with invalid reference", func(t *testing.T) {
		_, err := p.FetchSecret(ctx, "invalid-reference")
		if err == nil {
			t.Error("expected error for invalid reference")
		}
	})

	t.Run("fetch secret with missing vault", func(t *testing.T) {
		_, err := p.FetchSecret(ctx, "op://NonExistentVault12345/item/field")
		if err == nil {
			t.Error("expected error for non-existent vault")
		}
	})
}

func TestOnePasswordProvider_RateLimitDetection(t *testing.T) {
	tests := []struct {
		name    string
		errMsg  string
		isLimit bool
	}{
		{"rate limit message", "rate limit exceeded", true},
		{"429 status", "HTTP 429", true},
		{"too many requests", "too many requests", true},
		{"normal error", "vault not found", false},
		{"network error", "connection refused", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRateLimited(&testError{msg: tt.errMsg})
			if got != tt.isLimit {
				t.Errorf("isRateLimited(%q) = %v, want %v", tt.errMsg, got, tt.isLimit)
			}
		})
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestOnePasswordProvider_RetryQueueIntegration(t *testing.T) {
	token := os.Getenv("OP_SERVICE_ACCOUNT_TOKEN")
	if token == "" {
		t.Skip("skipping integration test: OP_SERVICE_ACCOUNT_TOKEN not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	p, err := NewOnePasswordProvider(ctx)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	// Verify retry queue is initialized
	if p.retryQueue == nil {
		t.Error("retry queue not initialized")
	}

	// Start the provider's retry queue
	p.Start(ctx)
	defer p.Stop()

	// Queue depth should be 0 initially
	if depth := p.retryQueue.Depth(); depth != 0 {
		t.Errorf("initial queue depth = %d, want 0", depth)
	}
}

func TestOnePasswordProvider_MetricsIntegration(t *testing.T) {
	token := os.Getenv("OP_SERVICE_ACCOUNT_TOKEN")
	if token == "" {
		t.Skip("skipping integration test: OP_SERVICE_ACCOUNT_TOKEN not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	p, err := NewOnePasswordProvider(ctx)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	// Verify metrics are initialized
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
