package accountservice_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/accountservice"
	"vk-ai-aggregator/internal/service/identityresolver"
)

func TestAccountServiceProfileAndLinkingExposeOnlySafeIdentities(t *testing.T) {
	ctx := context.Background()
	identities := memory.NewAccountIdentityRepo()
	resolver := identityresolver.New(memory.NewUserRepo(), identities, nil)
	auth := accountauth.New(resolver)
	service := accountservice.New(identities, auth)

	vk, err := auth.ResolveVKID(ctx, 123456789)
	if err != nil {
		t.Fatalf("resolve vk account: %v", err)
	}
	email, err := service.LinkVerifiedIdentity(ctx, vk.AccountID, vk.AccountID, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginEmailPassword,
		ExternalID: "Owner.Name@Example.COM",
		Verified:   true,
	})
	if err != nil {
		t.Fatalf("link email: %v", err)
	}
	if email.Label != "o***@example.com" {
		t.Fatalf("email label = %q, want masked email", email.Label)
	}
	phone, err := service.LinkVerifiedIdentity(ctx, vk.AccountID, vk.AccountID, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginPhoneOTP,
		ExternalID: "+7 (999) 123-45-67",
		Verified:   true,
	})
	if err != nil {
		t.Fatalf("link phone: %v", err)
	}
	if !strings.HasSuffix(phone.Label, "4567") || strings.Contains(phone.Label, "999") || strings.Contains(phone.Label, "123") {
		t.Fatalf("phone label is not safely masked: %q", phone.Label)
	}

	profile, err := service.Profile(ctx, vk.AccountID)
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if profile.AccountID != vk.AccountID {
		t.Fatalf("profile account = %s, want %s", profile.AccountID, vk.AccountID)
	}
	if len(profile.IdentityRefs) != 3 {
		t.Fatalf("identity count = %d, want 3: %+v", len(profile.IdentityRefs), profile.IdentityRefs)
	}
	encoded, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("marshal profile: %v", err)
	}
	for _, forbidden := range []string{
		"Owner.Name",
		"owner.name@example.com",
		"123456789",
		"(999) 123",
		"123-45",
		"+79991234567",
	} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("safe profile leaked %q: %s", forbidden, encoded)
		}
	}

	if err := service.UnlinkIdentity(ctx, vk.AccountID, vk.AccountID, phone.ID); err != nil {
		t.Fatalf("unlink phone: %v", err)
	}
	afterUnlink, err := service.ListIdentities(ctx, vk.AccountID, 10, 0)
	if err != nil {
		t.Fatalf("list after unlink: %v", err)
	}
	if len(afterUnlink) != 2 {
		t.Fatalf("identity count after unlink = %d, want 2", len(afterUnlink))
	}
}
