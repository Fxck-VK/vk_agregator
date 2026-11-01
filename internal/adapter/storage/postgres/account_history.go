package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

type AccountHistoryRepository struct{ db Querier }

var _ domain.AccountHistoryRepository = (*AccountHistoryRepository)(nil)

func NewAccountHistoryRepository(db Querier) *AccountHistoryRepository {
	return &AccountHistoryRepository{db: db}
}

// Every branch applies its keyset and limit before the final merge. The query
// only reads financial facts; it never initializes a wallet or repairs rows.
func (r *AccountHistoryRepository) ListAccountHistory(ctx context.Context, owner uuid.UUID, limit int, after *domain.AccountHistoryCursor) ([]domain.AccountHistoryRow, error) {
	if owner == uuid.Nil || limit < 1 || limit > 51 {
		return nil, domain.ErrInvalidInput
	}
	var at *time.Time
	var source domain.AccountHistorySource
	var id uuid.UUID
	if after != nil {
		if after.CreatedAt.IsZero() || after.ID == uuid.Nil || after.Source < domain.AccountHistoryLedger || after.Source > domain.AccountHistoryExternalRefund {
			return nil, domain.ErrInvalidInput
		}
		at, source, id = &after.CreatedAt, after.Source, after.ID
	}
	rows, err := r.db.Query(ctx, accountHistorySQL, owner, limit, at, source, id)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	out := make([]domain.AccountHistoryRow, 0, limit)
	for rows.Next() {
		var row domain.AccountHistoryRow
		if err := rows.Scan(&row.ID, &row.Source, &row.Kind, &row.CreatedAt, &row.StarsDelta, &row.CreditDenominationVersion, &row.AmountMinor, &row.Currency, &row.Status, &row.PlanCode, &row.Modality, &row.Operation, &row.TestMode); err != nil {
			return nil, mapError(err)
		}
		switch row.PlanCode {
		case "lite", "start", "pro", "ultima", "elite":
		default:
			row.PlanCode = ""
		}
		if !domain.Modality(row.Modality).Valid() {
			row.Modality = ""
		}
		if !domain.OperationType(row.Operation).Valid() {
			row.Operation = ""
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return out, nil
}

const accountHistorySQL = `
WITH ledger_page AS MATERIALIZED (
 SELECT l.* FROM ledger_entries l
 JOIN credit_accounts a ON a.id=l.account_id AND a.owner_account_id=$1 AND a.currency='credits'
 WHERE l.owner_account_id=$1 AND l.status='committed' AND l.amount<>0
 AND l.type NOT IN ('reserve','release')
 AND ($3::timestamptz IS NULL OR (l.created_at,0,l.id)<($3::timestamptz,$4::int,$5::uuid))
 AND NOT (l.type='topup' AND l.amount>0 AND EXISTS (
  SELECT 1 FROM payment_intents p WHERE p.account_id=$1 AND p.currency='rub' AND p.amount>0
  AND l.idempotency_key='topup:'||p.provider||':'||p.provider_payment_id))
 ORDER BY l.created_at DESC,l.id DESC LIMIT $2
), payment_page AS MATERIALIZED (
 SELECT p.* FROM payment_intents p WHERE p.account_id=$1 AND p.currency='rub' AND p.amount>0
 AND ($3::timestamptz IS NULL OR (p.created_at,1,p.id)<($3::timestamptz,$4::int,$5::uuid))
 ORDER BY p.created_at DESC,p.id DESC LIMIT $2
), refund_page AS MATERIALIZED (
 SELECT r.*,p.provider,p.account_id,p.provider_payment_id,p.amount AS payment_amount,p.currency
 FROM payment_refunds r JOIN payment_intents p ON p.id=r.intent_id AND p.account_id=$1
 WHERE p.currency='rub' AND r.amount>0
 AND ($3::timestamptz IS NULL OR (r.created_at,2,r.id)<($3::timestamptz,$4::int,$5::uuid))
 ORDER BY r.created_at DESC,r.id DESC LIMIT $2
), external_page AS MATERIALIZED (
 SELECT md5('web-history-refund:'||e.provider||':'||e.provider_refund_id)::uuid AS id,
 e.intent_id,e.amount,e.verified_at AS created_at,p.provider,p.account_id,p.provider_payment_id,p.amount AS payment_amount,p.currency
 FROM web_referral_refund_evidence e JOIN payment_intents p ON p.id=e.intent_id AND p.account_id=$1 AND p.provider=e.provider
 WHERE p.currency='rub' AND e.amount>0
 AND NOT EXISTS (SELECT 1 FROM payment_refunds r JOIN payment_intents owned ON owned.id=r.intent_id AND owned.account_id=$1
  WHERE r.intent_id=e.intent_id AND r.provider_refund_id=e.provider_refund_id AND owned.provider=e.provider)
 AND ($3::timestamptz IS NULL OR (e.verified_at,3,md5('web-history-refund:'||e.provider||':'||e.provider_refund_id)::uuid)<($3::timestamptz,$4::int,$5::uuid))
 ORDER BY e.verified_at DESC,id DESC LIMIT $2
), history AS (
 SELECT l.id,0 AS source,
 CASE
 WHEN EXISTS(SELECT 1 FROM web_referral_rewards w WHERE w.referrer_account_id=$1 AND w.grant_entry_id=l.id) THEN 'referral_bonus'
 WHEN EXISTS(SELECT 1 FROM web_referral_rewards w WHERE w.referrer_account_id=$1 AND w.reversal_entry_id=l.id) THEN 'referral_reversal'
 WHEN strpos(l.idempotency_key,'payment_refund_debit:')=1 THEN 'refund_adjustment'
 WHEN strpos(l.idempotency_key,'payment_refund_compensate:')=1 OR strpos(l.idempotency_key,'compensate:')=1 THEN 'refund_compensation'
 WHEN l.reason='credit lot expiry' THEN 'expiry'
 WHEN l.type='capture' THEN 'generation'
 WHEN l.type='refund' THEN 'generation_refund'
 WHEN l.lot_kind='bonus' THEN 'bonus'
 ELSE 'adjustment' END AS kind,
 l.created_at,l.amount AS stars_delta,l.credit_denomination_version::int,
 NULL::bigint AS amount_minor,'credits'::text AS currency,l.status,
 ''::text AS plan_code,COALESCE(j.modality,'') AS modality,COALESCE(j.operation_type,'') AS operation,NULL::boolean AS test_mode
 FROM ledger_page l LEFT JOIN jobs j ON j.id=l.job_id AND j.account_id=$1 AND j.channel='web'
 UNION ALL
 SELECT p.id,1,CASE WHEN p.metadata->'subscription' IS NOT NULL AND p.metadata->'subscription'<>'null'::jsonb THEN 'subscription' ELSE 'topup' END,
 p.created_at,COALESCE(l.amount,0),COALESCE(l.credit_denomination_version,2)::int,p.amount,p.currency,p.status,
 COALESCE(p.metadata#>>'{subscription,plan,code}',''),'','',
 CASE WHEN p.provider='mock' THEN true WHEN e.mode='test' THEN true WHEN e.mode='live' THEN false ELSE NULL END
 FROM payment_page p
 LEFT JOIN ledger_entries l ON l.owner_account_id=$1 AND l.type='topup' AND l.status='committed' AND l.amount>0
  AND l.idempotency_key='topup:'||p.provider||':'||p.provider_payment_id
  AND EXISTS(SELECT 1 FROM credit_accounts a WHERE a.id=l.account_id AND a.owner_account_id=$1 AND a.currency='credits')
 LEFT JOIN web_referral_payment_evidence e ON e.intent_id=p.id AND e.account_id=$1 AND e.provider=p.provider
  AND e.provider_payment_id=p.provider_payment_id AND e.amount=p.amount AND e.currency=p.currency
 UNION ALL
 SELECT r.id,2,'refund_money',r.created_at,0,2,COALESCE(verified.amount,r.amount),r.currency,
 CASE WHEN verified.intent_id IS NOT NULL THEN 'succeeded' ELSE r.status END,'','','',
 CASE WHEN r.provider='mock' THEN true WHEN e.mode='test' THEN true WHEN e.mode='live' THEN false ELSE NULL END
 FROM refund_page r
 LEFT JOIN web_referral_refund_evidence verified ON verified.intent_id=r.intent_id AND verified.provider=r.provider
  AND verified.provider_refund_id=r.provider_refund_id AND verified.amount>0 AND verified.amount<=r.payment_amount
 LEFT JOIN web_referral_payment_evidence e ON e.intent_id=r.intent_id AND e.account_id=$1 AND e.provider=r.provider
  AND e.provider_payment_id=r.provider_payment_id AND e.amount=r.payment_amount AND e.currency=r.currency
 UNION ALL
 SELECT r.id,3,'refund_money',r.created_at,0,2,r.amount,r.currency,'succeeded','','','',
 CASE WHEN e.mode='test' THEN true WHEN e.mode='live' THEN false ELSE NULL END
 FROM external_page r LEFT JOIN web_referral_payment_evidence e ON e.intent_id=r.intent_id AND e.account_id=$1 AND e.provider=r.provider
  AND e.provider_payment_id=r.provider_payment_id AND e.amount=r.payment_amount AND e.currency=r.currency
)
SELECT id,source,kind,created_at,stars_delta,credit_denomination_version,amount_minor,currency,status,plan_code,modality,operation,test_mode
FROM history ORDER BY created_at DESC,source DESC,id DESC LIMIT $2`
