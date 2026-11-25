package dialogcontext

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/presentationintake"
)

// PresentationIntakeMaxTurnBytes leaves room for earlier evidence and the
// backend-owned system prompt within the paid route's byte budget.
const PresentationIntakeMaxTurnBytes = 2800

// ErrInvalidPresentationIntakeOutput marks immutable provider output that no
// retry of a storage checkpoint can recover.
var ErrInvalidPresentationIntakeOutput = errors.New("dialogcontext: invalid presentation intake answer")

func encodeIntakeMessages(messages []string) (string, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(messages); err != nil {
		return "", err
	}
	return strings.TrimSuffix(output.String(), "\n"), nil
}

func ValidPresentationIntakePrompt(prompt string) bool {
	if strings.TrimSpace(prompt) == "" || len(prompt) > PresentationIntakeMaxTurnBytes || !utf8.ValidString(prompt) {
		return false
	}
	raw, err := encodeIntakeMessages([]string{prompt})
	return err == nil && intakeMessagesFit(raw)
}

func intakeMessagesFit(raw string) bool {
	return len(raw)+len(presentationintake.SystemPrompt) <= domain.PaidTextMaxInputTokens-512
}

// PresentationIntakeEvidence reads the retained user turns up to one exact
// intake job. Assistant text never supplies evidence for a generation action.
// The same bounded history is used for the worker request and server validation.
func PresentationIntakeEvidence(ctx context.Context, repo domain.ConversationRepository, conversationID, jobID uuid.UUID) ([]string, string, error) {
	message, err := repo.GetUserMessageByJobID(ctx, jobID)
	if err != nil {
		return nil, "", err
	}
	if message == nil || message.ConversationID != conversationID || message.JobID != jobID || message.Role != domain.ConversationRoleUser || message.Seq <= 0 || !ValidPresentationIntakePrompt(message.Text) {
		return nil, "", domain.ErrInvalidInput
	}
	recent, err := repo.ListRecentMessagesBefore(ctx, conversationID, message.Seq, 0, 48)
	if err != nil {
		return nil, "", err
	}
	messages := make([]string, 0, len(recent)+1)
	for _, earlier := range recent {
		if earlier == nil || earlier.ConversationID != conversationID || earlier.Seq >= message.Seq {
			return nil, "", domain.ErrInvalidInput
		}
		if earlier.Role == domain.ConversationRoleUser && ValidPresentationIntakePrompt(earlier.Text) {
			messages = append(messages, earlier.Text)
		}
	}
	messages = append(messages, message.Text)
	for len(messages) > 0 {
		raw, err := encodeIntakeMessages(messages)
		if err != nil {
			return nil, "", err
		}
		if intakeMessagesFit(raw) {
			return messages, raw, nil
		}
		messages = messages[1:]
	}
	return nil, "", errors.New("dialogcontext: presentation intake evidence exceeds the provider budget")
}
