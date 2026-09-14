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

func TestWebMediaPersistsInSameConversationWithoutSendingHistoryToProvider(t *testing.T) {
	for _, modality := range []domain.Modality{domain.ModalityImage, domain.ModalityVideo} {
		t.Run(string(modality), func(t *testing.T) {
			ctx := context.Background()
			repo := memory.NewConversationRepo()
			svc := dialogcontext.New(repo, dialogcontext.Config{Enabled: modality == domain.ModalityImage})
			owner := uuid.New()
			conv := &domain.Conversation{AccountID: owner, Source: domain.ConversationSourceWeb, Status: domain.ConversationActive}
			if err := repo.CreateConversation(ctx, conv); err != nil {
				t.Fatal(err)
			}
			_, _ = repo.UpsertMessage(ctx, msg(conv.ID, uuid.New(), domain.ConversationRoleUser, "Earlier text"))
			params, _ := json.Marshal(map[string]string{"conversation_id": conv.ID.String(), "conversation_source": "web"})
			operation := domain.OperationImageGenerate
			if modality == domain.ModalityVideo {
				operation = domain.OperationVideoGenerate
			}
			job := &domain.Job{ID: uuid.New(), AccountID: owner, Source: "web", OperationType: operation, Modality: modality, Params: params}
			for range 2 {
				prepared, err := svc.Prepare(ctx, job, "A crane on a cloud")
				if err != nil || prepared.ConversationID != conv.ID || prepared.Prompt != "A crane on a cloud" {
					t.Fatalf("media preparation failed: %v", err)
				}
				if err := svc.Complete(ctx, job, conv.ID, "Generated media"); err != nil {
					t.Fatal(err)
				}
			}
			messages, err := repo.ListMessagesAfter(ctx, conv.ID, 0, 20)
			if err != nil || len(messages) != 3 || messages[2].Role != domain.ConversationRoleAssistant || messages[2].JobID != job.ID {
				t.Fatal("media turn missing or duplicated")
			}
			job.AccountID = uuid.New()
			job.ID = uuid.New()
			prepared, err := svc.Prepare(ctx, job, "Foreign prompt")
			if err != nil || prepared.ConversationID != uuid.Nil {
				t.Fatal("foreign conversation accepted")
			}
			if err := svc.Complete(ctx, job, conv.ID, "Foreign answer"); err != nil {
				t.Fatal(err)
			}
			messages, _ = repo.ListMessagesAfter(ctx, conv.ID, 0, 20)
			if len(messages) != 3 {
				t.Fatal("foreign media appended")
			}
		})
	}
}
