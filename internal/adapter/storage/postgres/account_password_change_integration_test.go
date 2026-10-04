package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
)

func TestPostgresPasswordConditionalWrites(t *testing.T) {
	ctx := context.Background()
	pool := conversationManagementIntegrationPool(t, ctx)
	if _, err := pool.Exec(ctx, `CREATE TABLE account_credentials(id UUID PRIMARY KEY, account_id UUID REFERENCES accounts(id), credential_type TEXT, secret_hash TEXT, changed_at TIMESTAMPTZ, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ, UNIQUE(account_id, credential_type))`); err != nil {
		t.Fatal(err)
	}
	owner := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO accounts(id) VALUES($1)", owner); err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewAccountSecurityRepository(pool)
	value := domain.AccountCredential{ID: uuid.New(), AccountID: owner, CredentialType: domain.AccountCredentialPassword, SecretHash: "synthetic-original-verifier"}
	if _, err := repo.CompareAndSwapCredential(ctx, value, "missing-verifier"); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("conditional update inserted a missing credential")
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := repo.CompareAndSwapCredential(ctx, value, ""); errs <- err }()
	}
	wg.Wait()
	close(errs)
	success := 0
	for err := range errs {
		if err == nil {
			success++
		} else if !errors.Is(err, domain.ErrConflict) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("successful initial writers: %d", success)
	}
	value.SecretHash = "synthetic-updated-verifier"
	if _, err := repo.CompareAndSwapCredential(ctx, value, "synthetic-original-verifier"); err != nil {
		t.Fatal(err)
	}
	value.SecretHash = "synthetic-stale-verifier"
	if _, err := repo.CompareAndSwapCredential(ctx, value, "synthetic-original-verifier"); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("stale proof replaced the credential")
	}
	stored, err := repo.FindCredential(ctx, owner, domain.AccountCredentialPassword)
	if err != nil || stored.SecretHash != "synthetic-updated-verifier" {
		t.Fatal("conditional update corrupted credential")
	}
}
