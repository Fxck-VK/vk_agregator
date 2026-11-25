package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// PresentationHistoryRow contains the inputs for an account-owned list item.
// TitlePrompt is a bounded prefix of a retained message, used only to derive a
// display title for older jobs. It is not a public list field.
type PresentationHistoryRow struct {
	AccountID      uuid.UUID
	ConversationID uuid.UUID
	JobID          uuid.UUID
	Title          string
	TitleOrigin    ConversationTitleOrigin
	TitlePrompt    string
	Status         JobStatus
	CreatedAt      time.Time
	ModelName      string
	ErrorCode      string
}

// WebPresentationJobKeyPrefix binds the durable presentation marker to the
// server-verified account that created it.
func WebPresentationJobKeyPrefix(accountID uuid.UUID) string {
	return "web-presentation:" + accountID.String() + ":"
}

// WebPresentationIntakeJobKeyPrefix identifies billed assistant turns, which do
// not appear in the generated presentation history.
func WebPresentationIntakeJobKeyPrefix(accountID uuid.UUID) string {
	return "web-presentation-intake:" + accountID.String() + ":"
}

const webPresentationChatPrefix = "presentation-intake:"

func WebPresentationChatReference(key uuid.UUID) string {
	return webPresentationChatPrefix + key.String()
}

func (c *Conversation) IsWebPresentationChat() bool {
	return c != nil && c.Source == ConversationSourceWeb && c.AccountID != uuid.Nil && validUUIDSuffix(c.ExternalThreadID, webPresentationChatPrefix)
}

func validUUIDSuffix(value, prefix string) bool {
	key, ok := strings.CutPrefix(value, prefix)
	if !ok {
		return false
	}
	id, err := uuid.Parse(key)
	return err == nil && id != uuid.Nil && key == id.String()
}

// WebPresentationTitle gives a generated presentation a stable display title
// derived only from the topic field of the supported presentation prompt.
func WebPresentationTitle(prompt string) string {
	firstParagraph, _, _ := strings.Cut(prompt, "\n\n")
	_, topic, hasTopic := strings.Cut(firstParagraph, "Тема: ")
	topic = strings.TrimSuffix(strings.Join(strings.Fields(topic), " "), ".")
	if !hasTopic || topic == "" {
		return "Презентация"
	}
	return BoundedWebPresentationTitle("Презентация · " + topic)
}

func BoundedWebPresentationTitle(raw string) string {
	const maxRunes = 80
	title := strings.Join(strings.Fields(raw), " ")
	runes := []rune(title)
	if len(runes) <= maxRunes {
		return title
	}
	return strings.TrimSpace(string(runes[:maxRunes-3])) + "..."
}
