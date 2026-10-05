package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// AccountIdentityRepo is an in-memory implementation used by unit tests and
// local mock runs. Production uses the PostgreSQL repository.
type AccountIdentityRepo struct {
	mu     sync.Mutex
	byKey  map[string]*domain.AccountIdentity
	byID   map[uuid.UUID]*domain.AccountIdentity
	audits []domain.AccountLinkAuditEntry
}

func NewAccountIdentityRepo() *AccountIdentityRepo {
	return &AccountIdentityRepo{
		byKey: map[string]*domain.AccountIdentity{},
		byID:  map[uuid.UUID]*domain.AccountIdentity{},
	}
}

var _ domain.AccountIdentityRepository = (*AccountIdentityRepo)(nil)

func (r *AccountIdentityRepo) ResolveIdentity(_ context.Context, provider domain.IdentityProvider, normalizedID string) (*domain.AccountIdentity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	identity, ok := r.byKey[identityKey(provider, normalizedID)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return cloneAccountIdentity(identity), nil
}

func (r *AccountIdentityRepo) EnsureIdentityForUser(_ context.Context, user *domain.User, provider domain.IdentityProvider, externalID, normalizedID string) (*domain.AccountIdentity, error) {
	if user == nil || user.ID == uuid.Nil {
		return nil, domain.ErrInvalidIdentity
	}
	accountID := user.EffectiveAccountID()
	if accountID == uuid.Nil {
		accountID = user.ID
	}
	identity, err := r.linkIdentity(accountID, provider, externalID, normalizedID)
	if err != nil {
		return nil, err
	}
	user.AccountID = identity.AccountID
	return identity, nil
}

func (r *AccountIdentityRepo) CreateAccountWithIdentity(_ context.Context, provider domain.IdentityProvider, externalID, normalizedID string) (*domain.AccountIdentity, error) {
	return r.linkIdentity(uuid.New(), provider, externalID, normalizedID)
}

func (r *AccountIdentityRepo) ListIdentitiesByAccount(_ context.Context, accountID uuid.UUID, limit, offset int) ([]*domain.AccountIdentity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if accountID == uuid.Nil {
		return nil, domain.ErrInvalidIdentity
	}
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	matched := make([]*domain.AccountIdentity, 0)
	for _, identity := range r.byID {
		if identity.AccountID == accountID {
			matched = append(matched, cloneAccountIdentity(identity))
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		if matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].ID.String() > matched[j].ID.String()
		}
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})
	var out []*domain.AccountIdentity
	for i := offset; i < len(matched) && len(out) < limit; i++ {
		out = append(out, matched[i])
	}
	return out, nil
}

func (r *AccountIdentityRepo) LinkIdentity(_ context.Context, accountID uuid.UUID, provider domain.IdentityProvider, externalID, normalizedID string) (*domain.AccountIdentity, error) {
	if accountID == uuid.Nil {
		return nil, domain.ErrInvalidIdentity
	}
	return r.linkIdentity(accountID, provider, externalID, normalizedID)
}

func (r *AccountIdentityRepo) UnlinkIdentity(_ context.Context, accountID, identityID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	identity, ok := r.byID[identityID]
	if !ok {
		return domain.ErrNotFound
	}
	if identity.AccountID != accountID {
		return domain.ErrConflict
	}
	linkedCount := 0
	remainingLogin := 0
	for _, row := range r.byID {
		if row.AccountID == accountID {
			linkedCount++
			if row.ID != identityID && row.Provider != domain.IdentityProviderPhone && !row.VerifiedAt.IsZero() {
				remainingLogin++
			}
		}
	}
	if linkedCount <= 1 || (identity.Provider != domain.IdentityProviderPhone && remainingLogin == 0) {
		return domain.ErrAccountLastIdentity
	}
	delete(r.byID, identityID)
	delete(r.byKey, identityKey(identity.Provider, identity.NormalizedID))
	r.appendAuditLocked(identity, domain.AccountLinkActionUnlinked)
	return nil
}

func (r *AccountIdentityRepo) linkIdentity(accountID uuid.UUID, provider domain.IdentityProvider, externalID, normalizedID string) (*domain.AccountIdentity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := identityKey(provider, normalizedID)
	if existing, ok := r.byKey[key]; ok {
		if existing.AccountID != accountID {
			return nil, domain.ErrConflict
		}
		return cloneAccountIdentity(existing), nil
	}
	now := time.Now()
	if provider == domain.IdentityProviderEmail {
		count := 0
		for _, row := range r.byID {
			if row.AccountID == accountID && row.Provider == provider {
				count++
				if !now.After(row.CreatedAt) {
					now = row.CreatedAt.Add(time.Nanosecond)
				}
			}
		}
		if count >= domain.MaxAccountEmails {
			return nil, domain.ErrAccountEmailLimit
		}
	}
	identity := &domain.AccountIdentity{
		ID:           uuid.New(),
		AccountID:    accountID,
		Provider:     provider,
		ExternalID:   externalID,
		NormalizedID: normalizedID,
		VerifiedAt:   now,
		LastUsedAt:   now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	r.byKey[key] = identity
	r.byID[identity.ID] = identity
	r.appendAuditLocked(identity, domain.AccountLinkActionLinked)
	return cloneAccountIdentity(identity), nil
}

// AuditEntries returns a copy of PII-free identity link audit rows for tests.
func (r *AccountIdentityRepo) AuditEntries() []domain.AccountLinkAuditEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.AccountLinkAuditEntry, len(r.audits))
	copy(out, r.audits)
	return out
}

func (r *AccountIdentityRepo) appendAuditLocked(identity *domain.AccountIdentity, action domain.AccountLinkAction) {
	if identity == nil {
		return
	}
	identityID := identity.ID
	r.audits = append(r.audits, domain.AccountLinkAuditEntry{
		ID:         uuid.New(),
		AccountID:  identity.AccountID,
		Action:     action,
		Provider:   identity.Provider,
		IdentityID: &identityID,
		CreatedAt:  time.Now(),
	})
}

func identityKey(provider domain.IdentityProvider, normalizedID string) string {
	return string(domain.NormalizeIdentityProvider(provider)) + ":" + normalizedID
}

func cloneAccountIdentity(identity *domain.AccountIdentity) *domain.AccountIdentity {
	if identity == nil {
		return nil
	}
	cp := *identity
	return &cp
}

func (r *AccountIdentityRepo) ReplaceBackupEmailIdentity(_ context.Context, accountID, identityID uuid.UUID, externalID, normalizedID string, expected time.Time) (*domain.AccountIdentity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rows := make([]*domain.AccountIdentity, 0)
	for _, row := range r.byID {
		if row.AccountID == accountID {
			rows = append(rows, row)
		}
	}
	identity := r.byID[identityID]
	if identity == nil || identity.AccountID != accountID || expected.IsZero() || !identity.UpdatedAt.Equal(expected) || domain.AccountEmailRoles(rows)[identityID] != domain.AccountEmailBackup {
		return nil, domain.ErrAccountBackupEmailChanged
	}
	key := identityKey(domain.IdentityProviderEmail, normalizedID)
	if existing := r.byKey[key]; existing != nil {
		if existing.AccountID == accountID {
			return nil, domain.ErrAccountEmailAlreadyLinked
		}
		return nil, domain.ErrConflict
	}
	r.appendAuditLocked(identity, domain.AccountLinkActionUnlinked)
	delete(r.byKey, identityKey(identity.Provider, identity.NormalizedID))
	now := time.Now()
	if !now.After(identity.UpdatedAt) {
		now = identity.UpdatedAt.Add(time.Nanosecond)
	}
	identity.ExternalID = externalID
	identity.NormalizedID = normalizedID
	identity.VerifiedAt = now
	identity.LastUsedAt = now
	identity.UpdatedAt = now
	r.byKey[key] = identity
	r.appendAuditLocked(identity, domain.AccountLinkActionLinked)
	return cloneAccountIdentity(identity), nil
}

// WithVerifiedEmailLock serializes mock recovery with identity mutations.
func (r *AccountIdentityRepo) WithVerifiedEmailLock(ctx context.Context, accountID uuid.UUID, email string, action func(context.Context) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	identity := r.byKey[identityKey(domain.IdentityProviderEmail, email)]
	if identity == nil {
		return domain.ErrNotFound
	}
	if identity.AccountID != accountID {
		return domain.ErrAccountIdentityOwnershipRequired
	}
	if identity.VerifiedAt.IsZero() {
		return domain.ErrUnverifiedLogin
	}
	return action(ctx)
}
