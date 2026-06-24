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
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

// ErrBackendPriceRequired means a paid non-text job reached the orchestrator
// without a pricingcatalog snapshot or another backend-owned exact estimate.
var ErrBackendPriceRequired = errors.New("joborchestrator: backend price is required")

// Biller is the subset of the billing service the orchestrator depends on. The
// reservation is performed with a transaction-bound repository so it commits
// atomically with job creation (audit B1).
type Biller interface {
	Estimate(op domain.OperationType) (int64, error)
	ReserveWith(ctx context.Context, repo domain.BillingRepository, userID, jobID uuid.UUID, amount int64) (*domain.CreditReservation, error)
}

// CapacityCheckInput is the safe, product-level data a capacity guard may use
// before a job is persisted or credits are reserved.
type CapacityCheckInput struct {
	UserID    uuid.UUID
	Source    string
	Operation domain.OperationType
	Modality  domain.Modality
	Estimate  int64
}

// VideoRouteCheckInput is the bounded request shape route validators may use
// before a video job is persisted or credits are reserved.
type VideoRouteCheckInput struct {
	UserID           uuid.UUID
	Source           string
	Operation        domain.OperationType
	Modality         domain.Modality
	Params           json.RawMessage
	InputArtifactIDs []uuid.UUID
}

// VideoRouteResolution is the trusted server-side route decision applied before
// a job is persisted and credits are reserved.
type VideoRouteResolution struct {
	Resolved            bool
	Params              json.RawMessage
	Snapshot            domain.VideoRouteSnapshot
	InternalCostCredits int64
}

// CapacityGuard refuses new expensive work when shared product capacity is
// degraded. Implementations must not inspect prompts or raw provider payloads.
type CapacityGuard interface {
	CheckCapacity(ctx context.Context, in CapacityCheckInput) error
}

// VideoRouteValidator refuses unsupported/disabled video routes before billing
// reservation. Implementations must not call external providers.
type VideoRouteValidator interface {
	ValidateVideoRoute(ctx context.Context, in VideoRouteCheckInput) error
}

// VideoRouteResolver resolves route aliases into trusted job params and cost.
type VideoRouteResolver interface {
	ResolveVideoRoute(ctx context.Context, in VideoRouteCheckInput) (VideoRouteResolution, error)
}

// CapacityGuardFunc adapts a function into a CapacityGuard.
type CapacityGuardFunc func(context.Context, CapacityCheckInput) error

// CheckCapacity implements CapacityGuard.
func (f CapacityGuardFunc) CheckCapacity(ctx context.Context, in CapacityCheckInput) error {
	if f == nil {
		return nil
	}
	return f(ctx, in)
}

// VideoRouteValidatorFunc adapts a function into a VideoRouteValidator.
type VideoRouteValidatorFunc func(context.Context, VideoRouteCheckInput) error

// ValidateVideoRoute implements VideoRouteValidator.
func (f VideoRouteValidatorFunc) ValidateVideoRoute(ctx context.Context, in VideoRouteCheckInput) error {
	if f == nil {
		return nil
	}
	return f(ctx, in)
}

// VideoRouteResolverFunc adapts a function into a VideoRouteResolver.
type VideoRouteResolverFunc func(context.Context, VideoRouteCheckInput) (VideoRouteResolution, error)

// ResolveVideoRoute implements VideoRouteResolver.
func (f VideoRouteResolverFunc) ResolveVideoRoute(ctx context.Context, in VideoRouteCheckInput) (VideoRouteResolution, error) {
	if f == nil {
		return VideoRouteResolution{}, nil
	}
	return f(ctx, in)
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
	// CostEstimateCredits is a trusted backend-owned exact product price. It is
	// used by migrated consumers after their public product dimensions have been
	// resolved through pricingcatalog.
	CostEstimateCredits int64
	// PricingSnapshot is the immutable backend-owned pricingcatalog snapshot for
	// paid jobs. When present it is the source of reserved/captured amount.
	PricingSnapshot pricingcatalog.PricingSnapshot
}

// Orchestrator implements the command -> estimate -> reserve -> job -> outbox
// flow. The job, its reservation and the outbox events all commit in one
// transaction.
type Orchestrator struct {
	jobs                      domain.JobRepository
	uow                       uow.Manager
	billing                   Biller
	maxCost                   int64
	maxActiveVideoJobsPerUser int
	capacityGuard             CapacityGuard
	videoRouteValidator       VideoRouteValidator
	videoRouteResolver        VideoRouteResolver
	pricingCatalog            *pricingcatalog.Catalog
	now                       func() time.Time
}

// Option customizes orchestrator safety policy.
type Option func(*Orchestrator)

// WithMaxActiveVideoJobsPerUser rejects new video jobs before reservation when
// the same user already has this many active video jobs. A non-positive value
// disables the guard.
func WithMaxActiveVideoJobsPerUser(limit int) Option {
	return func(o *Orchestrator) {
		if limit > 0 {
			o.maxActiveVideoJobsPerUser = limit
		}
	}
}

// WithCapacityGuard installs a shared queue/capacity guard. It is checked after
// idempotency and cost estimate, but before job persistence and reservation.
func WithCapacityGuard(guard CapacityGuard) Option {
	return func(o *Orchestrator) {
		o.capacityGuard = guard
	}
}

// WithVideoRouteValidator installs the fail-closed video route policy guard.
func WithVideoRouteValidator(validator VideoRouteValidator) Option {
	return func(o *Orchestrator) {
		o.videoRouteValidator = validator
	}
}

// WithVideoRouteResolver installs the fail-closed video route resolver.
func WithVideoRouteResolver(resolver VideoRouteResolver) Option {
	return func(o *Orchestrator) {
		o.videoRouteResolver = resolver
	}
}

// WithPricingCatalog installs the backend-owned generation pricing catalog.
// Prompt 1 wires this shared dependency before later prompts migrate individual
// pricing consumers to it.
func WithPricingCatalog(catalog *pricingcatalog.Catalog) Option {
	return func(o *Orchestrator) {
		o.pricingCatalog = catalog
	}
}

// New builds an Orchestrator. jobs is used for the idempotency read; uow
// composes the job write, its credit reservation and the outbox events
// atomically. maxCost (0 = unlimited) rejects jobs whose estimate exceeds the
// per-job spend cap (audit C1).
func New(jobs domain.JobRepository, manager uow.Manager, billing Biller, maxCost int64, opts ...Option) *Orchestrator {
	o := &Orchestrator{
		jobs:    jobs,
		uow:     manager,
		billing: billing,
		maxCost: maxCost,
		now:     time.Now,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}
	return o
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
	operationLabel := string(in.Operation)
	modalityLabel := string(in.Modality)

	if existing, err := o.jobs.GetByIdempotencyKey(ctx, in.IdempotencyKey); err == nil {
		span.SetAttributes(attribute.String("job.id", existing.ID.String()), attribute.Bool("job.idempotent", true))
		metrics.ObserveProductEvent(source, "job", "create", operationLabel, modalityLabel, "idempotent")
		return existing, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		tracing.RecordError(span, err)
		metrics.ObserveProductEvent(source, "job", "create", operationLabel, modalityLabel, "idempotency_error")
		return nil, fmt.Errorf("joborchestrator: idempotency lookup: %w", err)
	}

	// 1. Resolve trusted route details, estimate cost and enforce spend caps.
	var estimate int64
	routeResolution, err := o.resolveVideoRoute(ctx, in, source)
	if err != nil {
		tracing.RecordError(span, err)
		metrics.ObserveProductEvent(source, "job", "create", operationLabel, modalityLabel, "rejected_video_route")
		return nil, err
	}
	if routeResolution.Resolved {
		in.Params = append(json.RawMessage(nil), routeResolution.Params...)
	}
	pricingSnapshot := in.PricingSnapshot
	var pricingSnapshotRaw json.RawMessage
	if pricingSnapshot.Valid() {
		estimate = pricingSnapshot.InternalCredits
		if in.CostEstimateCredits > 0 && in.CostEstimateCredits != estimate {
			err := fmt.Errorf("joborchestrator: pricing snapshot cost %d does not match estimate %d", estimate, in.CostEstimateCredits)
			tracing.RecordError(span, err)
			metrics.ObserveProductEvent(source, "job", "estimate", operationLabel, modalityLabel, "pricing_snapshot_mismatch")
			return nil, err
		}
		pricingSnapshotRaw, err = json.Marshal(pricingSnapshot)
		if err != nil {
			tracing.RecordError(span, err)
			metrics.ObserveProductEvent(source, "job", "estimate", operationLabel, modalityLabel, "pricing_snapshot_error")
			return nil, fmt.Errorf("joborchestrator: pricing snapshot: %w", err)
		}
	} else if in.CostEstimateCredits > 0 {
		estimate = in.CostEstimateCredits
	}
	routeSnapshot := routeResolution.Snapshot
	if estimate == 0 {
		if requiresBackendPrice(in.Operation, in.Modality) {
			err := fmt.Errorf("%w: missing price for %s/%s", ErrBackendPriceRequired, in.Operation, in.Modality)
			tracing.RecordError(span, err)
			metrics.ObserveProductEvent(source, "job", "estimate", operationLabel, modalityLabel, "price_required")
			return nil, err
		}
		// Legacy non-catalog fallback: image/video generation must already carry
		// a backend-owned exact estimate or pricing snapshot.
		estimate, err = o.billing.Estimate(in.Operation)
		if err != nil {
			tracing.RecordError(span, err)
			metrics.ObserveProductEvent(source, "job", "estimate", operationLabel, modalityLabel, "error")
			return nil, fmt.Errorf("joborchestrator: estimate: %w", err)
		}
	}
	if o.maxCost > 0 && estimate > o.maxCost {
		err := fmt.Errorf("joborchestrator: %w: estimate %d exceeds cap %d", domain.ErrCostCapExceeded, estimate, o.maxCost)
		tracing.RecordError(span, err)
		metrics.ObserveProductEvent(source, "job", "create", operationLabel, modalityLabel, "rejected_cost_cap")
		return nil, err
	}
	if err := o.checkCapacity(ctx, in, source, estimate); err != nil {
		tracing.RecordError(span, err)
		metrics.ObserveProductEvent(source, "job", "create", operationLabel, modalityLabel, "rejected_capacity")
		return nil, err
	}

	job := &domain.Job{
		ID:               uuid.New(),
		UserID:           in.UserID,
		Source:           source,
		VKPeerID:         in.VKPeerID,
		CommandID:        in.CommandID,
		OperationType:    in.Operation,
		Modality:         in.Modality,
		Status:           domain.JobStatusValidated,
		IdempotencyKey:   in.IdempotencyKey,
		CorrelationID:    in.CorrelationID,
		InputArtifactIDs: in.InputArtifactIDs,
		Params:           in.Params,
		PricingSnapshot:  pricingSnapshotRaw,
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
				if routeSnapshot.Valid() {
					metrics.ObserveVideoRouteBilling(string(routeSnapshot.Provider), string(routeSnapshot.Alias), "reserve", "insufficient_credits")
				}
				if err := repos.Jobs.UpdateStatus(ctx, job.ID, domain.JobStatusValidated, domain.JobStatusAwaitingPayment, "insufficient_credits", "not enough credits to reserve"); err != nil {
					return err
				}
				insufficient = true
				return nil
			}
			metrics.BillingReservations.WithLabelValues(string(in.Operation), "error").Inc()
			if routeSnapshot.Valid() {
				metrics.ObserveVideoRouteBilling(string(routeSnapshot.Provider), string(routeSnapshot.Alias), "reserve", "error")
			}
			return err
		}
		metrics.BillingReservations.WithLabelValues(string(in.Operation), "success").Inc()
		if routeSnapshot.Valid() {
			metrics.ObserveVideoRouteBilling(string(routeSnapshot.Provider), string(routeSnapshot.Alias), "reserve", "success")
		}

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
		metrics.ObserveProductEvent(source, "job", "create", operationLabel, modalityLabel, "error")
		return nil, fmt.Errorf("joborchestrator: create job: %w", err)
	}

	if insufficient {
		job.Status = domain.JobStatusAwaitingPayment
		metrics.JobsCreated.WithLabelValues(source, string(job.OperationType), string(job.Modality)).Inc()
		metrics.JobStatusCurrent.WithLabelValues(string(job.Status), string(job.OperationType), string(job.Modality)).Inc()
		metrics.ObserveProductEvent(source, "job", "create", operationLabel, modalityLabel, "awaiting_payment")
		metrics.ObserveProductActiveUserEvent(source, operationLabel, modalityLabel, "created")
		return job, domain.ErrInsufficientCredits
	}

	job.Status = domain.JobStatusQueued
	metrics.JobsCreated.WithLabelValues(source, string(job.OperationType), string(job.Modality)).Inc()
	metrics.JobStatusCurrent.WithLabelValues(string(job.Status), string(job.OperationType), string(job.Modality)).Inc()
	metrics.ObserveProductEvent(source, "job", "create", operationLabel, modalityLabel, "queued")
	metrics.ObserveProductActiveUserEvent(source, operationLabel, modalityLabel, "created")
	return job, nil
}

func requiresBackendPrice(op domain.OperationType, modality domain.Modality) bool {
	if modality == domain.ModalityVideo || modality == domain.ModalityImage {
		return true
	}
	switch op {
	case domain.OperationImageGenerate, domain.OperationImageEdit, domain.OperationVideoGenerate, domain.OperationVideoImageToVideo, domain.OperationVideoExtend:
		return true
	default:
		return false
	}
}

func (o *Orchestrator) resolveVideoRoute(ctx context.Context, in CreateJobInput, source string) (VideoRouteResolution, error) {
	check := VideoRouteCheckInput{
		UserID:           in.UserID,
		Source:           source,
		Operation:        in.Operation,
		Modality:         in.Modality,
		Params:           in.Params,
		InputArtifactIDs: append([]uuid.UUID(nil), in.InputArtifactIDs...),
	}
	if o.videoRouteResolver != nil {
		resolved, err := o.videoRouteResolver.ResolveVideoRoute(ctx, check)
		if err != nil {
			return VideoRouteResolution{}, fmt.Errorf("joborchestrator: %w", err)
		}
		return resolved, nil
	}
	if o.videoRouteValidator != nil {
		if err := o.videoRouteValidator.ValidateVideoRoute(ctx, check); err != nil {
			return VideoRouteResolution{}, fmt.Errorf("joborchestrator: %w", err)
		}
	}
	return VideoRouteResolution{}, nil
}

func (o *Orchestrator) checkCapacity(ctx context.Context, in CreateJobInput, source string, estimate int64) error {
	if o.maxActiveVideoJobsPerUser > 0 && in.Operation == domain.OperationVideoGenerate {
		active, err := o.jobs.CountActiveByUserOperation(ctx, in.UserID, domain.OperationVideoGenerate)
		if err != nil {
			return fmt.Errorf("joborchestrator: active video jobs: %w", err)
		}
		if active >= o.maxActiveVideoJobsPerUser {
			return fmt.Errorf("joborchestrator: %w", domain.ErrActiveJobLimitExceeded)
		}
	}
	if o.capacityGuard == nil {
		return nil
	}
	if err := o.capacityGuard.CheckCapacity(ctx, CapacityCheckInput{
		UserID:    in.UserID,
		Source:    source,
		Operation: in.Operation,
		Modality:  in.Modality,
		Estimate:  estimate,
	}); err != nil {
		return fmt.Errorf("joborchestrator: %w", err)
	}
	return nil
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
