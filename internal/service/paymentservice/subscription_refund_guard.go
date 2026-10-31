package paymentservice

import (
	"context"
	"github.com/google/uuid"
	"time"
	"vk-ai-aggregator/internal/domain"
)

type refundLotReader interface {
	CreditLotBalancesForOwner(context.Context, uuid.UUID, time.Time) ([]domain.CreditLotBalance, error)
}

// The legacy refund does not record debit allocations for compensation. Until
// that contract exists it must not consume any expiring lot via generic FEFO.
func requireNonExpiringRefundBalance(ctx context.Context, repo domain.BillingRepository, creditAccount uuid.UUID) error {
	reader, ok := repo.(refundLotReader)
	if !ok {
		return nil
	}
	account, err := repo.GetAccount(ctx, creditAccount)
	if err != nil {
		return err
	}
	if account.OwnerAccountID == uuid.Nil {
		return nil
	}
	lots, err := reader.CreditLotBalancesForOwner(ctx, account.OwnerAccountID, time.Now().UTC())
	if err != nil {
		return err
	}
	for _, lot := range lots {
		if lot.ExpiresAt != nil && (lot.AvailableCredits > 0 || lot.ReservedCredits > 0) {
			return ErrRefundNotAllowed
		}
	}
	return nil
}

// Serialize the guard+debit with incoming subscription grants. Otherwise a new
// expiring grant could race the guard and be consumed by the generic debit.
func lockTopupRefundOwner(ctx context.Context, repo domain.BillingRepository, intent *domain.PaymentIntent) error {
	store, ok := repo.(domain.SubscriptionRepository)
	if !ok {
		return nil
	}
	account, err := repo.GetAccountByUser(ctx, intentOwnerID(intent), domain.CurrencyCredits)
	if err != nil {
		return err
	}
	if account.OwnerAccountID == uuid.Nil {
		return nil
	}
	_, err = store.LockSubscription(ctx, account.OwnerAccountID)
	return err
}
