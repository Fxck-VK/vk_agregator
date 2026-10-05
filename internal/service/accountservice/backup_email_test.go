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

func TestEmailRolesAreGlobalAndReplacementRequiresOwnerAndVerifiedProof(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewAccountIdentityRepo()
	auth := accountauth.New(identityresolver.New(memory.NewUserRepo(), repo, nil))
	svc := accountservice.New(repo, auth)
	owner := uuid.New()
	primary, _ := repo.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, "primary@example.test", "primary@example.test")
	backup, _ := repo.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, "backup@example.test", "backup@example.test")
	page, err := svc.ListIdentities(ctx, owner, 1, 0)
	if err != nil || len(page) != 1 || page[0].EmailRole != domain.AccountEmailBackup {
		t.Fatal("pagination changed role")
	}
	login := domain.VerifiedAccountLogin{Method: domain.AccountLoginEmailPassword, ExternalID: "new@example.test", Verified: true}
	if _, err := svc.ReplaceVerifiedBackupEmail(ctx, uuid.New(), owner, backup.ID, login, backup.UpdatedAt); !errors.Is(err, domain.ErrAccountIdentityOwnershipRequired) {
		t.Fatal("foreign actor accepted")
	}
	login.Verified = false
	if _, err := svc.ReplaceVerifiedBackupEmail(ctx, owner, owner, backup.ID, login, backup.UpdatedAt); !errors.Is(err, domain.ErrUnverifiedLogin) {
		t.Fatal("unverified assertion accepted")
	}
	if _, err := svc.BackupEmailVersion(ctx, owner, primary.ID); !errors.Is(err, domain.ErrAccountBackupEmailChanged) {
		t.Fatal("primary proof requested")
	}
	if err := repo.UnlinkIdentity(ctx, owner, primary.ID); err != nil {
		t.Fatal(err)
	}
	profile, _ := svc.Profile(ctx, owner)
	if len(profile.IdentityRefs) != 1 || profile.IdentityRefs[0].EmailRole != domain.AccountEmailPrimary {
		t.Fatal("remaining email not promoted")
	}
	login.Verified = true
	if _, err := svc.ReplaceVerifiedBackupEmail(ctx, owner, owner, backup.ID, login, backup.UpdatedAt); !errors.Is(err, domain.ErrAccountBackupEmailChanged) {
		t.Fatal("former backup replaced after promotion")
	}
	if err := repo.UnlinkIdentity(ctx, owner, backup.ID); !errors.Is(err, domain.ErrAccountLastIdentity) {
		t.Fatal("last login removed")
	}
}
