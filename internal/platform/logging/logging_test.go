package logging

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"

	"vk-ai-aggregator/internal/domain"
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

func TestErrorCodeUsesProviderClassWithoutRawMessage(t *testing.T) {
	err := classifiedErr{
		class:   domain.ProviderErrAuthFailed,
		message: "Bearer secret leaked in provider body https://private.example/file?token=secret prompt text",
	}

	got := ErrorCode(err)
	if got != string(domain.ProviderErrAuthFailed) {
		t.Fatalf("ErrorCode() = %q, want %q", got, domain.ProviderErrAuthFailed)
	}
	for _, forbidden := range []string{"secret", "private.example", "prompt"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("ErrorCode leaked %q in %q", forbidden, got)
		}
	}
}

func TestErrorCodeFailsClosedForGenericSensitiveErrors(t *testing.T) {
	err := errors.New("raw prompt https://private.example/file?token=secret Authorization: Bearer abc")

	if got := ErrorCode(err); got != "internal_error" {
		t.Fatalf("ErrorCode() = %q, want internal_error", got)
	}
}

func TestErrorAttrUsesBoundedKey(t *testing.T) {
	attr := ErrorAttr(domain.ErrNotFound)

	if attr.Key != "error_code" {
		t.Fatalf("attr key = %q, want error_code", attr.Key)
	}
	if got := attr.Value.String(); got != "not_found" {
		t.Fatalf("attr value = %q, want not_found", got)
	}
}

type classifiedErr struct {
	class   domain.ProviderErrorClass
	message string
}

func (e classifiedErr) Error() string { return e.message }

func (e classifiedErr) ProviderErrorClass() domain.ProviderErrorClass { return e.class }
