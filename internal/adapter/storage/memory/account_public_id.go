package memory

import (
	"context"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

var _ domain.AccountPublicIDRepository = (*AccountIdentityRepo)(nil)

// ensurePublicAccountLocked runs only after creation/link validation. Reads and
// repeat identity operations never allocate numbers.
func (r *AccountIdentityRepo) ensurePublicAccountLocked(id uuid.UUID, now time.Time) {
	if _, exists := r.publicNumbers[id]; exists {
		return
	}
	r.nextPublicNumber++
	r.publicNumbers[id] = r.nextPublicNumber
	r.publicAccounts[r.nextPublicNumber] = domain.Account{ID: id, Status: "active", Role: "user", AccountType: "personal", Locale: "ru", Timezone: "Europe/Moscow", CreatedAt: now, UpdatedAt: now}
}

func (r *AccountIdentityRepo) GetAccountPublicNumber(_ context.Context, id uuid.UUID) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	number, exists := r.publicNumbers[id]
	if !exists {
		return 0, domain.ErrNotFound
	}
	return number, nil
}

func (r *AccountIdentityRepo) GetAccountByPublicNumber(_ context.Context, number int64) (*domain.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	account, exists := r.publicAccounts[number]
	if !exists {
		return nil, domain.ErrNotFound
	}
	return &account, nil
}
