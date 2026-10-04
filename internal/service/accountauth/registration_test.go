package accountauth_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/identityresolver"
)

func TestRegistrationCreatesVerifiedEmailAndHashedPasswordAtomically(t *testing.T) {
	ctx := context.Background()
	identities := memory.NewAccountIdentityRepo()
	security := memory.NewAccountSecurityRepo()
	auth := accountauth.New(identityresolver.New(nil, identities, nil), accountauth.WithCredentialRepository(security), accountauth.WithRegistrationRepository(memory.NewAccountRegistrationRepo(identities, security)))
	id := uuid.New()
	if _, err := auth.RegisterVerifiedEmailPassword(ctx, id, "New@Example.test", "weak"); !errors.Is(err, accountauth.ErrWeakPassword) {
		t.Fatal(err)
	}
	if _, err := identities.ResolveIdentity(ctx, domain.IdentityProviderEmail, "new@example.test"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("weak password created identity")
	}
	result, err := auth.RegisterVerifiedEmailPassword(ctx, id, "New@Example.test", "strong-password")
	if err != nil || result.AccountID != id {
		t.Fatalf("registration: %v", err)
	}
	credential, err := security.FindCredential(ctx, id, domain.AccountCredentialPassword)
	if err != nil || !strings.HasPrefix(credential.SecretHash, "$argon2id$") || strings.Contains(credential.SecretHash, "strong-password") {
		t.Fatal("password not hashed")
	}
	if _, err := auth.AuthenticateEmailPassword(ctx, "new@example.test", "strong-password"); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.RegisterVerifiedEmailPassword(ctx, id, "new@example.test", "other-password"); err != nil {
		t.Fatal("retry should succeed", err)
	}
	if _, err := auth.AuthenticateEmailPassword(ctx, "new@example.test", "strong-password"); err != nil {
		t.Fatal("retry replaced password")
	}
	if _, err := auth.RegisterVerifiedEmailPassword(ctx, uuid.New(), "new@example.test", "other-password"); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("existing email overwritten", err)
	}
}

func TestConcurrentRegistrationAllowsOnlyOneOwner(t *testing.T) {
	identities := memory.NewAccountIdentityRepo()
	security := memory.NewAccountSecurityRepo()
	auth := accountauth.New(identityresolver.New(nil, identities, nil), accountauth.WithRegistrationRepository(memory.NewAccountRegistrationRepo(identities, security)))
	var wg sync.WaitGroup
	outcomes := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := auth.RegisterVerifiedEmailPassword(context.Background(), uuid.New(), "new@example.test", "strong-password")
			outcomes <- err
		}()
	}
	wg.Wait()
	close(outcomes)
	successes := 0
	for err := range outcomes {
		if err == nil {
			successes++
		} else if !errors.Is(err, domain.ErrConflict) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("owners: %d", successes)
	}
}
