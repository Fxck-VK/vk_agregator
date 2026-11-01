package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"vk-ai-aggregator/internal/domain"
)

var _ domain.WebReferralEvidenceRepository = (*PaymentRepository)(nil)

func (r *PaymentRepository) RecordWebReferralPayment(ctx context.Context, e domain.WebReferralPaymentEvidence) error {
	if _, ok := r.db.(pgx.Tx); !ok {
		return fmt.Errorf("postgres: referral payment evidence requires a transaction")
	}
	if e.IntentID == uuid.Nil || e.AccountID == uuid.Nil || e.Provider != "yookassa" || strings.TrimSpace(e.ProviderPaymentID) == "" || e.Amount <= 0 || e.Currency != "rub" || e.VerifiedAt.IsZero() || (e.Mode != domain.WebReferralPaymentLive && e.Mode != domain.WebReferralPaymentTest && e.Mode != domain.WebReferralPaymentUnknown) {
		return domain.ErrInvalidInput
	}
	tag, err := r.db.Exec(ctx, `INSERT INTO web_referral_payment_evidence(intent_id,account_id,provider,provider_payment_id,amount,currency,mode,verified_at)
 SELECT id,account_id,provider,provider_payment_id,amount,currency,$7,$8 FROM payment_intents
 WHERE id=$1 AND account_id=$2 AND provider=$3 AND provider_payment_id=$4 AND amount=$5 AND currency=$6
 AND status IN ('succeeded','refunded','partially_refunded')
 ON CONFLICT(intent_id) DO NOTHING`, e.IntentID, e.AccountID, e.Provider, e.ProviderPaymentID, e.Amount, e.Currency, e.Mode, e.VerifiedAt)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	var ok bool
	err = r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM web_referral_payment_evidence WHERE intent_id=$1 AND account_id=$2 AND provider=$3 AND provider_payment_id=$4 AND amount=$5 AND currency=$6)`, e.IntentID, e.AccountID, e.Provider, e.ProviderPaymentID, e.Amount, e.Currency).Scan(&ok)
	if err != nil {
		return mapError(err)
	}
	if !ok {
		return domain.ErrConflict
	}
	return nil
}

func (r *PaymentRepository) RecordWebReferralRefund(ctx context.Context, e domain.WebReferralRefundEvidence) error {
	if _, ok := r.db.(pgx.Tx); !ok {
		return fmt.Errorf("postgres: referral refund evidence requires a transaction")
	}
	if e.IntentID == uuid.Nil || e.Provider != "yookassa" || e.ProviderPaymentID == "" || strings.TrimSpace(e.ProviderRefundID) == "" || e.Amount <= 0 || e.Currency != "rub" || e.VerifiedAt.IsZero() {
		return domain.ErrInvalidInput
	}
	// Serialize a refund with the processor's final proof check. A refund may
	// arrive before payment evidence, so absence of that row is permitted.
	rows, err := r.db.Query(ctx, `SELECT intent_id FROM web_referral_payment_evidence WHERE intent_id=$1 FOR UPDATE`, e.IntentID)
	if err != nil {
		return mapError(err)
	}
	for rows.Next() {
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return mapError(err)
	}
	tag, err := r.db.Exec(ctx, `INSERT INTO web_referral_refund_evidence(provider,provider_refund_id,intent_id,amount,verified_at)
 SELECT provider,$4,id,$5,$7 FROM payment_intents WHERE id=$1 AND provider=$2 AND provider_payment_id=$3 AND amount>=$5 AND currency=$6 AND account_id IS NOT NULL
 ON CONFLICT(provider,provider_refund_id) DO NOTHING`, e.IntentID, e.Provider, e.ProviderPaymentID, e.ProviderRefundID, e.Amount, e.Currency, e.VerifiedAt)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	var ok bool
	err = r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM web_referral_refund_evidence WHERE provider=$1 AND provider_refund_id=$2 AND intent_id=$3 AND amount=$4)`, e.Provider, e.ProviderRefundID, e.IntentID, e.Amount).Scan(&ok)
	if err != nil {
		return mapError(err)
	}
	if !ok {
		return domain.ErrConflict
	}
	return nil
}
