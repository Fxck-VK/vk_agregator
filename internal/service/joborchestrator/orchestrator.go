// Package joborchestrator turns a normalized command into a billable, queued
// Job. It is the only place that ties together estimation, credit reservation,
// job persistence and the transactional outbox. It never calls AI providers
// directly; that happens later in worker pools. Enqueueing is not done here:
// the queued job is recorded as an outbox event and the outbox relay publishes
// it to the worker queue, so a process crash after commit cannot lose the task
// (audit A2).
package joborchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/tracing"
	"vk-ai-aggregator/internal/platform/uow"
	"vk-ai-aggregator/internal/service/outboxrelay"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

// ErrBackendPriceRequired means a paid non-text job reached the orchestrator
// without a pricingcatalog snapshot or another backend-owned exact estimate.
var ErrBackendPriceRequired = errors.New("joborchestrator: backend price is required")

// ErrInvalidInputArtifact means a job referenced media that is not a ready,
// owner-scoped, storage-backed input image.
var ErrInvalidInputArtifact = errors.New("joborchestrator: invalid input artifact")

// ErrInvalidConversationTitleRequest means an internal title request was
// attached to work that is not an account-owned Web text generation job.
var ErrInvalidConversationTitleRequest = errors.New("joborchestrator: invalid conversation title request")

// Biller is the subset of the billing service the orchestrator depends on. The
// reservation is performed with a transaction-bound repository so it commits
// atomically with job creation (audit B1).
type Biller interface {
	Estimate(op domain.OperationType) (int64, error)
	ReserveWith(ctx context.Context, repo domain.BillingRepository, userID, jobID uuid.UUID, amount int64) (*domain.CreditReservation, error)
	ReserveWithOwner(ctx context.Context, repo domain.BillingRepository, userID, accountID, jobID uuid.UUID, amount int64) (*domain.CreditReservation, error)
	ReserveForAccountWith(ctx context.Context, repo domain.BillingRepository, accountID, jobID uuid.UUID, amount int64) (*domain.CreditReservation, error)
}

// CapacityCheckInput is the safe, product-level data a capacity guard may use
// before a job is persisted or credits are reserved.
type CapacityCheckInput struct {
	UserID    uuid.UUID
	AccountID uuid.UUID
	Source    string
	Operation domain.OperationType
	Modality  domain.Modality
	Estimate  int64
}

// VideoRouteCheckInput is the bounded request shape route validators may use
// before a video job is persisted or credits are reserved.
type VideoRouteCheckInput struct {
	AccountID        uuid.UUID
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
	// UserID is the legacy channel user that requested the job.
	UserID uuid.UUID
	// AccountID is the canonical owner of billing, history and artifacts. When
	// empty, legacy UserID is used as a compatibility fallback.
	AccountID uuid.UUID
	// Source is the trusted product surface that requested the job.
	Source string
	// ChannelContext is trusted adapter-owned origin provenance. New writes must
	// provide it together with an explicit result contract; old callers without
	// all contract fields remain legacy_unknown during the migration.
	ChannelContext *domain.ChannelContext
	// ResultMode controls finalization for this job. It is never client input.
	ResultMode domain.ResultMode
	// DeliveryTarget is required for external push and forbidden for account
	// history. It is adapter-owned routing data, not authorization input.
	DeliveryTarget *domain.DeliveryTarget
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
	// ConversationTitleRequested is internal orchestration metadata. It is never
	// client input and only enables a separate, best-effort title outbox event
	// for one account-owned Web text job.
	ConversationTitleRequested bool
}

// PrepareAccountJobInput is a trusted, normalized account-native request. It
// records a non-executable prepared job only; activation, reservation and
// queueing require a later delivery-neutral flow.
type PrepareAccountJobInput struct {
	AccountID           uuid.UUID
	Operation           domain.OperationType
	Modality            domain.Modality
	IdempotencyKey      string
	CorrelationID       string
	InputArtifactIDs    []uuid.UUID
	Params              json.RawMessage
	CostEstimateCredits int64
	PricingSnapshot     pricingcatalog.PricingSnapshot
}

const (
	maxPreparedIdempotencyKeyLength = 256
)

// Orchestrator implements the command -> estimate -> reserve -> job -> outbox
// flow. The job, its reservation and the outbox events all commit in one
// transaction.
type Orchestrator struct {
	jobs                      domain.JobRepository
	uow                       uow.Manager
	billing                   Biller
	maxCost                   int64
	maxActiveVideoJobsPerUser int
	maxPreparedWebImageJobs   int
	preparedWebImageTTL       time.Duration
	capacityGuard             CapacityGuard
	videoRouteValidator       VideoRouteValidator
	videoRouteResolver        VideoRouteResolver
	pricingCatalog            *pricingcatalog.Catalog
	artifacts                 domain.ArtifactRepository
	now                       func() time.Time
}

// memoryActivationLocker is deliberately not a domain repository contract:
// durable storage relies solely on its transaction-owned account row lock.
// The in-memory adapter has no transactions, so it exposes this local guard to
// preserve deterministic test and development semantics.
type memoryActivationLocker interface {
	LockAccountActivation(accountID uuid.UUID) func()
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

// WithMaxPreparedWebImageJobsPerAccount bounds account-owned image
// confirmations before they reserve credits or become worker work. Both values
// must be positive to enable the guard; an expiry prevents abandoned durable
// preparation rows from permanently consuming the small account allowance.
func WithMaxPreparedWebImageJobsPerAccount(limit int, confirmationTTL time.Duration) Option {
	return func(o *Orchestrator) {
		if limit > 0 && confirmationTTL > 0 {
			o.maxPreparedWebImageJobs = limit
			o.preparedWebImageTTL = confirmationTTL
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

// WithArtifactRepository installs the shared input-artifact validator backend.
func WithArtifactRepository(repo domain.ArtifactRepository) Option {
	return func(o *Orchestrator) {
		o.artifacts = repo
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
	ownerID := ownerAccountID(in.UserID, in.AccountID)
	channelContext, resultMode, deliveryTarget, err := createResultContract(in, ownerID)
	if err != nil {
		tracing.RecordError(span, err)
		metrics.ObserveProductEvent(source, "job", "create", string(in.Operation), string(in.Modality), "rejected_result_contract")
		return nil, err
	}
	if err := validateConversationTitleRequest(in, source, channelContext, resultMode, deliveryTarget); err != nil {
		tracing.RecordError(span, err)
		metrics.ObserveProductEvent(source, "job", "create", string(in.Operation), string(in.Modality), "rejected_conversation_title")
		return nil, err
	}
	operationLabel := string(in.Operation)
	modalityLabel := string(in.Modality)

	if existing, err := o.jobs.GetByIdempotencyKey(ctx, in.IdempotencyKey); err == nil {
		// Prepared account-native jobs deliberately share the durable unique key
		// column but are not executable legacy requests. Never expose such a row
		// through the legacy/VK/Mini App create path on a cross-surface key
		// collision.
		if existing.Status == domain.JobStatusPrepared {
			return nil, domain.ErrConflict
		}
		span.SetAttributes(attribute.String("job.id", existing.ID.String()), attribute.Bool("job.idempotent", true))
		metrics.ObserveProductEvent(source, "job", "create", operationLabel, modalityLabel, "idempotent")
		return existing, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		tracing.RecordError(span, err)
		metrics.ObserveProductEvent(source, "job", "create", operationLabel, modalityLabel, "idempotency_error")
		return nil, fmt.Errorf("joborchestrator: idempotency lookup: %w", err)
	}

	if err := o.validateInputArtifacts(ctx, in); err != nil {
		tracing.RecordError(span, err)
		metrics.ObserveProductEvent(source, "job", "create", operationLabel, modalityLabel, "rejected_input_artifact")
		return nil, err
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
		AccountID:        ownerID,
		Source:           source,
		ChannelContext:   channelContext,
		ResultMode:       resultMode,
		DeliveryTarget:   deliveryTarget,
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
			return addQueuedOutboxEvents(ctx, repos.Outbox, &queuedJob, in.ConversationTitleRequested)
		}

		if _, err := o.billing.ReserveWithOwner(ctx, repos.Billing, in.UserID, ownerID, job.ID, estimate); err != nil {
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
		return addQueuedOutboxEvents(ctx, repos.Outbox, &queuedJob, in.ConversationTitleRequested)
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

func validateConversationTitleRequest(
	in CreateJobInput,
	source string,
	channelContext *domain.ChannelContext,
	resultMode domain.ResultMode,
	deliveryTarget *domain.DeliveryTarget,
) error {
	if !in.ConversationTitleRequested {
		return nil
	}
	if in.AccountID == uuid.Nil || source != "web" ||
		in.Operation != domain.OperationTextGenerate || in.Modality != domain.ModalityText ||
		channelContext == nil || channelContext.Channel != domain.ChannelWeb ||
		resultMode != domain.ResultModeAccountHistory || deliveryTarget != nil {
		return ErrInvalidConversationTitleRequest
	}
	return nil
}

func addQueuedOutboxEvents(ctx context.Context, outbox domain.OutboxRepository, job *domain.Job, conversationTitleRequested bool) error {
	if err := outbox.Add(ctx, jobEvent(ctx, "event.job.queued", job)); err != nil {
		return err
	}
	if !conversationTitleRequested {
		return nil
	}
	return outbox.Add(ctx, conversationTitleEvent(ctx, job))
}

// PrepareAccountJob persists a safe account-owned record without reserving
// credits or creating a queue event. Its result contract is explicitly Web /
// account-history, so future activation never needs to invent VK provenance.
// Current workers still require their legacy delivery path, so this method
// remains separate from CreateJob until neutral finalization is deployed.
func (o *Orchestrator) PrepareAccountJob(ctx context.Context, in PrepareAccountJobInput) (*domain.Job, error) {
	if in.AccountID == uuid.Nil {
		return nil, fmt.Errorf("joborchestrator: account id is required")
	}
	if !in.Operation.Valid() || !in.Modality.Valid() {
		return nil, fmt.Errorf("joborchestrator: invalid operation or modality")
	}
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	if in.IdempotencyKey == "" || len(in.IdempotencyKey) > maxPreparedIdempotencyKeyLength {
		return nil, fmt.Errorf("joborchestrator: invalid idempotency key")
	}
	params, err := normalizePreparedParams(in.Params)
	if err != nil {
		return nil, err
	}
	in.Params = params
	in.CorrelationID = strings.TrimSpace(in.CorrelationID)
	estimate, pricingSnapshot, err := preparePrice(in)
	if err != nil {
		return nil, err
	}
	if o.maxCost > 0 && estimate > o.maxCost {
		return nil, fmt.Errorf("joborchestrator: %w: estimate %d exceeds cap %d", domain.ErrCostCapExceeded, estimate, o.maxCost)
	}
	prepareExpiry := o.preparedWebImageExpiry(in.Operation, in.Modality)
	if o.maxPreparedWebImageJobs > 0 && prepareExpiry != nil {
		if locker, ok := o.jobs.(memoryActivationLocker); ok {
			unlock := locker.LockAccountActivation(in.AccountID)
			defer unlock()
		}
	}

	if existing, err := o.jobs.GetByIdempotencyKeyForAccount(ctx, in.AccountID, in.IdempotencyKey); err == nil {
		if preparedJobMatches(existing, in, pricingSnapshot, estimate) {
			return existing, nil
		}
		return nil, domain.ErrConflict
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("joborchestrator: account idempotency lookup: %w", err)
	}
	if err := o.validatePreparedInputArtifacts(ctx, in.AccountID, in.InputArtifactIDs); err != nil {
		return nil, err
	}

	job := &domain.Job{
		ID:               uuid.New(),
		AccountID:        in.AccountID,
		UserID:           uuid.Nil,
		Source:           "web",
		ChannelContext:   &domain.ChannelContext{Channel: domain.ChannelWeb},
		ResultMode:       domain.ResultModeAccountHistory,
		VKPeerID:         0,
		CommandID:        uuid.Nil,
		OperationType:    in.Operation,
		Modality:         in.Modality,
		Status:           domain.JobStatusPrepared,
		IdempotencyKey:   in.IdempotencyKey,
		CorrelationID:    in.CorrelationID,
		InputArtifactIDs: append([]uuid.UUID(nil), in.InputArtifactIDs...),
		Params:           append(json.RawMessage(nil), params...),
		PricingSnapshot:  pricingSnapshot,
		CostEstimate:     estimate,
		ExpiresAt:        prepareExpiry,
	}
	var replayed *domain.Job
	if err := o.uow.Within(ctx, func(ctx context.Context, repos uow.Repositories) error {
		if o.maxPreparedWebImageJobs > 0 && prepareExpiry != nil {
			// A transaction-owned account lock serializes the count/check/create
			// sequence across API instances. The in-memory adapter is protected by
			// the account-keyed lock above because it has no database transaction.
			if err := repos.Jobs.LockAccountForCapacity(ctx, in.AccountID); err != nil {
				return fmt.Errorf("joborchestrator: lock account preparation: %w", err)
			}
			if existing, err := repos.Jobs.GetByIdempotencyKeyForAccount(ctx, in.AccountID, in.IdempotencyKey); err == nil {
				if preparedJobMatches(existing, in, pricingSnapshot, estimate) {
					replayed = existing
					return nil
				}
				return domain.ErrConflict
			} else if !errors.Is(err, domain.ErrNotFound) {
				return fmt.Errorf("joborchestrator: account idempotency lookup in preparation: %w", err)
			}
			prepared, err := repos.Jobs.CountUnexpiredPreparedByAccountOperation(ctx, in.AccountID, "web", in.Operation, in.Modality, o.now())
			if err != nil {
				return fmt.Errorf("joborchestrator: prepared image jobs: %w", err)
			}
			if prepared >= o.maxPreparedWebImageJobs {
				return domain.ErrPreparedJobLimitExceeded
			}
		}
		if err := repos.Jobs.Create(ctx, job); err != nil {
			return err
		}
		return repos.Outbox.Add(ctx, jobEvent(ctx, "event.job.created", job))
	}); err != nil {
		// PostgreSQL unique violations abort the transaction, so the global lookup
		// happens only after it has closed. It is used solely to distinguish a
		// same-owner concurrent replay from a conflict and never leaks a foreign row.
		if !errors.Is(err, domain.ErrConflict) {
			return nil, fmt.Errorf("joborchestrator: prepare account job: %w", err)
		}
		existing, lookupErr := o.jobs.GetByIdempotencyKey(ctx, in.IdempotencyKey)
		if lookupErr != nil || existing.AccountID != in.AccountID || !preparedJobMatches(existing, in, pricingSnapshot, estimate) {
			return nil, domain.ErrConflict
		}
		return existing, nil
	}
	if replayed != nil {
		return replayed, nil
	}

	metrics.JobsCreated.WithLabelValues("web", string(job.OperationType), string(job.Modality)).Inc()
	metrics.JobStatusCurrent.WithLabelValues(string(job.Status), string(job.OperationType), string(job.Modality)).Inc()
	metrics.ObserveProductEvent("web", "job", "prepare", string(job.OperationType), string(job.Modality), "prepared")
	return job, nil
}

func (o *Orchestrator) preparedWebImageExpiry(operation domain.OperationType, modality domain.Modality) *time.Time {
	if o.maxPreparedWebImageJobs <= 0 || o.preparedWebImageTTL <= 0 ||
		operation != domain.OperationImageGenerate || modality != domain.ModalityImage {
		return nil
	}
	expiresAt := o.now().Add(o.preparedWebImageTTL)
	return &expiresAt
}

// ActivatePreparedAccountJob is the only account-native path that turns a
// prepared web job into worker-visible work. The caller supplies ownership and
// job identity only; operation, price and channel contract are immutable facts
// loaded from the persisted job. Reservation, state transition and queue outbox
// event are committed through one UOW.
func (o *Orchestrator) ActivatePreparedAccountJob(ctx context.Context, accountID, jobID uuid.UUID) (*domain.Job, error) {
	if accountID == uuid.Nil || jobID == uuid.Nil {
		return nil, domain.ErrNotFound
	}
	if locker, ok := o.jobs.(memoryActivationLocker); ok {
		unlock := locker.LockAccountActivation(accountID)
		defer unlock()
	}

	var (
		activated           *domain.Job
		insufficient        bool
		confirmationExpired bool
	)
	err := o.uow.Within(ctx, func(ctx context.Context, repos uow.Repositories) error {
		// PostgreSQL locks the exact canonical account row before the job read.
		// A concurrent activation that waited here therefore reloads the current
		// job state and returns a stable queued/downstream replay instead of
		// applying capacity checks to stale prepared data.
		if err := repos.Jobs.LockAccountForCapacity(ctx, accountID); err != nil {
			return fmt.Errorf("joborchestrator: lock account activation: %w", err)
		}
		job, err := repos.Jobs.GetByIDForAccount(ctx, accountID, jobID)
		if err != nil {
			return err
		}
		if job.Status == domain.JobStatusPrepared && job.ExpiresAt != nil && !job.ExpiresAt.After(o.now()) {
			if err := repos.Jobs.UpdateStatus(ctx, job.ID, domain.JobStatusPrepared, domain.JobStatusExpired, domain.PreparedConfirmationExpiredCode, domain.PreparedConfirmationExpiredMessage); err != nil {
				return err
			}
			// Return success from the UOW so PostgreSQL commits the transition.
			// The caller receives the confirmation conflict only after that commit.
			confirmationExpired = true
			return nil
		}
		if activationReplayStatus(job.Status) {
			activated = job
			return nil
		}
		if job.Status != domain.JobStatusPrepared && job.Status != domain.JobStatusAwaitingPayment {
			return domain.ErrConflict
		}
		if err := validatePreparedActivation(job, accountID); err != nil {
			return err
		}
		amount, err := preparedActivationAmount(job)
		if err != nil {
			return err
		}
		if err := o.checkStoredJobCapacity(ctx, repos.Jobs, job, amount); err != nil {
			return err
		}

		from := job.Status
		if from == domain.JobStatusPrepared {
			if err := repos.Jobs.UpdateStatus(ctx, job.ID, domain.JobStatusPrepared, domain.JobStatusValidated, "", ""); err != nil {
				return err
			}
			from = domain.JobStatusValidated
			job.Status = domain.JobStatusValidated
			// The confirmation deadline protects only an unconfirmed prepared
			// record. Once activation begins, preserve the durable job for the
			// normal billing/retry lifecycle instead of letting the old deadline
			// affect a queued or awaiting-payment job later.
			job.ExpiresAt = nil
			if err := repos.Jobs.Update(ctx, job); err != nil {
				return err
			}
		}

		if amount > 0 {
			if _, err := o.billing.ReserveForAccountWith(ctx, repos.Billing, accountID, job.ID, amount); err != nil {
				if !errors.Is(err, domain.ErrInsufficientCredits) {
					return err
				}
				if from == domain.JobStatusValidated {
					if err := repos.Jobs.UpdateStatus(ctx, job.ID, domain.JobStatusValidated, domain.JobStatusAwaitingPayment, "insufficient_credits", "not enough credits to reserve"); err != nil {
						return err
					}
				}
				job.Status = domain.JobStatusAwaitingPayment
				job.ErrorCode = "insufficient_credits"
				job.ErrorMessage = "not enough credits to reserve"
				activated = job
				insufficient = true
				return nil
			}
		}

		if err := repos.Jobs.UpdateStatus(ctx, job.ID, from, domain.JobStatusCreditsReserved, "", ""); err != nil {
			return err
		}
		reserved := *job
		reserved.Status = domain.JobStatusCreditsReserved
		reserved.CostReserved = amount
		reserved.ErrorCode = ""
		reserved.ErrorMessage = ""
		if err := repos.Jobs.Update(ctx, &reserved); err != nil {
			return err
		}
		if err := repos.Jobs.UpdateStatus(ctx, job.ID, domain.JobStatusCreditsReserved, domain.JobStatusQueued, "", ""); err != nil {
			return err
		}
		queued := reserved
		queued.Status = domain.JobStatusQueued
		if err := repos.Outbox.Add(ctx, jobEvent(ctx, "event.job.queued", &queued)); err != nil {
			return err
		}
		activated = &queued
		return nil
	})
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			if replay, replayErr := o.jobs.GetByIDForAccount(ctx, accountID, jobID); replayErr == nil {
				if replay.Status == domain.JobStatusExpired && replay.ErrorCode == domain.PreparedConfirmationExpiredCode {
					return nil, domain.ErrConflict
				}
				if activationReplayStatus(replay.Status) {
					return replay, nil
				}
				if replay.Status == domain.JobStatusAwaitingPayment {
					return replay, domain.ErrInsufficientCredits
				}
			}
		}
		return nil, fmt.Errorf("joborchestrator: activate prepared job: %w", err)
	}
	if confirmationExpired {
		return nil, domain.ErrConflict
	}
	if insufficient {
		return activated, domain.ErrInsufficientCredits
	}
	return activated, nil
}

func validatePreparedActivation(job *domain.Job, accountID uuid.UUID) error {
	if job == nil || job.AccountID != accountID || job.UserID != uuid.Nil || job.CommandID != uuid.Nil || job.VKPeerID != 0 ||
		job.Source != "web" || job.ChannelContext == nil || job.ChannelContext.Channel != domain.ChannelWeb ||
		job.ResultMode != domain.ResultModeAccountHistory || job.DeliveryTarget != nil || job.CostReserved != 0 || job.CostCaptured != 0 {
		return domain.ErrConflict
	}
	if err := job.ValidateResultContract(); err != nil {
		return fmt.Errorf("joborchestrator: prepared result contract: %w", err)
	}
	return nil
}

func preparedActivationAmount(job *domain.Job) (int64, error) {
	if job == nil || job.CostEstimate < 0 {
		return 0, domain.ErrConflict
	}
	if len(job.PricingSnapshot) == 0 {
		if job.CostEstimate == 0 && requiresBackendPrice(job.OperationType, job.Modality) {
			return 0, ErrBackendPriceRequired
		}
		return job.CostEstimate, nil
	}
	amount, ok := job.PricingSnapshotCredits()
	if !ok || amount != job.CostEstimate {
		return 0, domain.ErrConflict
	}
	return amount, nil
}

func createResultContract(in CreateJobInput, accountID uuid.UUID) (*domain.ChannelContext, domain.ResultMode, *domain.DeliveryTarget, error) {
	// Existing unit callers and stored compatibility flows predate the neutral
	// contract. Preserve their explicit legacy-unknown behavior only when no
	// part of the new contract was supplied.
	if in.ChannelContext == nil && in.ResultMode == "" && in.DeliveryTarget == nil {
		return nil, domain.ResultModeLegacyUnknown, nil, nil
	}
	if in.ChannelContext == nil {
		return nil, "", nil, fmt.Errorf("joborchestrator: %w: channel context is required", domain.ErrInvalidResultContract)
	}
	contextCopy := *in.ChannelContext
	var targetCopy *domain.DeliveryTarget
	if in.DeliveryTarget != nil {
		copy := *in.DeliveryTarget
		targetCopy = &copy
	}
	job := domain.Job{
		AccountID:      accountID,
		ChannelContext: &contextCopy,
		ResultMode:     in.ResultMode,
		DeliveryTarget: targetCopy,
	}
	if err := job.ValidateResultContract(); err != nil {
		return nil, "", nil, fmt.Errorf("joborchestrator: %w", err)
	}
	return &contextCopy, in.ResultMode, targetCopy, nil
}

func (o *Orchestrator) checkStoredJobCapacity(ctx context.Context, jobs domain.JobRepository, job *domain.Job, estimate int64) error {
	if o.maxActiveVideoJobsPerUser > 0 && job.OperationType == domain.OperationVideoGenerate {
		active, err := jobs.CountActiveByAccountOperation(ctx, job.AccountID, domain.OperationVideoGenerate)
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
		UserID:    job.UserID,
		AccountID: job.AccountID,
		Source:    job.Source,
		Operation: job.OperationType,
		Modality:  job.Modality,
		Estimate:  estimate,
	}); err != nil {
		return fmt.Errorf("joborchestrator: %w", err)
	}
	return nil
}

func activationReplayStatus(status domain.JobStatus) bool {
	switch status {
	case domain.JobStatusQueued,
		domain.JobStatusDispatchingProvider,
		domain.JobStatusProviderSubmitted,
		domain.JobStatusProviderPending,
		domain.JobStatusProviderProcessing,
		domain.JobStatusProviderSucceeded,
		domain.JobStatusProviderFailed,
		domain.JobStatusPostprocessing,
		domain.JobStatusResultReady,
		domain.JobStatusDelivering,
		domain.JobStatusSucceeded,
		domain.JobStatusFailedRetryable,
		domain.JobStatusFailedTerminal,
		domain.JobStatusCancelled,
		domain.JobStatusExpired,
		domain.JobStatusRefunded:
		return true
	default:
		return false
	}
}

func normalizePreparedParams(raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return json.RawMessage(`{}`), nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("joborchestrator: invalid params: %w", err)
	}
	if value == nil {
		return nil, fmt.Errorf("joborchestrator: params must be a JSON object")
	}
	if _, ok := value.(map[string]any); !ok {
		return nil, fmt.Errorf("joborchestrator: params must be a JSON object")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("joborchestrator: invalid params")
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("joborchestrator: normalize params: %w", err)
	}
	return normalized, nil
}

func preparePrice(in PrepareAccountJobInput) (int64, json.RawMessage, error) {
	if in.PricingSnapshot.Valid() {
		if in.CostEstimateCredits > 0 && in.CostEstimateCredits != in.PricingSnapshot.InternalCredits {
			return 0, nil, fmt.Errorf("joborchestrator: pricing snapshot cost %d does not match estimate %d", in.PricingSnapshot.InternalCredits, in.CostEstimateCredits)
		}
		raw, err := json.Marshal(in.PricingSnapshot)
		if err != nil {
			return 0, nil, fmt.Errorf("joborchestrator: pricing snapshot: %w", err)
		}
		return in.PricingSnapshot.InternalCredits, raw, nil
	}
	if pricingSnapshotProvided(in.PricingSnapshot) {
		return 0, nil, fmt.Errorf("joborchestrator: invalid pricing snapshot")
	}
	if in.CostEstimateCredits < 0 {
		return 0, nil, fmt.Errorf("joborchestrator: invalid cost estimate")
	}
	if in.CostEstimateCredits == 0 && requiresBackendPrice(in.Operation, in.Modality) {
		return 0, nil, fmt.Errorf("%w: missing price for %s/%s", ErrBackendPriceRequired, in.Operation, in.Modality)
	}
	return in.CostEstimateCredits, nil, nil
}

// pricingSnapshotProvided distinguishes an intentionally omitted snapshot from
// a malformed one. A supplied snapshot is immutable price evidence and must
// never silently fall back to a caller-provided estimate.
func pricingSnapshotProvided(snapshot pricingcatalog.PricingSnapshot) bool {
	return snapshot != (pricingcatalog.PricingSnapshot{})
}

func (o *Orchestrator) validatePreparedInputArtifacts(ctx context.Context, accountID uuid.UUID, artifactIDs []uuid.UUID) error {
	if len(artifactIDs) == 0 {
		return nil
	}
	if o.artifacts == nil {
		return fmt.Errorf("%w: repository unavailable", ErrInvalidInputArtifact)
	}
	seen := make(map[uuid.UUID]struct{}, len(artifactIDs))
	for _, id := range artifactIDs {
		if id == uuid.Nil {
			return fmt.Errorf("%w: empty id", ErrInvalidInputArtifact)
		}
		if _, duplicate := seen[id]; duplicate {
			return fmt.Errorf("%w: duplicate id", ErrInvalidInputArtifact)
		}
		seen[id] = struct{}{}
		artifact, err := o.artifacts.GetByIDForAccount(ctx, accountID, id)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return fmt.Errorf("%w: missing or foreign", ErrInvalidInputArtifact)
			}
			return fmt.Errorf("joborchestrator: input artifact lookup: %w", err)
		}
		if artifact.OwnerAccountID != accountID || artifact.Kind != domain.ArtifactKindInput || artifact.MediaType != domain.MediaTypeImage || artifact.Status != domain.ArtifactStatusReady || strings.TrimSpace(artifact.StorageBucket) == "" || strings.TrimSpace(artifact.StorageKey) == "" {
			return fmt.Errorf("%w: invalid account input", ErrInvalidInputArtifact)
		}
	}
	return nil
}

func preparedJobMatches(job *domain.Job, in PrepareAccountJobInput, pricingSnapshot json.RawMessage, estimate int64) bool {
	return job != nil &&
		job.AccountID == in.AccountID &&
		job.UserID == uuid.Nil &&
		job.CommandID == uuid.Nil &&
		job.VKPeerID == 0 &&
		job.Source == "web" &&
		job.ChannelContext != nil &&
		job.ChannelContext.Channel == domain.ChannelWeb &&
		job.ChannelContext.RecipientRef == "" &&
		job.ChannelContext.ThreadRef == "" &&
		job.ResultMode == domain.ResultModeAccountHistory &&
		job.DeliveryTarget == nil &&
		job.Status == domain.JobStatusPrepared &&
		job.OperationType == in.Operation &&
		job.Modality == in.Modality &&
		job.CostEstimate == estimate &&
		bytes.Equal(job.Params, in.Params) &&
		bytes.Equal(job.PricingSnapshot, pricingSnapshot) &&
		equalUUIDs(job.InputArtifactIDs, in.InputArtifactIDs)
}

func equalUUIDs(left, right []uuid.UUID) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func (o *Orchestrator) validateInputArtifacts(ctx context.Context, in CreateJobInput) error {
	if len(in.InputArtifactIDs) == 0 {
		return nil
	}
	if o.artifacts == nil {
		return fmt.Errorf("%w: repository unavailable", ErrInvalidInputArtifact)
	}
	for _, id := range in.InputArtifactIDs {
		if id == uuid.Nil {
			return fmt.Errorf("%w: empty id", ErrInvalidInputArtifact)
		}
		artifact, err := o.artifacts.GetByID(ctx, id)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return fmt.Errorf("%w: missing", ErrInvalidInputArtifact)
			}
			return fmt.Errorf("joborchestrator: input artifact lookup: %w", err)
		}
		if !inputArtifactOwnedBy(artifact, in.UserID, in.AccountID) {
			return fmt.Errorf("%w: foreign owner", ErrInvalidInputArtifact)
		}
		if artifact.Kind != domain.ArtifactKindInput {
			return fmt.Errorf("%w: kind %s", ErrInvalidInputArtifact, artifact.Kind)
		}
		if artifact.MediaType != domain.MediaTypeImage {
			return fmt.Errorf("%w: media %s", ErrInvalidInputArtifact, artifact.MediaType)
		}
		if artifact.Status != domain.ArtifactStatusReady {
			return fmt.Errorf("%w: status %s", ErrInvalidInputArtifact, artifact.Status)
		}
		if strings.TrimSpace(artifact.StorageBucket) == "" || strings.TrimSpace(artifact.StorageKey) == "" {
			return fmt.Errorf("%w: storage missing", ErrInvalidInputArtifact)
		}
	}
	return nil
}

func inputArtifactOwnedBy(artifact *domain.Artifact, userID, accountID uuid.UUID) bool {
	if artifact == nil {
		return false
	}
	if artifact.OwnerAccountID != uuid.Nil {
		return artifact.OwnerAccountID == ownerAccountID(userID, accountID)
	}
	return artifact.OwnerUserID == userID
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
		AccountID:        ownerAccountID(in.UserID, in.AccountID),
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
	ownerID := ownerAccountID(in.UserID, in.AccountID)
	if o.maxActiveVideoJobsPerUser > 0 && in.Operation == domain.OperationVideoGenerate {
		active, err := o.jobs.CountActiveByUserOperation(ctx, ownerID, domain.OperationVideoGenerate)
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
		AccountID: ownerID,
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
		AccountID     uuid.UUID            `json:"account_id,omitempty"`
		CorrelationID string               `json:"correlation_id,omitempty"`
		Traceparent   string               `json:"traceparent,omitempty"`
	}{job.ID, job.Status, job.OperationType, job.Modality, job.UserID, job.AccountID, job.CorrelationID, tracing.Traceparent(ctx)})

	return &domain.OutboxEvent{
		AggregateType: "job",
		AggregateID:   job.ID,
		EventType:     eventType,
		Payload:       payload,
	}
}

// conversationTitleEvent builds the deliberately narrow title-work envelope.
// The raw prompt remains in the normal job record for the text worker; it must
// never enter the outbox or Redis title task.
func conversationTitleEvent(ctx context.Context, job *domain.Job) *domain.OutboxEvent {
	payload, _ := json.Marshal(struct {
		JobID         uuid.UUID            `json:"job_id"`
		AccountID     uuid.UUID            `json:"account_id"`
		Source        string               `json:"source"`
		Operation     domain.OperationType `json:"operation"`
		Modality      domain.Modality      `json:"modality"`
		CorrelationID string               `json:"correlation_id,omitempty"`
		Traceparent   string               `json:"traceparent,omitempty"`
	}{job.ID, job.AccountID, job.Source, job.OperationType, job.Modality, job.CorrelationID, tracing.Traceparent(ctx)})

	return &domain.OutboxEvent{
		AggregateType: "job",
		AggregateID:   job.ID,
		EventType:     outboxrelay.EventConversationTitleQueued,
		Payload:       payload,
	}
}

func ownerAccountID(userID, accountID uuid.UUID) uuid.UUID {
	if accountID != uuid.Nil {
		return accountID
	}
	return userID
}
