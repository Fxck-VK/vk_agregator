package postgres_test

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"sync"
	"testing"
	"time"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
)

func TestPostgresEmailRegistrationIsAtomicAndPreservesCredentialOnRetry(t *testing.T) {
	ctx := context.Background()
	pool := conversationManagementIntegrationPool(t, ctx)
	_, err := pool.Exec(ctx, `ALTER TABLE accounts ADD status TEXT, ADD role TEXT, ADD account_type TEXT, ADD locale TEXT, ADD timezone TEXT, ADD risk_level INT, ADD created_at TIMESTAMPTZ, ADD updated_at TIMESTAMPTZ;
 CREATE TABLE account_identities(id UUID PRIMARY KEY,account_id UUID REFERENCES accounts(id),provider TEXT,external_id TEXT,normalized_id TEXT,verified_at TIMESTAMPTZ,last_used_at TIMESTAMPTZ,created_at TIMESTAMPTZ,updated_at TIMESTAMPTZ,UNIQUE(provider,normalized_id));
 CREATE TABLE account_credentials(id UUID PRIMARY KEY,account_id UUID REFERENCES accounts(id),credential_type TEXT,secret_hash TEXT CHECK(secret_hash <> 'reject-for-test'),changed_at TIMESTAMPTZ,created_at TIMESTAMPTZ,updated_at TIMESTAMPTZ,UNIQUE(account_id,credential_type));
 CREATE TABLE account_links_audit(id UUID PRIMARY KEY,account_id UUID REFERENCES accounts(id),actor_account_id UUID,action TEXT,provider TEXT,identity_id UUID,created_at TIMESTAMPTZ);`)
	if err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewAccountIdentityRepository(pool)
	if _, err := repo.RegisterEmailAccount(ctx, uuid.New(), "failure@example.test", "reject-for-test", time.Now()); err == nil {
		t.Fatal("constraint failure ignored")
	}
	var count int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM accounts").Scan(&count)
	if count != 0 {
		t.Fatal("partial account committed")
	}
	var wg sync.WaitGroup
	results := make(chan domain.IdentityResolution, 2)
	failures := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			row, err := repo.RegisterEmailAccount(ctx, uuid.New(), "new@example.test", "test-hash", time.Now())
			if err != nil {
				failures <- err
			} else {
				results <- row
			}
		}()
	}
	wg.Wait()
	close(results)
	close(failures)
	successes := 0
	var owner uuid.UUID
	for row := range results {
		successes++
		owner = row.AccountID
	}
	for err := range failures {
		if !errors.Is(err, domain.ErrConflict) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("expected one email owner, got %d", successes)
	}
	if _, err := repo.RegisterEmailAccount(ctx, owner, "new@example.test", "changed-hash", time.Now()); err != nil {
		t.Fatal(err)
	}
	credential, err := postgres.NewAccountSecurityRepository(pool).FindCredential(ctx, owner, domain.AccountCredentialPassword)
	if err != nil || credential.SecretHash != "test-hash" {
		t.Fatal("retry replaced password")
	}
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM accounts").Scan(&count)
	if count != 1 {
		t.Fatal("conflict left orphan account")
	}
}
