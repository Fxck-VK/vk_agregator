package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/platform/tracing"
)

// ObjectStore fetches stored artifact bytes for generation and polling flows.
type ObjectStore interface {
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
}

// ExternalPublisher is the channel adapter boundary used for external-push
// finalization. Implementations own all channel-specific target, formatting,
// upload, replay-validation and persisted-send behavior.
type ExternalPublisher interface {
	Channel() domain.Channel
	BuildDelivery(ctx context.Context, job *domain.Job, idempotencyKey string) (*domain.Delivery, error)
	Publish(ctx context.Context, job *domain.Job, delivery *domain.Delivery) error
}

// DeliveryBiller captures a job's reserved credits once it is delivered.
type DeliveryBiller interface {
	CaptureForJob(ctx context.Context, jobID uuid.UUID, amount int64) error
	ReleaseForJob(ctx context.Context, jobID uuid.UUID) error
}

// CompletionReadiness is the narrow canonical result gate used before any
// successful publication or capture. Implementations must verify the exact
// account owner and every durable output without exposing result data.
type CompletionReadiness interface {
	RequireCompletionReady(ctx context.Context, accountID, jobID uuid.UUID) error
}

// DeliveryDeps bundles the delivery worker's collaborators.
type DeliveryDeps struct {
	Jobs       domain.JobRepository
	Deliveries domain.DeliveryRepository
	Artifacts  domain.ArtifactRepository
	Publishers []ExternalPublisher
	Billing    DeliveryBiller
	Readiness  CompletionReadiness
	// Streams, when set, receives dead-lettered delivery tasks once the retry
	// budget is exhausted.
	Streams StreamPublisher
	// MaxAttempts caps delivery send attempts before dead-lettering (default 3).
	MaxAttempts int
	// Backoff returns the delay before the next delivery retry; defaults to none.
	Backoff func(attempt int) time.Duration
	Now     func() time.Time
}

// DeliveryWorker consumes the delivery stream and finalizes ready results.
// Account-history results capture without an external delivery row. External
// push results are delegated to the publisher selected by the persisted target.
type DeliveryWorker struct {
	jobs        domain.JobRepository
	deliveries  domain.DeliveryRepository
	publishers  map[domain.Channel]ExternalPublisher
	billing     DeliveryBiller
	readiness   CompletionReadiness
	streams     StreamPublisher
	maxAttempts int
	backoff     func(attempt int) time.Duration
	now         func() time.Time
}

// NewDeliveryWorker builds a DeliveryWorker.
func NewDeliveryWorker(d DeliveryDeps) *DeliveryWorker {
	now := d.Now
	if now == nil {
		now = time.Now
	}
	maxAttempts := d.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = maxProviderAttempts
	}
	backoff := d.Backoff
	if backoff == nil {
		backoff = func(int) time.Duration { return 0 }
	}
	publishers := make(map[domain.Channel]ExternalPublisher, len(d.Publishers))
	for _, publisher := range d.Publishers {
		if publisher != nil {
			publishers[publisher.Channel()] = publisher
		}
	}
	return &DeliveryWorker{
		jobs:        d.Jobs,
		deliveries:  d.Deliveries,
		publishers:  publishers,
		billing:     d.Billing,
		readiness:   d.Readiness,
		streams:     d.Streams,
		maxAttempts: maxAttempts,
		backoff:     backoff,
		now:         now,
	}
}

// Process delivers one job's result. Returning nil acknowledges the task;
// returning an error leaves it pending for retry/recovery.
func (w *DeliveryWorker) Process(ctx context.Context, task queue.Task) error {
	ctx, span := tracing.Start(ctx, "delivery.process",
		attribute.String("job.id", task.JobID.String()),
		attribute.String("operation", string(task.Operation)),
		tracing.CorrelationAttr(task.CorrelationID),
	)
	defer span.End()

	job, err := w.jobs.GetByID(ctx, task.JobID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		tracing.RecordError(span, err)
		return err
	}
	span.SetAttributes(attribute.String("job.status", string(job.Status)))

	switch job.Status {
	case domain.JobStatusSucceeded:
		return nil
	case domain.JobStatusResultReady, domain.JobStatusDelivering:
		// Ready for result-mode finalization or resuming an external push.
	case domain.JobStatusFailedTerminal:
		if !isTerminalMediaFailure(job) {
			return nil
		}
	default:
		return nil
	}

	if err := job.ValidateResultContract(); err != nil {
		tracing.RecordError(span, err)
		return err
	}

	switch job.ResultMode {
	case domain.ResultModeAccountHistory:
		if job.Status == domain.JobStatusFailedTerminal {
			return nil
		}
		return w.finalizeAccountHistory(ctx, span, job)
	case domain.ResultModeExternalPush:
		return w.finalizeExternalPush(ctx, span, task, job)
	case domain.ResultModeLegacyUnknown:
		return fmt.Errorf("%w: legacy-unknown result cannot be finalized", domain.ErrInvalidResultContract)
	default:
		return fmt.Errorf("%w: unsupported result mode %q", domain.ErrInvalidResultContract, job.ResultMode)
	}
}

func (w *DeliveryWorker) finalizeAccountHistory(ctx context.Context, span trace.Span, job *domain.Job) error {
	if job.Status != domain.JobStatusResultReady {
		return fmt.Errorf("%w: account-history job must remain result_ready until capture", domain.ErrInvalidResultContract)
	}
	if w.readiness == nil {
		metrics.ObserveFinalizationReadinessFailure(string(job.ResultMode), "unconfigured")
		return errors.New("worker: account-history result readiness is not configured")
	}
	if err := w.readiness.RequireCompletionReady(ctx, job.AccountID, job.ID); err != nil {
		metrics.ObserveFinalizationReadinessFailure(string(job.ResultMode), completionReadinessFailureReason(err))
		tracing.RecordError(span, err)
		return fmt.Errorf("worker: account-history result is not completion-ready: %w", err)
	}
	captureLatencyOrigin := job.UpdatedAt
	capturePending := job.ChargeAmountCredits() > 0 && job.CostCaptured != job.ChargeAmountCredits()
	if err := w.captureReserved(ctx, job); err != nil {
		tracing.RecordError(span, err)
		return err
	}
	if capturePending {
		observeSuccessfulCaptureLatency(job, captureLatencyOrigin)
	}
	metrics.JobsTerminal.WithLabelValues(string(domain.JobStatusSucceeded)).Inc()
	return w.setStatus(ctx, job, domain.JobStatusSucceeded, "", "")
}

func (w *DeliveryWorker) finalizeExternalPush(ctx context.Context, span trace.Span, task queue.Task, job *domain.Job) error {
	if job.AccountID == uuid.Nil {
		metrics.ObserveFinalizationReadinessFailure(string(job.ResultMode), "missing_owner")
		return fmt.Errorf("%w: canonical external result has no account owner", domain.ErrInvalidResultContract)
	}
	failureNotice := job.Status == domain.JobStatusFailedTerminal
	if !failureNotice {
		if w.readiness == nil {
			metrics.ObserveFinalizationReadinessFailure(string(job.ResultMode), "unconfigured")
			return fmt.Errorf("%w: canonical external result readiness is unavailable", domain.ErrInvalidResultContract)
		}
		if err := w.readiness.RequireCompletionReady(ctx, job.AccountID, job.ID); err != nil {
			metrics.ObserveFinalizationReadinessFailure(string(job.ResultMode), completionReadinessFailureReason(err))
			return fmt.Errorf("worker: external-push result is not completion-ready: %w", err)
		}
	}
	if job.DeliveryTarget == nil {
		return fmt.Errorf("%w: external push has no delivery target", domain.ErrInvalidResultContract)
	}
	publisher, ok := w.publishers[job.DeliveryTarget.Channel]
	if !ok {
		return fmt.Errorf("%w: no publisher for channel %q", domain.ErrInvalidResultContract, job.DeliveryTarget.Channel)
	}

	delivery, err := w.ensureDelivery(ctx, job, publisher)
	if err != nil {
		tracing.RecordError(span, err)
		return err
	}

	if delivery.Status == domain.DeliveryStatusFailed {
		return w.finalizeFailedDelivery(ctx, span, task, job, false)
	}

	if !failureNotice && job.Status == domain.JobStatusResultReady {
		if err := w.setStatus(ctx, job, domain.JobStatusDelivering, "", ""); err != nil {
			tracing.RecordError(span, err)
			return err
		}
	}

	if delivery.Status != domain.DeliveryStatusSent {
		if err := publisher.Publish(ctx, job, delivery); err != nil {
			tracing.RecordError(span, err)
			if errors.Is(err, domain.ErrInvalidResultContract) {
				return err
			}
			return w.handlePublishFailure(ctx, span, task, job, delivery, err)
		}
	}

	if failureNotice {
		metrics.DeliveriesSent.Inc()
		return nil
	}

	captureLatencyOrigin := earliestNonZeroTime(job.UpdatedAt, delivery.CreatedAt)
	capturePending := job.ChargeAmountCredits() > 0 && job.CostCaptured != job.ChargeAmountCredits()
	if err := w.captureReserved(ctx, job); err != nil {
		tracing.RecordError(span, err)
		return err
	}
	if capturePending {
		observeSuccessfulCaptureLatency(job, captureLatencyOrigin)
	}

	metrics.DeliveriesSent.Inc()
	metrics.JobsTerminal.WithLabelValues(string(domain.JobStatusSucceeded)).Inc()
	return w.setStatus(ctx, job, domain.JobStatusSucceeded, "", "")
}

func (w *DeliveryWorker) handlePublishFailure(
	ctx context.Context,
	span trace.Span,
	task queue.Task,
	job *domain.Job,
	delivery *domain.Delivery,
	publishErr error,
) error {
	delivery.ErrorCode = domain.JobErrMediaDeliveryFailed
	delivery.ErrorMessage = safeDeliveryFailureMessage()
	delivery.AttemptNo++
	if delivery.AttemptNo > w.maxAttempts {
		delivery.Status = domain.DeliveryStatusFailed
		if err := w.deliveries.Update(ctx, delivery); err != nil {
			tracing.RecordError(span, err)
			return fmt.Errorf("worker: persist exhausted delivery: %w", err)
		}
		return w.finalizeFailedDelivery(ctx, span, task, job, true)
	}
	delivery.Status = domain.DeliveryStatusRetrying
	_ = w.deliveries.Update(ctx, delivery)
	w.sleepBackoff(ctx, delivery.AttemptNo)
	return fmt.Errorf("worker: external publish: %w", publishErr)
}

// finalizeFailedDelivery resumes only the bookkeeping that follows a durable
// exhausted-delivery marker. DLQ routing belongs to the attempt that first
// persists that marker; retries after release/status failures must not publish
// either the original media or a duplicate DLQ entry.
func (w *DeliveryWorker) finalizeFailedDelivery(
	ctx context.Context,
	span trace.Span,
	task queue.Task,
	job *domain.Job,
	routeDLQ bool,
) error {
	if routeDLQ {
		metrics.DLQRouted.WithLabelValues("delivery").Inc()
		metrics.ObserveMediaDeliveryCaptureGap(deliveryOperationLabel(job), deliveryModalityLabel(job), "delivery_failed")
		if w.streams != nil {
			_ = w.streams.PublishTo(ctx, redisqueue.StreamDLQ, task)
		}
	}
	if job.Status == domain.JobStatusFailedTerminal {
		return nil
	}
	if err := w.releaseReserved(ctx, job); err != nil {
		tracing.RecordError(span, err)
		return err
	}
	if err := w.setStatus(ctx, job, domain.JobStatusFailedTerminal, domain.JobErrMediaDeliveryFailed, safeDeliveryFailureMessage()); err != nil {
		tracing.RecordError(span, err)
		return err
	}
	metrics.JobsTerminal.WithLabelValues(string(domain.JobStatusFailedTerminal)).Inc()
	return nil
}

func (w *DeliveryWorker) captureReserved(ctx context.Context, job *domain.Job) error {
	amount := job.ChargeAmountCredits()
	if amount <= 0 || job.CostCaptured == amount {
		return nil
	}
	if w.billing == nil {
		return errors.New("worker: billing is not configured")
	}
	captureCtx, captureSpan := tracing.Start(ctx, "billing.capture",
		attribute.String("job.id", job.ID.String()),
		attribute.Int64("billing.amount", amount),
		tracing.CorrelationAttr(job.CorrelationID),
	)
	defer captureSpan.End()
	if err := w.billing.CaptureForJob(captureCtx, job.ID, amount); err != nil {
		metrics.BillingCaptures.WithLabelValues(deliveryOperationLabel(job), "error").Inc()
		observeVideoRouteBillingForJob(job, "capture", "error")
		metrics.ObserveMediaDeliveryCaptureGap(deliveryOperationLabel(job), deliveryModalityLabel(job), "capture_failed")
		tracing.RecordError(captureSpan, err)
		return fmt.Errorf("worker: capture: %w", err)
	}
	metrics.BillingCaptures.WithLabelValues(deliveryOperationLabel(job), "success").Inc()
	observeVideoRouteBillingForJob(job, "capture", "success")
	metrics.AddProductCreditsFlow("job_delivery", "capture", "success", amount)
	if job.CostCaptured != amount {
		job.CostCaptured = amount
		if err := w.jobs.Update(ctx, job); err != nil {
			return err
		}
	}
	return nil
}

func observeSuccessfulCaptureLatency(job *domain.Job, origin time.Time) {
	if job == nil || origin.IsZero() {
		return
	}
	metrics.ObserveResultFinalizationCaptureDuration(string(job.ResultMode), time.Since(origin))
}

func earliestNonZeroTime(left, right time.Time) time.Time {
	if left.IsZero() {
		return right
	}
	if right.IsZero() || left.Before(right) {
		return left
	}
	return right
}

func completionReadinessFailureReason(err error) string {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return "incomplete"
	case errors.Is(err, domain.ErrInvalidResultContract):
		return "invalid_contract"
	default:
		return "dependency_error"
	}
}

func (w *DeliveryWorker) releaseReserved(ctx context.Context, job *domain.Job) error {
	if w.billing == nil || job.CostReserved <= 0 || job.CostCaptured > 0 {
		return nil
	}
	if err := w.billing.ReleaseForJob(ctx, job.ID); err != nil {
		metrics.BillingReleases.WithLabelValues(deliveryOperationLabel(job), "error").Inc()
		observeVideoRouteBillingForJob(job, "release", "error")
		return err
	}
	metrics.BillingReleases.WithLabelValues(deliveryOperationLabel(job), "success").Inc()
	observeVideoRouteBillingForJob(job, "release", "success")
	return nil
}

// ensureDelivery returns the job's external delivery row, creating it once
// through the selected channel publisher.
func (w *DeliveryWorker) ensureDelivery(ctx context.Context, job *domain.Job, publisher ExternalPublisher) (*domain.Delivery, error) {
	key := "delivery:" + job.ID.String()
	del, err := publisher.BuildDelivery(ctx, job, key)
	if err != nil {
		return nil, err
	}
	if del == nil {
		return nil, errors.New("worker: publisher returned nil delivery")
	}
	if del.ID != uuid.Nil {
		return del, nil
	}
	if err := w.deliveries.Create(ctx, del); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			replay, buildErr := publisher.BuildDelivery(ctx, job, key)
			if buildErr != nil {
				return nil, buildErr
			}
			if replay == nil || replay.ID == uuid.Nil {
				return nil, errors.New("worker: publisher did not resolve conflicting delivery replay")
			}
			return replay, nil
		}
		return nil, err
	}
	return del, nil
}

func isTerminalMediaFailure(job *domain.Job) bool {
	return job != nil &&
		job.Status == domain.JobStatusFailedTerminal &&
		(job.Modality == domain.ModalityImage || job.Modality == domain.ModalityVideo)
}

func safeDeliveryFailureMessage() string {
	return "media delivery failed; credits were not charged"
}

// sleepBackoff waits for the configured backoff before the next retry, honoring
// context cancellation.
func (w *DeliveryWorker) sleepBackoff(ctx context.Context, attempt int) {
	d := w.backoff(attempt)
	if d <= 0 {
		return
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func (w *DeliveryWorker) setStatus(ctx context.Context, job *domain.Job, to domain.JobStatus, code, msg string) error {
	if job.Status == to {
		return nil
	}
	from := job.Status
	if err := w.jobs.UpdateStatus(ctx, job.ID, job.Status, to, code, msg); err != nil {
		return err
	}
	job.Status = to
	metrics.JobStatusCurrent.WithLabelValues(string(from), deliveryOperationLabel(job), deliveryModalityLabel(job)).Dec()
	metrics.JobStatusCurrent.WithLabelValues(string(to), deliveryOperationLabel(job), deliveryModalityLabel(job)).Inc()
	if to.IsTerminal() && !job.CreatedAt.IsZero() {
		duration := time.Since(job.CreatedAt)
		if duration > 0 {
			metrics.JobDuration.WithLabelValues(deliveryOperationLabel(job), deliveryModalityLabel(job), string(to)).Observe(duration.Seconds())
		}
		observeVideoRouteTerminalForJob(job, to)
	}
	if to.IsTerminal() {
		metrics.ObserveProductEvent("worker", "job", "terminal", deliveryOperationLabel(job), deliveryModalityLabel(job), string(to))
	}
	return nil
}

func deliveryOperationLabel(job *domain.Job) string {
	if job == nil || job.OperationType == "" {
		return "unknown"
	}
	return deliveryMetricLabel(string(job.OperationType))
}

func deliveryModalityLabel(job *domain.Job) string {
	if job == nil || job.Modality == "" {
		return "unknown"
	}
	return deliveryMetricLabel(string(job.Modality))
}

func deliveryMetricLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-' || r == '.' || r == ':' || r == '/':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
		if b.Len() >= 96 {
			break
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "unknown"
	}
	return out
}
