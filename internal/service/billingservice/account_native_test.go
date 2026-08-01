package billingservice_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
)

func TestAccountNativeEnsureCreatesCreditAccountWithoutLegacyUser(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewBillingRepo()
	svc := billingservice.New(repo)
	ownerAccountID := uuid.New()

	account, err := svc.EnsureAccountForAccount(ctx, repo, ownerAccountID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("ensure native account: %v", err)
	}
	if account.UserID != uuid.Nil {
		t.Fatalf("legacy user ID = %s, want nil", account.UserID)
	}
	if account.OwnerAccountID != ownerAccountID {
		t.Fatalf("owner account ID = %s, want %s", account.OwnerAccountID, ownerAccountID)
	}
	if got, err := repo.GetAccountByOwner(ctx, ownerAccountID, domain.CurrencyCredits); err != nil || got.ID != account.ID {
		t.Fatalf("canonical owner lookup = %#v, %v; want %s", got, err, account.ID)
	}
}

func TestAccountNativeEnsureReturnsOneAccountUnderConcurrentReplay(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewBillingRepo()
	svc := billingservice.New(repo)
	ownerAccountID := uuid.New()

	const callers = 16
	ids := make(chan uuid.UUID, callers)
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			account, err := svc.EnsureAccountForAccount(ctx, repo, ownerAccountID, domain.CurrencyCredits)
			if err == nil {
				ids <- account.ID
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)

	var want uuid.UUID
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent ensure: %v", err)
		}
	}
	for id := range ids {
		if want == uuid.Nil {
			want = id
		} else if id != want {
			t.Fatalf("concurrent ensure created different accounts: %s and %s", want, id)
		}
	}
}

func TestAccountNativeEnsureIsolatesOwnersAndCurrencies(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewBillingRepo()
	svc := billingservice.New(repo)
	ownerA, ownerB := uuid.New(), uuid.New()

	aCredits, err := svc.EnsureAccountForAccount(ctx, repo, ownerA, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("ensure owner A credits: %v", err)
	}
	aStars, err := svc.EnsureAccountForAccount(ctx, repo, ownerA, domain.CurrencyStars)
	if err != nil {
		t.Fatalf("ensure owner A stars: %v", err)
	}
	bCredits, err := svc.EnsureAccountForAccount(ctx, repo, ownerB, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("ensure owner B credits: %v", err)
	}
	if aCredits.ID == aStars.ID || aCredits.ID == bCredits.ID {
		t.Fatalf("canonical accounts must be distinct by owner and currency: %#v %#v %#v", aCredits, aStars, bCredits)
	}
	if _, err := repo.GetAccountByOwner(ctx, ownerB, domain.CurrencyStars); err != domain.ErrNotFound {
		t.Fatalf("cross-owner/currency lookup error = %v, want ErrNotFound", err)
	}
}

func TestAccountNativeEnsureRejectsNilOwner(t *testing.T) {
	repo := memory.NewBillingRepo()
	_, err := billingservice.New(repo).EnsureAccountForAccount(context.Background(), repo, uuid.Nil, domain.CurrencyCredits)
	if err == nil {
		t.Fatal("nil owner account ID was accepted")
	}
}
