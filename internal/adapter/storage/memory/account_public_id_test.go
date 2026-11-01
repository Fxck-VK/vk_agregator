package memory_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
)

func TestAccountPublicIDConcurrentAssignmentAndReadOnlyLookup(t *testing.T) {
	r := memory.NewAccountIdentityRepo(memory.WithUsableLoginProviders(domain.UsableLoginProviders{domain.IdentityProviderEmail: true, domain.IdentityProviderYandex: true}))
	ctx := context.Background()
	if _, err := r.GetAccountPublicNumber(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("unknown account must not allocate a number", err)
	}
	const count = 32
	type result struct {
		id     uuid.UUID
		number int64
		err    error
	}
	results := make(chan result, count)
	var group sync.WaitGroup
	for i := 0; i < count; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			email := fmt.Sprintf("fixture-%d@example.test", i)
			identity, err := r.CreateAccountWithIdentity(ctx, domain.IdentityProviderEmail, email, email)
			if err != nil {
				results <- result{err: err}
				return
			}
			number, err := r.GetAccountPublicNumber(ctx, identity.AccountID)
			results <- result{identity.AccountID, number, err}
		}(i)
	}
	group.Wait()
	close(results)
	seen := map[int64]bool{}
	for got := range results {
		if got.err != nil || got.number <= 0 || seen[got.number] {
			t.Fatalf("invalid/duplicate number: %+v", got)
		}
		seen[got.number] = true
		account, err := r.GetAccountByPublicNumber(ctx, got.number)
		if err != nil || account.ID != got.id {
			t.Fatal("lookup did not resolve the same account", err)
		}
	}
	if len(seen) != count {
		t.Fatal("incomplete assignment")
	}
	if _, err := r.GetAccountByPublicNumber(ctx, 999999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("unknown number resolved", err)
	}
}

func TestAccountPublicIDSurvivesLinkedIdentitiesAndRegistrationRetry(t *testing.T) {
	ctx := context.Background()
	r := memory.NewAccountIdentityRepo(memory.WithUsableLoginProviders(domain.UsableLoginProviders{domain.IdentityProviderEmail: true, domain.IdentityProviderYandex: true}))
	security := memory.NewAccountSecurityRepo()
	registration := memory.NewAccountRegistrationRepo(r, security)
	owner := uuid.New()
	now := time.Now()
	first, err := registration.RegisterEmailAccount(ctx, owner, "primary@example.test", "synthetic-verifier", now)
	if err != nil {
		t.Fatal(err)
	}
	number, err := r.GetAccountPublicNumber(ctx, owner)
	if err != nil || number <= 0 {
		t.Fatal("signup has no number", err)
	}
	if _, err = registration.RegisterEmailAccount(ctx, owner, "primary@example.test", "synthetic-verifier", now); err != nil {
		t.Fatal(err)
	}
	if _, err = r.LinkIdentity(ctx, owner, domain.IdentityProviderYandex, "yandex-subject", "yandex-subject"); err != nil {
		t.Fatal(err)
	}
	if err = r.UnlinkIdentity(ctx, owner, first.Identity.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := r.GetAccountPublicNumber(ctx, owner); err != nil || got != number {
		t.Fatal("identity change renumbered the account", err)
	}
}
