package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFinancialMigrationRollsBackAndRetriesWhenVersionRecordingFails(t *testing.T) {
	for _, tc := range []struct {
		version string
		tables  []string
	}{
		{"000066_web_referral_rewards", []string{"web_referral_payment_evidence", "web_referral_refund_evidence", "web_referral_rewards"}},
		{"000067_subscription_promotions", []string{"subscription_promotions", "subscription_promotion_uses", "subscription_promotion_mutations"}},
	} {
		t.Run(tc.version, func(t *testing.T) {
			ctx := context.Background()
			pool := nullableLegacyMigrationPool(t, ctx)
			if err := ensureTable(ctx, pool); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.up.sql"))
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range files {
				name := filepath.Base(path)
				if strings.TrimSuffix(name, ".up.sql") >= tc.version {
					continue
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := up(ctx, pool, dir); err != nil {
				t.Fatalf("apply prerequisites: %v", err)
			}
			if _, err := pool.Exec(ctx, `
				CREATE FUNCTION reject_financial_migration_version() RETURNS trigger AS $$
				BEGIN RAISE EXCEPTION 'forced schema_migrations insert failure'; END;
				$$ LANGUAGE plpgsql;
				CREATE TRIGGER reject_financial_migration_version BEFORE INSERT ON schema_migrations
				FOR EACH ROW EXECUTE FUNCTION reject_financial_migration_version();
			`); err != nil {
				t.Fatal(err)
			}
			name := tc.version + ".up.sql"
			raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
				t.Fatal(err)
			}
			if err := up(ctx, pool, dir); err == nil || !strings.Contains(err.Error(), "forced schema_migrations insert failure") {
				t.Fatalf("expected forced version-recording failure, got %v", err)
			}
			assertState := func(want bool) {
				t.Helper()
				for _, table := range tc.tables {
					var exists bool
					if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema=current_schema() AND table_name=$1)`, table).Scan(&exists); err != nil {
						t.Fatal(err)
					}
					if exists != want {
						t.Fatalf("table %s exists=%t, want %t", table, exists, want)
					}
				}
				var recorded bool
				if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, tc.version).Scan(&recorded); err != nil {
					t.Fatal(err)
				}
				if recorded != want {
					t.Fatalf("migration recorded=%t, want %t", recorded, want)
				}
			}
			assertState(false)
			if _, err := pool.Exec(ctx, `DROP TRIGGER reject_financial_migration_version ON schema_migrations; DROP FUNCTION reject_financial_migration_version()`); err != nil {
				t.Fatal(err)
			}
			if err := up(ctx, pool, dir); err != nil {
				t.Fatalf("retry after removing failure: %v", err)
			}
			assertState(true)
			if err := up(ctx, pool, dir); err != nil {
				t.Fatalf("resume applied migration: %v", err)
			}
		})
	}
}
