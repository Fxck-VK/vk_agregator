package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/queue"
)

// ObjectStore fetches stored artifact bytes (needed to deliver text results).
type ObjectStore interface {
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
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
	Now        func() time.Time
}

// DeliveryWorker consumes the delivery stream and runs the final stage of the
// pipeline: Artifact -> Delivery -> Billing Capture -> Job Success. It is
// idempotent (one delivery row per job, deduped by key), uses a deterministic
// random_id so VK suppresses duplicate sends, and is safe to retry/recover.
type DeliveryWorker struct {
	jobs       domain.JobRepository
	deliveries domain.DeliveryRepository
	artifacts  domain.ArtifactRepository
	objects    ObjectStore
	vk         vkdelivery.Client
	billing    DeliveryBiller
	now        func() time.Time
}

// NewDeliveryWorker builds a DeliveryWorker.
func NewDeliveryWorker(d DeliveryDeps) *DeliveryWorker {
	now := d.Now
	if now == nil {
		now = time.Now
	}
	return &DeliveryWorker{
		jobs:       d.Jobs,
		deliveries: d.Deliveries,
		artifacts:  d.Artifacts,
		objects:    d.Objects,
		vk:         d.VK,
		billing:    d.Billing,
		now:        now,
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
			_ = w.deliveries.Update(ctx, del)
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
		del.Attachment = attachmentRef(art)
	case domain.MediaTypeVideo:
		del.Type = domain.DeliveryTypeVideo
		del.Attachment = attachmentRef(art)
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

// attachmentRef returns the VK attachment reference for a media artifact,
// preferring a public URL and falling back to the storage location.
func attachmentRef(art *domain.Artifact) string {
	if art.PublicURL != "" {
		return art.PublicURL
	}
	return art.StorageBucket + "/" + art.StorageKey
}
