package memory

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"sort"
	"strings"
	"time"
	"vk-ai-aggregator/internal/domain"
)

type memoryPromotionUse struct {
	Owner    uuid.UUID
	Snapshot domain.PromotionSnapshot
	State    string
}
type memoryPromotionMutation struct {
	Fingerprint string
	Result      domain.Promotion
}

func (r *BillingRepo) HasPromotionPaidGrant(_ context.Context, owner uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, entry := range r.ledger {
		account := r.accounts[entry.AccountID]
		if (entry.OwnerAccountID == owner || account.OwnerAccountID == owner) && entry.Type == domain.LedgerTopup && entry.Status == domain.LedgerStatusCommitted && strings.HasPrefix(entry.IdempotencyKey, "topup:") {
			return true, nil
		}
	}
	return false, nil
}

func clonePromotion(p domain.Promotion) *domain.Promotion {
	b, _ := json.Marshal(p)
	var out domain.Promotion
	_ = json.Unmarshal(b, &out)
	return &out
}
func (r *PaymentRepo) LockPromotionAccount(_ context.Context, owner uuid.UUID) error {
	if owner == uuid.Nil {
		return domain.ErrConflict
	}
	return nil
}
func (r *PaymentRepo) PromotionEligibleLocked(_ context.Context, owner uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.promotionEligible(owner), nil
}
func (r *PaymentRepo) promotionEligible(owner uuid.UUID) bool {
	for _, p := range r.intentsByID {
		if p.AccountID == owner && (p.Status == domain.PaymentIntentSucceeded || p.Status == domain.PaymentIntentRefunded || p.Status == domain.PaymentIntentPartiallyRefunded || !p.Status.IsTerminal()) {
			return false
		}
	}
	for _, u := range r.promotionUses {
		if u.Owner == owner && u.State != "released" {
			return false
		}
	}
	return true
}
func (r *PaymentRepo) promotionCounts(p domain.Promotion) *domain.Promotion {
	p.ReservedCount = 0
	p.RedeemedCount = 0
	for _, u := range r.promotionUses {
		if u.Snapshot.PromotionID == p.ID {
			if u.State == "reserved" {
				p.ReservedCount++
			}
			if u.State == "consumed" {
				p.RedeemedCount++
			}
		}
	}
	return clonePromotion(p)
}
func (r *PaymentRepo) GetPromotionLocked(_ context.Context, code string) (*domain.Promotion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.promotions {
		if p.Code == code {
			return r.promotionCounts(p), nil
		}
	}
	return nil, domain.ErrNotFound
}
func (r *PaymentRepo) ListPromotions(_ context.Context, limit, offset int) ([]domain.Promotion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := []domain.Promotion{}
	for _, p := range r.promotions {
		items = append(items, *r.promotionCounts(p))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	if offset >= len(items) {
		return []domain.Promotion{}, nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], nil
}
func (r *PaymentRepo) ReservePromotionLocked(_ context.Context, owner, intent uuid.UUID, s domain.PromotionSnapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.promotionEligible(owner) {
		return domain.ErrConflict
	}
	p, ok := r.promotions[s.PromotionID]
	if !ok {
		return domain.ErrNotFound
	}
	count := r.promotionCounts(p)
	if p.Revision != s.Revision || p.MaxUses > 0 && count.ReservedCount+count.RedeemedCount >= p.MaxUses {
		return domain.ErrConflict
	}
	if r.promotionUses == nil {
		r.promotionUses = map[uuid.UUID]memoryPromotionUse{}
	}
	r.promotionUses[intent] = memoryPromotionUse{owner, s, "reserved"}
	return nil
}
func (r *PaymentRepo) FinishPromotionLocked(_ context.Context, owner, intent uuid.UUID, paid bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.promotionUses[intent]
	if !ok || u.Owner != owner {
		return domain.ErrConflict
	}
	if paid {
		u.State = "consumed"
	} else if u.State == "reserved" {
		u.State = "released"
	}
	r.promotionUses[intent] = u
	return nil
}
func (r *PaymentRepo) MutatePromotion(_ context.Context, actor string, key uuid.UUID, fp string, p *domain.Promotion) (*domain.Promotion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	request := actor + ":" + key.String()
	if old, ok := r.promotionMutations[request]; ok {
		if old.Fingerprint != fp {
			return nil, domain.ErrConflict
		}
		return clonePromotion(old.Result), nil
	}
	if r.promotions == nil {
		r.promotions = map[uuid.UUID]domain.Promotion{}
	}
	if r.promotionMutations == nil {
		r.promotionMutations = map[string]memoryPromotionMutation{}
	}
	next := clonePromotion(*p)
	now := time.Now().UTC()
	if p.ID == uuid.Nil {
		for _, old := range r.promotions {
			if old.Code == p.Code {
				return nil, domain.ErrConflict
			}
		}
		next.ID = uuid.New()
		next.Revision = 1
		next.CreatedAt = now
	} else {
		old, ok := r.promotions[p.ID]
		if !ok {
			return nil, domain.ErrNotFound
		}
		if old.Revision != p.Revision || old.Code != p.Code {
			return nil, domain.ErrConflict
		}
		count := r.promotionCounts(old)
		if p.MaxUses > 0 && p.MaxUses < count.ReservedCount+count.RedeemedCount {
			return nil, domain.ErrConflict
		}
		next.Revision++
		next.CreatedAt = old.CreatedAt
	}
	next.UpdatedAt = now
	r.promotions[next.ID] = *next
	next = r.promotionCounts(*next)
	r.promotionMutations[request] = memoryPromotionMutation{fp, *clonePromotion(*next)}
	return next, nil
}
