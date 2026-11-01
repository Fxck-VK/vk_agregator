package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestAccountPublicIDMigrationRejectsDownWithoutLosingVersionOrMapping(t *testing.T) {
	ctx := context.Background()
	pool := nullableLegacyMigrationPool(t, ctx)
	if _, err := pool.Exec(ctx, "CREATE TABLE accounts(id uuid PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if err := ensureTable(ctx, pool); err != nil {
		t.Fatal(err)
	}
	owner := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO accounts(id) VALUES($1)", owner); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	const version = "000065_account_public_id"
	for _, suffix := range []string{"up", "down"} {
		name := version + "." + suffix + ".sql"
		data, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := up(ctx, pool, dir); err != nil {
		t.Fatal(err)
	}
	var before int64
	if err := pool.QueryRow(ctx, "SELECT public_number FROM accounts WHERE id=$1", owner).Scan(&before); err != nil || before <= 0 {
		t.Fatal("missing assigned number", err)
	}
	if err := down(ctx, pool, dir); err == nil || !strings.Contains(err.Error(), "public IDs are permanent") {
		t.Fatal("destructive rollback was not blocked", err)
	}
	var after int64
	if err := pool.QueryRow(ctx, "SELECT public_number FROM accounts WHERE id=$1", owner).Scan(&after); err != nil || after != before {
		t.Fatal("rollback changed public ID", err)
	}
	var recorded bool
	if err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", version).Scan(&recorded); err != nil || !recorded {
		t.Fatal("rollback lost migration version", err)
	}
	if err := up(ctx, pool, dir); err != nil {
		t.Fatal("unchanged migration was not resumable", err)
	}
}
