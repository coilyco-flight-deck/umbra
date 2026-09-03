package otelsink

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// ProviderConfig wires an OTLP/HTTP exporter. Endpoint and headers follow the
// OTEL_EXPORTER_* environment when left zero. See docs/audit-spans.md.
type ProviderConfig struct {
	// ServiceName names the emitting binary in the resource. Required: an
	// unnamed service is unattributable in a backend holding several.
	ServiceName string

	// ServiceVersion is optional and omitted from the resource when empty.
	ServiceVersion string

	// Endpoint overrides OTEL_EXPORTER_OTLP_ENDPOINT when set.
	Endpoint string

	// Insecure sends over plain HTTP. Off by default.
	Insecure bool
}

// NewProvider builds a batching TracerProvider exporting over OTLP/HTTP. The
// caller owns shutdown: call Shutdown to flush before the process exits.
func NewProvider(ctx context.Context, cfg ProviderConfig) (*sdktrace.TracerProvider, error) {
	if cfg.ServiceName == "" {
		return nil, fmt.Errorf("otelsink: ProviderConfig.ServiceName is required")
	}
	opts := []otlptracehttp.Option{}
	if cfg.Endpoint != "" {
		opts = append(opts, otlptracehttp.WithEndpointURL(cfg.Endpoint))
	}
	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	exp, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("otelsink: build otlp exporter: %w", err)
	}
	attrs := []any{semconv.ServiceName(cfg.ServiceName)}
	if cfg.ServiceVersion != "" {
		attrs = append(attrs, semconv.ServiceVersion(cfg.ServiceVersion))
	}
	res, err := resource.Merge(resource.Default(), resourceOf(attrs))
	if err != nil {
		return nil, fmt.Errorf("otelsink: build resource: %w", err)
	}
	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	), nil
}
