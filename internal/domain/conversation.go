package domain

import (
	"time"

	"github.com/google/uuid"
)

// ConversationStatus describes whether a dialog thread can receive new
// messages. VK bot context uses one active conversation per user/peer.
type ConversationStatus string

const (
	ConversationActive   ConversationStatus = "active"
	ConversationArchived ConversationStatus = "archived"
)

// Conversation is the server-side memory thread used to build compact text
// model context. It is not sent to providers directly; workers render a bounded
// prompt from its messages and summary.
type Conversation struct {
	ID        uuid.UUID          `json:"id"`
	UserID    uuid.UUID          `json:"user_id"`
	VKPeerID  int64              `json:"vk_peer_id"`
	Status    ConversationStatus `json:"status"`
	Title     string             `json:"title,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

// ConversationMessageRole is the author role stored in conversation history.
type ConversationMessageRole string

const (
	ConversationRoleUser      ConversationMessageRole = "user"
	ConversationRoleAssistant ConversationMessageRole = "assistant"
)

// ConversationMessage is one persisted user/assistant turn in a conversation.
type ConversationMessage struct {
	ID             uuid.UUID               `json:"id"`
	ConversationID uuid.UUID               `json:"conversation_id"`
	JobID          uuid.UUID               `json:"job_id"`
	Seq            int64                   `json:"seq"`
	Role           ConversationMessageRole `json:"role"`
	Text           string                  `json:"text"`
	TokenCount     int                     `json:"token_count"`
	CreatedAt      time.Time               `json:"created_at"`
}

// ConversationSummary is the compact memory of older turns up to
// SummarizedUntilSeq. Newer turns are still included as recent messages.
type ConversationSummary struct {
	ID                 uuid.UUID `json:"id"`
	ConversationID     uuid.UUID `json:"conversation_id"`
	Text               string    `json:"text"`
	TokenCount         int       `json:"token_count"`
	SummarizedUntilSeq int64     `json:"summarized_until_seq"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
