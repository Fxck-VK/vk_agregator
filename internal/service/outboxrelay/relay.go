// Package outboxrelay drains the transactional outbox and publishes the
// resulting work to the worker queue. Job creation only records an
// "event.job.queued" outbox row inside the same transaction as the job; this
// relay is the single component that turns those rows into queue tasks, so a
// crash between commit and enqueue can never lose a job (audit A2). Delivery is
// at-least-once: a task may be re-published if the relay crashes after enqueue
// but before marking the row published, and workers deduplicate by job.
package outboxrelay

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/platform/uow"
)

// EventJobQueued is the outbox event type that maps to a worker enqueue.
const EventJobQueued = "event.job.queued"

// Relay publishes pending outbox events to the worker queue.
type Relay struct {
	uow   uow.Manager
	pub   queue.Publisher
	batch int
	log   *slog.Logger
}

// Option customizes a Relay.
type Option func(*Relay)

// WithBatchSize sets how many events are drained per pass (default 100).
func WithBatchSize(n int) Option {
	return func(r *Relay) {
		if n > 0 {
			r.batch = n
		}
	}
}

// WithLogger sets the relay logger.
func WithLogger(l *slog.Logger) Option {
	return func(r *Relay) {
		if l != nil {
			r.log = l
		}
	}
}

// New builds a Relay over the unit-of-work manager and queue publisher.
func New(manager uow.Manager, pub queue.Publisher, opts ...Option) *Relay {
	r := &Relay{uow: manager, pub: pub, batch: 100, log: slog.Default()}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Drain publishes up to one batch of pending events and marks them published.
// Fetch, publish and mark run in one transaction so the FOR UPDATE SKIP LOCKED
// rows stay locked until the publish is durable. It returns the number of
// events published.
func (r *Relay) Drain(ctx context.Context) (int, error) {
	var published int
	err := r.uow.Within(ctx, func(ctx context.Context, repos uow.Repositories) error {
		events, err := repos.Outbox.FetchPending(ctx, r.batch)
		if err != nil {
			return err
		}
		for _, e := range events {
			if err := r.publish(ctx, e); err != nil {
				return err
			}
			if err := repos.Outbox.MarkPublished(ctx, e.ID, time.Now()); err != nil {
				return err
			}
			published++
		}
		return nil
	})
	return published, err
}

// Run drains the outbox on an interval until the context is cancelled.
func (r *Relay) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := r.Drain(ctx); err != nil {
				r.log.Error("outbox relay drain failed", "error", err)
			} else if n > 0 {
				r.log.Debug("outbox relay published events", "count", n)
			}
		}
	}
}

// queuedPayload is the subset of the queued event payload needed to rebuild the
// worker task.
type queuedPayload struct {
	JobID         uuid.UUID            `json:"job_id"`
	Operation     domain.OperationType `json:"operation"`
	Modality      domain.Modality      `json:"modality"`
	CorrelationID string               `json:"correlation_id"`
	Traceparent   string               `json:"traceparent"`
}

// publish turns a single outbox event into a queue task. Non-enqueue events are
// treated as audit-only and acknowledged without publishing.
func (r *Relay) publish(ctx context.Context, e *domain.OutboxEvent) error {
	if e.EventType != EventJobQueued {
		return nil
	}
	var p queuedPayload
	if err := json.Unmarshal(e.Payload, &p); err != nil {
		return err
	}
	return r.pub.Enqueue(ctx, queue.Task{
		JobID:         p.JobID,
		Operation:     p.Operation,
		Modality:      p.Modality,
		CorrelationID: p.CorrelationID,
		Traceparent:   p.Traceparent,
	})
}
