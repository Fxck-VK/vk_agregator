package memory

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"testing"
	"time"
	"vk-ai-aggregator/internal/domain"
)

func TestSecurityNoticesCaptureVerifiedEmailMutations(t *testing.T) {
	ctx := context.Background()
	repo := NewAccountIdentityRepo()
	owner := uuid.New()
	primary, err := repo.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, "primary@example.test", "primary@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.SecurityNotices()) != 0 {
		t.Fatal("initial binding sent notice")
	}
	backup, err := repo.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, "backup@example.test", "backup@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.SecurityNotices()) != 2 {
		t.Fatal("add did not notify verified recipients")
	}
	replacement, err := repo.ReplaceBackupEmailIdentity(ctx, owner, backup.ID, "new@example.test", "new@example.test", backup.UpdatedAt)
	if err != nil {
		t.Fatal(err)
	}
	notices := repo.SecurityNotices()
	if len(notices) != 5 {
		t.Fatal("replace did not include old and new recipients")
	}
	if _, err := repo.ReplaceBackupEmailIdentity(ctx, owner, backup.ID, "stale@example.test", "stale@example.test", backup.UpdatedAt); !errors.Is(err, domain.ErrAccountBackupEmailChanged) {
		t.Fatal("stale replacement allowed")
	}
	if len(repo.SecurityNotices()) != 5 {
		t.Fatal("failed mutation queued notice")
	}
	if err := repo.UnlinkIdentity(ctx, owner, replacement.ID); err != nil {
		t.Fatal(err)
	}
	if len(repo.SecurityNotices()) != 7 {
		t.Fatal("remove did not notify old and remaining recipients")
	}
	if err := repo.UnlinkIdentity(ctx, owner, primary.ID); !errors.Is(err, domain.ErrAccountLastIdentity) {
		t.Fatal("last identity removed")
	}
	if len(repo.SecurityNotices()) != 7 {
		t.Fatal("failed unlink queued notice")
	}
	at := time.Now().Add(time.Second)
	leased, err := repo.LeaseAccountSecurityNotices(ctx, at, 100, time.Minute)
	if err != nil || len(leased) != 7 {
		t.Fatal("notice lease failed")
	}
	other, err := repo.LeaseAccountSecurityNotices(ctx, at, 100, time.Minute)
	if err != nil || len(other) != 0 {
		t.Fatal("active notices leased twice")
	}
}
