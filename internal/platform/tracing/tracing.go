// Package tracing wraps OpenTelemetry setup and context propagation helpers.
package tracing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "vk-ai-aggregator"

// Config controls trace initialization.
type Config struct {
	ServiceName         string
	Exporter            string
	OTLPEndpoint        string
	SampleRatio         float64
	CriticalSampleRatio float64
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
	if exporter != "stdout" && exporter != "otlp" {
		return nil, fmt.Errorf("tracing: unsupported OTEL_TRACES_EXPORTER %q", cfg.Exporter)
	}

	exp, err := newExporter(ctx, exporter, cfg)
	if err != nil {
		return nil, err
	}
	res, err := resource.New(ctx, resource.WithAttributes(attribute.String("service.name", serviceName)))
	if err != nil {
		return nil, fmt.Errorf("tracing: resource: %w", err)
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRatio(cfg.SampleRatio)))),
	)
	otel.SetTracerProvider(provider)
	if logger != nil {
		logger.Info("tracing enabled", "exporter", exporter, "service", serviceName)
	}
	return provider.Shutdown, nil
}

func newExporter(ctx context.Context, exporter string, cfg Config) (sdktrace.SpanExporter, error) {
	switch exporter {
	case "stdout":
		exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("tracing: stdout exporter: %w", err)
		}
		return exp, nil
	case "otlp":
		endpoint := otlpEndpoint(cfg.OTLPEndpoint)
		exp, err := otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(endpoint),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("tracing: otlp exporter: %w", err)
		}
		return exp, nil
	default:
		return nil, fmt.Errorf("tracing: unsupported exporter %q", exporter)
	}
}

func otlpEndpoint(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "127.0.0.1:4317"
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err == nil && u.Host != "" {
			return u.Host
		}
	}
	return strings.TrimPrefix(raw, "//")
}

func sampleRatio(v float64) float64 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 1
	}
	rounded, err := strconv.ParseFloat(strconv.FormatFloat(v, 'f', 6, 64), 64)
	if err != nil {
		return v
	}
	return rounded
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
	class := SafeErrorClass(err)
	span.RecordError(safeTraceError(class), trace.WithAttributes(attribute.String("error.class", class)))
	span.SetStatus(codes.Error, class)
}

type safeTraceError string

func (e safeTraceError) Error() string {
	if e == "" {
		return "error"
	}
	return string(e)
}

// SafeErrorClass maps an error to a bounded class for traces. It deliberately
// avoids recording raw error text because provider/payment errors can contain
// payload fragments, URLs or credentials.
func SafeErrorClass(err error) string {
	if err == nil {
		return "none"
	}
	switch {
	case errors.Is(err, context.Canceled):
		return "context_canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "timeout"):
		return "timeout"
	case strings.Contains(msg, "rate limit") || strings.Contains(msg, "too many requests"):
		return "rate_limit"
	case strings.Contains(msg, "unauthorized") || strings.Contains(msg, "forbidden"):
		return "auth"
	case strings.Contains(msg, "not found"):
		return "not_found"
	case strings.Contains(msg, "conflict") || strings.Contains(msg, "duplicate"):
		return "conflict"
	case strings.Contains(msg, "invalid") || strings.Contains(msg, "validation"):
		return "invalid_input"
	case strings.Contains(msg, "unavailable") || strings.Contains(msg, "connection refused") || strings.Contains(msg, "no such host"):
		return "unavailable"
	default:
		return "error"
	}
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
