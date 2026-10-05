package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/identityresolver"
)

type unlinkRecoveryLimiter struct{ unlink func() }

func (l unlinkRecoveryLimiter) Allow(context.Context, string) (bool, error) {
	l.unlink()
	return true, nil
}

func TestPostgresRecoveryRechecksEmailAndRollsBackAllSecurityWrites(t *testing.T) {
	ctx := context.Background()
	pool := conversationManagementIntegrationPool(t, ctx)
	if _, err := pool.Exec(ctx, `CREATE TABLE account_identities (
 id uuid PRIMARY KEY, account_id uuid NOT NULL REFERENCES accounts(id), provider text NOT NULL,
 external_id text NOT NULL, normalized_id text NOT NULL, verified_at timestamptz, last_used_at timestamptz,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL, UNIQUE(provider, normalized_id));
 CREATE TABLE account_credentials (id uuid PRIMARY KEY, account_id uuid REFERENCES accounts(id), credential_type text, secret_hash text, changed_at timestamptz, created_at timestamptz, updated_at timestamptz, UNIQUE(account_id,credential_type));
 CREATE TABLE account_sessions(id uuid PRIMARY KEY, account_id uuid, revoked_at timestamptz, updated_at timestamptz);
 CREATE TABLE account_links_audit(id uuid PRIMARY KEY, account_id uuid, actor_account_id uuid, action text, provider text, identity_id uuid, created_at timestamptz)`); err != nil {
		t.Fatal(err)
	}
	owner := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO accounts(id) VALUES($1)", owner); err != nil {
		t.Fatal(err)
	}
	identities := postgres.NewAccountIdentityRepository(pool)
	security := postgres.NewAccountSecurityRepository(pool)
	if _, err := identities.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, "primary@example.test", "primary@example.test"); err != nil {
		t.Fatal(err)
	}
	backup, err := identities.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, "backup@example.test", "backup@example.test")
	if err != nil {
		t.Fatal(err)
	}
	original := domain.AccountCredential{ID: uuid.New(), AccountID: owner, CredentialType: domain.AccountCredentialPassword, SecretHash: "synthetic-original-verifier"}
	if _, err := security.UpsertCredential(ctx, original); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO account_sessions(id,account_id) VALUES($1,$2)", uuid.New(), owner); err != nil {
		t.Fatal(err)
	}
	limiter := unlinkRecoveryLimiter{unlink: func() {
		if err := identities.UnlinkIdentity(ctx, owner, backup.ID); err != nil {
			t.Fatal(err)
		}
	}}
	auth := accountauth.New(identityresolver.New(nil, identities, nil), accountauth.WithCredentialRepository(security), accountauth.WithLimiter(limiter), accountauth.WithSessionRepository(postgres.NewAccountSessionRepository(pool)), accountauth.WithAccountAuditRepository(security))
	if err := auth.ResetPasswordForVerifiedEmail(ctx, owner, "backup@example.test", "replacement-password"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("removed recovery result: %v", err)
	}
	stored, err := security.FindCredential(ctx, owner, domain.AccountCredentialPassword)
	if err != nil || stored.SecretHash != original.SecretHash {
		t.Fatal("removed backup changed credential")
	}
	var revoked int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM account_sessions WHERE revoked_at IS NOT NULL").Scan(&revoked); err != nil || revoked != 0 {
		t.Fatal("failed recovery revoked sessions")
	}
	next := original
	next.SecretHash = "synthetic-next-verifier"
	if _, err := pool.Exec(ctx, "ALTER TABLE account_links_audit ADD CONSTRAINT reject_reset CHECK(action <> 'password_reset') NOT VALID"); err != nil {
		t.Fatal(err)
	}
	if err := security.ResetCredentialForLinkedEmail(ctx, next, "primary@example.test", true, true, time.Now()); err == nil {
		t.Fatal("audit failure ignored")
	}
	stored, _ = security.FindCredential(ctx, owner, domain.AccountCredentialPassword)
	if stored.SecretHash != original.SecretHash {
		t.Fatal("audit failure changed credential")
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM account_sessions WHERE revoked_at IS NOT NULL").Scan(&revoked); err != nil || revoked != 0 {
		t.Fatal("audit failure revoked sessions")
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE account_links_audit DROP CONSTRAINT reject_reset"); err != nil {
		t.Fatal(err)
	}
	if err := security.ResetCredentialForLinkedEmail(ctx, next, "primary@example.test", true, true, time.Now()); err != nil {
		t.Fatal(err)
	}
	stored, _ = security.FindCredential(ctx, owner, domain.AccountCredentialPassword)
	if stored.SecretHash != next.SecretHash {
		t.Fatal("successful recovery did not update credential")
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM account_sessions WHERE revoked_at IS NOT NULL").Scan(&revoked); err != nil || revoked != 1 {
		t.Fatal("successful recovery did not revoke session")
	}
}
