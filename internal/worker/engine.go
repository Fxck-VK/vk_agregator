package worker

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"

	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/platform/logging"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/platform/tracing"
)

// ExponentialBackoff returns a backoff function that grows as base*2^(attempt-1)
// capped at max. attempt is 1-based. A non-positive base disables backoff.
func ExponentialBackoff(base, max time.Duration) func(attempt int) time.Duration {
	return func(attempt int) time.Duration {
		if base <= 0 || attempt <= 0 {
			return 0
		}
		d := base
		for i := 1; i < attempt; i++ {
			d *= 2
			if max > 0 && d >= max {
				return max
			}
		}
		if max > 0 && d > max {
			return max
		}
		return d
	}
}

// Reader is the slice of the Redis consumer the engine needs. It is satisfied
// by *redisqueue.Consumer and faked in tests.
type Reader interface {
	Read(ctx context.Context, opts redisqueue.ReadOptions) ([]redisqueue.Delivery, error)
	Ack(ctx context.Context, stream string, ids ...string) error
	AutoClaim(ctx context.Context, stream string, minIdle time.Duration, count int64) ([]redisqueue.Delivery, error)
}

// Handler processes a single task. Returning nil means "done, acknowledge";
// returning an error leaves the entry pending so it is retried/recovered.
type Handler func(ctx context.Context, task queue.Task) error

// Engine drives a worker: it reads tasks from its streams, dispatches each to a
// Handler and acknowledges only successfully handled entries. Unacknowledged
// entries stay in the consumer group's pending list and are reclaimed on the
// next Recover, which provides at-least-once delivery and restart recovery.
type Engine struct {
	reader  Reader
	streams []string
	handle  Handler
	block   time.Duration
	count   int64
	minIdle time.Duration
	logger  *slog.Logger
}

// EngineOption customizes an Engine.
type EngineOption func(*Engine)

// WithBlock sets how long each read blocks waiting for new entries.
func WithBlock(d time.Duration) EngineOption { return func(e *Engine) { e.block = d } }

// WithCount sets the max entries read per batch.
func WithCount(n int64) EngineOption { return func(e *Engine) { e.count = n } }

// WithMinIdle sets how long an entry must be pending before recovery reclaims it.
func WithMinIdle(d time.Duration) EngineOption { return func(e *Engine) { e.minIdle = d } }

// WithLogger sets the structured logger.
func WithLogger(l *slog.Logger) EngineOption { return func(e *Engine) { e.logger = l } }

// NewEngine builds an Engine reading the given streams and dispatching to handle.
func NewEngine(reader Reader, streams []string, handle Handler, opts ...EngineOption) *Engine {
	e := &Engine{
		reader:  reader,
		streams: streams,
		handle:  handle,
		block:   2 * time.Second,
		count:   16,
		minIdle: 30 * time.Second,
		logger:  slog.Default(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Recover reclaims entries left pending by crashed consumers and reprocesses
// them. It should be called once on startup before the main loop.
func (e *Engine) Recover(ctx context.Context) error {
	for _, stream := range e.streams {
		deliveries, err := e.reader.AutoClaim(ctx, stream, e.minIdle, e.count)
		if err != nil {
			return err
		}
		e.dispatch(ctx, deliveries)
	}
	return nil
}

// Poll performs a single read-process-ack cycle and returns the number of
// entries handled.
func (e *Engine) Poll(ctx context.Context) (int, error) {
	deliveries, err := e.reader.Read(ctx, redisqueue.ReadOptions{Streams: e.streams, Count: e.count, Block: e.block})
	if err != nil {
		return 0, err
	}
	return e.dispatch(ctx, deliveries), nil
}

// Run recovers pending work then loops Poll until the context is cancelled.
func (e *Engine) Run(ctx context.Context) error {
	return e.RunWithHandlerContext(ctx, ctx)
}

// RunWithHandlerContext recovers pending work and polls with readCtx, while
// handlers receive handlerCtx. Worker entrypoints use this to stop taking new
// Redis entries on shutdown while allowing in-flight handlers to drain.
func (e *Engine) RunWithHandlerContext(readCtx, handlerCtx context.Context) error {
	if err := e.RecoverWithHandlerContext(readCtx, handlerCtx); err != nil {
		e.logger.WarnContext(readCtx, "worker recovery failed", logging.ErrorAttr(err))
	}
	for {
		if readCtx.Err() != nil {
			return readCtx.Err()
		}
		if _, err := e.PollWithHandlerContext(readCtx, handlerCtx); err != nil {
			if readCtx.Err() != nil {
				return readCtx.Err()
			}
			e.logger.WarnContext(readCtx, "worker poll failed", logging.ErrorAttr(err))
			select {
			case <-readCtx.Done():
				return readCtx.Err()
			case <-time.After(time.Second):
			}
		}
	}
}

// RecoverWithHandlerContext is Recover with a distinct handler context.
func (e *Engine) RecoverWithHandlerContext(readCtx, handlerCtx context.Context) error {
	for _, stream := range e.streams {
		deliveries, err := e.reader.AutoClaim(readCtx, stream, e.minIdle, e.count)
		if err != nil {
			return err
		}
		e.dispatch(handlerCtx, deliveries)
	}
	return nil
}

// PollWithHandlerContext is Poll with a distinct handler context.
func (e *Engine) PollWithHandlerContext(readCtx, handlerCtx context.Context) (int, error) {
	deliveries, err := e.reader.Read(readCtx, redisqueue.ReadOptions{Streams: e.streams, Count: e.count, Block: e.block})
	if err != nil {
		return 0, err
	}
	return e.dispatch(handlerCtx, deliveries), nil
}

// dispatch processes each delivery, acknowledging only those handled without
// error. It returns the count handled successfully.
func (e *Engine) dispatch(ctx context.Context, deliveries []redisqueue.Delivery) int {
	handled := 0
	for _, d := range deliveries {
		started := time.Now()
		phase := streamPhase(d.Stream)
		operation := string(d.Task.Operation)
		modality := string(d.Task.Modality)
		taskCtx := tracing.ContextWithTraceparent(ctx, d.Task.Traceparent)
		taskCtx, span := tracing.Start(taskCtx, "worker.task",
			attribute.String("messaging.system", "redis"),
			attribute.String("messaging.source.name", d.Stream),
			attribute.String("messaging.message.id", d.ID),
			attribute.String("job.id", d.Task.JobID.String()),
			attribute.String("operation", string(d.Task.Operation)),
			tracing.CorrelationAttr(d.Task.CorrelationID),
		)
		if err := e.handle(taskCtx, d.Task); err != nil {
			tracing.RecordError(span, err)
			span.End()
			metrics.WorkerTaskDuration.WithLabelValues(phase, operation, modality, "error").Observe(time.Since(started).Seconds())
			metrics.WorkerRetries.WithLabelValues(phase, operation, modality).Inc()
			e.logger.WarnContext(taskCtx, "worker handler failed; leaving entry pending",
				"stream", d.Stream, "job_id", d.Task.JobID, logging.ErrorAttr(err))
			continue
		}
		if err := e.reader.Ack(taskCtx, d.Stream, d.ID); err != nil {
			tracing.RecordError(span, err)
			span.End()
			metrics.WorkerTaskDuration.WithLabelValues(phase, operation, modality, "ack_error").Observe(time.Since(started).Seconds())
			e.logger.WarnContext(taskCtx, "worker ack failed", "stream", d.Stream, logging.ErrorAttr(err))
			continue
		}
		span.End()
		metrics.WorkerTaskDuration.WithLabelValues(phase, operation, modality, "success").Observe(time.Since(started).Seconds())
		handled++
	}
	return handled
}

func streamPhase(stream string) string {
	switch stream {
	case redisqueue.StreamText, redisqueue.StreamImage, redisqueue.StreamVideo:
		return "generation"
	case redisqueue.StreamProviderPoll:
		return "provider_poll"
	case redisqueue.StreamDelivery:
		return "delivery"
	default:
		return "unknown"
	}
}
