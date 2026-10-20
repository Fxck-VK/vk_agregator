package accountservice_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/accountservice"
	"vk-ai-aggregator/internal/service/identityresolver"
)

func TestMissingOrOversizedVKNameDoesNotPreventSignIn(t *testing.T) {
	for _, name := range []string{"", " \t\n", strings.Repeat("Я", 161)} {
		ctx := context.Background()
		repo := memory.NewAccountIdentityRepo()
		auth := accountauth.New(identityresolver.New(memory.NewUserRepo(), repo, nil), accountauth.WithProfileRepository(repo))
		result, err := auth.ResolveOrCreate(ctx, domain.VerifiedAccountLogin{Method: domain.AccountLoginVKID, ExternalID: "12345", Verified: true, DisplayName: name})
		if err != nil || result.AccountID == uuid.Nil {
			t.Fatal("optional profile metadata prevented login", err)
		}
		stored, err := repo.GetAccountDisplayName(ctx, result.AccountID)
		if err != nil || stored != "" {
			t.Fatal("invalid name persisted", err)
		}
	}
}

func TestVerifiedVKNameSurvivesLoginAndProfileRead(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewAccountIdentityRepo()
	auth := accountauth.New(identityresolver.New(memory.NewUserRepo(), repo, nil), accountauth.WithProfileRepository(repo))
	svc := accountservice.New(repo, auth, accountservice.WithProfileRepository(repo))
	login := domain.VerifiedAccountLogin{Method: domain.AccountLoginVKID, ExternalID: "12345", Verified: true, DisplayName: "  Тестовый  Пользователь  "}
	result, err := auth.ResolveOrCreate(ctx, login)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := svc.Profile(ctx, result.AccountID)
	if err != nil || profile.DisplayName != "Тестовый Пользователь" {
		t.Fatal("VK name not persisted in account profile", err)
	}
	if profile.IdentityRefs[0].Label != "VK" {
		t.Fatal("login method label replaced by personal name")
	}
	login.DisplayName = "Changed Provider Name"
	if _, err := auth.ResolveOrCreate(ctx, login); err != nil {
		t.Fatal(err)
	}
	profile, err = svc.Profile(ctx, result.AccountID)
	if err != nil || profile.DisplayName != "Тестовый Пользователь" {
		t.Fatal("existing account name overwritten", err)
	}
}

func TestVKLinkFillsOnlyOwnedVerifiedProfile(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewAccountIdentityRepo()
	auth := accountauth.New(identityresolver.New(memory.NewUserRepo(), repo, nil), accountauth.WithProfileRepository(repo))
	svc := accountservice.New(repo, auth, accountservice.WithProfileRepository(repo))
	owner, err := auth.ResolveOrCreate(ctx, domain.VerifiedAccountLogin{Method: domain.AccountLoginEmailPassword, ExternalID: "synthetic@example.test", Verified: true, DisplayName: "Ignored email assertion name"})
	if err != nil {
		t.Fatal(err)
	}
	vk := domain.VerifiedAccountLogin{Method: domain.AccountLoginVKID, ExternalID: "98765", Verified: true, DisplayName: "Provider Name"}
	if _, err := svc.LinkVerifiedIdentity(ctx, uuid.New(), owner.AccountID, vk); !errors.Is(err, domain.ErrAccountIdentityOwnershipRequired) {
		t.Fatal("foreign actor accepted")
	}
	vk.Verified = false
	if _, err := svc.LinkVerifiedIdentity(ctx, owner.AccountID, owner.AccountID, vk); !errors.Is(err, domain.ErrUnverifiedLogin) {
		t.Fatal("unverified assertion accepted")
	}
	before, err := svc.Profile(ctx, owner.AccountID)
	if err != nil || before.DisplayName != "" {
		t.Fatal("unverified or non-VK name stored", err)
	}
	vk.Verified = true
	if _, err := svc.LinkVerifiedIdentity(ctx, owner.AccountID, owner.AccountID, vk); err != nil {
		t.Fatal(err)
	}
	after, err := svc.Profile(ctx, owner.AccountID)
	if err != nil || after.DisplayName != vk.DisplayName {
		t.Fatal("verified linked VK name missing", err)
	}
	foreign, err := auth.ResolveOrCreate(ctx, domain.VerifiedAccountLogin{Method: domain.AccountLoginEmailPassword, ExternalID: "foreign@example.test", Verified: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.LinkVerifiedIdentity(ctx, foreign.AccountID, foreign.AccountID, vk); !errors.Is(err, domain.ErrAccountMergeRequiresConfirmation) {
		t.Fatal("occupied VK identity reassigned")
	}
	unchanged, err := svc.Profile(ctx, foreign.AccountID)
	if err != nil || unchanged.DisplayName != "" {
		t.Fatal("failed link populated another account", err)
	}
}
