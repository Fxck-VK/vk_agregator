package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// OperatorAuditRepo is an in-memory sanitized operator audit repository.
type OperatorAuditRepo struct {
	mu      sync.Mutex
	entries map[uuid.UUID]domain.OperatorAuditEntry
}

// NewOperatorAuditRepo builds an empty OperatorAuditRepo.
func NewOperatorAuditRepo() *OperatorAuditRepo {
	return &OperatorAuditRepo{entries: map[uuid.UUID]domain.OperatorAuditEntry{}}
}

var _ domain.OperatorAuditRepository = (*OperatorAuditRepo)(nil)

func (r *OperatorAuditRepo) Create(_ context.Context, entry *domain.OperatorAuditEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	r.entries[entry.ID] = *entry
	return nil
}

func (r *OperatorAuditRepo) List(_ context.Context, filter domain.OperatorAuditFilter, limit, offset int) ([]*domain.OperatorAuditEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit <= 0 {
		limit = 20
	}
	matched := make([]domain.OperatorAuditEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		if filter.Action != "" && entry.Action != filter.Action {
			continue
		}
		if filter.TargetType != "" && entry.TargetType != filter.TargetType {
			continue
		}
		if filter.Result != "" && entry.Result != filter.Result {
			continue
		}
		matched = append(matched, entry)
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})
	out := make([]*domain.OperatorAuditEntry, 0, limit)
	for i := offset; i < len(matched) && len(out) < limit; i++ {
		entry := matched[i]
		out = append(out, &entry)
	}
	return out, nil
}
