package admin

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

// JobDTO is the admin representation of a job.
type JobDTO struct {
	ID                uuid.UUID   `json:"id"`
	UserID            uuid.UUID   `json:"user_id"`
	VKPeerID          int64       `json:"vk_peer_id"`
	Operation         string      `json:"operation"`
	Modality          string      `json:"modality"`
	Status            string      `json:"status"`
	CostEstimate      int64       `json:"cost_estimate"`
	CostReserved      int64       `json:"cost_reserved"`
	CostCaptured      int64       `json:"cost_captured"`
	OutputArtifactIDs []uuid.UUID `json:"output_artifact_ids"`
	ErrorCode         string      `json:"error_code,omitempty"`
	ErrorMessage      string      `json:"error_message,omitempty"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

func newJobDTO(j *domain.Job) JobDTO {
	out := JobDTO{
		ID:                j.ID,
		UserID:            j.UserID,
		VKPeerID:          j.VKPeerID,
		Operation:         string(j.OperationType),
		Modality:          string(j.Modality),
		Status:            string(j.Status),
		CostEstimate:      j.CostEstimate,
		CostReserved:      j.CostReserved,
		CostCaptured:      j.CostCaptured,
		OutputArtifactIDs: j.OutputArtifactIDs,
		ErrorCode:         j.ErrorCode,
		ErrorMessage:      j.ErrorMessage,
		CreatedAt:         j.CreatedAt,
		UpdatedAt:         j.UpdatedAt,
	}
	if out.OutputArtifactIDs == nil {
		out.OutputArtifactIDs = []uuid.UUID{}
	}
	return out
}

// UserDTO is the admin representation of a user.
type UserDTO struct {
	ID             uuid.UUID `json:"id"`
	VKUserID       int64     `json:"vk_user_id"`
	Role           string    `json:"role"`
	Status         string    `json:"status"`
	Locale         string    `json:"locale"`
	BalanceCredits *int64    `json:"balance_credits,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func newUserDTO(u *domain.User) UserDTO {
	return UserDTO{
		ID:        u.ID,
		VKUserID:  u.VKUserID,
		Role:      string(u.Role),
		Status:    string(u.Status),
		Locale:    u.Locale,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

// DeliveryDTO is the admin representation of a delivery attempt.
type DeliveryDTO struct {
	ID           uuid.UUID  `json:"id"`
	JobID        uuid.UUID  `json:"job_id"`
	UserID       uuid.UUID  `json:"user_id"`
	VKPeerID     int64      `json:"vk_peer_id"`
	ArtifactID   *uuid.UUID `json:"artifact_id,omitempty"`
	Type         string     `json:"type"`
	Status       string     `json:"status"`
	VKRandomID   int64      `json:"vk_random_id"`
	VKMessageID  *int64     `json:"vk_message_id,omitempty"`
	Attachment   string     `json:"attachment,omitempty"`
	AttemptNo    int        `json:"attempt_no"`
	ErrorCode    string     `json:"error_code,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func newDeliveryDTO(d *domain.Delivery) DeliveryDTO {
	return DeliveryDTO{
		ID:           d.ID,
		JobID:        d.JobID,
		UserID:       d.UserID,
		VKPeerID:     d.VKPeerID,
		ArtifactID:   d.ArtifactID,
		Type:         string(d.Type),
		Status:       string(d.Status),
		VKRandomID:   d.VKRandomID,
		VKMessageID:  d.VKMessageID,
		Attachment:   d.Attachment,
		AttemptNo:    d.AttemptNo,
		ErrorCode:    d.ErrorCode,
		ErrorMessage: d.ErrorMessage,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	}
}
