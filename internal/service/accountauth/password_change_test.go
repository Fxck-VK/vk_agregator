package accountauth

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/identityresolver"
)

type competingCredentialStore struct {
	domain.AccountCredentialRepository
	beforeWrite func()
}

func (r *competingCredentialStore) ExecutePasswordSecurity(ctx context.Context, op domain.PasswordSecurityOperation, deps domain.PasswordSecurityDependencies) error {
	r.beforeWrite()
	return r.AccountCredentialRepository.(domain.AccountPasswordSecurityRepository).ExecutePasswordSecurity(ctx, op, deps)
}

func (r *competingCredentialStore) UpsertCredential(ctx context.Context, value domain.AccountCredential) (*domain.AccountCredential, error) {
	r.beforeWrite()
	return r.AccountCredentialRepository.UpsertCredential(ctx, value)
}
func (r *competingCredentialStore) CompareAndSwapCredential(ctx context.Context, value domain.AccountCredential, expected string) (*domain.AccountCredential, error) {
	r.beforeWrite()
	return r.AccountCredentialRepository.CompareAndSwapCredential(ctx, value, expected)
}

func TestLegacyLoginCannotOverwriteConcurrentPasswordChange(t *testing.T) {
	ctx := context.Background()
	base := memory.NewAccountSecurityRepo()
	resolver := identityresolver.New(nil, memory.NewAccountIdentityRepo(), nil)
	auth := New(resolver, WithCredentialRepository(base))
	owner, err := auth.ResolveVerifiedEmailPassword(ctx, "member@example.test")
	if err != nil {
		t.Fatal(err)
	}
	credential := domain.AccountCredential{ID: uuid.New(), AccountID: owner.AccountID, CredentialType: domain.AccountCredentialPassword, SecretHash: legacyPBKDF2Hash("original-password")}
	if _, err := base.UpsertCredential(ctx, credential); err != nil {
		t.Fatal(err)
	}
	replacement, _ := hashPassword("replacement-password")
	store := &competingCredentialStore{AccountCredentialRepository: base, beforeWrite: func() {
		credential.SecretHash = replacement
		if _, err := base.UpsertCredential(ctx, credential); err != nil {
			t.Fatal(err)
		}
	}}
	auth = New(resolver, WithCredentialRepository(store))
	if _, err := auth.AuthenticateEmailPassword(ctx, "member@example.test", "original-password"); !errors.Is(err, ErrInvalidPasswordLogin) {
		t.Fatalf("stale password login accepted: %v", err)
	}
	stored, _ := base.FindCredential(ctx, owner.AccountID, domain.AccountCredentialPassword)
	if stored.SecretHash != replacement {
		t.Fatal("legacy rehash replaced a concurrently changed password")
	}
}

func TestPasswordChangeRejectsStaleProofAndWrongOwner(t *testing.T) {
	ctx := context.Background()
	base := memory.NewAccountSecurityRepo()
	resolver := identityresolver.New(nil, memory.NewAccountIdentityRepo(), nil)
	auth := New(resolver, WithCredentialRepository(base))
	owner, _ := auth.ResolveVerifiedEmailPassword(ctx, "member@example.test")
	if err := auth.SetPasswordForVerifiedEmail(ctx, owner.AccountID, owner.AccountID, "member@example.test", "original-password"); err != nil {
		t.Fatal(err)
	}
	if err := auth.ChangePasswordForVerifiedEmail(ctx, uuid.New(), owner.AccountID, "member@example.test", "original-password", "replacement-password"); !errors.Is(err, domain.ErrAccountIdentityOwnershipRequired) {
		t.Fatal("wrong owner accepted")
	}
	if err := auth.ChangePasswordForVerifiedEmail(ctx, owner.AccountID, owner.AccountID, "unlinked@example.test", "original-password", "replacement-password"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("unlinked email accepted")
	}
	if err := auth.ChangePasswordForVerifiedEmail(ctx, owner.AccountID, owner.AccountID, "member@example.test", "original-password", "short"); !errors.Is(err, ErrWeakPassword) {
		t.Fatal("weak replacement accepted")
	}
	stored, _ := base.FindCredential(ctx, owner.AccountID, domain.AccountCredentialPassword)
	changedHash, _ := hashPassword("concurrent-password")
	store := &competingCredentialStore{AccountCredentialRepository: base, beforeWrite: func() {
		stored.SecretHash = changedHash
		if _, err := base.UpsertCredential(ctx, *stored); err != nil {
			t.Fatal(err)
		}
	}}
	auth = New(resolver, WithCredentialRepository(store))
	if err := auth.ChangePasswordForVerifiedEmail(ctx, owner.AccountID, owner.AccountID, "member@example.test", "original-password", "replacement-password"); !errors.Is(err, ErrPasswordConfirmationRequired) {
		t.Fatal("stale proof accepted")
	}
	after, _ := base.FindCredential(ctx, owner.AccountID, domain.AccountCredentialPassword)
	if after.SecretHash != changedHash {
		t.Fatal("concurrent password overwritten")
	}
}

func TestConcurrentPasswordSetupAllowsOnlyOneWriter(t *testing.T) {
	ctx := context.Background()
	auth := New(identityresolver.New(nil, memory.NewAccountIdentityRepo(), nil), WithCredentialRepository(memory.NewAccountSecurityRepo()))
	owner, _ := auth.ResolveVerifiedEmailPassword(ctx, "member@example.test")
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, password := range []string{"first-password", "second-password"} {
		wg.Add(1)
		go func(password string) {
			defer wg.Done()
			errs <- auth.SetPasswordForVerifiedEmail(ctx, owner.AccountID, owner.AccountID, "member@example.test", password)
		}(password)
	}
	wg.Wait()
	close(errs)
	success := 0
	for err := range errs {
		if err == nil {
			success++
		} else if !errors.Is(err, ErrPasswordConfirmationRequired) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("setup writers succeeded: %d", success)
	}
}
