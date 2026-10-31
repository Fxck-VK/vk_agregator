package domain

import (
	"github.com/google/uuid"
	"time"
)

// CreditLotKind records the provenance of a grant without changing its units.
type CreditLotKind string

const (
	CreditLotLegacy       CreditLotKind = "legacy"
	CreditLotPurchased    CreditLotKind = "purchased"
	CreditLotSubscription CreditLotKind = "subscription"
	CreditLotBonus        CreditLotKind = "bonus"
)

func (k CreditLotKind) Valid() bool {
	return k == CreditLotLegacy || k == CreditLotPurchased || k == CreditLotSubscription || k == CreditLotBonus
}

// CreditLot is a ledger projection. AvailableCredits may be negative only on a
// non-expiring legacy adjustment lot, so old signed corrections retain debt
// without taking credits away from an already admitted job.
type CreditLot struct {
	ID               uuid.UUID
	AccountID        uuid.UUID
	Kind             CreditLotKind
	AvailableCredits int64
	ReservedCredits  int64
	ExpiresAt        *time.Time
	CreatedAt        time.Time
}

// CreditLotBalance is one owner-scoped lot snapshot. Expired available credits
// are excluded; reserved credits survive until their reservation terminates.
type CreditLotBalance struct {
	ID               uuid.UUID     `json:"id"`
	Kind             CreditLotKind `json:"kind"`
	AvailableCredits int64         `json:"available_credits"`
	ReservedCredits  int64         `json:"reserved_credits"`
	ExpiresAt        *time.Time    `json:"expires_at,omitempty"`
}
