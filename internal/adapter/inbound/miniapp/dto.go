package miniapp

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// pagination is the echoed paging metadata for list responses.
type pagination struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	Count   int  `json:"count"`
	HasMore bool `json:"has_more"`
}

// listResponse is the envelope for paginated list endpoints.
type listResponse[T any] struct {
	Items      []T        `json:"items"`
	Pagination pagination `json:"pagination"`
}

// CreateJobRequest is the body accepted by POST /miniapp/jobs.
type CreateJobRequest struct {
	// Operation is the AI operation to perform.
	// Allowed values: "text_generate", "image_generate", "video_generate".
	Operation string `json:"operation"`
	// Prompt is the user's input text for the generation.
	Prompt string `json:"prompt"`
	// ModelID is the optional user-selected model. It is validated server-side
	// by operation and is never trusted for provider choice or pricing.
	ModelID string `json:"model_id,omitempty"`
	// ReferenceArtifactIDs are optional input images owned by the user. They are
	// validated server-side and never expanded into URLs in the BFF response.
	ReferenceArtifactIDs []uuid.UUID `json:"reference_artifact_ids,omitempty"`
	// DurationSec is the requested video length in seconds for video_generate.
	// Allowed values: 3, 5, 10. Omitted defaults to 5.
	DurationSec int `json:"duration_sec,omitempty"`
}

// ChatMessageRequest is the body accepted by POST /miniapp/chat/messages.
type ChatMessageRequest struct {
	Prompt         string `json:"prompt"`
	ConversationID string `json:"conversation_id,omitempty"`
}

type miniAppJobParams struct {
	Prompt               string                    `json:"prompt"`
	ModelID              string                    `json:"model_id,omitempty"`
	ModelName            string                    `json:"model_name,omitempty"`
	ModelCode            string                    `json:"model_code,omitempty"`
	ReferenceArtifactIDs []uuid.UUID               `json:"reference_artifact_ids,omitempty"`
	ConversationID       string                    `json:"conversation_id,omitempty"`
	ConversationSource   domain.ConversationSource `json:"conversation_source,omitempty"`
	ExternalThreadID     string                    `json:"external_thread_id,omitempty"`
	DurationSec          int                       `json:"duration_sec,omitempty"`
}

// EstimateDTO is returned by POST /miniapp/estimate. It exposes only
// backend-owned cost and balance information, never provider details.
type EstimateDTO struct {
	Operation      string `json:"operation"`
	ModelID        string `json:"model_id,omitempty"`
	ModelName      string `json:"model_name,omitempty"`
	CostEstimate   int64  `json:"cost_estimate"`
	BalanceCredits int64  `json:"balance_credits"`
	EnoughCredits  bool   `json:"enough_credits"`
}

// JobDTO is the miniapp representation of a job.
type JobDTO struct {
	ID        uuid.UUID `json:"id"`
	Operation string    `json:"operation"`
	Modality  string    `json:"modality"`
	Status    string    `json:"status"`
	Prompt    string    `json:"prompt,omitempty"`
	// ConversationID links text_generate jobs to a Mini App chat thread.
	ConversationID    string      `json:"conversation_id,omitempty"`
	ModelID           string      `json:"model_id,omitempty"`
	ModelName         string      `json:"model_name,omitempty"`
	CostEstimate      int64       `json:"cost_estimate"`
	CostCaptured      int64       `json:"cost_captured"`
	OutputArtifactIDs []uuid.UUID `json:"output_artifact_ids"`
	ErrorCode         string      `json:"error_code,omitempty"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

// ChatJobDTO is the Mini App chat response. It keeps the real provider/model
// private and exposes only the public product alias.
type ChatJobDTO struct {
	JobDTO
	ModelName string `json:"model_name"`
}

// ChatConversationDTO is a durable Mini App chat thread owned by the verified
// backend user. ID is the opaque Mini App thread id, not a database id.
type ChatConversationDTO struct {
	ID                 string    `json:"id"`
	Title              string    `json:"title"`
	LastMessagePreview string    `json:"last_message_preview,omitempty"`
	LastMessageRole    string    `json:"last_message_role,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// ChatConversationMessageDTO is one persisted user/bot turn. It intentionally
// exposes only product-level roles and text, never provider metadata.
type ChatConversationMessageDTO struct {
	ID        uuid.UUID `json:"id"`
	JobID     uuid.UUID `json:"job_id"`
	Seq       int64     `json:"seq"`
	Role      string    `json:"role"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

func newJobDTO(j *domain.Job) JobDTO {
	out := JobDTO{
		ID:                j.ID,
		Operation:         string(j.OperationType),
		Modality:          string(j.Modality),
		Status:            string(j.Status),
		CostEstimate:      j.CostEstimate,
		CostCaptured:      j.CostCaptured,
		OutputArtifactIDs: j.OutputArtifactIDs,
		ErrorCode:         j.ErrorCode,
		CreatedAt:         j.CreatedAt,
		UpdatedAt:         j.UpdatedAt,
	}
	if out.OutputArtifactIDs == nil {
		out.OutputArtifactIDs = []uuid.UUID{}
	}
	if len(j.Params) > 0 {
		var params miniAppJobParams
		if err := json.Unmarshal(j.Params, &params); err == nil {
			if params.Prompt != "" {
				out.Prompt = params.Prompt
			}
			switch {
			case params.ConversationID != "":
				out.ConversationID = params.ConversationID
			case params.ExternalThreadID != "":
				out.ConversationID = params.ExternalThreadID
			}
			if j.OperationType != domain.OperationTextGenerate {
				out.ModelID = params.ModelID
				out.ModelName = params.ModelName
			}
		}
	}
	return out
}

func newChatJobDTO(j *domain.Job) ChatJobDTO {
	return ChatJobDTO{
		JobDTO:    newJobDTO(j),
		ModelName: miniAppChatPublicModelName,
	}
}

// BalanceDTO is the miniapp representation of a user's credit balance.
type BalanceDTO struct {
	BalanceCredits int64 `json:"balance_credits"`
}

// ArtifactUploadDTO is returned by POST /miniapp/artifacts. It exposes only the
// backend-owned artifact id; URLs and storage paths stay private.
type ArtifactUploadDTO struct {
	ArtifactID uuid.UUID `json:"artifact_id"`
}
