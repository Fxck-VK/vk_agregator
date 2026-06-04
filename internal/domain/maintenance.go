package domain

import "github.com/google/uuid"

// BalanceMismatch reports a cached account balance that differs from the
// append-only committed ledger projection.
type BalanceMismatch struct {
	AccountID     uuid.UUID `json:"account_id"`
	UserID        uuid.UUID `json:"user_id"`
	Currency      Currency  `json:"currency"`
	BalanceCached int64     `json:"balance_cached"`
	LedgerBalance int64     `json:"ledger_balance"`
	Difference    int64     `json:"difference"`
}
