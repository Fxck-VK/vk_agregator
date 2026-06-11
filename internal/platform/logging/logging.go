// Package logging provides safe structured logging helpers shared by binaries.
package logging

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
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
