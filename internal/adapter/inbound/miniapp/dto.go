package miniapp

import (
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
}

// EstimateDTO is returned by POST /miniapp/estimate. It exposes only
// backend-owned cost and balance information, never provider details.
type EstimateDTO struct {
	Operation      string `json:"operation"`
	ModelID        string `json:"model_id"`
	CostEstimate   int64  `json:"cost_estimate"`
	BalanceCredits int64  `json:"balance_credits"`
	EnoughCredits  bool   `json:"enough_credits"`
}

// JobDTO is the miniapp representation of a job.
type JobDTO struct {
	ID                uuid.UUID   `json:"id"`
	Operation         string      `json:"operation"`
	Modality          string      `json:"modality"`
	Status            string      `json:"status"`
	Prompt            string      `json:"prompt,omitempty"`
	CostEstimate      int64       `json:"cost_estimate"`
	CostCaptured      int64       `json:"cost_captured"`
	OutputArtifactIDs []uuid.UUID `json:"output_artifact_ids"`
	ErrorCode         string      `json:"error_code,omitempty"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
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
	return out
}

// BalanceDTO is the miniapp representation of a user's credit balance.
type BalanceDTO struct {
	BalanceCredits int64 `json:"balance_credits"`
}
