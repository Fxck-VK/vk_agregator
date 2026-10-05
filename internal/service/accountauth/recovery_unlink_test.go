package accountauth_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/identityresolver"
)

type recoveryUnlinkLimiter struct{ remove func() }

func (l recoveryUnlinkLimiter) Allow(context.Context, string) (bool, error) {
	l.remove()
	return true, nil
}

func TestRecoveryRejectsEmailRemovedAfterInitialOwnershipCheck(t *testing.T) {
	ctx := context.Background()
	identities := memory.NewAccountIdentityRepo()
	credentials := memory.NewAccountSecurityRepo()
	owner := uuid.New()
	_, _ = identities.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, "primary@example.test", "primary@example.test")
	backup, _ := identities.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, "backup@example.test", "backup@example.test")
	original := domain.AccountCredential{ID: uuid.New(), AccountID: owner, CredentialType: domain.AccountCredentialPassword, SecretHash: "synthetic-original-verifier"}
	if _, err := credentials.UpsertCredential(ctx, original); err != nil {
		t.Fatal(err)
	}
	limiter := recoveryUnlinkLimiter{remove: func() {
		if err := identities.UnlinkIdentity(ctx, owner, backup.ID); err != nil {
			t.Fatal(err)
		}
	}}
	auth := accountauth.New(identityresolver.New(memory.NewUserRepo(), identities, nil), accountauth.WithCredentialRepository(credentials), accountauth.WithLimiter(limiter))
	if err := auth.ResetPasswordForVerifiedEmail(ctx, owner, "backup@example.test", "replacement-password"); err == nil {
		t.Fatal("removed backup recovered password")
	}
	stored, err := credentials.FindCredential(ctx, owner, domain.AccountCredentialPassword)
	if err != nil || stored.SecretHash != original.SecretHash {
		t.Fatal("removed backup changed credential")
	}
}
