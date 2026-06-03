package memory

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// ModerationRepo is an in-memory domain.ModerationResultRepository.
type ModerationRepo struct {
	mu      sync.Mutex
	results []*domain.ModerationResult
}

// NewModerationRepo builds an empty ModerationRepo.
func NewModerationRepo() *ModerationRepo {
	return &ModerationRepo{}
}

var _ domain.ModerationResultRepository = (*ModerationRepo)(nil)

// Create stores a verdict.
func (r *ModerationRepo) Create(_ context.Context, m *domain.ModerationResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	cp := *m
	r.results = append(r.results, &cp)
	return nil
}

// ListByJob returns verdicts for a job in insertion order.
func (r *ModerationRepo) ListByJob(_ context.Context, jobID uuid.UUID) ([]*domain.ModerationResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.ModerationResult
	for _, m := range r.results {
		if m.JobID == jobID {
			cp := *m
			out = append(out, &cp)
		}
	}
	return out, nil
}
