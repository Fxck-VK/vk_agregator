// Package billingservice implements credit accounting on top of the append-only
// ledger exposed by domain.BillingRepository. It owns price estimation and the
// reserve/capture/release/refund lifecycle so that callers (the job
// orchestrator, workers) never touch the ledger directly.
package billingservice

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// DefaultStartingBalance is the amount of test credits granted to a freshly
// created account.
const DefaultStartingBalance int64 = 1000

// DefaultReservationTTL bounds how long a hold may live before it is eligible
// for automatic release.
const DefaultReservationTTL = 30 * time.Minute

// ErrUnknownOperation is returned by Estimate when no price is configured for an
// operation type.
var ErrUnknownOperation = errors.New("billingservice: unknown operation type")

// defaultPrices is the seed price list, in credits, per operation type. Per the
// product spec image_to_video shares the video price.
var defaultPrices = map[domain.OperationType]int64{
	domain.OperationTextGenerate:      1,
	domain.OperationImageGenerate:     10,
	domain.OperationImageEdit:         10,
	domain.OperationVideoGenerate:     50,
	domain.OperationVideoImageToVideo: 50,
}

// Service provides credit estimation and lifecycle operations.
type Service struct {
	repo            domain.BillingRepository
	prices          map[domain.OperationType]int64
	startingBalance int64
	currency        domain.Currency
	reservationTTL  time.Duration
	now             func() time.Time
}

// Option customizes a Service.
type Option func(*Service)

// WithStartingBalance overrides the starting balance for new accounts.
func WithStartingBalance(credits int64) Option {
	return func(s *Service) { s.startingBalance = credits }
}

// WithPrices replaces the default price list.
func WithPrices(prices map[domain.OperationType]int64) Option {
	return func(s *Service) { s.prices = prices }
}

// WithClock overrides the time source (useful in tests).
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// New builds a Service over the given billing repository.
func New(repo domain.BillingRepository, opts ...Option) *Service {
	s := &Service{
		repo:            repo,
		prices:          clonePrices(defaultPrices),
		startingBalance: DefaultStartingBalance,
		currency:        domain.CurrencyCredits,
		reservationTTL:  DefaultReservationTTL,
		now:             time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Estimate returns the credit cost of an operation, or ErrUnknownOperation.
func (s *Service) Estimate(op domain.OperationType) (int64, error) {
	price, ok := s.prices[op]
	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrUnknownOperation, op)
	}
	return price, nil
}

// EnsureAccount returns the user's credit account, creating it with the
// starting balance if it does not yet exist.
func (s *Service) EnsureAccount(ctx context.Context, userID uuid.UUID) (*domain.CreditAccount, error) {
	acc, err := s.repo.GetAccountByUser(ctx, userID, s.currency)
	if err == nil {
		return acc, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	acc = &domain.CreditAccount{
		UserID:        userID,
		Currency:      s.currency,
		BalanceCached: s.startingBalance,
	}
	if err := s.repo.CreateAccount(ctx, acc); err != nil {
		// A concurrent creation may have won the race; re-read in that case.
		if errors.Is(err, domain.ErrConflict) {
			return s.repo.GetAccountByUser(ctx, userID, s.currency)
		}
		return nil, err
	}
	return acc, nil
}

// Reserve holds amount credits for a job, returning ErrInsufficientCredits when
// the available balance is too low. The reservation is keyed by the job so the
// call is idempotent per job.
func (s *Service) Reserve(ctx context.Context, userID, jobID uuid.UUID, amount int64) (*domain.CreditReservation, error) {
	acc, err := s.EnsureAccount(ctx, userID)
	if err != nil {
		return nil, err
	}
	res := &domain.CreditReservation{
		AccountID:      acc.ID,
		JobID:          jobID,
		Amount:         amount,
		Status:         domain.ReservationReserved,
		IdempotencyKey: "resv:" + jobID.String(),
		ExpiresAt:      s.now().Add(s.reservationTTL),
	}
	if err := s.repo.Reserve(ctx, res); err != nil {
		return nil, err
	}
	return res, nil
}

// Capture converts a reservation into a committed charge.
func (s *Service) Capture(ctx context.Context, reservationID uuid.UUID, amount int64) error {
	return s.repo.Capture(ctx, reservationID, amount, "cap:"+reservationID.String())
}

// CaptureForJob captures the job's reservation by amount. It is idempotent: an
// already-captured reservation is treated as success, so a re-delivered
// delivery never double-charges.
func (s *Service) CaptureForJob(ctx context.Context, jobID uuid.UUID, amount int64) error {
	res, err := s.repo.GetReservationByJob(ctx, jobID)
	if err != nil {
		return err
	}
	if res.Status == domain.ReservationCaptured {
		return nil
	}
	if err := s.repo.Capture(ctx, res.ID, amount, "cap:"+res.ID.String()); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil
		}
		return err
	}
	return nil
}

// Release frees a reservation without charging the account.
func (s *Service) Release(ctx context.Context, reservationID uuid.UUID) error {
	return s.repo.Release(ctx, reservationID, "rel:"+reservationID.String())
}

// Refund returns previously captured credits to the user's account by appending
// a committed positive ledger entry. It is idempotent per job.
func (s *Service) Refund(ctx context.Context, userID, jobID uuid.UUID, amount int64) error {
	acc, err := s.EnsureAccount(ctx, userID)
	if err != nil {
		return err
	}
	entry := &domain.LedgerEntry{
		AccountID:      acc.ID,
		JobID:          &jobID,
		Type:           domain.LedgerRefund,
		Amount:         amount,
		Status:         domain.LedgerStatusCommitted,
		IdempotencyKey: "refund:" + jobID.String(),
		Reason:         "job refund",
	}
	return s.repo.AppendEntry(ctx, entry)
}

func clonePrices(in map[domain.OperationType]int64) map[domain.OperationType]int64 {
	out := make(map[domain.OperationType]int64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
