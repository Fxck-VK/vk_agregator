package worker

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/platform/tracing"
)

// GenerationWorker handles text/image/video generation tasks: it submits the
// job to a provider, persists the provider task, then either finishes the job
// (synchronous providers) or hands it to the poll worker (asynchronous ones).
// The same struct serves every modality; the queue/stream it consumes from
// decides which jobs it sees.
type GenerationWorker struct {
	processor
}

// NewGenerationWorker builds a GenerationWorker from shared dependencies.
func NewGenerationWorker(d Deps) *GenerationWorker {
	return &GenerationWorker{processor: newProcessor(d)}
}

// Process advances one job task. Returning nil means the task is fully handled
// and may be acknowledged; returning an error leaves it pending so it is
// retried/recovered (retry safety). It is idempotent: re-delivery of an
// already-submitted job resumes from its provider task instead of re-submitting.
func (g *GenerationWorker) Process(ctx context.Context, task queue.Task) error {
	job, err := g.jobs.GetByID(ctx, task.JobID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if isDone(job.Status) {
		return nil
	}

	// Recovery / idempotency: if a provider task is already in flight, do not
	// submit again — resume by polling it.
	if active, err := g.activeTask(ctx, job.ID); err != nil {
		return err
	} else if active != nil {
		provider, perr := g.providers.ForName(active.Provider)
		if perr != nil {
			return perr
		}
		return g.pollOnce(ctx, job, active, provider, task)
	}

	if err := g.toDispatching(ctx, job); err != nil {
		return err
	}

	attempt, err := g.attemptCount(ctx, job.ID)
	if err != nil {
		return err
	}
	attempt++

	req, err := g.buildRequest(ctx, job, attempt)
	if err != nil {
		return err
	}
	provider, err := g.providers.ForRequest(ctx, req)
	if err != nil {
		return g.handleFailure(ctx, job, task, domain.ProviderErrUnsupportedCapab, err.Error())
	}
	callCtx, cancel := g.providerCallContext(ctx)
	submitCtx, submitSpan := tracing.Start(callCtx, "provider.submit",
		attribute.String("job.id", job.ID.String()),
		attribute.String("provider", string(provider.Name())),
		attribute.String("operation", string(job.OperationType)),
		tracing.CorrelationAttr(job.CorrelationID),
	)
	submitted, err := provider.Submit(submitCtx, req)
	if err != nil {
		tracing.RecordError(submitSpan, err)
		submitSpan.End()
		cancel()
		return g.handleFailure(ctx, job, task, classOf(err), err.Error())
	}
	submitSpan.End()
	cancel()

	taskProvider := provider.Name()
	if submitted.Provider != "" {
		taskProvider = submitted.Provider
	}
	pt, err := g.persistTask(ctx, job, taskProvider, submitted, req, attempt)
	if err != nil {
		return err
	}
	if err := g.setStatus(ctx, job, domain.JobStatusProviderSubmitted, "", ""); err != nil {
		return err
	}
	return g.pollOnce(ctx, job, pt, provider, task)
}

// toDispatching moves a queued job into dispatching_provider, tolerating a job
// that is already there after a crash mid-dispatch.
func (g *GenerationWorker) toDispatching(ctx context.Context, job *domain.Job) error {
	if job.Status == domain.JobStatusDispatchingProvider {
		return nil
	}
	return g.setStatus(ctx, job, domain.JobStatusDispatchingProvider, "", "")
}

// persistTask records the submitted provider task, reconciling with an existing
// row if a concurrent attempt already created it (idempotency key conflict).
func (g *GenerationWorker) persistTask(ctx context.Context, job *domain.Job, provider domain.ProviderName, submitted domain.ProviderTask, req domain.ProviderRequest, attempt int) (*domain.ProviderTask, error) {
	now := time.Now()
	pt := &domain.ProviderTask{
		ID:             uuid.New(),
		JobID:          job.ID,
		Provider:       provider,
		ModelCode:      submitted.ModelCode,
		ExternalID:     submitted.ExternalID,
		AttemptNo:      attempt,
		Status:         submitted.Status,
		Request:        req.Params,
		IdempotencyKey: req.IdempotencyKey,
		SubmittedAt:    &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if pt.Status == "" {
		pt.Status = domain.ProviderTaskPending
	}
	err := g.tasks.Create(ctx, pt)
	if errors.Is(err, domain.ErrConflict) {
		// Another attempt already created the task; reconcile by external id.
		if existing, gerr := g.tasks.GetByExternalID(ctx, provider, submitted.ExternalID); gerr == nil {
			return existing, nil
		}
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return pt, nil
}
