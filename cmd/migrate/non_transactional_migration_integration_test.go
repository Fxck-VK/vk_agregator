package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const nonTransactionalRetryMigrationVersion = "000999_non_transactional_index_retry"

func TestNonTransactionalMigrationRecordsOnlyAfterSQLSucceedsAndCanRetry(t *testing.T) {
	ctx := context.Background()
	pool := nullableLegacyMigrationPool(t, ctx)
	if err := ensureTable(ctx, pool); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	if _, err := pool.Exec(ctx, `CREATE TABLE concurrent_migration_subject (id BIGINT NOT NULL)`); err != nil {
		t.Fatalf("create migration subject: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO concurrent_migration_subject (id) VALUES (1), (1)`); err != nil {
		t.Fatalf("seed duplicate rows: %v", err)
	}

	dir := t.TempDir()
	writeNonTransactionalMigration(t, dir, "up", `-- migrate: no-transaction
DROP INDEX CONCURRENTLY IF EXISTS concurrent_migration_subject_id_idx;
CREATE UNIQUE INDEX CONCURRENTLY concurrent_migration_subject_id_idx
    ON concurrent_migration_subject (id);
`)
	writeNonTransactionalMigration(t, dir, "down", `-- migrate: no-transaction
DROP INDEX CONCURRENTLY IF EXISTS concurrent_migration_subject_id_idx;
`)

	if err := up(ctx, pool, dir); err == nil {
		t.Fatal("up succeeded despite duplicate values for a unique concurrent index")
	}
	assertMigrationRecord(t, ctx, pool, nonTransactionalRetryMigrationVersion, false)
	assertConcurrentIndexValidity(t, ctx, pool, "concurrent_migration_subject_id_idx", false)

	if _, err := pool.Exec(ctx, `DELETE FROM concurrent_migration_subject WHERE ctid IN (
		SELECT ctid FROM concurrent_migration_subject WHERE id = 1 LIMIT 1
	)`); err != nil {
		t.Fatalf("remove duplicate row: %v", err)
	}
	if err := up(ctx, pool, dir); err != nil {
		t.Fatalf("retry non-transactional migration: %v", err)
	}
	assertMigrationRecord(t, ctx, pool, nonTransactionalRetryMigrationVersion, true)
	assertConcurrentIndexValidity(t, ctx, pool, "concurrent_migration_subject_id_idx", true)

	if err := down(ctx, pool, dir); err != nil {
		t.Fatalf("rollback non-transactional migration: %v", err)
	}
	assertMigrationRecord(t, ctx, pool, nonTransactionalRetryMigrationVersion, false)
	assertConcurrentIndexMissing(t, ctx, pool, "concurrent_migration_subject_id_idx")
}

func TestNonTransactionalMigrationAdvisoryLockExcludesAnotherSession(t *testing.T) {
	ctx := context.Background()
	pool := nullableLegacyMigrationPool(t, ctx)

	if err := withMigrationAdvisoryLock(ctx, pool, func(_ *pgxpool.Conn) error {
		contender, err := pool.Acquire(ctx)
		if err != nil {
			return fmt.Errorf("acquire contender connection: %w", err)
		}
		defer contender.Release()
		var acquired bool
		if err := contender.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, migrationRunnerAdvisoryLockKey).Scan(&acquired); err != nil {
			return fmt.Errorf("try migration advisory lock: %w", err)
		}
		if acquired {
			_, _ = contender.Exec(ctx, `SELECT pg_advisory_unlock($1)`, migrationRunnerAdvisoryLockKey)
			return fmt.Errorf("another session acquired the migration advisory lock")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	verifier, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire verification connection: %v", err)
	}
	defer verifier.Release()
	var acquired bool
	if err := verifier.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, migrationRunnerAdvisoryLockKey).Scan(&acquired); err != nil {
		t.Fatalf("try released migration advisory lock: %v", err)
	}
	if !acquired {
		t.Fatal("migration advisory lock remained held after callback")
	}
	if _, err := verifier.Exec(ctx, `SELECT pg_advisory_unlock($1)`, migrationRunnerAdvisoryLockKey); err != nil {
		t.Fatalf("release test migration advisory lock: %v", err)
	}
}

func writeNonTransactionalMigration(t *testing.T, dir, direction, sqlText string) {
	t.Helper()
	path := filepath.Join(dir, nonTransactionalRetryMigrationVersion+"."+direction+".sql")
	if err := os.WriteFile(path, []byte(sqlText), 0o600); err != nil {
		t.Fatalf("write %s migration: %v", direction, err)
	}
}

func assertMigrationRecord(t *testing.T, ctx context.Context, pool *pgxpool.Pool, version string, recorded bool) {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations WHERE version = $1`, version).Scan(&count); err != nil {
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

func assertConcurrentIndexValidity(t *testing.T, ctx context.Context, pool *pgxpool.Pool, indexName string, valid bool) {
	t.Helper()
	var got bool
	if err := pool.QueryRow(ctx, `
		SELECT indisvalid
		FROM pg_index
		JOIN pg_class ON pg_class.oid = indexrelid
		JOIN pg_namespace ON pg_namespace.oid = pg_class.relnamespace
		WHERE pg_class.relname = $1 AND pg_namespace.nspname = current_schema()
	`, indexName).Scan(&got); err != nil {
		t.Fatalf("read index %s: %v", indexName, err)
	}
	if got != valid {
		t.Fatalf("index %s validity = %t, want %t", indexName, got, valid)
	}
}

func assertConcurrentIndexMissing(t *testing.T, ctx context.Context, pool *pgxpool.Pool, indexName string) {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_class
		JOIN pg_namespace ON pg_namespace.oid = pg_class.relnamespace
		WHERE pg_class.relname = $1 AND pg_namespace.nspname = current_schema()
	`, indexName).Scan(&count); err != nil {
		t.Fatalf("read index count: %v", err)
	}
	if count != 0 {
		t.Fatalf("index %s still exists", indexName)
	}
}
