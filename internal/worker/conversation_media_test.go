package worker

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"testing"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/dialogcontext"
)

func TestWebMediaCompletionUsesConversationAndSafeCaption(t *testing.T) {
	for _, modality := range []domain.Modality{domain.ModalityImage, domain.ModalityVideo} {
		id := uuid.New()
		textContext := &mediaTestContext{conversationID: id}
		processor := &processor{textContext: textContext}
		params, _ := json.Marshal(map[string]string{"conversation_id": id.String(), "conversation_source": "web"})
		op := domain.OperationImageGenerate
		expected := "Изображение готово."
		if modality == domain.ModalityVideo {
			op = domain.OperationVideoGenerate
			expected = "Видео готово."
		}
		job := &domain.Job{ID: uuid.New(), AccountID: uuid.New(), Source: "web", OperationType: op, Modality: modality, Params: params}
		if err := processor.saveDialogAnswer(context.Background(), job, "untrusted provider caption"); err != nil {
			t.Fatal(err)
		}
		if textContext.completeCalls != 1 || textContext.completedAnswer != expected {
			t.Fatal("media completion not linked to conversation")
		}
		job.Source = "miniapp"
		if err := processor.saveDialogAnswer(context.Background(), job, ""); err != nil {
			t.Fatal(err)
		}
		if textContext.completeCalls != 1 {
			t.Fatal("non-web media affected")
		}
	}
}

type mediaTestContext struct {
	conversationID  uuid.UUID
	completeCalls   int
	completedAnswer string
}

func (f *mediaTestContext) Prepare(_ context.Context, _ *domain.Job, prompt string) (dialogcontext.Prepared, error) {
	return dialogcontext.Prepared{Prompt: prompt, ConversationID: f.conversationID}, nil
}
func (f *mediaTestContext) Complete(_ context.Context, _ *domain.Job, id uuid.UUID, answer string) error {
	if id == f.conversationID {
		f.completeCalls++
		f.completedAnswer = answer
	}
	return nil
}
