// Package worker contains the worker pools that turn queued Jobs into delivered
// results. Workers are the ONLY place AI providers are called (architecture
// invariant: VK handlers and the orchestrator never touch providers).
//
// Generation workers run the flow Job -> Provider -> ProviderTask -> Artifact ->
// Delivery Queue. When a provider is asynchronous the work is handed to the
// provider-poll worker (Provider Poll -> Update Status -> Requeue -> Artifact ->
// Delivery Queue). Every worker is retry-safe and idempotent: re-delivering the
// same task never double-submits to a provider or duplicates artifacts, and
// in-flight work is recovered after a restart via the consumer group's pending
// list.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/service/moderationservice"
)

// maxProviderAttempts caps how many times a job is re-submitted to a provider
// before a retryable failure is treated as terminal.
const maxProviderAttempts = 3

// ArtifactSaver stores provider outputs as artifacts. Implemented by
// artifactservice.Service.
type ArtifactSaver interface {
	SaveRemoteArtifact(ctx context.Context, ownerID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, url string) (*domain.Artifact, error)
}

// StreamPublisher publishes a task to a specific stream. Implemented by
// redisqueue.Publisher.
type StreamPublisher interface {
	PublishTo(ctx context.Context, stream string, task queue.Task) error
}

// classedError is implemented by provider errors that carry a normalized class.
type classedError interface {
	ProviderErrorClass() domain.ProviderErrorClass
}

// Registry resolves which provider handles an operation, and looks providers up
// by name when reconciling persisted provider tasks.
type Registry struct {
	byName map[domain.ProviderName]domain.Provider
	def    domain.Provider
}

// NewRegistry builds a registry with a default provider and optional extras.
func NewRegistry(def domain.Provider, more ...domain.Provider) *Registry {
	r := &Registry{byName: map[domain.ProviderName]domain.Provider{}, def: def}
	if def != nil {
		r.byName[def.Name()] = def
	}
	for _, p := range more {
		r.byName[p.Name()] = p
	}
	return r
}

// ForOperation returns the provider that should handle an operation. Routing is
// currently static (the default provider); model/provider routing is future
// work, but the seam is here.
func (r *Registry) ForOperation(_ domain.OperationType) (domain.Provider, error) {
	if r.def == nil {
		return nil, errors.New("worker: no default provider configured")
	}
	return r.def, nil
}

// ForName returns the provider with the given name.
func (r *Registry) ForName(name domain.ProviderName) (domain.Provider, error) {
	p, ok := r.byName[name]
	if !ok {
		return nil, fmt.Errorf("worker: unknown provider %q", name)
	}
	return p, nil
}

// processor holds the shared dependencies and result-handling logic used by
// both the generation and poll workers.
type processor struct {
	jobs        domain.JobRepository
	tasks       domain.ProviderTaskRepository
	artifacts   ArtifactSaver
	providers   *Registry
	streams     StreamPublisher
	moderator   Moderator
	modResults  domain.ModerationResultRepository
	releaser    ReservationReleaser
	maxAttempts int
	backoff     func(attempt int) time.Duration
	now         func() time.Time
}

// Moderator gates delivery: generated output must pass a moderation check
// before it is delivered (invariant #15). Implemented by moderationservice.
type Moderator interface {
	Name() string
	Check(ctx context.Context, in moderationservice.Input) (moderationservice.Outcome, error)
}

// ReservationReleaser frees a job's reserved credits when delivery is blocked
// (e.g. by moderation) so blocked jobs are never charged. Implemented by
// billingservice.Service.
type ReservationReleaser interface {
	ReleaseForJob(ctx context.Context, jobID uuid.UUID) error
}

// Deps bundles the dependencies shared by the workers.
type Deps struct {
	Jobs      domain.JobRepository
	Tasks     domain.ProviderTaskRepository
	Artifacts ArtifactSaver
	Providers *Registry
	Streams   StreamPublisher
	// Moderator, when set, runs an output moderation check before delivery.
	// When nil, moderation is skipped (allow-all) for local/test wiring.
	Moderator Moderator
	// ModResults, when set, persists moderation verdicts for audit.
	ModResults domain.ModerationResultRepository
	// Releaser, when set, frees reserved credits for moderation-blocked jobs.
	Releaser ReservationReleaser
	// MaxAttempts caps retryable re-enqueues before dead-lettering (default 3).
	MaxAttempts int
	// Backoff returns the delay before re-enqueue for the given attempt number.
	// Defaults to no delay (keeps tests fast).
	Backoff func(attempt int) time.Duration
	// Now overrides the clock; defaults to time.Now.
	Now func() time.Time
}

func newProcessor(d Deps) processor {
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
	return processor{
		jobs:        d.Jobs,
		tasks:       d.Tasks,
		artifacts:   d.Artifacts,
		providers:   d.Providers,
		streams:     d.Streams,
		moderator:   d.Moderator,
		modResults:  d.ModResults,
		releaser:    d.Releaser,
		maxAttempts: maxAttempts,
		backoff:     backoff,
		now:         now,
	}
}

// promptParams is the subset of job params the provider request needs.
type promptParams struct {
	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negative_prompt"`
}

// buildRequest builds the normalized provider request for a job. The submit
// idempotency key is scoped to the attempt so a re-delivered task maps to one
// provider task, while a genuine retry after failure starts a fresh one.
func (p *processor) buildRequest(job *domain.Job, attempt int) domain.ProviderRequest {
	var pp promptParams
	if len(job.Params) > 0 {
		_ = json.Unmarshal(job.Params, &pp)
	}
	return domain.ProviderRequest{
		JobID:          job.ID,
		Operation:      job.OperationType,
		Modality:       job.Modality,
		Prompt:         pp.Prompt,
		NegativePrompt: pp.NegativePrompt,
		Params:         job.Params,
		IdempotencyKey: fmt.Sprintf("provider_submit:%s:%d", job.ID, attempt),
	}
}

// taskOf builds the queue task that represents a job.
func taskOf(job *domain.Job) queue.Task {
	return queue.Task{
		JobID:         job.ID,
		Operation:     job.OperationType,
		Modality:      job.Modality,
		CorrelationID: job.CorrelationID,
	}
}

func mediaTypeFor(modality domain.Modality) domain.MediaType {
	switch modality {
	case domain.ModalityImage:
		return domain.MediaTypeImage
	case domain.ModalityVideo:
		return domain.MediaTypeVideo
	case domain.ModalityAudio:
		return domain.MediaTypeAudio
	default:
		return domain.MediaTypeText
	}
}

// isDone reports whether a job has reached a state where neither generation nor
// polling should act on it any further.
func isDone(status domain.JobStatus) bool {
	switch status {
	case domain.JobStatusResultReady,
		domain.JobStatusDelivering,
		domain.JobStatusSucceeded,
		domain.JobStatusFailedTerminal,
		domain.JobStatusCancelled,
		domain.JobStatusRefunded,
		domain.JobStatusRejected,
		domain.JobStatusExpired:
		return true
	default:
		return false
	}
}

// isRetryable maps a normalized provider error class to a retry decision.
func isRetryable(class domain.ProviderErrorClass) bool {
	switch class {
	case domain.ProviderErrRateLimited,
		domain.ProviderErrTimeout,
		domain.ProviderErrOverloaded,
		domain.ProviderErrInternal,
		domain.ProviderErrOutputDownloadFailed:
		return true
	default:
		return false
	}
}

// classOf extracts the normalized error class from a provider error, defaulting
// to provider_internal_error for unclassified failures.
func classOf(err error) domain.ProviderErrorClass {
	var ce classedError
	if errors.As(err, &ce) {
		return ce.ProviderErrorClass()
	}
	return domain.ProviderErrInternal
}

// setStatus applies a state-machine transition, treating "already there" as a
// no-op so repeated deliveries are idempotent.
func (p *processor) setStatus(ctx context.Context, job *domain.Job, to domain.JobStatus, errCode, errMsg string) error {
	if job.Status == to {
		return nil
	}
	if err := p.jobs.UpdateStatus(ctx, job.ID, job.Status, to, errCode, errMsg); err != nil {
		return err
	}
	job.Status = to
	return nil
}

// activeTask returns the most recent provider task for a job that is still
// pending/processing/succeeded (i.e. worth polling), or nil if the job needs a
// fresh submission.
func (p *processor) activeTask(ctx context.Context, jobID uuid.UUID) (*domain.ProviderTask, error) {
	tasks, err := p.tasks.ListByJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	for i := len(tasks) - 1; i >= 0; i-- {
		switch tasks[i].Status {
		case domain.ProviderTaskPending, domain.ProviderTaskProcessing, domain.ProviderTaskSucceeded:
			return tasks[i], nil
		}
	}
	return nil, nil
}

// latestTask returns the most recent provider task for a job, or nil.
func (p *processor) latestTask(ctx context.Context, jobID uuid.UUID) (*domain.ProviderTask, error) {
	tasks, err := p.tasks.ListByJob(ctx, jobID)
	if err != nil || len(tasks) == 0 {
		return nil, err
	}
	return tasks[len(tasks)-1], nil
}

// pollOnce polls the provider once and applies the normalized result.
func (p *processor) pollOnce(ctx context.Context, job *domain.Job, pt *domain.ProviderTask, provider domain.Provider, task queue.Task) error {
	res, err := provider.Poll(ctx, domain.ProviderTaskRef{Provider: pt.Provider, ExternalID: pt.ExternalID})
	if err != nil {
		return p.handleFailure(ctx, job, task, classOf(err), err.Error())
	}
	return p.applyResult(ctx, job, pt, res, task)
}

// applyResult persists the task result and advances the job: success stores
// artifacts and enqueues delivery, in-progress requeues for polling, failure is
// classified and retried or made terminal.
func (p *processor) applyResult(ctx context.Context, job *domain.Job, pt *domain.ProviderTask, res domain.ProviderTaskResult, task queue.Task) error {
	pt.Status = res.Status
	if raw, err := json.Marshal(res); err == nil {
		pt.Result = raw
	}
	if res.ErrorClass != "" {
		pt.ErrorClass = res.ErrorClass
	}
	if res.Status.IsTerminal() {
		now := p.now()
		pt.CompletedAt = &now
	}
	if err := p.tasks.Update(ctx, pt); err != nil {
		return err
	}

	switch res.Status {
	case domain.ProviderTaskSucceeded:
		if err := p.saveOutputs(ctx, job, res.OutputURLs); err != nil {
			// A download failure is retryable provider-side.
			return p.handleFailure(ctx, job, task, domain.ProviderErrOutputDownloadFailed, err.Error())
		}
		if err := p.setStatus(ctx, job, domain.JobStatusProviderSucceeded, "", ""); err != nil {
			return err
		}
		// Output moderation gates delivery (invariant #15). A block stops the
		// pipeline here: no delivery, no capture.
		blocked, err := p.moderateOutput(ctx, job)
		if err != nil {
			return err
		}
		if blocked {
			return nil
		}
		if err := p.setStatus(ctx, job, domain.JobStatusResultReady, "", ""); err != nil {
			return err
		}
		return p.streams.PublishTo(ctx, redisqueue.StreamDelivery, taskOf(job))

	case domain.ProviderTaskProcessing:
		if err := p.setStatus(ctx, job, domain.JobStatusProviderProcessing, "", ""); err != nil {
			return err
		}
		return p.streams.PublishTo(ctx, redisqueue.StreamProviderPoll, taskOf(job))

	case domain.ProviderTaskPending:
		if err := p.setStatus(ctx, job, domain.JobStatusProviderPending, "", ""); err != nil {
			return err
		}
		return p.streams.PublishTo(ctx, redisqueue.StreamProviderPoll, taskOf(job))

	case domain.ProviderTaskFailed:
		return p.handleFailure(ctx, job, task, res.ErrorClass, res.ErrorMessage)

	case domain.ProviderTaskCancelled:
		// Cancellation may arrive from a non-cancellable state; best effort.
		_ = p.setStatus(ctx, job, domain.JobStatusCancelled, "cancelled", "provider task cancelled")
		return nil
	}
	return nil
}

// saveOutputs stores each provider output URL as an output artifact and records
// the artifact ids on the job, skipping ids already attached (idempotent).
func (p *processor) saveOutputs(ctx context.Context, job *domain.Job, urls []string) error {
	mediaType := mediaTypeFor(job.Modality)
	for _, url := range urls {
		art, err := p.artifacts.SaveRemoteArtifact(ctx, job.UserID, &job.ID, domain.ArtifactKindOutput, mediaType, url)
		if err != nil {
			return err
		}
		if !containsID(job.OutputArtifactIDs, art.ID) {
			job.OutputArtifactIDs = append(job.OutputArtifactIDs, art.ID)
		}
	}
	return p.jobs.Update(ctx, job)
}

// handleFailure classifies a provider failure and either re-queues the job for
// another attempt (with backoff) or, once the retry budget is exhausted (or the
// error is non-retryable), dead-letters the task and moves the job to a terminal
// failed state. The budget spans every phase (submit, poll, download): it is the
// max of the provider-task count and the task's own re-enqueue counter, so a
// failure that does not create a new provider task (e.g. output_download_failed)
// can no longer loop forever.
func (p *processor) handleFailure(ctx context.Context, job *domain.Job, task queue.Task, class domain.ProviderErrorClass, msg string) error {
	code := string(class)

	// If a provider task was running, record the provider_failed state first.
	switch job.Status {
	case domain.JobStatusProviderSubmitted, domain.JobStatusProviderPending, domain.JobStatusProviderProcessing:
		_ = p.setStatus(ctx, job, domain.JobStatusProviderFailed, code, msg)
	}

	providerAttempts, err := p.attemptCount(ctx, job.ID)
	if err != nil {
		return err
	}
	attempts := providerAttempts
	if task.Attempt+1 > attempts {
		attempts = task.Attempt + 1
	}

	if isRetryable(class) && attempts < p.maxAttempts {
		if err := p.setStatus(ctx, job, domain.JobStatusFailedRetryable, code, msg); err != nil {
			return err
		}
		if err := p.setStatus(ctx, job, domain.JobStatusQueued, code, msg); err != nil {
			return err
		}
		next := task
		next.Attempt = task.Attempt + 1
		p.sleepBackoff(ctx, next.Attempt)
		return p.streams.PublishTo(ctx, redisqueue.StreamForOperation(job.OperationType), next)
	}

	// Budget exhausted (retryable) or non-retryable: dead-letter retryable
	// failures for inspection, then move the job to a terminal state.
	if isRetryable(class) {
		p.toDLQ(ctx, task, code, msg)
	}
	metrics.JobsTerminal.WithLabelValues(string(domain.JobStatusFailedTerminal)).Inc()
	return p.setStatus(ctx, job, domain.JobStatusFailedTerminal, code, msg)
}

// moderateOutput runs the output moderation check and, on a block, rejects the
// job (no delivery, no capture), releases the reservation and records an audit
// verdict. It returns blocked=true when delivery must be stopped. When no
// moderator is configured it is a no-op (allow).
func (p *processor) moderateOutput(ctx context.Context, job *domain.Job) (bool, error) {
	if p.moderator == nil {
		return false, nil
	}
	var pp promptParams
	if len(job.Params) > 0 {
		_ = json.Unmarshal(job.Params, &pp)
	}
	out, err := p.moderator.Check(ctx, moderationservice.Input{
		Stage:    domain.ModerationStageOutput,
		Modality: job.Modality,
		Prompt:   pp.Prompt,
	})
	if err != nil {
		return false, err
	}

	if p.modResults != nil {
		var artID *uuid.UUID
		if len(job.OutputArtifactIDs) > 0 {
			id := job.OutputArtifactIDs[0]
			artID = &id
		}
		_ = p.modResults.Create(ctx, &domain.ModerationResult{
			JobID:      job.ID,
			ArtifactID: artID,
			Stage:      domain.ModerationStageOutput,
			Decision:   out.Decision,
			Categories: out.Categories,
			Provider:   p.moderator.Name(),
		})
	}

	metrics.ModerationDecisions.WithLabelValues(string(out.Decision)).Inc()
	if out.Decision.Allowed() {
		return false, nil
	}

	if err := p.setStatus(ctx, job, domain.JobStatusRejected, "content_rejected", "blocked by output moderation"); err != nil {
		return false, err
	}
	metrics.JobsTerminal.WithLabelValues(string(domain.JobStatusRejected)).Inc()
	if p.releaser != nil {
		if err := p.releaser.ReleaseForJob(ctx, job.ID); err != nil {
			return false, err
		}
	}
	return true, nil
}

// toDLQ publishes the exhausted task to the dead-letter stream. It is best
// effort: a DLQ publish failure must not block moving the job to terminal.
func (p *processor) toDLQ(ctx context.Context, task queue.Task, code, msg string) {
	metrics.DLQRouted.WithLabelValues("provider").Inc()
	_ = p.streams.PublishTo(ctx, redisqueue.StreamDLQ, task)
	_ = code
	_ = msg
}

// sleepBackoff waits for the configured backoff before a re-enqueue, honoring
// context cancellation.
func (p *processor) sleepBackoff(ctx context.Context, attempt int) {
	d := p.backoff(attempt)
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

func (p *processor) attemptCount(ctx context.Context, jobID uuid.UUID) (int, error) {
	tasks, err := p.tasks.ListByJob(ctx, jobID)
	if err != nil {
		return 0, err
	}
	return len(tasks), nil
}

func containsID(ids []uuid.UUID, id uuid.UUID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}
