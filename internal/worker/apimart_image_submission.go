package worker

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/service/providermodels"
)

func durablePaidSubmitRoute(provider domain.ProviderName, model string) bool {
	return providermodels.IsKlingVeoVideoRoute(provider, model) || providermodels.IsOmniVideoRoute(provider, model) || providermodels.IsPaidTextRoute(provider, model) || (provider == domain.ProviderAPIMart && (model == providermodels.ProviderModelMidjourneyV7 || model == providermodels.ProviderModelFlux2Pro))
}

func isDurablePaidSubmitJob(job *domain.Job) bool {
	if job == nil || (job.Modality != domain.ModalityImage && job.Modality != domain.ModalityText && job.Modality != domain.ModalityVideo) {
		return false
	}
	var params struct {
		Provider  domain.ProviderName `json:"provider"`
		ModelCode string              `json:"model_code"`
	}
	return json.Unmarshal(job.Params, &params) == nil && durablePaidSubmitRoute(params.Provider, params.ModelCode)
}

func unresolvedPaidSubmitIntent(task *domain.ProviderTask) bool {
	return task != nil && durablePaidSubmitRoute(task.Provider, task.ModelCode) && task.ExternalID == ""
}

// The existing UNIQUE provider_tasks.idempotency_key is a durable per-Job
// claim. Only the successful inserter may make the paid request.
// A retry must never allocate another attempt key for the same Job.
func (g *GenerationWorker) claimPaidSubmit(ctx context.Context, req *domain.ProviderRequest) (*domain.ProviderTask, error) {
	prefix := "flux_2_pro_submit:"
	if providermodels.IsOmniVideoRoute(req.Provider, req.ModelCode) {
		prefix = "apimart_omni_video_submit:"
	}
	if req.Provider == domain.ProviderKIE {
		prefix = "kie_text_submit:"
	} else if providermodels.IsPaidTextRoute(req.Provider, req.ModelCode) {
		prefix = "apimart_text_submit:"
	}
	if req.ModelCode == providermodels.ProviderModelMidjourneyV7 {
		prefix = "midjourney_imagine_submit:"
	}
	req.IdempotencyKey = prefix + req.JobID.String()
	req.AttemptNo = 1
	intent := &domain.ProviderTask{ID: uuid.New(), JobID: req.JobID, Provider: req.Provider, ModelCode: req.ModelCode, AttemptNo: 1,
		Status: domain.ProviderTaskPending, IdempotencyKey: req.IdempotencyKey, Request: domain.DurableProviderTaskRequestJSON()}
	if err := g.tasks.Create(ctx, intent); err != nil {
		return nil, err
	}
	return intent, nil
}

func (g *GenerationWorker) completePaidSubmit(ctx context.Context, intent *domain.ProviderTask, submitted domain.ProviderTask) (*domain.ProviderTask, error) {
	intent.ExternalID, intent.Status = submitted.ExternalID, submitted.Status
	intent.SubmittedAt, intent.CompletedAt, intent.ErrorClass = submitted.SubmittedAt, submitted.CompletedAt, submitted.ErrorClass
	intent.Result = domain.DurableProviderTaskResultJSONFromRaw(submitted.Result, submitted.Status, submitted.ErrorClass)
	if err := g.tasks.Update(ctx, intent); err != nil {
		return nil, err
	}
	return intent, nil
}

func (p *processor) failPaidSubmit(ctx context.Context, job *domain.Job, intent *domain.ProviderTask, task queue.Task, class domain.ProviderErrorClass) error {
	now := time.Now()
	intent.Status, intent.ErrorClass, intent.CompletedAt = domain.ProviderTaskFailed, class, &now
	if err := p.tasks.Update(ctx, intent); err != nil {
		return err
	}
	return p.handleFailure(ctx, job, task, class, safeProviderFailureMessage(class))
}

func (p *processor) resumePaidSubmit(ctx context.Context, job *domain.Job, intent *domain.ProviderTask, task queue.Task) error {
	if providermodels.IsPaidTextRoute(intent.Provider, intent.ModelCode) && len(job.OutputArtifactIDs) > 0 && intent.ErrorClass == "" {
		intent.ExternalID = "text:" + job.ID.String()
		intent.Status = domain.ProviderTaskSucceeded
		if job.Status == domain.JobStatusDispatchingProvider {
			if err := p.setStatus(ctx, job, domain.JobStatusProviderSubmitted, "", ""); err != nil {
				return err
			}
		}
		return p.applyResult(ctx, job, intent, domain.ProviderTaskResult{Status: domain.ProviderTaskSucceeded}, task)
	}
	if intent.ErrorClass != "" {
		return p.failPaidSubmit(ctx, job, intent, task, intent.ErrorClass)
	}
	// Give an in-flight sender its bounded call timeout plus time to persist.
	// On recovery after that deadline, fail closed instead of guessing whether
	// the upstream accepted the request. Operator reconciliation may be needed.
	if time.Since(intent.CreatedAt) < p.callTimeout+time.Minute {
		return errors.New("worker: paid submission is awaiting its persisted outcome")
	}
	return p.failPaidSubmit(ctx, job, intent, task, domain.ProviderErrSubmitIndeterminate)
}
