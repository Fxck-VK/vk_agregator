package postgres_test

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
)

func applySecurityNoticeMigration(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	sql, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "migrations", "000055_account_security_notices.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(context.Background(), string(sql)); err != nil {
		t.Fatal("notice migration failed")
	}
}
func noticeEmailFixture(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := conversationManagementIntegrationPool(t, context.Background())
	if _, err := pool.Exec(context.Background(), `CREATE TABLE account_identities (
 id uuid PRIMARY KEY,account_id uuid NOT NULL REFERENCES accounts(id),provider text NOT NULL,
 external_id text NOT NULL,normalized_id text NOT NULL,verified_at timestamptz,last_used_at timestamptz,
 created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,UNIQUE(provider,normalized_id));
 CREATE TABLE account_links_audit(id uuid PRIMARY KEY,account_id uuid NOT NULL,actor_account_id uuid,action text NOT NULL,provider text NOT NULL,identity_id uuid,created_at timestamptz NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	applySecurityNoticeMigration(t, pool)
	return pool
}
func TestPostgresSecurityNoticeMutationRollbackAndRecipients(t *testing.T) {
	ctx := context.Background()
	pool := noticeEmailFixture(t)
	id := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO accounts(id) VALUES($1)", id); err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewAccountIdentityRepository(pool)
	_, err := repo.LinkIdentity(ctx, id, domain.IdentityProviderEmail, "primary@example.test", "primary@example.test")
	if err != nil {
		t.Fatal(err)
	}
	count := func(want int) {
		t.Helper()
		var got int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM account_security_notices").Scan(&got); err != nil || got != want {
			t.Fatalf("outbox count=%d want=%d", got, want)
		}
	}
	auditCount := func(want int) {
		t.Helper()
		var got int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM account_links_audit WHERE account_id=$1", id).Scan(&got); err != nil || got != want {
			t.Fatalf("audit count=%d want=%d", got, want)
		}
	}
	recipients := func(db postgres.Querier, kind domain.AccountSecurityNoticeKind, expected ...string) {
		t.Helper()
		rows, err := db.Query(ctx, "SELECT recipient FROM account_security_notices WHERE account_id=$1 AND kind=$2", id, kind)
		if err != nil {
			t.Fatal("query notice recipients failed")
		}
		defer rows.Close()
		remaining := make(map[string]bool, len(expected))
		for _, address := range expected {
			remaining[address] = true
		}
		got := 0
		for rows.Next() {
			var address string
			if err := rows.Scan(&address); err != nil {
				t.Fatal("scan notice recipient failed")
			}
			if !remaining[address] {
				t.Fatalf("unexpected or duplicate notice recipient for event %s", kind)
			}
			delete(remaining, address)
			got++
		}
		if rows.Err() != nil || len(remaining) != 0 || got != len(expected) {
			t.Fatalf("recipient set mismatch for event %s: count=%d want=%d", kind, got, len(expected))
		}
	}
	count(0)
	auditCount(1)
	// Fail closed when durable enqueue is unavailable, rolling back binding and audit.
	if _, err := pool.Exec(ctx, "ALTER TABLE account_security_notices ADD CONSTRAINT reject_notice CHECK(false) NOT VALID"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.LinkIdentity(ctx, id, domain.IdentityProviderEmail, "backup@example.test", "backup@example.test"); err == nil {
		t.Fatal("outbox failure accepted mutation")
	}
	count(0)
	auditCount(1)
	if _, err := repo.ResolveIdentity(ctx, domain.IdentityProviderEmail, "backup@example.test"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("binding escaped rollback")
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE account_security_notices DROP CONSTRAINT reject_notice"); err != nil {
		t.Fatal(err)
	}
	backup, err := repo.LinkIdentity(ctx, id, domain.IdentityProviderEmail, "backup@example.test", "backup@example.test")
	if err != nil {
		t.Fatal(err)
	}
	count(2)
	auditCount(2)
	recipients(pool, domain.AccountSecurityNoticeBackupEmailAdded, "primary@example.test", "backup@example.test")
	// A legacy unverified email is never a notification recipient, including
	// after replacement/removal and when the password helper captures recipients.
	if _, err := pool.Exec(ctx, `INSERT INTO account_identities
		(id,account_id,provider,external_id,normalized_id,created_at,updated_at)
		VALUES($1,$2,'email','unverified@example.test','unverified@example.test',now(),now())`, uuid.New(), id); err != nil {
		t.Fatal("seed unverified identity failed")
	}
	replacement, err := repo.ReplaceBackupEmailIdentity(ctx, id, backup.ID, "new@example.test", "new@example.test", backup.UpdatedAt)
	if err != nil {
		t.Fatal(err)
	}
	count(5)
	auditCount(4)
	recipients(pool, domain.AccountSecurityNoticeBackupEmailReplaced, "primary@example.test", "backup@example.test", "new@example.test")
	if _, err := pool.Exec(ctx, "ALTER TABLE account_security_notices ADD CONSTRAINT reject_notice CHECK(false) NOT VALID"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReplaceBackupEmailIdentity(ctx, id, replacement.ID, "rollback@example.test", "rollback@example.test", replacement.UpdatedAt); err == nil {
		t.Fatal("replacement ignored outbox failure")
	}
	auditCount(4)
	if err := repo.UnlinkIdentity(ctx, id, replacement.ID); err == nil {
		t.Fatal("unlink ignored outbox failure")
	}
	auditCount(4)
	if _, err := repo.ResolveIdentity(ctx, domain.IdentityProviderEmail, replacement.NormalizedID); err != nil {
		t.Fatal("outbox failure removed current email")
	}
	count(5)
	recipients(pool, domain.AccountSecurityNoticeBackupEmailReplaced, "primary@example.test", "backup@example.test", "new@example.test")
	if _, err := pool.Exec(ctx, "ALTER TABLE account_security_notices DROP CONSTRAINT reject_notice"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReplaceBackupEmailIdentity(ctx, id, backup.ID, "stale@example.test", "stale@example.test", backup.UpdatedAt); !errors.Is(err, domain.ErrAccountBackupEmailChanged) {
		t.Fatal("stale replacement accepted")
	}
	count(5)
	auditCount(4)
	if err := repo.UnlinkIdentity(ctx, id, replacement.ID); err != nil {
		t.Fatal(err)
	}
	count(7)
	auditCount(5)
	recipients(pool, domain.AccountSecurityNoticeEmailRemoved, "primary@example.test", "new@example.test")
	// Enqueue is invisible until commit; rollback removes all deduplicated snapshots.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := postgres.EnqueueAccountSecurityNotifications(ctx, tx, id, domain.AccountSecurityNoticePasswordChanged, []string{"PRIMARY@example.test", " primary@example.test "}, time.Now()); err != nil {
		t.Fatal(err)
	}
	var staged int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM account_security_notices").Scan(&staged); err != nil || staged != 8 {
		t.Fatal("deduplication failed")
	}
	recipients(tx, domain.AccountSecurityNoticePasswordChanged, "primary@example.test")
	recipients(pool, domain.AccountSecurityNoticePasswordChanged)
	count(7)
	auditCount(5)
	_ = tx.Rollback(ctx)
	count(7)
	auditCount(5)
	recipients(pool, domain.AccountSecurityNoticePasswordChanged)
}
func TestPostgresSecurityNoticeLeaseRetryAndRetention(t *testing.T) {
	ctx := context.Background()
	pool := noticeEmailFixture(t)
	owner := uuid.New()
	at := time.Now().UTC()
	if _, err := pool.Exec(ctx, "INSERT INTO accounts(id) VALUES($1)", owner); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if _, err := pool.Exec(ctx, `INSERT INTO account_security_notices(id,account_id,kind,recipient,event_at,next_attempt_at) VALUES($1,$2,'password_changed','fixture@example.test',$3,$3)`, uuid.New(), owner, at); err != nil {
			t.Fatal(err)
		}
	}
	repo := postgres.NewAccountSecurityNoticeRepository(pool)
	var wg sync.WaitGroup
	leased := make(chan []domain.AccountSecurityNotice, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rows, err := repo.LeaseAccountSecurityNotices(ctx, at, 2, time.Minute)
			leased <- rows
			errs <- err
		}()
	}
	wg.Wait()
	close(leased)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	all := make([]domain.AccountSecurityNotice, 0)
	seen := map[uuid.UUID]bool{}
	for rows := range leased {
		if len(rows) != 2 {
			t.Fatal("unbounded/missing lease")
		}
		for _, row := range rows {
			if seen[row.ID] {
				t.Fatal("duplicate lease")
			}
			seen[row.ID] = true
			all = append(all, row)
		}
	}
	first := all[0]
	if err := repo.RetryAccountSecurityNotice(ctx, first.ID, first.LeaseToken, at, at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	rows, err := repo.LeaseAccountSecurityNotices(ctx, at, 10, time.Minute)
	if err != nil || len(rows) != 0 {
		t.Fatal("premature retry")
	}
	rows, err = repo.LeaseAccountSecurityNotices(ctx, at.Add(time.Minute), 10, time.Minute)
	if err != nil || len(rows) != 4 {
		t.Fatal("retry/expired lease not recovered")
	}
	if err := repo.CompleteAccountSecurityNotice(ctx, first.ID, first.LeaseToken, at.Add(time.Minute)); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("stale lease acknowledged")
	}
	for _, row := range rows {
		if err := repo.CompleteAccountSecurityNotice(ctx, row.ID, row.LeaseToken, at.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.CleanupAccountSecurityNotices(ctx, at.Add(domain.AccountSecurityNoticeSentRetention+2*time.Minute), 1000); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM account_security_notices").Scan(&count); err != nil || count != 0 {
		t.Fatal("sent retention failed")
	}
}
