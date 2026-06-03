package domain

import (
	"time"

	"github.com/google/uuid"
)

// DeliveryType is the kind of VK delivery being performed.
type DeliveryType string

const (
	// DeliveryTypeMessage is a plain text message.
	DeliveryTypeMessage DeliveryType = "message"
	// DeliveryTypePhoto is a photo attachment.
	DeliveryTypePhoto DeliveryType = "photo"
	// DeliveryTypeVideo is a video attachment.
	DeliveryTypeVideo DeliveryType = "video"
	// DeliveryTypeDoc is a document attachment.
	DeliveryTypeDoc DeliveryType = "doc"
)

// Valid reports whether the delivery type is one of the known types.
func (d DeliveryType) Valid() bool {
	switch d {
	case DeliveryTypeMessage, DeliveryTypePhoto, DeliveryTypeVideo, DeliveryTypeDoc:
		return true
	default:
		return false
	}
}

// DeliveryStatus is the lifecycle state of a delivery attempt. Every delivery
// attempt is persisted (invariant #9).
type DeliveryStatus string

const (
	// DeliveryStatusPending means the delivery is queued but not started.
	DeliveryStatusPending DeliveryStatus = "pending"
	// DeliveryStatusUploading means media is being uploaded to VK.
	DeliveryStatusUploading DeliveryStatus = "uploading"
	// DeliveryStatusSent means the message was successfully sent.
	DeliveryStatusSent DeliveryStatus = "sent"
	// DeliveryStatusRetrying means a transient failure will be retried.
	DeliveryStatusRetrying DeliveryStatus = "retrying"
	// DeliveryStatusFailed is the terminal failure state.
	DeliveryStatusFailed DeliveryStatus = "failed"
)

// IsTerminal reports whether the delivery status is final.
func (s DeliveryStatus) IsTerminal() bool {
	switch s {
	case DeliveryStatusSent, DeliveryStatusFailed:
		return true
	default:
		return false
	}
}

// Delivery is the persisted record of delivering a job result to VK. The VK
// random_id deduplicates sends so that retries never produce duplicate messages
// (invariant: every delivery attempt is persisted and deduplicated).
type Delivery struct {
	// ID is the internal primary key.
	ID uuid.UUID `json:"id"`
	// JobID is the job whose result is being delivered.
	JobID uuid.UUID `json:"job_id"`
	// UserID is the recipient user.
	UserID uuid.UUID `json:"user_id"`
	// VKPeerID is the VK conversation to deliver into.
	VKPeerID int64 `json:"vk_peer_id"`
	// ArtifactID is the artifact being delivered, nil for plain messages.
	ArtifactID *uuid.UUID `json:"artifact_id,omitempty"`
	// Type is the kind of delivery.
	Type DeliveryType `json:"type"`
	// Status is the current delivery state.
	Status DeliveryStatus `json:"status"`
	// VKRandomID is the unique id used by messages.send to avoid duplicates.
	VKRandomID int64 `json:"vk_random_id"`
	// VKMessageID is the VK message id assigned once sent.
	VKMessageID *int64 `json:"vk_message_id,omitempty"`
	// Attachment is the VK attachment string (e.g. "photo123_456").
	Attachment string `json:"attachment,omitempty"`
	// Text is the message body for text deliveries.
	Text string `json:"text,omitempty"`
	// AttemptNo is the delivery attempt number, starting at 1.
	AttemptNo int `json:"attempt_no"`
	// IdempotencyKey makes the delivery safe to retry.
	IdempotencyKey string `json:"idempotency_key"`
	// ErrorCode is an internal error class set on failure.
	ErrorCode string `json:"error_code,omitempty"`
	// ErrorMessage is a human-readable failure description.
	ErrorMessage string `json:"error_message,omitempty"`
	// CreatedAt is the row creation timestamp.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the last mutation timestamp.
	UpdatedAt time.Time `json:"updated_at"`
}
