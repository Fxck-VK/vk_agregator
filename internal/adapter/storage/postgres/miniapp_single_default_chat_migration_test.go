package postgres_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"vk-ai-aggregator/internal/adapter/storage/postgres"
)

func TestMiniAppSingleDefaultChatMigrationFilesDeclareIrreversibleMerge(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}

	upRaw, err := os.ReadFile(filepath.Join(root, "migrations", "000030_miniapp_single_default_chat.up.sql"))
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	up := string(upRaw)
	requiredUp := []string{
		"BEGIN;",
		"external_thread_id = 'default'",
		"UPDATE conversation_messages",
		"UPDATE conversation_summaries",
		"redacted_at",
		"jsonb_set",
		"- 'conversation_id'",
		"status = 'archived'",
		"COMMIT;",
	}
	for _, required := range requiredUp {
		if !strings.Contains(up, required) {
			t.Fatalf("up migration missing %q", required)
		}
	}
	forbiddenUp := []string{
		"DELETE FROM",
		"DROP TABLE",
		"TRUNCATE ",
	}
	for _, forbidden := range forbiddenUp {
		if strings.Contains(strings.ToUpper(up), forbidden) {
			t.Fatalf("up migration must stay non-destructive for deploy safety, found %q", forbidden)
		}
	}

	downRaw, err := os.ReadFile(filepath.Join(root, "migrations", "000030_miniapp_single_default_chat.down.sql"))
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	down := strings.ToLower(string(downRaw))
	if !strings.Contains(down, "irreversible") || !strings.Contains(down, "no-op") {
		t.Fatalf("down migration must declare irreversible no-op merge, got: %s", string(downRaw))
	}
}

func TestMiniAppSingleDefaultChatMigrationMergesCustomThreads(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL migration test")
	}
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire connection: %v", err)
	}
	defer conn.Release()

	schema := "miniapp_single_default_chat_" + strings.ReplaceAll(uuid.NewString(), "-", "_")
	if _, err := conn.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	defer func() {
		_, _ = conn.Exec(ctx, `RESET search_path`)
		_, _ = conn.Exec(ctx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	}()
	if _, err := conn.Exec(ctx, `SET search_path TO `+schema+`, public`); err != nil {
		t.Fatalf("set search path: %v", err)
	}

	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	for _, name := range []string{
		"000001_init_schema.up.sql",
		"000006_conversation_context.up.sql",
		"000008_conversation_sources.up.sql",
		"000020_jobs_source.up.sql",
		"000021_retention_schema.up.sql",
	} {
		runMigrationFile(t, ctx, conn, filepath.Join(root, "migrations", name))
	}

	userOneID := insertMiniAppMigrationUser(t, ctx, conn, uniqueVKID())
	userTwoID := insertMiniAppMigrationUser(t, ctx, conn, uniqueVKID())
	defaultOneID := insertMiniAppMigrationConversation(t, ctx, conn, userOneID, 7001, "miniapp", "default", "Default")
	customOneID := insertMiniAppMigrationConversation(t, ctx, conn, userOneID, 7001, "miniapp", "custom-one", "Custom one")
	customTwoID := insertMiniAppMigrationConversation(t, ctx, conn, userTwoID, 7002, "miniapp", "custom-two", "Custom two")
	vkBotID := insertMiniAppMigrationConversation(t, ctx, conn, userOneID, 7001, "vk_bot", "", "VK bot")

	defaultJobID := insertMiniAppMigrationJob(t, ctx, conn, userOneID, "miniapp", "text_generate", map[string]string{"external_thread_id": "default"})
	customJobID := insertMiniAppMigrationJob(t, ctx, conn, userOneID, "miniapp", "text_generate", map[string]string{
		"prompt":              "custom prompt",
		"conversation_id":     "custom-one",
		"conversation_source": "miniapp",
		"external_thread_id":  "custom-one",
	})
	customTwoJobID := insertMiniAppMigrationJob(t, ctx, conn, userTwoID, "miniapp", "text_generate", map[string]string{"external_thread_id": "custom-two"})
	imageJobID := insertMiniAppMigrationJob(t, ctx, conn, userOneID, "miniapp", "image_generate", map[string]string{"external_thread_id": "image-thread"})
	vkBotJobID := insertMiniAppMigrationJob(t, ctx, conn, userOneID, "vk_bot", "text_generate", map[string]string{"external_thread_id": "vk-thread"})

	insertMiniAppMigrationMessage(t, ctx, conn, defaultOneID, defaultJobID, "user", "default message")
	insertMiniAppMigrationMessage(t, ctx, conn, customOneID, customJobID, "user", "custom one message")
	insertMiniAppMigrationMessage(t, ctx, conn, customTwoID, customTwoJobID, "user", "custom two message")
	insertMiniAppMigrationMessage(t, ctx, conn, vkBotID, vkBotJobID, "user", "vk bot message")
	insertMiniAppMigrationSummary(t, ctx, conn, defaultOneID, "default summary")
	insertMiniAppMigrationSummary(t, ctx, conn, customOneID, "custom summary")

	runMigrationFile(t, ctx, conn, filepath.Join(root, "migrations", "000030_miniapp_single_default_chat.up.sql"))

	assertMiniAppConversationCount(t, ctx, conn, userOneID, "miniapp", "default", 1)
	assertMiniAppConversationStatus(t, ctx, conn, userOneID, "miniapp", "custom-one", "archived")
	assertMiniAppConversationStatus(t, ctx, conn, userTwoID, "miniapp", "custom-two", "archived")
	assertMiniAppConversationCount(t, ctx, conn, userOneID, "vk_bot", "", 1)

	userTwoDefaultID := getMiniAppMigrationConversationID(t, ctx, conn, userTwoID, "miniapp", "default")
	assertMiniAppMessageTexts(t, ctx, conn, defaultOneID, []string{"default message", "custom one message"})
	assertMiniAppMessageTexts(t, ctx, conn, userTwoDefaultID, []string{"custom two message"})
	assertMiniAppMessageTexts(t, ctx, conn, vkBotID, []string{"vk bot message"})
	assertMiniAppSummaryText(t, ctx, conn, defaultOneID, "default summary")
	assertMiniAppSummaryRedacted(t, ctx, conn, customOneID)

	assertMiniAppJobParams(t, ctx, conn, customJobID, "default", true)
	assertMiniAppJobParams(t, ctx, conn, customTwoJobID, "default", true)
	assertMiniAppJobParams(t, ctx, conn, imageJobID, "image-thread", false)
	assertMiniAppJobParams(t, ctx, conn, vkBotJobID, "vk-thread", false)
}

func insertMiniAppMigrationUser(t *testing.T, ctx context.Context, execer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, vkUserID int64) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := execer.QueryRow(ctx, `INSERT INTO users (vk_user_id) VALUES ($1) RETURNING id`, vkUserID).Scan(&id); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

func insertMiniAppMigrationConversation(t *testing.T, ctx context.Context, execer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, userID uuid.UUID, vkPeerID int64, source, externalThreadID, title string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := execer.QueryRow(ctx, `
INSERT INTO conversations (user_id, vk_peer_id, source, external_thread_id, title)
VALUES ($1, $2, $3, $4, $5)
RETURNING id`, userID, vkPeerID, source, externalThreadID, title).Scan(&id); err != nil {
		t.Fatalf("insert conversation: %v", err)
	}
	return id
}

func insertMiniAppMigrationJob(t *testing.T, ctx context.Context, execer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, userID uuid.UUID, source, operation string, params map[string]string) uuid.UUID {
	t.Helper()
	rawParams, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	var id uuid.UUID
	if err := execer.QueryRow(ctx, `
INSERT INTO jobs (
    user_id, vk_peer_id, source, operation_type, modality, idempotency_key,
    correlation_id, params
) VALUES ($1, 7001, $2, $3, 'text', $4, $4, $5::jsonb)
RETURNING id`, userID, source, operation, "miniapp-single-default-chat:"+uuid.NewString(), rawParams).Scan(&id); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	return id
}

func insertMiniAppMigrationMessage(t *testing.T, ctx context.Context, execer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, conversationID, jobID uuid.UUID, role, text string) {
	t.Helper()
	if _, err := execer.Exec(ctx, `
INSERT INTO conversation_messages (conversation_id, job_id, role, text, token_count)
VALUES ($1, $2, $3, $4, 1)`, conversationID, jobID, role, text); err != nil {
		t.Fatalf("insert message: %v", err)
	}
}

func insertMiniAppMigrationSummary(t *testing.T, ctx context.Context, execer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, conversationID uuid.UUID, text string) {
	t.Helper()
	if _, err := execer.Exec(ctx, `
INSERT INTO conversation_summaries (conversation_id, text, token_count, summarized_until_seq)
VALUES ($1, $2, 1, 1)`, conversationID, text); err != nil {
		t.Fatalf("insert summary: %v", err)
	}
}

func assertMiniAppConversationCount(t *testing.T, ctx context.Context, execer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, userID uuid.UUID, source, externalThreadID string, want int) {
	t.Helper()
	var got int
	if err := execer.QueryRow(ctx, `
SELECT count(*) FROM conversations
WHERE user_id = $1 AND source = $2 AND external_thread_id = $3`, userID, source, externalThreadID).Scan(&got); err != nil {
		t.Fatalf("count conversations: %v", err)
	}
	if got != want {
		t.Fatalf("conversation count user=%s source=%s thread=%s = %d, want %d", userID, source, externalThreadID, got, want)
	}
}

func assertMiniAppConversationStatus(t *testing.T, ctx context.Context, execer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, userID uuid.UUID, source, externalThreadID, want string) {
	t.Helper()
	var got string
	if err := execer.QueryRow(ctx, `
SELECT status FROM conversations
WHERE user_id = $1 AND source = $2 AND external_thread_id = $3`, userID, source, externalThreadID).Scan(&got); err != nil {
		t.Fatalf("conversation status: %v", err)
	}
	if got != want {
		t.Fatalf("conversation status user=%s source=%s thread=%s = %q, want %q", userID, source, externalThreadID, got, want)
	}
}

func getMiniAppMigrationConversationID(t *testing.T, ctx context.Context, execer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, userID uuid.UUID, source, externalThreadID string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := execer.QueryRow(ctx, `
SELECT id FROM conversations
WHERE user_id = $1 AND source = $2 AND external_thread_id = $3`, userID, source, externalThreadID).Scan(&id); err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	return id
}

func assertMiniAppMessageTexts(t *testing.T, ctx context.Context, execer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, conversationID uuid.UUID, want []string) {
	t.Helper()
	rows, err := execer.Query(ctx, `
SELECT text FROM conversation_messages
WHERE conversation_id = $1
ORDER BY seq`, conversationID)
	if err != nil {
		t.Fatalf("query messages: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			t.Fatalf("scan message: %v", err)
		}
		got = append(got, text)
	}
	if rows.Err() != nil {
		t.Fatalf("message rows: %v", rows.Err())
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("messages = %#v, want %#v", got, want)
	}
}

func assertMiniAppSummaryText(t *testing.T, ctx context.Context, execer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, conversationID uuid.UUID, want string) {
	t.Helper()
	var got string
	if err := execer.QueryRow(ctx, `SELECT text FROM conversation_summaries WHERE conversation_id = $1`, conversationID).Scan(&got); err != nil {
		t.Fatalf("summary text: %v", err)
	}
	if got != want {
		t.Fatalf("summary text = %q, want %q", got, want)
	}
}

func assertMiniAppSummaryRedacted(t *testing.T, ctx context.Context, execer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, conversationID uuid.UUID) {
	t.Helper()
	var text string
	var tokenCount int
	var summarizedUntilSeq int64
	var redacted bool
	if err := execer.QueryRow(ctx, `
SELECT text, token_count, summarized_until_seq, redacted_at IS NOT NULL
FROM conversation_summaries
WHERE conversation_id = $1`, conversationID).Scan(&text, &tokenCount, &summarizedUntilSeq, &redacted); err != nil {
		t.Fatalf("summary redaction: %v", err)
	}
	if text != "" || tokenCount != 0 || summarizedUntilSeq != 0 || !redacted {
		t.Fatalf("summary not redacted: text=%q token_count=%d summarized_until_seq=%d redacted=%v", text, tokenCount, summarizedUntilSeq, redacted)
	}
}

func assertMiniAppSummaryCount(t *testing.T, ctx context.Context, execer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, conversationID uuid.UUID, want int) {
	t.Helper()
	var got int
	if err := execer.QueryRow(ctx, `SELECT count(*) FROM conversation_summaries WHERE conversation_id = $1`, conversationID).Scan(&got); err != nil {
		t.Fatalf("summary count: %v", err)
	}
	if got != want {
		t.Fatalf("summary count = %d, want %d", got, want)
	}
}

func assertMiniAppJobParams(t *testing.T, ctx context.Context, execer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, jobID uuid.UUID, wantThread string, wantMiniAppSource bool) {
	t.Helper()
	var params map[string]any
	if err := execer.QueryRow(ctx, `SELECT params FROM jobs WHERE id = $1`, jobID).Scan(&params); err != nil {
		t.Fatalf("job params: %v", err)
	}
	if got, _ := params["external_thread_id"].(string); got != wantThread {
		t.Fatalf("job %s external_thread_id = %q, want %q; params=%#v", jobID, got, wantThread, params)
	}
	_, hasConversationID := params["conversation_id"]
	if wantMiniAppSource {
		if hasConversationID {
			t.Fatalf("job %s kept stale conversation_id: %#v", jobID, params)
		}
		if got, _ := params["conversation_source"].(string); got != "miniapp" {
			t.Fatalf("job %s conversation_source = %q, want miniapp; params=%#v", jobID, got, params)
		}
	}
}
