// Package joborchestrator turns a normalized command into a billable, queued
// Job. It is the only place that ties together estimation, credit reservation,
// job persistence and the transactional outbox. It never calls AI providers
// directly; that happens later in worker pools. Enqueueing is not done here:
// the queued job is recorded as an outbox event and the outbox relay publishes
// it to the worker queue, so a process crash after commit cannot lose the task
// (audit A2).
package joborchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/tracing"
	"vk-ai-aggregator/internal/platform/uow"
)

// Biller is the subset of the billing service the orchestrator depends on. The
// reservation is performed with a transaction-bound repository so it commits
// atomically with job creation (audit B1).
type Biller interface {
	Estimate(op domain.OperationType) (int64, error)
	ReserveWith(ctx context.Context, repo domain.BillingRepository, userID, jobID uuid.UUID, amount int64) (*domain.CreditReservation, error)
}

// CreateJobInput is the normalized request to create a job from a command.
type CreateJobInput struct {
	// UserID is the owner of the job.
	UserID uuid.UUID
	// Source is the trusted product surface that requested the job.
	Source string
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
// flow. The job, its reservation and the outbox events all commit in one
// transaction.
type Orchestrator struct {
	jobs    domain.JobRepository
	uow     uow.Manager
	billing Biller
	maxCost int64
	now     func() time.Time
}

// New builds an Orchestrator. jobs is used for the idempotency read; uow
// composes the job write, its credit reservation and the outbox events
// atomically. maxCost (0 = unlimited) rejects jobs whose estimate exceeds the
// per-job spend cap (audit C1).
func New(jobs domain.JobRepository, manager uow.Manager, billing Biller, maxCost int64) *Orchestrator {
	return &Orchestrator{
		jobs:    jobs,
		uow:     manager,
		billing: billing,
		maxCost: maxCost,
		now:     time.Now,
	}
}

// CreateJob runs the full intake flow and returns the queued job. If a job with
// the same idempotency key already exists it is returned unchanged. When the
// user cannot afford the operation the job is parked in awaiting_payment and
// domain.ErrInsufficientCredits is returned alongside the job.
func (o *Orchestrator) CreateJob(ctx context.Context, in CreateJobInput) (*domain.Job, error) {
	ctx, span := tracing.Start(ctx, "job.create",
		attribute.String("operation", string(in.Operation)),
		attribute.String("modality", string(in.Modality)),
		tracing.CorrelationAttr(in.CorrelationID),
	)
	defer span.End()

	source := strings.TrimSpace(in.Source)
	if source == "" {
		source = "unknown"
	}

	if existing, err := o.jobs.GetByIdempotencyKey(ctx, in.IdempotencyKey); err == nil {
		span.SetAttributes(attribute.String("job.id", existing.ID.String()), attribute.Bool("job.idempotent", true))
		return existing, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		tracing.RecordError(span, err)
		return nil, fmt.Errorf("joborchestrator: idempotency lookup: %w", err)
	}

	// 1. Estimate the cost of the operation and enforce the spend cap.
	estimate, err := o.billing.Estimate(in.Operation)
	if err != nil {
		tracing.RecordError(span, err)
		return nil, fmt.Errorf("joborchestrator: estimate: %w", err)
	}
	if o.maxCost > 0 && estimate > o.maxCost {
		err := fmt.Errorf("joborchestrator: %w: estimate %d exceeds cap %d", domain.ErrCostCapExceeded, estimate, o.maxCost)
		tracing.RecordError(span, err)
		return nil, err
	}

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
	span.SetAttributes(attribute.String("job.id", job.ID.String()), attribute.Int64("job.cost_estimate", estimate))

	// 2. Persist the job, reserve its credits and record the created+queued
	//    events in a single transaction. Either everything commits or nothing
	//    does, so a reservation can never outlive a missing job and a queued job
	//    always has its enqueue event in the outbox (audit B1).
	var insufficient bool
	if err := o.uow.Within(ctx, func(ctx context.Context, repos uow.Repositories) error {
		if err := repos.Jobs.Create(ctx, job); err != nil {
			return err
		}
		if err := repos.Outbox.Add(ctx, jobEvent(ctx, "event.job.created", job)); err != nil {
			return err
		}

		if estimate == 0 {
			if err := repos.Jobs.UpdateStatus(ctx, job.ID, domain.JobStatusValidated, domain.JobStatusQueued, "", ""); err != nil {
				return err
			}
			queuedJob := *job
			queuedJob.Status = domain.JobStatusQueued
			return repos.Outbox.Add(ctx, jobEvent(ctx, "event.job.queued", &queuedJob))
		}

		if _, err := o.billing.ReserveWith(ctx, repos.Billing, in.UserID, job.ID, estimate); err != nil {
			if errors.Is(err, domain.ErrInsufficientCredits) {
				metrics.BillingReservations.WithLabelValues(string(in.Operation), "insufficient_credits").Inc()
				if err := repos.Jobs.UpdateStatus(ctx, job.ID, domain.JobStatusValidated, domain.JobStatusAwaitingPayment, "insufficient_credits", "not enough credits to reserve"); err != nil {
					return err
				}
				insufficient = true
				return nil
			}
			metrics.BillingReservations.WithLabelValues(string(in.Operation), "error").Inc()
			return err
		}
		metrics.BillingReservations.WithLabelValues(string(in.Operation), "success").Inc()

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
		queuedJob := *job
		queuedJob.Status = domain.JobStatusQueued
		return repos.Outbox.Add(ctx, jobEvent(ctx, "event.job.queued", &queuedJob))
	}); err != nil {
		tracing.RecordError(span, err)
		return nil, fmt.Errorf("joborchestrator: create job: %w", err)
	}

	if insufficient {
		job.Status = domain.JobStatusAwaitingPayment
		metrics.JobsCreated.WithLabelValues(source, string(job.OperationType), string(job.Modality)).Inc()
		metrics.JobStatusCurrent.WithLabelValues(string(job.Status), string(job.OperationType), string(job.Modality)).Inc()
		return job, domain.ErrInsufficientCredits
	}

	job.Status = domain.JobStatusQueued
	metrics.JobsCreated.WithLabelValues(source, string(job.OperationType), string(job.Modality)).Inc()
	metrics.JobStatusCurrent.WithLabelValues(string(job.Status), string(job.OperationType), string(job.Modality)).Inc()
	return job, nil
}

// jobEvent builds an outbox event describing a job state change. The queued
// event carries everything the outbox relay needs to reconstruct the worker
// task (operation, modality, correlation id).
func jobEvent(ctx context.Context, eventType string, job *domain.Job) *domain.OutboxEvent {
	payload, _ := json.Marshal(struct {
		JobID         uuid.UUID            `json:"job_id"`
		Status        domain.JobStatus     `json:"status"`
		Operation     domain.OperationType `json:"operation"`
		Modality      domain.Modality      `json:"modality"`
		UserID        uuid.UUID            `json:"user_id"`
		CorrelationID string               `json:"correlation_id,omitempty"`
		Traceparent   string               `json:"traceparent,omitempty"`
	}{job.ID, job.Status, job.OperationType, job.Modality, job.UserID, job.CorrelationID, tracing.Traceparent(ctx)})

	return &domain.OutboxEvent{
		AggregateType: "job",
		AggregateID:   job.ID,
		EventType:     eventType,
		Payload:       payload,
	}
}
