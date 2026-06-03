package memory

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// ProviderTaskRepo is an in-memory domain.ProviderTaskRepository.
type ProviderTaskRepo struct {
	mu         sync.Mutex
	byID       map[uuid.UUID]domain.ProviderTask
	byKey      map[string]uuid.UUID
	byExternal map[string]uuid.UUID
	byJob      map[uuid.UUID][]uuid.UUID
}

// NewProviderTaskRepo builds an empty ProviderTaskRepo.
func NewProviderTaskRepo() *ProviderTaskRepo {
	return &ProviderTaskRepo{
		byID:       map[uuid.UUID]domain.ProviderTask{},
		byKey:      map[string]uuid.UUID{},
		byExternal: map[string]uuid.UUID{},
		byJob:      map[uuid.UUID][]uuid.UUID{},
	}
}

var _ domain.ProviderTaskRepository = (*ProviderTaskRepo)(nil)

func externalKey(provider domain.ProviderName, externalID string) string {
	return string(provider) + "|" + externalID
}

func (r *ProviderTaskRepo) Create(_ context.Context, t *domain.ProviderTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t.IdempotencyKey != "" {
		if _, ok := r.byKey[t.IdempotencyKey]; ok {
			return domain.ErrConflict
		}
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	now := time.Now()
	t.CreatedAt, t.UpdatedAt = now, now
	r.store(*t)
	return nil
}

func (r *ProviderTaskRepo) Update(_ context.Context, t *domain.ProviderTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.byID[t.ID]
	if !ok {
		return domain.ErrNotFound
	}
	t.CreatedAt = cur.CreatedAt
	t.UpdatedAt = time.Now()
	r.store(*t)
	return nil
}

// store indexes a task; caller holds the lock.
func (r *ProviderTaskRepo) store(t domain.ProviderTask) {
	if _, seen := r.byID[t.ID]; !seen {
		r.byJob[t.JobID] = append(r.byJob[t.JobID], t.ID)
	}
	r.byID[t.ID] = t
	if t.IdempotencyKey != "" {
		r.byKey[t.IdempotencyKey] = t.ID
	}
	if t.ExternalID != "" {
		r.byExternal[externalKey(t.Provider, t.ExternalID)] = t.ID
	}
}

func (r *ProviderTaskRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.ProviderTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &t, nil
}

func (r *ProviderTaskRepo) GetByExternalID(_ context.Context, provider domain.ProviderName, externalID string) (*domain.ProviderTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byExternal[externalKey(provider, externalID)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	t := r.byID[id]
	return &t, nil
}

func (r *ProviderTaskRepo) ListByJob(_ context.Context, jobID uuid.UUID) ([]*domain.ProviderTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.ProviderTask
	for _, id := range r.byJob[jobID] {
		t := r.byID[id]
		out = append(out, &t)
	}
	return out, nil
}
