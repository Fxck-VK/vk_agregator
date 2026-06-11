package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestTraceHandlerAddsTraceFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewJSONHandler(&buf, nil))

	traceID := trace.TraceID{0x01}
	spanID := trace.SpanID{0x02}
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
		Remote:  true,
	}))

	logger.InfoContext(ctx, "test message", slog.String("surface", "test"))

	out := buf.String()
	if !strings.Contains(out, `"trace_id":"`+traceID.String()+`"`) {
		t.Fatalf("log output missing trace_id: %s", out)
	}
	if !strings.Contains(out, `"span_id":"`+spanID.String()+`"`) {
		t.Fatalf("log output missing span_id: %s", out)
	}
	if strings.Contains(out, "prompt") || strings.Contains(out, "payload") {
		t.Fatalf("log output included forbidden content: %s", out)
	}
}
