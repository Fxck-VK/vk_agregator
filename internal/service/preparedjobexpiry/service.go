// Package preparedjobexpiry durably finalizes browser image preparations whose
// explicit confirmation window elapsed before activation. It has no billing,
// outbox, queue, provider, or delivery dependency.
package preparedjobexpiry

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	// DefaultGlobalReconcileInterval keeps the background pass bounded while
	// ensuring abandoned confirmations do not remain visible indefinitely.
	DefaultGlobalReconcileInterval = time.Minute
	// DefaultGlobalReconcileLimit bounds one cross-account database claim.
	DefaultGlobalReconcileLimit = 100
	// DefaultAccountReconcileLimit bounds the proactive account cleanup before
	// browser history reads. Targeted reconciliation closes any residual race.
	DefaultAccountReconcileLimit = 10
)

// Repository atomically expires only due browser image preparation rows. An
// accountID of nil selects one global bounded page; a non-nil accountID selects
// only that exact canonical account.
type Repository interface {
	ExpireDuePreparedWebImages(ctx context.Context, accountID *uuid.UUID, now time.Time, limit int) (expired int, hasMore bool, err error)
	ExpireDuePreparedWebImage(ctx context.Context, accountID, jobID uuid.UUID, now time.Time) (changed bool, err error)
}

// Result contains only bounded operational counts; it deliberately has no job
// identifiers, prompts, account data, or private routing information.
type Result struct {
	Expired int
	HasMore bool
}

// Service coordinates repository-owned atomic expiry mutations.
type Service struct {
	repository Repository
	now        func() time.Time
}

// Option customizes a Service.
type Option func(*Service)

// WithClock supplies a deterministic clock for tests.
func WithClock(now func() time.Time) Option {
	return func(service *Service) {
		if now != nil {
			service.now = now
		}
	}
}

// New constructs a prepared-image expiry service.
func New(repository Repository, options ...Option) *Service {
	service := &Service{repository: repository, now: time.Now}
	for _, option := range options {
		option(service)
	}
	return service
}

// Reconcile expires one bounded cross-account page. Repository implementations
// must claim rows atomically so independently running API instances never apply
// duplicate transitions.
func (s *Service) Reconcile(ctx context.Context, limit int) (Result, error) {
	return s.reconcile(ctx, nil, limit)
}

// ReconcileAccount expires one bounded due page only for the exact account.
// It is used before browser reads and never falls back to legacy user ids.
func (s *Service) ReconcileAccount(ctx context.Context, accountID uuid.UUID, limit int) (Result, error) {
	if accountID == uuid.Nil {
		return Result{}, errors.New("preparedjobexpiry: account id is required")
	}
	return s.reconcile(ctx, &accountID, limit)
}

// ReconcileJob atomically expires one exact account-owned job if it is still a
// due web image preparation. It closes the read-after-account-page race without
// performing a cross-account scan.
func (s *Service) ReconcileJob(ctx context.Context, accountID, jobID uuid.UUID) (bool, error) {
	if s == nil || s.repository == nil {
		return false, errors.New("preparedjobexpiry: repository is required")
	}
	if accountID == uuid.Nil || jobID == uuid.Nil {
		return false, errors.New("preparedjobexpiry: account and job ids are required")
	}
	return s.repository.ExpireDuePreparedWebImage(ctx, accountID, jobID, s.now())
}

func (s *Service) reconcile(ctx context.Context, accountID *uuid.UUID, limit int) (Result, error) {
	if s == nil || s.repository == nil {
		return Result{}, errors.New("preparedjobexpiry: repository is required")
	}
	if limit <= 0 {
		return Result{}, errors.New("preparedjobexpiry: limit must be positive")
	}
	expired, hasMore, err := s.repository.ExpireDuePreparedWebImages(ctx, accountID, s.now(), limit)
	if err != nil {
		return Result{}, err
	}
	return Result{Expired: expired, HasMore: hasMore}, nil
}

// Run performs one bounded reconciliation immediately and on each interval
// until ctx is cancelled. It deliberately does not retry in a tight loop: an
// outage must not cause a hot database loop, and the next interval is a fresh
// independent claim across all API instances.
func (s *Service) Run(ctx context.Context, interval time.Duration, limit int, onError func(error)) {
	if interval <= 0 {
		interval = DefaultGlobalReconcileInterval
	}
	if limit <= 0 {
		limit = DefaultGlobalReconcileLimit
	}
	reconcile := func() {
		if _, err := s.Reconcile(ctx, limit); err != nil && onError != nil {
			onError(err)
		}
	}
	reconcile()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reconcile()
		}
	}
}
