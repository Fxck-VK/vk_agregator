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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"

	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/platform/tracing"
	"vk-ai-aggregator/internal/service/dialogcontext"
	"vk-ai-aggregator/internal/service/moderationservice"
)

// maxProviderAttempts caps how many times a job is re-submitted to a provider
// before a retryable failure is treated as terminal.
const maxProviderAttempts = 3

// defaultProviderCallTimeout bounds one provider Submit/Poll call inside the
// worker so a stuck provider cannot keep a job generating forever.
const defaultProviderCallTimeout = 60 * time.Second

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

// Registry resolves which provider handles a request, and looks providers up
// by name when reconciling persisted provider tasks. Selection prefers healthy
// capable providers with lower configured cost and observed latency, then falls
// back across the chain when retryable submit failures occur.
type Registry struct {
	mu                  sync.Mutex
	byName              map[domain.ProviderName]domain.Provider
	order               []domain.ProviderName
	preferredByModality map[domain.Modality]domain.ProviderName
	breakers            map[domain.ProviderName]*breakerState
	now                 func() time.Time
}

// NewRegistry builds a registry with a default provider and optional extras.
func NewRegistry(def domain.Provider, more ...domain.Provider) *Registry {
	r := &Registry{
		byName:              map[domain.ProviderName]domain.Provider{},
		preferredByModality: map[domain.Modality]domain.ProviderName{},
		breakers:            map[domain.ProviderName]*breakerState{},
		now:                 time.Now,
	}
	if def != nil {
		r.add(def)
	}
	for _, p := range more {
		r.add(p)
	}
	return r
}

// PreferProvider makes a provider the first candidate for a modality when it
// supports the request. Other capable providers remain explicit fallbacks.
func (r *Registry) PreferProvider(modality domain.Modality, name domain.ProviderName) {
	if modality == "" || name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.preferredByModality[modality] = name
}

func (r *Registry) add(p domain.Provider) {
	name := p.Name()
	if _, exists := r.byName[name]; !exists {
		r.order = append(r.order, name)
	}
	r.byName[name] = p
	if _, exists := r.breakers[name]; !exists {
		r.breakers[name] = &breakerState{}
	}
}

// ForOperation returns the provider that should handle an operation. It is kept
// for old callers/tests; new generation flow uses ForRequest so the router can
// account for modality and cost.
func (r *Registry) ForOperation(_ domain.OperationType) (domain.Provider, error) {
	if len(r.order) == 0 {
		return nil, errors.New("worker: no default provider configured")
	}
	return r.ForName(r.order[0])
}

// ForRequest returns a routed provider wrapper. Submit on the wrapper tries the
// selected provider chain in order and returns the task from the actual provider
// that accepted the request.
func (r *Registry) ForRequest(ctx context.Context, req domain.ProviderRequest) (domain.Provider, error) {
	candidates, err := r.candidates(ctx, req)
	if err != nil {
		return nil, err
	}
	return &routedProvider{registry: r, candidates: candidates}, nil
}

// ForName returns the provider with the given name.
func (r *Registry) ForName(name domain.ProviderName) (domain.Provider, error) {
	p, err := r.rawForName(name)
	if err != nil {
		return nil, err
	}
	return &observedProvider{registry: r, provider: p}, nil
}

func (r *Registry) rawForName(name domain.ProviderName) (domain.Provider, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byName[name]
	if !ok {
		return nil, fmt.Errorf("worker: unknown provider %q", name)
	}
	return p, nil
}

type providerCandidate struct {
	provider  domain.Provider
	name      domain.ProviderName
	cost      int64
	latency   time.Duration
	open      bool
	preferred bool
	order     int
}

type breakerState struct {
	failures    int
	openedUntil time.Time
	latency     time.Duration
}

func (r *Registry) candidates(ctx context.Context, req domain.ProviderRequest) ([]providerCandidate, error) {
	r.mu.Lock()
	order := append([]domain.ProviderName(nil), r.order...)
	providers := make(map[domain.ProviderName]domain.Provider, len(r.byName))
	states := make(map[domain.ProviderName]breakerState, len(r.breakers))
	preferred := r.preferredByModality[req.Modality]
	now := r.now()
	for name, provider := range r.byName {
		providers[name] = provider
	}
	for name, state := range r.breakers {
		states[name] = *state
	}
	r.mu.Unlock()

	var healthy []providerCandidate
	var open []providerCandidate
	for idx, name := range order {
		provider := providers[name]
		if provider == nil {
			continue
		}
		if ok, err := supports(ctx, provider, req); err != nil || !ok {
			continue
		}
		estimate, err := provider.Estimate(ctx, req)
		if err != nil {
			if classOf(err) == domain.ProviderErrUnsupportedCapab {
				continue
			}
			estimate.AmountCredits = int64(idx + 1)
		}
		st := states[name]
		candidate := providerCandidate{
			provider:  provider,
			name:      name,
			cost:      estimate.AmountCredits,
			latency:   st.latency,
			open:      !st.openedUntil.IsZero() && now.Before(st.openedUntil),
			preferred: preferred != "" && name == preferred,
			order:     idx,
		}
		if candidate.open {
			open = append(open, candidate)
			continue
		}
		healthy = append(healthy, candidate)
	}
	sortCandidates(healthy)
	if len(healthy) > 0 {
		return healthy, nil
	}
	sortCandidates(open)
	if len(open) > 0 {
		// All capable providers are open. Try them anyway so a temporary breaker
		// window cannot make the platform permanently unavailable.
		return open, nil
	}
	return nil, fmt.Errorf("worker: no provider supports %s/%s", req.Operation, req.Modality)
}

func sortCandidates(c []providerCandidate) {
	sort.SliceStable(c, func(i, j int) bool {
		if c[i].preferred != c[j].preferred {
			return c[i].preferred
		}
		if c[i].cost != c[j].cost {
			return c[i].cost < c[j].cost
		}
		if c[i].latency != c[j].latency {
			if c[i].latency == 0 {
				return false
			}
			if c[j].latency == 0 {
				return true
			}
			return c[i].latency < c[j].latency
		}
		return c[i].order < c[j].order
	})
}

func supports(ctx context.Context, provider domain.Provider, req domain.ProviderRequest) (bool, error) {
	caps, err := provider.Capabilities(ctx)
	if err != nil {
		return false, err
	}
	for _, cap := range caps {
		if cap.Operation != req.Operation {
			continue
		}
		if cap.Modality != "" && req.Modality != "" && cap.Modality != req.Modality {
			continue
		}
		if req.ModelCode != "" && cap.ModelCode != "" && req.ModelCode != cap.ModelCode {
			continue
		}
		return true, nil
	}
	return false, nil
}

func (r *Registry) record(name domain.ProviderName, started time.Time, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.breakers[name]
	if state == nil {
		state = &breakerState{}
		r.breakers[name] = state
	}
	if !started.IsZero() {
		elapsed := r.now().Sub(started)
		if elapsed > 0 {
			if state.latency == 0 {
				state.latency = elapsed
			} else {
				state.latency = (state.latency + elapsed) / 2
			}
		}
	}
	if err == nil {
		state.failures = 0
		state.openedUntil = time.Time{}
		return
	}
	if !isRetryable(classOf(err)) {
		return
	}
	state.failures++
	if state.failures >= 2 {
		state.openedUntil = r.now().Add(time.Duration(state.failures) * 30 * time.Second)
	}
}

type observedProvider struct {
	registry *Registry
	provider domain.Provider
}

func (p *observedProvider) Name() domain.ProviderName { return p.provider.Name() }
func (p *observedProvider) Capabilities(ctx context.Context) ([]domain.Capability, error) {
	return p.provider.Capabilities(ctx)
}
func (p *observedProvider) Estimate(ctx context.Context, req domain.ProviderRequest) (domain.CostEstimate, error) {
	return p.provider.Estimate(ctx, req)
}
func (p *observedProvider) Submit(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	started := p.registry.now()
	task, err := p.provider.Submit(ctx, req)
	p.registry.record(p.provider.Name(), started, err)
	return task, err
}
func (p *observedProvider) Poll(ctx context.Context, ref domain.ProviderTaskRef) (domain.ProviderTaskResult, error) {
	started := p.registry.now()
	res, err := p.provider.Poll(ctx, ref)
	recordErr := err
	if err == nil && res.Status == domain.ProviderTaskFailed && isRetryable(res.ErrorClass) {
		recordErr = providerResultError{class: res.ErrorClass, message: res.ErrorMessage}
	}
	p.registry.record(p.provider.Name(), started, recordErr)
	return res, err
}
func (p *observedProvider) Cancel(ctx context.Context, ref domain.ProviderTaskRef) error {
	started := p.registry.now()
	err := p.provider.Cancel(ctx, ref)
	p.registry.record(p.provider.Name(), started, err)
	return err
}

type routedProvider struct {
	registry   *Registry
	candidates []providerCandidate
}

func (p *routedProvider) Name() domain.ProviderName {
	if len(p.candidates) == 0 {
		return ""
	}
	return p.candidates[0].name
}
func (p *routedProvider) Capabilities(ctx context.Context) ([]domain.Capability, error) {
	if len(p.candidates) == 0 {
		return nil, errors.New("worker: no routed provider candidates")
	}
	return p.candidates[0].provider.Capabilities(ctx)
}
func (p *routedProvider) Estimate(ctx context.Context, req domain.ProviderRequest) (domain.CostEstimate, error) {
	if len(p.candidates) == 0 {
		return domain.CostEstimate{}, errors.New("worker: no routed provider candidates")
	}
	return p.candidates[0].provider.Estimate(ctx, req)
}
func (p *routedProvider) Submit(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	var last error
	for _, candidate := range p.candidates {
		started := p.registry.now()
		task, err := candidate.provider.Submit(ctx, req)
		p.registry.record(candidate.name, started, err)
		if err == nil {
			if task.Provider == "" {
				task.Provider = candidate.name
			}
			return task, nil
		}
		last = err
		if !isFallbackError(err) {
			break
		}
	}
	if last == nil {
		last = errors.New("worker: no routed provider candidates")
	}
	return domain.ProviderTask{}, last
}
func (p *routedProvider) Poll(ctx context.Context, ref domain.ProviderTaskRef) (domain.ProviderTaskResult, error) {
	provider, err := p.registry.ForName(ref.Provider)
	if err != nil {
		return domain.ProviderTaskResult{}, err
	}
	return provider.Poll(ctx, ref)
}
func (p *routedProvider) Cancel(ctx context.Context, ref domain.ProviderTaskRef) error {
	provider, err := p.registry.ForName(ref.Provider)
	if err != nil {
		return err
	}
	return provider.Cancel(ctx, ref)
}

func isFallbackError(err error) bool {
	class := classOf(err)
	return isRetryable(class) || class == domain.ProviderErrUnsupportedCapab
}

type providerResultError struct {
	class   domain.ProviderErrorClass
	message string
}

func (e providerResultError) Error() string {
	if e.message != "" {
		return e.message
	}
	return string(e.class)
}

func (e providerResultError) ProviderErrorClass() domain.ProviderErrorClass { return e.class }

// processor holds the shared dependencies and result-handling logic used by
// both the generation and poll workers.
type processor struct {
	jobs         domain.JobRepository
	tasks        domain.ProviderTaskRepository
	artifacts    ArtifactSaver
	artifactRepo domain.ArtifactRepository
	objects      ObjectStore
	providers    *Registry
	streams      StreamPublisher
	imageModel   string
	imageSize    string
	textContext  TextContext
	moderator    Moderator
	modResults   domain.ModerationResultRepository
	releaser     ReservationReleaser
	maxAttempts  int
	backoff      func(attempt int) time.Duration
	callTimeout  time.Duration
	now          func() time.Time
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

// TextContext prepares compact dialog context for text jobs and records
// assistant answers after provider success.
type TextContext interface {
	Prepare(ctx context.Context, job *domain.Job, prompt string) (dialogcontext.Prepared, error)
	Complete(ctx context.Context, job *domain.Job, conversationID uuid.UUID, answer string) error
}

// Deps bundles the dependencies shared by the workers.
type Deps struct {
	Jobs      domain.JobRepository
	Tasks     domain.ProviderTaskRepository
	Artifacts ArtifactSaver
	// ArtifactRepo loads input artifact metadata for provider request assembly.
	ArtifactRepo domain.ArtifactRepository
	// Objects loads input artifact bytes for provider request assembly.
	Objects   ObjectStore
	Providers *Registry
	Streams   StreamPublisher
	// ImageModel/ImageSize are optional product-level defaults for image jobs.
	// They are translated into the provider request only inside workers.
	ImageModel string
	ImageSize  string
	// TextContext, when set, stores VK text dialog history and renders compact
	// provider prompts for text jobs.
	TextContext TextContext
	// Moderator, when set, runs an output moderation check before delivery.
	// When nil, moderation is skipped (allow-all) for local/test wiring.
	Moderator Moderator
	// ModResults, when set, persists moderation verdicts for audit.
	ModResults domain.ModerationResultRepository
	// Releaser, when set, frees reserved credits for moderation-blocked jobs
	// and terminal provider failures before capture.
	Releaser ReservationReleaser
	// MaxAttempts caps retryable re-enqueues before dead-lettering (default 3).
	MaxAttempts int
	// Backoff returns the delay before re-enqueue for the given attempt number.
	// Defaults to no delay (keeps tests fast).
	Backoff func(attempt int) time.Duration
	// ProviderCallTimeout bounds one provider Submit/Poll call (default 60s).
	ProviderCallTimeout time.Duration
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
	callTimeout := d.ProviderCallTimeout
	if callTimeout <= 0 {
		callTimeout = defaultProviderCallTimeout
	}
	return processor{
		jobs:         d.Jobs,
		tasks:        d.Tasks,
		artifacts:    d.Artifacts,
		artifactRepo: d.ArtifactRepo,
		objects:      d.Objects,
		providers:    d.Providers,
		streams:      d.Streams,
		imageModel:   d.ImageModel,
		imageSize:    d.ImageSize,
		textContext:  d.TextContext,
		moderator:    d.Moderator,
		modResults:   d.ModResults,
		releaser:     d.Releaser,
		maxAttempts:  maxAttempts,
		backoff:      backoff,
		callTimeout:  callTimeout,
		now:          now,
	}
}

// maxReferenceArtifacts must match miniapp/references.go limit.
const maxReferenceArtifacts = 4

const maxReferenceArtifactBytes = 20 << 20

// promptParams is the subset of job params the provider request needs.
type promptParams struct {
	Prompt                 string      `json:"prompt"`
	NegativePrompt         string      `json:"negative_prompt"`
	ModelCode              string      `json:"model_code,omitempty"`
	Size                   string      `json:"size,omitempty"`
	AspectRatio            string      `json:"aspect_ratio,omitempty"`
	ReferenceArtifactIDs   []uuid.UUID `json:"reference_artifact_ids,omitempty"`
	InputURLs              []string    `json:"input_urls,omitempty"`
	VKPlaceholderMessageID int64       `json:"vk_placeholder_message_id,omitempty"`
	ConversationID         string      `json:"conversation_id,omitempty"`
	ConversationSource     string      `json:"conversation_source,omitempty"`
	ExternalThreadID       string      `json:"external_thread_id,omitempty"`
}

// buildRequest builds the normalized provider request for a job. The submit
// idempotency key is scoped to the attempt so a re-delivered task maps to one
// provider task, while a genuine retry after failure starts a fresh one.
func (p *processor) buildRequest(ctx context.Context, job *domain.Job, attempt int) (domain.ProviderRequest, error) {
	var pp promptParams
	if len(job.Params) > 0 {
		_ = json.Unmarshal(job.Params, &pp)
	}
	prompt := pp.Prompt
	modelCode := pp.ModelCode
	size := pp.Size
	maxOutputTokens := 0
	if job.Modality == domain.ModalityImage {
		if modelCode == "" {
			modelCode = p.imageModel
		}
		if size == "" {
			size = p.imageSize
		}
	}
	if p.textContext != nil && job.OperationType == domain.OperationTextGenerate && job.Modality == domain.ModalityText {
		prepared, err := p.textContext.Prepare(ctx, job, pp.Prompt)
		if err != nil {
			return domain.ProviderRequest{}, err
		}
		if prepared.Prompt != "" {
			prompt = prepared.Prompt
		}
		maxOutputTokens = prepared.MaxOutputTokens
		if prepared.ConversationID != uuid.Nil && pp.ConversationID != prepared.ConversationID.String() {
			pp.ConversationID = prepared.ConversationID.String()
			if raw, err := json.Marshal(pp); err == nil {
				job.Params = raw
				if err := p.jobs.Update(ctx, job); err != nil {
					return domain.ProviderRequest{}, err
				}
			}
		}
	}
	var inputURLs []string
	if job.Modality == domain.ModalityImage && len(pp.ReferenceArtifactIDs) > 0 {
		var err error
		inputURLs, err = p.resolveReferenceInputURLs(ctx, job, pp.ReferenceArtifactIDs)
		if err != nil {
			return domain.ProviderRequest{}, err
		}
	}
	return domain.ProviderRequest{
		JobID:                job.ID,
		UserID:               job.UserID,
		Operation:            job.OperationType,
		Modality:             job.Modality,
		ModelCode:            modelCode,
		Prompt:               prompt,
		NegativePrompt:       pp.NegativePrompt,
		Size:                 size,
		AspectRatio:          pp.AspectRatio,
		ReferenceArtifactIDs: pp.ReferenceArtifactIDs,
		InputURLs:            inputURLs,
		Params:               stripProviderInputURLs(job.Params),
		MaxOutputTokens:      maxOutputTokens,
		IdempotencyKey:       fmt.Sprintf("provider_submit:%s:%d", job.ID, attempt),
	}, nil
}

func stripProviderInputURLs(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var params map[string]json.RawMessage
	if err := json.Unmarshal(raw, &params); err != nil {
		return raw
	}
	if _, ok := params["input_urls"]; !ok {
		return raw
	}
	delete(params, "input_urls")
	out, err := json.Marshal(params)
	if err != nil {
		return raw
	}
	return out
}

func (p *processor) resolveReferenceInputURLs(ctx context.Context, job *domain.Job, ids []uuid.UUID) ([]string, error) {
	if len(ids) > maxReferenceArtifacts {
		return nil, fmt.Errorf("worker: too many reference artifacts")
	}
	if p.artifactRepo == nil {
		return nil, fmt.Errorf("worker: artifact repository unavailable")
	}
	if p.objects == nil {
		return nil, fmt.Errorf("worker: object store unavailable")
	}
	inputURLs := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			return nil, fmt.Errorf("worker: invalid reference artifact id")
		}
		artifact, err := p.artifactRepo.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("worker: reference artifact not found: %w", err)
		}
		if artifact.OwnerUserID != job.UserID {
			return nil, fmt.Errorf("worker: invalid reference artifact owner")
		}
		if artifact.Kind != domain.ArtifactKindInput || artifact.MediaType != domain.MediaTypeImage || artifact.Status != domain.ArtifactStatusReady {
			return nil, fmt.Errorf("worker: invalid reference artifact")
		}
		if artifact.StorageBucket == "" || artifact.StorageKey == "" {
			return nil, fmt.Errorf("worker: reference artifact storage missing")
		}
		data, err := p.objects.GetObject(ctx, artifact.StorageBucket, artifact.StorageKey)
		if err != nil {
			return nil, fmt.Errorf("worker: reference artifact object missing: %w", err)
		}
		if len(data) > maxReferenceArtifactBytes {
			return nil, fmt.Errorf("worker: reference artifact too large")
		}
		mime := artifact.MimeType
		if mime == "" {
			mime = http.DetectContentType(data)
		}
		inputURLs = append(inputURLs, "data:"+mime+";base64,"+base64.StdEncoding.EncodeToString(data))
	}
	return inputURLs, nil
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
	if errors.Is(err, context.DeadlineExceeded) {
		return domain.ProviderErrTimeout
	}
	return domain.ProviderErrInternal
}

func (p *processor) providerCallContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if p.callTimeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, p.callTimeout)
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
	callCtx, cancel := p.providerCallContext(ctx)
	pollCtx, span := tracing.Start(callCtx, "provider.poll",
		attribute.String("job.id", job.ID.String()),
		attribute.String("provider", string(provider.Name())),
		attribute.String("provider.external_id", pt.ExternalID),
		tracing.CorrelationAttr(job.CorrelationID),
	)
	res, err := provider.Poll(pollCtx, domain.ProviderTaskRef{Provider: pt.Provider, ExternalID: pt.ExternalID})
	if err != nil {
		tracing.RecordError(span, err)
		span.End()
		cancel()
		return p.handleFailure(ctx, job, task, classOf(err), err.Error())
	}
	span.SetAttributes(attribute.String("provider.task_status", string(res.Status)))
	span.End()
	cancel()
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
		if err := p.saveDialogAnswer(ctx, job, res.Text); err != nil {
			slog.WarnContext(ctx, "dialog context answer save failed",
				slog.String("job_id", job.ID.String()),
				slog.String("error", err.Error()))
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

func (p *processor) saveDialogAnswer(ctx context.Context, job *domain.Job, answer string) error {
	if p.textContext == nil || answer == "" || job.OperationType != domain.OperationTextGenerate || job.Modality != domain.ModalityText {
		return nil
	}
	var pp promptParams
	if len(job.Params) > 0 {
		_ = json.Unmarshal(job.Params, &pp)
	}
	if pp.ConversationID == "" {
		return nil
	}
	conversationID, err := uuid.Parse(pp.ConversationID)
	if err != nil {
		return nil
	}
	return p.textContext.Complete(ctx, job, conversationID, answer)
}

// saveOutputs stores each provider output URL as an output artifact and records
// the artifact ids on the job, skipping ids already attached (idempotent).
func (p *processor) saveOutputs(ctx context.Context, job *domain.Job, urls []string) error {
	ctx, span := tracing.Start(ctx, "artifact.save_outputs",
		attribute.String("job.id", job.ID.String()),
		attribute.String("modality", string(job.Modality)),
		attribute.Int("artifact.output_count", len(urls)),
		tracing.CorrelationAttr(job.CorrelationID),
	)
	defer span.End()

	mediaType := mediaTypeFor(job.Modality)
	for _, url := range urls {
		art, err := p.artifacts.SaveRemoteArtifact(ctx, job.UserID, &job.ID, domain.ArtifactKindOutput, mediaType, url)
		if err != nil {
			tracing.RecordError(span, err)
			return err
		}
		if !containsID(job.OutputArtifactIDs, art.ID) {
			job.OutputArtifactIDs = append(job.OutputArtifactIDs, art.ID)
		}
	}
	if err := p.jobs.Update(ctx, job); err != nil {
		tracing.RecordError(span, err)
		return err
	}
	return nil
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
	if err := p.releaseReserved(ctx, job); err != nil {
		return err
	}
	if isRetryable(class) {
		p.toDLQ(ctx, task, code, msg)
	}
	metrics.JobsTerminal.WithLabelValues(string(domain.JobStatusFailedTerminal)).Inc()
	if err := p.setStatus(ctx, job, domain.JobStatusFailedTerminal, code, msg); err != nil {
		return err
	}
	if p.shouldNotifyTerminalProviderFailure(job) {
		return p.streams.PublishTo(ctx, redisqueue.StreamDelivery, taskOf(job))
	}
	return nil
}

func (p *processor) releaseReserved(ctx context.Context, job *domain.Job) error {
	if p.releaser == nil || job.CostReserved <= 0 || job.CostCaptured > 0 {
		return nil
	}
	return p.releaser.ReleaseForJob(ctx, job.ID)
}

func (p *processor) shouldNotifyTerminalProviderFailure(job *domain.Job) bool {
	return p.streams != nil && job.VKPeerID != 0 && job.Modality == domain.ModalityImage
}

// moderateOutput runs the output moderation check and, on a block, rejects the
// job (no delivery, no capture), releases the reservation and records an audit
// verdict. It returns blocked=true when delivery must be stopped. When no
// moderator is configured it is a no-op (allow).
func (p *processor) moderateOutput(ctx context.Context, job *domain.Job) (bool, error) {
	if p.moderator == nil {
		return false, nil
	}
	ctx, span := tracing.Start(ctx, "moderation.output",
		attribute.String("job.id", job.ID.String()),
		attribute.String("modality", string(job.Modality)),
		tracing.CorrelationAttr(job.CorrelationID),
	)
	defer span.End()

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
		tracing.RecordError(span, err)
		return false, err
	}
	span.SetAttributes(attribute.String("moderation.decision", string(out.Decision)))

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
		tracing.RecordError(span, err)
		return false, err
	}
	metrics.JobsTerminal.WithLabelValues(string(domain.JobStatusRejected)).Inc()
	if p.releaser != nil {
		if err := p.releaser.ReleaseForJob(ctx, job.ID); err != nil {
			tracing.RecordError(span, err)
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
