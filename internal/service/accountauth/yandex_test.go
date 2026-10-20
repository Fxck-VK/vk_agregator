package accountauth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/identityresolver"
)

func TestYandexIdentityLoginLinkAndOwnership(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewAccountIdentityRepo()
	svc := accountauth.New(identityresolver.New(memory.NewUserRepo(), repo, nil))
	login := domain.VerifiedAccountLogin{Method: "yandex", ExternalID: "Stable-Subject", Verified: true}
	owner, err := svc.ResolveOrCreate(ctx, login)
	if err != nil {
		t.Fatal(err)
	}
	again, err := svc.ResolveOrCreate(ctx, login)
	if err != nil || owner.AccountID != again.AccountID || owner.Identity.NormalizedID != "Stable-Subject" {
		t.Fatal("Yandex login did not reuse its account", err)
	}
	if _, err := svc.LinkVerifiedIdentity(ctx, uuid.New(), owner.AccountID, login); !errors.Is(err, domain.ErrAccountIdentityOwnershipRequired) {
		t.Fatal("foreign actor accepted")
	}
	other, err := svc.ResolveVerifiedEmailPassword(ctx, "synthetic@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.LinkVerifiedIdentity(ctx, other.AccountID, other.AccountID, login); !errors.Is(err, domain.ErrAccountMergeRequiresConfirmation) {
		t.Fatal("implicit account merge accepted", err)
	}
	login.ExternalID = "Other-Subject"
	if _, err := svc.LinkVerifiedIdentity(ctx, other.AccountID, other.AccountID, login); err != nil {
		t.Fatal(err)
	}
	linked, err := svc.ResolveOrCreate(ctx, login)
	if err != nil || linked.AccountID != other.AccountID {
		t.Fatal("linked subject logged into another account", err)
	}
	login.Verified = false
	if _, err := svc.ResolveOrCreate(ctx, login); !errors.Is(err, domain.ErrUnverifiedLogin) {
		t.Fatal("unverified Yandex login accepted")
	}
}
