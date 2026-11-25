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
