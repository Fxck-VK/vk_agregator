// Package memory provides in-memory implementations of the domain repository
// interfaces. They are intended for unit tests and local development, mirroring
// the semantics of the PostgreSQL adapters (idempotency conflicts, optimistic
// status transitions, ledger-based balances) without an external database.
package memory

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/uow"
)

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

// UserRepo is an in-memory domain.UserRepository.
type UserRepo struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]domain.User
	byVKID map[int64]uuid.UUID
}

// NewUserRepo builds an empty UserRepo.
func NewUserRepo() *UserRepo {
	return &UserRepo{byID: map[uuid.UUID]domain.User{}, byVKID: map[int64]uuid.UUID{}}
}

var _ domain.UserRepository = (*UserRepo)(nil)

func (r *UserRepo) Create(_ context.Context, u *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byVKID[u.VKUserID]; ok {
		return domain.ErrConflict
	}
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	now := time.Now()
	u.CreatedAt, u.UpdatedAt = now, now
	if u.FirstSeenAt.IsZero() {
		u.FirstSeenAt = now
	}
	if u.LastSeenAt.IsZero() {
		u.LastSeenAt = now
	}
	r.byID[u.ID] = *u
	r.byVKID[u.VKUserID] = u.ID
	return nil
}

func (r *UserRepo) Update(_ context.Context, u *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[u.ID]; !ok {
		return domain.ErrNotFound
	}
	u.UpdatedAt = time.Now()
	r.byID[u.ID] = *u
	return nil
}

func (r *UserRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &u, nil
}

func (r *UserRepo) GetByVKUserID(_ context.Context, vkUserID int64) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byVKID[vkUserID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	u := r.byID[id]
	return &u, nil
}

// ---------------------------------------------------------------------------
// Jobs
// ---------------------------------------------------------------------------

// JobRepo is an in-memory domain.JobRepository.
type JobRepo struct {
	mu              sync.Mutex
	byID            map[uuid.UUID]domain.Job
	byKey           map[string]uuid.UUID
	byUser          map[uuid.UUID][]uuid.UUID
	activationLocks [64]sync.Mutex
}

// NewJobRepo builds an empty JobRepo.
func NewJobRepo() *JobRepo {
	return &JobRepo{byID: map[uuid.UUID]domain.Job{}, byKey: map[string]uuid.UUID{}, byUser: map[uuid.UUID][]uuid.UUID{}}
}

var _ domain.JobRepository = (*JobRepo)(nil)

func (r *JobRepo) Create(_ context.Context, j *domain.Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byKey[j.IdempotencyKey]; ok {
		return domain.ErrConflict
	}
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	if j.AccountID == uuid.Nil {
		j.AccountID = j.UserID
	}
	j.Source = strings.TrimSpace(j.Source)
	if j.Source == "" {
		j.Source = "unknown"
	}
	if j.ResultMode == "" {
		j.ResultMode = domain.ResultModeLegacyUnknown
	}
	if err := j.ValidateResultContract(); err != nil {
		return err
	}
	now := time.Now()
	j.CreatedAt, j.UpdatedAt = now, now
	stored := cloneJob(*j)
	r.byID[j.ID] = stored
	*j = cloneJob(stored)
	r.byKey[j.IdempotencyKey] = j.ID
	r.byUser[j.UserID] = append([]uuid.UUID{j.ID}, r.byUser[j.UserID]...)
	if j.AccountID != uuid.Nil && j.AccountID != j.UserID {
		r.byUser[j.AccountID] = append([]uuid.UUID{j.ID}, r.byUser[j.AccountID]...)
	}
	return nil
}

func (r *JobRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	j = cloneJob(j)
	return &j, nil
}

func (r *JobRepo) GetByIDForAccount(_ context.Context, accountID, id uuid.UUID) (*domain.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.byID[id]
	if !ok || accountID == uuid.Nil || j.AccountID != accountID {
		return nil, domain.ErrNotFound
	}
	j = cloneJob(j)
	return &j, nil
}

func (r *JobRepo) GetByIdempotencyKey(_ context.Context, key string) (*domain.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byKey[key]
	if !ok {
		return nil, domain.ErrNotFound
	}
	j := r.byID[id]
	j = cloneJob(j)
	return &j, nil
}

func (r *JobRepo) GetByIdempotencyKeyForAccount(_ context.Context, accountID uuid.UUID, key string) (*domain.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byKey[key]
	if !ok || accountID == uuid.Nil {
		return nil, domain.ErrNotFound
	}
	j := r.byID[id]
	if j.AccountID != accountID {
		return nil, domain.ErrNotFound
	}
	j = cloneJob(j)
	return &j, nil
}

// LockAccountForCapacity is intentionally a no-op for memory storage. The
// orchestrator serializes account activation with shared account-keyed locks,
// while this repository only supplies per-operation map consistency.
func (r *JobRepo) LockAccountForCapacity(_ context.Context, accountID uuid.UUID) error {
	if accountID == uuid.Nil {
		return domain.ErrNotFound
	}
	return nil
}

// LockAccountActivation serializes memory-only activation flows for one
// canonical account. PostgreSQL does not implement this method because its UOW
// holds the exact accounts row lock instead.
func (r *JobRepo) LockAccountActivation(accountID uuid.UUID) func() {
	lock := &r.activationLocks[int(accountID[0])%len(r.activationLocks)]
	lock.Lock()
	return lock.Unlock
}

func (r *JobRepo) UpdateStatus(_ context.Context, id uuid.UUID, from, to domain.JobStatus, errCode, errMessage string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	if j.Status != from {
		return domain.ErrConflict
	}
	j.Status = to
	j.ErrorCode = errCode
	j.ErrorMessage = errMessage
	j.UpdatedAt = time.Now()
	r.byID[id] = j
	return nil
}

func (r *JobRepo) Update(_ context.Context, j *domain.Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.byID[j.ID]
	if !ok {
		return domain.ErrNotFound
	}
	// Status is owned by UpdateStatus; preserve it across Update.
	status := cur.Status
	updated := *j
	if updated.AccountID == uuid.Nil {
		updated.AccountID = cur.AccountID
	}
	if updated.AccountID == uuid.Nil {
		updated.AccountID = updated.UserID
	}
	updated.Source = strings.TrimSpace(updated.Source)
	if updated.Source == "" {
		updated.Source = "unknown"
	}
	if updated.ResultMode == "" {
		updated.ResultMode = domain.ResultModeLegacyUnknown
	}
	if err := updated.ValidateResultContract(); err != nil {
		return err
	}
	updated.Status = status
	updated.CreatedAt = cur.CreatedAt
	updated.UpdatedAt = time.Now()
	updated = cloneJob(updated)
	r.byID[j.ID] = updated
	if updated.AccountID != uuid.Nil && updated.AccountID != updated.UserID {
		r.byUser[updated.AccountID] = append([]uuid.UUID{updated.ID}, r.byUser[updated.AccountID]...)
	}
	*j = updated
	return nil
}

func (r *JobRepo) ListByUser(_ context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	matched := r.jobsForOwnerLocked(userID, func(j domain.Job, ownerID uuid.UUID) bool {
		return j.AccountID == ownerID
	})
	if len(matched) == 0 {
		matched = r.jobsForOwnerLocked(userID, func(j domain.Job, ownerID uuid.UUID) bool {
			return j.UserID == ownerID
		})
	}
	var out []*domain.Job
	for i := offset; i < len(matched) && len(out) < limit; i++ {
		j := matched[i]
		j = cloneJob(j)
		out = append(out, &j)
	}
	return out, nil
}

func (r *JobRepo) ListByAccount(_ context.Context, accountID uuid.UUID, limit, offset int) ([]*domain.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if accountID == uuid.Nil {
		return []*domain.Job{}, nil
	}
	matched := r.jobsForOwnerLocked(accountID, func(j domain.Job, ownerID uuid.UUID) bool {
		return j.AccountID == ownerID
	})
	var out []*domain.Job
	for i := offset; i < len(matched) && len(out) < limit; i++ {
		j := matched[i]
		j = cloneJob(j)
		out = append(out, &j)
	}
	return out, nil
}

func (r *JobRepo) jobsForOwnerLocked(ownerID uuid.UUID, matches func(domain.Job, uuid.UUID) bool) []domain.Job {
	matched := make([]domain.Job, 0, len(r.byID))
	for _, j := range r.byID {
		if matches(j, ownerID) {
			matched = append(matched, j)
		}
	}
	sort.Slice(matched, func(i, k int) bool {
		if matched[i].CreatedAt.Equal(matched[k].CreatedAt) {
			return matched[i].ID.String() > matched[k].ID.String()
		}
		return matched[i].CreatedAt.After(matched[k].CreatedAt)
	})
	return matched
}

func (r *JobRepo) List(_ context.Context, filter domain.JobFilter, limit, offset int) ([]*domain.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	matched := r.filterJobsLocked(filter)
	var out []*domain.Job
	for i := offset; i < len(matched) && len(out) < limit; i++ {
		j := matched[i]
		j = cloneJob(j)
		out = append(out, &j)
	}
	return out, nil
}

func (r *JobRepo) ListCursor(_ context.Context, filter domain.JobFilter, limit int, after *domain.JobCursor) ([]*domain.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	matched := r.filterJobsLocked(filter)
	var out []*domain.Job
	for _, job := range matched {
		if after != nil {
			if job.CreatedAt.After(after.CreatedAt) || job.CreatedAt.Equal(after.CreatedAt) && job.ID.String() >= after.ID.String() {
				continue
			}
		}
		j := job
		j = cloneJob(j)
		out = append(out, &j)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (r *JobRepo) filterJobsLocked(filter domain.JobFilter) []domain.Job {
	matched := make([]domain.Job, 0, len(r.byID))
	for _, j := range r.byID {
		if filter.UserID != nil && j.UserID != *filter.UserID && j.AccountID != *filter.UserID {
			continue
		}
		if filter.AccountID != nil && j.AccountID != *filter.AccountID {
			continue
		}
		if filter.Source != "" && j.Source != filter.Source {
			continue
		}
		if filter.Status != "" && j.Status != filter.Status {
			continue
		}
		if filter.Operation != "" && j.OperationType != filter.Operation {
			continue
		}
		if filter.Modality != "" && j.Modality != filter.Modality {
			continue
		}
		if filter.ErrorCode != "" && j.ErrorCode != filter.ErrorCode {
			continue
		}
		if filter.Provider != "" && (j.ProviderID == nil || j.ProviderID.String() != filter.Provider) {
			continue
		}
		if filter.CorrelationID != "" && j.CorrelationID != filter.CorrelationID {
			continue
		}
		if filter.CreatedFrom != nil && j.CreatedAt.Before(*filter.CreatedFrom) {
			continue
		}
		if filter.CreatedTo != nil && !j.CreatedAt.Before(*filter.CreatedTo) {
			continue
		}
		matched = append(matched, j)
	}
	sort.Slice(matched, func(i, k int) bool {
		if matched[i].CreatedAt.Equal(matched[k].CreatedAt) {
			return matched[i].ID.String() > matched[k].ID.String()
		}
		return matched[i].CreatedAt.After(matched[k].CreatedAt)
	})
	return matched
}

func (r *JobRepo) CountActiveByUserOperation(_ context.Context, userID uuid.UUID, operation domain.OperationType) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := r.countJobsByOwnerLocked(userID, func(j domain.Job, ownerID uuid.UUID) bool {
		return j.AccountID == ownerID && j.OperationType == operation && j.Status.IsActiveWork()
	})
	if count > 0 {
		return count, nil
	}
	return r.countJobsByOwnerLocked(userID, func(j domain.Job, ownerID uuid.UUID) bool {
		return j.UserID == ownerID && j.OperationType == operation && j.Status.IsActiveWork()
	}), nil
}

// CountActiveByAccountOperation counts active jobs for one exact canonical
// account. Unlike the legacy user counter, it intentionally never falls back
// to user_id when the account has no active jobs.
func (r *JobRepo) CountActiveByAccountOperation(_ context.Context, accountID uuid.UUID, operation domain.OperationType) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.countJobsByOwnerLocked(accountID, func(j domain.Job, ownerID uuid.UUID) bool {
		return j.AccountID == ownerID && j.OperationType == operation && j.Status.IsActiveWork()
	}), nil
}

// CountUnexpiredPreparedByAccountOperation counts exact account-owned prepared
// jobs that still have a valid confirmation window. A zero ExpiresAt is treated
// as active for backwards compatibility with rows created before confirmation
// expiries existed.
func (r *JobRepo) CountUnexpiredPreparedByAccountOperation(_ context.Context, accountID uuid.UUID, source string, operation domain.OperationType, modality domain.Modality, now time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	source = strings.TrimSpace(source)
	return r.countJobsByOwnerLocked(accountID, func(j domain.Job, ownerID uuid.UUID) bool {
		return j.AccountID == ownerID &&
			j.Source == source &&
			j.OperationType == operation &&
			j.Modality == modality &&
			j.Status == domain.JobStatusPrepared &&
			(j.ExpiresAt == nil || j.ExpiresAt.After(now))
	}), nil
}

func (r *JobRepo) CountSucceededByUser(_ context.Context, userID uuid.UUID) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := r.countJobsByOwnerLocked(userID, func(j domain.Job, ownerID uuid.UUID) bool {
		return j.AccountID == ownerID && j.Status == domain.JobStatusSucceeded
	})
	if count > 0 {
		return count, nil
	}
	return r.countJobsByOwnerLocked(userID, func(j domain.Job, ownerID uuid.UUID) bool {
		return j.UserID == ownerID && j.Status == domain.JobStatusSucceeded
	}), nil
}

func (r *JobRepo) countJobsByOwnerLocked(ownerID uuid.UUID, matches func(domain.Job, uuid.UUID) bool) int {
	count := 0
	for _, j := range r.byID {
		if matches(j, ownerID) {
			count++
		}
	}
	return count
}

func cloneJob(job domain.Job) domain.Job {
	if job.ChannelContext != nil {
		context := *job.ChannelContext
		job.ChannelContext = &context
	}
	if job.DeliveryTarget != nil {
		target := *job.DeliveryTarget
		job.DeliveryTarget = &target
	}
	return job
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

// CommandRepo is an in-memory domain.CommandRepository.
type CommandRepo struct {
	mu    sync.Mutex
	byID  map[uuid.UUID]domain.Command
	byKey map[string]uuid.UUID
}

// NewCommandRepo builds an empty CommandRepo.
func NewCommandRepo() *CommandRepo {
	return &CommandRepo{byID: map[uuid.UUID]domain.Command{}, byKey: map[string]uuid.UUID{}}
}

var _ domain.CommandRepository = (*CommandRepo)(nil)

func (r *CommandRepo) Create(_ context.Context, c *domain.Command) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byKey[c.IdempotencyKey]; ok {
		return domain.ErrConflict
	}
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	now := time.Now()
	c.CreatedAt, c.UpdatedAt = now, now
	r.byID[c.ID] = *c
	r.byKey[c.IdempotencyKey] = c.ID
	return nil
}

func (r *CommandRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Command, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &c, nil
}

func (r *CommandRepo) GetByIdempotencyKey(_ context.Context, key string) (*domain.Command, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byKey[key]
	if !ok {
		return nil, domain.ErrNotFound
	}
	c := r.byID[id]
	return &c, nil
}

func (r *CommandRepo) ListByUser(_ context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Command, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var all []*domain.Command
	for i := range r.byID {
		c := r.byID[i]
		if c.UserID == userID {
			all = append(all, &c)
		}
	}
	return all, nil
}

// ---------------------------------------------------------------------------
// Inbound events
// ---------------------------------------------------------------------------

// InboundRepo is an in-memory domain.InboundEventRepository.
type InboundRepo struct {
	mu    sync.Mutex
	byID  map[uuid.UUID]domain.InboundEvent
	byKey map[string]uuid.UUID
}

// NewInboundRepo builds an empty InboundRepo.
func NewInboundRepo() *InboundRepo {
	return &InboundRepo{byID: map[uuid.UUID]domain.InboundEvent{}, byKey: map[string]uuid.UUID{}}
}

var _ domain.InboundEventRepository = (*InboundRepo)(nil)

func (r *InboundRepo) Create(_ context.Context, e *domain.InboundEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byKey[e.IdempotencyKey]; ok {
		return domain.ErrConflict
	}
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.Status == "" {
		e.Status = domain.InboundReceived
	}
	now := time.Now()
	e.CreatedAt, e.UpdatedAt = now, now
	r.byID[e.ID] = *e
	r.byKey[e.IdempotencyKey] = e.ID
	return nil
}

func (r *InboundRepo) GetByIdempotencyKey(_ context.Context, key string) (*domain.InboundEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byKey[key]
	if !ok {
		return nil, domain.ErrNotFound
	}
	e := r.byID[id]
	return &e, nil
}

func (r *InboundRepo) SetStatus(_ context.Context, id uuid.UUID, status domain.InboundEventStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	e.Status = status
	e.UpdatedAt = time.Now()
	r.byID[id] = e
	return nil
}

// ---------------------------------------------------------------------------
// Idempotency
// ---------------------------------------------------------------------------

// IdempotencyRepo is an in-memory domain.IdempotencyRepository.
type IdempotencyRepo struct {
	mu      sync.Mutex
	records map[string]domain.IdempotencyRecord
}

// NewIdempotencyRepo builds an empty IdempotencyRepo.
func NewIdempotencyRepo() *IdempotencyRepo {
	return &IdempotencyRepo{records: map[string]domain.IdempotencyRecord{}}
}

var _ domain.IdempotencyRepository = (*IdempotencyRepo)(nil)

func (r *IdempotencyRepo) GetOrCreate(_ context.Context, rec *domain.IdempotencyRecord) (*domain.IdempotencyRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.records[rec.Key]; ok {
		cp := existing
		return &cp, false, nil
	}
	if rec.Status == "" {
		rec.Status = domain.IdempotencyStarted
	}
	rec.CreatedAt = time.Now()
	if rec.ExpiresAt.IsZero() {
		rec.ExpiresAt = rec.CreatedAt.Add(24 * time.Hour)
	}
	r.records[rec.Key] = *rec
	cp := *rec
	return &cp, true, nil
}

func (r *IdempotencyRepo) MarkCompleted(_ context.Context, key string, resourceID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.records[key]
	if !ok {
		return domain.ErrNotFound
	}
	rec.Status = domain.IdempotencyCompleted
	rec.ResourceID = &resourceID
	r.records[key] = rec
	return nil
}

func (r *IdempotencyRepo) MarkFailed(_ context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.records[key]
	if !ok {
		return domain.ErrNotFound
	}
	rec.Status = domain.IdempotencyFailed
	r.records[key] = rec
	return nil
}

func (r *IdempotencyRepo) Get(_ context.Context, key string) (*domain.IdempotencyRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.records[key]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &rec, nil
}

// ---------------------------------------------------------------------------
// Outbox
// ---------------------------------------------------------------------------

// OutboxRepo is an in-memory domain.OutboxRepository.
type OutboxRepo struct {
	mu     sync.Mutex
	events []domain.OutboxEvent
}

// NewOutboxRepo builds an empty OutboxRepo.
func NewOutboxRepo() *OutboxRepo {
	return &OutboxRepo{}
}

var _ domain.OutboxRepository = (*OutboxRepo)(nil)

func (r *OutboxRepo) Add(_ context.Context, e *domain.OutboxEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	prepareMemoryOutboxEvent(e)
	r.events = append(r.events, *e)
	return nil
}

func (r *OutboxRepo) ExistsForAggregateEvent(_ context.Context, aggregateType string, aggregateID uuid.UUID, eventType string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.events {
		event := r.events[i]
		if event.AggregateType == aggregateType && event.AggregateID == aggregateID && event.EventType == eventType {
			return true, nil
		}
	}
	return false, nil
}

func (r *OutboxRepo) AddIfAbsentByID(_ context.Context, e *domain.OutboxEvent) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	prepareMemoryOutboxEvent(e)
	for i := range r.events {
		existing := r.events[i]
		sameID := existing.ID == e.ID
		sameResultReadySemanticIdentity :=
			e.AggregateType == "job" &&
				e.EventType == "event.job.result_ready" &&
				existing.AggregateType == e.AggregateType &&
				existing.AggregateID == e.AggregateID &&
				existing.EventType == e.EventType
		if sameID || sameResultReadySemanticIdentity {
			return false, nil
		}
	}
	r.events = append(r.events, *e)
	return true, nil
}

func prepareMemoryOutboxEvent(e *domain.OutboxEvent) {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.Status == "" {
		e.Status = domain.OutboxPending
	}
	e.CreatedAt = time.Now()
}

const maxOutboxErrorCodeRunes = 128

// ClaimPending atomically leases ready pending or expired processing events.
func (r *OutboxRepo) ClaimPending(_ context.Context, owner string, now, leaseUntil time.Time, limit int) ([]*domain.OutboxEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit <= 0 {
		return []*domain.OutboxEvent{}, nil
	}

	ready := make([]int, 0, len(r.events))
	for i := range r.events {
		event := &r.events[i]
		pendingReady := event.Status == domain.OutboxPending && !event.NextAttemptAt.After(now)
		processingExpired := event.Status == domain.OutboxStatusProcessing &&
			event.LeaseUntil != nil && !event.LeaseUntil.After(now)
		if pendingReady || processingExpired {
			ready = append(ready, i)
		}
	}
	sort.Slice(ready, func(i, j int) bool {
		left := r.events[ready[i]]
		right := r.events[ready[j]]
		if left.NextAttemptAt.Equal(right.NextAttemptAt) {
			return left.ID.String() < right.ID.String()
		}
		return left.NextAttemptAt.Before(right.NextAttemptAt)
	})
	if len(ready) > limit {
		ready = ready[:limit]
	}

	claimed := make([]*domain.OutboxEvent, 0, len(ready))
	for _, index := range ready {
		event := &r.events[index]
		token := uuid.New()
		lease := leaseUntil
		event.Status = domain.OutboxStatusProcessing
		event.ClaimToken = &token
		event.ClaimOwner = owner
		event.LeaseUntil = &lease
		copy := cloneMemoryOutboxEvent(*event)
		claimed = append(claimed, &copy)
	}
	return claimed, nil
}

// MarkPublishedClaimed publishes an event only for its current claim token.
func (r *OutboxRepo) MarkPublishedClaimed(_ context.Context, id, claimToken uuid.UUID, publishedAt time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.events {
		event := &r.events[i]
		if event.ID != id || event.Status != domain.OutboxStatusProcessing ||
			event.ClaimToken == nil || *event.ClaimToken != claimToken {
			continue
		}
		event.Status = domain.OutboxPublished
		event.PublishedAt = memoryTimePointer(publishedAt)
		clearMemoryOutboxClaim(event)
		event.LastErrorCode = ""
		event.FailedAt = nil
		return true, nil
	}
	return false, nil
}

// RetryClaimed returns an event to pending only for its current claim token.
func (r *OutboxRepo) RetryClaimed(_ context.Context, id, claimToken uuid.UUID, nextAttemptAt time.Time, errorCode string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.events {
		event := &r.events[i]
		if event.ID != id || event.Status != domain.OutboxStatusProcessing ||
			event.ClaimToken == nil || *event.ClaimToken != claimToken {
			continue
		}
		event.Status = domain.OutboxPending
		event.Attempts++
		event.NextAttemptAt = nextAttemptAt
		event.LastErrorCode = boundMemoryOutboxErrorCode(errorCode)
		event.FailedAt = nil
		clearMemoryOutboxClaim(event)
		return true, nil
	}
	return false, nil
}

// FailClaimed quarantines an event only for its current claim token.
func (r *OutboxRepo) FailClaimed(_ context.Context, id, claimToken uuid.UUID, failedAt time.Time, errorCode string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.events {
		event := &r.events[i]
		if event.ID != id || event.Status != domain.OutboxStatusProcessing ||
			event.ClaimToken == nil || *event.ClaimToken != claimToken {
			continue
		}
		event.Status = domain.OutboxFailed
		event.Attempts++
		event.LastErrorCode = boundMemoryOutboxErrorCode(errorCode)
		event.FailedAt = memoryTimePointer(failedAt)
		clearMemoryOutboxClaim(event)
		return true, nil
	}
	return false, nil
}

func clearMemoryOutboxClaim(event *domain.OutboxEvent) {
	event.ClaimToken = nil
	event.ClaimOwner = ""
	event.LeaseUntil = nil
}

func boundMemoryOutboxErrorCode(errorCode string) string {
	runes := []rune(errorCode)
	if len(runes) <= maxOutboxErrorCodeRunes {
		return errorCode
	}
	return string(runes[:maxOutboxErrorCodeRunes])
}

func memoryTimePointer(value time.Time) *time.Time {
	copy := value
	return &copy
}

func cloneMemoryOutboxEvent(event domain.OutboxEvent) domain.OutboxEvent {
	event.Payload = append([]byte(nil), event.Payload...)
	if event.PublishedAt != nil {
		event.PublishedAt = memoryTimePointer(*event.PublishedAt)
	}
	if event.ClaimToken != nil {
		token := *event.ClaimToken
		event.ClaimToken = &token
	}
	if event.LeaseUntil != nil {
		event.LeaseUntil = memoryTimePointer(*event.LeaseUntil)
	}
	if event.FailedAt != nil {
		event.FailedAt = memoryTimePointer(*event.FailedAt)
	}
	return event
}

func (r *OutboxRepo) FetchPending(_ context.Context, limit int) ([]*domain.OutboxEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.OutboxEvent
	for i := range r.events {
		if r.events[i].Status == domain.OutboxPending {
			e := r.events[i]
			out = append(out, &e)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (r *OutboxRepo) MarkPublished(_ context.Context, id uuid.UUID, publishedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.events {
		if r.events[i].ID == id {
			r.events[i].Status = domain.OutboxPublished
			r.events[i].PublishedAt = &publishedAt
			return nil
		}
	}
	return domain.ErrNotFound
}

func (r *OutboxRepo) MarkFailed(_ context.Context, id uuid.UUID, nextAttemptAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.events {
		if r.events[i].ID == id {
			r.events[i].Attempts++
			r.events[i].NextAttemptAt = nextAttemptAt
			return nil
		}
	}
	return domain.ErrNotFound
}

// Events returns a copy of all stored events.
func (r *OutboxRepo) Events() []domain.OutboxEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.OutboxEvent, len(r.events))
	for i := range r.events {
		out[i] = cloneMemoryOutboxEvent(r.events[i])
	}
	return out
}

// OutboxHealthSnapshot returns count-only operational state.
func (r *OutboxRepo) OutboxHealthSnapshot(_ context.Context, now time.Time) (domain.OutboxHealth, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var snapshot domain.OutboxHealth
	for i := range r.events {
		event := r.events[i]
		switch event.Status {
		case domain.OutboxPending:
			snapshot.Pending++
			if snapshot.OldestPendingCreatedAt == nil || event.CreatedAt.Before(*snapshot.OldestPendingCreatedAt) {
				createdAt := event.CreatedAt
				snapshot.OldestPendingCreatedAt = &createdAt
			}
		case domain.OutboxStatusProcessing:
			snapshot.Processing++
			if event.LeaseUntil != nil && !event.LeaseUntil.After(now) {
				snapshot.ExpiredLeases++
			}
		case domain.OutboxFailed:
			snapshot.Failed++
		}
	}
	return snapshot, nil
}

// ---------------------------------------------------------------------------
// Unit of work
// ---------------------------------------------------------------------------

// UnitOfWork is an in-memory uow.Manager. Without real transactions it simply
// invokes the callback with the supplied repositories; the maps' own locking
// keeps individual operations consistent.
type UnitOfWork struct {
	repos uow.Repositories
}

// NewUnitOfWork builds a UnitOfWork bound to the given repositories. billing may
// be nil for callers that do not compose reservations transactionally.
func NewUnitOfWork(jobs *JobRepo, outbox *OutboxRepo, billing *BillingRepo) *UnitOfWork {
	repos := uow.Repositories{Jobs: jobs, Outbox: outbox}
	if billing != nil {
		repos.Billing = billing
	}
	return &UnitOfWork{repos: repos}
}

var _ uow.Manager = (*UnitOfWork)(nil)

func (u *UnitOfWork) Within(ctx context.Context, fn func(ctx context.Context, repos uow.Repositories) error) error {
	return fn(ctx, u.repos)
}
