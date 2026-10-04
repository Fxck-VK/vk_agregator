package accountservice_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/accountservice"
	"vk-ai-aggregator/internal/service/identityresolver"
)

func TestProfileReportsPasswordStateWithoutCredentialData(t *testing.T) {
	ctx := context.Background()
	identities := memory.NewAccountIdentityRepo()
	auth := accountauth.New(identityresolver.New(nil, identities, nil), accountauth.WithCredentialRepository(memory.NewAccountSecurityRepo()))
	service := accountservice.New(identities, auth)
	owner, _ := auth.ResolveVerifiedEmailPassword(ctx, "member@example.test")
	profile, err := service.Profile(ctx, owner.AccountID)
	if err != nil || profile.PasswordSet == nil || *profile.PasswordSet {
		t.Fatal("first setup state not reported")
	}
	if err := auth.SetPasswordForVerifiedEmail(ctx, owner.AccountID, owner.AccountID, "member@example.test", "initial-password"); err != nil {
		t.Fatal(err)
	}
	profile, err = service.Profile(ctx, owner.AccountID)
	if err != nil || profile.PasswordSet == nil || !*profile.PasswordSet {
		t.Fatal("existing password state not reported")
	}
	payload, _ := json.Marshal(profile)
	for _, secret := range []string{"secret_hash", "initial-password", "$argon2id$", "member@example.test"} {
		if strings.Contains(string(payload), secret) {
			t.Fatal("profile exposes credential or raw identity data")
		}
	}
}
