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
	pool := noticeEmailFixture(t)
	accountID, emailID, googleID, phoneID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO accounts(id) VALUES($1)", accountID); err != nil {
		t.Fatal(err)
	}
	for provider, id := range map[string]uuid.UUID{"email": emailID, "google": googleID, "phone": phoneID} {
		if _, err := pool.Exec(ctx, "INSERT INTO account_identities(id, account_id, provider, external_id, normalized_id, verified_at,created_at,updated_at) VALUES($1,$2,$3,$4,$4,now(),now(),now())", id, accountID, provider, provider+"@example.test"); err != nil {
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

func TestPostgresUnlinkDoesNotCountDisabledGoogleAsUsableLogin(t *testing.T) {
	ctx := context.Background()
	pool := noticeEmailFixture(t)
	accountID, emailID, googleID := uuid.New(), uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO accounts(id) VALUES($1)", accountID); err != nil {
		t.Fatal(err)
	}
	for provider, id := range map[string]uuid.UUID{"email": emailID, "google": googleID} {
		if _, err := pool.Exec(ctx, "INSERT INTO account_identities(id, account_id, provider, external_id, normalized_id, verified_at,created_at,updated_at) VALUES($1,$2,$3,$4,$4,now(),now(),now())", id, accountID, provider, provider+"@example.test"); err != nil {
			t.Fatal(err)
		}
	}
	repo := postgres.NewAccountIdentityRepository(pool)
	if err := repo.UnlinkIdentity(ctx, accountID, emailID); !errors.Is(err, domain.ErrAccountLastIdentity) {
		t.Fatalf("email unlink error = %v, want ErrAccountLastIdentity", err)
	}
	if err := repo.UnlinkIdentity(ctx, accountID, googleID); err != nil {
		t.Fatalf("disabled google unlink: %v", err)
	}
	var remainingProvider string
	if err := pool.QueryRow(ctx, "SELECT provider FROM account_identities WHERE account_id=$1", accountID).Scan(&remainingProvider); err != nil || remainingProvider != "email" {
		t.Fatalf("remaining provider = %q, err=%v; want email", remainingProvider, err)
	}
}

func TestPostgresUnlinkCountsConfiguredGoogleAsUsableLogin(t *testing.T) {
	ctx := context.Background()
	pool := noticeEmailFixture(t)
	accountID, emailID, googleID := uuid.New(), uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO accounts(id) VALUES($1)", accountID); err != nil {
		t.Fatal(err)
	}
	for provider, id := range map[string]uuid.UUID{"email": emailID, "google": googleID} {
		if _, err := pool.Exec(ctx, "INSERT INTO account_identities(id, account_id, provider, external_id, normalized_id, verified_at,created_at,updated_at) VALUES($1,$2,$3,$4,$4,now(),now(),now())", id, accountID, provider, provider+"@example.test"); err != nil {
			t.Fatal(err)
		}
	}
	repo := postgres.NewAccountIdentityRepository(pool, postgres.WithUsableLoginProviders(domain.UsableLoginProviders{
		domain.IdentityProviderEmail:  true,
		domain.IdentityProviderGoogle: true,
	}))
	if err := repo.UnlinkIdentity(ctx, accountID, emailID); err != nil {
		t.Fatalf("enabled google should remain a usable login method: %v", err)
	}
}
