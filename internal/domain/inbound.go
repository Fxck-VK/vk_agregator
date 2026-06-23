package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// InboundEventStatus is the processing state of an inbound event.
type InboundEventStatus string

const (
	// InboundReceived means the event was accepted and persisted.
	InboundReceived InboundEventStatus = "received"
	// InboundProcessed means the event was fully handled (command/job created).
	InboundProcessed InboundEventStatus = "processed"
	// InboundFailed means processing failed and may be retried.
	InboundFailed InboundEventStatus = "failed"
	// InboundIgnored means the event was intentionally not acted upon.
	InboundIgnored InboundEventStatus = "ignored"
)

// InboundEvent is the persisted record of an external event received by an
// inbound gateway (for example a VK callback). It stores source metadata and a
// minimized/redactable payload before business processing to support auditing
// and idempotent reprocessing (invariant: every external inbound event has an
// idempotency key and is deduplicated).
type InboundEvent struct {
	// ID is the internal primary key.
	ID uuid.UUID `json:"id"`
	// Source is the inbound channel, e.g. "vk".
	Source string `json:"source"`
	// EventType is the source-specific event name, e.g. "message_new".
	EventType string `json:"event_type"`
	// GroupID is the VK community id the event targets (0 if not applicable).
	GroupID int64 `json:"group_id"`
	// VKEventID is the source-assigned event id used for deduplication.
	VKEventID string `json:"vk_event_id"`
	// PeerID is the VK conversation the event came from (0 if not applicable).
	PeerID int64 `json:"peer_id"`
	// VKUserID is the external user id that triggered the event.
	VKUserID int64 `json:"vk_user_id"`
	// Payload is a minimized/redactable event representation, not raw content.
	Payload json.RawMessage `json:"payload"`
	// Status is the processing state of the event.
	Status InboundEventStatus `json:"status"`
	// IdempotencyKey deduplicates repeated deliveries of the same event.
	IdempotencyKey string `json:"idempotency_key"`
	// CreatedAt is the row creation timestamp.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the last mutation timestamp.
	UpdatedAt time.Time `json:"updated_at"`
}
