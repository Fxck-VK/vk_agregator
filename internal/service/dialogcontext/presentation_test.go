package dialogcontext_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/dialogcontext"
)

func TestPresentationUsesTopicAndPersistsAnswerWithoutChatContext(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewConversationRepo()
	service := dialogcontext.New(repo, dialogcontext.Config{})
	owner := uuid.New()
	conversation := &domain.Conversation{
		AccountID: owner, Source: domain.ConversationSourceWeb, Status: domain.ConversationActive,
		TitleOrigin: domain.ConversationTitleOriginAutoPending,
	}
	if err := repo.CreateConversation(ctx, conversation); err != nil {
		t.Fatal(err)
	}
	params, _ := json.Marshal(map[string]string{"conversation_id": conversation.ID.String(), "conversation_source": "web"})
	job := &domain.Job{
		ID: uuid.New(), AccountID: owner, Source: "web", Modality: domain.ModalityText, OperationType: domain.OperationTextGenerate,
		ResultMode: domain.ResultModeAccountHistory, ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb},
		IdempotencyKey: "web-presentation:" + owner.String() + ":" + uuid.NewString(), Params: params,
	}
	const prompt = "Подготовь презентацию на русском языке. Тема: Alfa Romeo.\n\nРовно 10 слайдов."
	const answer = "## Обложка\n- Alfa Romeo"
	prepared, err := service.Prepare(ctx, job, prompt)
	if err != nil || prepared.ConversationID != conversation.ID || prepared.Prompt != prompt {
		t.Fatalf("presentation was not bound to its conversation: %v", err)
	}
	conversation, err = repo.GetByIDForAccount(ctx, owner, conversation.ID)
	if err != nil || conversation.Title != "Презентация · Alfa Romeo" {
		t.Fatalf("expected a deterministic title from the topic: %v", err)
	}
	if err := service.Complete(ctx, job, conversation.ID, answer); err != nil {
		t.Fatal(err)
	}
	messages, err := repo.ListMessagesAfter(ctx, conversation.ID, 0, 10)
	if err != nil || len(messages) != 2 || messages[0].Text != prompt || messages[1].JobID != job.ID || messages[1].Text != answer {
		t.Fatalf("presentation request and answer were not durably stored: %v", err)
	}
}
