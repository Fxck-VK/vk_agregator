package postgres_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"vk-ai-aggregator/internal/adapter/storage/postgres"
)

func TestReferralActivationMigrationMapsLegacyAppliedRewards(t *testing.T) {
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

	schema := "referral_migration_" + strings.ReplaceAll(uuid.NewString(), "-", "_")
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
		"000007_referrals.up.sql",
	} {
		runMigrationFile(t, ctx, conn, filepath.Join(root, "migrations", name))
	}

	var referrerID, appliedUserID, pendingUserID uuid.UUID
	if err := conn.QueryRow(ctx, `INSERT INTO users (vk_user_id) VALUES (710001) RETURNING id`).Scan(&referrerID); err != nil {
		t.Fatalf("insert referrer: %v", err)
	}
	if err := conn.QueryRow(ctx, `INSERT INTO users (vk_user_id) VALUES (710002) RETURNING id`).Scan(&appliedUserID); err != nil {
		t.Fatalf("insert applied user: %v", err)
	}
	if err := conn.QueryRow(ctx, `INSERT INTO users (vk_user_id) VALUES (710003) RETURNING id`).Scan(&pendingUserID); err != nil {
		t.Fatalf("insert pending user: %v", err)
	}
	if _, err := conn.Exec(ctx, `
		INSERT INTO referral_codes (user_id, code) VALUES ($1, 'MIGR2345')
	`, referrerID); err != nil {
		t.Fatalf("insert referral code: %v", err)
	}
	if _, err := conn.Exec(ctx, `
		INSERT INTO referrals (
			referrer_user_id, referred_user_id, referral_code, source, reward_status, rewarded_at
		) VALUES
			($1, $2, 'MIGR2345', 'vk_bot', 'applied', now()),
			($1, $3, 'MIGR2345', 'vk_bot', 'pending', NULL)
	`, referrerID, appliedUserID, pendingUserID); err != nil {
		t.Fatalf("insert legacy referrals: %v", err)
	}

	runMigrationFile(t, ctx, conn, filepath.Join(root, "migrations", "000013_referral_activation_status.up.sql"))

	var appliedStatus string
	var appliedActivatedNil bool
	if err := conn.QueryRow(ctx, `
		SELECT status, activated_at IS NULL
		FROM referrals
		WHERE referred_user_id = $1
	`, appliedUserID).Scan(&appliedStatus, &appliedActivatedNil); err != nil {
		t.Fatalf("read applied referral: %v", err)
	}
	if appliedStatus != "rewarded" || appliedActivatedNil {
		t.Fatalf("legacy applied referral migrated to status=%q activated_nil=%v, want rewarded/non-null", appliedStatus, appliedActivatedNil)
	}

	var pendingStatus string
	var pendingActivatedNil bool
	if err := conn.QueryRow(ctx, `
		SELECT status, activated_at IS NULL
		FROM referrals
		WHERE referred_user_id = $1
	`, pendingUserID).Scan(&pendingStatus, &pendingActivatedNil); err != nil {
		t.Fatalf("read pending referral: %v", err)
	}
	if pendingStatus != "registered" || !pendingActivatedNil {
		t.Fatalf("legacy pending referral migrated to status=%q activated_nil=%v, want registered/null", pendingStatus, pendingActivatedNil)
	}
}

func runMigrationFile(t *testing.T, ctx context.Context, execer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration %s: %v", path, err)
	}
	for _, stmt := range splitStatements(string(raw)) {
		if _, err := execer.Exec(ctx, stmt); err != nil {
			t.Fatalf("exec migration %s statement %q: %v", path, stmt, err)
		}
	}
}
