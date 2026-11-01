// Package promotionservice owns first-purchase commercial policy. It makes no
// provider calls; reservation and consumption share existing payment transactions.
package promotionservice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	"sort"
	"time"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/economycatalog"
)

type TxRunner interface {
	RunPaymentTx(context.Context, func(context.Context, domain.PaymentRepository, domain.BillingRepository) error) error
}
type Service struct {
	tx    TxRunner
	store domain.PromotionRepository
}

func New(tx TxRunner, store domain.PromotionRepository) *Service {
	return &Service{tx: tx, store: store}
}

func Validate(p *domain.Promotion) error {
	code, err := domain.NormalizePromotionCode(p.Code)
	if err != nil {
		return err
	}
	p.Code = code
	if p.Percent < 1 || p.Percent > 99 || p.MaxUses < 0 || p.StartsAt.IsZero() || p.EndsAt != nil && !p.EndsAt.After(p.StartsAt) || len(p.PlanCodes) == 0 || len(p.PlanCodes) > 20 {
		return domain.ErrPromotionInvalid
	}
	codes := append([]string(nil), p.PlanCodes...)
	sort.Strings(codes)
	for i, c := range codes {
		if _, ok := economycatalog.FindPlan(c); !ok || i > 0 && codes[i-1] == c {
			return domain.ErrPromotionInvalid
		}
	}
	p.PlanCodes = codes
	p.StartsAt = p.StartsAt.UTC()
	if p.EndsAt != nil {
		v := p.EndsAt.UTC()
		p.EndsAt = &v
	}
	return nil
}
func (s *Service) List(ctx context.Context, limit, offset int) ([]domain.Promotion, error) {
	if s == nil || s.store == nil {
		return nil, domain.ErrPromotionUnavailable
	}
	if limit < 1 || limit > 101 || offset < 0 {
		return nil, domain.ErrPromotionInvalid
	}
	return s.store.ListPromotions(ctx, limit, offset)
}

// Mutate binds canonical configuration to the actor and durable UUID request key.
func (s *Service) Mutate(ctx context.Context, actor string, key uuid.UUID, p *domain.Promotion) (*domain.Promotion, error) {
	if s == nil || s.tx == nil {
		return nil, domain.ErrPromotionUnavailable
	}
	if p == nil || actor == "" || len(actor) > 128 || key == uuid.Nil {
		return nil, domain.ErrPromotionInvalid
	}
	if err := Validate(p); err != nil {
		return nil, err
	}
	if p.ID == uuid.Nil && p.Revision != 0 || p.ID != uuid.Nil && p.Revision < 1 {
		return nil, domain.ErrPromotionInvalid
	}
	raw, _ := json.Marshal(p)
	hash := sha256.Sum256(raw)
	fingerprint := hex.EncodeToString(hash[:])
	var out *domain.Promotion
	err := s.tx.RunPaymentTx(ctx, func(ctx context.Context, payments domain.PaymentRepository, _ domain.BillingRepository) error {
		r, ok := payments.(domain.PromotionRepository)
		if !ok {
			return domain.ErrPromotionUnavailable
		}
		var e error
		out, e = r.MutatePromotion(ctx, actor, key, fingerprint, p)
		return e
	})
	return out, err
}

func QuoteLocked(ctx context.Context, r domain.PromotionRepository, owner uuid.UUID, code string, now time.Time) (*domain.PromotionQuote, error) {
	if r == nil {
		return nil, domain.ErrPromotionUnavailable
	}
	code, err := domain.NormalizePromotionCode(code)
	if err != nil {
		return nil, err
	}
	if owner == uuid.Nil {
		return nil, domain.ErrPromotionInvalid
	}
	if err = r.LockPromotionAccount(ctx, owner); err != nil {
		return nil, err
	}
	eligible, err := r.PromotionEligibleLocked(ctx, owner)
	if err != nil {
		return nil, err
	}
	if !eligible {
		return nil, domain.ErrConflict
	}
	p, err := r.GetPromotionLocked(ctx, code)
	if err == domain.ErrNotFound {
		return nil, domain.ErrConflict
	}
	if err != nil {
		return nil, err
	}
	if !p.Enabled || now.Before(p.StartsAt) || p.EndsAt != nil && !now.Before(*p.EndsAt) || p.MaxUses > 0 && p.ReservedCount+p.RedeemedCount >= p.MaxUses {
		return nil, domain.ErrConflict
	}
	q := &domain.PromotionQuote{Code: p.Code, Revision: p.Revision, Percent: p.Percent, Plans: []domain.PromotionQuotePlan{}}
	for _, c := range p.PlanCodes {
		plan, ok := economycatalog.FindPlan(c)
		if !ok {
			return nil, domain.ErrPromotionUnavailable
		}
		amount, e := domain.PromotionAmount(plan.Amount, p.Percent)
		if e != nil {
			return nil, e
		}
		q.Plans = append(q.Plans, domain.PromotionQuotePlan{PlanCode: c, BaseAmount: plan.Amount, Amount: amount, Currency: "RUB", Stars: plan.Stars})
	}
	return q, nil
}

func ReserveLocked(ctx context.Context, r domain.PromotionRepository, owner, intent uuid.UUID, code string, revision int64, planCode string, now time.Time) (*domain.PromotionSnapshot, error) {
	q, err := QuoteLocked(ctx, r, owner, code, now)
	if err != nil {
		return nil, err
	}
	if revision != q.Revision {
		return nil, domain.ErrConflict
	}
	for _, plan := range q.Plans {
		if plan.PlanCode == planCode {
			p, err := r.GetPromotionLocked(ctx, q.Code)
			if err != nil {
				return nil, err
			}
			snap := &domain.PromotionSnapshot{PromotionID: p.ID, Code: q.Code, Revision: q.Revision, Percent: q.Percent, BaseAmount: plan.BaseAmount, Amount: plan.Amount}
			if err = r.ReservePromotionLocked(ctx, owner, intent, *snap); err != nil {
				return nil, err
			}
			return snap, nil
		}
	}
	return nil, domain.ErrConflict
}
