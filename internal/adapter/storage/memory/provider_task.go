package memory

import (
	"context"
	"math"
	"sort"
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

func (r *ProviderTaskRepo) HealthSnapshot(_ context.Context, since time.Time) ([]domain.ProviderTaskHealth, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := map[domain.ProviderName]domain.ProviderTaskHealth{}
	latencies := map[domain.ProviderName][]int64{}
	for _, task := range r.byID {
		if !since.IsZero() && task.CreatedAt.Before(since) {
			continue
		}
		item := items[task.Provider]
		item.Provider = task.Provider
		item.TotalCount++
		switch task.Status {
		case domain.ProviderTaskFailed:
			item.FailedCount++
		case domain.ProviderTaskPending, domain.ProviderTaskProcessing:
			item.InFlightCount++
		}
		if task.ErrorClass == domain.ProviderErrRateLimited {
			item.RateLimitedCount++
		}
		if task.ErrorClass != "" && (item.LatestErrorAt == nil || task.UpdatedAt.After(*item.LatestErrorAt)) {
			at := task.UpdatedAt
			item.LatestErrorClass = task.ErrorClass
			item.LatestErrorAt = &at
		}
		if task.CompletedAt != nil {
			start := task.CreatedAt
			if task.SubmittedAt != nil {
				start = *task.SubmittedAt
			}
			if task.CompletedAt.After(start) {
				latencies[task.Provider] = append(latencies[task.Provider], task.CompletedAt.Sub(start).Milliseconds())
			}
		}
		items[task.Provider] = item
	}

	out := make([]domain.ProviderTaskHealth, 0, len(items))
	for provider, item := range items {
		item.LatencyP95Ms = percentile95(latencies[provider])
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Provider < out[j].Provider })
	return out, nil
}

func percentile95(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	idx := int(math.Ceil(float64(len(values))*0.95)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(values) {
		idx = len(values) - 1
	}
	return values[idx]
}
