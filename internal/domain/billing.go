package domain

import (
	"time"

	"github.com/google/uuid"
)

// Currency is the unit a credit account is denominated in.
type Currency string

const (
	// CurrencyCredits is the internal generation credit.
	CurrencyCredits Currency = "credits"
	// CurrencyRUB is Russian rubles.
	CurrencyRUB Currency = "rub"
	// CurrencyStars is the VK Stars currency.
	CurrencyStars Currency = "stars"
)

// LedgerEntryType is the kind of accounting movement an entry represents. The
// ledger is append-only (invariant #3) and balance is derived from entries.
type LedgerEntryType string

const (
	// LedgerTopup adds credits to the account.
	LedgerTopup LedgerEntryType = "topup"
	// LedgerReserve holds credits for an in-flight job.
	LedgerReserve LedgerEntryType = "reserve"
	// LedgerCapture converts a reservation into a final charge.
	LedgerCapture LedgerEntryType = "capture"
	// LedgerRelease frees a reservation without charging.
	LedgerRelease LedgerEntryType = "release"
	// LedgerRefund returns previously captured credits.
	LedgerRefund LedgerEntryType = "refund"
	// LedgerAdjustment is a manual correction by an administrator.
	LedgerAdjustment LedgerEntryType = "adjustment"
)

// Valid reports whether the ledger entry type is one of the known types.
func (t LedgerEntryType) Valid() bool {
	switch t {
	case LedgerTopup, LedgerReserve, LedgerCapture, LedgerRelease, LedgerRefund, LedgerAdjustment:
		return true
	default:
		return false
	}
}

// LedgerEntryStatus is the commit state of a ledger entry.
type LedgerEntryStatus string

const (
	// LedgerStatusPending means the entry is not yet committed.
	LedgerStatusPending LedgerEntryStatus = "pending"
	// LedgerStatusCommitted means the entry is final and counts toward balance.
	LedgerStatusCommitted LedgerEntryStatus = "committed"
	// LedgerStatusCancelled means the entry was voided before commit.
	LedgerStatusCancelled LedgerEntryStatus = "cancelled"
)

// ReservationStatus is the lifecycle state of a credit reservation.
type ReservationStatus string

const (
	// ReservationReserved means credits are held for a job.
	ReservationReserved ReservationStatus = "reserved"
	// ReservationCaptured means the hold was converted into a charge.
	ReservationCaptured ReservationStatus = "captured"
	// ReservationReleased means the hold was freed without charging.
	ReservationReleased ReservationStatus = "released"
	// ReservationExpired means the hold lapsed past its deadline.
	ReservationExpired ReservationStatus = "expired"
)

// CreditAccount is a user's balance for a single currency. balance_cached is a
// performance projection of the ledger and must never be mutated directly
// without a corresponding ledger entry (invariant #14).
type CreditAccount struct {
	// ID is the internal primary key.
	ID uuid.UUID `json:"id"`
	// UserID is the owner of the account.
	UserID uuid.UUID `json:"user_id"`
	// Currency is the unit the account is denominated in.
	Currency Currency `json:"currency"`
	// BalanceCached is the projected available balance from the ledger.
	BalanceCached int64 `json:"balance_cached"`
	// CreatedAt is the row creation timestamp.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the last mutation timestamp.
	UpdatedAt time.Time `json:"updated_at"`
}

// LedgerEntry is a single immutable accounting movement on an account. Entries
// are never updated in place except for their commit status.
type LedgerEntry struct {
	// ID is the internal primary key.
	ID uuid.UUID `json:"id"`
	// AccountID is the affected credit account.
	AccountID uuid.UUID `json:"account_id"`
	// JobID is the job this movement relates to, if any.
	JobID *uuid.UUID `json:"job_id,omitempty"`
	// ReservationID links capture/release entries to a reservation, if any.
	ReservationID *uuid.UUID `json:"reservation_id,omitempty"`
	// Type is the kind of movement.
	Type LedgerEntryType `json:"type"`
	// Amount is the signed magnitude in the account's currency. Positive adds
	// to the available balance, negative removes from it.
	Amount int64 `json:"amount"`
	// Status is the commit state of the entry.
	Status LedgerEntryStatus `json:"status"`
	// IdempotencyKey makes the entry safe to create exactly once.
	IdempotencyKey string `json:"idempotency_key"`
	// Reason is a human-readable explanation for audit.
	Reason string `json:"reason,omitempty"`
	// CreatedAt is the row creation timestamp.
	CreatedAt time.Time `json:"created_at"`
}

// CreditReservation is a hold placed on an account before a job runs. It is
// later captured (charged) or released (refunded) according to job outcome.
type CreditReservation struct {
	// ID is the internal primary key.
	ID uuid.UUID `json:"id"`
	// AccountID is the account the hold is placed on.
	AccountID uuid.UUID `json:"account_id"`
	// JobID is the job the reservation is for.
	JobID uuid.UUID `json:"job_id"`
	// Amount is the held amount in the account's currency.
	Amount int64 `json:"amount"`
	// Status is the lifecycle state of the reservation.
	Status ReservationStatus `json:"status"`
	// IdempotencyKey makes the reservation safe to create exactly once.
	IdempotencyKey string `json:"idempotency_key"`
	// ExpiresAt is when an unresolved reservation should be auto-released.
	ExpiresAt time.Time `json:"expires_at"`
	// CreatedAt is the row creation timestamp.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the last mutation timestamp.
	UpdatedAt time.Time `json:"updated_at"`
}
