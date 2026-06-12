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

// OverviewDTO is a bounded, secret-free operational summary for the first
// read-only operator dashboard.
type OverviewDTO struct {
	GeneratedAt time.Time         `json:"generated_at"`
	Cards       []OverviewCardDTO `json:"cards"`
}

// OverviewCardDTO summarizes one product area without raw entity identifiers,
// payment/provider payloads or private URLs.
type OverviewCardDTO struct {
	ID      string              `json:"id"`
	Title   string              `json:"title"`
	Status  string              `json:"status"`
	Summary string              `json:"summary"`
	Metrics []OverviewMetricDTO `json:"metrics,omitempty"`
}

// OverviewMetricDTO contains bounded display metrics only.
type OverviewMetricDTO struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Status string `json:"status,omitempty"`
}

// OperatorJobsDTO is the safe read-only Jobs screen payload. LookupID is an
// opaque protected-admin lookup value used by the UI to request details; display
// should prefer DisplayID and safe refs.
type OperatorJobsDTO struct {
	GeneratedAt time.Time               `json:"generated_at"`
	Items       []OperatorJobListItem   `json:"items"`
	Pagination  pagination              `json:"pagination"`
	Queue       OperatorQueueSummaryDTO `json:"queue"`
}

// OperatorJobListItem is a bounded job row without raw user/VK identifiers,
// prompts, params, provider payloads or private artifact URLs.
type OperatorJobListItem struct {
	LookupID       string    `json:"lookup_id"`
	DisplayID      string    `json:"display_id"`
	CorrelationRef string    `json:"correlation_ref,omitempty"`
	Operation      string    `json:"operation"`
	Modality       string    `json:"modality"`
	Status         string    `json:"status"`
	ErrorClass     string    `json:"error_class,omitempty"`
	CostEstimate   int64     `json:"cost_estimate"`
	CostReserved   int64     `json:"cost_reserved"`
	CostCaptured   int64     `json:"cost_captured"`
	InputCount     int       `json:"input_count"`
	OutputCount    int       `json:"output_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	AgeSeconds     int64     `json:"age_seconds"`
}

// OperatorJobDetailDTO is a safe job detail view for operator triage.
type OperatorJobDetailDTO struct {
	Job            OperatorJobListItem       `json:"job"`
	AllowedNext    []string                  `json:"allowed_next_statuses"`
	Artifacts      OperatorJobArtifactsDTO   `json:"artifacts"`
	Reservation    *OperatorReservationDTO   `json:"reservation,omitempty"`
	Delivery       OperatorDeliverySummary   `json:"delivery"`
	DeliveryEvents []OperatorDeliveryAttempt `json:"delivery_events"`
}

// OperatorJobArtifactsDTO exposes safe artifact references only.
type OperatorJobArtifactsDTO struct {
	InputRefs  []string `json:"input_refs"`
	OutputRefs []string `json:"output_refs"`
}

// OperatorReservationDTO shows ledger-backed reservation state without account
// ids or idempotency keys.
type OperatorReservationDTO struct {
	Status    string    `json:"status"`
	Amount    int64     `json:"amount"`
	ExpiresAt time.Time `json:"expires_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// OperatorDeliverySummary summarizes persisted delivery attempts.
type OperatorDeliverySummary struct {
	Status             string `json:"status"`
	Attempts           int    `json:"attempts"`
	RetryCount         int    `json:"retry_count"`
	LastErrorClass     string `json:"last_error_class,omitempty"`
	LastArtifactRef    string `json:"last_artifact_ref,omitempty"`
	LastDeliveryType   string `json:"last_delivery_type,omitempty"`
	LastDeliveryStatus string `json:"last_delivery_status,omitempty"`
}

// OperatorDeliveryAttempt is a safe delivery attempt row.
type OperatorDeliveryAttempt struct {
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	AttemptNo   int       `json:"attempt_no"`
	ErrorClass  string    `json:"error_class,omitempty"`
	ArtifactRef string    `json:"artifact_ref,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// OperatorQueueSummaryDTO is a safe worker/queue pressure snapshot. It is
// derived from persisted job state and explicitly marks missing DLQ/Redis stream
// sources as not_wired instead of pretending they are healthy.
type OperatorQueueSummaryDTO struct {
	GeneratedAt            time.Time                `json:"generated_at"`
	DegradationState       string                   `json:"degradation_state"`
	Backlog                []OperatorQueueMetricDTO `json:"backlog"`
	OldestQueuedAgeSeconds *int64                   `json:"oldest_queued_age_seconds,omitempty"`
	RetryCount             int                      `json:"retry_count"`
	DLQ                    OperatorQueueNotWiredDTO `json:"dlq"`
	ProviderCircuit        OperatorQueueNotWiredDTO `json:"provider_circuit"`
	Notes                  []string                 `json:"notes,omitempty"`
}

// OperatorQueueMetricDTO is a bounded queue metric row.
type OperatorQueueMetricDTO struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Status string `json:"status"`
}

// OperatorQueueNotWiredDTO marks data that needs a dedicated source before the
// UI can render it as healthy.
type OperatorQueueNotWiredDTO struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
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

// ReferralStatsDTO is a safe operator view of one public referral code.
type ReferralStatsDTO struct {
	Code            string `json:"code"`
	InvitedCount    int    `json:"invited_count"`
	RegisteredCount int    `json:"registered_count"`
	ActivatedCount  int    `json:"activated_count"`
	RewardedCount   int    `json:"rewarded_count"`
}

func newReferralStatsDTO(stats domain.ReferralCodeStats) ReferralStatsDTO {
	return ReferralStatsDTO{
		Code:            stats.Code,
		InvitedCount:    stats.Stats.Total(),
		RegisteredCount: stats.Stats.RegisteredCount,
		ActivatedCount:  stats.Stats.ActivatedCount,
		RewardedCount:   stats.Stats.RewardedCount,
	}
}

// SuspiciousReferralDTO explains aggregate referral-code patterns without
// exposing invited-user identities.
type SuspiciousReferralDTO struct {
	ReferralStatsDTO
	Reasons []string `json:"reasons"`
}

type referralFutureFlagDTO struct {
	Code    string `json:"code"`
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
	Message string `json:"message"`
}
