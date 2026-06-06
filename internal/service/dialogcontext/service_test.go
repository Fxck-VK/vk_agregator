package dialogcontext_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/dialogcontext"
)

func TestPrepareUsesRecentMessagesAndStoresCurrentPrompt(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewConversationRepo()
	svc := dialogcontext.New(repo, dialogcontext.Config{
		Enabled:             true,
		MaxInputTokens:      1600,
		MaxOutputTokens:     800,
		RecentMessagesLimit: 4,
	})
	userID := uuid.New()
	conv := &domain.Conversation{UserID: userID, VKPeerID: 55, Status: domain.ConversationActive}
	if err := repo.CreateConversation(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	_, _ = repo.UpsertMessage(ctx, msg(conv.ID, uuid.New(), domain.ConversationRoleUser, "помоги выбрать машину"))
	_, _ = repo.UpsertMessage(ctx, msg(conv.ID, uuid.New(), domain.ConversationRoleAssistant, "Сравним бюджет, надежность и задачи."))

	job := textJob(userID, 55)
	prepared, err := svc.Prepare(ctx, job, "а что лучше для города?")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if prepared.ConversationID != conv.ID {
		t.Fatalf("conversation id = %s, want %s", prepared.ConversationID, conv.ID)
	}
	if prepared.MaxOutputTokens != 800 {
		t.Fatalf("max output = %d, want 800", prepared.MaxOutputTokens)
	}
	for _, want := range []string{"Recent messages", "помоги выбрать машину", "а что лучше для города?"} {
		if !strings.Contains(prepared.Prompt, want) {
			t.Fatalf("prompt does not contain %q:\n%s", want, prepared.Prompt)
		}
	}
	messages, err := repo.ListMessagesAfter(ctx, conv.ID, 0, 10)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 3 || messages[2].Role != domain.ConversationRoleUser || messages[2].Text != "а что лучше для города?" {
		t.Fatalf("unexpected messages: %+v", messages)
	}
}

func TestCompleteStoresAssistantAndUpdatesSummary(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewConversationRepo()
	svc := dialogcontext.New(repo, dialogcontext.Config{
		Enabled:                true,
		SummaryMaxTokens:       80,
		RecentMessagesLimit:    2,
		SummarizeAfterMessages: 3,
		SummarizeAfterTokens:   1,
	})
	userID := uuid.New()
	job := textJob(userID, 77)
	prepared, err := svc.Prepare(ctx, job, "первый вопрос")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if err := svc.Complete(ctx, job, prepared.ConversationID, "первый ответ"); err != nil {
		t.Fatalf("complete 1: %v", err)
	}
	for i := 0; i < 3; i++ {
		j := textJob(userID, 77)
		p, err := svc.Prepare(ctx, j, "следующий вопрос")
		if err != nil {
			t.Fatalf("prepare %d: %v", i, err)
		}
		if err := svc.Complete(ctx, j, p.ConversationID, "следующий ответ"); err != nil {
			t.Fatalf("complete %d: %v", i, err)
		}
	}
	summary, err := repo.LatestSummary(ctx, prepared.ConversationID)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.SummarizedUntilSeq == 0 || !strings.Contains(summary.Text, "User:") {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestDisabledContextPassesPromptThrough(t *testing.T) {
	svc := dialogcontext.New(memory.NewConversationRepo(), dialogcontext.Config{Enabled: false, MaxOutputTokens: 700})
	prepared, err := svc.Prepare(context.Background(), textJob(uuid.New(), 99), "plain prompt")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if prepared.Prompt != "plain prompt" || prepared.ConversationID != uuid.Nil || prepared.MaxOutputTokens != 700 {
		t.Fatalf("unexpected prepared: %+v", prepared)
	}
}

func textJob(userID uuid.UUID, peerID int64) *domain.Job {
	return &domain.Job{
		ID:            uuid.New(),
		UserID:        userID,
		VKPeerID:      peerID,
		OperationType: domain.OperationTextGenerate,
		Modality:      domain.ModalityText,
	}
}

func msg(conversationID, jobID uuid.UUID, role domain.ConversationMessageRole, text string) *domain.ConversationMessage {
	return &domain.ConversationMessage{
		ConversationID: conversationID,
		JobID:          jobID,
		Role:           role,
		Text:           text,
		TokenCount:     dialogcontext.EstimateTokens(text),
	}
}
