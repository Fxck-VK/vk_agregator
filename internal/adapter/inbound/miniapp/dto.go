package miniapp

import (
	"encoding/json"
	"strings"
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
	// VideoRouteAlias is a public product route alias. Backend resolves it to
	// provider/model/cost; provider-native ids and prices are never accepted.
	VideoRouteAlias string `json:"video_route_alias,omitempty"`
	// ImageQuality is a public quality alias from /miniapp/model-catalog. The
	// backend validates it against the selected image model before pricing.
	ImageQuality string `json:"image_quality,omitempty"`
	// ReferenceArtifactIDs are optional input images owned by the user. They are
	// validated server-side and never expanded into URLs in the BFF response.
	ReferenceArtifactIDs []uuid.UUID `json:"reference_artifact_ids,omitempty"`
	// DurationSec is the requested video length in seconds for video_generate.
	// Allowed values: 3, 5, 10. Omitted defaults to 5.
	DurationSec int `json:"duration_sec,omitempty"`
	// AspectRatio is backend-derived from trusted reference artifact metadata.
	// It is never accepted from client JSON.
	AspectRatio string `json:"-"`
}

// ChatMessageRequest is the body accepted by POST /miniapp/chat/messages.
type ChatMessageRequest struct {
	Prompt         string `json:"prompt"`
	ConversationID string `json:"conversation_id,omitempty"`
}

// ClientEventRequest accepts only coarse, safe frontend telemetry fields. It
// intentionally has no prompt, launch params, raw URL, user id or payload field.
type ClientEventRequest struct {
	Surface    string `json:"surface,omitempty"`
	EventType  string `json:"event_type"`
	Screen     string `json:"screen,omitempty"`
	Route      string `json:"route,omitempty"`
	Status     string `json:"status,omitempty"`
	ErrorClass string `json:"error_class,omitempty"`
	Step       string `json:"step,omitempty"`
	Reason     string `json:"reason,omitempty"`
	// DurationMS is a bounded client-side duration bucket source. It is never
	// paired with raw URLs, prompts, launch params or user identifiers.
	DurationMS int64 `json:"duration_ms,omitempty"`
}

type miniAppJobParams struct {
	Prompt               string                    `json:"prompt"`
	ModelID              string                    `json:"model_id,omitempty"`
	ModelName            string                    `json:"model_name,omitempty"`
	VideoRouteAlias      string                    `json:"video_route_alias,omitempty"`
	Provider             domain.ProviderName       `json:"provider,omitempty"`
	ModelCode            string                    `json:"model_code,omitempty"`
	Size                 string                    `json:"size,omitempty"`
	Resolution           string                    `json:"resolution,omitempty"`
	ImageQuality         string                    `json:"image_quality,omitempty"`
	ReferenceArtifactIDs []uuid.UUID               `json:"reference_artifact_ids,omitempty"`
	ConversationID       string                    `json:"conversation_id,omitempty"`
	ConversationSource   domain.ConversationSource `json:"conversation_source,omitempty"`
	ExternalThreadID     string                    `json:"external_thread_id,omitempty"`
	DurationSec          int                       `json:"duration_sec,omitempty"`
	AspectRatio          string                    `json:"aspect_ratio,omitempty"`
}

type VideoRouteDTO struct {
	Type                   string   `json:"type,omitempty"`
	Alias                  string   `json:"alias"`
	Name                   string   `json:"name,omitempty"`
	Description            string   `json:"description,omitempty"`
	EstimateCredits        int64    `json:"estimate_credits,omitempty"`
	Enabled                bool     `json:"enabled"`
	AllowedDurationsSec    []int    `json:"allowed_durations_sec,omitempty"`
	AllowedResolutions     []string `json:"allowed_resolutions,omitempty"`
	AllowedAspectRatios    []string `json:"allowed_aspect_ratios,omitempty"`
	DefaultDurationSec     int      `json:"default_duration_sec,omitempty"`
	DefaultResolution      string   `json:"default_resolution,omitempty"`
	DefaultAspectRatio     string   `json:"default_aspect_ratio,omitempty"`
	RequiresStartImage     bool     `json:"requires_start_image"`
	SupportsReferenceImage bool     `json:"supports_reference_image"`
	MaxReferenceImages     int      `json:"max_reference_images,omitempty"`
}

type ImageModelDTO struct {
	Type                   string   `json:"type,omitempty"`
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	Description            string   `json:"description,omitempty"`
	EstimateCredits        int64    `json:"estimate_credits,omitempty"`
	Enabled                bool     `json:"enabled"`
	QualityOptions         []string `json:"quality_options,omitempty"`
	DefaultQuality         string   `json:"default_quality,omitempty"`
	SupportsReferenceImage bool     `json:"supports_reference_image"`
	MaxReferenceImages     int      `json:"max_reference_images,omitempty"`
}

type ModelCatalogItemDTO struct {
	Type                   string   `json:"type"`
	ID                     string   `json:"id"`
	Alias                  string   `json:"alias,omitempty"`
	Name                   string   `json:"name"`
	Description            string   `json:"description,omitempty"`
	EstimateCredits        int64    `json:"estimate_credits,omitempty"`
	Enabled                bool     `json:"enabled"`
	QualityOptions         []string `json:"quality_options,omitempty"`
	DefaultQuality         string   `json:"default_quality,omitempty"`
	AllowedDurationsSec    []int    `json:"allowed_durations_sec,omitempty"`
	AllowedResolutions     []string `json:"allowed_resolutions,omitempty"`
	AllowedAspectRatios    []string `json:"allowed_aspect_ratios,omitempty"`
	DefaultDurationSec     int      `json:"default_duration_sec,omitempty"`
	DefaultResolution      string   `json:"default_resolution,omitempty"`
	DefaultAspectRatio     string   `json:"default_aspect_ratio,omitempty"`
	RequiresStartImage     bool     `json:"requires_start_image"`
	SupportsReferenceImage bool     `json:"supports_reference_image"`
	MaxReferenceImages     int      `json:"max_reference_images,omitempty"`
}

// EstimateDTO is returned by POST /miniapp/estimate. It exposes only
// backend-owned cost and balance information, never provider details.
type EstimateDTO struct {
	Operation       string `json:"operation"`
	ModelID         string `json:"model_id,omitempty"`
	ModelName       string `json:"model_name,omitempty"`
	VideoRouteAlias string `json:"video_route_alias,omitempty"`
	ImageQuality    string `json:"image_quality,omitempty"`
	CostEstimate    int64  `json:"cost_estimate"`
	BalanceCredits  int64  `json:"balance_credits"`
	EnoughCredits   bool   `json:"enough_credits"`
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
	VideoRouteAlias   string      `json:"video_route_alias,omitempty"`
	ImageQuality      string      `json:"image_quality,omitempty"`
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
		ID:           j.ID,
		Operation:    string(j.OperationType),
		Modality:     string(j.Modality),
		Status:       string(j.Status),
		CostEstimate: j.CostEstimate,
		CostCaptured: j.CostCaptured,
		ErrorCode:    safeJobErrorCode(j),
		CreatedAt:    j.CreatedAt,
		UpdatedAt:    j.UpdatedAt,
	}
	if j.Status == domain.JobStatusSucceeded {
		out.OutputArtifactIDs = j.OutputArtifactIDs
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
			if j.OperationType == domain.OperationVideoGenerate {
				out.VideoRouteAlias = params.VideoRouteAlias
				out.ModelName = params.ModelName
			} else if j.OperationType != domain.OperationTextGenerate {
				out.ModelID = params.ModelID
				out.ModelName = params.ModelName
				out.ImageQuality = params.ImageQuality
			}
		}
	}
	return out
}

func safeJobErrorCode(j *domain.Job) string {
	if j == nil || j.ErrorCode == "" {
		return ""
	}
	switch j.ErrorCode {
	case domain.JobErrMediaUploadInvalid,
		domain.JobErrMediaUploadTooLarge,
		domain.JobErrMediaUploadUnsupported,
		domain.JobErrMediaProviderOutputInvalid,
		domain.JobErrMediaProcessingUnavailable,
		domain.JobErrMediaDeliveryFailed,
		domain.JobErrMediaOverloadedRetryLater:
		return j.ErrorCode
	case string(domain.ProviderErrMediaProbeFailed),
		"media_contract_output_not_delivery_ready",
		"video_output_missing",
		"video_artifact_missing":
		return domain.JobErrMediaProviderOutputInvalid
	case string(domain.ProviderErrMediaTranscodeFailed),
		"variant_probe_failed",
		"variant_store_failed":
		return domain.JobErrMediaProcessingUnavailable
	case "media_probe_overloaded",
		"media_transcode_overloaded",
		"variant_probe_overloaded",
		"media_variant_backlog_full",
		string(domain.ProviderErrOverloaded),
		string(domain.ProviderErrRateLimited):
		return domain.JobErrMediaOverloadedRetryLater
	case "media_probe_unavailable",
		"media_probe_storage_unavailable",
		"media_contract_unavailable",
		"artifact_metadata_unavailable",
		"artifact_bytes_unavailable",
		"variant_probe_unavailable",
		string(domain.ProviderErrOutputDownloadFailed):
		return domain.JobErrMediaProcessingUnavailable
	case "delivery_failed":
		return domain.JobErrMediaDeliveryFailed
	default:
		if isFailedMediaJob(j) && looksInternalMediaErrorCode(j.ErrorCode) {
			return domain.JobErrMediaProcessingUnavailable
		}
		return j.ErrorCode
	}
}

func isFailedMediaJob(j *domain.Job) bool {
	if j == nil {
		return false
	}
	if j.Modality != domain.ModalityImage && j.Modality != domain.ModalityVideo {
		return false
	}
	return j.Status == domain.JobStatusFailedTerminal || j.Status == domain.JobStatusRejected || j.Status == domain.JobStatusRefunded
}

func looksInternalMediaErrorCode(code string) bool {
	code = strings.ToLower(strings.TrimSpace(code))
	return strings.Contains(code, "provider") ||
		strings.Contains(code, "probe") ||
		strings.Contains(code, "transcode") ||
		strings.Contains(code, "artifact") ||
		strings.Contains(code, "storage") ||
		strings.Contains(code, "delivery")
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

// ReferralDTO is the Mini App-safe representation of the shared referral state.
// It never exposes internal user IDs or another user's profile data.
type ReferralDTO struct {
	Code                        string `json:"code"`
	InviteURL                   string `json:"invite_url"`
	InvitedCount                int    `json:"invited_count"`
	RegisteredCount             int    `json:"registered_count"`
	ActivatedCount              int    `json:"activated_count"`
	RewardedCount               int    `json:"rewarded_count"`
	ReferrerSignupRewardCredits int64  `json:"referrer_signup_reward_credits"`
	ReferredSignupRewardCredits int64  `json:"referred_signup_reward_credits"`
}

// ApplyReferralRequest accepts only the public referral code. User identity is
// derived from verified Mini App launch params in the handler.
type ApplyReferralRequest struct {
	Code string `json:"code"`
}

// ApplyReferralDTO reports a safe, no-PII referral acceptance result.
type ApplyReferralDTO struct {
	Applied        bool `json:"applied"`
	AlreadyApplied bool `json:"already_applied"`
	InvalidCode    bool `json:"invalid_code"`
	SelfReferral   bool `json:"self_referral"`
}

// PaymentProductDTO is the Mini App-safe representation of an active top-up
// catalog entry.
type PaymentProductDTO struct {
	ID           uuid.UUID `json:"id"`
	Code         string    `json:"code"`
	Title        string    `json:"title"`
	Amount       int64     `json:"amount"`
	Currency     string    `json:"currency"`
	Credits      int64     `json:"credits"`
	PriceVersion int       `json:"price_version"`
}

func newPaymentProductDTO(product *domain.PaymentProduct) PaymentProductDTO {
	return PaymentProductDTO{
		ID:           product.ID,
		Code:         product.Code,
		Title:        product.Title,
		Amount:       product.Amount,
		Currency:     string(product.Currency),
		Credits:      product.Credits,
		PriceVersion: product.PriceVersion,
	}
}

// CreatePaymentIntentRequest is accepted by POST /miniapp/payments/intents.
// User identity is never accepted in the body; it comes from verified launch
// params in the handler.
type CreatePaymentIntentRequest struct {
	ProductCode  string `json:"product_code"`
	ReceiptEmail string `json:"receipt_email,omitempty"`
	ReceiptPhone string `json:"receipt_phone,omitempty"`
	ReturnURL    string `json:"return_url,omitempty"`
	ForceNew     bool   `json:"force_new,omitempty"`
}

// PaymentIntentDTO is the Mini App-safe representation of a top-up intent.
type PaymentIntentDTO struct {
	ID                  uuid.UUID `json:"id"`
	ProductID           uuid.UUID `json:"product_id,omitempty"`
	Status              string    `json:"status"`
	Amount              int64     `json:"amount"`
	Currency            string    `json:"currency"`
	Credits             int64     `json:"credits"`
	PriceVersion        int       `json:"price_version"`
	ConfirmationURL     string    `json:"confirmation_url,omitempty"`
	ReusedActivePayment bool      `json:"reused_active_payment,omitempty"`
	Notice              string    `json:"notice,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func newPaymentIntentDTO(intent *domain.PaymentIntent) PaymentIntentDTO {
	dto := PaymentIntentDTO{
		ID:              intent.ID,
		Status:          string(intent.Status),
		Amount:          intent.Amount,
		Currency:        string(intent.Currency),
		Credits:         intent.Credits,
		PriceVersion:    intent.PriceVersion,
		ConfirmationURL: intent.ConfirmationURL,
		CreatedAt:       intent.CreatedAt,
		UpdatedAt:       intent.UpdatedAt,
	}
	if intent.ProductID != nil {
		dto.ProductID = *intent.ProductID
	}
	return dto
}

// ArtifactUploadDTO is returned by POST /miniapp/artifacts. It exposes only the
// backend-owned artifact id; URLs and storage paths stay private.
type ArtifactUploadDTO struct {
	ArtifactID uuid.UUID `json:"artifact_id"`
}
