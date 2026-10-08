package postgres_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/referralservice"
)

func TestPostgresLegacyReferralActivationIgnoresCanonicalOnlyReferral(t *testing.T) {
	ctx := context.Background()
	pool := postgresLegacyReferralPool(t, ctx)
	referrals := postgres.NewReferralRepository(pool)
	billingRepo := postgres.NewBillingRepository(pool)
	billing := billingservice.New(billingRepo, billingservice.WithStartingBalance(0))
	now := time.Date(2026, 10, 8, 13, 0, 0, 0, time.UTC)
	referrerAccountID := insertPostgresLegacyReferralAccount(t, ctx, pool, now)
	referredAccountID := insertPostgresLegacyReferralAccount(t, ctx, pool, now.Add(time.Minute))
	referredUserID := insertPostgresLegacyReferralUser(t, ctx, pool, referredAccountID, 970001)
	if _, err := pool.Exec(ctx, `
		INSERT INTO referral_codes (account_id, code)
		VALUES ($1, 'CANSQL234')`, referrerAccountID); err != nil {
		t.Fatalf("insert account-owned code: %v", err)
	}
	svc := referralservice.New(referrals, billing, referralservice.Config{
		ReferrerSignupRewardCredits: 10,
		ReferredSignupRewardCredits: 3,
		RewardOnActivation:          true,
	}, referralservice.WithClock(func() time.Time { return now }))

	applied, err := svc.Apply(ctx, referralservice.ApplyInput{
		Code:              "CANSQL234",
		ReferredUserID:    referredUserID,
		ReferredAccountID: referredAccountID,
		Source:            domain.ReferralSourceVKBot,
	})
	if err != nil {
		t.Fatalf("apply canonical-only code: %v", err)
	}
	if !applied.Applied || applied.Referral == nil || applied.Referral.ReferrerUserID != uuid.Nil {
		t.Fatalf("unexpected apply result: %+v", applied)
	}

	activated, err := svc.Activate(ctx, referralservice.ActivateInput{
		ReferredUserID:    referredUserID,
		ReferredAccountID: referredAccountID,
		Source:            domain.ReferralSourceVKBot,
	})
	if err != nil {
		t.Fatalf("activate canonical-only referral: %v", err)
	}
	if !activated.NotFound || activated.Activated || activated.Rewarded || activated.AlreadyRewarded {
		t.Fatalf("canonical-only referral should be ignored by legacy activation, got %+v", activated)
	}

	var status, rewardStatus string
	var activatedNil, rewardedNil bool
	if err := pool.QueryRow(ctx, `
		SELECT status, reward_status, activated_at IS NULL, rewarded_at IS NULL
		FROM referrals
		WHERE referred_account_id = $1`, referredAccountID).Scan(&status, &rewardStatus, &activatedNil, &rewardedNil); err != nil {
		t.Fatalf("load referral: %v", err)
	}
	if status != string(domain.ReferralStatusRegistered) || rewardStatus != string(domain.ReferralRewardPending) || !activatedNil || !rewardedNil {
		t.Fatalf("referral status=%q reward=%q activatedNil=%v rewardedNil=%v", status, rewardStatus, activatedNil, rewardedNil)
	}
	if _, err := billingRepo.GetAccountByUser(ctx, referrerAccountID, domain.CurrencyCredits); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("legacy activation created referrer billing account, err=%v", err)
	}
	if _, err := billingRepo.GetAccountByUser(ctx, referredUserID, domain.CurrencyCredits); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("legacy activation created referred billing account, err=%v", err)
	}
}

func postgresLegacyReferralPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL legacy referral integration test")
	}
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	schema := "legacy_referral_" + strings.ReplaceAll(uuid.NewString(), "-", "_")
	if _, err := admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		admin.Close()
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
		admin.Close()
	})

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect schema pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping schema pool: %v", err)
	}
	t.Cleanup(pool.Close)

	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	for _, path := range postgresLegacyReferralMigrationUpFiles(t, root, "000056") {
		if err := runPostgresLegacyReferralMigrationPath(t, ctx, pool, path); err != nil {
			t.Fatalf("run migration %s: %v", filepath.Base(path), err)
		}
	}
	return pool
}

func postgresLegacyReferralMigrationUpFiles(t *testing.T, root, maxVersion string) []string {
	t.Helper()
	dir := filepath.Join(root, "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	var out []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".up.sql") || len(name) < 6 || name[:6] > maxVersion {
			continue
		}
		out = append(out, filepath.Join(dir, name))
	}
	sort.Strings(out)
	return out
}

func runPostgresLegacyReferralMigrationPath(t *testing.T, ctx context.Context, pool *pgxpool.Pool, path string) error {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, stmt := range splitPostgresLegacyReferralMigrationStatements(string(raw)) {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func splitPostgresLegacyReferralMigrationStatements(script string) []string {
	var out []string
	var current strings.Builder
	inString := false
	inLineComment := false
	dollarQuote := ""
	for i := 0; i < len(script); i++ {
		if inLineComment {
			if script[i] == '\n' {
				inLineComment = false
				current.WriteByte(script[i])
			}
			continue
		}
		if dollarQuote != "" {
			if strings.HasPrefix(script[i:], dollarQuote) {
				current.WriteString(dollarQuote)
				i += len(dollarQuote) - 1
				dollarQuote = ""
				continue
			}
			current.WriteByte(script[i])
			continue
		}
		if !inString && script[i] == '-' && i+1 < len(script) && script[i+1] == '-' {
			inLineComment = true
			i++
			continue
		}
		if !inString && script[i] == '$' {
			if tagEnd := strings.IndexByte(script[i+1:], '$'); tagEnd >= 0 {
				tagBody := script[i+1 : i+1+tagEnd]
				if validPostgresLegacyReferralDollarQuoteTag(tagBody) {
					tag := script[i : i+tagEnd+2]
					current.WriteString(tag)
					i += len(tag) - 1
					dollarQuote = tag
					continue
				}
			}
		}
		if script[i] == '\'' {
			current.WriteByte(script[i])
			if inString && i+1 < len(script) && script[i+1] == '\'' {
				i++
				current.WriteByte(script[i])
				continue
			}
			inString = !inString
			continue
		}
		if script[i] == ';' && !inString {
			appendStatement(&out, current.String())
			current.Reset()
			continue
		}
		current.WriteByte(script[i])
	}
	appendStatement(&out, current.String())
	return out
}

func validPostgresLegacyReferralDollarQuoteTag(tag string) bool {
	if tag == "" {
		return true
	}
	for i, r := range tag {
		if r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || i > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

func insertPostgresLegacyReferralAccount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, createdAt time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO accounts (id, status, role, account_type, locale, timezone, risk_level, created_at, updated_at)
		VALUES ($1, 'active', 'user', 'personal', 'ru', 'Europe/Moscow', 0, $2, $2)`, id, createdAt); err != nil {
		t.Fatalf("insert account: %v", err)
	}
	return id
}

func insertPostgresLegacyReferralUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, accountID uuid.UUID, vkUserID int64) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (account_id, vk_user_id)
		VALUES ($1, $2)
		RETURNING id`, accountID, vkUserID).Scan(&id); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}
