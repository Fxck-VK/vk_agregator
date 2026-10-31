package memory

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

func lotExpired(l domain.CreditLot, now time.Time) bool {
	return l.ExpiresAt != nil && !l.ExpiresAt.After(now)
}

func (r *BillingRepo) sortedLotsLocked(account uuid.UUID) []domain.CreditLot {
	var lots []domain.CreditLot
	for _, l := range r.lots {
		if l.AccountID == account {
			lots = append(lots, l)
		}
	}
	sort.Slice(lots, func(i, j int) bool {
		a, b := lots[i], lots[j]
		if a.ExpiresAt != nil || b.ExpiresAt != nil {
			if a.ExpiresAt == nil {
				return false
			}
			if b.ExpiresAt == nil {
				return true
			}
			if !a.ExpiresAt.Equal(*b.ExpiresAt) {
				return a.ExpiresAt.Before(*b.ExpiresAt)
			}
		}
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		return a.ID.String() < b.ID.String()
	})
	return lots
}

// projectEntryLocked handles generic grants/corrections. Reservation terminals
// use their allocations instead, so capture cannot debit another grant twice.
func (r *BillingRepo) projectEntryLocked(e *domain.LedgerEntry) error {
	if e.Status != domain.LedgerStatusCommitted || e.Amount == 0 {
		return nil
	}
	if e.Amount > 0 {
		kind := e.LotKind
		if kind == "" {
			kind = domain.CreditLotLegacy
		}
		r.lots[e.ID] = domain.CreditLot{ID: e.ID, AccountID: e.AccountID, Kind: kind, AvailableCredits: e.Amount, ExpiresAt: copyLotTime(e.LotExpiresAt), CreatedAt: e.CreatedAt}
		return nil
	}
	// Preflight the combined expiry and debit before changing either ledger
	// projection. Raw expiry append bypasses generic projection to avoid recursion.
	afterExpiry := r.accounts[e.AccountID].BalanceCached
	for _, lot := range r.sortedLotsLocked(e.AccountID) {
		if !lotExpired(lot, e.CreatedAt) || lot.AvailableCredits <= 0 {
			continue
		}
		if e.IdempotencyKey == "credit-lot-expiry:"+lot.ID.String() || afterExpiry < math.MinInt64+lot.AvailableCredits {
			return domain.ErrConflict
		}
		afterExpiry -= lot.AvailableCredits
	}
	if afterExpiry < math.MinInt64-e.Amount {
		return domain.ErrConflict
	}
	if _, err := r.expireAccountLotsLocked(e.AccountID, e.CreatedAt, 0); err != nil {
		return err
	}
	remaining := -e.Amount
	for _, l := range r.sortedLotsLocked(e.AccountID) {
		if remaining == 0 {
			break
		}
		if l.AvailableCredits <= 0 {
			continue
		}
		take := min(remaining, l.AvailableCredits)
		l.AvailableCredits -= take
		remaining -= take
		r.lots[l.ID] = l
	}
	if remaining > 0 {
		r.lots[e.ID] = domain.CreditLot{ID: e.ID, AccountID: e.AccountID, Kind: domain.CreditLotLegacy, AvailableCredits: -remaining, CreatedAt: e.CreatedAt}
	}
	return nil
}

func copyLotTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := *t
	return &v
}

func (r *BillingRepo) allocateLotsLocked(res *domain.CreditReservation, now time.Time) {
	left := res.Amount
	r.allocations[res.ID] = map[uuid.UUID]int64{}
	for _, l := range r.sortedLotsLocked(res.AccountID) {
		if left == 0 {
			break
		}
		if l.AvailableCredits <= 0 || lotExpired(l, now) {
			continue
		}
		take := min(left, l.AvailableCredits)
		l.AvailableCredits -= take
		l.ReservedCredits += take
		r.lots[l.ID] = l
		r.allocations[res.ID][l.ID] = take
		left -= take
	}
}

func (r *BillingRepo) terminalLotsLocked(res domain.CreditReservation, capture bool, now time.Time) error {
	for id, amount := range r.allocations[res.ID] {
		l := r.lots[id]
		l.ReservedCredits -= amount
		if !capture && !lotExpired(l, now) {
			l.AvailableCredits += amount
		}
		r.lots[id] = l
		if !capture && lotExpired(l, now) {
			if err := r.appendExpiryLocked(l, amount, "credit-lot-expiry:release:"+res.ID.String()+":"+id.String()); err != nil {
				return err
			}
		}
	}
	return nil
}

// Validate every possible terminal failure before appending any ledger entry.
// The mutex keeps this preflight and the terminal projection indivisible.
func (r *BillingRepo) preflightTerminalLotsLocked(res domain.CreditReservation, capture bool, now time.Time) error {
	var total, expired int64
	for id, amount := range r.allocations[res.ID] {
		l, ok := r.lots[id]
		if !ok || l.AccountID != res.AccountID || amount <= 0 || l.ReservedCredits < amount {
			return domain.ErrConflict
		}
		total += amount
		if !capture && lotExpired(l, now) {
			expired += amount
			if r.ledgerKeys["credit-lot-expiry:release:"+res.ID.String()+":"+id.String()] {
				return domain.ErrConflict
			}
		}
	}
	if total != res.Amount || r.accounts[res.AccountID].BalanceCached < math.MinInt64+expired {
		return domain.ErrConflict
	}
	return nil
}

// appendExpiryLocked writes the audit movement after its exact lot projection
// has been removed; generic negative adjustment attribution must not run again.
func (r *BillingRepo) appendExpiryLocked(l domain.CreditLot, amount int64, key string) error {
	e := &domain.LedgerEntry{AccountID: l.AccountID, Type: domain.LedgerAdjustment, Amount: -amount, Status: domain.LedgerStatusCommitted, IdempotencyKey: key, Reason: "credit lot expiry"}
	return r.appendProjectedLocked(e, false)
}

func (r *BillingRepo) expireAccountLotsLocked(account uuid.UUID, now time.Time, limit int) (int64, error) {
	// Preflight the complete bounded batch so collisions/overflow leave no
	// partially expired lot or unmatched ledger movement in this memory adapter.
	var selected int
	remainingBalance := r.accounts[account].BalanceCached
	for _, l := range r.sortedLotsLocked(account) {
		if limit > 0 && selected >= limit {
			break
		}
		if !lotExpired(l, now) || l.AvailableCredits <= 0 {
			continue
		}
		if r.ledgerKeys["credit-lot-expiry:"+l.ID.String()] || remainingBalance < math.MinInt64+l.AvailableCredits {
			return 0, domain.ErrConflict
		}
		remainingBalance -= l.AvailableCredits
		selected++
	}
	var n int64
	for _, l := range r.sortedLotsLocked(account) {
		if limit > 0 && n >= int64(limit) {
			break
		}
		if !lotExpired(l, now) || l.AvailableCredits <= 0 {
			continue
		}
		amount := l.AvailableCredits
		l.AvailableCredits = 0
		r.lots[l.ID] = l
		if err := r.appendExpiryLocked(l, amount, "credit-lot-expiry:"+l.ID.String()); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func (r *BillingRepo) ExpireCreditLots(_ context.Context, now time.Time, limit int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit <= 0 {
		return 0, nil
	}
	var total int64
	for account := range r.accounts {
		n, err := r.expireAccountLotsLocked(account, now, limit-int(total))
		total += n
		if err != nil || total >= int64(limit) {
			return total, err
		}
	}
	return total, nil
}

func (r *BillingRepo) CreditLotBalancesForOwner(_ context.Context, owner uuid.UUID, now time.Time) ([]domain.CreditLotBalance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.CreditLotBalance{}
	id, ok := r.byOwner[userCurrencyKey(owner, domain.CurrencyCredits)]
	if !ok {
		return out, nil
	}
	for _, l := range r.sortedLotsLocked(id) {
		available := l.AvailableCredits
		if lotExpired(l, now) && available > 0 {
			available = 0
		}
		if available == 0 && l.ReservedCredits == 0 {
			continue
		}
		out = append(out, domain.CreditLotBalance{ID: l.ID, Kind: l.Kind, AvailableCredits: available, ReservedCredits: l.ReservedCredits, ExpiresAt: copyLotTime(l.ExpiresAt)})
	}
	return out, nil
}

// RevokeCreditLotForGrant expires only the subscription lot created by the
// exact ledger grant. Spent credits and other lots are never charged again;
// active holds retain their original allocation until capture or late release.
func (r *BillingRepo) RevokeCreditLotForGrant(_ context.Context, owner uuid.UUID, grantLedgerKey string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if owner == uuid.Nil {
		return domain.ErrConflict
	}
	entry := r.ledgerByKeyLocked(grantLedgerKey)
	if entry == nil {
		return domain.ErrNotFound
	}
	if entry.OwnerAccountID != owner || r.accounts[entry.AccountID].OwnerAccountID != owner || entry.Type != domain.LedgerTopup || entry.Status != domain.LedgerStatusCommitted || entry.Amount <= 0 || entry.LotKind != domain.CreditLotSubscription {
		return domain.ErrConflict
	}
	lot, ok := r.lots[entry.ID]
	if !ok {
		return domain.ErrNotFound
	}
	if lot.AccountID != entry.AccountID || lot.Kind != domain.CreditLotSubscription {
		return domain.ErrConflict
	}
	if lot.ExpiresAt == nil || lot.ExpiresAt.After(now) {
		lot.ExpiresAt = copyLotTime(&now)
	}
	if lot.AvailableCredits > 0 {
		// Append first: any audit collision/overflow leaves the original lot intact.
		if err := r.appendExpiryLocked(lot, lot.AvailableCredits, "credit-lot-expiry:"+lot.ID.String()); err != nil {
			return err
		}
		lot.AvailableCredits = 0
	}
	r.lots[lot.ID] = lot
	return nil
}
