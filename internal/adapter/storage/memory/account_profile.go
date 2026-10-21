package memory

import (
	"context"
	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

var _ domain.AccountProfileRepository = (*AccountIdentityRepo)(nil)

func (r *AccountIdentityRepo) GetAccountDisplayName(_ context.Context, accountID uuid.UUID) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if accountID == uuid.Nil {
		return "", domain.ErrInvalidIdentity
	}
	for _, identity := range r.byID {
		if identity.AccountID == accountID {
			return r.displayNames[accountID], nil
		}
	}
	return "", domain.ErrNotFound
}

func (r *AccountIdentityRepo) SetAccountDisplayNameIfEmpty(_ context.Context, accountID, identityID uuid.UUID, name string) error {
	name = domain.NormalizeAccountDisplayName(name)
	if accountID == uuid.Nil || identityID == uuid.Nil || name == "" {
		return domain.ErrInvalidIdentity
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	identity, ok := r.byID[identityID]
	if !ok || identity.AccountID != accountID ||
		(identity.Provider != domain.IdentityProviderVK && identity.Provider != domain.IdentityProviderYandex) || identity.VerifiedAt.IsZero() {
		return domain.ErrUnverifiedLogin
	}
	if r.displayNames[accountID] == "" {
		r.displayNames[accountID] = name
	}
	return nil
}
