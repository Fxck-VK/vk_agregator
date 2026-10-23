package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
)

func TestConversationFirstModelPostgresIntegration(t *testing.T) {
	ctx := context.Background()
	pool := conversationManagementIntegrationPool(t, ctx)
	index, err := os.ReadFile("../../../../migrations/000059_conversation_first_model.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(index)); err != nil {
		t.Fatal(err)
	}
	owner, foreign, id := uuid.New(), uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO accounts (id) VALUES ($1), ($2);`, owner, foreign); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO conversations (id, account_id, source, status) VALUES ($1, $2, 'web', 'active')`, id, owner); err != nil {
		t.Fatal(err)
	}
	seed := func(account uuid.UUID, status, model, when string) {
		t.Helper()
		if _, err := pool.Exec(ctx, `INSERT INTO jobs VALUES ($1, $2, 'web', $3, jsonb_build_object('conversation_id', $4::text, 'conversation_source', 'web', 'model_id', $5::text), $6::timestamptz)`, uuid.New(), account, status, id.String(), model, when); err != nil {
			t.Fatal(err)
		}
	}
	seed(foreign, "succeeded", "foreign", "2026-01-01T00:00:00Z")
	seed(owner, "awaiting_payment", "rejected", "2026-01-02T00:00:00Z")
	seed(owner, "failed", "claude_sonnet_4_6", "2026-01-03T00:00:00Z")
	seed(owner, "succeeded", "gpt_5", "2026-01-04T00:00:00Z")
	repo := postgres.NewConversationRepository(pool)
	assertModel := func() {
		t.Helper()
		conversation, err := repo.GetByIDForAccount(ctx, owner, id)
		if err != nil || conversation.FirstModelID != "claude_sonnet_4_6" {
			t.Fatalf("first model = %#v, %v", conversation, err)
		}
		list, err := repo.ListActiveByAccountSource(ctx, owner, domain.ConversationSourceWeb, 20, 0)
		if err != nil || len(list) != 1 || list[0].FirstModelID != "claude_sonnet_4_6" {
			t.Fatalf("list = %#v, %v", list, err)
		}
	}
	assertModel()
	seed(owner, "succeeded", "gemini_3_pro", "2026-01-05T00:00:00Z")
	if _, err := repo.RenameActiveConversationForAccount(ctx, owner, id, domain.ConversationSourceWeb, "Renamed"); err != nil {
		t.Fatal(err)
	}
	assertModel()
}
