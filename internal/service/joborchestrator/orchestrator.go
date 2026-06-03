// Package joborchestrator turns a normalized command into a billable, queued
// Job. It is the only place that ties together estimation, credit reservation,
// job persistence, the transactional outbox and task enqueueing. It never calls
// AI providers directly; that happens later in worker pools.
package joborchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/platform/uow"
)

// Biller is the subset of the billing service the orchestrator depends on.
type Biller interface {
	Estimate(op domain.OperationType) (int64, error)
	Reserve(ctx context.Context, userID, jobID uuid.UUID, amount int64) (*domain.CreditReservation, error)
	Release(ctx context.Context, reservationID uuid.UUID) error
}

// CreateJobInput is the normalized request to create a job from a command.
type CreateJobInput struct {
	// UserID is the owner of the job.
	UserID uuid.UUID
	// VKPeerID is the conversation the job belongs to.
	VKPeerID int64
	// CommandID is the command that produced the job.
	CommandID uuid.UUID
	// Operation is the operation to run.
	Operation domain.OperationType
	// Modality is the content kind of the operation.
	Modality domain.Modality
	// IdempotencyKey makes job creation safe to retry for the same request.
	IdempotencyKey string
	// CorrelationID links the job to its command and inbound event.
	CorrelationID string
	// InputArtifactIDs are artifacts fed into the job.
	InputArtifactIDs []uuid.UUID
	// Params holds normalized operation parameters.
	Params json.RawMessage
}

// Orchestrator implements the command -> estimate -> reserve -> job -> outbox
// -> queue flow.
type Orchestrator struct {
	jobs    domain.JobRepository
	uow     uow.Manager
	billing Biller
	queue   queue.Publisher
	now     func() time.Time
}

// New builds an Orchestrator. jobs is used for reads and out-of-transaction
// status updates; uow composes the job write with its outbox event atomically.
func New(jobs domain.JobRepository, manager uow.Manager, billing Biller, publisher queue.Publisher) *Orchestrator {
	return &Orchestrator{
		jobs:    jobs,
		uow:     manager,
		billing: billing,
		queue:   publisher,
		now:     time.Now,
	}
}

// CreateJob runs the full intake flow and returns the queued job. If a job with
// the same idempotency key already exists it is returned unchanged. When the
// user cannot afford the operation the job is parked in awaiting_payment and
// domain.ErrInsufficientCredits is returned alongside the job.
func (o *Orchestrator) CreateJob(ctx context.Context, in CreateJobInput) (*domain.Job, error) {
	if existing, err := o.jobs.GetByIdempotencyKey(ctx, in.IdempotencyKey); err == nil {
		return existing, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("joborchestrator: idempotency lookup: %w", err)
	}

	// 1. Estimate the cost of the operation.
	estimate, err := o.billing.Estimate(in.Operation)
	if err != nil {
		return nil, fmt.Errorf("joborchestrator: estimate: %w", err)
	}

	// 2. Persist the validated job together with its created event so the
	//    reservation (which references the job) has a row to point at.
	job := &domain.Job{
		ID:               uuid.New(),
		UserID:           in.UserID,
		VKPeerID:         in.VKPeerID,
		CommandID:        in.CommandID,
		OperationType:    in.Operation,
		Modality:         in.Modality,
		Status:           domain.JobStatusValidated,
		IdempotencyKey:   in.IdempotencyKey,
		CorrelationID:    in.CorrelationID,
		InputArtifactIDs: in.InputArtifactIDs,
		Params:           in.Params,
		CostEstimate:     estimate,
	}
	if err := o.uow.Within(ctx, func(ctx context.Context, repos uow.Repositories) error {
		if err := repos.Jobs.Create(ctx, job); err != nil {
			return err
		}
		return repos.Outbox.Add(ctx, jobEvent("event.job.created", job))
	}); err != nil {
		return nil, fmt.Errorf("joborchestrator: persist job: %w", err)
	}

	// 3. Reserve credits for the job.
	reservation, err := o.billing.Reserve(ctx, in.UserID, job.ID, estimate)
	if err != nil {
		if errors.Is(err, domain.ErrInsufficientCredits) {
			_ = o.jobs.UpdateStatus(ctx, job.ID, domain.JobStatusValidated, domain.JobStatusAwaitingPayment, "insufficient_credits", "not enough credits to reserve")
			job.Status = domain.JobStatusAwaitingPayment
			return job, domain.ErrInsufficientCredits
		}
		return nil, fmt.Errorf("joborchestrator: reserve: %w", err)
	}

	// 4. Mark the job reserved + queued and record the events atomically.
	if err := o.uow.Within(ctx, func(ctx context.Context, repos uow.Repositories) error {
		if err := repos.Jobs.UpdateStatus(ctx, job.ID, domain.JobStatusValidated, domain.JobStatusCreditsReserved, "", ""); err != nil {
			return err
		}
		job.CostReserved = estimate
		if err := repos.Jobs.Update(ctx, job); err != nil {
			return err
		}
		if err := repos.Jobs.UpdateStatus(ctx, job.ID, domain.JobStatusCreditsReserved, domain.JobStatusQueued, "", ""); err != nil {
			return err
		}
		return repos.Outbox.Add(ctx, jobEvent("event.job.queued", job))
	}); err != nil {
		// Compensate: the reservation succeeded but bookkeeping failed.
		_ = o.billing.Release(ctx, reservation.ID)
		return nil, fmt.Errorf("joborchestrator: queue job: %w", err)
	}
	job.Status = domain.JobStatusQueued

	// 5. Hand the job to the appropriate worker queue.
	if err := o.queue.Enqueue(ctx, queue.Task{
		JobID:         job.ID,
		Operation:     job.OperationType,
		Modality:      job.Modality,
		CorrelationID: job.CorrelationID,
	}); err != nil {
		return nil, fmt.Errorf("joborchestrator: enqueue: %w", err)
	}

	return job, nil
}

// jobEvent builds an outbox event describing a job state change.
func jobEvent(eventType string, job *domain.Job) *domain.OutboxEvent {
	payload, _ := json.Marshal(struct {
		JobID     uuid.UUID            `json:"job_id"`
		Status    domain.JobStatus     `json:"status"`
		Operation domain.OperationType `json:"operation"`
		UserID    uuid.UUID            `json:"user_id"`
	}{job.ID, job.Status, job.OperationType, job.UserID})

	return &domain.OutboxEvent{
		AggregateType: "job",
		AggregateID:   job.ID,
		EventType:     eventType,
		Payload:       payload,
	}
}
