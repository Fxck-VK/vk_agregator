// Package mock provides an in-memory implementation of domain.Provider used for
// tests and local development. It deterministically simulates the asynchronous
// provider task lifecycle (submit -> poll -> succeed) and can inject the
// normalized error classes the rest of the system must handle.
package mock

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// Trigger keywords: when a prompt contains one of these, Submit/Poll simulate
// the corresponding failure instead of succeeding.
const (
	TriggerTimeout       = "mock_timeout"
	TriggerRateLimit     = "mock_rate_limit"
	TriggerProviderError = "mock_provider_error"
)

// triggerClasses maps a trigger keyword to the normalized error class it raises.
var triggerClasses = map[string]domain.ProviderErrorClass{
	TriggerTimeout:       domain.ProviderErrTimeout,
	TriggerRateLimit:     domain.ProviderErrRateLimited,
	TriggerProviderError: domain.ProviderErrInternal,
}

// estimates is the simulated credit cost per supported operation.
var estimates = map[domain.OperationType]int64{
	domain.OperationTextGenerate:  1,
	domain.OperationImageGenerate: 10,
	domain.OperationVideoGenerate: 50,
}

// Error is a provider failure carrying a normalized error class.
type Error struct {
	Class   domain.ProviderErrorClass
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("mock provider: %s: %s", e.Class, e.Message) }

// ProviderErrorClass exposes the normalized error class so callers (workers)
// can classify the failure without depending on this package's concrete type.
func (e *Error) ProviderErrorClass() domain.ProviderErrorClass { return e.Class }

// taskState tracks one submitted task across Poll calls.
type taskState struct {
	jobID     uuid.UUID
	operation domain.OperationType
	modality  domain.Modality
	text      string
	errClass  domain.ProviderErrorClass
	polls     int
	cancelled bool
}

// Provider is the mock domain.Provider.
type Provider struct {
	mu sync.Mutex
	// completeAfterPolls is the number of Poll calls before a task succeeds.
	completeAfterPolls int
	tasks              map[string]*taskState
	now                func() time.Time
}

// Option customizes a Provider.
type Option func(*Provider)

// WithCompleteAfterPolls sets how many Poll calls a task needs before it
// reports success (default 1).
func WithCompleteAfterPolls(n int) Option {
	return func(p *Provider) { p.completeAfterPolls = n }
}

// New builds a mock Provider.
func New(opts ...Option) *Provider {
	p := &Provider{
		completeAfterPolls: 1,
		tasks:              map[string]*taskState{},
		now:                time.Now,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

var _ domain.Provider = (*Provider)(nil)

// Name returns the mock provider identifier.
func (p *Provider) Name() domain.ProviderName { return domain.ProviderMock }

// Capabilities reports the operations the mock supports.
func (p *Provider) Capabilities(_ context.Context) ([]domain.Capability, error) {
	return []domain.Capability{
		{Operation: domain.OperationTextGenerate, Modality: domain.ModalityText, ModelCode: "mock-text", SupportsPolling: true},
		// Image model is intentionally wildcarded so local/dev image jobs can
		// carry a future real model code while still falling back to mock.
		{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, SupportsPolling: true},
		{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, ModelCode: "mock-video", SupportsPolling: true, MaxDurationSec: 10},
	}, nil
}

// Estimate returns the simulated cost of a request.
func (p *Provider) Estimate(_ context.Context, req domain.ProviderRequest) (domain.CostEstimate, error) {
	amount, ok := estimates[req.Operation]
	if !ok {
		return domain.CostEstimate{}, &Error{Class: domain.ProviderErrUnsupportedCapab, Message: string(req.Operation)}
	}
	return domain.CostEstimate{AmountCredits: amount, Currency: "credits", Estimated: true}, nil
}

// Submit registers a task. Unsupported operations fail immediately; supported
// ones return a pending task whose later outcome depends on the prompt triggers.
func (p *Provider) Submit(_ context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	if _, ok := estimates[req.Operation]; !ok {
		return domain.ProviderTask{}, &Error{Class: domain.ProviderErrUnsupportedCapab, Message: string(req.Operation)}
	}

	externalID := "mock-" + uuid.NewString()
	state := &taskState{
		jobID:     req.JobID,
		operation: req.Operation,
		modality:  req.Modality,
		text:      textOutput(externalID, req.Modality),
		errClass:  triggerFor(req.Prompt),
	}

	p.mu.Lock()
	p.tasks[externalID] = state
	p.mu.Unlock()

	now := p.now()
	return domain.ProviderTask{
		JobID:          req.JobID,
		Provider:       domain.ProviderMock,
		ModelCode:      req.ModelCode,
		ExternalID:     externalID,
		AttemptNo:      1,
		Status:         domain.ProviderTaskPending,
		Request:        req.Params,
		SubmittedAt:    &now,
		ErrorClass:     "",
		CreatedAt:      now,
		UpdatedAt:      now,
		IdempotencyKey: req.IdempotencyKey,
	}, nil
}

// Poll advances the task lifecycle and returns its normalized status/result.
func (p *Provider) Poll(_ context.Context, ref domain.ProviderTaskRef) (domain.ProviderTaskResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	state, ok := p.tasks[ref.ExternalID]
	if !ok {
		return domain.ProviderTaskResult{
			Status:     domain.ProviderTaskFailed,
			ErrorClass: domain.ProviderErrTaskNotFound,
		}, nil
	}
	if state.cancelled {
		return domain.ProviderTaskResult{Status: domain.ProviderTaskCancelled}, nil
	}
	if state.errClass != "" {
		return domain.ProviderTaskResult{
			Status:       domain.ProviderTaskFailed,
			ErrorClass:   state.errClass,
			ErrorMessage: "simulated " + string(state.errClass),
		}, nil
	}

	state.polls++
	if state.polls < p.completeAfterPolls {
		return domain.ProviderTaskResult{Status: domain.ProviderTaskProcessing}, nil
	}
	return domain.ProviderTaskResult{
		Status:     domain.ProviderTaskSucceeded,
		OutputURLs: []string{outputURL(ref.ExternalID, state.modality)},
		Text:       state.text,
	}, nil
}

// Cancel marks a task cancelled. Unknown tasks are treated as a no-op.
func (p *Provider) Cancel(_ context.Context, ref domain.ProviderTaskRef) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if state, ok := p.tasks[ref.ExternalID]; ok {
		state.cancelled = true
	}
	return nil
}

func triggerFor(prompt string) domain.ProviderErrorClass {
	lower := strings.ToLower(prompt)
	for keyword, class := range triggerClasses {
		if strings.Contains(lower, keyword) {
			return class
		}
	}
	return ""
}

func outputURL(externalID string, modality domain.Modality) string {
	ext := "bin"
	switch modality {
	case domain.ModalityText:
		ext = "txt"
	case domain.ModalityImage:
		ext = "png"
	case domain.ModalityVideo:
		ext = "mp4"
	case domain.ModalityAudio:
		ext = "mp3"
	}
	return fmt.Sprintf("mock://%s/output.%s", externalID, ext)
}

func textOutput(externalID string, modality domain.Modality) string {
	if modality != domain.ModalityText {
		return ""
	}
	return "Mock generated text result.\nsource=mock://" + externalID + "/output.txt\n"
}
