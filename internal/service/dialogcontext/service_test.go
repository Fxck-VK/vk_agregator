package dialogcontext_test

import (
	"context"
	"encoding/json"
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

func TestPrepareUsesExplicitMiniAppThreadsWithoutMixing(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewConversationRepo()
	svc := dialogcontext.New(repo, dialogcontext.Config{Enabled: true, RecentMessagesLimit: 4})
	userID := uuid.New()

	jobA := textJobWithParams(userID, 0, map[string]string{
		"conversation_source": "miniapp",
		"external_thread_id":  "thread-a",
	})
	preparedA, err := svc.Prepare(ctx, jobA, "thread a first")
	if err != nil {
		t.Fatalf("prepare a: %v", err)
	}
	if preparedA.ConversationID == uuid.Nil {
		t.Fatal("prepare a conversation id is nil")
	}
	if err := svc.Complete(ctx, jobA, preparedA.ConversationID, "thread a answer"); err != nil {
		t.Fatalf("complete a: %v", err)
	}

	jobB := textJobWithParams(userID, 0, map[string]string{
		"conversation_source": "miniapp",
		"external_thread_id":  "thread-b",
	})
	preparedB, err := svc.Prepare(ctx, jobB, "thread b first")
	if err != nil {
		t.Fatalf("prepare b: %v", err)
	}
	if preparedB.ConversationID == uuid.Nil {
		t.Fatal("prepare b conversation id is nil")
	}
	if preparedB.ConversationID == preparedA.ConversationID {
		t.Fatalf("miniapp threads mixed: %s", preparedA.ConversationID)
	}
	if err := svc.Complete(ctx, jobB, preparedB.ConversationID, "thread b answer"); err != nil {
		t.Fatalf("complete b: %v", err)
	}

	jobA2 := textJobWithParams(userID, 0, map[string]string{
		"conversation_source": "miniapp",
		"external_thread_id":  "thread-a",
	})
	preparedA2, err := svc.Prepare(ctx, jobA2, "thread a follow up")
	if err != nil {
		t.Fatalf("prepare a2: %v", err)
	}
	if preparedA2.ConversationID != preparedA.ConversationID {
		t.Fatalf("thread a id = %s, want %s", preparedA2.ConversationID, preparedA.ConversationID)
	}
	if !strings.Contains(preparedA2.Prompt, "thread a first") || !strings.Contains(preparedA2.Prompt, "thread a answer") {
		t.Fatalf("thread a prompt lost own context:\n%s", preparedA2.Prompt)
	}
	if strings.Contains(preparedA2.Prompt, "thread b first") || strings.Contains(preparedA2.Prompt, "thread b answer") {
		t.Fatalf("thread a prompt contains thread b context:\n%s", preparedA2.Prompt)
	}

	conversations, err := repo.ListByUserSource(ctx, userID, domain.ConversationSourceMiniApp, 10, 0)
	if err != nil {
		t.Fatalf("list miniapp conversations: %v", err)
	}
	if len(conversations) != 2 {
		t.Fatalf("miniapp conversations = %d, want 2", len(conversations))
	}
}

func TestPrepareSetsMiniAppConversationTitleFromFirstUserPrompt(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewConversationRepo()
	svc := dialogcontext.New(repo, dialogcontext.Config{Enabled: true})
	userID := uuid.New()

	job := textJobWithParams(userID, 0, map[string]string{
		"conversation_source": "miniapp",
		"external_thread_id":  "title-thread",
	})
	if _, err := svc.Prepare(ctx, job, "  Как сделать презентацию?  "); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	conversation := mustMiniAppConversation(t, repo, userID, "title-thread")
	if conversation.Title != "Как сделать презентацию?" {
		t.Fatalf("title = %q, want %q", conversation.Title, "Как сделать презентацию?")
	}
}

func TestPrepareDoesNotOverwriteMiniAppConversationTitle(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewConversationRepo()
	svc := dialogcontext.New(repo, dialogcontext.Config{Enabled: true})
	userID := uuid.New()

	firstJob := textJobWithParams(userID, 0, map[string]string{
		"conversation_source": "miniapp",
		"external_thread_id":  "stable-title-thread",
	})
	if _, err := svc.Prepare(ctx, firstJob, "Первый вопрос"); err != nil {
		t.Fatalf("prepare first: %v", err)
	}
	secondJob := textJobWithParams(userID, 0, map[string]string{
		"conversation_source": "miniapp",
		"external_thread_id":  "stable-title-thread",
	})
	if _, err := svc.Prepare(ctx, secondJob, "Второй вопрос"); err != nil {
		t.Fatalf("prepare second: %v", err)
	}

	conversation := mustMiniAppConversation(t, repo, userID, "stable-title-thread")
	if conversation.Title != "Первый вопрос" {
		t.Fatalf("title = %q, want first prompt", conversation.Title)
	}
}

func TestPrepareLeavesMiniAppConversationTitleEmptyForWhitespacePrompt(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewConversationRepo()
	svc := dialogcontext.New(repo, dialogcontext.Config{Enabled: true})
	userID := uuid.New()

	job := textJobWithParams(userID, 0, map[string]string{
		"conversation_source": "miniapp",
		"external_thread_id":  "empty-title-thread",
	})
	if _, err := svc.Prepare(ctx, job, " \n\t "); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	conversation := mustMiniAppConversation(t, repo, userID, "empty-title-thread")
	if conversation.Title != "" {
		t.Fatalf("title = %q, want empty fallback title in API", conversation.Title)
	}
}

func TestPrepareTruncatesMiniAppConversationTitle(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewConversationRepo()
	svc := dialogcontext.New(repo, dialogcontext.Config{Enabled: true})
	userID := uuid.New()

	job := textJobWithParams(userID, 0, map[string]string{
		"conversation_source": "miniapp",
		"external_thread_id":  "long-title-thread",
	})
	if _, err := svc.Prepare(ctx, job, strings.Repeat("а", 100)); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	conversation := mustMiniAppConversation(t, repo, userID, "long-title-thread")
	want := strings.Repeat("а", 77) + "..."
	if conversation.Title != want {
		t.Fatalf("title runes = %d, title = %q, want %q", len([]rune(conversation.Title)), conversation.Title, want)
	}
}

func TestPrepareSeparatesVKBotAndMiniAppForSameUser(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewConversationRepo()
	svc := dialogcontext.New(repo, dialogcontext.Config{Enabled: true, RecentMessagesLimit: 4})
	userID := uuid.New()
	vkConversation := &domain.Conversation{UserID: userID, VKPeerID: 55, Status: domain.ConversationActive}
	if err := repo.CreateConversation(ctx, vkConversation); err != nil {
		t.Fatalf("create vk conversation: %v", err)
	}
	_, _ = repo.UpsertMessage(ctx, msg(vkConversation.ID, uuid.New(), domain.ConversationRoleUser, "vk only memory"))
	_, _ = repo.UpsertMessage(ctx, msg(vkConversation.ID, uuid.New(), domain.ConversationRoleAssistant, "vk only answer"))

	miniJob := textJobWithParams(userID, 55, map[string]string{
		"conversation_source": "miniapp",
		"external_thread_id":  "default",
	})
	prepared, err := svc.Prepare(ctx, miniJob, "miniapp prompt")
	if err != nil {
		t.Fatalf("prepare miniapp: %v", err)
	}
	if prepared.ConversationID == uuid.Nil || prepared.ConversationID == vkConversation.ID {
		t.Fatalf("unexpected miniapp conversation id: %s", prepared.ConversationID)
	}
	if strings.Contains(prepared.Prompt, "vk only memory") || strings.Contains(prepared.Prompt, "vk only answer") {
		t.Fatalf("miniapp prompt contains vk bot context:\n%s", prepared.Prompt)
	}
}

func TestPrepareInvalidExplicitReferencePassesPromptThrough(t *testing.T) {
	ctx := context.Background()
	svc := dialogcontext.New(memory.NewConversationRepo(), dialogcontext.Config{Enabled: true, MaxOutputTokens: 900})
	userID := uuid.New()

	for name, params := range map[string]map[string]string{
		"empty miniapp thread": {
			"conversation_source": "miniapp",
		},
		"unknown source": {
			"conversation_source": "unknown",
			"external_thread_id":  "thread",
		},
		"missing conversation id": {
			"conversation_source": "miniapp",
			"conversation_id":     uuid.NewString(),
		},
	} {
		t.Run(name, func(t *testing.T) {
			prepared, err := svc.Prepare(ctx, textJobWithParams(userID, 0, params), "raw prompt")
			if err != nil {
				t.Fatalf("prepare: %v", err)
			}
			if prepared.Prompt != "raw prompt" || prepared.ConversationID != uuid.Nil || prepared.MaxOutputTokens != 900 {
				t.Fatalf("unexpected prepared: %+v", prepared)
			}
		})
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

func textJobWithParams(userID uuid.UUID, peerID int64, params map[string]string) *domain.Job {
	job := textJob(userID, peerID)
	raw, err := json.Marshal(params)
	if err != nil {
		panic(err)
	}
	job.Params = raw
	return job
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

func mustMiniAppConversation(t *testing.T, repo domain.ConversationRepository, userID uuid.UUID, threadID string) *domain.Conversation {
	t.Helper()
	conversation, err := repo.GetActiveByReference(context.Background(), domain.ConversationRef{
		UserID:           userID,
		Source:           domain.ConversationSourceMiniApp,
		ExternalThreadID: threadID,
	})
	if err != nil {
		t.Fatalf("get miniapp conversation: %v", err)
	}
	return conversation
}
