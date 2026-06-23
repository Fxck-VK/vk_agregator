// Package logging provides safe structured logging helpers shared by binaries.
package logging

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"

	"go.opentelemetry.io/otel/trace"

	"vk-ai-aggregator/internal/domain"
)

// NewJSONHandler returns a JSON slog handler that adds trace_id/span_id from
// context when a log record is emitted with slog.*Context.
func NewJSONHandler(w io.Writer, opts *slog.HandlerOptions) slog.Handler {
	return traceHandler{next: slog.NewJSONHandler(w, opts)}
}

type traceHandler struct {
	next slog.Handler
}

func (h traceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h traceHandler) Handle(ctx context.Context, record slog.Record) error {
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		record.AddAttrs(
			slog.String("trace_id", span.TraceID().String()),
			slog.String("span_id", span.SpanID().String()),
		)
	}
	return h.next.Handle(ctx, record)
}

func (h traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceHandler{next: h.next.WithAttrs(attrs)}
}

func (h traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{next: h.next.WithGroup(name)}
}

type providerClassError interface {
	ProviderErrorClass() domain.ProviderErrorClass
}

// ErrorAttr returns a bounded error label for structured logs. It intentionally
// avoids err.Error(), because external errors can contain prompts, tokens,
// private URLs or raw provider/payment payload details.
func ErrorAttr(err error) slog.Attr {
	return slog.String("error_code", ErrorCode(err))
}

// ErrorCode maps errors to stable, low-cardinality labels safe for logs and
// metrics. Unknown errors fail closed to "internal_error".
func ErrorCode(err error) string {
	if err == nil {
		return "none"
	}
	if errors.Is(err, context.Canceled) {
		return "context_canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return "not_found"
	case errors.Is(err, domain.ErrConflict):
		return "conflict"
	case errors.Is(err, domain.ErrInsufficientCredits):
		return "insufficient_credits"
	case errors.Is(err, domain.ErrCostCapExceeded):
		return "cost_cap_exceeded"
	case errors.Is(err, domain.ErrCapacityDegraded):
		return "capacity_degraded"
	case errors.Is(err, domain.ErrActiveJobLimitExceeded):
		return "active_job_limit_exceeded"
	}
	var classified providerClassError
	if errors.As(err, &classified) {
		if code := safeErrorLabel(string(classified.ProviderErrorClass())); code != "" {
			return code
		}
		return "provider_error"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "network_timeout"
	}
	return "internal_error"
}

func safeErrorLabel(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
		if b.Len() >= 64 {
			break
		}
	}
	return strings.Trim(b.String(), "_")
}
