package accountservice_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/accountservice"
	"vk-ai-aggregator/internal/service/identityresolver"
)

func TestYandexNameSurvivesLoginAndProfileRead(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewAccountIdentityRepo()
	auth := accountauth.New(identityresolver.New(memory.NewUserRepo(), repo, nil), accountauth.WithProfileRepository(repo))
	svc := accountservice.New(repo, auth, accountservice.WithProfileRepository(repo))
	login := domain.VerifiedAccountLogin{Method: domain.AccountLoginYandex, ExternalID: "Stable-Subject", Verified: true}
	owner, err := auth.ResolveOrCreate(ctx, login)
	if err != nil {
		t.Fatal(err)
	}
	// Accounts created before this change acquire the name on their next login.
	login.DisplayName = "  Тестовый  Пользователь  "
	again, err := auth.ResolveOrCreate(ctx, login)
	if err != nil || again.AccountID != owner.AccountID {
		t.Fatal("name changed account ownership", err)
	}
	profile, err := svc.Profile(ctx, owner.AccountID)
	if err != nil || profile.DisplayName != "Тестовый Пользователь" || profile.IdentityRefs[0].Label != "Yandex" {
		t.Fatal("Yandex name missing from account profile", err)
	}
	login.DisplayName = "Changed Provider Name"
	if _, err := auth.ResolveOrCreate(ctx, login); err != nil {
		t.Fatal(err)
	}
	if profile, err := svc.Profile(ctx, owner.AccountID); err != nil || profile.DisplayName != "Тестовый Пользователь" {
		t.Fatal("existing account name overwritten", err)
	}
}

func TestYandexLinkNameRequiresOwnedVerifiedIdentity(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewAccountIdentityRepo()
	auth := accountauth.New(identityresolver.New(memory.NewUserRepo(), repo, nil), accountauth.WithProfileRepository(repo))
	owner, err := auth.ResolveVerifiedEmailPassword(ctx, "synthetic@example.test")
	if err != nil {
		t.Fatal(err)
	}
	login := domain.VerifiedAccountLogin{Method: domain.AccountLoginYandex, ExternalID: "Linked-Subject", Verified: true, DisplayName: "Provider Name"}
	if _, err := auth.LinkVerifiedIdentity(ctx, uuid.New(), owner.AccountID, login); !errors.Is(err, domain.ErrAccountIdentityOwnershipRequired) {
		t.Fatal("foreign actor accepted")
	}
	login.Verified = false
	if _, err := auth.LinkVerifiedIdentity(ctx, owner.AccountID, owner.AccountID, login); !errors.Is(err, domain.ErrUnverifiedLogin) {
		t.Fatal("unverified assertion accepted")
	}
	if name, err := repo.GetAccountDisplayName(ctx, owner.AccountID); err != nil || name != "" {
		t.Fatal("failed link populated a name", err)
	}
	login.Verified = true
	if _, err := auth.LinkVerifiedIdentity(ctx, owner.AccountID, owner.AccountID, login); err != nil {
		t.Fatal(err)
	}
	if name, err := repo.GetAccountDisplayName(ctx, owner.AccountID); err != nil || name != login.DisplayName {
		t.Fatal("verified linked Yandex name missing", err)
	}
	foreign, err := auth.ResolveVerifiedEmailPassword(ctx, "foreign@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.LinkVerifiedIdentity(ctx, foreign.AccountID, foreign.AccountID, login); !errors.Is(err, domain.ErrAccountMergeRequiresConfirmation) {
		t.Fatal("occupied identity reassigned")
	}
	if name, err := repo.GetAccountDisplayName(ctx, foreign.AccountID); err != nil || name != "" {
		t.Fatal("conflicting link populated another account", err)
	}
}
