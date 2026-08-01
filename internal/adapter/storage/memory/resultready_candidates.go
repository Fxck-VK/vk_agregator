package memory

import (
	"context"
	"errors"
	"sort"

	"vk-ai-aggregator/internal/domain"
)

const resultReadyCandidateEventType = "event.job.result_ready"

// ResultReadyCandidateRepository is the in-memory bounded anti-join adapter
// used by tests and local processes.
type ResultReadyCandidateRepository struct {
	jobs   *JobRepo
	outbox *OutboxRepo
}

// NewResultReadyCandidateRepository builds a candidate repository over the
// shared job and outbox stores.
func NewResultReadyCandidateRepository(
	jobs *JobRepo,
	outbox *OutboxRepo,
) *ResultReadyCandidateRepository {
	return &ResultReadyCandidateRepository{jobs: jobs, outbox: outbox}
}

var _ domain.ResultReadyCandidateRepository = (*ResultReadyCandidateRepository)(nil)

// ListMissingCanonicalResultReady returns at most limit missing candidates in
// descending (created_at, id) order and derives hasMore from one extra row.
func (r *ResultReadyCandidateRepository) ListMissingCanonicalResultReady(
	_ context.Context,
	limit int,
	after *domain.JobCursor,
) ([]*domain.Job, bool, error) {
	if r == nil || r.jobs == nil || r.outbox == nil {
		return nil, false, errors.New("memory: result-ready candidate repositories are required")
	}
	if limit <= 0 {
		return nil, false, errors.New("memory: result-ready candidate limit must be positive")
	}

	r.outbox.mu.Lock()
	existing := make(map[domainAggregateEvent]struct{}, len(r.outbox.events))
	for i := range r.outbox.events {
		event := r.outbox.events[i]
		existing[domainAggregateEvent{
			aggregateType: event.AggregateType,
			aggregateID:   event.AggregateID,
			eventType:     event.EventType,
		}] = struct{}{}
	}
	r.outbox.mu.Unlock()

	r.jobs.mu.Lock()
	matched := make([]domain.Job, 0, len(r.jobs.byID))
	for _, stored := range r.jobs.byID {
		if stored.Status != domain.JobStatusResultReady ||
			stored.AccountID == [16]byte{} ||
			!canonicalResultMode(stored.ResultMode) {
			continue
		}
		if _, ok := existing[domainAggregateEvent{
			aggregateType: "job",
			aggregateID:   stored.ID,
			eventType:     resultReadyCandidateEventType,
		}]; ok {
			continue
		}
		if after != nil && !resultReadyCandidateBefore(stored, *after) {
			continue
		}
		matched = append(matched, cloneJob(stored))
	}
	r.jobs.mu.Unlock()

	sort.Slice(matched, func(i, j int) bool {
		if matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].ID.String() > matched[j].ID.String()
		}
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})

	hasMore := len(matched) > limit
	if hasMore {
		matched = matched[:limit]
	}
	jobs := make([]*domain.Job, len(matched))
	for i := range matched {
		job := cloneJob(matched[i])
		jobs[i] = &job
	}
	return jobs, hasMore, nil
}

type domainAggregateEvent struct {
	aggregateType string
	aggregateID   [16]byte
	eventType     string
}

func canonicalResultMode(mode domain.ResultMode) bool {
	return mode == domain.ResultModeAccountHistory || mode == domain.ResultModeExternalPush
}

func resultReadyCandidateBefore(job domain.Job, cursor domain.JobCursor) bool {
	if job.CreatedAt.Equal(cursor.CreatedAt) {
		return job.ID.String() < cursor.ID.String()
	}
	return job.CreatedAt.Before(cursor.CreatedAt)
}
