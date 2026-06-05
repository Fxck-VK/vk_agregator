package domain

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ProviderName is the stable code identifying an external AI provider, e.g.
// "openai", "google", "kling". It is used to look up the right adapter.
type ProviderName string

const (
	// ProviderOpenAI is the OpenAI provider.
	ProviderOpenAI ProviderName = "openai"
	// ProviderDeepInfra is the DeepInfra provider.
	ProviderDeepInfra ProviderName = "deepinfra"
	// ProviderGoogle is the Google Gemini provider.
	ProviderGoogle ProviderName = "google"
	// ProviderKling is the Kling video provider.
	ProviderKling ProviderName = "kling"
	// ProviderRunway is the Runway video provider.
	ProviderRunway ProviderName = "runway"
	// ProviderMock is an in-memory provider used for tests.
	ProviderMock ProviderName = "mock"
)

// ProviderTaskStatus is the normalized status of a provider-side task. Each
// provider's native statuses are mapped onto this enum (invariant: every
// provider response is normalized).
type ProviderTaskStatus string

const (
	// ProviderTaskPending means the task is accepted but not started.
	ProviderTaskPending ProviderTaskStatus = "pending"
	// ProviderTaskProcessing means the provider is actively working.
	ProviderTaskProcessing ProviderTaskStatus = "processing"
	// ProviderTaskSucceeded means the task finished with output.
	ProviderTaskSucceeded ProviderTaskStatus = "succeeded"
	// ProviderTaskFailed means the task failed on the provider side.
	ProviderTaskFailed ProviderTaskStatus = "failed"
	// ProviderTaskCancelled means the task was cancelled.
	ProviderTaskCancelled ProviderTaskStatus = "cancelled"
)

// IsTerminal reports whether the provider task status is final.
func (s ProviderTaskStatus) IsTerminal() bool {
	switch s {
	case ProviderTaskSucceeded, ProviderTaskFailed, ProviderTaskCancelled:
		return true
	default:
		return false
	}
}

// ProviderErrorClass is the normalized error taxonomy that every provider
// failure maps onto (invariant #11). It drives the retry/fallback policy.
type ProviderErrorClass string

const (
	ProviderErrRateLimited          ProviderErrorClass = "rate_limited"
	ProviderErrAuthFailed           ProviderErrorClass = "auth_failed"
	ProviderErrInsufficientBalance  ProviderErrorClass = "insufficient_provider_balance"
	ProviderErrInvalidRequest       ProviderErrorClass = "invalid_request"
	ProviderErrContentRejected      ProviderErrorClass = "content_rejected"
	ProviderErrOverloaded           ProviderErrorClass = "provider_overloaded"
	ProviderErrTimeout              ProviderErrorClass = "provider_timeout"
	ProviderErrInternal             ProviderErrorClass = "provider_internal_error"
	ProviderErrTaskNotFound         ProviderErrorClass = "task_not_found"
	ProviderErrOutputDownloadFailed ProviderErrorClass = "output_download_failed"
	ProviderErrUnsupportedCapab     ProviderErrorClass = "unsupported_capability"
)

// ProviderRequest is the normalized, provider-agnostic description of a single
// generation request. The adapter translates it into the provider's native API
// shape. It must never contain VK- or billing-specific concerns.
type ProviderRequest struct {
	// JobID is the originating job, used for correlation and idempotency.
	JobID uuid.UUID `json:"job_id"`
	// Operation is the operation the provider must perform.
	Operation OperationType `json:"operation"`
	// Modality is the content kind of the request.
	Modality Modality `json:"modality"`
	// ModelCode is the provider-specific model code (e.g. "kling-v2").
	ModelCode string `json:"model_code"`
	// Prompt is the final, fully-rendered user/system prompt.
	Prompt string `json:"prompt"`
	// NegativePrompt is the optional negative prompt for image/video models.
	NegativePrompt string `json:"negative_prompt,omitempty"`
	// InputURLs are signed URLs of input artifacts the provider may fetch.
	InputURLs []string `json:"input_urls,omitempty"`
	// Params holds operation-specific tuning (aspect_ratio, duration, seed...).
	Params json.RawMessage `json:"params,omitempty"`
	// IdempotencyKey makes the submit safe to retry.
	IdempotencyKey string `json:"idempotency_key"`
}

// ProviderTaskRef is the minimal reference needed to poll or cancel a task on a
// provider without carrying the full task record.
type ProviderTaskRef struct {
	// Provider identifies which adapter owns the task.
	Provider ProviderName `json:"provider"`
	// ExternalID is the provider-assigned task identifier.
	ExternalID string `json:"external_id"`
}

// CostEstimate is the normalized cost prediction for a provider request,
// expressed in internal credits.
type CostEstimate struct {
	// AmountCredits is the predicted cost in internal credits.
	AmountCredits int64 `json:"amount_credits"`
	// Currency is the unit of the estimate, normally "credits".
	Currency string `json:"currency"`
	// Estimated marks whether the value is a guess (true) or fixed price.
	Estimated bool `json:"estimated"`
}

// Capability describes a single operation a provider/model can perform. It is
// used by the provider router to decide where a request can go.
type Capability struct {
	// Operation is the supported operation.
	Operation OperationType `json:"operation"`
	// Modality is the operation's content kind.
	Modality Modality `json:"modality"`
	// ModelCode is the provider-specific model code that offers it.
	ModelCode string `json:"model_code"`
	// SupportsWebhook reports whether completion is delivered via webhook.
	SupportsWebhook bool `json:"supports_webhook"`
	// SupportsPolling reports whether completion can be polled.
	SupportsPolling bool `json:"supports_polling"`
	// MaxDurationSec is the max video/audio duration; 0 if not applicable.
	MaxDurationSec int `json:"max_duration_sec"`
}

// ProviderTaskResult holds the normalized output of a finished provider task.
type ProviderTaskResult struct {
	// Status is the normalized terminal/intermediate status.
	Status ProviderTaskStatus `json:"status"`
	// OutputURLs are URLs of produced artifacts to be downloaded and stored.
	OutputURLs []string `json:"output_urls,omitempty"`
	// ErrorClass is set when Status is failed.
	ErrorClass ProviderErrorClass `json:"error_class,omitempty"`
	// ErrorMessage is a human-readable failure description.
	ErrorMessage string `json:"error_message,omitempty"`
	// Raw is the untouched provider payload for audit (no secrets).
	Raw json.RawMessage `json:"raw,omitempty"`
}

// ProviderTask is the persisted record of one submission to an external
// provider. It lets the platform poll, cancel and reconcile asynchronously.
type ProviderTask struct {
	// ID is the internal primary key.
	ID uuid.UUID `json:"id"`
	// JobID is the job this task belongs to.
	JobID uuid.UUID `json:"job_id"`
	// Provider is the provider that owns the task.
	Provider ProviderName `json:"provider"`
	// ModelCode is the provider-specific model used.
	ModelCode string `json:"model_code"`
	// ExternalID is the provider-assigned task id, empty until submitted.
	ExternalID string `json:"external_id"`
	// AttemptNo is the submission attempt number, starting at 1.
	AttemptNo int `json:"attempt_no"`
	// Status is the normalized provider task status.
	Status ProviderTaskStatus `json:"status"`
	// Request is the normalized request that was submitted.
	Request json.RawMessage `json:"request"`
	// Result is the normalized result once available.
	Result json.RawMessage `json:"result,omitempty"`
	// ErrorClass is the normalized error class on failure.
	ErrorClass ProviderErrorClass `json:"error_class,omitempty"`
	// IdempotencyKey makes the submission retry-safe.
	IdempotencyKey string `json:"idempotency_key"`
	// SubmittedAt is when the task was accepted by the provider.
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`
	// CompletedAt is when the task reached a terminal status.
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	// CreatedAt is the row creation timestamp.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the last mutation timestamp.
	UpdatedAt time.Time `json:"updated_at"`
}

// Provider is the unified contract every provider adapter must implement. It
// isolates the rest of the system from provider-specific API details and must
// never reference VK delivery or billing concerns.
type Provider interface {
	// Name returns the stable provider identifier.
	Name() ProviderName

	// Capabilities reports the operations/models the provider currently offers.
	Capabilities(ctx context.Context) ([]Capability, error)

	// Estimate predicts the cost of a request in internal credits.
	Estimate(ctx context.Context, req ProviderRequest) (CostEstimate, error)

	// Submit creates a task on the provider and returns its normalized record.
	// It must be safe to retry under the same idempotency key.
	Submit(ctx context.Context, req ProviderRequest) (ProviderTask, error)

	// Poll fetches the current normalized status/result of a task.
	Poll(ctx context.Context, ref ProviderTaskRef) (ProviderTaskResult, error)

	// Cancel requests cancellation of a task. It is a no-op if already done.
	Cancel(ctx context.Context, ref ProviderTaskRef) error
}
