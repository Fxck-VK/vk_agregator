package memory

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"time"
	"vk-ai-aggregator/internal/domain"
)

func copySubscription(s domain.AccountSubscription) *domain.AccountSubscription {
	b, _ := json.Marshal(s)
	var out domain.AccountSubscription
	_ = json.Unmarshal(b, &out)
	return &out
}
func (r *BillingRepo) GetSubscription(_ context.Context, id uuid.UUID) (*domain.AccountSubscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.subscriptions[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return copySubscription(s), nil
}
func (r *BillingRepo) LockSubscription(ctx context.Context, id uuid.UUID) (*domain.AccountSubscription, error) {
	if id == uuid.Nil {
		return nil, domain.ErrConflict
	}
	s, err := r.GetSubscription(ctx, id)
	if err == domain.ErrNotFound {
		return &domain.AccountSubscription{AccountID: id}, nil
	}
	return s, err
}
func (r *BillingRepo) SaveSubscription(_ context.Context, s *domain.AccountSubscription) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s == nil || s.AccountID == uuid.Nil {
		return domain.ErrConflict
	}
	if r.subscriptions == nil {
		r.subscriptions = map[uuid.UUID]domain.AccountSubscription{}
	}
	old := r.subscriptions[s.AccountID]
	if old.Revision != s.Revision {
		return domain.ErrConflict
	}
	s.Revision++
	r.subscriptions[s.AccountID] = *copySubscription(*s)
	return nil
}
func (r *BillingRepo) ListDueSubscriptions(_ context.Context, now time.Time, limit int) ([]uuid.UUID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit < 1 || limit > 100 {
		limit = 100
	}
	ids := []uuid.UUID{}
	for id, s := range r.subscriptions {
		if s.AutoRenew && !s.PaidUntil.IsZero() && !s.PaidUntil.After(now) && !s.NextAttemptAt.After(now) {
			ids = append(ids, id)
			if len(ids) == limit {
				break
			}
		}
	}
	return ids, nil
}
