// Package openai is a production-ready domain.Provider adapter for OpenAI. It
// is wired only when PROVIDER=openai and OPENAI_API_KEY is set; the default
// runtime still uses the mock provider, so no real key is required for local
// development or CI (audit P1).
//
// The adapter currently implements image generation through the OpenAI Images
// API, which returns a hosted https URL that fits the existing
// download-and-store artifact pipeline. Text and video providers follow the
// same Submit/Poll/normalize shape and can be added without touching the worker
// flow.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// Config holds the OpenAI connection settings.
type Config struct {
	// APIKey authenticates requests (Bearer token). Required.
	APIKey string
	// BaseURL is the API root, e.g. https://api.openai.com/v1.
	BaseURL string
	// ImageModel is the model used for image generation (e.g. gpt-image-1).
	ImageModel string
	// ImageSize is the requested image size (e.g. 1024x1024).
	ImageSize string
	// ImagePrice is the credit cost charged per image generation.
	ImagePrice int64
	// HTTPClient overrides the HTTP client (mainly for tests). Optional.
	HTTPClient *http.Client
}

// Provider is the OpenAI domain.Provider adapter.
type Provider struct {
	cfg   Config
	http  *http.Client
	mu    sync.Mutex
	tasks map[string]taskState
	now   func() time.Time
}

type taskState struct {
	outputURL string
	errClass  domain.ProviderErrorClass
	errMsg    string
}

// New builds an OpenAI provider from cfg, applying sensible defaults.
func New(cfg Config) *Provider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.ImageModel == "" {
		cfg.ImageModel = "gpt-image-1"
	}
	if cfg.ImageSize == "" {
		cfg.ImageSize = "1024x1024"
	}
	if cfg.ImagePrice == 0 {
		cfg.ImagePrice = 10
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	return &Provider{
		cfg:   cfg,
		http:  httpClient,
		tasks: map[string]taskState{},
		now:   time.Now,
	}
}

var _ domain.Provider = (*Provider)(nil)

// Name returns the OpenAI provider identifier.
func (p *Provider) Name() domain.ProviderName { return domain.ProviderOpenAI }

// Capabilities reports the operations this adapter supports.
func (p *Provider) Capabilities(_ context.Context) ([]domain.Capability, error) {
	return []domain.Capability{
		{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ModelCode: p.cfg.ImageModel, SupportsPolling: true},
	}, nil
}

// Estimate returns the configured credit cost for the operation.
func (p *Provider) Estimate(_ context.Context, req domain.ProviderRequest) (domain.CostEstimate, error) {
	if req.Operation != domain.OperationImageGenerate {
		return domain.CostEstimate{}, &Error{Class: domain.ProviderErrUnsupportedCapab, Message: string(req.Operation)}
	}
	return domain.CostEstimate{AmountCredits: p.cfg.ImagePrice, Currency: "credits", Estimated: false}, nil
}

type imageRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	N      int    `json:"n"`
	Size   string `json:"size"`
}

type imageResponse struct {
	Data []struct {
		URL string `json:"url"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// Submit calls the OpenAI Images API and caches the normalized result so Poll
// can return it. The synchronous OpenAI call is wrapped in the platform's
// async submit/poll contract.
func (p *Provider) Submit(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	if req.Operation != domain.OperationImageGenerate {
		return domain.ProviderTask{}, &Error{Class: domain.ProviderErrUnsupportedCapab, Message: string(req.Operation)}
	}

	externalID := "openai-" + uuid.NewString()
	now := p.now()
	state := taskState{}

	url, err := p.generateImage(ctx, req.Prompt)
	if err != nil {
		var perr *Error
		if as, ok := err.(*Error); ok {
			perr = as
		} else {
			perr = &Error{Class: domain.ProviderErrInternal, Message: err.Error()}
		}
		// Record the failure so Poll surfaces the normalized error class.
		state.errClass = perr.Class
		state.errMsg = perr.Message
		p.store(externalID, state)
		return domain.ProviderTask{}, perr
	}
	state.outputURL = url
	p.store(externalID, state)

	return domain.ProviderTask{
		JobID:          req.JobID,
		Provider:       domain.ProviderOpenAI,
		ModelCode:      p.cfg.ImageModel,
		ExternalID:     externalID,
		AttemptNo:      1,
		Status:         domain.ProviderTaskProcessing,
		Request:        req.Params,
		SubmittedAt:    &now,
		CreatedAt:      now,
		UpdatedAt:      now,
		IdempotencyKey: req.IdempotencyKey,
	}, nil
}

// Poll returns the cached normalized result for a submitted task.
func (p *Provider) Poll(_ context.Context, ref domain.ProviderTaskRef) (domain.ProviderTaskResult, error) {
	p.mu.Lock()
	state, ok := p.tasks[ref.ExternalID]
	p.mu.Unlock()
	if !ok {
		return domain.ProviderTaskResult{Status: domain.ProviderTaskFailed, ErrorClass: domain.ProviderErrTaskNotFound}, nil
	}
	if state.errClass != "" {
		return domain.ProviderTaskResult{Status: domain.ProviderTaskFailed, ErrorClass: state.errClass, ErrorMessage: state.errMsg}, nil
	}
	return domain.ProviderTaskResult{Status: domain.ProviderTaskSucceeded, OutputURLs: []string{state.outputURL}}, nil
}

// Cancel is a no-op: image generation completes synchronously on submit.
func (p *Provider) Cancel(_ context.Context, _ domain.ProviderTaskRef) error { return nil }

func (p *Provider) store(externalID string, state taskState) {
	p.mu.Lock()
	p.tasks[externalID] = state
	p.mu.Unlock()
}

func (p *Provider) generateImage(ctx context.Context, prompt string) (string, error) {
	body, err := json.Marshal(imageRequest{Model: p.cfg.ImageModel, Prompt: prompt, N: 1, Size: p.cfg.ImageSize})
	if err != nil {
		return "", &Error{Class: domain.ProviderErrInvalidRequest, Message: err.Error()}
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return "", &Error{Class: domain.ProviderErrInternal, Message: err.Error()}
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.http.Do(httpReq)
	if err != nil {
		return "", &Error{Class: domain.ProviderErrTimeout, Message: err.Error()}
	}
	defer resp.Body.Close()

	var decoded imageResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", &Error{Class: domain.ProviderErrInternal, Message: "decode response: " + err.Error()}
	}
	if resp.StatusCode != http.StatusOK {
		return "", &Error{Class: classifyStatus(resp.StatusCode), Message: errorMessage(decoded, resp.StatusCode)}
	}
	if len(decoded.Data) == 0 || decoded.Data[0].URL == "" {
		return "", &Error{Class: domain.ProviderErrInternal, Message: "empty image response"}
	}
	return decoded.Data[0].URL, nil
}

func errorMessage(r imageResponse, status int) string {
	if r.Error != nil && r.Error.Message != "" {
		return r.Error.Message
	}
	return fmt.Sprintf("openai http %d", status)
}

// classifyStatus maps an HTTP status onto the normalized provider error class
// that drives the retry policy (invariant #11).
func classifyStatus(status int) domain.ProviderErrorClass {
	switch {
	case status == http.StatusTooManyRequests:
		return domain.ProviderErrRateLimited
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return domain.ProviderErrAuthFailed
	case status == http.StatusBadRequest || status == http.StatusUnprocessableEntity:
		return domain.ProviderErrInvalidRequest
	case status >= 500:
		return domain.ProviderErrOverloaded
	default:
		return domain.ProviderErrInternal
	}
}

// Error is an OpenAI failure carrying a normalized error class so workers can
// classify it without importing this package's internals.
type Error struct {
	Class   domain.ProviderErrorClass
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("openai provider: %s: %s", e.Class, e.Message) }

// ProviderErrorClass exposes the normalized class for worker classification.
func (e *Error) ProviderErrorClass() domain.ProviderErrorClass { return e.Class }
