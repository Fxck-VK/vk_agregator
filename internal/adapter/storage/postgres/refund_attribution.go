package postgres

import (
	"context"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

var _ domain.SubscriptionRefundReversalReader = (*BillingRepository)(nil)

// IsUnusedSubscriptionRefundReversal never infers provenance from a caller's
// entry or its reason text. All links are checked against stored rows in the
// refund transaction, whose canonical owner lock serializes billing writers.
func (r *BillingRepository) IsUnusedSubscriptionRefundReversal(ctx context.Context, creditAccountID, entryID uuid.UUID) (bool, error) {
	if creditAccountID == uuid.Nil || entryID == uuid.Nil {
		return false, nil
	}
	const query = `SELECT EXISTS (
	 SELECT 1
	 FROM ledger_entries reversal
	 JOIN credit_accounts account ON account.id=reversal.account_id
	 JOIN credit_lots lot ON lot.account_id=account.id
	  AND reversal.idempotency_key='credit-lot-expiry:' || lot.id::text
	 JOIN ledger_entries source_grant ON source_grant.id=lot.source_entry_id
	  AND source_grant.account_id=account.id AND lot.id=source_grant.id
	 JOIN payment_intents purchase ON purchase.account_id=account.owner_account_id
	  AND source_grant.idempotency_key='topup:' || purchase.provider || ':' || purchase.provider_payment_id
	 WHERE reversal.id=$2 AND account.id=$1 AND account.currency='credits'
	  AND account.owner_account_id IS NOT NULL
	  AND reversal.owner_account_id=account.owner_account_id
	  AND source_grant.owner_account_id=account.owner_account_id
	  AND reversal.type='adjustment' AND reversal.status='committed'
	  AND reversal.job_id IS NULL AND reversal.reservation_id IS NULL
	  AND reversal.credit_denomination_version=$3
	  AND source_grant.type='topup' AND source_grant.status='committed'
	  AND source_grant.job_id IS NULL AND source_grant.reservation_id IS NULL
	  AND source_grant.credit_denomination_version=$3 AND source_grant.amount>0
	  AND reversal.amount::numeric=-source_grant.amount::numeric
	  AND source_grant.lot_kind='subscription' AND source_grant.lot_expires_at IS NOT NULL
	  AND lot.kind='subscription' AND lot.available_credits=0 AND lot.reserved_credits=0
	  AND lot.expires_at IS NOT NULL AND lot.expires_at<=now()
	  AND purchase.provider='yookassa' AND purchase.provider_payment_id<>''
	  AND purchase.status='refunded' AND purchase.currency='rub'
	  AND purchase.credit_denomination_version=$3 AND purchase.credits=source_grant.amount
	  AND jsonb_typeof(purchase.metadata->'subscription')='object'
	  AND purchase.metadata->'subscription'->'plan'->'stars'=to_jsonb(purchase.credits)
	  AND purchase.metadata->'subscription'->'plan'->'amount'=to_jsonb(purchase.amount)
	  AND EXISTS (
	   SELECT 1 FROM payment_events event
	   WHERE event.provider=purchase.provider AND event.provider_payment_id=purchase.provider_payment_id
	    AND event.event_type='refund.succeeded' AND event.processed_at IS NOT NULL
	    AND event.provider_refund_id IS NOT NULL AND event.provider_refund_id<>''
	  )
	)`
	var proven bool
	if err := r.q().QueryRow(ctx, query, creditAccountID, entryID, domain.CurrentCreditDenominationVersion).Scan(&proven); err != nil {
		return false, mapError(err)
	}
	return proven, nil
}
