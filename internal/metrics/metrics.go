package metrics

import (
	"context"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

const (
	meterName = "jasm"
)

// Attribute keys for metrics.
const (
	AttrStatus = "status"
	AttrVault  = "vault"
)

// ProviderMetrics contains metrics for a secret provider.
type ProviderMetrics struct {
	RequestsTotal   metric.Int64Counter
	RateLimitsTotal metric.Int64Counter
	RequestDuration metric.Float64Histogram
	RetryQueueDepth metric.Int64UpDownCounter
}

// Config holds configuration for metrics setup.
type Config struct {
	Endpoint string
	Insecure bool
}

// ConfigFromEnv creates a Config from environment variables.
func ConfigFromEnv() Config {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:4317"
	}
	return Config{
		Endpoint: endpoint,
		Insecure: os.Getenv("OTEL_EXPORTER_OTLP_INSECURE") == "true",
	}
}

// SetupMeterProvider initializes the OTLP meter provider.
// Returns a shutdown function that should be called when the application exits.
// If the OTLP endpoint is unavailable, metrics will be dropped silently.
func SetupMeterProvider(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}

	exporter, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	reader := sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(30*time.Second))

	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	otel.SetMeterProvider(provider)

	return provider.Shutdown, nil
}

// NewProviderMetrics creates metrics instruments for a secret provider.
func NewProviderMetrics(provider string) (*ProviderMetrics, error) {
	meter := otel.Meter(meterName)

	requestsTotal, err := meter.Int64Counter(
		"jasm_"+provider+"_requests_total",
		metric.WithDescription("Total number of requests to "+provider+" API"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	rateLimitsTotal, err := meter.Int64Counter(
		"jasm_"+provider+"_rate_limits_total",
		metric.WithDescription("Total number of rate limit responses from "+provider+" API"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	requestDuration, err := meter.Float64Histogram(
		"jasm_"+provider+"_request_duration_seconds",
		metric.WithDescription("Duration of requests to "+provider+" API"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	retryQueueDepth, err := meter.Int64UpDownCounter(
		"jasm_"+provider+"_retry_queue_depth",
		metric.WithDescription("Current depth of the retry queue for "+provider),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	return &ProviderMetrics{
		RequestsTotal:   requestsTotal,
		RateLimitsTotal: rateLimitsTotal,
		RequestDuration: requestDuration,
		RetryQueueDepth: retryQueueDepth,
	}, nil
}

// RecordRequest records a request to the provider.
func (m *ProviderMetrics) RecordRequest(ctx context.Context, status string, vault string) {
	m.RequestsTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String(AttrStatus, status),
			attribute.String(AttrVault, vault),
		),
	)
}

// RecordRateLimit records a rate limit response.
func (m *ProviderMetrics) RecordRateLimit(ctx context.Context) {
	m.RateLimitsTotal.Add(ctx, 1)
}

// RecordDuration records the duration of a request.
func (m *ProviderMetrics) RecordDuration(ctx context.Context, duration time.Duration, status string) {
	m.RequestDuration.Record(ctx, duration.Seconds(),
		metric.WithAttributes(attribute.String(AttrStatus, status)),
	)
}

// UpdateQueueDepth updates the retry queue depth.
func (m *ProviderMetrics) UpdateQueueDepth(ctx context.Context, delta int64) {
	m.RetryQueueDepth.Add(ctx, delta)
}
