package memory

import (
	"context"
	"vk-ai-aggregator/internal/domain"
)

func (r *AccountIdentityRepo) WithPasswordEmailLock(ctx context.Context, binding domain.AccountIdentity, action func(context.Context) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	current := r.byKey[identityKey(domain.IdentityProviderEmail, binding.NormalizedID)]
	if current == nil {
		return domain.ErrNotFound
	}
	if current.AccountID != binding.AccountID || current.ID != binding.ID || !current.UpdatedAt.Equal(binding.UpdatedAt) {
		return domain.ErrConflict
	}
	if current.VerifiedAt.IsZero() {
		return domain.ErrUnverifiedLogin
	}
	return action(context.WithValue(ctx, noticeIdentityLockKey{}, r))
}
