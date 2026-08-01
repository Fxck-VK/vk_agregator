package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const accountSessionAccessTokensMigrationVersion = "000042_account_session_access_tokens"

func TestAccountSessionAccessTokensMigrationDoesNotControlItsOwnTransaction(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", accountSessionAccessTokensMigrationVersion+".up.sql")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	upper := strings.ToUpper(string(raw))
	if strings.Contains(upper, "BEGIN;") || strings.Contains(upper, "COMMIT;") {
		t.Fatalf("migration controls its own transaction; cmd/migrate owns transaction boundaries")
	}
}

func TestAccountSessionAccessTokensMigrationRollsBackSchemaWhenVersionRecordingFails(t *testing.T) {
	ctx := context.Background()
	pool := nullableLegacyMigrationPool(t, ctx)
	prepareAccountSessionAccessTokensMigrationSchema(t, ctx, pool)

	if _, err := pool.Exec(ctx, `
		CREATE FUNCTION reject_account_session_migration_version() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'forced schema_migrations insert failure';
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER reject_account_session_migration_version
			BEFORE INSERT ON schema_migrations
			FOR EACH ROW EXECUTE FUNCTION reject_account_session_migration_version();
	`); err != nil {
		t.Fatalf("install failing schema_migrations trigger: %v", err)
	}

	err := up(ctx, pool, accountSessionAccessTokensMigrationDir(t))
	if err == nil {
		t.Fatal("up succeeded despite forced schema_migrations insert failure")
	}
	assertAccountSessionAccessTokensSchema(t, ctx, pool, false)
	assertAccountSessionAccessTokensMigrationRecorded(t, ctx, pool, false)
}

func TestAccountSessionAccessTokensMigrationAppliesAndRecordsVersionTogether(t *testing.T) {
	ctx := context.Background()
	pool := nullableLegacyMigrationPool(t, ctx)
	prepareAccountSessionAccessTokensMigrationSchema(t, ctx, pool)

	if err := up(ctx, pool, accountSessionAccessTokensMigrationDir(t)); err != nil {
		t.Fatalf("apply account-session migration: %v", err)
	}
	assertAccountSessionAccessTokensSchema(t, ctx, pool, true)
	assertAccountSessionAccessTokensMigrationRecorded(t, ctx, pool, true)
}

func prepareAccountSessionAccessTokensMigrationSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		CREATE TABLE account_sessions (
			id UUID PRIMARY KEY,
			refresh_token_hash TEXT NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL
		)
	`); err != nil {
		t.Fatalf("create account_sessions: %v", err)
	}
	if err := ensureTable(ctx, pool); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
}

func accountSessionAccessTokensMigrationDir(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "migrations", accountSessionAccessTokensMigrationVersion+".up.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, accountSessionAccessTokensMigrationVersion+".up.sql"), raw, 0o600); err != nil {
		t.Fatalf("write migration: %v", err)
	}
	return dir
}

func assertAccountSessionAccessTokensSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool, exists bool) {
	t.Helper()
	for _, column := range []string{"access_token_hash", "access_expires_at"} {
		var columnExists bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = current_schema()
					AND table_name = 'account_sessions'
					AND column_name = $1
			)
		`, column).Scan(&columnExists); err != nil {
			t.Fatalf("check account_sessions.%s: %v", column, err)
		}
		if columnExists != exists {
			t.Fatalf("account_sessions.%s exists = %t, want %t", column, columnExists, exists)
		}
	}

	var constraintExists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM pg_constraint c
			JOIN pg_class rel ON rel.oid = c.conrelid
			JOIN pg_namespace ns ON ns.oid = rel.relnamespace
			WHERE ns.nspname = current_schema()
				AND rel.relname = 'account_sessions'
				AND c.conname = 'account_sessions_access_token_pair_check'
		)
	`).Scan(&constraintExists); err != nil {
		t.Fatalf("check access-token constraint: %v", err)
	}
	if constraintExists != exists {
		t.Fatalf("access-token constraint exists = %t, want %t", constraintExists, exists)
	}
}

func assertAccountSessionAccessTokensMigrationRecorded(t *testing.T, ctx context.Context, pool *pgxpool.Pool, recorded bool) {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations WHERE version = $1`, accountSessionAccessTokensMigrationVersion).Scan(&count); err != nil {
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
