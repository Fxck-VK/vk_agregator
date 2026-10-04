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

func TestConcurrentPostgresUnlinkPreservesSignIn(t *testing.T) {
	ctx := context.Background()
	pool := conversationManagementIntegrationPool(t, ctx)
	_, err := pool.Exec(ctx, `CREATE TABLE account_identities (id UUID PRIMARY KEY, account_id UUID NOT NULL REFERENCES accounts(id), provider TEXT NOT NULL, verified_at TIMESTAMPTZ);
		CREATE TABLE account_links_audit (id UUID PRIMARY KEY, account_id UUID NOT NULL, actor_account_id UUID, action TEXT NOT NULL, provider TEXT NOT NULL, identity_id UUID, created_at TIMESTAMPTZ NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	accountID, emailID, googleID, phoneID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO accounts(id) VALUES($1)", accountID); err != nil {
		t.Fatal(err)
	}
	for provider, id := range map[string]uuid.UUID{"email": emailID, "google": googleID, "phone": phoneID} {
		if _, err := pool.Exec(ctx, "INSERT INTO account_identities(id, account_id, provider, verified_at) VALUES($1,$2,$3,now())", id, accountID, provider); err != nil {
			t.Fatal(err)
		}
	}
	repo := postgres.NewAccountIdentityRepository(pool)
	var group sync.WaitGroup
	results := make(chan error, 2)
	for _, id := range []uuid.UUID{emailID, googleID} {
		group.Add(1)
		go func() { defer group.Done(); results <- repo.UnlinkIdentity(ctx, accountID, id) }()
	}
	group.Wait()
	close(results)
	winners := 0
	for err := range results {
		if err == nil {
			winners++
		} else if !errors.Is(err, domain.ErrAccountLastIdentity) {
			t.Fatal(err)
		}
	}
	if winners != 1 {
		t.Fatalf("removed %d sign-in methods, expected one", winners)
	}
	if err := repo.UnlinkIdentity(ctx, accountID, phoneID); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM account_identities WHERE account_id=$1", accountID).Scan(&remaining); err != nil || remaining != 1 {
		t.Fatal("last sign-in was not preserved")
	}
}
