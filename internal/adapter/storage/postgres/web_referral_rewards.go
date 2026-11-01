package postgres

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
)

var _ domain.WebReferralRewardProcessor = (*MaintenanceRepository)(nil)

// ProcessWebReferralRewards is bounded and safe to run from multiple workers.
// Each batch commits qualification, ledger movements and counters together.
func (r *MaintenanceRepository) ProcessWebReferralRewards(ctx context.Context, now time.Time, limit int) (int64, error) {
	if limit <= 0 {
		return 0, nil
	}
	if limit > 1000 {
		limit = 1000
	}
	if now.IsZero() {
		return 0, domain.ErrInvalidInput
	}
	var count int64
	fn := func(ctx context.Context, tx pgx.Tx) error {
		var err error
		count, err = processWebReferralRewardBatch(ctx, tx, now, limit)
		return err
	}
	var err error
	switch db := r.db.(type) {
	case *pgxpool.Pool:
		err = RunInTx(ctx, db, fn)
	case pgx.Tx:
		var tx pgx.Tx
		tx, err = db.Begin(ctx)
		if err == nil {
			defer tx.Rollback(ctx)
			err = fn(ctx, tx)
			if err == nil {
				err = tx.Commit(ctx)
			}
		}
	default:
		err = fmt.Errorf("postgres: web referral rewards require a pool or transaction")
	}
	if err != nil {
		return 0, err
	}
	return count, nil
}

func processWebReferralRewardBatch(ctx context.Context, tx pgx.Tx, now time.Time, limit int) (int64, error) {
	// Recover pre-hook settlements (including a rolling deployment's old writer)
	// as ineligible history. Never infer live/test mode from client metadata.
	_, err := tx.Exec(ctx, `INSERT INTO web_referral_payment_evidence(intent_id,account_id,provider,provider_payment_id,amount,currency,mode,verified_at)
 SELECT p.id,p.account_id,p.provider,p.provider_payment_id,p.amount,p.currency,'historical',l.created_at
 FROM payment_intents p JOIN ledger_entries l ON l.idempotency_key='topup:'||p.provider||':'||p.provider_payment_id AND l.owner_account_id=p.account_id AND l.status='committed' AND l.amount>0
 WHERE p.provider='yookassa' AND p.status IN ('succeeded','refunded','partially_refunded')
 AND EXISTS(SELECT 1 FROM referrals f WHERE f.referred_account_id=p.account_id AND f.source='web')
 AND NOT EXISTS(SELECT 1 FROM web_referral_payment_evidence e WHERE e.intent_id=p.id)
 ORDER BY l.created_at,p.id LIMIT $1 ON CONFLICT DO NOTHING`, limit)
	if err != nil {
		return 0, mapError(err)
	}
	policy := domain.CurrentWebReferralRewardPolicy()
	_, err = tx.Exec(ctx, `INSERT INTO web_referral_rewards(referral_id,referred_account_id,referrer_account_id,intent_id,policy_version,stars,qualified_at,due_at)
 SELECT f.id,f.referred_account_id,f.referrer_account_id,e.intent_id,$2,$3,e.verified_at,e.verified_at+make_interval(hours=>$4)
 FROM referrals f CROSS JOIN LATERAL(SELECT * FROM web_referral_payment_evidence e WHERE e.account_id=f.referred_account_id AND e.mode<>'test' ORDER BY e.verified_at,e.intent_id LIMIT 1)e
 WHERE f.source='web' AND e.mode='live' AND f.referrer_account_id<>f.referred_account_id
 AND NOT EXISTS(SELECT 1 FROM web_referral_rewards w WHERE w.referred_account_id=f.referred_account_id)
 AND NOT EXISTS(SELECT 1 FROM payment_intents p JOIN ledger_entries l ON l.idempotency_key='topup:'||p.provider||':'||p.provider_payment_id AND l.owner_account_id=p.account_id AND l.status='committed' AND l.amount>0 WHERE p.account_id=f.referred_account_id AND p.provider='yookassa' AND NOT EXISTS(SELECT 1 FROM web_referral_payment_evidence x WHERE x.intent_id=p.id))
 ORDER BY e.verified_at,f.id LIMIT $1 ON CONFLICT DO NOTHING`, limit, policy.Version, policy.Stars, policy.DelayHours)
	if err != nil {
		return 0, mapError(err)
	}
	_, err = tx.Exec(ctx, `UPDATE referrals f SET status='activated',updated_at=$1 FROM web_referral_rewards w WHERE f.id=w.referral_id AND f.status='registered'`, now)
	if err != nil {
		return 0, mapError(err)
	}
	rows, err := tx.Query(ctx, `SELECT w.referral_id,w.referrer_account_id,w.intent_id,w.stars,w.status,w.grant_entry_id
 FROM web_referral_rewards w JOIN payment_intents p ON p.id=w.intent_id
 WHERE (w.status='pending' AND w.due_at<=$1) OR (w.status IN ('pending','applied') AND (p.status IN ('refunded','partially_refunded') OR EXISTS(SELECT 1 FROM web_referral_refund_evidence x WHERE x.intent_id=w.intent_id)))
 ORDER BY w.due_at,w.referral_id LIMIT $2 FOR UPDATE OF w SKIP LOCKED`, now, limit)
	if err != nil {
		return 0, mapError(err)
	}
	type reward struct {
		id, owner, intent uuid.UUID
		stars             int64
		status            string
		grant             *uuid.UUID
	}
	var rewards []reward
	for rows.Next() {
		var w reward
		if err = rows.Scan(&w.id, &w.owner, &w.intent, &w.stars, &w.status, &w.grant); err != nil {
			rows.Close()
			return 0, mapError(err)
		}
		rewards = append(rewards, w)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, mapError(err)
	}
	var count int64
	// Disjoint claimed batches may contain the same owners in different due
	// order. Lock their wallets in one global order to avoid A/B vs B/A cycles.
	sort.Slice(rewards, func(i, j int) bool {
		if rewards[i].owner == rewards[j].owner {
			return rewards[i].id.String() < rewards[j].id.String()
		}
		return rewards[i].owner.String() < rewards[j].owner.String()
	})
	for _, w := range rewards {
		var locked uuid.UUID
		if err = tx.QueryRow(ctx, `SELECT intent_id FROM web_referral_payment_evidence WHERE intent_id=$1 FOR UPDATE`, w.intent).Scan(&locked); err != nil {
			return 0, mapError(err)
		}
		var refunded bool
		err = tx.QueryRow(ctx, `SELECT status IN ('refunded','partially_refunded') OR EXISTS(SELECT 1 FROM web_referral_refund_evidence WHERE intent_id=$1) FROM payment_intents WHERE id=$1`, w.intent).Scan(&refunded)
		if err != nil {
			return 0, mapError(err)
		}
		next := w.status
		var reversal *uuid.UUID
		grant := w.grant
		if refunded && w.status == "pending" {
			next = "cancelled"
		} else {
			billing := NewBillingRepositoryTx(tx)
			wallet, ensureErr := billingservice.New(billing).EnsureAccountForAccount(ctx, billing, w.owner, domain.CurrencyCredits)
			if ensureErr != nil {
				return 0, ensureErr
			}
			if refunded && w.status == "applied" {
				if grant == nil {
					return 0, domain.ErrConflict
				}
				id, reverseErr := reverseWebReferralGrant(ctx, tx, w.owner, wallet.ID, *grant, w.stars, "web-referral-reward:"+w.id.String()+":reverse")
				if reverseErr != nil {
					return 0, reverseErr
				}
				reversal = &id
				next = "reversed"
			} else if w.status == "pending" {
				e := &domain.LedgerEntry{AccountID: wallet.ID, OwnerAccountID: w.owner, Type: domain.LedgerTopup, Amount: w.stars, LotKind: domain.CreditLotBonus, IdempotencyKey: "web-referral-reward:" + w.id.String() + ":grant", Reason: "web referral first payment reward"}
				if err = billing.AppendEntry(ctx, e); err != nil {
					return 0, err
				}
				grant = &e.ID
				next = "applied"
			}
		}
		if next == w.status {
			continue
		}
		if _, err = tx.Exec(ctx, `UPDATE web_referral_rewards SET status=$2,grant_entry_id=$3,reversal_entry_id=$4,updated_at=$5 WHERE referral_id=$1`, w.id, next, grant, reversal, now); err != nil {
			return 0, mapError(err)
		}
		state, rewardState := "activated", "pending"
		var awardedAt *time.Time
		if next == "applied" {
			state, rewardState = "rewarded", "applied"
			awardedAt = &now
		}
		if _, err = tx.Exec(ctx, `UPDATE referrals SET status=$2,reward_status=$3,rewarded_at=$4,updated_at=$5 WHERE id=$1 AND source='web'`, w.id, state, rewardState, awardedAt, now); err != nil {
			return 0, mapError(err)
		}
		count++
	}
	return count, nil
}

// Reverse only this grant's provenance. Reserved allocations remain usable by
// their existing jobs; nonexpiring debt offsets spent/held credits on release.
func reverseWebReferralGrant(ctx context.Context, q Querier, owner, wallet, grant uuid.UUID, stars int64, key string) (uuid.UUID, error) {
	lockedOwner, err := lockCreditAccountOwner(ctx, q, wallet)
	if err != nil {
		return uuid.Nil, err
	}
	if lockedOwner != owner {
		return uuid.Nil, domain.ErrConflict
	}
	var available int64
	err = q.QueryRow(ctx, `SELECT available_credits FROM credit_lots WHERE source_entry_id=$1 AND account_id=$2 AND kind='bonus' AND expires_at IS NULL FOR UPDATE`, grant, wallet).Scan(&available)
	if err != nil {
		return uuid.Nil, mapError(err)
	}
	if available < 0 || available > stars {
		return uuid.Nil, domain.ErrConflict
	}
	e := &domain.LedgerEntry{AccountID: wallet, OwnerAccountID: owner, Type: domain.LedgerAdjustment, Amount: -stars, IdempotencyKey: key, Reason: "web referral payment refunded"}
	inserted, err := insertLedgerEntry(ctx, q, e)
	if err != nil {
		return uuid.Nil, err
	}
	if !inserted {
		old, err := ledgerByIdempotencyKey(ctx, q, key)
		if err != nil {
			return uuid.Nil, err
		}
		if old.AccountID != wallet || old.Amount != -stars || old.OwnerAccountID != owner {
			return uuid.Nil, domain.ErrConflict
		}
		return old.ID, nil
	}
	if _, err = q.Exec(ctx, `UPDATE credit_lots SET available_credits=0 WHERE source_entry_id=$1`, grant); err != nil {
		return uuid.Nil, mapError(err)
	}
	if debt := stars - available; debt > 0 {
		if _, err = q.Exec(ctx, `INSERT INTO credit_lots(id,account_id,source_entry_id,kind,available_credits) VALUES($1,$2,$1,'legacy',$3)`, e.ID, wallet, -debt); err != nil {
			return uuid.Nil, mapError(err)
		}
	}
	if err = adjustBalance(ctx, q, wallet, -stars); err != nil {
		return uuid.Nil, err
	}
	return e.ID, nil
}
