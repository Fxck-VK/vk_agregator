package accountlink_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/accountlink"
	"vk-ai-aggregator/internal/service/accountservice"
	"vk-ai-aggregator/internal/service/identityresolver"
)

func TestReplacementExpiryAndExistingAddressPreserveOldBackup(t *testing.T) {
	ctx := context.Background()
	_, sender, repo, owner := newLinkFixture(t)
	_, _ = repo.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, "primary@example.test", "primary@example.test")
	backup, _ := repo.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, "old@example.test", "old@example.test")
	now := time.Now()
	clock := func() time.Time { return now }
	store := accountlink.NewMemoryStore()
	store.SetNow(clock)
	account := accountservice.New(repo, accountauth.New(identityresolver.New(memory.NewUserRepo(), repo, nil)))
	linker, err := accountlink.New(store, sender, account, accountlink.Config{CodeTTL: time.Second, HashSecret: "test-secret", Now: clock})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := linker.RequestEmailCode(ctx, owner, "new@example.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := linker.VerifyBackupEmailCode(ctx, owner, backup.ID, "new@example.test", sender.emailCode); err == nil {
		t.Fatal("ordinary link proof accepted for replacement")
	}
	if _, err := linker.RequestBackupEmailCode(ctx, owner, backup.ID, "old@example.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := linker.VerifyBackupEmailCode(ctx, owner, backup.ID, "old@example.test", sender.emailCode); !errors.Is(err, domain.ErrAccountEmailAlreadyLinked) {
		t.Fatalf("same address: %v", err)
	}
	if _, err := linker.RequestBackupEmailCode(ctx, owner, backup.ID, "new@example.test"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	if _, err := linker.VerifyBackupEmailCode(ctx, owner, backup.ID, "new@example.test", sender.emailCode); !errors.Is(err, accountlink.ErrExpiredCode) {
		t.Fatalf("expired proof: %v", err)
	}
	old, err := repo.ResolveIdentity(ctx, domain.IdentityProviderEmail, "old@example.test")
	if err != nil || old.ID != backup.ID {
		t.Fatal("invalid/expired proof changed backup")
	}
}

func TestBackupEmailReplacementKeepsOldUntilProofAndIsPurposeBound(t *testing.T) {
	ctx := context.Background()
	linker, sender, repo, owner := newLinkFixture(t)
	primary, _ := repo.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, "primary@example.test", "primary@example.test")
	backup, _ := repo.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, "old@example.test", "old@example.test")
	if _, err := linker.RequestBackupEmailCode(ctx, owner, primary.ID, "new@example.test"); !errors.Is(err, domain.ErrAccountBackupEmailChanged) {
		t.Fatalf("primary replacement: %v", err)
	}
	if _, err := linker.RequestBackupEmailCode(ctx, owner, backup.ID, "new@example.test"); err != nil {
		t.Fatal(err)
	}
	code := sender.emailCode
	if _, err := repo.ResolveIdentity(ctx, domain.IdentityProviderEmail, "old@example.test"); err != nil {
		t.Fatal("old backup removed before proof")
	}
	if err := linker.VerifyEmailRecoveryCode(ctx, owner, "new@example.test", code); err == nil {
		t.Fatal("replacement proof accepted for recovery")
	}
	if _, err := linker.VerifyEmailCode(ctx, owner, "new@example.test", code); err == nil {
		t.Fatal("replacement proof accepted for ordinary link")
	}
	if _, err := linker.VerifyBackupEmailCode(ctx, owner, uuid.New(), "new@example.test", code); err == nil {
		t.Fatal("proof accepted for another binding")
	}
	if _, err := linker.VerifyBackupEmailCode(ctx, uuid.New(), backup.ID, "new@example.test", code); err == nil {
		t.Fatal("proof accepted for another account")
	}
	if _, err := linker.VerifyBackupEmailCode(ctx, owner, backup.ID, "new@example.test", "invalid"); err == nil {
		t.Fatal("bad proof accepted")
	}
	linked, err := linker.VerifyBackupEmailCode(ctx, owner, backup.ID, "new@example.test", code)
	if err != nil || linked.ID != backup.ID || linked.EmailRole != domain.AccountEmailBackup {
		t.Fatalf("replacement: %+v %v", linked, err)
	}
	if _, err := repo.ResolveIdentity(ctx, domain.IdentityProviderEmail, "old@example.test"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("old binding retained after success")
	}
	if _, err := linker.VerifyBackupEmailCode(ctx, owner, backup.ID, "new@example.test", code); err == nil {
		t.Fatal("proof replay accepted")
	}
	rows, _ := repo.ListIdentitiesByAccount(ctx, owner, 100, 0)
	roles := domain.AccountEmailRoles(rows)
	if roles[primary.ID] != domain.AccountEmailPrimary || roles[backup.ID] != domain.AccountEmailBackup {
		t.Fatal("replacement changed roles")
	}
	audits := repo.AuditEntries()
	if len(audits) != 5 || audits[3].Action != domain.AccountLinkActionUnlinked || audits[4].Action != domain.AccountLinkActionLinked {
		t.Fatal("replacement audit missing")
	}
}

func TestConcurrentReplacementProofsCannotOverwriteWinner(t *testing.T) {
	ctx := context.Background()
	linker, sender, repo, owner := newLinkFixture(t)
	_, _ = repo.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, "primary@example.test", "primary@example.test")
	backup, _ := repo.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, "old@example.test", "old@example.test")
	emails := []string{"first@example.test", "second@example.test"}
	codes := make([]string, 2)
	for i, email := range emails {
		if _, err := linker.RequestBackupEmailCode(ctx, owner, backup.ID, email); err != nil {
			t.Fatal(err)
		}
		codes[i] = sender.emailCode
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := range emails {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := linker.VerifyBackupEmailCode(ctx, owner, backup.ID, emails[i], codes[i])
			results <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	successes, stale := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, domain.ErrAccountBackupEmailChanged) {
			stale++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || stale != 1 {
		t.Fatalf("successes=%d stale=%d", successes, stale)
	}
}

func TestReplacementConflictAndRemovedBackupKeepBindingsSafe(t *testing.T) {
	ctx := context.Background()
	linker, sender, repo, owner := newLinkFixture(t)
	_, _ = repo.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, "primary@example.test", "primary@example.test")
	backup, _ := repo.LinkIdentity(ctx, owner, domain.IdentityProviderEmail, "old@example.test", "old@example.test")
	foreign, _ := repo.CreateAccountWithIdentity(ctx, domain.IdentityProviderEmail, "occupied@example.test", "occupied@example.test")
	if _, err := linker.RequestBackupEmailCode(ctx, owner, backup.ID, "occupied@example.test"); err != nil {
		t.Fatal("request leaked address conflict")
	}
	if _, err := linker.VerifyBackupEmailCode(ctx, owner, backup.ID, "occupied@example.test", sender.emailCode); !errors.Is(err, domain.ErrAccountMergeRequiresConfirmation) {
		t.Fatalf("foreign conflict: %v", err)
	}
	old, _ := repo.ResolveIdentity(ctx, domain.IdentityProviderEmail, "old@example.test")
	occupied, _ := repo.ResolveIdentity(ctx, domain.IdentityProviderEmail, "occupied@example.test")
	if old == nil || occupied.AccountID != foreign.AccountID {
		t.Fatal("conflict changed bindings")
	}
	if _, err := linker.RequestBackupEmailCode(ctx, owner, backup.ID, "new@example.test"); err != nil {
		t.Fatal(err)
	}
	if err := repo.UnlinkIdentity(ctx, owner, backup.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := linker.VerifyBackupEmailCode(ctx, owner, backup.ID, "new@example.test", sender.emailCode); !errors.Is(err, domain.ErrAccountBackupEmailChanged) {
		t.Fatalf("removed binding recreated: %v", err)
	}
}
