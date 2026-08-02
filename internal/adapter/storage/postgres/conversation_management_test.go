package postgres_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
)

func TestWebConversationManagementPostgresIntegration(t *testing.T) {
	ctx := context.Background()
	pool := conversationManagementIntegrationPool(t, ctx)
	repository := postgres.NewConversationRepository(pool)
	accountID := uuid.New()
	foreignAccountID := uuid.New()
	activeID := uuid.New()
	secondActiveID := uuid.New()
	archivedID := uuid.New()
	miniAppID := uuid.New()
	foreignID := uuid.New()
	createdAt := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)

	for _, id := range []uuid.UUID{accountID, foreignAccountID} {
		if _, err := pool.Exec(ctx, `INSERT INTO accounts (id) VALUES ($1)`, id); err != nil {
			t.Fatalf("seed account: %v", err)
		}
	}
	seedConversation := func(id, owner uuid.UUID, source domain.ConversationSource, status domain.ConversationStatus, title string, updatedAt time.Time) {
		t.Helper()
		if _, err := pool.Exec(ctx, `
			INSERT INTO conversations (id, account_id, source, status, title, external_thread_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			id, owner, source, status, title, "thread-"+id.String(), createdAt, updatedAt); err != nil {
			t.Fatalf("seed conversation: %v", err)
		}
	}
	seedConversation(activeID, accountID, domain.ConversationSourceWeb, domain.ConversationActive, "Rename me", createdAt.Add(time.Minute))
	seedConversation(secondActiveID, accountID, domain.ConversationSourceWeb, domain.ConversationActive, "Keep active", createdAt.Add(2*time.Minute))
	seedConversation(archivedID, accountID, domain.ConversationSourceWeb, domain.ConversationArchived, "Already archived", createdAt.Add(3*time.Minute))
	seedConversation(miniAppID, accountID, domain.ConversationSourceMiniApp, domain.ConversationActive, "Mini App", createdAt.Add(4*time.Minute))
	seedConversation(foreignID, foreignAccountID, domain.ConversationSourceWeb, domain.ConversationActive, "Foreign", createdAt.Add(5*time.Minute))
	if _, err := pool.Exec(ctx, `INSERT INTO conversation_messages (id, conversation_id, text) VALUES ($1, $2, $3)`, uuid.New(), activeID, "historical message"); err != nil {
		t.Fatalf("seed historical message: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO conversation_summaries (id, conversation_id, text) VALUES ($1, $2, $3)`, uuid.New(), activeID, "historical summary"); err != nil {
		t.Fatalf("seed historical summary: %v", err)
	}

	listed, err := repository.ListActiveByAccountSource(ctx, accountID, domain.ConversationSourceWeb, 10, 0)
	if err != nil {
		t.Fatalf("list active web conversations: %v", err)
	}
	if len(listed) != 2 || listed[0].ID != secondActiveID || listed[1].ID != activeID {
		t.Fatalf("active web list = %#v, want only %s then %s", conversationIDs(listed), secondActiveID, activeID)
	}

	renamed, err := repository.RenameActiveConversationForAccount(ctx, accountID, activeID, domain.ConversationSourceWeb, "Renamed title")
	if err != nil {
		t.Fatalf("rename active conversation: %v", err)
	}
	if renamed.ID != activeID || renamed.AccountID != accountID || renamed.Source != domain.ConversationSourceWeb || renamed.Status != domain.ConversationActive || renamed.Title != "Renamed title" {
		t.Fatalf("renamed conversation = %#v", renamed)
	}
	for name, tc := range map[string]struct {
		accountID      uuid.UUID
		conversationID uuid.UUID
		source         domain.ConversationSource
	}{
		"foreign account": {accountID: accountID, conversationID: foreignID, source: domain.ConversationSourceWeb},
		"wrong source":    {accountID: accountID, conversationID: miniAppID, source: domain.ConversationSourceWeb},
		"archived":        {accountID: accountID, conversationID: archivedID, source: domain.ConversationSourceWeb},
		"missing":         {accountID: accountID, conversationID: uuid.New(), source: domain.ConversationSourceWeb},
	} {
		t.Run("rename rejects "+name, func(t *testing.T) {
			got, err := repository.RenameActiveConversationForAccount(ctx, tc.accountID, tc.conversationID, tc.source, "Must not change")
			if !errors.Is(err, domain.ErrNotFound) || got != nil {
				t.Fatalf("rename = %#v, %v; want nil, %v", got, err, domain.ErrNotFound)
			}
		})
	}

	if err := repository.ArchiveConversationForAccount(ctx, accountID, activeID, domain.ConversationSourceWeb); err != nil {
		t.Fatalf("archive active conversation: %v", err)
	}
	var firstArchivedAt time.Time
	if err := pool.QueryRow(ctx, `SELECT updated_at FROM conversations WHERE id = $1`, activeID).Scan(&firstArchivedAt); err != nil {
		t.Fatalf("read first archived timestamp: %v", err)
	}
	if err := repository.ArchiveConversationForAccount(ctx, accountID, activeID, domain.ConversationSourceWeb); err != nil {
		t.Fatalf("repeat archive active conversation: %v", err)
	}
	var status domain.ConversationStatus
	var secondArchivedAt time.Time
	if err := pool.QueryRow(ctx, `SELECT status, updated_at FROM conversations WHERE id = $1`, activeID).Scan(&status, &secondArchivedAt); err != nil {
		t.Fatalf("read repeated archive state: %v", err)
	}
	if status != domain.ConversationArchived || !secondArchivedAt.Equal(firstArchivedAt) {
		t.Fatalf("repeated archive state = %q at %s, want archived with unchanged timestamp %s", status, secondArchivedAt, firstArchivedAt)
	}
	for name, id := range map[string]uuid.UUID{
		"foreign account": foreignID,
		"wrong source":    miniAppID,
		"missing":         uuid.New(),
	} {
		t.Run("archive rejects "+name, func(t *testing.T) {
			if err := repository.ArchiveConversationForAccount(ctx, accountID, id, domain.ConversationSourceWeb); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("archive error = %v, want %v", err, domain.ErrNotFound)
			}
		})
	}

	listed, err = repository.ListActiveByAccountSource(ctx, accountID, domain.ConversationSourceWeb, 10, 0)
	if err != nil {
		t.Fatalf("list after archive: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != secondActiveID {
		t.Fatalf("active web list after archive = %#v, want only %s", conversationIDs(listed), secondActiveID)
	}
	var messages, summaries int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM conversation_messages WHERE conversation_id = $1`, activeID).Scan(&messages); err != nil {
		t.Fatalf("count historical messages: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM conversation_summaries WHERE conversation_id = $1`, activeID).Scan(&summaries); err != nil {
		t.Fatalf("count historical summaries: %v", err)
	}
	if messages != 1 || summaries != 1 {
		t.Fatalf("historical rows messages/summaries = %d/%d, want 1/1", messages, summaries)
	}
}

func conversationManagementIntegrationPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping isolated PostgreSQL conversation-management integration test")
	}
	adminConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("TEST_DATABASE_URL is invalid")
	}
	admin, err := pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		t.Fatal("connect PostgreSQL integration admin pool failed")
	}
	if err := admin.Ping(ctx); err != nil {
		admin.Close()
		t.Fatal("ping PostgreSQL integration database failed")
	}
	schema := "conversation_management_" + strings.ReplaceAll(uuid.NewString(), "-", "_")
	quotedSchema := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quotedSchema); err != nil {
		admin.Close()
		t.Fatalf("create isolated integration schema: %v", err)
	}

	testConfig := adminConfig.Copy()
	testConfig.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, testConfig)
	if err != nil {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+quotedSchema+" CASCADE")
		admin.Close()
		t.Fatal("connect isolated PostgreSQL integration pool failed")
	}
	t.Cleanup(func() {
		pool.Close()
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = admin.Exec(cleanupCtx, "DROP SCHEMA IF EXISTS "+quotedSchema+" CASCADE")
		admin.Close()
	})

	if _, err := pool.Exec(ctx, `
		CREATE TABLE accounts (
			id UUID PRIMARY KEY
		);
		CREATE TABLE conversations (
			id UUID PRIMARY KEY,
			user_id UUID,
			account_id UUID REFERENCES accounts (id),
			source TEXT NOT NULL CHECK (source IN ('vk_bot', 'miniapp', 'web')),
			vk_peer_id BIGINT NOT NULL DEFAULT 0,
			external_thread_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL CHECK (status IN ('active', 'archived')),
			title TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE conversation_messages (
			id UUID PRIMARY KEY,
			conversation_id UUID NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
			text TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE conversation_summaries (
			id UUID PRIMARY KEY,
			conversation_id UUID NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
			text TEXT NOT NULL DEFAULT ''
		)`); err != nil {
		t.Fatalf("create isolated conversation schema: %v", err)
	}
	return pool
}

func conversationIDs(conversations []*domain.Conversation) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(conversations))
	for _, conversation := range conversations {
		ids = append(ids, conversation.ID)
	}
	return ids
}
