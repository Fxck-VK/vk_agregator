package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vk-ai-aggregator/internal/domain"
)

// BillingRepository is the PostgreSQL implementation of domain.BillingRepository.
//
// Accounting model:
//   - balance_cached is the total owned credits and equals the sum of all
//     committed ledger entries.
//   - A reservation is a soft hold: it is recorded as a pending ledger entry
//     (which does not move balance_cached) and gates spending via the available
//     balance = balance_cached - sum(active reservations).
//   - Capture turns a hold into a committed charge (balance_cached decreases);
//     Release frees the hold without charging.
//
// A BillingRepository can run either standalone (over a pool, opening its own
// transaction for atomic mutations) or transaction-bound (over a Querier that is
// already a pgx.Tx), letting reservations compose with job creation in a single
// transaction (audit B1).
type BillingRepository struct {
	pool *pgxpool.Pool
	db   Querier
}

// NewBillingRepository builds a standalone BillingRepository over the given
// pool. Atomic mutations open their own transaction.
func NewBillingRepository(pool *pgxpool.Pool) *BillingRepository {
	return &BillingRepository{pool: pool}
}

// NewBillingRepositoryTx builds a transaction-bound BillingRepository over a
// caller-managed querier (a pgx.Tx). Mutations run directly on that querier so
// they commit or roll back with the surrounding unit of work (audit B1).
func NewBillingRepositoryTx(db Querier) *BillingRepository {
	return &BillingRepository{db: db}
}

var _ domain.BillingRepository = (*BillingRepository)(nil)

// q returns the querier used for reads: the pool when standalone, otherwise the
// transaction-bound querier.
func (r *BillingRepository) q() Querier {
	if r.pool != nil {
		return r.pool
	}
	return r.db
}

// inTx runs fn with a transaction-scoped querier. In standalone mode it opens a
// new transaction; in transaction-bound mode it reuses the caller's querier so
// the work joins the surrounding unit of work.
func (r *BillingRepository) inTx(ctx context.Context, fn func(q Querier) error) error {
	if r.pool != nil {
		return RunInTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
			return fn(tx)
		})
	}
	return fn(r.db)
}

const accountColumns = `id, user_id, owner_account_id, currency, balance_cached,
	credit_denomination_version, created_at, updated_at`
const creditAccountOwnerFilter = `(owner_account_id = $1 OR (owner_account_id IS NULL AND user_id = $1))`

const reservationColumns = `id, account_id, owner_account_id, job_id, amount, credit_denomination_version, status, idempotency_key,
	expires_at, created_at, updated_at`

const ledgerColumns = `id, account_id, owner_account_id, job_id, reservation_id, type, amount, credit_denomination_version, status,
	idempotency_key, reason, created_at`

// CreateAccount inserts a new credit account. The account is always created
// with a zero cached balance; any requested starting balance is granted through
// a committed opening ledger entry in the same transaction, so balance_cached
// never diverges from the ledger sum (invariant #14, audit B1).
func (r *BillingRepository) CreateAccount(ctx context.Context, a *domain.CreditAccount) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.Currency == "" {
		a.Currency = domain.CurrencyCredits
	}
	if a.CreditDenominationVersion == 0 {
		a.CreditDenominationVersion = domain.CurrentCreditDenominationVersion
	}
	grant := a.BalanceCached
	return r.inTx(ctx, func(q Querier) error {
		const insAcc = `
			INSERT INTO credit_accounts (id, user_id, owner_account_id, currency, balance_cached, credit_denomination_version)
			VALUES ($1, $2, COALESCE($3::uuid, (SELECT account_id FROM users WHERE id = $2)), $4, 0, $5)
			ON CONFLICT (owner_account_id, currency) WHERE owner_account_id IS NOT NULL DO NOTHING
			RETURNING ` + accountColumns
		if err := scanAccount(q.QueryRow(ctx, insAcc, a.ID, nullableUUID(a.UserID), nullableUUID(a.OwnerAccountID), a.Currency, a.CreditDenominationVersion), a); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// The exact owner/currency partial unique index won the race.
				// DO NOTHING keeps a caller-managed transaction usable so the
				// service can re-read the canonical account.
				return domain.ErrConflict
			}
			return mapError(err)
		}
		if grant == 0 {
			return nil
		}
		// Opening grant: a committed ledger entry that establishes the starting
		// balance, keyed uniquely per account so it is created exactly once.
		entry := &domain.LedgerEntry{
			AccountID:                 a.ID,
			OwnerAccountID:            a.OwnerAccountID,
			Type:                      domain.LedgerTopup,
			Amount:                    grant,
			CreditDenominationVersion: a.CreditDenominationVersion,
			Status:                    domain.LedgerStatusCommitted,
			IdempotencyKey:            "grant:open:" + a.ID.String(),
			Reason:                    "opening balance grant",
		}
		inserted, err := insertLedgerEntry(ctx, q, entry)
		if err != nil {
			return err
		}
		if !inserted {
			return nil
		}
		if err := adjustBalance(ctx, q, a.ID, grant); err != nil {
			return err
		}
		a.BalanceCached = grant
		return nil
	})
}

// GetAccount fetches an account by id.
func (r *BillingRepository) GetAccount(ctx context.Context, id uuid.UUID) (*domain.CreditAccount, error) {
	const q = `SELECT ` + accountColumns + ` FROM credit_accounts WHERE id = $1`
	var a domain.CreditAccount
	if err := mapError(scanAccount(r.q().QueryRow(ctx, q, id), &a)); err != nil {
		return nil, err
	}
	return &a, nil
}

// GetAccountByUser fetches a user's account for a currency.
func (r *BillingRepository) GetAccountByUser(ctx context.Context, userID uuid.UUID, currency domain.Currency) (*domain.CreditAccount, error) {
	q := `SELECT ` + accountColumns + ` FROM credit_accounts WHERE ` + creditAccountOwnerFilter + ` AND currency = $2`
	var a domain.CreditAccount
	if err := mapError(scanAccount(r.q().QueryRow(ctx, q, userID, currency), &a)); err != nil {
		return nil, err
	}
	return &a, nil
}

// GetAccountByOwner fetches an account-native credit account by its canonical
// owner and currency without considering legacy user provenance.
func (r *BillingRepository) GetAccountByOwner(ctx context.Context, ownerAccountID uuid.UUID, currency domain.Currency) (*domain.CreditAccount, error) {
	q := `SELECT ` + accountColumns + ` FROM credit_accounts WHERE owner_account_id = $1 AND currency = $2`
	var a domain.CreditAccount
	if err := mapError(scanAccount(r.q().QueryRow(ctx, q, ownerAccountID, currency), &a)); err != nil {
		return nil, err
	}
	return &a, nil
}

// AppendEntry inserts an immutable ledger entry, adjusting the cached balance by
// the entry amount when the entry is committed.
func (r *BillingRepository) AppendEntry(ctx context.Context, entry *domain.LedgerEntry) error {
	return r.inTx(ctx, func(q Querier) error {
		inserted, err := insertLedgerEntry(ctx, q, entry)
		if err != nil {
			return err
		}
		if inserted && entry.Status == domain.LedgerStatusCommitted && entry.Amount != 0 {
			return adjustBalance(ctx, q, entry.AccountID, entry.Amount)
		}
		return nil
	})
}

// ListEntries returns ledger entries for an account, newest first.
func (r *BillingRepository) ListEntries(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*domain.LedgerEntry, error) {
	const q = `SELECT ` + ledgerColumns + `
		FROM ledger_entries WHERE account_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.q().Query(ctx, q, accountID, limit, offset)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var entries []*domain.LedgerEntry
	for rows.Next() {
		var e domain.LedgerEntry
		if err := scanLedgerEntry(rows, &e); err != nil {
			return nil, mapError(err)
		}
		entries = append(entries, &e)
	}
	return entries, mapError(rows.Err())
}

// Reserve creates a hold and its pending ledger entry atomically, failing with
// ErrInsufficientCredits when the available balance is too low. It remains the
// legacy-compatible wrapper; the stored owner is still derived from the credit
// account and a supplied non-zero owner is verified rather than trusted.
func (r *BillingRepository) Reserve(ctx context.Context, res *domain.CreditReservation) error {
	return r.reserve(ctx, uuid.Nil, res, false)
}

// ReserveForOwner creates a reservation only for the supplied canonical owner.
func (r *BillingRepository) ReserveForOwner(ctx context.Context, ownerAccountID uuid.UUID, res *domain.CreditReservation) error {
	if ownerAccountID == uuid.Nil {
		return domain.ErrConflict
	}
	return r.reserve(ctx, ownerAccountID, res, true)
}

func (r *BillingRepository) reserve(ctx context.Context, requestedOwner uuid.UUID, res *domain.CreditReservation, ownerScoped bool) error {
	return r.inTx(ctx, func(q Querier) error {
		accountOwner, err := lockCreditAccountOwner(ctx, q, res.AccountID)
		if err != nil {
			return err
		}
		if requestedOwner != uuid.Nil && requestedOwner != accountOwner {
			return domain.ErrConflict
		}
		if res.OwnerAccountID != uuid.Nil && res.OwnerAccountID != accountOwner {
			return domain.ErrConflict
		}
		if ownerScoped && accountOwner == uuid.Nil {
			return domain.ErrConflict
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
		if existing, err := reservationByIdempotencyKey(ctx, q, res.IdempotencyKey); err == nil {
			if sameReservationPayload(existing, accountOwner, res) {
				*res = *existing
				return nil
			}
			return domain.ErrConflict
		} else if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		available, err := availableBalanceLocked(ctx, q, res.AccountID)
		if err != nil {
			return err
		}
		if available < res.Amount {
			return domain.ErrInsufficientCredits
		}
		const insRes = `
			INSERT INTO credit_reservations (id, account_id, owner_account_id, job_id, amount, credit_denomination_version, status, idempotency_key, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (idempotency_key) DO NOTHING
			RETURNING ` + reservationColumns
		row := q.QueryRow(ctx, insRes,
			res.ID, res.AccountID, nullableUUID(accountOwner), res.JobID, res.Amount, res.CreditDenominationVersion, res.Status, res.IdempotencyKey, res.ExpiresAt,
		)
		if err := scanReservation(row, res); err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				return mapError(err)
			}
			existing, lookupErr := reservationByIdempotencyKey(ctx, q, res.IdempotencyKey)
			if lookupErr != nil {
				return lookupErr
			}
			if sameReservationPayload(existing, accountOwner, res) {
				*res = *existing
				return nil
			}
			return domain.ErrConflict
		}
		res.OwnerAccountID = accountOwner
		entry := &domain.LedgerEntry{
			AccountID:                 res.AccountID,
			OwnerAccountID:            res.OwnerAccountID,
			JobID:                     &res.JobID,
			ReservationID:             &res.ID,
			Type:                      domain.LedgerReserve,
			Amount:                    -res.Amount,
			CreditDenominationVersion: res.CreditDenominationVersion,
			Status:                    domain.LedgerStatusPending,
			IdempotencyKey:            "reserve:" + res.IdempotencyKey,
			Reason:                    "credit reservation",
		}
		inserted, err := insertLedgerEntry(ctx, q, entry)
		if err != nil {
			return err
		}
		if inserted {
			return nil
		}
		// A pre-existing ledger idempotency key must not leave a new reservation
		// behind without its linked reserve entry, even in a caller-owned tx.
		if _, err := q.Exec(ctx, `DELETE FROM credit_reservations WHERE id = $1`, res.ID); err != nil {
			return mapError(err)
		}
		return domain.ErrConflict
	})
}

// Capture converts a reservation into a committed charge.
func (r *BillingRepository) Capture(ctx context.Context, reservationID uuid.UUID, amount int64, idempotencyKey string) error {
	return r.capture(ctx, uuid.Nil, reservationID, amount, idempotencyKey, false)
}

func (r *BillingRepository) CaptureForOwner(ctx context.Context, ownerAccountID, reservationID uuid.UUID, amount int64, idempotencyKey string) error {
	if ownerAccountID == uuid.Nil {
		return domain.ErrConflict
	}
	return r.capture(ctx, ownerAccountID, reservationID, amount, idempotencyKey, true)
}

func (r *BillingRepository) capture(ctx context.Context, ownerAccountID, reservationID uuid.UUID, amount int64, idempotencyKey string, ownerScoped bool) error {
	return r.inTx(ctx, func(q Querier) error {
		res, err := lockReservation(ctx, q, reservationID)
		if err != nil {
			return err
		}
		if ownerScoped && res.OwnerAccountID != ownerAccountID {
			return domain.ErrConflict
		}
		if amount <= 0 || amount != res.Amount {
			return domain.ErrConflict
		}
		if res.Status == domain.ReservationCaptured {
			entry, err := ledgerByIdempotencyKey(ctx, q, idempotencyKey)
			if err == nil && sameTerminalLedger(entry, res, domain.LedgerCapture, -amount, idempotencyKey) {
				return nil
			}
			if errors.Is(err, domain.ErrNotFound) {
				return domain.ErrConflict
			}
			return err
		}
		if res.Status != domain.ReservationReserved {
			return domain.ErrConflict
		}
		entry := &domain.LedgerEntry{
			AccountID:                 res.AccountID,
			OwnerAccountID:            res.OwnerAccountID,
			JobID:                     &res.JobID,
			ReservationID:             &res.ID,
			Type:                      domain.LedgerCapture,
			Amount:                    -amount,
			CreditDenominationVersion: res.CreditDenominationVersion,
			Status:                    domain.LedgerStatusCommitted,
			IdempotencyKey:            idempotencyKey,
			Reason:                    "credit capture",
		}
		inserted, err := insertLedgerEntry(ctx, q, entry)
		if err != nil {
			return err
		}
		if !inserted {
			return domain.ErrConflict
		}
		if _, err := q.Exec(ctx,
			`UPDATE credit_reservations SET status = $2, updated_at = now() WHERE id = $1`,
			reservationID, domain.ReservationCaptured,
		); err != nil {
			return mapError(err)
		}
		return adjustBalance(ctx, q, res.AccountID, -amount)
	})
}

// Release frees a reservation without charging the account.
func (r *BillingRepository) Release(ctx context.Context, reservationID uuid.UUID, idempotencyKey string) error {
	return r.release(ctx, uuid.Nil, reservationID, idempotencyKey, false)
}

func (r *BillingRepository) ReleaseForOwner(ctx context.Context, ownerAccountID, reservationID uuid.UUID, idempotencyKey string) error {
	if ownerAccountID == uuid.Nil {
		return domain.ErrConflict
	}
	return r.release(ctx, ownerAccountID, reservationID, idempotencyKey, true)
}

func (r *BillingRepository) release(ctx context.Context, ownerAccountID, reservationID uuid.UUID, idempotencyKey string, ownerScoped bool) error {
	return r.inTx(ctx, func(q Querier) error {
		res, err := lockReservation(ctx, q, reservationID)
		if err != nil {
			return err
		}
		if ownerScoped && res.OwnerAccountID != ownerAccountID {
			return domain.ErrConflict
		}
		if res.Status == domain.ReservationReleased {
			entry, err := ledgerByIdempotencyKey(ctx, q, idempotencyKey)
			if err == nil && sameTerminalLedger(entry, res, domain.LedgerRelease, 0, idempotencyKey) {
				return nil
			}
			if errors.Is(err, domain.ErrNotFound) {
				return domain.ErrConflict
			}
			return err
		}
		if res.Status != domain.ReservationReserved {
			return domain.ErrConflict
		}
		entry := &domain.LedgerEntry{
			AccountID:                 res.AccountID,
			OwnerAccountID:            res.OwnerAccountID,
			JobID:                     &res.JobID,
			ReservationID:             &res.ID,
			Type:                      domain.LedgerRelease,
			Amount:                    0,
			CreditDenominationVersion: res.CreditDenominationVersion,
			Status:                    domain.LedgerStatusCommitted,
			IdempotencyKey:            idempotencyKey,
			Reason:                    "credit release",
		}
		inserted, err := insertLedgerEntry(ctx, q, entry)
		if err != nil {
			return err
		}
		if !inserted {
			return domain.ErrConflict
		}
		if _, err := q.Exec(ctx,
			`UPDATE credit_reservations SET status = $2, updated_at = now() WHERE id = $1`,
			reservationID, domain.ReservationReleased,
		); err != nil {
			return mapError(err)
		}
		return nil
	})
}

// GetReservation fetches a reservation by id.
func (r *BillingRepository) GetReservation(ctx context.Context, id uuid.UUID) (*domain.CreditReservation, error) {
	const q = `SELECT ` + reservationColumns + ` FROM credit_reservations WHERE id = $1`
	var res domain.CreditReservation
	if err := mapError(scanReservation(r.q().QueryRow(ctx, q, id), &res)); err != nil {
		return nil, err
	}
	return &res, nil
}

// GetReservationByJob fetches the most recent reservation for a job.
func (r *BillingRepository) GetReservationByJob(ctx context.Context, jobID uuid.UUID) (*domain.CreditReservation, error) {
	const q = `SELECT ` + reservationColumns + `
		FROM credit_reservations WHERE job_id = $1
		ORDER BY created_at DESC LIMIT 1`
	var res domain.CreditReservation
	if err := mapError(scanReservation(r.q().QueryRow(ctx, q, jobID), &res)); err != nil {
		return nil, err
	}
	return &res, nil
}

// lockCreditAccountOwner serializes reservations for one credit account and
// returns the authoritative canonical owner from the selected row.
func lockCreditAccountOwner(ctx context.Context, q Querier, accountID uuid.UUID) (uuid.UUID, error) {
	const sql = `SELECT owner_account_id FROM credit_accounts WHERE id = $1 FOR UPDATE`
	var ownerAccountID *uuid.UUID
	if err := q.QueryRow(ctx, sql, accountID).Scan(&ownerAccountID); err != nil {
		return uuid.Nil, mapError(err)
	}
	if ownerAccountID == nil {
		return uuid.Nil, nil
	}
	return *ownerAccountID, nil
}

// availableBalanceLocked requires the credit account row to be locked by the
// caller. Keeping the replay lookup before this check makes exact idempotency
// replays valid even when later reservations have consumed available credits.
func availableBalanceLocked(ctx context.Context, q Querier, accountID uuid.UUID) (int64, error) {
	const sql = `
		SELECT c.balance_cached - COALESCE((
			SELECT SUM(amount) FROM credit_reservations
			WHERE account_id = c.id AND status = 'reserved'
		), 0)
		FROM credit_accounts c
		WHERE c.id = $1`
	var available int64
	if err := q.QueryRow(ctx, sql, accountID).Scan(&available); err != nil {
		return 0, mapError(err)
	}
	return available, nil
}

func lockReservation(ctx context.Context, q Querier, id uuid.UUID) (*domain.CreditReservation, error) {
	const sql = `SELECT ` + reservationColumns + ` FROM credit_reservations WHERE id = $1 FOR UPDATE`
	var res domain.CreditReservation
	if err := mapError(scanReservation(q.QueryRow(ctx, sql, id), &res)); err != nil {
		return nil, err
	}
	return &res, nil
}

func reservationByIdempotencyKey(ctx context.Context, q Querier, key string) (*domain.CreditReservation, error) {
	const sql = `SELECT ` + reservationColumns + ` FROM credit_reservations WHERE idempotency_key = $1`
	var reservation domain.CreditReservation
	if err := mapError(scanReservation(q.QueryRow(ctx, sql, key), &reservation)); err != nil {
		return nil, err
	}
	return &reservation, nil
}

func sameReservationPayload(existing *domain.CreditReservation, ownerAccountID uuid.UUID, requested *domain.CreditReservation) bool {
	if existing == nil || requested == nil {
		return false
	}
	denomination := requested.CreditDenominationVersion
	if denomination == 0 {
		denomination = domain.CurrentCreditDenominationVersion
	}
	return existing.OwnerAccountID == ownerAccountID &&
		existing.AccountID == requested.AccountID &&
		existing.JobID == requested.JobID &&
		existing.Amount == requested.Amount &&
		existing.CreditDenominationVersion == denomination
}

func ledgerByIdempotencyKey(ctx context.Context, q Querier, key string) (*domain.LedgerEntry, error) {
	const sql = `SELECT ` + ledgerColumns + ` FROM ledger_entries WHERE idempotency_key = $1`
	var entry domain.LedgerEntry
	if err := mapError(scanLedgerEntry(q.QueryRow(ctx, sql, key), &entry)); err != nil {
		return nil, err
	}
	return &entry, nil
}

func sameTerminalLedger(entry *domain.LedgerEntry, reservation *domain.CreditReservation, entryType domain.LedgerEntryType, amount int64, key string) bool {
	return entry != nil && reservation != nil && entry.IdempotencyKey == key &&
		entry.AccountID == reservation.AccountID && entry.OwnerAccountID == reservation.OwnerAccountID &&
		entry.JobID != nil && *entry.JobID == reservation.JobID &&
		entry.ReservationID != nil && *entry.ReservationID == reservation.ID &&
		entry.Type == entryType && entry.Amount == amount &&
		entry.CreditDenominationVersion == reservation.CreditDenominationVersion &&
		entry.Status == domain.LedgerStatusCommitted
}

func insertLedgerEntry(ctx context.Context, q Querier, e *domain.LedgerEntry) (bool, error) {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.Status == "" {
		e.Status = domain.LedgerStatusCommitted
	}
	if e.CreditDenominationVersion == 0 {
		e.CreditDenominationVersion = domain.CurrentCreditDenominationVersion
	}
	const sql = `
		INSERT INTO ledger_entries (id, account_id, owner_account_id, job_id, reservation_id, type, amount, credit_denomination_version, status, idempotency_key, reason)
		VALUES ($1, $2, COALESCE($3::uuid, (SELECT owner_account_id FROM credit_accounts WHERE id = $2)), $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING ` + ledgerColumns
	row := q.QueryRow(ctx, sql,
		e.ID, e.AccountID, nullableUUID(e.OwnerAccountID), e.JobID, e.ReservationID, e.Type, e.Amount, e.CreditDenominationVersion, e.Status, e.IdempotencyKey, e.Reason,
	)
	if err := scanLedgerEntry(row, e); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, mapError(err)
	}
	return true, nil
}

func adjustBalance(ctx context.Context, q Querier, accountID uuid.UUID, delta int64) error {
	_, err := q.Exec(ctx,
		`UPDATE credit_accounts SET balance_cached = balance_cached + $2, updated_at = now() WHERE id = $1`,
		accountID, delta,
	)
	return mapError(err)
}

func scanAccount(row rowScanner, a *domain.CreditAccount) error {
	var legacyUserID *uuid.UUID
	var ownerAccountID *uuid.UUID
	if err := row.Scan(&a.ID, &legacyUserID, &ownerAccountID, &a.Currency, &a.BalanceCached, &a.CreditDenominationVersion, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return err
	}
	a.UserID = uuid.Nil
	a.OwnerAccountID = uuid.Nil
	if legacyUserID != nil {
		a.UserID = *legacyUserID
	}
	if ownerAccountID != nil {
		a.OwnerAccountID = *ownerAccountID
	}
	return nil
}

func scanReservation(row rowScanner, res *domain.CreditReservation) error {
	var ownerAccountID *uuid.UUID
	res.OwnerAccountID = uuid.Nil
	if err := row.Scan(
		&res.ID, &res.AccountID, &ownerAccountID, &res.JobID, &res.Amount, &res.CreditDenominationVersion, &res.Status, &res.IdempotencyKey,
		&res.ExpiresAt, &res.CreatedAt, &res.UpdatedAt,
	); err != nil {
		return err
	}
	if ownerAccountID != nil {
		res.OwnerAccountID = *ownerAccountID
	}
	return nil
}

func scanLedgerEntry(row rowScanner, e *domain.LedgerEntry) error {
	var ownerAccountID *uuid.UUID
	e.OwnerAccountID = uuid.Nil
	e.JobID = nil
	e.ReservationID = nil
	if err := row.Scan(
		&e.ID, &e.AccountID, &ownerAccountID, &e.JobID, &e.ReservationID, &e.Type, &e.Amount, &e.CreditDenominationVersion, &e.Status,
		&e.IdempotencyKey, &e.Reason, &e.CreatedAt,
	); err != nil {
		return err
	}
	if ownerAccountID != nil {
		e.OwnerAccountID = *ownerAccountID
	}
	return nil
}
