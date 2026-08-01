package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const nullableLegacyMigrationVersion = "000043_account_native_legacy_nullable"

func TestNullableLegacyMigrationRejectsDuplicateOwnerCurrencyAtomically(t *testing.T) {
	ctx := context.Background()
	pool := nullableLegacyMigrationPool(t, ctx)
	prepareNullableLegacyMigrationSchema(t, ctx, pool)

	ownerID := uuid.New()
	for _, userID := range []uuid.UUID{uuid.New(), uuid.New()} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO credit_accounts (user_id, owner_account_id, currency, balance)
			VALUES ($1, $2, 'credits', 100)
		`, userID, ownerID); err != nil {
			t.Fatalf("seed duplicate credit account: %v", err)
		}
	}

	err := up(ctx, pool, nullableLegacyMigrationDir(t))
	if err == nil {
		t.Fatal("up succeeded with duplicate owner_account_id/currency rows")
	}
	if !strings.Contains(err.Error(), "duplicate canonical owner/currency") {
		t.Fatalf("up error = %v, want duplicate owner/currency failure", err)
	}
	assertNullableLegacyColumns(t, ctx, pool, false)
	assertNullableLegacyOwnerCurrencyIndex(t, ctx, pool, false)
	assertNullableLegacyMigrationRecorded(t, ctx, pool, false)
}

func TestNullableLegacyMigrationAppliesWithRunnerTransaction(t *testing.T) {
	ctx := context.Background()
	pool := nullableLegacyMigrationPool(t, ctx)
	prepareNullableLegacyMigrationSchema(t, ctx, pool)

	jobUserID := uuid.New()
	artifactUserID := uuid.New()
	creditUserID := uuid.New()
	paymentUserID := uuid.New()
	ownerID := uuid.New()
	for _, seed := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO jobs (user_id, vk_peer_id, payload) VALUES ($1, 101, 'job payload')`, []any{jobUserID}},
		{`INSERT INTO artifacts (owner_user_id, payload) VALUES ($1, 'artifact payload')`, []any{artifactUserID}},
		{`INSERT INTO credit_accounts (user_id, owner_account_id, currency, balance) VALUES ($1, $2, 'credits', 117)`, []any{creditUserID, ownerID}},
		{`INSERT INTO payment_intents (user_id, payload) VALUES ($1, 'payment payload')`, []any{paymentUserID}},
	} {
		if _, err := pool.Exec(ctx, seed.query, seed.args...); err != nil {
			t.Fatalf("seed legacy row: %v", err)
		}
	}

	if err := up(ctx, pool, nullableLegacyMigrationDir(t)); err != nil {
		t.Fatalf("up via runner transaction: %v", err)
	}
	assertNullableLegacyColumns(t, ctx, pool, true)
	assertNullableLegacyColumnsRemainRequired(t, ctx, pool)
	assertNullableLegacyOwnerCurrencyIndex(t, ctx, pool, true)
	assertNullableLegacyMigrationRecorded(t, ctx, pool, true)

	var jobPayload, artifactPayload, currency, paymentPayload string
	var vkPeerID, balance int64
	var storedJobUserID, storedArtifactUserID, storedCreditUserID, storedPaymentUserID, storedOwnerID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT user_id, vk_peer_id, payload FROM jobs`).Scan(&storedJobUserID, &vkPeerID, &jobPayload); err != nil {
		t.Fatalf("read job: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT owner_user_id, payload FROM artifacts`).Scan(&storedArtifactUserID, &artifactPayload); err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT user_id, owner_account_id, currency, balance FROM credit_accounts`).Scan(&storedCreditUserID, &storedOwnerID, &currency, &balance); err != nil {
		t.Fatalf("read credit account: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT user_id, payload FROM payment_intents`).Scan(&storedPaymentUserID, &paymentPayload); err != nil {
		t.Fatalf("read payment intent: %v", err)
	}
	if storedJobUserID != jobUserID || vkPeerID != 101 || jobPayload != "job payload" ||
		storedArtifactUserID != artifactUserID || artifactPayload != "artifact payload" ||
		storedCreditUserID != creditUserID || storedOwnerID != ownerID || currency != "credits" || balance != 117 ||
		storedPaymentUserID != paymentUserID || paymentPayload != "payment payload" {
		t.Fatalf("migration changed legacy data: job=%s/%d/%q artifact=%s/%q credit=%s/%s/%q/%d payment=%s/%q",
			storedJobUserID, vkPeerID, jobPayload, storedArtifactUserID, artifactPayload,
			storedCreditUserID, storedOwnerID, currency, balance, storedPaymentUserID, paymentPayload)
	}
}

func nullableLegacyMigrationPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL migration test")
	}
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect admin pool: %v", err)
	}
	schema := "nullable_legacy_" + strings.ReplaceAll(uuid.NewString(), "-", "_")
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatalf("create schema: %v", err)
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
		t.Fatalf("parse pool config: %v", err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
		t.Fatalf("connect schema pool: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		admin.Close()
	})
	return pool
}

func prepareNullableLegacyMigrationSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for _, stmt := range []string{
		`CREATE TABLE jobs (user_id UUID NOT NULL, vk_peer_id BIGINT NOT NULL, payload TEXT NOT NULL)`,
		`CREATE TABLE artifacts (owner_user_id UUID NOT NULL, payload TEXT NOT NULL)`,
		`CREATE TABLE credit_accounts (user_id UUID NOT NULL, owner_account_id UUID, currency TEXT NOT NULL, balance BIGINT NOT NULL)`,
		`CREATE TABLE payment_intents (user_id UUID NOT NULL, payload TEXT NOT NULL)`,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("create legacy schema: %v", err)
		}
	}
	if err := ensureTable(ctx, pool); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
}

func nullableLegacyMigrationDir(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "migrations", nullableLegacyMigrationVersion+".up.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, nullableLegacyMigrationVersion+".up.sql"), raw, 0o600); err != nil {
		t.Fatalf("write migration: %v", err)
	}
	return dir
}

func assertNullableLegacyColumns(t *testing.T, ctx context.Context, pool *pgxpool.Pool, nullable bool) {
	t.Helper()
	want := "NO"
	if nullable {
		want = "YES"
	}
	for _, column := range []struct{ table, name string }{
		{"jobs", "user_id"},
		{"jobs", "vk_peer_id"},
		{"artifacts", "owner_user_id"},
		{"credit_accounts", "user_id"},
		{"payment_intents", "user_id"},
	} {
		var got string
		if err := pool.QueryRow(ctx, `
			SELECT is_nullable
			FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = $1 AND column_name = $2
		`, column.table, column.name).Scan(&got); err != nil {
			t.Fatalf("read %s.%s nullability: %v", column.table, column.name, err)
		}
		if got != want {
			t.Fatalf("%s.%s is_nullable = %s, want %s", column.table, column.name, got, want)
		}
	}
}

func assertNullableLegacyColumnsRemainRequired(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for _, column := range []struct{ table, name string }{
		{"jobs", "payload"},
		{"artifacts", "payload"},
		{"credit_accounts", "currency"},
		{"credit_accounts", "balance"},
		{"payment_intents", "payload"},
	} {
		var got string
		if err := pool.QueryRow(ctx, `
			SELECT is_nullable
			FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = $1 AND column_name = $2
		`, column.table, column.name).Scan(&got); err != nil {
			t.Fatalf("read %s.%s nullability: %v", column.table, column.name, err)
		}
		if got != "NO" {
			t.Fatalf("unrelated column %s.%s is_nullable = %s, want NO", column.table, column.name, got)
		}
	}
}

func assertNullableLegacyOwnerCurrencyIndex(t *testing.T, ctx context.Context, pool *pgxpool.Pool, exists bool) {
	t.Helper()
	var indexDef string
	err := pool.QueryRow(ctx, `
		SELECT pg_get_indexdef(indexrelid)
		FROM pg_index
		JOIN pg_class ON pg_class.oid = indexrelid
		JOIN pg_namespace ON pg_namespace.oid = pg_class.relnamespace
		WHERE pg_class.relname = $1 AND pg_namespace.nspname = current_schema()
	`, "credit_accounts_owner_account_currency_unique").Scan(&indexDef)
	if !exists {
		if err == nil {
			t.Fatalf("owner/currency index exists after failed migration: %s", indexDef)
		}
		return
	}
	if err != nil {
		t.Fatalf("read owner/currency index: %v", err)
	}
	indexDef = strings.ToLower(indexDef)
	if !strings.Contains(indexDef, "create unique index") || !strings.Contains(indexDef, "(owner_account_id, currency)") || !strings.Contains(indexDef, "where (owner_account_id is not null)") {
		t.Fatalf("owner/currency index definition = %q, want partial unique owner_account_id/currency index", indexDef)
	}
}

func assertNullableLegacyMigrationRecorded(t *testing.T, ctx context.Context, pool *pgxpool.Pool, recorded bool) {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations WHERE version = $1`, nullableLegacyMigrationVersion).Scan(&count); err != nil {
		t.Fatalf("read migration record: %v", err)
	}
	want := 0
	if recorded {
		want = 1
	}
	if count != want {
		t.Fatalf("migration record count = %d, want %d", count, want)
	}
}
