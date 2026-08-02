// Package outboxrelay drains the transactional outbox and publishes the
// resulting work to the worker queue. Job creation only records an
// "event.job.queued" outbox row inside the same transaction as the job; this
// relay is the single component that turns those rows into queue tasks, so a
// crash between commit and enqueue can never lose a job (audit A2). Delivery is
// at-least-once: a task may be re-published if the relay crashes after enqueue
// but before marking the row published, and workers deduplicate by job.
package outboxrelay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/logging"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/platform/uow"
)

const (
	// EventJobCreated is an explicit audit-only event.
	EventJobCreated = "event.job.created"
	// EventJobQueued is the outbox event type that maps to generation work.
	EventJobQueued = "event.job.queued"
	// EventJobResultReady maps to the existing finalization stream only after a
	// durable result_ready transition has committed.
	EventJobResultReady = "event.job.result_ready"
	// EventConversationTitleQueued maps a title-only task to its isolated
	// stream. Its payload must contain queue metadata only, never prompt or job
	// params.
	EventConversationTitleQueued = "event.conversation_title.queued"

	maxTrackedUnresolvedLeases = 1024
)

type eventAction uint8

const (
	actionAuditOnly eventAction = iota
	actionGeneration
	actionFinalization
	actionConversationTitle
)

// StreamPublisher combines normal generation enqueueing with explicit stream
// publication for finalization work.
type StreamPublisher interface {
	queue.Publisher
	PublishTo(ctx context.Context, stream string, task queue.Task) error
}

// Relay publishes pending outbox events to the worker queue.
type Relay struct {
	uow           uow.Manager
	pub           StreamPublisher
	batch         int
	leaseDuration time.Duration
	maxAttempts   int
	owner         string
	now           func() time.Time
	retryBackoff  func(attempt int) time.Duration
	log           *slog.Logger
	unresolvedMu  sync.Mutex
	unresolved    map[uuid.UUID]struct{}
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

// WithNow sets the relay clock.
func WithNow(now func() time.Time) Option {
	return func(r *Relay) {
		if now != nil {
			r.now = now
		}
	}
}

// WithLeaseDuration sets how long one claimed batch remains owned.
func WithLeaseDuration(duration time.Duration) Option {
	return func(r *Relay) {
		if duration > 0 {
			r.leaseDuration = duration
		}
	}
}

// WithMaxAttempts sets the terminal publication-attempt budget.
func WithMaxAttempts(attempts int) Option {
	return func(r *Relay) {
		if attempts > 0 {
			r.maxAttempts = attempts
		}
	}
}

// WithOwner sets the stable process identity recorded on claimed rows.
func WithOwner(owner string) Option {
	return func(r *Relay) {
		if owner = strings.TrimSpace(owner); owner != "" {
			r.owner = owner
		}
	}
}

// WithRetryBackoff sets the deterministic delay before a publication retry.
func WithRetryBackoff(backoff func(attempt int) time.Duration) Option {
	return func(r *Relay) {
		if backoff != nil {
			r.retryBackoff = backoff
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
func New(manager uow.Manager, pub StreamPublisher, opts ...Option) *Relay {
	r := &Relay{
		uow:           manager,
		pub:           pub,
		batch:         100,
		leaseDuration: 30 * time.Second,
		maxAttempts:   5,
		owner:         "outbox-relay",
		now:           time.Now,
		retryBackoff:  defaultRetryBackoff,
		log:           slog.Default(),
		unresolved:    make(map[uuid.UUID]struct{}),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Drain claims up to one batch in a short transaction, publishes each event
// outside any transaction, and conditionally resolves each claim in another
// short transaction. It returns the number of events resolved as published.
func (r *Relay) Drain(ctx context.Context) (int, error) {
	claimedAt := r.now()
	// Keep one monotonic origin for the entire claimed batch. Per-event SLO
	// samples must include the claim transaction and sequential time spent on
	// publications earlier in this batch, not just a single Redis client call.
	claimStarted := time.Now()
	var events []*domain.OutboxEvent
	if err := r.uow.Within(ctx, func(ctx context.Context, repos uow.Repositories) error {
		var err error
		events, err = repos.Outbox.ClaimPending(
			ctx,
			r.owner,
			claimedAt,
			claimedAt.Add(r.leaseDuration),
			r.batch,
		)
		return err
	}); err != nil {
		metrics.ObserveOutboxRelayClaim("batch", "error")
		metrics.ObserveOutboxRelayClaimDuration("error", time.Since(claimStarted))
		return 0, err
	}
	metrics.ObserveOutboxRelayClaimDuration("success", time.Since(claimStarted))

	var published int
	var firstErr error
	for _, event := range events {
		metrics.ObserveOutboxRelayClaim(event.EventType, "claimed")
		if r.recoveredLease(event.ID) {
			metrics.ObserveOutboxRelayLeaseRecovery(event.EventType, "recovered")
		}
		if event.ClaimToken == nil || *event.ClaimToken == uuid.Nil {
			err := errors.New("outboxrelay: claimed event has no claim token")
			firstErr = firstError(firstErr, err)
			metrics.ObserveOutboxRelayResolution(event.EventType, "resolve_error", "resolve_error")
			metrics.ObserveOutboxRelayLeaseRecovery(event.EventType, "awaiting_expiry")
			r.trackUnresolvedLease(event.ID)
			continue
		}

		action, task, err := prepare(event)
		if err != nil {
			firstErr = firstError(firstErr, err)
			metrics.ObserveOutboxRelayPublish(event.EventType, "invalid", "invalid_event")
			if resolveErr := r.fail(ctx, event, r.now(), "invalid_event"); resolveErr != nil {
				firstErr = firstError(firstErr, resolveErr)
				r.observeResolveFailure(event, resolveErr)
			} else {
				metrics.ObserveOutboxRelayResolution(event.EventType, "quarantine", "invalid_event")
			}
			continue
		}

		publishErr := r.publish(ctx, action, task)
		if action != actionAuditOnly && publishErr == nil {
			metrics.ObserveOutboxRelayClaimToAcknowledgedPublicationDuration(event.EventType, time.Since(claimStarted))
		}
		if publishErr != nil {
			err := publishErr
			firstErr = firstError(firstErr, err)
			metrics.ObserveOutboxRelayPublish(event.EventType, "error", "publish_error")
			attempt := event.Attempts + 1
			var resolveErr error
			if attempt >= r.maxAttempts {
				resolveErr = r.fail(ctx, event, r.now(), "publish_error")
				if resolveErr == nil {
					metrics.ObserveOutboxRelayResolution(event.EventType, "quarantine", "publish_error")
				}
			} else {
				retryAt := r.now().Add(nonNegativeDuration(r.retryBackoff(attempt)))
				resolveErr = r.retry(ctx, event, retryAt, "publish_error")
				if resolveErr == nil {
					metrics.ObserveOutboxRelayResolution(event.EventType, "retry", "publish_error")
				}
			}
			firstErr = firstError(firstErr, resolveErr)
			if resolveErr != nil {
				r.observeResolveFailure(event, resolveErr)
			}
			continue
		}

		if action == actionAuditOnly {
			metrics.ObserveOutboxRelayPublish(event.EventType, "audit_only", "none")
		} else {
			metrics.ObserveOutboxRelayPublish(event.EventType, "success", "none")
		}
		if err := r.markPublished(ctx, event, r.now()); err != nil {
			firstErr = firstError(firstErr, err)
			r.observeResolveFailure(event, err)
			continue
		}
		metrics.ObserveOutboxRelayResolution(event.EventType, "published", "none")
		published++
	}
	return published, firstErr
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
				r.log.Error("outbox relay drain failed", logging.ErrorAttr(err))
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

// conversationTitleQueuedPayload is the intentionally narrow schema for a
// background title request. Account ownership and Web/text provenance are
// validated before this task can reach the title worker. It never includes a
// prompt, job parameters, artifacts, or billing data.
type conversationTitleQueuedPayload struct {
	JobID         uuid.UUID            `json:"job_id"`
	AccountID     uuid.UUID            `json:"account_id"`
	Source        string               `json:"source"`
	Operation     domain.OperationType `json:"operation"`
	Modality      domain.Modality      `json:"modality"`
	CorrelationID string               `json:"correlation_id,omitempty"`
	Traceparent   string               `json:"traceparent,omitempty"`
}

// prepare validates the common job-event envelope and rebuilds its queue task.
// Creation remains audit-only after passing the same envelope validation.
func prepare(e *domain.OutboxEvent) (eventAction, queue.Task, error) {
	action, err := classify(e.EventType)
	if err != nil {
		return 0, queue.Task{}, err
	}
	if action == actionConversationTitle {
		return prepareConversationTitle(e)
	}
	var p queuedPayload
	if err := json.Unmarshal(e.Payload, &p); err != nil {
		return 0, queue.Task{}, err
	}
	if p.JobID == uuid.Nil || !p.Operation.Valid() || !p.Modality.Valid() {
		return 0, queue.Task{}, errors.New("outboxrelay: invalid executable event payload")
	}
	if e.AggregateType != "job" || e.AggregateID != p.JobID {
		return 0, queue.Task{}, errors.New("outboxrelay: executable event aggregate mismatch")
	}
	return action, queue.Task{
		JobID:         p.JobID,
		Operation:     p.Operation,
		Modality:      p.Modality,
		CorrelationID: p.CorrelationID,
		Traceparent:   p.Traceparent,
	}, nil
}

func prepareConversationTitle(e *domain.OutboxEvent) (eventAction, queue.Task, error) {
	var p conversationTitleQueuedPayload
	decoder := json.NewDecoder(bytes.NewReader(e.Payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return 0, queue.Task{}, fmt.Errorf("outboxrelay: invalid conversation title payload: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return 0, queue.Task{}, errors.New("outboxrelay: invalid conversation title payload suffix")
	}
	if p.JobID == uuid.Nil || p.AccountID == uuid.Nil || p.Source != "web" ||
		p.Operation != domain.OperationTextGenerate || p.Modality != domain.ModalityText {
		return 0, queue.Task{}, errors.New("outboxrelay: invalid conversation title event payload")
	}
	if e.AggregateType != "job" || e.AggregateID != p.JobID {
		return 0, queue.Task{}, errors.New("outboxrelay: conversation title event aggregate mismatch")
	}
	return actionConversationTitle, queue.Task{
		JobID:         p.JobID,
		Operation:     p.Operation,
		Modality:      p.Modality,
		CorrelationID: p.CorrelationID,
		Traceparent:   p.Traceparent,
	}, nil
}

// publish performs only the external operation and is always called after the
// claiming unit of work has returned.
func (r *Relay) publish(ctx context.Context, action eventAction, task queue.Task) error {
	if action == actionAuditOnly {
		return nil
	}
	if action == actionFinalization {
		return r.pub.PublishTo(ctx, redisqueue.StreamDelivery, task)
	}
	if action == actionConversationTitle {
		return r.pub.PublishTo(ctx, redisqueue.StreamConversationTitle, task)
	}
	return r.pub.Enqueue(ctx, task)
}

func (r *Relay) markPublished(ctx context.Context, event *domain.OutboxEvent, publishedAt time.Time) error {
	return r.resolve(ctx, event, func(ctx context.Context, outbox domain.OutboxRepository, token uuid.UUID) (bool, error) {
		return outbox.MarkPublishedClaimed(ctx, event.ID, token, publishedAt)
	})
}

func (r *Relay) retry(ctx context.Context, event *domain.OutboxEvent, retryAt time.Time, errorCode string) error {
	return r.resolve(ctx, event, func(ctx context.Context, outbox domain.OutboxRepository, token uuid.UUID) (bool, error) {
		return outbox.RetryClaimed(ctx, event.ID, token, retryAt, errorCode)
	})
}

func (r *Relay) fail(ctx context.Context, event *domain.OutboxEvent, failedAt time.Time, errorCode string) error {
	return r.resolve(ctx, event, func(ctx context.Context, outbox domain.OutboxRepository, token uuid.UUID) (bool, error) {
		return outbox.FailClaimed(ctx, event.ID, token, failedAt, errorCode)
	})
}

func (r *Relay) resolve(
	ctx context.Context,
	event *domain.OutboxEvent,
	fn func(context.Context, domain.OutboxRepository, uuid.UUID) (bool, error),
) error {
	token := *event.ClaimToken
	return r.uow.Within(ctx, func(ctx context.Context, repos uow.Repositories) error {
		updated, err := fn(ctx, repos.Outbox, token)
		if err != nil {
			return err
		}
		if !updated {
			return errClaimLost
		}
		return nil
	})
}

func (r *Relay) observeResolveFailure(event *domain.OutboxEvent, err error) {
	outcome := "resolve_error"
	failureClass := "resolve_error"
	if errors.Is(err, errClaimLost) {
		outcome = "lost_claim"
		failureClass = "lost_claim"
	}
	metrics.ObserveOutboxRelayResolution(event.EventType, outcome, failureClass)
	if !errors.Is(err, errClaimLost) {
		metrics.ObserveOutboxRelayLeaseRecovery(event.EventType, "awaiting_expiry")
		r.trackUnresolvedLease(event.ID)
	}
}

func (r *Relay) trackUnresolvedLease(id uuid.UUID) {
	r.unresolvedMu.Lock()
	defer r.unresolvedMu.Unlock()
	if _, exists := r.unresolved[id]; exists {
		return
	}
	if len(r.unresolved) >= maxTrackedUnresolvedLeases {
		for oldest := range r.unresolved {
			delete(r.unresolved, oldest)
			break
		}
	}
	r.unresolved[id] = struct{}{}
}

func (r *Relay) recoveredLease(id uuid.UUID) bool {
	r.unresolvedMu.Lock()
	defer r.unresolvedMu.Unlock()
	if _, exists := r.unresolved[id]; !exists {
		return false
	}
	delete(r.unresolved, id)
	return true
}

func firstError(current, candidate error) error {
	if current != nil || candidate == nil {
		return current
	}
	return candidate
}

var errClaimLost = errors.New("outboxrelay: claim no longer owned")

func nonNegativeDuration(duration time.Duration) time.Duration {
	if duration < 0 {
		return 0
	}
	return duration
}

func defaultRetryBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := time.Second
	for i := 1; i < attempt && delay < time.Minute; i++ {
		delay *= 2
		if delay > time.Minute {
			return time.Minute
		}
	}
	return delay
}

func classify(eventType string) (eventAction, error) {
	switch eventType {
	case EventJobCreated:
		return actionAuditOnly, nil
	case EventJobQueued:
		return actionGeneration, nil
	case EventJobResultReady:
		return actionFinalization, nil
	case EventConversationTitleQueued:
		return actionConversationTitle, nil
	default:
		return 0, fmt.Errorf("outboxrelay: unsupported event type %q", eventType)
	}
}
