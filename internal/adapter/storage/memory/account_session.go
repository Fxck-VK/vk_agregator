package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// AccountSessionRepo is an in-memory account session repository for tests and
// local mock runs.
type AccountSessionRepo struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]*domain.AccountSession
	byHash map[string]uuid.UUID
}

func NewAccountSessionRepo() *AccountSessionRepo {
	return &AccountSessionRepo{
		byID:   map[uuid.UUID]*domain.AccountSession{},
		byHash: map[string]uuid.UUID{},
	}
}

var _ domain.AccountSessionRepository = (*AccountSessionRepo)(nil)

func (r *AccountSessionRepo) CreateSession(_ context.Context, session domain.AccountSession) (*domain.AccountSession, error) {
	if err := session.Validate(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if session.ID == uuid.Nil {
		session.ID = uuid.New()
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now().UTC()
	}
	if session.UpdatedAt.IsZero() {
		session.UpdatedAt = session.CreatedAt
	}
	if _, exists := r.byHash[session.RefreshTokenHash]; exists {
		return nil, domain.ErrConflict
	}
	cp := session
	r.byID[session.ID] = &cp
	r.byHash[session.RefreshTokenHash] = session.ID
	return cloneAccountSession(&cp), nil
}

func (r *AccountSessionRepo) FindSessionByRefreshHash(_ context.Context, refreshTokenHash string) (*domain.AccountSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byHash[refreshTokenHash]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return cloneAccountSession(r.byID[id]), nil
}

func (r *AccountSessionRepo) ListActiveSessionsByAccount(_ context.Context, accountID uuid.UUID, now time.Time, limit int) ([]*domain.AccountSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if accountID == uuid.Nil {
		return nil, domain.ErrInvalidIdentity
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	matched := make([]*domain.AccountSession, 0)
	for _, session := range r.byID {
		if session.AccountID == accountID && session.RevokedAt == nil && session.ExpiresAt.After(now) {
			matched = append(matched, cloneAccountSession(session))
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		if matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].ID.String() > matched[j].ID.String()
		}
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})
	if len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, nil
}

func (r *AccountSessionRepo) RevokeSession(_ context.Context, accountID, sessionID uuid.UUID, revokedAt time.Time) (*domain.AccountSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.byID[sessionID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	if session.AccountID != accountID {
		return nil, domain.ErrNotFound
	}
	r.revokeLocked(session, revokedAt)
	return cloneAccountSession(session), nil
}

func (r *AccountSessionRepo) RevokeSessionByRefreshHash(_ context.Context, refreshTokenHash string, revokedAt time.Time) (*domain.AccountSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byHash[refreshTokenHash]
	if !ok {
		return nil, domain.ErrNotFound
	}
	session := r.byID[id]
	r.revokeLocked(session, revokedAt)
	return cloneAccountSession(session), nil
}

func (r *AccountSessionRepo) RevokeAllSessions(_ context.Context, accountID uuid.UUID, revokedAt time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var count int64
	for _, session := range r.byID {
		if session.AccountID == accountID && session.RevokedAt == nil {
			r.revokeLocked(session, revokedAt)
			count++
		}
	}
	return count, nil
}

func (r *AccountSessionRepo) revokeLocked(session *domain.AccountSession, revokedAt time.Time) {
	if session == nil {
		return
	}
	t := revokedAt.UTC()
	session.RevokedAt = &t
	session.UpdatedAt = t
}

func cloneAccountSession(session *domain.AccountSession) *domain.AccountSession {
	if session == nil {
		return nil
	}
	cp := *session
	if session.IdentityID != nil {
		id := *session.IdentityID
		cp.IdentityID = &id
	}
	if session.RevokedAt != nil {
		t := *session.RevokedAt
		cp.RevokedAt = &t
	}
	return &cp
}
