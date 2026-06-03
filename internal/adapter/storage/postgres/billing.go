package postgres

import (
	"context"

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
// Mutations that must be atomic open their own transaction, so this repository
// holds a connection pool rather than a bare Querier.
type BillingRepository struct {
	pool *pgxpool.Pool
}

// NewBillingRepository builds a BillingRepository over the given pool.
func NewBillingRepository(pool *pgxpool.Pool) *BillingRepository {
	return &BillingRepository{pool: pool}
}

var _ domain.BillingRepository = (*BillingRepository)(nil)

const accountColumns = `id, user_id, currency, balance_cached, created_at, updated_at`

const reservationColumns = `id, account_id, job_id, amount, status, idempotency_key,
	expires_at, created_at, updated_at`

const ledgerColumns = `id, account_id, job_id, reservation_id, type, amount, status,
	idempotency_key, reason, created_at`

// CreateAccount inserts a new credit account.
func (r *BillingRepository) CreateAccount(ctx context.Context, a *domain.CreditAccount) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.Currency == "" {
		a.Currency = domain.CurrencyCredits
	}
	const q = `
		INSERT INTO credit_accounts (id, user_id, currency, balance_cached)
		VALUES ($1, $2, $3, $4)
		RETURNING ` + accountColumns
	return mapError(scanAccount(r.pool.QueryRow(ctx, q, a.ID, a.UserID, a.Currency, a.BalanceCached), a))
}

// GetAccount fetches an account by id.
func (r *BillingRepository) GetAccount(ctx context.Context, id uuid.UUID) (*domain.CreditAccount, error) {
	const q = `SELECT ` + accountColumns + ` FROM credit_accounts WHERE id = $1`
	var a domain.CreditAccount
	if err := mapError(scanAccount(r.pool.QueryRow(ctx, q, id), &a)); err != nil {
		return nil, err
	}
	return &a, nil
}

// GetAccountByUser fetches a user's account for a currency.
func (r *BillingRepository) GetAccountByUser(ctx context.Context, userID uuid.UUID, currency domain.Currency) (*domain.CreditAccount, error) {
	const q = `SELECT ` + accountColumns + ` FROM credit_accounts WHERE user_id = $1 AND currency = $2`
	var a domain.CreditAccount
	if err := mapError(scanAccount(r.pool.QueryRow(ctx, q, userID, currency), &a)); err != nil {
		return nil, err
	}
	return &a, nil
}

// AppendEntry inserts an immutable ledger entry, adjusting the cached balance by
// the entry amount when the entry is committed.
func (r *BillingRepository) AppendEntry(ctx context.Context, entry *domain.LedgerEntry) error {
	return RunInTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := insertLedgerEntry(ctx, tx, entry); err != nil {
			return err
		}
		if entry.Status == domain.LedgerStatusCommitted && entry.Amount != 0 {
			return adjustBalance(ctx, tx, entry.AccountID, entry.Amount)
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
	rows, err := r.pool.Query(ctx, q, accountID, limit, offset)
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
// ErrInsufficientCredits when the available balance is too low.
func (r *BillingRepository) Reserve(ctx context.Context, res *domain.CreditReservation) error {
	if res.ID == uuid.Nil {
		res.ID = uuid.New()
	}
	if res.Status == "" {
		res.Status = domain.ReservationReserved
	}
	return RunInTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		available, err := availableBalance(ctx, tx, res.AccountID)
		if err != nil {
			return err
		}
		if available < res.Amount {
			return domain.ErrInsufficientCredits
		}
		const insRes = `
			INSERT INTO credit_reservations (id, account_id, job_id, amount, status, idempotency_key, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING ` + reservationColumns
		row := tx.QueryRow(ctx, insRes,
			res.ID, res.AccountID, res.JobID, res.Amount, res.Status, res.IdempotencyKey, res.ExpiresAt,
		)
		if err := mapError(scanReservation(row, res)); err != nil {
			return err
		}
		entry := &domain.LedgerEntry{
			AccountID:      res.AccountID,
			JobID:          &res.JobID,
			ReservationID:  &res.ID,
			Type:           domain.LedgerReserve,
			Amount:         -res.Amount,
			Status:         domain.LedgerStatusPending,
			IdempotencyKey: "reserve:" + res.IdempotencyKey,
			Reason:         "credit reservation",
		}
		return insertLedgerEntry(ctx, tx, entry)
	})
}

// Capture converts a reservation into a committed charge.
func (r *BillingRepository) Capture(ctx context.Context, reservationID uuid.UUID, amount int64, idempotencyKey string) error {
	return RunInTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		res, err := lockReservation(ctx, tx, reservationID)
		if err != nil {
			return err
		}
		if res.Status != domain.ReservationReserved {
			return domain.ErrConflict
		}
		if _, err := tx.Exec(ctx,
			`UPDATE credit_reservations SET status = $2, updated_at = now() WHERE id = $1`,
			reservationID, domain.ReservationCaptured,
		); err != nil {
			return mapError(err)
		}
		entry := &domain.LedgerEntry{
			AccountID:      res.AccountID,
			JobID:          &res.JobID,
			ReservationID:  &res.ID,
			Type:           domain.LedgerCapture,
			Amount:         -amount,
			Status:         domain.LedgerStatusCommitted,
			IdempotencyKey: idempotencyKey,
			Reason:         "credit capture",
		}
		if err := insertLedgerEntry(ctx, tx, entry); err != nil {
			return err
		}
		return adjustBalance(ctx, tx, res.AccountID, -amount)
	})
}

// Release frees a reservation without charging the account.
func (r *BillingRepository) Release(ctx context.Context, reservationID uuid.UUID, idempotencyKey string) error {
	return RunInTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		res, err := lockReservation(ctx, tx, reservationID)
		if err != nil {
			return err
		}
		if res.Status != domain.ReservationReserved {
			return domain.ErrConflict
		}
		if _, err := tx.Exec(ctx,
			`UPDATE credit_reservations SET status = $2, updated_at = now() WHERE id = $1`,
			reservationID, domain.ReservationReleased,
		); err != nil {
			return mapError(err)
		}
		entry := &domain.LedgerEntry{
			AccountID:      res.AccountID,
			JobID:          &res.JobID,
			ReservationID:  &res.ID,
			Type:           domain.LedgerRelease,
			Amount:         0,
			Status:         domain.LedgerStatusCommitted,
			IdempotencyKey: idempotencyKey,
			Reason:         "credit release",
		}
		return insertLedgerEntry(ctx, tx, entry)
	})
}

// GetReservation fetches a reservation by id.
func (r *BillingRepository) GetReservation(ctx context.Context, id uuid.UUID) (*domain.CreditReservation, error) {
	const q = `SELECT ` + reservationColumns + ` FROM credit_reservations WHERE id = $1`
	var res domain.CreditReservation
	if err := mapError(scanReservation(r.pool.QueryRow(ctx, q, id), &res)); err != nil {
		return nil, err
	}
	return &res, nil
}

// availableBalance returns balance_cached minus the sum of active holds for an
// account, locking the account row for the duration of the transaction.
func availableBalance(ctx context.Context, tx pgx.Tx, accountID uuid.UUID) (int64, error) {
	const q = `
		SELECT c.balance_cached - COALESCE((
			SELECT SUM(amount) FROM credit_reservations
			WHERE account_id = c.id AND status = 'reserved'
		), 0)
		FROM credit_accounts c
		WHERE c.id = $1
		FOR UPDATE`
	var available int64
	if err := tx.QueryRow(ctx, q, accountID).Scan(&available); err != nil {
		return 0, mapError(err)
	}
	return available, nil
}

func lockReservation(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.CreditReservation, error) {
	const q = `SELECT ` + reservationColumns + ` FROM credit_reservations WHERE id = $1 FOR UPDATE`
	var res domain.CreditReservation
	if err := mapError(scanReservation(tx.QueryRow(ctx, q, id), &res)); err != nil {
		return nil, err
	}
	return &res, nil
}

func insertLedgerEntry(ctx context.Context, tx pgx.Tx, e *domain.LedgerEntry) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.Status == "" {
		e.Status = domain.LedgerStatusCommitted
	}
	const q = `
		INSERT INTO ledger_entries (id, account_id, job_id, reservation_id, type, amount, status, idempotency_key, reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING ` + ledgerColumns
	row := tx.QueryRow(ctx, q,
		e.ID, e.AccountID, e.JobID, e.ReservationID, e.Type, e.Amount, e.Status, e.IdempotencyKey, e.Reason,
	)
	return mapError(scanLedgerEntry(row, e))
}

func adjustBalance(ctx context.Context, tx pgx.Tx, accountID uuid.UUID, delta int64) error {
	_, err := tx.Exec(ctx,
		`UPDATE credit_accounts SET balance_cached = balance_cached + $2, updated_at = now() WHERE id = $1`,
		accountID, delta,
	)
	return mapError(err)
}

func scanAccount(row rowScanner, a *domain.CreditAccount) error {
	return row.Scan(&a.ID, &a.UserID, &a.Currency, &a.BalanceCached, &a.CreatedAt, &a.UpdatedAt)
}

func scanReservation(row rowScanner, res *domain.CreditReservation) error {
	return row.Scan(
		&res.ID, &res.AccountID, &res.JobID, &res.Amount, &res.Status, &res.IdempotencyKey,
		&res.ExpiresAt, &res.CreatedAt, &res.UpdatedAt,
	)
}

func scanLedgerEntry(row rowScanner, e *domain.LedgerEntry) error {
	return row.Scan(
		&e.ID, &e.AccountID, &e.JobID, &e.ReservationID, &e.Type, &e.Amount, &e.Status,
		&e.IdempotencyKey, &e.Reason, &e.CreatedAt,
	)
}
