package domain

import (
	"context"

	"github.com/google/uuid"
)

// SubscriptionRefundReversalReader is an optional, fail-closed billing proof.
// True proves that the stored committed adjustment reverses the entire unused
// subscription grant of this credit account, with exact lot attribution and a
// provider-verified, processed full-refund event. Key prefixes and balances
// alone are insufficient. The refund caller must hold the canonical owner lock
// and use its transaction-bound repository for the proof and subsequent debit.
type SubscriptionRefundReversalReader interface {
	IsUnusedSubscriptionRefundReversal(ctx context.Context, creditAccountID, entryID uuid.UUID) (bool, error)
}
