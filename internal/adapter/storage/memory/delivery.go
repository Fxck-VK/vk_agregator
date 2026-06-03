package memory

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// DeliveryRepo is an in-memory domain.DeliveryRepository. Like the PostgreSQL
// adapter it deduplicates by idempotency key so a retried delivery never
// produces a second row.
type DeliveryRepo struct {
	mu    sync.Mutex
	byID  map[uuid.UUID]domain.Delivery
	byKey map[string]uuid.UUID
	byJob map[uuid.UUID][]uuid.UUID
}

// NewDeliveryRepo builds an empty DeliveryRepo.
func NewDeliveryRepo() *DeliveryRepo {
	return &DeliveryRepo{
		byID:  map[uuid.UUID]domain.Delivery{},
		byKey: map[string]uuid.UUID{},
		byJob: map[uuid.UUID][]uuid.UUID{},
	}
}

var _ domain.DeliveryRepository = (*DeliveryRepo)(nil)

func (r *DeliveryRepo) Create(_ context.Context, d *domain.Delivery) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if d.IdempotencyKey != "" {
		if _, ok := r.byKey[d.IdempotencyKey]; ok {
			return domain.ErrConflict
		}
	}
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	if d.AttemptNo == 0 {
		d.AttemptNo = 1
	}
	now := time.Now()
	d.CreatedAt, d.UpdatedAt = now, now
	r.byID[d.ID] = *d
	if d.IdempotencyKey != "" {
		r.byKey[d.IdempotencyKey] = d.ID
	}
	r.byJob[d.JobID] = append(r.byJob[d.JobID], d.ID)
	return nil
}

func (r *DeliveryRepo) Update(_ context.Context, d *domain.Delivery) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.byID[d.ID]
	if !ok {
		return domain.ErrNotFound
	}
	d.CreatedAt = cur.CreatedAt
	d.UpdatedAt = time.Now()
	r.byID[d.ID] = *d
	return nil
}

func (r *DeliveryRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Delivery, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &d, nil
}

func (r *DeliveryRepo) GetByIdempotencyKey(_ context.Context, key string) (*domain.Delivery, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byKey[key]
	if !ok {
		return nil, domain.ErrNotFound
	}
	d := r.byID[id]
	return &d, nil
}

func (r *DeliveryRepo) ListByJob(_ context.Context, jobID uuid.UUID) ([]*domain.Delivery, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Delivery
	for _, id := range r.byJob[jobID] {
		d := r.byID[id]
		out = append(out, &d)
	}
	return out, nil
}
