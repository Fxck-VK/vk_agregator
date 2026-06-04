package memory

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// BillingRepo is an in-memory domain.BillingRepository. It mirrors the ledger
// semantics of the PostgreSQL implementation: balance_cached changes only via
// committed ledger entries, and available balance subtracts active holds.
type BillingRepo struct {
	mu           sync.Mutex
	accounts     map[uuid.UUID]domain.CreditAccount
	byUser       map[string]uuid.UUID
	reservations map[uuid.UUID]domain.CreditReservation
	ledger       []domain.LedgerEntry
	ledgerKeys   map[string]bool
	resKeys      map[string]bool
}

// NewBillingRepo builds an empty BillingRepo.
func NewBillingRepo() *BillingRepo {
	return &BillingRepo{
		accounts:     map[uuid.UUID]domain.CreditAccount{},
		byUser:       map[string]uuid.UUID{},
		reservations: map[uuid.UUID]domain.CreditReservation{},
		ledgerKeys:   map[string]bool{},
		resKeys:      map[string]bool{},
	}
}

var _ domain.BillingRepository = (*BillingRepo)(nil)

func userCurrencyKey(userID uuid.UUID, currency domain.Currency) string {
	return userID.String() + "|" + string(currency)
}

func (r *BillingRepo) CreateAccount(_ context.Context, a *domain.CreditAccount) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := userCurrencyKey(a.UserID, a.Currency)
	if _, ok := r.byUser[key]; ok {
		return domain.ErrConflict
	}
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	now := time.Now()
	a.CreatedAt, a.UpdatedAt = now, now
	// The account always starts at zero; a requested starting balance is granted
	// through a committed opening ledger entry so balance_cached never diverges
	// from the ledger sum (invariant #14, audit B1).
	grant := a.BalanceCached
	a.BalanceCached = 0
	r.accounts[a.ID] = *a
	r.byUser[key] = a.ID
	if grant != 0 {
		if err := r.appendLocked(&domain.LedgerEntry{
			AccountID:      a.ID,
			Type:           domain.LedgerTopup,
			Amount:         grant,
			Status:         domain.LedgerStatusCommitted,
			IdempotencyKey: "grant:open:" + a.ID.String(),
			Reason:         "opening balance grant",
		}); err != nil {
			return err
		}
		a.BalanceCached = r.accounts[a.ID].BalanceCached
	}
	return nil
}

func (r *BillingRepo) GetAccount(_ context.Context, id uuid.UUID) (*domain.CreditAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.accounts[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &a, nil
}

func (r *BillingRepo) GetAccountByUser(_ context.Context, userID uuid.UUID, currency domain.Currency) (*domain.CreditAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byUser[userCurrencyKey(userID, currency)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	a := r.accounts[id]
	return &a, nil
}

func (r *BillingRepo) AppendEntry(_ context.Context, e *domain.LedgerEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.appendLocked(e)
}

func (r *BillingRepo) appendLocked(e *domain.LedgerEntry) error {
	if r.ledgerKeys[e.IdempotencyKey] {
		return domain.ErrConflict
	}
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.Status == "" {
		e.Status = domain.LedgerStatusCommitted
	}
	e.CreatedAt = time.Now()
	r.ledger = append(r.ledger, *e)
	r.ledgerKeys[e.IdempotencyKey] = true
	if e.Status == domain.LedgerStatusCommitted && e.Amount != 0 {
		acc := r.accounts[e.AccountID]
		acc.BalanceCached += e.Amount
		acc.UpdatedAt = time.Now()
		r.accounts[e.AccountID] = acc
	}
	return nil
}

func (r *BillingRepo) ListEntries(_ context.Context, accountID uuid.UUID, limit, offset int) ([]*domain.LedgerEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var filtered []domain.LedgerEntry
	for i := len(r.ledger) - 1; i >= 0; i-- {
		if r.ledger[i].AccountID == accountID {
			filtered = append(filtered, r.ledger[i])
		}
	}
	var out []*domain.LedgerEntry
	for i := offset; i < len(filtered) && len(out) < limit; i++ {
		e := filtered[i]
		out = append(out, &e)
	}
	return out, nil
}

// availableLocked returns balance minus the sum of active reservations.
func (r *BillingRepo) availableLocked(accountID uuid.UUID) int64 {
	avail := r.accounts[accountID].BalanceCached
	for _, res := range r.reservations {
		if res.AccountID == accountID && res.Status == domain.ReservationReserved {
			avail -= res.Amount
		}
	}
	return avail
}

func (r *BillingRepo) Reserve(_ context.Context, res *domain.CreditReservation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.resKeys[res.IdempotencyKey] {
		return domain.ErrConflict
	}
	if _, ok := r.accounts[res.AccountID]; !ok {
		return domain.ErrNotFound
	}
	if r.availableLocked(res.AccountID) < res.Amount {
		return domain.ErrInsufficientCredits
	}
	if res.ID == uuid.Nil {
		res.ID = uuid.New()
	}
	if res.Status == "" {
		res.Status = domain.ReservationReserved
	}
	now := time.Now()
	res.CreatedAt, res.UpdatedAt = now, now
	r.reservations[res.ID] = *res
	r.resKeys[res.IdempotencyKey] = true

	jobID := res.JobID
	return r.appendLocked(&domain.LedgerEntry{
		AccountID:      res.AccountID,
		JobID:          &jobID,
		ReservationID:  &res.ID,
		Type:           domain.LedgerReserve,
		Amount:         -res.Amount,
		Status:         domain.LedgerStatusPending,
		IdempotencyKey: "reserve:" + res.IdempotencyKey,
		Reason:         "credit reservation",
	})
}

func (r *BillingRepo) Capture(_ context.Context, reservationID uuid.UUID, amount int64, idempotencyKey string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	res, ok := r.reservations[reservationID]
	if !ok {
		return domain.ErrNotFound
	}
	if res.Status != domain.ReservationReserved {
		return domain.ErrConflict
	}
	res.Status = domain.ReservationCaptured
	res.UpdatedAt = time.Now()
	r.reservations[reservationID] = res

	jobID := res.JobID
	return r.appendLocked(&domain.LedgerEntry{
		AccountID:      res.AccountID,
		JobID:          &jobID,
		ReservationID:  &res.ID,
		Type:           domain.LedgerCapture,
		Amount:         -amount,
		Status:         domain.LedgerStatusCommitted,
		IdempotencyKey: idempotencyKey,
		Reason:         "credit capture",
	})
}

func (r *BillingRepo) Release(_ context.Context, reservationID uuid.UUID, idempotencyKey string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	res, ok := r.reservations[reservationID]
	if !ok {
		return domain.ErrNotFound
	}
	if res.Status != domain.ReservationReserved {
		return domain.ErrConflict
	}
	res.Status = domain.ReservationReleased
	res.UpdatedAt = time.Now()
	r.reservations[reservationID] = res

	jobID := res.JobID
	return r.appendLocked(&domain.LedgerEntry{
		AccountID:      res.AccountID,
		JobID:          &jobID,
		ReservationID:  &res.ID,
		Type:           domain.LedgerRelease,
		Amount:         0,
		Status:         domain.LedgerStatusCommitted,
		IdempotencyKey: idempotencyKey,
		Reason:         "credit release",
	})
}

func (r *BillingRepo) GetReservation(_ context.Context, id uuid.UUID) (*domain.CreditReservation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	res, ok := r.reservations[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &res, nil
}

func (r *BillingRepo) GetReservationByJob(_ context.Context, jobID uuid.UUID) (*domain.CreditReservation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var (
		latest domain.CreditReservation
		found  bool
	)
	for _, res := range r.reservations {
		if res.JobID != jobID {
			continue
		}
		if !found || res.CreatedAt.After(latest.CreatedAt) {
			latest, found = res, true
		}
	}
	if !found {
		return nil, domain.ErrNotFound
	}
	return &latest, nil
}
