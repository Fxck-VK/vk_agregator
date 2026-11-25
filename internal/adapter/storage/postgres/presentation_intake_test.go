package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/dialogcontext"
)

func TestPresentationIntakePostgresHistoryAndQueuedRecovery(t *testing.T) {
	ctx := context.Background()
	pool := preparedAccountIntegrationPool(t, ctx)
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"000046_web_account_conversations.up.sql", "000050_conversation_title_origin.up.sql", "000051_conversation_message_rating.up.sql"} {
		preparedRunMigrationFile(t, ctx, conn, filepath.Join(root, "migrations", name))
	}
	conn.Release()
	jobs := postgres.NewJobRepository(pool)
	conversations := postgres.NewConversationRepository(pool)
	owner, foreign := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO accounts (id) VALUES ($1), ($2)`, owner, foreign); err != nil {
		t.Fatal(err)
	}
	newConversation := func(reference, title string) *domain.Conversation {
		t.Helper()
		conversation := &domain.Conversation{
			AccountID: owner, Source: domain.ConversationSourceWeb, Status: domain.ConversationActive,
			ExternalThreadID: reference, Title: title,
		}
		if err := conversations.CreateConversation(ctx, conversation); err != nil {
			t.Fatal(err)
		}
		return conversation
	}
	intake := newConversation(domain.WebPresentationChatReference(uuid.New()), "Epifera Презентации")
	deck := newConversation("presentation-from-intake:"+uuid.NewString(), "")
	newJob := func(conversationID uuid.UUID, prefix, prompt string) *domain.Job {
		t.Helper()
		params, err := json.Marshal(map[string]string{
			"conversation_id": conversationID.String(), "conversation_source": "web", "model_name": "Qwen", "prompt": prompt,
		})
		if err != nil {
			t.Fatal(err)
		}
		job := &domain.Job{
			AccountID: owner, Source: "web", ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb}, ResultMode: domain.ResultModeAccountHistory,
			OperationType: domain.OperationTextGenerate, Modality: domain.ModalityText, Status: domain.JobStatusQueued,
			IdempotencyKey: prefix + uuid.NewString(), CorrelationID: uuid.NewString(), Params: params,
		}
		if err := jobs.Create(ctx, job); err != nil {
			t.Fatal(err)
		}
		return job
	}
	older := newJob(intake.ID, domain.WebPresentationIntakeJobKeyPrefix(owner), "Тема керамики")
	intakeLatest := newJob(intake.ID, domain.WebPresentationIntakeJobKeyPrefix(owner), "10 слайдов")
	if _, err := pool.Exec(ctx, `UPDATE jobs SET created_at=now()-interval '1 minute' WHERE id=$1`, older.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := jobs.GetLatestPresentationChatTurn(ctx, owner, intake.ID); err != nil || got.ID != intakeLatest.ID || got.Status != domain.JobStatusQueued {
		t.Fatalf("queued intake recovery = %#v, %v", got, err)
	}
	if got, err := jobs.GetLatestPresentationChatTurn(ctx, foreign, intake.ID); !errors.Is(err, domain.ErrNotFound) || got != nil {
		t.Fatalf("foreign intake recovery = %#v, %v", got, err)
	}
	const prompt = "Подготовь презентацию на русском языке. Тема: Керамика.\n\nРовно 10 слайдов."
	deckJob := newJob(deck.ID, domain.WebPresentationJobKeyPrefix(owner), prompt)
	if _, err := dialogcontext.New(conversations, dialogcontext.Config{}).Prepare(ctx, deckJob, prompt); err != nil {
		t.Fatal(err)
	}
	retainedDeck, err := conversations.GetByIDForAccount(ctx, owner, deck.ID)
	if err != nil || retainedDeck.Title != "Презентация · Керамика" {
		t.Fatalf("child conversation title = %#v, %v", retainedDeck, err)
	}
	items, err := postgres.NewPresentationHistoryRepository(pool).ListPresentationHistory(ctx, owner, 10, nil)
	if err != nil || len(items) != 1 || items[0].ConversationID != deck.ID || items[0].JobID != deckJob.ID || items[0].Title != retainedDeck.Title {
		t.Fatalf("history must contain only the separate deck: %#v, %v", items, err)
	}
}
