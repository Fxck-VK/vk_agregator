// Package tracing wraps OpenTelemetry setup and context propagation helpers.
package tracing

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "vk-ai-aggregator"

// Config controls trace initialization.
type Config struct {
	ServiceName string
	Exporter    string
}

// Init configures OpenTelemetry trace context propagation and an optional trace
// exporter. "none" keeps the default no-op tracer provider while still allowing
// traceparent injection/extraction across async queue boundaries.
func Init(ctx context.Context, cfg Config, logger *slog.Logger) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.TraceContext{})

	serviceName := strings.TrimSpace(cfg.ServiceName)
	if serviceName == "" {
		serviceName = "vk-ai-aggregator"
	}
	exporter := strings.ToLower(strings.TrimSpace(cfg.Exporter))
	if exporter == "" || exporter == "none" {
		return func(context.Context) error { return nil }, nil
	}
	if exporter != "stdout" {
		return nil, fmt.Errorf("tracing: unsupported OTEL_TRACES_EXPORTER %q", cfg.Exporter)
	}

	exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, fmt.Errorf("tracing: stdout exporter: %w", err)
	}
	res, err := resource.New(ctx, resource.WithAttributes(attribute.String("service.name", serviceName)))
	if err != nil {
		return nil, fmt.Errorf("tracing: resource: %w", err)
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(provider)
	if logger != nil {
		logger.Info("tracing enabled", "exporter", exporter, "service", serviceName)
	}
	return provider.Shutdown, nil
}

// Start starts a span using the project tracer.
func Start(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, name, trace.WithAttributes(attrs...))
}

// RecordError records err on span and marks the span as failed.
func RecordError(span trace.Span, err error) {
	if err == nil || span == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// Traceparent injects the current trace context into a W3C traceparent string.
func Traceparent(ctx context.Context) string {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier.Get("traceparent")
}

// ContextWithTraceparent extracts a W3C traceparent string into ctx.
func ContextWithTraceparent(ctx context.Context, traceparent string) context.Context {
	traceparent = strings.TrimSpace(traceparent)
	if traceparent == "" {
		return ctx
	}
	carrier := propagation.MapCarrier{"traceparent": traceparent}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

// CorrelationAttr returns the standard correlation_id span attribute.
func CorrelationAttr(correlationID string) attribute.KeyValue {
	return attribute.String("correlation_id", correlationID)
}
