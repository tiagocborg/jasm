package metrics

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestConfigFromEnv(t *testing.T) {
	tests := []struct {
		name         string
		envEndpoint  string
		envInsecure  string
		wantEndpoint string
		wantInsecure bool
	}{
		{
			name:         "default values",
			wantEndpoint: "localhost:4317",
			wantInsecure: false,
		},
		{
			name:         "custom endpoint",
			envEndpoint:  "otel-collector:4317",
			wantEndpoint: "otel-collector:4317",
			wantInsecure: false,
		},
		{
			name:         "insecure mode",
			envInsecure:  "true",
			wantEndpoint: "localhost:4317",
			wantInsecure: true,
		},
		{
			name:         "insecure mode false",
			envInsecure:  "false",
			wantEndpoint: "localhost:4317",
			wantInsecure: false,
		},
		{
			name:         "custom endpoint with insecure",
			envEndpoint:  "otel:4317",
			envInsecure:  "true",
			wantEndpoint: "otel:4317",
			wantInsecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear env vars
			os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
			os.Unsetenv("OTEL_EXPORTER_OTLP_INSECURE")

			if tt.envEndpoint != "" {
				os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", tt.envEndpoint)
			}
			if tt.envInsecure != "" {
				os.Setenv("OTEL_EXPORTER_OTLP_INSECURE", tt.envInsecure)
			}

			cfg := ConfigFromEnv()

			if cfg.Endpoint != tt.wantEndpoint {
				t.Errorf("Endpoint = %q, want %q", cfg.Endpoint, tt.wantEndpoint)
			}
			if cfg.Insecure != tt.wantInsecure {
				t.Errorf("Insecure = %v, want %v", cfg.Insecure, tt.wantInsecure)
			}
		})
	}

	// Cleanup
	os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	os.Unsetenv("OTEL_EXPORTER_OTLP_INSECURE")
}

func TestNewProviderMetrics(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		wantErr  bool
	}{
		{
			name:     "1password provider",
			provider: "1password",
			wantErr:  false,
		},
		{
			name:     "aws provider",
			provider: "aws",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics, err := NewProviderMetrics(tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProviderMetrics() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if metrics.RequestsTotal == nil {
					t.Error("RequestsTotal is nil")
				}
				if metrics.RateLimitsTotal == nil {
					t.Error("RateLimitsTotal is nil")
				}
				if metrics.RequestDuration == nil {
					t.Error("RequestDuration is nil")
				}
				if metrics.RetryQueueDepth == nil {
					t.Error("RetryQueueDepth is nil")
				}
			}
		})
	}
}

func TestProviderMetrics_RecordRequest(t *testing.T) {
	metrics, err := NewProviderMetrics("test")
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	ctx := context.Background()
	// Should not panic
	metrics.RecordRequest(ctx, "success", "Production")
	metrics.RecordRequest(ctx, "error", "Development")
}

func TestProviderMetrics_RecordRateLimit(t *testing.T) {
	metrics, err := NewProviderMetrics("test")
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	ctx := context.Background()
	// Should not panic
	metrics.RecordRateLimit(ctx)
}

func TestProviderMetrics_RecordDuration(t *testing.T) {
	metrics, err := NewProviderMetrics("test")
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	ctx := context.Background()
	// Should not panic
	metrics.RecordDuration(ctx, 100*time.Millisecond, "success")
	metrics.RecordDuration(ctx, 5*time.Second, "error")
}

func TestProviderMetrics_UpdateQueueDepth(t *testing.T) {
	metrics, err := NewProviderMetrics("test")
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	ctx := context.Background()
	// Should not panic
	metrics.UpdateQueueDepth(ctx, 1)
	metrics.UpdateQueueDepth(ctx, 5)
	metrics.UpdateQueueDepth(ctx, -3)
}
