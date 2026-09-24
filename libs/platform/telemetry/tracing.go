// Package telemetry wires OpenTelemetry tracing and Prometheus metrics.
package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// TracingConfig configures the global tracer provider.
type TracingConfig struct {
	ServiceName string
	Version     string
	Environment string
	// Enabled turns exporting on. When false spans are still created (and
	// trace IDs still propagate), but nothing is exported.
	Enabled bool
	// OTLPEndpoint is host:port of an OTLP/HTTP collector, e.g. jaeger:4318.
	OTLPEndpoint string
	Insecure     bool
	// SampleRatio in [0,1]. Parent-based; defaults to 1.
	SampleRatio float64
}

// ShutdownFunc flushes and stops telemetry.
type ShutdownFunc func(context.Context) error

// SetupTracing installs a global tracer provider and W3C propagators.
func SetupTracing(ctx context.Context, cfg TracingConfig) (ShutdownFunc, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Schemaless: merging with resource.Default() must not depend on the
	// semconv schema version bundled with the SDK.
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.Version),
		semconv.DeploymentEnvironment(cfg.Environment),
	))
	if err != nil {
		return nil, fmt.Errorf("telemetry resource: %w", err)
	}

	ratio := cfg.SampleRatio
	if ratio <= 0 || ratio > 1 {
		ratio = 1
	}
	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
	}

	if cfg.Enabled {
		expOpts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(cfg.OTLPEndpoint)}
		if cfg.Insecure {
			expOpts = append(expOpts, otlptracehttp.WithInsecure())
		}
		exp, err := otlptracehttp.New(ctx, expOpts...)
		if err != nil {
			return nil, fmt.Errorf("otlp exporter: %w", err)
		}
		opts = append(opts, sdktrace.WithBatcher(exp, sdktrace.WithBatchTimeout(5*time.Second)))
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}
