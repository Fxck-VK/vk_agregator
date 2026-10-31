package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

const lotColumns = `id, account_id, kind, available_credits, reserved_credits, expires_at, created_at`

// All mutation helpers require the account lock first. Reservation operations
// then lock their reservation, then its lots, in that order on every path.
func lockLots(ctx context.Context, q Querier, account uuid.UUID) ([]domain.CreditLot, error) {
	rows, err := q.Query(ctx, `SELECT `+lotColumns+` FROM credit_lots WHERE account_id=$1 AND available_credits>0 ORDER BY expires_at ASC NULLS LAST,created_at,id FOR UPDATE`, account)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var lots []domain.CreditLot
	for rows.Next() {
		var l domain.CreditLot
		if err := rows.Scan(&l.ID, &l.AccountID, &l.Kind, &l.AvailableCredits, &l.ReservedCredits, &l.ExpiresAt, &l.CreatedAt); err != nil {
			return nil, mapError(err)
		}
		lots = append(lots, l)
	}
	return lots, mapError(rows.Err())
}

func projectLedgerLot(ctx context.Context, q Querier, e *domain.LedgerEntry) error {
	if e.Status != domain.LedgerStatusCommitted || e.Amount == 0 {
		return nil
	}
	if e.Amount > 0 {
		kind := e.LotKind
		if kind == "" {
			kind = domain.CreditLotLegacy
		}
		_, err := q.Exec(ctx, `INSERT INTO credit_lots(id,account_id,source_entry_id,kind,available_credits,expires_at) VALUES($1,$2,$1,$3,$4,$5)`, e.ID, e.AccountID, kind, e.Amount, e.LotExpiresAt)
		return mapError(err)
	}
	// Expired stars are already unavailable. Retire their unreserved remainder
	// before attributing a new debit, so stale lots cannot absorb a correction
	// or refund that must reduce still-spendable funds. Expiry uses raw ledger
	// writes and never re-enters this generic projection.
	// Ledger created_at can be the outer transaction's start time; expiry must
	// use the current time after obtaining the account lock instead.
	if _, err := expireAccountLots(ctx, q, e.AccountID, time.Now(), 0); err != nil {
		return err
	}
	lots, err := lockLots(ctx, q, e.AccountID)
	if err != nil {
		return err
	}
	left := -e.Amount
	for _, l := range lots {
		if left == 0 {
			break
		}
		if l.AvailableCredits <= 0 {
			continue
		}
		take := min(left, l.AvailableCredits)
		if _, err := q.Exec(ctx, `UPDATE credit_lots SET available_credits=available_credits-$2 WHERE id=$1`, l.ID, take); err != nil {
			return mapError(err)
		}
		left -= take
	}
	if left > 0 {
		_, err := q.Exec(ctx, `INSERT INTO credit_lots(id,account_id,source_entry_id,kind,available_credits) VALUES($1,$2,$1,'legacy',$3)`, e.ID, e.AccountID, -left)
		return mapError(err)
	}
	return nil
}

func allocateReservationLots(ctx context.Context, q Querier, res *domain.CreditReservation, now time.Time) error {
	lots, err := lockLots(ctx, q, res.AccountID)
	if err != nil {
		return err
	}
	left := res.Amount
	for _, l := range lots {
		if left == 0 {
			break
		}
		if l.AvailableCredits <= 0 || (l.ExpiresAt != nil && !l.ExpiresAt.After(now)) {
			continue
		}
		take := min(left, l.AvailableCredits)
		if _, err := q.Exec(ctx, `UPDATE credit_lots SET available_credits=available_credits-$2,reserved_credits=reserved_credits+$2 WHERE id=$1`, l.ID, take); err != nil {
			return mapError(err)
		}
		if _, err := q.Exec(ctx, `INSERT INTO credit_reservation_lots(reservation_id,lot_id,account_id,amount) VALUES($1,$2,$3,$4)`, res.ID, l.ID, res.AccountID, take); err != nil {
			return mapError(err)
		}
		left -= take
	}
	if left != 0 {
		return domain.ErrInsufficientCredits
	}
	return nil
}

func insertLotExpiry(ctx context.Context, q Querier, l domain.CreditLot, amount int64, key string) error {
	e := &domain.LedgerEntry{AccountID: l.AccountID, Type: domain.LedgerAdjustment, Amount: -amount, Status: domain.LedgerStatusCommitted, IdempotencyKey: key, Reason: "credit lot expiry"}
	inserted, err := insertLedgerEntry(ctx, q, e)
	if err != nil {
		return err
	}
	if !inserted {
		return domain.ErrConflict
	}
	return adjustBalance(ctx, q, l.AccountID, -amount)
}

func expireAccountLots(ctx context.Context, q Querier, account uuid.UUID, now time.Time, limit int) (int64, error) {
	rows, err := q.Query(ctx, `SELECT `+lotColumns+` FROM credit_lots WHERE account_id=$1 AND expires_at<=$2 AND available_credits>0 ORDER BY expires_at,created_at,id LIMIT NULLIF($3::int,0) FOR UPDATE`, account, now, limit)
	if err != nil {
		return 0, mapError(err)
	}
	var lots []domain.CreditLot
	for rows.Next() {
		var l domain.CreditLot
		if err := rows.Scan(&l.ID, &l.AccountID, &l.Kind, &l.AvailableCredits, &l.ReservedCredits, &l.ExpiresAt, &l.CreatedAt); err != nil {
			rows.Close()
			return 0, mapError(err)
		}
		lots = append(lots, l)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, mapError(err)
	}
	var n int64
	for _, l := range lots {
		if limit > 0 && n >= int64(limit) {
			break
		}
		if l.ExpiresAt == nil || l.ExpiresAt.After(now) || l.AvailableCredits <= 0 {
			continue
		}
		if err := insertLotExpiry(ctx, q, l, l.AvailableCredits, "credit-lot-expiry:"+l.ID.String()); err != nil {
			return n, err
		}
		if _, err := q.Exec(ctx, `UPDATE credit_lots SET available_credits=0 WHERE id=$1`, l.ID); err != nil {
			return n, mapError(err)
		}
		n++
	}
	return n, nil
}

func terminalReservationLots(ctx context.Context, q Querier, res *domain.CreditReservation, capture bool, now time.Time) error {
	rows, err := q.Query(ctx, `SELECT l.id,l.account_id,l.kind,l.available_credits,l.reserved_credits,l.expires_at,l.created_at,a.amount
 FROM credit_reservation_lots a JOIN credit_lots l ON l.id=a.lot_id AND l.account_id=a.account_id
 WHERE a.reservation_id=$1 ORDER BY l.id FOR UPDATE OF l`, res.ID)
	if err != nil {
		return mapError(err)
	}
	type allocation struct {
		lot    domain.CreditLot
		amount int64
	}
	var allocations []allocation
	for rows.Next() {
		var a allocation
		if err := rows.Scan(&a.lot.ID, &a.lot.AccountID, &a.lot.Kind, &a.lot.AvailableCredits, &a.lot.ReservedCredits, &a.lot.ExpiresAt, &a.lot.CreatedAt, &a.amount); err != nil {
			rows.Close()
			return mapError(err)
		}
		allocations = append(allocations, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return mapError(err)
	}
	var total int64
	for _, a := range allocations {
		total += a.amount
	}
	if total != res.Amount {
		return domain.ErrConflict
	}
	for _, a := range allocations {
		restore := int64(0)
		expired := a.lot.ExpiresAt != nil && !a.lot.ExpiresAt.After(now)
		if !capture && !expired {
			restore = a.amount
		}
		if _, err := q.Exec(ctx, `UPDATE credit_lots SET reserved_credits=reserved_credits-$2,available_credits=available_credits+$3 WHERE id=$1`, a.lot.ID, a.amount, restore); err != nil {
			return mapError(err)
		}
		if !capture && expired {
			if err := insertLotExpiry(ctx, q, a.lot, a.amount, "credit-lot-expiry:release:"+res.ID.String()+":"+a.lot.ID.String()); err != nil {
				return err
			}
		}
	}
	return nil
}

// ExpireCreditLots expires at most limit lots. Account IDs are acquired in a
// stable order; account locks always precede lot locks, including maintenance.
func (r *BillingRepository) ExpireCreditLots(ctx context.Context, now time.Time, limit int) (int64, error) {
	if limit <= 0 {
		return 0, nil
	}
	var count int64
	err := r.inTx(ctx, func(q Querier) error {
		rows, err := q.Query(ctx, `SELECT DISTINCT account_id FROM credit_lots WHERE expires_at<=$1 AND available_credits>0 ORDER BY account_id LIMIT $2`, now, limit)
		if err != nil {
			return mapError(err)
		}
		var accounts []uuid.UUID
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return mapError(err)
			}
			accounts = append(accounts, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return mapError(err)
		}
		for _, id := range accounts {
			if _, err := lockCreditAccountOwner(ctx, q, id); err != nil {
				return err
			}
			n, err := expireAccountLots(ctx, q, id, now, limit-int(count))
			count += n
			if err != nil {
				return err
			}
			if count >= int64(limit) {
				break
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *BillingRepository) CreditLotBalancesForOwner(ctx context.Context, owner uuid.UUID, now time.Time) ([]domain.CreditLotBalance, error) {
	rows, err := r.q().Query(ctx, `SELECT l.id,l.kind,CASE WHEN l.expires_at<=$2 AND l.available_credits>0 THEN 0 ELSE l.available_credits END,l.reserved_credits,l.expires_at
 FROM credit_lots l JOIN credit_accounts c ON c.id=l.account_id WHERE c.owner_account_id=$1 AND c.currency='credits'
 AND (l.available_credits<>0 OR l.reserved_credits<>0) ORDER BY l.expires_at ASC NULLS LAST,l.created_at,l.id`, owner, now)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	out := []domain.CreditLotBalance{}
	for rows.Next() {
		var l domain.CreditLotBalance
		if err := rows.Scan(&l.ID, &l.Kind, &l.AvailableCredits, &l.ReservedCredits, &l.ExpiresAt); err != nil {
			return nil, mapError(err)
		}
		if l.AvailableCredits != 0 || l.ReservedCredits != 0 {
			out = append(out, l)
		}
	}
	return out, mapError(rows.Err())
}

// RevokeCreditLotForGrant is the optional full-refund accounting primitive.
// The caller verifies the provider refund and changes subscription state in the
// same outer transaction. This method touches only the original grant's lot.
func (r *BillingRepository) RevokeCreditLotForGrant(ctx context.Context, owner uuid.UUID, grantLedgerKey string, now time.Time) error {
	if owner == uuid.Nil {
		return domain.ErrConflict
	}
	return r.inTx(ctx, func(q Querier) error {
		entry, err := ledgerByIdempotencyKey(ctx, q, grantLedgerKey)
		if err != nil {
			return err
		}
		if entry.OwnerAccountID != owner || entry.Type != domain.LedgerTopup || entry.Status != domain.LedgerStatusCommitted || entry.Amount <= 0 || entry.LotKind != domain.CreditLotSubscription {
			return domain.ErrConflict
		}
		storedOwner, err := lockCreditAccountOwner(ctx, q, entry.AccountID)
		if err != nil {
			return err
		}
		if storedOwner != owner {
			return domain.ErrConflict
		}
		var lot domain.CreditLot
		if err := q.QueryRow(ctx, `SELECT `+lotColumns+` FROM credit_lots WHERE source_entry_id=$1 AND account_id=$2 FOR UPDATE`, entry.ID, entry.AccountID).Scan(&lot.ID, &lot.AccountID, &lot.Kind, &lot.AvailableCredits, &lot.ReservedCredits, &lot.ExpiresAt, &lot.CreatedAt); err != nil {
			return mapError(err)
		}
		if lot.Kind != domain.CreditLotSubscription {
			return domain.ErrConflict
		}
		shorten := lot.ExpiresAt == nil || lot.ExpiresAt.After(now)
		if lot.AvailableCredits == 0 && !shorten {
			return nil
		}
		if lot.AvailableCredits > 0 {
			if err := insertLotExpiry(ctx, q, lot, lot.AvailableCredits, "credit-lot-expiry:"+lot.ID.String()); err != nil {
				return err
			}
		}
		_, err = q.Exec(ctx, `UPDATE credit_lots SET available_credits=0,expires_at=LEAST(COALESCE(expires_at,$2),$2) WHERE id=$1`, lot.ID, now)
		return mapError(err)
	})
}
