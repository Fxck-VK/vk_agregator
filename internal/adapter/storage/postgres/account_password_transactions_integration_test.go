package postgres_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
)

func passwordSecurityIntegrationPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	pool := conversationManagementIntegrationPool(t, ctx)
	_, err := pool.Exec(ctx, `CREATE TABLE account_identities(id uuid PRIMARY KEY, account_id uuid REFERENCES accounts(id), provider text NOT NULL, external_id text NOT NULL, normalized_id text NOT NULL, verified_at timestamptz, last_used_at timestamptz, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL, UNIQUE(provider,normalized_id));
 CREATE TABLE account_credentials(id uuid PRIMARY KEY, account_id uuid REFERENCES accounts(id), credential_type text, secret_hash text, changed_at timestamptz, created_at timestamptz, updated_at timestamptz, UNIQUE(account_id,credential_type));
 CREATE TABLE account_sessions(id uuid PRIMARY KEY, account_id uuid NOT NULL REFERENCES accounts(id), identity_id uuid, access_token_hash text UNIQUE, access_expires_at timestamptz, refresh_token_hash text NOT NULL UNIQUE, device_id text NOT NULL, ip_hash text NOT NULL, user_agent_hash text NOT NULL, expires_at timestamptz NOT NULL, revoked_at timestamptz, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL);
 CREATE TABLE account_links_audit(id uuid PRIMARY KEY, account_id uuid, actor_account_id uuid, action text, provider text, identity_id uuid, created_at timestamptz)`)
	if err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../../../migrations/000055_account_security_notices.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	return pool
}

func passwordTransactionFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (domain.AccountIdentity, domain.AccountCredential, domain.AccountSession) {
	t.Helper()
	owner := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO accounts(id) VALUES($1)", owner); err != nil {
		t.Fatal(err)
	}
	binding, err := postgres.NewAccountIdentityRepository(pool).LinkIdentity(ctx, owner, domain.IdentityProviderEmail, "member@example.test", "member@example.test")
	if err != nil {
		t.Fatal(err)
	}
	credential := domain.AccountCredential{ID: uuid.New(), AccountID: owner, CredentialType: domain.AccountCredentialPassword, SecretHash: "synthetic-original-verifier"}
	if _, err := postgres.NewAccountSecurityRepository(pool).UpsertCredential(ctx, credential); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Truncate(time.Microsecond)
	access := at.Add(time.Hour)
	session := domain.AccountSession{ID: uuid.New(), AccountID: owner, AccessTokenHash: uuid.NewString(), AccessExpiresAt: &access, RefreshTokenHash: uuid.NewString(), DeviceID: "synthetic", IPHash: "synthetic", UserAgentHash: "synthetic", CreatedAt: at, UpdatedAt: at, ExpiresAt: access}
	if _, err := postgres.NewAccountSessionRepository(pool).CreateSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	return *binding, credential, session
}

func TestPostgresRefreshSerializesWithPasswordRevocation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool := passwordSecurityIntegrationPool(t, ctx)
	binding, _, old := passwordTransactionFixture(t, ctx, pool)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT id FROM accounts WHERE id=$1 FOR UPDATE", binding.AccountID); err != nil {
		t.Fatal(err)
	}
	replacement := old
	replacement.ID = uuid.New()
	replacement.RefreshTokenHash = uuid.NewString()
	replacement.AccessTokenHash = uuid.NewString()
	done := make(chan error, 1)
	go func() {
		_, err := postgres.NewAccountSessionRepository(pool).RotateSession(ctx, old.RefreshTokenHash, replacement)
		done <- err
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		select {
		case err := <-done:
			t.Fatalf("refresh bypassed password account lock: %v", err)
		default:
		}
		var blocked bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE '%account-session-rotation%')`).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("refresh did not wait on account lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := tx.Exec(ctx, "UPDATE account_sessions SET revoked_at=now() WHERE account_id=$1", binding.AccountID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("revoked refresh result: %v", err)
	}
}

func TestPostgresPasswordTransactionRollbackAndConcurrentCAS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool := passwordSecurityIntegrationPool(t, ctx)
	binding, original, current := passwordTransactionFixture(t, ctx, pool)
	sessions := postgres.NewAccountSessionRepository(pool)
	other := current
	other.ID = uuid.New()
	other.RefreshTokenHash = uuid.NewString()
	other.AccessTokenHash = uuid.NewString()
	if _, err := sessions.CreateSession(ctx, other); err != nil {
		t.Fatal(err)
	}
	assertSessionState := func(session domain.AccountSession, wantRevoked bool) {
		t.Helper()
		stored, err := sessions.FindSessionByRefreshHash(ctx, session.RefreshTokenHash)
		if err != nil {
			t.Fatal(err)
		}
		if (stored.RevokedAt != nil) != wantRevoked {
			t.Fatalf("session revoked = %v, want %v", stored.RevokedAt != nil, wantRevoked)
		}
	}
	repo := postgres.NewAccountSecurityRepository(pool)
	next := original
	next.SecretHash = "synthetic-next-verifier"
	at := time.Now().UTC().Truncate(time.Microsecond)
	op := domain.PasswordSecurityOperation{AccountID: binding.AccountID, EmailBinding: binding, ExpectedHash: original.SecretHash, Credential: &next, PreserveSessionID: current.ID, At: at, Audit: domain.AccountLinkAuditEntry{ID: uuid.New(), AccountID: binding.AccountID, Action: domain.AccountLinkActionPasswordSet, Provider: domain.IdentityProviderEmail, CreatedAt: at}}
	if _, err := pool.Exec(ctx, "ALTER TABLE account_links_audit ADD CONSTRAINT reject_password CHECK(action<>'password_set') NOT VALID"); err != nil {
		t.Fatal(err)
	}
	if err := repo.ExecutePasswordSecurity(ctx, op, domain.PasswordSecurityDependencies{}); err == nil {
		t.Fatal("audit failure ignored")
	}
	stored, _ := repo.FindCredential(ctx, binding.AccountID, domain.AccountCredentialPassword)
	if stored.SecretHash != original.SecretHash {
		t.Fatal("failed audit committed password")
	}
	var notices, revoked int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM account_security_notices WHERE kind='password_changed'").Scan(&notices); err != nil || notices != 0 {
		t.Fatal("failed audit enqueued notice")
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM account_sessions WHERE revoked_at IS NOT NULL").Scan(&revoked); err != nil || revoked != 0 {
		t.Fatal("failed audit revoked session")
	}
	assertSessionState(current, false)
	assertSessionState(other, false)
	if _, err := pool.Exec(ctx, "ALTER TABLE account_links_audit DROP CONSTRAINT reject_password"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE account_security_notices ADD CONSTRAINT reject_password_notice CHECK(kind<>'password_changed') NOT VALID"); err != nil {
		t.Fatal(err)
	}
	if err := repo.ExecutePasswordSecurity(ctx, op, domain.PasswordSecurityDependencies{}); err == nil {
		t.Fatal("notice persistence failure ignored")
	}
	stored, _ = repo.FindCredential(ctx, binding.AccountID, domain.AccountCredentialPassword)
	if stored.SecretHash != original.SecretHash {
		t.Fatal("failed notice committed credential")
	}
	var passwordAudits int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM account_links_audit WHERE action='password_set'").Scan(&passwordAudits); err != nil || passwordAudits != 0 {
		t.Fatal("failed notice committed audit")
	}
	assertSessionState(current, false)
	assertSessionState(other, false)
	if _, err := pool.Exec(ctx, "ALTER TABLE account_security_notices DROP CONSTRAINT reject_password_notice"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			request := op
			request.Audit.ID = uuid.New()
			results <- repo.ExecutePasswordSecurity(ctx, request, domain.PasswordSecurityDependencies{})
		}()
	}
	wg.Wait()
	close(results)
	winners := 0
	for err := range results {
		if err == nil {
			winners++
		} else if !errors.Is(err, domain.ErrConflict) {
			t.Fatal(err)
		}
	}
	if winners != 1 {
		t.Fatalf("successful password writers: %d", winners)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM account_security_notices WHERE kind='password_changed'").Scan(&notices); err != nil || notices != 1 {
		t.Fatal("password transaction notice missing or duplicated")
	}
	assertSessionState(current, false)
	assertSessionState(other, true)
}

func TestPostgresPasswordLoginRejectsStaleCredentialAndBinding(t *testing.T) {
	for _, mutation := range []string{"reset", "change", "unlink", "replace"} {
		t.Run(mutation, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			pool := passwordSecurityIntegrationPool(t, ctx)
			binding, credential, session := passwordTransactionFixture(t, ctx, pool)
			session.ID = uuid.New()
			session.RefreshTokenHash = uuid.NewString()
			session.AccessTokenHash = uuid.NewString()
			at := time.Now().UTC().Truncate(time.Microsecond)
			op := domain.PasswordSecurityOperation{AccountID: binding.AccountID, EmailBinding: binding, ExpectedHash: credential.SecretHash, Session: &session, At: at, Audit: domain.AccountLinkAuditEntry{ID: uuid.New(), AccountID: binding.AccountID, Action: domain.AccountLinkActionLogin, Provider: domain.IdentityProviderEmail, CreatedAt: at}}
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err := tx.Exec(ctx, "SELECT id FROM accounts WHERE id=$1 FOR UPDATE", binding.AccountID); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				done <- postgres.NewAccountSecurityRepository(pool).ExecutePasswordSecurity(ctx, op, domain.PasswordSecurityDependencies{})
			}()
			deadline := time.Now().Add(3 * time.Second)
			for {
				select {
				case err := <-done:
					t.Fatalf("password login bypassed account lock: %v", err)
				default:
				}
				var blocked bool
				if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE '%account-password-security%')`).Scan(&blocked); err != nil {
					t.Fatal(err)
				}
				if blocked {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("password login did not wait on account lock")
				}
				time.Sleep(10 * time.Millisecond)
			}
			switch mutation {
			case "reset", "change":
				_, err = tx.Exec(ctx, "UPDATE account_credentials SET secret_hash='synthetic-replacement-verifier' WHERE account_id=$1", binding.AccountID)
			case "unlink":
				_, err = tx.Exec(ctx, "DELETE FROM account_identities WHERE id=$1", binding.ID)
			case "replace":
				_, err = tx.Exec(ctx, "UPDATE account_identities SET updated_at=updated_at + INTERVAL '1 second' WHERE id=$1", binding.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if err := <-done; !errors.Is(err, domain.ErrConflict) && !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("stale login transaction result: %v", err)
			}
			var issued, audits int
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM account_sessions WHERE id=$1", session.ID).Scan(&issued); err != nil || issued != 0 {
				t.Fatal("stale login issued session")
			}
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM account_links_audit WHERE action='login'").Scan(&audits); err != nil || audits != 0 {
				t.Fatal("stale login committed audit")
			}
		})
	}
}
