package domain

import (
	"context"
	"github.com/google/uuid"
	"time"
)

type AccountHistorySource int

const (
	AccountHistoryLedger AccountHistorySource = iota
	AccountHistoryPayment
	AccountHistoryRefund
	AccountHistoryExternalRefund
)

// AccountHistoryRow is a narrow private projection, never a raw ledger,
// payment or Job. It contains no provider identifiers, keys, reasons or input.
type AccountHistoryRow struct {
	ID                        uuid.UUID
	Source                    AccountHistorySource
	Kind                      string
	CreatedAt                 time.Time
	StarsDelta                int64
	CreditDenominationVersion int
	AmountMinor               *int64
	Currency                  Currency
	Status                    string
	PlanCode                  string
	Modality                  string
	Operation                 string
	TestMode                  *bool
}

type AccountHistoryCursor struct {
	CreatedAt time.Time
	Source    AccountHistorySource
	ID        uuid.UUID
}

type AccountHistoryRepository interface {
	// ListAccountHistory must apply owner to every branch and related-object join.
	// limit includes the service's one lookahead row and is bounded to 51.
	ListAccountHistory(context.Context, uuid.UUID, int, *AccountHistoryCursor) ([]AccountHistoryRow, error)
}
