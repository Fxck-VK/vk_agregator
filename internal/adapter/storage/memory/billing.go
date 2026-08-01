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
	mu            sync.Mutex
	accounts      map[uuid.UUID]domain.CreditAccount
	byUser        map[string]uuid.UUID
	byOwner       map[string]uuid.UUID
	claimedUsers  map[string]bool
	claimedOwners map[string]bool
	reservations  map[uuid.UUID]domain.CreditReservation
	ledger        []domain.LedgerEntry
	ledgerKeys    map[string]bool
	resKeys       map[string]uuid.UUID
}

// NewBillingRepo builds an empty BillingRepo.
func NewBillingRepo() *BillingRepo {
	return &BillingRepo{
		accounts:      map[uuid.UUID]domain.CreditAccount{},
		byUser:        map[string]uuid.UUID{},
		byOwner:       map[string]uuid.UUID{},
		claimedUsers:  map[string]bool{},
		claimedOwners: map[string]bool{},
		reservations:  map[uuid.UUID]domain.CreditReservation{},
		ledgerKeys:    map[string]bool{},
		resKeys:       map[string]uuid.UUID{},
	}
}

var _ domain.BillingRepository = (*BillingRepo)(nil)

func userCurrencyKey(userID uuid.UUID, currency domain.Currency) string {
	return userID.String() + "|" + string(currency)
}

func (r *BillingRepo) CreateAccount(_ context.Context, a *domain.CreditAccount) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a.OwnerAccountID == uuid.Nil {
		a.OwnerAccountID = a.UserID
	}
	if a.CreditDenominationVersion == 0 {
		a.CreditDenominationVersion = domain.CurrentCreditDenominationVersion
	}
	var legacyKey, ownerKey string
	if a.UserID != uuid.Nil {
		legacyKey = userCurrencyKey(a.UserID, a.Currency)
		if r.claimedUsers[legacyKey] || r.claimedOwners[legacyKey] {
			return domain.ErrConflict
		}
	}
	if a.OwnerAccountID != uuid.Nil {
		ownerKey = userCurrencyKey(a.OwnerAccountID, a.Currency)
		if r.claimedOwners[ownerKey] || r.claimedUsers[ownerKey] {
			return domain.ErrConflict
		}
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
	if legacyKey != "" {
		r.claimedUsers[legacyKey] = true
	}
	if ownerKey != "" {
		r.claimedOwners[ownerKey] = true
		// GetAccountByUser is a legacy compatibility lookup that has long
		// resolved the effective owner key. Keep that public behavior intact.
		r.byUser[ownerKey] = a.ID
		r.byOwner[ownerKey] = a.ID
	}
	if grant != 0 {
		if err := r.appendLocked(&domain.LedgerEntry{
			AccountID:                 a.ID,
			OwnerAccountID:            a.OwnerAccountID,
			Type:                      domain.LedgerTopup,
			Amount:                    grant,
			CreditDenominationVersion: a.CreditDenominationVersion,
			Status:                    domain.LedgerStatusCommitted,
			IdempotencyKey:            "grant:open:" + a.ID.String(),
			Reason:                    "opening balance grant",
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

// GetAccountByOwner fetches only by the canonical owner/currency key.
func (r *BillingRepo) GetAccountByOwner(_ context.Context, ownerAccountID uuid.UUID, currency domain.Currency) (*domain.CreditAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byOwner[userCurrencyKey(ownerAccountID, currency)]
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
	if e.CreditDenominationVersion == 0 {
		e.CreditDenominationVersion = domain.CurrentCreditDenominationVersion
	}
	if e.OwnerAccountID == uuid.Nil {
		if acc, ok := r.accounts[e.AccountID]; ok {
			e.OwnerAccountID = acc.OwnerAccountID
		}
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

func (r *BillingRepo) Reserve(ctx context.Context, res *domain.CreditReservation) error {
	return r.reserve(ctx, uuid.Nil, res, false)
}

// ReserveForOwner creates a reservation only for the supplied canonical owner.
// A supplied stored owner must match that canonical owner; the stored owner is
// then copied from the selected credit account.
func (r *BillingRepo) ReserveForOwner(ctx context.Context, ownerAccountID uuid.UUID, res *domain.CreditReservation) error {
	if ownerAccountID == uuid.Nil {
		return domain.ErrConflict
	}
	return r.reserve(ctx, ownerAccountID, res, true)
}

func (r *BillingRepo) reserve(_ context.Context, requestedOwner uuid.UUID, res *domain.CreditReservation, ownerScoped bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	account, ok := r.accounts[res.AccountID]
	if !ok {
		return domain.ErrNotFound
	}
	if requestedOwner != uuid.Nil && requestedOwner != account.OwnerAccountID {
		return domain.ErrConflict
	}
	if res.OwnerAccountID != uuid.Nil && res.OwnerAccountID != account.OwnerAccountID {
		return domain.ErrConflict
	}
	if ownerScoped && account.OwnerAccountID == uuid.Nil {
		return domain.ErrConflict
	}
	if existingID, ok := r.resKeys[res.IdempotencyKey]; ok {
		existing := r.reservations[existingID]
		if existing.OwnerAccountID == account.OwnerAccountID &&
			existing.AccountID == res.AccountID &&
			existing.JobID == res.JobID &&
			existing.Amount == res.Amount &&
			existing.CreditDenominationVersion == reservationDenomination(res) {
			*res = existing
			return nil
		}
		return domain.ErrConflict
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
	if res.CreditDenominationVersion == 0 {
		res.CreditDenominationVersion = domain.CurrentCreditDenominationVersion
	}
	res.OwnerAccountID = account.OwnerAccountID
	// Preflight the linked ledger idempotency key so a collision cannot leave a
	// new reservation without its pending reserve ledger entry.
	if r.ledgerKeys["reserve:"+res.IdempotencyKey] {
		return domain.ErrConflict
	}
	now := time.Now()
	res.CreatedAt, res.UpdatedAt = now, now
	r.reservations[res.ID] = *res

	jobID := res.JobID
	if err := r.appendLocked(&domain.LedgerEntry{
		AccountID:                 res.AccountID,
		OwnerAccountID:            res.OwnerAccountID,
		JobID:                     &jobID,
		ReservationID:             &res.ID,
		Type:                      domain.LedgerReserve,
		Amount:                    -res.Amount,
		CreditDenominationVersion: res.CreditDenominationVersion,
		Status:                    domain.LedgerStatusPending,
		IdempotencyKey:            "reserve:" + res.IdempotencyKey,
		Reason:                    "credit reservation",
	}); err != nil {
		delete(r.reservations, res.ID)
		return err
	}
	r.resKeys[res.IdempotencyKey] = res.ID
	return nil
}

func (r *BillingRepo) Capture(ctx context.Context, reservationID uuid.UUID, amount int64, idempotencyKey string) error {
	return r.capture(ctx, uuid.Nil, reservationID, amount, idempotencyKey, false)
}

func (r *BillingRepo) CaptureForOwner(ctx context.Context, ownerAccountID, reservationID uuid.UUID, amount int64, idempotencyKey string) error {
	if ownerAccountID == uuid.Nil {
		return domain.ErrConflict
	}
	return r.capture(ctx, ownerAccountID, reservationID, amount, idempotencyKey, true)
}

func (r *BillingRepo) capture(_ context.Context, ownerAccountID, reservationID uuid.UUID, amount int64, idempotencyKey string, ownerScoped bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	res, ok := r.reservations[reservationID]
	if !ok {
		return domain.ErrNotFound
	}
	if ownerScoped && res.OwnerAccountID != ownerAccountID {
		return domain.ErrConflict
	}
	if amount <= 0 || amount != res.Amount {
		return domain.ErrConflict
	}
	if res.Status == domain.ReservationCaptured {
		if ledgerMatches(r.ledgerByKeyLocked(idempotencyKey), res, domain.LedgerCapture, -amount, idempotencyKey) {
			return nil
		}
		return domain.ErrConflict
	}
	if res.Status != domain.ReservationReserved || r.ledgerKeys[idempotencyKey] {
		return domain.ErrConflict
	}

	jobID := res.JobID
	if err := r.appendLocked(&domain.LedgerEntry{
		AccountID:                 res.AccountID,
		OwnerAccountID:            res.OwnerAccountID,
		JobID:                     &jobID,
		ReservationID:             &res.ID,
		Type:                      domain.LedgerCapture,
		Amount:                    -amount,
		CreditDenominationVersion: res.CreditDenominationVersion,
		Status:                    domain.LedgerStatusCommitted,
		IdempotencyKey:            idempotencyKey,
		Reason:                    "credit capture",
	}); err != nil {
		return err
	}
	res.Status = domain.ReservationCaptured
	res.UpdatedAt = time.Now()
	r.reservations[reservationID] = res
	return nil
}

func (r *BillingRepo) Release(ctx context.Context, reservationID uuid.UUID, idempotencyKey string) error {
	return r.release(ctx, uuid.Nil, reservationID, idempotencyKey, false)
}

func (r *BillingRepo) ReleaseForOwner(ctx context.Context, ownerAccountID, reservationID uuid.UUID, idempotencyKey string) error {
	if ownerAccountID == uuid.Nil {
		return domain.ErrConflict
	}
	return r.release(ctx, ownerAccountID, reservationID, idempotencyKey, true)
}

func (r *BillingRepo) release(_ context.Context, ownerAccountID, reservationID uuid.UUID, idempotencyKey string, ownerScoped bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	res, ok := r.reservations[reservationID]
	if !ok {
		return domain.ErrNotFound
	}
	if ownerScoped && res.OwnerAccountID != ownerAccountID {
		return domain.ErrConflict
	}
	if res.Status == domain.ReservationReleased {
		if ledgerMatches(r.ledgerByKeyLocked(idempotencyKey), res, domain.LedgerRelease, 0, idempotencyKey) {
			return nil
		}
		return domain.ErrConflict
	}
	if res.Status != domain.ReservationReserved || r.ledgerKeys[idempotencyKey] {
		return domain.ErrConflict
	}

	jobID := res.JobID
	if err := r.appendLocked(&domain.LedgerEntry{
		AccountID:                 res.AccountID,
		OwnerAccountID:            res.OwnerAccountID,
		JobID:                     &jobID,
		ReservationID:             &res.ID,
		Type:                      domain.LedgerRelease,
		Amount:                    0,
		CreditDenominationVersion: res.CreditDenominationVersion,
		Status:                    domain.LedgerStatusCommitted,
		IdempotencyKey:            idempotencyKey,
		Reason:                    "credit release",
	}); err != nil {
		return err
	}
	res.Status = domain.ReservationReleased
	res.UpdatedAt = time.Now()
	r.reservations[reservationID] = res
	return nil
}

func reservationDenomination(res *domain.CreditReservation) int {
	if res.CreditDenominationVersion == 0 {
		return domain.CurrentCreditDenominationVersion
	}
	return res.CreditDenominationVersion
}

func (r *BillingRepo) ledgerByKeyLocked(key string) *domain.LedgerEntry {
	for i := range r.ledger {
		if r.ledger[i].IdempotencyKey == key {
			entry := r.ledger[i]
			return &entry
		}
	}
	return nil
}

func ledgerMatches(entry *domain.LedgerEntry, res domain.CreditReservation, entryType domain.LedgerEntryType, amount int64, key string) bool {
	return entry != nil && entry.IdempotencyKey == key && entry.AccountID == res.AccountID &&
		entry.OwnerAccountID == res.OwnerAccountID && entry.Type == entryType && entry.Amount == amount &&
		entry.ReservationID != nil && *entry.ReservationID == res.ID
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
