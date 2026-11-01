package domain

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"strings"
	"time"
)

var ErrPromotionInvalid = errors.New("invalid promotion")
var ErrPromotionUnavailable = errors.New("promotion infrastructure unavailable")

func NormalizePromotionCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	if len(code) < 3 || len(code) > 64 {
		return "", ErrPromotionInvalid
	}
	for _, c := range []byte(code) {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			return "", ErrPromotionInvalid
		}
	}
	return strings.ToUpper(code), nil
}

// Decompose before multiplication to avoid overflowing int64 kopecks.
func PromotionAmount(base int64, percent int) (int64, error) {
	if base <= 0 || percent < 1 || percent > 99 {
		return 0, ErrPromotionInvalid
	}
	discount := (base/100)*int64(percent) + (base%100)*int64(percent)/100
	return base - discount, nil
}

// Promotion contains only safe operator configuration and aggregate counters.
type Promotion struct {
	ID            uuid.UUID  `json:"id"`
	Code          string     `json:"code"`
	Percent       int        `json:"percent"`
	Enabled       bool       `json:"enabled"`
	Revision      int64      `json:"revision"`
	StartsAt      time.Time  `json:"starts_at"`
	EndsAt        *time.Time `json:"ends_at,omitempty"`
	MaxUses       int64      `json:"max_uses"`
	PlanCodes     []string   `json:"plan_codes"`
	ReservedCount int64      `json:"reserved_count"`
	RedeemedCount int64      `json:"redeemed_count"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type PromotionSnapshot struct {
	PromotionID uuid.UUID `json:"promotion_id"`
	Code        string    `json:"code"`
	Revision    int64     `json:"revision"`
	Percent     int       `json:"percent"`
	BaseAmount  int64     `json:"base_amount"`
	Amount      int64     `json:"amount"`
}

type PromotionQuotePlan struct {
	PlanCode   string `json:"plan_code"`
	BaseAmount int64  `json:"base_amount"`
	Amount     int64  `json:"amount"`
	Currency   string `json:"currency"`
	Stars      int64  `json:"stars"`
}
type PromotionQuote struct {
	Code     string               `json:"code"`
	Revision int64                `json:"revision"`
	Percent  int                  `json:"percent"`
	Plans    []PromotionQuotePlan `json:"plans"`
}

// Mutations and all *Locked methods require the existing payment transaction.
// LockPromotionAccount MUST precede locking a promotion or its reservations.
type PromotionRepository interface {
	LockPromotionAccount(context.Context, uuid.UUID) error
	PromotionEligibleLocked(context.Context, uuid.UUID) (bool, error)
	GetPromotionLocked(context.Context, string) (*Promotion, error)
	ListPromotions(context.Context, int, int) ([]Promotion, error)
	ReservePromotionLocked(context.Context, uuid.UUID, uuid.UUID, PromotionSnapshot) error
	FinishPromotionLocked(context.Context, uuid.UUID, uuid.UUID, bool) error
	MutatePromotion(context.Context, string, uuid.UUID, string, *Promotion) (*Promotion, error)
}

// Split in-memory billing/payment stores supply the historical ledger check
// through billing; PostgreSQL performs it in PromotionEligibleLocked directly.
type PromotionGrantHistory interface {
	HasPromotionPaidGrant(context.Context, uuid.UUID) (bool, error)
}
