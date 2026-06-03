package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/queue"
)

// ObjectStore fetches stored artifact bytes (needed to deliver text results).
type ObjectStore interface {
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
}

// URLSigner issues time-limited download URLs for stored artifacts so media is
// delivered via signed URLs rather than exposing the raw storage location
// (audit ST1).
type URLSigner interface {
	PresignedGetURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
}

// DeliveryBiller captures a job's reserved credits once it is delivered.
type DeliveryBiller interface {
	CaptureForJob(ctx context.Context, jobID uuid.UUID, amount int64) error
}

// DeliveryDeps bundles the delivery worker's collaborators.
type DeliveryDeps struct {
	Jobs       domain.JobRepository
	Deliveries domain.DeliveryRepository
	Artifacts  domain.ArtifactRepository
	Objects    ObjectStore
	VK         vkdelivery.Client
	Billing    DeliveryBiller
	// Streams, when set, receives dead-lettered delivery tasks once the retry
	// budget is exhausted.
	Streams StreamPublisher
	// MaxAttempts caps delivery send attempts before dead-lettering (default 3).
	MaxAttempts int
	// Backoff returns the delay before the next delivery retry; defaults to none.
	Backoff func(attempt int) time.Duration
	// Signer issues signed media URLs when SignedURLs is enabled (audit ST1).
	Signer URLSigner
	// SignedURLs delivers media via time-limited signed URLs instead of raw
	// bucket/key references.
	SignedURLs bool
	// URLTTL is the validity window of signed media URLs (default 1h).
	URLTTL time.Duration
	Now    func() time.Time
}

// DeliveryWorker consumes the delivery stream and runs the final stage of the
// pipeline: Artifact -> Delivery -> Billing Capture -> Job Success. It is
// idempotent (one delivery row per job, deduped by key), uses a deterministic
// random_id so VK suppresses duplicate sends, and is safe to retry/recover.
type DeliveryWorker struct {
	jobs        domain.JobRepository
	deliveries  domain.DeliveryRepository
	artifacts   domain.ArtifactRepository
	objects     ObjectStore
	vk          vkdelivery.Client
	billing     DeliveryBiller
	streams     StreamPublisher
	maxAttempts int
	backoff     func(attempt int) time.Duration
	signer      URLSigner
	signURLs    bool
	urlTTL      time.Duration
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
	urlTTL := d.URLTTL
	if urlTTL <= 0 {
		urlTTL = time.Hour
	}
	return &DeliveryWorker{
		jobs:        d.Jobs,
		deliveries:  d.Deliveries,
		artifacts:   d.Artifacts,
		objects:     d.Objects,
		vk:          d.VK,
		billing:     d.Billing,
		streams:     d.Streams,
		maxAttempts: maxAttempts,
		backoff:     backoff,
		signer:      d.Signer,
		signURLs:    d.SignedURLs,
		urlTTL:      urlTTL,
		now:         now,
	}
}

// Process delivers one job's result. Returning nil acknowledges the task;
// returning an error leaves it pending for retry/recovery.
func (w *DeliveryWorker) Process(ctx context.Context, task queue.Task) error {
	job, err := w.jobs.GetByID(ctx, task.JobID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	switch job.Status {
	case domain.JobStatusSucceeded:
		return nil
	case domain.JobStatusResultReady, domain.JobStatusDelivering:
		// deliverable
	default:
		// Not ready to deliver yet (or in a failed/terminal state): ack and drop.
		return nil
	}

	if err := w.setStatus(ctx, job, domain.JobStatusDelivering, "", ""); err != nil {
		return err
	}

	del, err := w.ensureDelivery(ctx, job)
	if err != nil {
		return err
	}

	if del.Status != domain.DeliveryStatusSent {
		if err := w.send(ctx, del); err != nil {
			del.Status = domain.DeliveryStatusRetrying
			del.ErrorMessage = err.Error()
			del.AttemptNo++
			_ = w.deliveries.Update(ctx, del)
			// Retry budget: dead-letter once exhausted so a permanently failing
			// VK send can no longer be retried forever.
			if del.AttemptNo > w.maxAttempts {
				metrics.DLQRouted.WithLabelValues("delivery").Inc()
				if w.streams != nil {
					_ = w.streams.PublishTo(ctx, redisqueue.StreamDLQ, task)
				}
				metrics.JobsTerminal.WithLabelValues(string(domain.JobStatusFailedTerminal)).Inc()
				return w.setStatus(ctx, job, domain.JobStatusFailedTerminal, "delivery_failed", err.Error())
			}
			w.sleepBackoff(ctx, del.AttemptNo)
			return fmt.Errorf("worker: vk send: %w", err)
		}
	}

	// Billing capture: charge the reserved credits now that delivery succeeded.
	if job.CostReserved > 0 {
		if err := w.billing.CaptureForJob(ctx, job.ID, job.CostReserved); err != nil {
			return fmt.Errorf("worker: capture: %w", err)
		}
		if job.CostCaptured != job.CostReserved {
			job.CostCaptured = job.CostReserved
			if err := w.jobs.Update(ctx, job); err != nil {
				return err
			}
		}
	}

	metrics.DeliveriesSent.Inc()
	metrics.JobsTerminal.WithLabelValues(string(domain.JobStatusSucceeded)).Inc()
	return w.setStatus(ctx, job, domain.JobStatusSucceeded, "", "")
}

// ensureDelivery returns the job's delivery row, creating it on first run. The
// delivery is keyed by job so a retry reuses the same row and random_id.
func (w *DeliveryWorker) ensureDelivery(ctx context.Context, job *domain.Job) (*domain.Delivery, error) {
	key := "delivery:" + job.ID.String()
	if existing, err := w.deliveries.GetByIdempotencyKey(ctx, key); err == nil {
		return existing, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	del, err := w.buildDelivery(ctx, job, key)
	if err != nil {
		return nil, err
	}
	if err := w.deliveries.Create(ctx, del); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return w.deliveries.GetByIdempotencyKey(ctx, key)
		}
		return nil, err
	}
	return del, nil
}

// buildDelivery assembles a pending delivery from the job's output artifact.
func (w *DeliveryWorker) buildDelivery(ctx context.Context, job *domain.Job, key string) (*domain.Delivery, error) {
	del := &domain.Delivery{
		JobID:          job.ID,
		UserID:         job.UserID,
		VKPeerID:       job.VKPeerID,
		Type:           domain.DeliveryTypeMessage,
		Status:         domain.DeliveryStatusPending,
		VKRandomID:     vkdelivery.DeterministicRandomID(key),
		IdempotencyKey: key,
		AttemptNo:      1,
	}

	if len(job.OutputArtifactIDs) == 0 {
		del.Text = "(no result produced)"
		return del, nil
	}

	artID := job.OutputArtifactIDs[0]
	art, err := w.artifacts.GetByID(ctx, artID)
	if err != nil {
		return nil, err
	}
	del.ArtifactID = &artID

	switch art.MediaType {
	case domain.MediaTypeImage:
		del.Type = domain.DeliveryTypePhoto
		del.Attachment = w.mediaAttachment(ctx, art)
	case domain.MediaTypeVideo:
		del.Type = domain.DeliveryTypeVideo
		del.Attachment = w.mediaAttachment(ctx, art)
	default:
		del.Type = domain.DeliveryTypeMessage
		del.Text = w.textContent(ctx, art)
	}
	return del, nil
}

// textContent loads the stored text bytes for a text artifact, falling back to
// a placeholder when the bytes are unavailable.
func (w *DeliveryWorker) textContent(ctx context.Context, art *domain.Artifact) string {
	if w.objects == nil {
		return "(result ready)"
	}
	data, err := w.objects.GetObject(ctx, art.StorageBucket, art.StorageKey)
	if err != nil {
		return "(result ready)"
	}
	return string(data)
}

func (w *DeliveryWorker) send(ctx context.Context, del *domain.Delivery) error {
	var (
		res vkdelivery.SendResult
		err error
	)
	switch del.Type {
	case domain.DeliveryTypePhoto:
		res, err = w.vk.SendPhoto(ctx, del.VKPeerID, del.VKRandomID, del.Attachment, del.Text)
	case domain.DeliveryTypeVideo:
		res, err = w.vk.SendVideo(ctx, del.VKPeerID, del.VKRandomID, del.Attachment, del.Text)
	default:
		res, err = w.vk.SendText(ctx, del.VKPeerID, del.VKRandomID, del.Text)
	}
	if err != nil {
		return err
	}
	msgID := res.MessageID
	del.Status = domain.DeliveryStatusSent
	del.VKMessageID = &msgID
	del.ErrorCode = ""
	del.ErrorMessage = ""
	return w.deliveries.Update(ctx, del)
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
	if err := w.jobs.UpdateStatus(ctx, job.ID, job.Status, to, code, msg); err != nil {
		return err
	}
	job.Status = to
	return nil
}

// mediaAttachment resolves the attachment reference for a media artifact. When
// signed delivery is enabled it issues a time-limited signed URL so the raw
// storage location is never exposed (audit ST1); otherwise it falls back to the
// artifact's public URL or storage location.
func (w *DeliveryWorker) mediaAttachment(ctx context.Context, art *domain.Artifact) string {
	if w.signURLs && w.signer != nil && art.StorageKey != "" {
		if signed, err := w.signer.PresignedGetURL(ctx, art.StorageBucket, art.StorageKey, w.urlTTL); err == nil && signed != "" {
			return signed
		}
	}
	return attachmentRef(art)
}

// attachmentRef returns the VK attachment reference for a media artifact,
// preferring a public URL and falling back to the storage location.
func attachmentRef(art *domain.Artifact) string {
	if art.PublicURL != "" {
		return art.PublicURL
	}
	return art.StorageBucket + "/" + art.StorageKey
}
