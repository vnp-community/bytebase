// Package otel provides OpenTelemetry tracing initialization for Bytebase services.
package otel

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// TracerConfig holds configuration for OpenTelemetry tracing.
type TracerConfig struct {
	ServiceName string
	SampleRate  float64 // 0.0 to 1.0
}

// InitTracer initializes OTel TracerProvider.
// If OTEL_EXPORTER_OTLP_ENDPOINT env var is not set, returns a noop tracer.
func InitTracer(ctx context.Context, cfg TracerConfig) (func(context.Context) error, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		// No exporter configured — use noop tracer.
		otel.SetTracerProvider(noop.NewTracerProvider())
		slog.Info("OTel tracing disabled (no OTEL_EXPORTER_OTLP_ENDPOINT)")
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
	}

	sampler := sdktrace.AlwaysSample()
	if cfg.SampleRate > 0 && cfg.SampleRate < 1.0 {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	slog.Info("OTel tracing initialized", "endpoint", endpoint, "sample_rate", cfg.SampleRate)
	return tp.Shutdown, nil
}

// Tracer returns a named tracer for a service.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
