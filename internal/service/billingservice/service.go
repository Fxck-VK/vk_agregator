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

// DefaultStartingBalance is the amount of credits granted to a freshly created
// account.
const DefaultStartingBalance int64 = 30

// DefaultReservationTTL bounds how long a hold may live before it is eligible
// for automatic release.
const DefaultReservationTTL = 30 * time.Minute

// ErrUnknownOperation is returned by Estimate when no price is configured for an
// operation type.
var ErrUnknownOperation = errors.New("billingservice: unknown operation type")

// ErrInvalidAmount is returned when a configured price or accounting operation
// amount is zero or negative where a positive credit amount is required.
var ErrInvalidAmount = errors.New("billingservice: amount must be positive")

// defaultPrices is a legacy fallback price list for uncataloged/old jobs.
// Cataloged generation products must reserve from pricingcatalog snapshots.
// Per the product spec image_to_video shares the video fallback.
var defaultPrices = map[domain.OperationType]int64{
	domain.OperationTextGenerate:      0,
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

// WithPriceOverrides merges legacy per-operation overrides onto the fallback
// price list. It must not be used as the runtime source for cataloged
// image/video generation pricing.
func WithPriceOverrides(overrides map[string]int64) Option {
	return func(s *Service) {
		for op, amount := range overrides {
			s.prices[domain.OperationType(op)] = amount
		}
	}
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
	if price < 0 || (price == 0 && op != domain.OperationTextGenerate) {
		return 0, fmt.Errorf("%w: price for %s is %d", ErrInvalidAmount, op, price)
	}
	return price, nil
}

// StartingBalance returns the configured grant for a new account without
// creating the account or writing the opening ledger entry.
func (s *Service) StartingBalance() int64 {
	return s.startingBalance
}

// BalanceForEstimate returns the current cached balance for estimate-only
// reads. If the account does not exist yet, it returns the configured starting
// balance without creating a ledger entry. The ownerID is the canonical account
// id; repositories still provide legacy user_id fallback for old rows.
func (s *Service) BalanceForEstimate(ctx context.Context, ownerID uuid.UUID) (int64, error) {
	acc, err := s.repo.GetAccountByUser(ctx, ownerID, s.currency)
	if err == nil {
		return acc.BalanceCached, nil
	}
	if errors.Is(err, domain.ErrNotFound) {
		return s.startingBalance, nil
	}
	return 0, err
}

// EnsureAccount returns the canonical credit account, creating it with the
// starting balance if it does not yet exist.
func (s *Service) EnsureAccount(ctx context.Context, userID uuid.UUID) (*domain.CreditAccount, error) {
	return s.ensureAccountWith(ctx, s.repo, userID, uuid.Nil)
}

// EnsureAccountForOwner returns the canonical credit account for accountID
// while retaining legacy userID as channel metadata for schema compatibility.
func (s *Service) EnsureAccountForOwner(ctx context.Context, userID, accountID uuid.UUID) (*domain.CreditAccount, error) {
	return s.ensureAccountWith(ctx, s.repo, userID, accountID)
}

// ensureAccountWith resolves or creates the owner's account using the supplied
// repository, which may be transaction-bound so account creation joins the
// caller's unit of work.
func (s *Service) ensureAccountWith(ctx context.Context, repo domain.BillingRepository, userID, accountID uuid.UUID) (*domain.CreditAccount, error) {
	ownerID := accountOwnerID(userID, accountID)
	acc, err := repo.GetAccountByUser(ctx, ownerID, s.currency)
	if err == nil {
		if acc.OwnerAccountID == uuid.Nil {
			acc.OwnerAccountID = ownerID
		}
		return acc, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	acc = &domain.CreditAccount{
		UserID:         userID,
		OwnerAccountID: accountID,
		Currency:       s.currency,
		BalanceCached:  s.startingBalance,
	}
	if err := repo.CreateAccount(ctx, acc); err != nil {
		// A concurrent creation may have won the race; re-read in that case.
		if errors.Is(err, domain.ErrConflict) {
			return repo.GetAccountByUser(ctx, ownerID, s.currency)
		}
		return nil, err
	}
	return acc, nil
}

// Reserve holds amount credits for a job, returning ErrInsufficientCredits when
// the available balance is too low. The reservation is keyed by the job so the
// call is idempotent per job.
func (s *Service) Reserve(ctx context.Context, userID, jobID uuid.UUID, amount int64) (*domain.CreditReservation, error) {
	return s.ReserveWith(ctx, s.repo, userID, jobID, amount)
}

// ReserveWith holds credits using the supplied repository. Passing a
// transaction-bound billing repository lets the reservation commit atomically
// with job creation (audit B1). The reservation is keyed by the job, so the call
// is idempotent per job.
func (s *Service) ReserveWith(ctx context.Context, repo domain.BillingRepository, userID, jobID uuid.UUID, amount int64) (*domain.CreditReservation, error) {
	return s.ReserveWithOwner(ctx, repo, userID, uuid.Nil, jobID, amount)
}

// ReserveWithOwner holds credits for the canonical account owner while keeping
// the legacy user id on the credit account for old foreign-key relationships.
func (s *Service) ReserveWithOwner(ctx context.Context, repo domain.BillingRepository, userID, accountID, jobID uuid.UUID, amount int64) (*domain.CreditReservation, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("%w: reservation amount %d", ErrInvalidAmount, amount)
	}
	acc, err := s.ensureAccountWith(ctx, repo, userID, accountID)
	if err != nil {
		return nil, err
	}
	res := &domain.CreditReservation{
		AccountID:      acc.ID,
		OwnerAccountID: creditAccountOwnerID(acc, accountOwnerID(userID, accountID)),
		JobID:          jobID,
		Amount:         amount,
		Status:         domain.ReservationReserved,
		IdempotencyKey: "resv:" + jobID.String(),
		ExpiresAt:      s.now().Add(s.reservationTTL),
	}
	if err := repo.Reserve(ctx, res); err != nil {
		return nil, err
	}
	return res, nil
}

// Capture converts a reservation into a committed charge.
func (s *Service) Capture(ctx context.Context, reservationID uuid.UUID, amount int64) error {
	if amount <= 0 {
		return fmt.Errorf("%w: capture amount %d", ErrInvalidAmount, amount)
	}
	return s.repo.Capture(ctx, reservationID, amount, "cap:"+reservationID.String())
}

// CaptureForJob captures the job's reservation by amount. It is idempotent: an
// already-captured reservation is treated as success, so a re-delivered
// delivery never double-charges.
func (s *Service) CaptureForJob(ctx context.Context, jobID uuid.UUID, amount int64) error {
	if amount <= 0 {
		return fmt.Errorf("%w: capture amount %d", ErrInvalidAmount, amount)
	}
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

// ReleaseForJob frees the job's reservation without charging it. It is
// idempotent: a reservation that is no longer in the reserved state (already
// captured/released) is treated as success. Used when output moderation blocks
// delivery so reserved credits are not held or charged.
func (s *Service) ReleaseForJob(ctx context.Context, jobID uuid.UUID) error {
	res, err := s.repo.GetReservationByJob(ctx, jobID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return err
	}
	if res.Status != domain.ReservationReserved {
		return nil
	}
	if err := s.repo.Release(ctx, res.ID, "rel:"+res.ID.String()); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil
		}
		return err
	}
	return nil
}

// Refund returns previously captured credits to the owner's account by appending
// a committed positive ledger entry. It is idempotent per job.
func (s *Service) Refund(ctx context.Context, userID, jobID uuid.UUID, amount int64) error {
	return s.RefundForOwner(ctx, userID, uuid.Nil, jobID, amount)
}

// RefundForOwner returns previously captured credits to the canonical account.
func (s *Service) RefundForOwner(ctx context.Context, userID, accountID, jobID uuid.UUID, amount int64) error {
	ownerID := accountOwnerID(userID, accountID)
	acc, err := s.EnsureAccountForOwner(ctx, userID, accountID)
	if err != nil {
		return err
	}
	entry := &domain.LedgerEntry{
		AccountID:      acc.ID,
		OwnerAccountID: creditAccountOwnerID(acc, ownerID),
		JobID:          &jobID,
		Type:           domain.LedgerRefund,
		Amount:         amount,
		Status:         domain.LedgerStatusCommitted,
		IdempotencyKey: "refund:" + jobID.String(),
		Reason:         "job refund",
	}
	return s.repo.AppendEntry(ctx, entry)
}

// Grant appends an idempotent positive top-up entry using the service's
// repository. It is used for non-payment bonuses such as referral rewards and
// delegates to GrantWith for the actual ledger write.
func (s *Service) Grant(ctx context.Context, userID uuid.UUID, amount int64, idempotencyKey, reason string) error {
	return s.GrantWith(ctx, s.repo, userID, amount, idempotencyKey, reason)
}

// GrantWith appends an idempotent positive top-up entry using the supplied
// repository. Passing a transaction-bound billing repository lets payment
// webhook processing commit payment event processing, intent status and the
// ledger top-up atomically in one caller-owned transaction.
func (s *Service) GrantWith(ctx context.Context, repo domain.BillingRepository, userID uuid.UUID, amount int64, idempotencyKey, reason string) error {
	return s.GrantWithOwner(ctx, repo, userID, uuid.Nil, amount, idempotencyKey, reason)
}

// GrantWithOwner appends an idempotent top-up for the canonical account owner.
func (s *Service) GrantWithOwner(ctx context.Context, repo domain.BillingRepository, userID, accountID uuid.UUID, amount int64, idempotencyKey, reason string) error {
	if amount <= 0 {
		return nil
	}
	if idempotencyKey == "" {
		return errors.New("billingservice: grant idempotency key is required")
	}
	ownerID := accountOwnerID(userID, accountID)
	acc, err := s.ensureAccountWith(ctx, repo, userID, accountID)
	if err != nil {
		return err
	}
	entry := &domain.LedgerEntry{
		AccountID:      acc.ID,
		OwnerAccountID: creditAccountOwnerID(acc, ownerID),
		Type:           domain.LedgerTopup,
		Amount:         amount,
		Status:         domain.LedgerStatusCommitted,
		IdempotencyKey: idempotencyKey,
		Reason:         reason,
	}
	if err := repo.AppendEntry(ctx, entry); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil
		}
		return err
	}
	return nil
}

func accountOwnerID(userID, accountID uuid.UUID) uuid.UUID {
	if accountID != uuid.Nil {
		return accountID
	}
	return userID
}

func creditAccountOwnerID(acc *domain.CreditAccount, fallback uuid.UUID) uuid.UUID {
	if acc != nil && acc.OwnerAccountID != uuid.Nil {
		return acc.OwnerAccountID
	}
	return fallback
}

func clonePrices(in map[domain.OperationType]int64) map[domain.OperationType]int64 {
	out := make(map[domain.OperationType]int64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
