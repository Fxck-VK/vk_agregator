package identityresolver_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/identityresolver"
)

func TestResolveOrCreateVKCreatesLegacyUserIdentityAndBillingOnce(t *testing.T) {
	ctx := context.Background()
	users := memory.NewUserRepo()
	identities := newFakeIdentityRepo()
	billing := &fakeBillingEnsurer{}
	resolver := identityresolver.New(users, identities, billing)

	first, err := resolver.ResolveOrCreate(ctx, domain.IdentityProviderVK, "+777")
	if err != nil {
		t.Fatalf("resolve first: %v", err)
	}
	if first.AccountID == uuid.Nil || first.Identity == nil || first.User == nil {
		t.Fatalf("incomplete first resolution: %+v", first)
	}
	if first.User.AccountID != first.AccountID || first.User.EffectiveAccountID() != first.AccountID {
		t.Fatalf("legacy user account bridge = %s/effective %s, want %s", first.User.AccountID, first.User.EffectiveAccountID(), first.AccountID)
	}
	if first.Identity.Provider != domain.IdentityProviderVK || first.Identity.NormalizedID != "777" {
		t.Fatalf("unexpected identity: %+v", first.Identity)
	}
	if billing.calls != 1 {
		t.Fatalf("billing calls after create = %d, want 1", billing.calls)
	}
	if billing.lastUserID != first.User.ID || billing.lastAccountID != first.AccountID {
		t.Fatalf("billing ensured owner = user %s account %s, want user %s account %s",
			billing.lastUserID, billing.lastAccountID, first.User.ID, first.AccountID)
	}

	second, err := resolver.ResolveOrCreate(ctx, domain.IdentityProviderVK, "777")
	if err != nil {
		t.Fatalf("resolve second: %v", err)
	}
	if second.AccountID != first.AccountID || second.User.ID != first.User.ID || second.Identity.ID != first.Identity.ID {
		t.Fatalf("resolve is not stable: first=%+v second=%+v", first, second)
	}
	if second.User.AccountID != first.AccountID {
		t.Fatalf("second user account bridge = %s, want %s", second.User.AccountID, first.AccountID)
	}
	if billing.calls != 1 {
		t.Fatalf("billing calls after repeated resolve = %d, want 1", billing.calls)
	}
}

func TestLinkIdentityRejectsAlreadyLinkedIdentity(t *testing.T) {
	ctx := context.Background()
	identities := newFakeIdentityRepo()
	resolver := identityresolver.New(memory.NewUserRepo(), identities, nil)

	accountID := uuid.New()
	if _, err := resolver.LinkIdentity(ctx, accountID, domain.IdentityProviderEmail, "User@Example.COM"); err != nil {
		t.Fatalf("link first: %v", err)
	}
	if got, err := resolver.Resolve(ctx, domain.IdentityProviderEmail, "user@example.com"); err != nil || got != accountID {
		t.Fatalf("resolve linked identity = %s, %v", got, err)
	}
	if _, err := resolver.LinkIdentity(ctx, uuid.New(), domain.IdentityProviderEmail, "user@example.com"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("link to another account error = %v, want conflict", err)
	}
}

type fakeBillingEnsurer struct {
	calls         int
	lastUserID    uuid.UUID
	lastAccountID uuid.UUID
}

func (f *fakeBillingEnsurer) EnsureAccountForOwner(_ context.Context, userID, accountID uuid.UUID) (*domain.CreditAccount, error) {
	f.calls++
	f.lastUserID = userID
	f.lastAccountID = accountID
	return &domain.CreditAccount{ID: uuid.New(), UserID: userID, OwnerAccountID: accountID}, nil
}

type fakeIdentityRepo struct {
	byKey map[string]*domain.AccountIdentity
}

func newFakeIdentityRepo() *fakeIdentityRepo {
	return &fakeIdentityRepo{byKey: map[string]*domain.AccountIdentity{}}
}

func (r *fakeIdentityRepo) ResolveIdentity(_ context.Context, provider domain.IdentityProvider, normalizedID string) (*domain.AccountIdentity, error) {
	identity, ok := r.byKey[key(provider, normalizedID)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *identity
	return &cp, nil
}

func (r *fakeIdentityRepo) EnsureIdentityForUser(_ context.Context, user *domain.User, provider domain.IdentityProvider, externalID, normalizedID string) (*domain.AccountIdentity, error) {
	k := key(provider, normalizedID)
	if identity, ok := r.byKey[k]; ok {
		cp := *identity
		return &cp, nil
	}
	identity := &domain.AccountIdentity{
		ID:           uuid.New(),
		AccountID:    uuid.New(),
		Provider:     provider,
		ExternalID:   externalID,
		NormalizedID: normalizedID,
	}
	user.AccountID = identity.AccountID
	r.byKey[k] = identity
	cp := *identity
	return &cp, nil
}

func (r *fakeIdentityRepo) CreateAccountWithIdentity(_ context.Context, provider domain.IdentityProvider, externalID, normalizedID string) (*domain.AccountIdentity, error) {
	identity := &domain.AccountIdentity{
		ID:           uuid.New(),
		AccountID:    uuid.New(),
		Provider:     provider,
		ExternalID:   externalID,
		NormalizedID: normalizedID,
	}
	r.byKey[key(provider, normalizedID)] = identity
	cp := *identity
	return &cp, nil
}

func (r *fakeIdentityRepo) ListIdentitiesByAccount(_ context.Context, accountID uuid.UUID, limit, offset int) ([]*domain.AccountIdentity, error) {
	var out []*domain.AccountIdentity
	for _, identity := range r.byKey {
		if identity.AccountID == accountID {
			cp := *identity
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *fakeIdentityRepo) LinkIdentity(_ context.Context, accountID uuid.UUID, provider domain.IdentityProvider, externalID, normalizedID string) (*domain.AccountIdentity, error) {
	k := key(provider, normalizedID)
	if identity, ok := r.byKey[k]; ok {
		if identity.AccountID == accountID {
			cp := *identity
			return &cp, nil
		}
		return nil, domain.ErrConflict
	}
	identity := &domain.AccountIdentity{
		ID:           uuid.New(),
		AccountID:    accountID,
		Provider:     provider,
		ExternalID:   externalID,
		NormalizedID: normalizedID,
	}
	r.byKey[k] = identity
	cp := *identity
	return &cp, nil
}

func (r *fakeIdentityRepo) UnlinkIdentity(_ context.Context, accountID, identityID uuid.UUID) error {
	for k, identity := range r.byKey {
		if identity.AccountID == accountID && identity.ID == identityID {
			delete(r.byKey, k)
			return nil
		}
	}
	return domain.ErrNotFound
}

func key(provider domain.IdentityProvider, normalizedID string) string {
	return string(provider) + ":" + normalizedID
}
