package postgres_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/webreferralservice"
)

func TestWebReferralRepositoryAcceptsNewAccountOnce(t *testing.T) {
	ctx := context.Background()
	pool := webReferralPool(t, ctx)
	repo := postgres.NewWebReferralRepository(pool)
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	referrerID := insertAccount(t, ctx, pool, now.Add(-24*time.Hour), "active")
	svc := webreferralservice.New(repo, webreferralservice.WithCodeGenerator(func(int) (string, error) {
		return "WEB2345678", nil
	}))

	initial, err := svc.Summary(ctx, referrerID)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if initial.Code != "WEB2345678" || initial.RewardsEnabled {
		t.Fatalf("initial summary = %+v", initial)
	}

	tokenHash := strings.Repeat("a", 64)
	visitAt := now
	if err := svc.Capture(ctx, initial.Code, tokenHash, visitAt); err != nil {
		t.Fatalf("capture: %v", err)
	}
	referredID := insertAccount(t, ctx, pool, visitAt.Add(time.Second), "active")
	if err := svc.Accept(ctx, tokenHash, referredID, visitAt.Add(2*time.Second)); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if err := svc.Accept(ctx, tokenHash, referredID, visitAt.Add(3*time.Second)); err != nil {
		t.Fatalf("repeat accept same account: %v", err)
	}
	otherID := insertAccount(t, ctx, pool, visitAt.Add(time.Minute), "active")
	if err := svc.Accept(ctx, tokenHash, otherID, visitAt.Add(4*time.Second)); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("accept different account error = %v, want ErrConflict", err)
	}

	assertWebReferralRow(t, ctx, pool, referrerID, referredID, initial.Code)

	selfTokenHash := strings.Repeat("b", 64)
	if err := svc.Capture(ctx, initial.Code, selfTokenHash, visitAt); err != nil {
		t.Fatalf("capture self token: %v", err)
	}
	if err := svc.Accept(ctx, selfTokenHash, referrerID, visitAt.Add(time.Second)); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("self accept error = %v, want ErrForbidden", err)
	}

	oldTokenHash := strings.Repeat("c", 64)
	oldAccountID := insertAccount(t, ctx, pool, visitAt.Add(-time.Minute), "active")
	if err := svc.Capture(ctx, initial.Code, oldTokenHash, visitAt); err != nil {
		t.Fatalf("capture old-account token: %v", err)
	}
	if err := svc.Accept(ctx, oldTokenHash, oldAccountID, visitAt.Add(time.Second)); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("old-account accept error = %v, want ErrForbidden", err)
	}

	concurrentTokenHash := strings.Repeat("d", 64)
	concurrentAccountID := insertAccount(t, ctx, pool, visitAt.Add(2*time.Minute), "active")
	if err := svc.Capture(ctx, initial.Code, concurrentTokenHash, visitAt); err != nil {
		t.Fatalf("capture concurrent token: %v", err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- svc.Accept(ctx, concurrentTokenHash, concurrentAccountID, visitAt.Add(3*time.Minute))
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent accept: %v", err)
		}
	}

	got, err := svc.Summary(ctx, referrerID)
	if err != nil {
		t.Fatalf("summary after accepts: %v", err)
	}
	if got.Visits != 4 || got.Registered != 2 || got.Activated != 0 || got.Rewarded != 0 || got.RewardsEnabled {
		t.Fatalf("summary after accepts = %+v", got)
	}
	var ledgerEntries int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM ledger_entries`).Scan(&ledgerEntries); err != nil {
		t.Fatalf("count ledger entries: %v", err)
	}
	if ledgerEntries != 0 {
		t.Fatalf("web referral accept posted ledger entries: %d", ledgerEntries)
	}
}

func TestWebReferralMigrationAllowsAccountNativeRowsAndGuardsOwners(t *testing.T) {
	ctx := context.Background()
	pool := webReferralPool(t, ctx)
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	referrerID := insertAccount(t, ctx, pool, now, "active")
	referredID := insertAccount(t, ctx, pool, now.Add(time.Second), "active")

	if _, err := pool.Exec(ctx, `
		INSERT INTO referral_codes (account_id, code)
		VALUES ($1, 'WEBMIG2345')`, referrerID); err != nil {
		t.Fatalf("insert account-native code: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO referrals (
			referrer_account_id, referred_account_id, referral_code, source, status, reward_status
		) VALUES ($1, $2, 'WEBMIG2345', 'web', 'registered', 'pending')`, referrerID, referredID); err != nil {
		t.Fatalf("insert account-native referral: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO referral_codes (code)
		VALUES ('NOOWNER234')`); err == nil {
		t.Fatal("ownerless referral code insert succeeded")
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO referrals (
			referrer_account_id, referred_account_id, referral_code, source, status, reward_status
		) VALUES ($1, $1, 'WEBMIG2345', 'web', 'registered', 'pending')`, referrerID); err == nil {
		t.Fatal("account self-referral insert succeeded")
	}
}

func TestWebReferralCleanupPreservesVisitTotalsAndReferralRelations(t *testing.T) {
	ctx := context.Background()
	pool := webReferralPool(t, ctx)
	repo := postgres.NewWebReferralRepository(pool)
	maintenance := postgres.NewMaintenanceRepository(pool)
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	referrerA := insertAccount(t, ctx, pool, now.Add(-48*time.Hour), "active")
	referrerB := insertAccount(t, ctx, pool, now.Add(-48*time.Hour), "active")
	referredID := insertAccount(t, ctx, pool, now.Add(-30*time.Minute), "active")
	if _, err := pool.Exec(ctx, `
		INSERT INTO referral_codes (account_id, code)
		VALUES ($1, 'WEBCLEANA'), ($2, 'WEBCLEANB')`, referrerA, referrerB); err != nil {
		t.Fatalf("insert codes: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO referrals (
			referrer_account_id, referred_account_id, referral_code, source, status, reward_status, first_seen_at
		) VALUES ($1, $2, 'WEBCLEANA', 'web', 'registered', 'pending', $3)`, referrerA, referredID, now.Add(-2*time.Hour)); err != nil {
		t.Fatalf("insert accepted relation: %v", err)
	}
	insertWebReferralVisit(t, ctx, pool, strings.Repeat("1", 64), "WEBCLEANA", referrerA, now.Add(-3*time.Hour), now.Add(-2*time.Hour), uuid.Nil)
	insertWebReferralVisit(t, ctx, pool, strings.Repeat("2", 64), "WEBCLEANA", referrerA, now.Add(-2*time.Hour), now.Add(-time.Hour), referredID)
	insertWebReferralVisit(t, ctx, pool, strings.Repeat("3", 64), "WEBCLEANA", referrerA, now.Add(-time.Hour), now.Add(time.Hour), uuid.Nil)
	insertWebReferralVisit(t, ctx, pool, strings.Repeat("4", 64), "WEBCLEANB", referrerB, now.Add(-90*time.Minute), now.Add(-30*time.Minute), uuid.Nil)

	deleted, err := maintenance.CleanupExpiredWebReferralVisits(ctx, now, 2)
	if err != nil {
		t.Fatalf("first cleanup: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("first cleanup deleted %d, want bounded batch of 2", deleted)
	}
	assertWebReferralSummaryCounts(t, ctx, repo, referrerA, 3)
	assertWebReferralSummaryCounts(t, ctx, repo, referrerB, 1)
	assertWebReferralRelationCount(t, ctx, pool, referredID, 1)

	deleted, err = maintenance.CleanupExpiredWebReferralVisits(ctx, now, 10)
	if err != nil {
		t.Fatalf("second cleanup: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("second cleanup deleted %d, want remaining expired visit", deleted)
	}
	deleted, err = maintenance.CleanupExpiredWebReferralVisits(ctx, now, 10)
	if err != nil {
		t.Fatalf("third cleanup: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("third cleanup deleted %d, want idempotent no-op", deleted)
	}
	assertWebReferralSummaryCounts(t, ctx, repo, referrerA, 3)
	assertWebReferralSummaryCounts(t, ctx, repo, referrerB, 1)
	assertWebReferralRelationCount(t, ctx, pool, referredID, 1)
}

func TestWebReferralMaintenanceCleanupRequiresTransactionalQuerier(t *testing.T) {
	maintenance := postgres.NewMaintenanceRepository(nonTransactionalWebReferralQuerier{})
	_, err := maintenance.CleanupExpiredWebReferralVisits(context.Background(), time.Now(), 10)
	if !errors.Is(err, postgres.ErrWebReferralCleanupRequiresTransactions) {
		t.Fatalf("cleanup error = %v, want ErrWebReferralCleanupRequiresTransactions", err)
	}
}

func TestWebReferralDownMigrationRefusesCollectedAnalytics(t *testing.T) {
	ctx := context.Background()
	pool := webReferralPool(t, ctx)
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	referrerID := insertAccount(t, ctx, pool, now, "active")
	if _, err := pool.Exec(ctx, `
		INSERT INTO referral_codes (account_id, code)
		VALUES ($1, 'WEBDOWN234')`, referrerID); err != nil {
		t.Fatalf("insert code: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO web_referral_visits (token_hash, code, referrer_account_id, created_at, expires_at)
		VALUES ($1, 'WEBDOWN234', $2, $3, $4)`,
		strings.Repeat("e", 64), referrerID, now, now.Add(24*time.Hour)); err != nil {
		t.Fatalf("insert visit: %v", err)
	}

	err := runMigrationFileResult(t, ctx, pool, "000056_web_referrals.down.sql")
	if err == nil {
		t.Fatal("down migration succeeded despite collected visit analytics")
	}
	if !strings.Contains(err.Error(), "web_referral_visits contains collected analytics") {
		t.Fatalf("down migration error = %v", err)
	}
}

func TestWebReferralDownMigrationRestoresLegacyConstraintsWhenEmpty(t *testing.T) {
	ctx := context.Background()
	pool := webReferralPool(t, ctx)

	if err := runMigrationFileResult(t, ctx, pool, "000056_web_referrals.down.sql"); err != nil {
		t.Fatalf("empty down migration: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO referral_codes (code)
		VALUES ('NOOWNER234')`); err == nil {
		t.Fatal("ownerless referral code insert succeeded after rollback")
	}

	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	referrerUserID := insertUser(t, ctx, pool, 860001)
	referredUserID := insertUser(t, ctx, pool, 860002)
	if _, err := pool.Exec(ctx, `
		INSERT INTO referral_codes (user_id, code)
		VALUES ($1, 'LEGDOWN234')`, referrerUserID); err != nil {
		t.Fatalf("insert legacy code after rollback: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO referrals (
			referrer_user_id, referred_user_id, referral_code, source, status, reward_status, first_seen_at
		) VALUES ($1, $2, 'LEGDOWN234', 'web', 'registered', 'pending', $3)`, referrerUserID, referredUserID, now); err == nil {
		t.Fatal("web-source referral insert succeeded after rollback")
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO referrals (
			referrer_user_id, referred_user_id, referral_code, source, status, reward_status, first_seen_at
		) VALUES ($1, $2, 'LEGDOWN234', 'vk_bot', 'registered', 'pending', $3)`, referrerUserID, referredUserID, now); err != nil {
		t.Fatalf("legacy referral insert after rollback: %v", err)
	}
}

func webReferralPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL web referral integration test")
	}
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	defer admin.Close()
	schema := "web_referral_" + strings.ReplaceAll(uuid.NewString(), "-", "_")
	if _, err := admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
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
	for _, path := range webReferralMigrationUpFiles(t, root, "000056") {
		if err := runMigrationPathResult(t, ctx, pool, path); err != nil {
			t.Fatalf("run migration %s: %v", filepath.Base(path), err)
		}
	}
	return pool
}

func runMigrationFileResult(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string) error {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		return err
	}
	return runMigrationPathResult(t, ctx, pool, filepath.Join(root, "migrations", name))
}

func runMigrationPathResult(t *testing.T, ctx context.Context, pool *pgxpool.Pool, path string) error {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, stmt := range splitWebReferralMigrationStatements(string(raw)) {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func webReferralMigrationUpFiles(t *testing.T, root, maxVersion string) []string {
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

func splitWebReferralMigrationStatements(script string) []string {
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
				if validDollarQuoteTag(tagBody) {
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

func validDollarQuoteTag(tag string) bool {
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

func insertAccount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, createdAt time.Time, status string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO accounts (id, status, role, account_type, locale, timezone, risk_level, created_at, updated_at)
		VALUES ($1, $2, 'user', 'personal', 'ru', 'Europe/Moscow', 0, $3, $3)`, id, status, createdAt); err != nil {
		t.Fatalf("insert account: %v", err)
	}
	return id
}

func insertUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, vkUserID int64) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (vk_user_id)
		VALUES ($1)
		RETURNING id`, vkUserID).Scan(&id); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

func assertWebReferralRow(t *testing.T, ctx context.Context, pool *pgxpool.Pool, referrerID, referredID uuid.UUID, code string) {
	t.Helper()
	var source, status, rewardStatus string
	var referrerUserNil, referredUserNil bool
	if err := pool.QueryRow(ctx, `
		SELECT source, status, reward_status, referrer_user_id IS NULL, referred_user_id IS NULL
		FROM referrals
		WHERE referrer_account_id = $1 AND referred_account_id = $2 AND referral_code = $3`, referrerID, referredID, code).
		Scan(&source, &status, &rewardStatus, &referrerUserNil, &referredUserNil); err != nil {
		t.Fatalf("load referral row: %v", err)
	}
	if source != "web" || status != "registered" || rewardStatus != "pending" || !referrerUserNil || !referredUserNil {
		t.Fatalf("referral row source=%q status=%q reward=%q referrer_user_nil=%v referred_user_nil=%v",
			source, status, rewardStatus, referrerUserNil, referredUserNil)
	}
}

func insertWebReferralVisit(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tokenHash, code string, referrerID uuid.UUID, createdAt, expiresAt time.Time, acceptedAccountID uuid.UUID) {
	t.Helper()
	var accepted any
	if acceptedAccountID != uuid.Nil {
		accepted = acceptedAccountID
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO web_referral_visits (token_hash, code, referrer_account_id, created_at, expires_at, accepted_account_id)
		VALUES ($1, $2, $3, $4, $5, $6)`, tokenHash, code, referrerID, createdAt, expiresAt, accepted); err != nil {
		t.Fatalf("insert visit %s: %v", tokenHash[:4], err)
	}
}

func assertWebReferralSummaryCounts(t *testing.T, ctx context.Context, repo *postgres.WebReferralRepository, accountID uuid.UUID, visits int) {
	t.Helper()
	summary, err := repo.Summary(ctx, accountID)
	if err != nil {
		t.Fatalf("summary for %s: %v", accountID, err)
	}
	if summary.Visits != visits {
		t.Fatalf("summary visits for %s = %d, want %d", accountID, summary.Visits, visits)
	}
}

func assertWebReferralRelationCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, referredID uuid.UUID, want int) {
	t.Helper()
	var got int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)::int
		FROM referrals
		WHERE referred_account_id = $1
		  AND source = 'web'`, referredID).Scan(&got); err != nil {
		t.Fatalf("count accepted relation: %v", err)
	}
	if got != want {
		t.Fatalf("accepted relation count = %d, want %d", got, want)
	}
}

type nonTransactionalWebReferralQuerier struct{}

func (nonTransactionalWebReferralQuerier) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	panic("unexpected Exec")
}

func (nonTransactionalWebReferralQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("unexpected Query")
}

func (nonTransactionalWebReferralQuerier) QueryRow(context.Context, string, ...any) pgx.Row {
	panic("unexpected QueryRow")
}
