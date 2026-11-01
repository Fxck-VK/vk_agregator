package paymentservice

import (
	"context"
	"github.com/google/uuid"
	"time"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/promotionservice"
)

func (s *Service) QuotePromotion(ctx context.Context, owner uuid.UUID, code string) (*domain.PromotionQuote, error) {
	if !s.SubscriptionsAvailable() {
		return nil, domain.ErrPromotionUnavailable
	}
	var quote *domain.PromotionQuote
	err := s.cfg.SubscriptionTx.RunPaymentTx(ctx, func(ctx context.Context, payments domain.PaymentRepository, billing domain.BillingRepository) error {
		r, ok := payments.(domain.PromotionRepository)
		if !ok {
			return domain.ErrPromotionUnavailable
		}
		if err := r.LockPromotionAccount(ctx, owner); err != nil {
			return err
		}
		if err := checkPromotionGrantHistory(ctx, billing, owner); err != nil {
			return err
		}
		var e error
		quote, e = promotionservice.QuoteLocked(ctx, r, owner, code, time.Now().UTC())
		return e
	})
	return quote, err
}

func checkPromotionGrantHistory(ctx context.Context, billing domain.BillingRepository, owner uuid.UUID) error {
	if history, ok := billing.(domain.PromotionGrantHistory); ok {
		paid, err := history.HasPromotionPaidGrant(ctx, owner)
		if err != nil {
			return err
		}
		if paid {
			return domain.ErrConflict
		}
	}
	return nil
}

func finishPromotion(ctx context.Context, payments domain.PaymentRepository, intent *domain.PaymentIntent, paid bool) error {
	store, ok := payments.(domain.PromotionRepository)
	if !ok {
		return domain.ErrPromotionUnavailable
	}
	return store.FinishPromotionLocked(ctx, intent.AccountID, intent.ID, paid)
}
