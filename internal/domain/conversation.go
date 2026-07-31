package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ConversationSource identifies the app surface that owns a dialog thread.
type ConversationSource string

const (
	ConversationSourceVKBot   ConversationSource = "vk_bot"
	ConversationSourceMiniApp ConversationSource = "miniapp"
	ConversationSourceWeb     ConversationSource = "web"
)

// ErrConversationAccountOwnershipRequired is returned when a web conversation
// lacks the canonical account owner required for account-native access.
var ErrConversationAccountOwnershipRequired = errors.New("domain: conversation account ownership required")

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
	ID               uuid.UUID          `json:"id"`
	UserID           uuid.UUID          `json:"user_id"`
	AccountID        uuid.UUID          `json:"account_id,omitempty"`
	Source           ConversationSource `json:"source"`
	VKPeerID         int64              `json:"vk_peer_id"`
	ExternalThreadID string             `json:"external_thread_id,omitempty"`
	Status           ConversationStatus `json:"status"`
	Title            string             `json:"title,omitempty"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
}

// ValidateOwnership verifies source-specific ownership invariants before a
// conversation reaches storage.
func (c Conversation) ValidateOwnership() error {
	if c.Source == ConversationSourceWeb && c.AccountID == uuid.Nil {
		return ErrConversationAccountOwnershipRequired
	}
	return nil
}

// ConversationRef is a stable lookup key for an active conversation. VK bot
// uses VKPeerID; Mini App uses ExternalThreadID scoped by backend UserID.
type ConversationRef struct {
	UserID           uuid.UUID
	AccountID        uuid.UUID
	Source           ConversationSource
	VKPeerID         int64
	ExternalThreadID string
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
