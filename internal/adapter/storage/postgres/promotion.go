package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"vk-ai-aggregator/internal/domain"
)

var _ domain.PromotionRepository = (*PaymentRepository)(nil)

func (r *PaymentRepository) LockPromotionAccount(ctx context.Context, owner uuid.UUID) error {
	var id uuid.UUID
	return mapError(r.db.QueryRow(ctx, `SELECT id FROM accounts WHERE id=$1 FOR UPDATE`, owner).Scan(&id))
}
func (r *PaymentRepository) PromotionEligibleLocked(ctx context.Context, owner uuid.UUID) (bool, error) {
	var eligible bool
	err := r.db.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM payment_intents p LEFT JOIN users u ON u.id=p.user_id WHERE COALESCE(p.account_id,u.account_id)=$1 AND p.status NOT IN ('canceled','expired','failed'))
	 AND NOT EXISTS(SELECT 1 FROM ledger_entries e JOIN credit_accounts b ON b.id=e.account_id LEFT JOIN users u ON u.id=b.user_id WHERE COALESCE(e.owner_account_id,b.owner_account_id,u.account_id)=$1 AND e.type='topup' AND e.status='committed' AND e.idempotency_key LIKE 'topup:%')
 AND NOT EXISTS(SELECT 1 FROM subscription_promotion_uses WHERE account_id=$1 AND state IN ('reserved','consumed'))`, owner).Scan(&eligible)
	return eligible, mapError(err)
}

const promotionProjection = `p.config || jsonb_build_object('id',p.id,'code',p.code,'revision',p.revision,'created_at',p.created_at,'updated_at',p.updated_at,'reserved_count',(SELECT count(*) FROM subscription_promotion_uses u WHERE u.promotion_id=p.id AND u.state='reserved'),'redeemed_count',(SELECT count(*) FROM subscription_promotion_uses u WHERE u.promotion_id=p.id AND u.state='consumed'))`

func scanPromotion(row rowScanner) (*domain.Promotion, error) {
	var raw []byte
	if err := row.Scan(&raw); err != nil {
		return nil, mapError(err)
	}
	var p domain.Promotion
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
func (r *PaymentRepository) GetPromotionLocked(ctx context.Context, code string) (*domain.Promotion, error) {
	// Acquire first, then read counters in a fresh READ COMMITTED statement.
	// A SELECT that waits on FOR UPDATE retains its pre-wait subquery snapshot.
	var id uuid.UUID
	if err := r.db.QueryRow(ctx, `SELECT id FROM subscription_promotions WHERE code=$1 FOR UPDATE`, code).Scan(&id); err != nil {
		return nil, mapError(err)
	}
	return scanPromotion(r.db.QueryRow(ctx, `SELECT `+promotionProjection+` FROM subscription_promotions p WHERE p.id=$1`, id))
}
func (r *PaymentRepository) ListPromotions(ctx context.Context, limit, offset int) ([]domain.Promotion, error) {
	rows, err := r.db.Query(ctx, `SELECT `+promotionProjection+` FROM subscription_promotions p ORDER BY p.created_at DESC,p.id LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	out := []domain.Promotion{}
	for rows.Next() {
		p, e := scanPromotion(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, *p)
	}
	return out, mapError(rows.Err())
}
func (r *PaymentRepository) ReservePromotionLocked(ctx context.Context, owner, intent uuid.UUID, s domain.PromotionSnapshot) error {
	if _, ok := r.db.(pgx.Tx); !ok {
		return domain.ErrPromotionUnavailable
	}
	eligible, e := r.PromotionEligibleLocked(ctx, owner)
	if e != nil {
		return e
	}
	if !eligible {
		return domain.ErrConflict
	}
	p, e := r.GetPromotionLocked(ctx, s.Code)
	if e != nil {
		return e
	}
	if p.ID != s.PromotionID || p.Revision != s.Revision || p.MaxUses > 0 && p.ReservedCount+p.RedeemedCount >= p.MaxUses {
		return domain.ErrConflict
	}
	raw, e := json.Marshal(s)
	if e != nil {
		return e
	}
	_, e = r.db.Exec(ctx, `INSERT INTO subscription_promotion_uses(intent_id,promotion_id,account_id,state,snapshot) VALUES($1,$2,$3,'reserved',$4)`, intent, s.PromotionID, owner, raw)
	return mapError(e)
}
func (r *PaymentRepository) FinishPromotionLocked(ctx context.Context, owner, intent uuid.UUID, paid bool) error {
	if _, ok := r.db.(pgx.Tx); !ok {
		return domain.ErrPromotionUnavailable
	}
	if e := r.LockPromotionAccount(ctx, owner); e != nil {
		return e
	}
	var promo uuid.UUID
	if e := r.db.QueryRow(ctx, `SELECT promotion_id FROM subscription_promotion_uses WHERE intent_id=$1 AND account_id=$2`, intent, owner).Scan(&promo); e != nil {
		return mapError(e)
	}
	var locked uuid.UUID
	if e := r.db.QueryRow(ctx, `SELECT id FROM subscription_promotions WHERE id=$1 FOR UPDATE`, promo).Scan(&locked); e != nil {
		return mapError(e)
	}
	state := "released"
	if paid {
		state = "consumed"
	}
	_, e := r.db.Exec(ctx, `UPDATE subscription_promotion_uses SET state=$3,updated_at=now() WHERE intent_id=$1 AND account_id=$2 AND (state='reserved' OR ($3='consumed' AND state='released'))`, intent, owner, state)
	return mapError(e)
}
func (r *PaymentRepository) MutatePromotion(ctx context.Context, actor string, key uuid.UUID, fingerprint string, p *domain.Promotion) (*domain.Promotion, error) {
	if _, ok := r.db.(pgx.Tx); !ok {
		return nil, domain.ErrPromotionUnavailable
	}
	// The advisory lock serializes same-key creation before the audit row exists.
	if _, e := r.db.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, actor+":"+key.String()); e != nil {
		return nil, e
	}
	var fp string
	var saved []byte
	e := r.db.QueryRow(ctx, `SELECT fingerprint,after_snapshot FROM subscription_promotion_mutations WHERE actor_ref=$1 AND request_key=$2`, actor, key).Scan(&fp, &saved)
	if e == nil {
		if fp != fingerprint {
			return nil, domain.ErrConflict
		}
		var result domain.Promotion
		if e = json.Unmarshal(saved, &result); e != nil {
			return nil, e
		}
		return &result, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return nil, mapError(e)
	}
	var before []byte
	id := p.ID
	if id == uuid.Nil {
		id = uuid.New()
	} else {
		var code string
		if e := r.db.QueryRow(ctx, `SELECT code FROM subscription_promotions WHERE id=$1 FOR UPDATE`, id).Scan(&code); e != nil {
			return nil, mapError(e)
		}
		old, e := r.GetPromotionLocked(ctx, code)
		if e != nil {
			return nil, e
		}
		if old.Revision != p.Revision || old.Code != p.Code || p.MaxUses > 0 && p.MaxUses < old.ReservedCount+old.RedeemedCount {
			return nil, domain.ErrConflict
		}
		before, _ = json.Marshal(old)
	}
	raw, e := json.Marshal(p)
	if e != nil {
		return nil, e
	}
	if p.ID == uuid.Nil {
		_, e = r.db.Exec(ctx, `INSERT INTO subscription_promotions(id,code,revision,config) VALUES($1,$2,1,$3)`, id, p.Code, raw)
	} else {
		_, e = r.db.Exec(ctx, `UPDATE subscription_promotions SET config=$2,revision=revision+1,updated_at=now() WHERE id=$1`, id, raw)
	}
	if e != nil {
		return nil, mapError(e)
	}
	next, e := r.GetPromotionLocked(ctx, p.Code)
	if e != nil {
		return nil, e
	}
	after, e := json.Marshal(next)
	if e != nil {
		return nil, e
	}
	_, e = r.db.Exec(ctx, `INSERT INTO subscription_promotion_mutations(actor_ref,request_key,fingerprint,promotion_id,before_snapshot,after_snapshot) VALUES($1,$2,$3,$4,$5,$6)`, actor, key, fingerprint, id, before, after)
	if e != nil {
		return nil, mapError(e)
	}
	return next, nil
}
