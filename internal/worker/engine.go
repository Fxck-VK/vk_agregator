package worker

import (
	"context"
	"log/slog"
	"time"

	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/platform/queue"
)

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
	if err := e.Recover(ctx); err != nil {
		e.logger.WarnContext(ctx, "worker recovery failed", "error", err)
	}
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := e.Poll(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			e.logger.WarnContext(ctx, "worker poll failed", "error", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}
		}
	}
}

// dispatch processes each delivery, acknowledging only those handled without
// error. It returns the count handled successfully.
func (e *Engine) dispatch(ctx context.Context, deliveries []redisqueue.Delivery) int {
	handled := 0
	for _, d := range deliveries {
		if err := e.handle(ctx, d.Task); err != nil {
			e.logger.WarnContext(ctx, "worker handler failed; leaving entry pending",
				"stream", d.Stream, "job_id", d.Task.JobID, "error", err)
			continue
		}
		if err := e.reader.Ack(ctx, d.Stream, d.ID); err != nil {
			e.logger.WarnContext(ctx, "worker ack failed", "stream", d.Stream, "error", err)
			continue
		}
		handled++
	}
	return handled
}
