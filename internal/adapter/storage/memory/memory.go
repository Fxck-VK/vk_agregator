// Package memory provides in-memory implementations of the domain repository
// interfaces. They are intended for unit tests and local development, mirroring
// the semantics of the PostgreSQL adapters (idempotency conflicts, optimistic
// status transitions, ledger-based balances) without an external database.
package memory

import (
	"context"
	"sort"
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
	mu     sync.Mutex
	byID   map[uuid.UUID]domain.Job
	byKey  map[string]uuid.UUID
	byUser map[uuid.UUID][]uuid.UUID
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
	now := time.Now()
	j.CreatedAt, j.UpdatedAt = now, now
	r.byID[j.ID] = *j
	r.byKey[j.IdempotencyKey] = j.ID
	r.byUser[j.UserID] = append([]uuid.UUID{j.ID}, r.byUser[j.UserID]...)
	return nil
}

func (r *JobRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
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
	return &j, nil
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
	updated.Status = status
	updated.CreatedAt = cur.CreatedAt
	updated.UpdatedAt = time.Now()
	r.byID[j.ID] = updated
	*j = updated
	return nil
}

func (r *JobRepo) ListByUser(_ context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := r.byUser[userID]
	var out []*domain.Job
	for i := offset; i < len(ids) && len(out) < limit; i++ {
		j := r.byID[ids[i]]
		out = append(out, &j)
	}
	return out, nil
}

func (r *JobRepo) List(_ context.Context, filter domain.JobFilter, limit, offset int) ([]*domain.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	matched := make([]domain.Job, 0, len(r.byID))
	for _, j := range r.byID {
		if filter.UserID != nil && j.UserID != *filter.UserID {
			continue
		}
		if filter.Status != "" && j.Status != filter.Status {
			continue
		}
		if filter.Operation != "" && j.OperationType != filter.Operation {
			continue
		}
		matched = append(matched, j)
	}
	sort.Slice(matched, func(i, k int) bool {
		return matched[i].CreatedAt.After(matched[k].CreatedAt)
	})
	var out []*domain.Job
	for i := offset; i < len(matched) && len(out) < limit; i++ {
		j := matched[i]
		out = append(out, &j)
	}
	return out, nil
}

func (r *JobRepo) CountActiveByUserOperation(_ context.Context, userID uuid.UUID, operation domain.OperationType) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, j := range r.byID {
		if j.UserID == userID && j.OperationType == operation && j.Status.IsActiveWork() {
			count++
		}
	}
	return count, nil
}

func (r *JobRepo) CountSucceededByUser(_ context.Context, userID uuid.UUID) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, j := range r.byID {
		if j.UserID == userID && j.Status == domain.JobStatusSucceeded {
			count++
		}
	}
	return count, nil
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
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.Status == "" {
		e.Status = domain.OutboxPending
	}
	e.CreatedAt = time.Now()
	r.events = append(r.events, *e)
	return nil
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
	copy(out, r.events)
	return out
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
