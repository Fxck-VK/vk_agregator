package memory

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

func TestConcurrentUnlinkPreservesLoginWhenPhoneBindingRemains(t *testing.T) {
	repo := NewAccountIdentityRepo()
	accountID := uuid.New()
	email, err := repo.LinkIdentity(context.Background(), accountID, domain.IdentityProviderEmail, "owner@example.test", "owner@example.test")
	if err != nil {
		t.Fatal(err)
	}
	google, err := repo.LinkIdentity(context.Background(), accountID, domain.IdentityProviderGoogle, "google-subject", "google-subject")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.LinkIdentity(context.Background(), accountID, domain.IdentityProviderPhone, "+79991234567", "+79991234567"); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	results := make(chan error, 2)
	for _, id := range []uuid.UUID{email.ID, google.ID} {
		group.Add(1)
		go func() { defer group.Done(); results <- repo.UnlinkIdentity(context.Background(), accountID, id) }()
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
		t.Fatalf("removed %d sign-in identities; expected to preserve one", winners)
	}
}

func TestUnlinkDoesNotCountDisabledGoogleAsUsableLogin(t *testing.T) {
	repo := NewAccountIdentityRepo()
	accountID := uuid.New()
	email, err := repo.LinkIdentity(context.Background(), accountID, domain.IdentityProviderEmail, "owner@example.test", "owner@example.test")
	if err != nil {
		t.Fatal(err)
	}
	google, err := repo.LinkIdentity(context.Background(), accountID, domain.IdentityProviderGoogle, "google-subject", "google-subject")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.UnlinkIdentity(context.Background(), accountID, email.ID); !errors.Is(err, domain.ErrAccountLastIdentity) {
		t.Fatalf("email unlink error = %v, want ErrAccountLastIdentity", err)
	}
	if err := repo.UnlinkIdentity(context.Background(), accountID, google.ID); err != nil {
		t.Fatalf("disabled google unlink: %v", err)
	}
	rows, err := repo.ListIdentitiesByAccount(context.Background(), accountID, 10, 0)
	if err != nil || len(rows) != 1 || rows[0].ID != email.ID {
		t.Fatalf("remaining identities = %+v, err=%v; want primary email retained", rows, err)
	}
}

func TestUnlinkCountsConfiguredGoogleAsUsableLogin(t *testing.T) {
	repo := NewAccountIdentityRepo(WithUsableLoginProviders(domain.UsableLoginProviders{
		domain.IdentityProviderEmail:  true,
		domain.IdentityProviderGoogle: true,
	}))
	accountID := uuid.New()
	email, err := repo.LinkIdentity(context.Background(), accountID, domain.IdentityProviderEmail, "owner@example.test", "owner@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.LinkIdentity(context.Background(), accountID, domain.IdentityProviderGoogle, "google-subject", "google-subject"); err != nil {
		t.Fatal(err)
	}
	if err := repo.UnlinkIdentity(context.Background(), accountID, email.ID); err != nil {
		t.Fatalf("enabled google should remain a usable login method: %v", err)
	}
}

func TestUnlinkDoesNotCountUnconfiguredAlternateProvidersAsUsableLogin(t *testing.T) {
	repo := NewAccountIdentityRepo(WithUsableLoginProviders(domain.UsableLoginProviders{
		domain.IdentityProviderEmail: true,
	}))
	accountID := uuid.New()
	email, err := repo.LinkIdentity(context.Background(), accountID, domain.IdentityProviderEmail, "owner@example.test", "owner@example.test")
	if err != nil {
		t.Fatal(err)
	}
	for _, provider := range []domain.IdentityProvider{domain.IdentityProviderApple, domain.IdentityProviderTelegram, domain.IdentityProviderVK} {
		if _, err := repo.LinkIdentity(context.Background(), accountID, provider, string(provider)+"-subject", string(provider)+"-subject"); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.UnlinkIdentity(context.Background(), accountID, email.ID); !errors.Is(err, domain.ErrAccountLastIdentity) {
		t.Fatalf("email unlink error = %v, want ErrAccountLastIdentity", err)
	}
}
