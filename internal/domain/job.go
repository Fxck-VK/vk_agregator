package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Modality is the kind of content a job operates on. It drives worker pool
// selection and provider routing.
type Modality string

const (
	// ModalityText is plain text generation.
	ModalityText Modality = "text"
	// ModalityImage is still-image generation or editing.
	ModalityImage Modality = "image"
	// ModalityVideo is video generation.
	ModalityVideo Modality = "video"
	// ModalityAudio is audio (TTS, STT, music) generation.
	ModalityAudio Modality = "audio"
)

// Valid reports whether the modality is one of the known modalities.
func (m Modality) Valid() bool {
	switch m {
	case ModalityText, ModalityImage, ModalityVideo, ModalityAudio:
		return true
	default:
		return false
	}
}

// OperationType is the concrete operation requested by the user. It is more
// specific than Modality (e.g. an image modality has both generate and edit).
type OperationType string

const (
	// OperationTextGenerate produces text from a prompt.
	OperationTextGenerate OperationType = "text_generate"
	// OperationImageGenerate produces a new image from a prompt.
	OperationImageGenerate OperationType = "image_generate"
	// OperationImageEdit edits an existing input image.
	OperationImageEdit OperationType = "image_edit"
	// OperationVideoGenerate produces a video from a text prompt.
	OperationVideoGenerate OperationType = "video_generate"
	// OperationVideoImageToVideo animates an input image into a video.
	OperationVideoImageToVideo OperationType = "video_image_to_video"
	// OperationVideoExtend extends the duration of an existing video.
	OperationVideoExtend OperationType = "video_extend"
	// OperationAudioTTS synthesizes speech from text.
	OperationAudioTTS OperationType = "audio_tts"
	// OperationAudioSTT transcribes speech to text.
	OperationAudioSTT OperationType = "audio_stt"
	// OperationImageUpscale increases the resolution of an image.
	OperationImageUpscale OperationType = "image_upscale"
)

// Valid reports whether the operation type is one of the known operations.
func (o OperationType) Valid() bool {
	switch o {
	case OperationTextGenerate,
		OperationImageGenerate,
		OperationImageEdit,
		OperationVideoGenerate,
		OperationVideoImageToVideo,
		OperationVideoExtend,
		OperationAudioTTS,
		OperationAudioSTT,
		OperationImageUpscale:
		return true
	default:
		return false
	}
}

// JobStatus is an explicit state in the job lifecycle state machine. Every
// transition between statuses is intentional and persisted (invariant #6).
type JobStatus string

const (
	// JobStatusReceived means the job was created from a command.
	JobStatusReceived JobStatus = "received"
	// JobStatusValidated means input and quota checks passed.
	JobStatusValidated JobStatus = "validated"
	// JobStatusRejected means the job failed validation/moderation up front.
	JobStatusRejected JobStatus = "rejected"
	// JobStatusAwaitingPayment means the user lacks credits to proceed.
	JobStatusAwaitingPayment JobStatus = "awaiting_payment"
	// JobStatusCreditsReserved means credits were reserved for the job.
	JobStatusCreditsReserved JobStatus = "credits_reserved"
	// JobStatusQueued means the job is waiting in a worker queue.
	JobStatusQueued JobStatus = "queued"
	// JobStatusDispatchingProvider means a worker is preparing the provider call.
	JobStatusDispatchingProvider JobStatus = "dispatching_provider"
	// JobStatusProviderSubmitted means the provider task was created.
	JobStatusProviderSubmitted JobStatus = "provider_submitted"
	// JobStatusProviderPending means the provider accepted but has not started.
	JobStatusProviderPending JobStatus = "provider_pending"
	// JobStatusProviderProcessing means the provider is actively working.
	JobStatusProviderProcessing JobStatus = "provider_processing"
	// JobStatusProviderSucceeded means the provider produced a result.
	JobStatusProviderSucceeded JobStatus = "provider_succeeded"
	// JobStatusProviderFailed means the provider task failed.
	JobStatusProviderFailed JobStatus = "provider_failed"
	// JobStatusPostprocessing means media is being transcoded/packaged.
	JobStatusPostprocessing JobStatus = "postprocessing"
	// JobStatusResultReady means the artifact is ready and moderated.
	JobStatusResultReady JobStatus = "result_ready"
	// JobStatusDelivering means the result is being delivered to VK.
	JobStatusDelivering JobStatus = "delivering"
	// JobStatusSucceeded is the terminal success state.
	JobStatusSucceeded JobStatus = "succeeded"
	// JobStatusFailedRetryable means it failed but may be retried.
	JobStatusFailedRetryable JobStatus = "failed_retryable"
	// JobStatusFailedTerminal is the terminal failure state.
	JobStatusFailedTerminal JobStatus = "failed_terminal"
	// JobStatusCancelled means the user or system cancelled the job.
	JobStatusCancelled JobStatus = "cancelled"
	// JobStatusExpired means the job exceeded its deadline.
	JobStatusExpired JobStatus = "expired"
	// JobStatusRefunded means reserved/captured credits were refunded.
	JobStatusRefunded JobStatus = "refunded"
)

// jobTransitions defines the allowed next states for each job status. It is the
// machine-readable form of the job state machine and is consulted by the
// orchestrator before applying a transition.
var jobTransitions = map[JobStatus][]JobStatus{
	JobStatusReceived:            {JobStatusValidated, JobStatusRejected, JobStatusCancelled},
	JobStatusValidated:           {JobStatusAwaitingPayment, JobStatusCreditsReserved, JobStatusRejected, JobStatusCancelled},
	JobStatusRejected:            {JobStatusRefunded},
	JobStatusAwaitingPayment:     {JobStatusCreditsReserved, JobStatusCancelled, JobStatusExpired},
	JobStatusCreditsReserved:     {JobStatusQueued, JobStatusCancelled, JobStatusRefunded},
	JobStatusQueued:              {JobStatusDispatchingProvider, JobStatusCancelled, JobStatusExpired},
	JobStatusDispatchingProvider: {JobStatusProviderSubmitted, JobStatusFailedRetryable, JobStatusFailedTerminal},
	JobStatusProviderSubmitted:   {JobStatusProviderPending, JobStatusProviderProcessing, JobStatusProviderSucceeded, JobStatusProviderFailed},
	JobStatusProviderPending:     {JobStatusProviderProcessing, JobStatusProviderSucceeded, JobStatusProviderFailed, JobStatusCancelled},
	JobStatusProviderProcessing:  {JobStatusProviderSucceeded, JobStatusProviderFailed, JobStatusCancelled},
	JobStatusProviderSucceeded:   {JobStatusPostprocessing, JobStatusResultReady, JobStatusFailedRetryable, JobStatusRejected},
	JobStatusProviderFailed:      {JobStatusFailedRetryable, JobStatusFailedTerminal, JobStatusRefunded},
	JobStatusPostprocessing:      {JobStatusResultReady, JobStatusFailedRetryable, JobStatusFailedTerminal},
	JobStatusResultReady:         {JobStatusDelivering, JobStatusFailedRetryable},
	JobStatusDelivering:          {JobStatusSucceeded, JobStatusFailedRetryable, JobStatusFailedTerminal},
	JobStatusSucceeded:           {},
	JobStatusFailedRetryable:     {JobStatusQueued, JobStatusFailedTerminal, JobStatusRefunded},
	JobStatusFailedTerminal:      {JobStatusRefunded},
	JobStatusCancelled:           {JobStatusRefunded},
	JobStatusExpired:             {JobStatusRefunded},
	JobStatusRefunded:            {},
}

// IsTerminal reports whether the status admits no further transitions.
func (s JobStatus) IsTerminal() bool {
	next, ok := jobTransitions[s]
	return ok && len(next) == 0
}

// CanTransitionTo reports whether moving from the receiver status to the target
// status is an allowed job state-machine transition.
func (s JobStatus) CanTransitionTo(target JobStatus) bool {
	for _, allowed := range jobTransitions[s] {
		if allowed == target {
			return true
		}
	}
	return false
}

// Job is the central unit of work in the platform. Every user request becomes a
// Job (invariant: all user requests must become Jobs).
type Job struct {
	// ID is the internal primary key.
	ID uuid.UUID `json:"id"`
	// UserID is the owner of the job.
	UserID uuid.UUID `json:"user_id"`
	// VKPeerID is the VK conversation the job belongs to.
	VKPeerID int64 `json:"vk_peer_id"`
	// CommandID is the command that produced this job.
	CommandID uuid.UUID `json:"command_id"`
	// OperationType is the concrete operation requested.
	OperationType OperationType `json:"operation_type"`
	// Modality is the content kind of the job.
	Modality Modality `json:"modality"`
	// ProviderID is the selected provider, nil until routing happens.
	ProviderID *uuid.UUID `json:"provider_id,omitempty"`
	// ModelID is the selected model, nil until routing happens.
	ModelID *uuid.UUID `json:"model_id,omitempty"`
	// Status is the current job state-machine state.
	Status JobStatus `json:"status"`
	// Priority is the scheduling priority; higher runs sooner.
	Priority int `json:"priority"`
	// IdempotencyKey deduplicates job creation for the same request.
	IdempotencyKey string `json:"idempotency_key"`
	// CorrelationID links all events and logs for one request flow.
	CorrelationID string `json:"correlation_id"`
	// InputArtifactIDs are the artifacts fed into the job.
	InputArtifactIDs []uuid.UUID `json:"input_artifact_ids"`
	// OutputArtifactIDs are the artifacts produced by the job.
	OutputArtifactIDs []uuid.UUID `json:"output_artifact_ids"`
	// Params holds normalized operation parameters (aspect ratio, duration...).
	Params json.RawMessage `json:"params"`
	// CostEstimate is the estimated cost in credits before reservation.
	CostEstimate int64 `json:"cost_estimate"`
	// CostReserved is the amount of credits reserved for the job.
	CostReserved int64 `json:"cost_reserved"`
	// CostCaptured is the amount of credits actually charged.
	CostCaptured int64 `json:"cost_captured"`
	// ErrorCode is an internal error class set on failure.
	ErrorCode string `json:"error_code,omitempty"`
	// ErrorMessage is a human-readable error description.
	ErrorMessage string `json:"error_message,omitempty"`
	// CreatedAt is the row creation timestamp.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the last mutation timestamp.
	UpdatedAt time.Time `json:"updated_at"`
	// ExpiresAt is the deadline after which the job is expired.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}
