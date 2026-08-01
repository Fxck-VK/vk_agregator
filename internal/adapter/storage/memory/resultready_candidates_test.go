package memory

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/outboxrelay"
)

func TestResultReadyCandidatesUseSemanticAntiJoinAndPagePlusOne(t *testing.T) {
	ctx := context.Background()
	jobs := NewJobRepo()
	outbox := NewOutboxRepo()
	ready := make([]*domain.Job, 0, 4)
	for i := range 4 {
		job := memoryCanonicalReadyJob("candidate-" + string(rune('a'+i)))
		if err := jobs.Create(ctx, job); err != nil {
			t.Fatalf("create canonical job %d: %v", i, err)
		}
		ready = append(ready, job)
	}
	existing := &domain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "job",
		AggregateID:   ready[0].ID,
		EventType:     outboxrelay.EventJobResultReady,
		Payload:       json.RawMessage(`{"malformed_historical":true}`),
		Status:        domain.OutboxFailed,
	}
	if err := outbox.Add(ctx, existing); err != nil {
		t.Fatalf("add semantic event: %v", err)
	}
	legacy := memoryCanonicalReadyJob("legacy")
	legacy.ResultMode = domain.ResultModeLegacyUnknown
	if err := jobs.Create(ctx, legacy); err != nil {
		t.Fatalf("create legacy job: %v", err)
	}
	ownerless := memoryCanonicalReadyJob("ownerless")
	ownerless.UserID = uuid.Nil
	ownerless.AccountID = uuid.Nil
	ownerless.ResultMode = domain.ResultModeExternalPush
	ownerless.DeliveryTarget = &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "123"}
	if err := jobs.Create(ctx, ownerless); err != nil {
		t.Fatalf("create ownerless external-push job: %v", err)
	}

	repo := NewResultReadyCandidateRepository(jobs, outbox)
	first, hasMore, err := repo.ListMissingCanonicalResultReady(ctx, 2, nil)
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(first) != 2 || !hasMore {
		t.Fatalf("first page len/hasMore = %d/%v, want 2/true", len(first), hasMore)
	}
	for _, job := range first {
		if job == nil || job.AccountID == uuid.Nil || job.ResultMode == domain.ResultModeLegacyUnknown ||
			job.ID == existing.AggregateID {
			t.Fatalf("non-candidate returned: %+v", job)
		}
	}
	cursor := &domain.JobCursor{CreatedAt: first[len(first)-1].CreatedAt, ID: first[len(first)-1].ID}
	second, hasMore, err := repo.ListMissingCanonicalResultReady(ctx, 2, cursor)
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if len(second) != 1 || hasMore {
		t.Fatalf("second page len/hasMore = %d/%v, want 1/false", len(second), hasMore)
	}
	if !candidateBefore(first[0], first[1]) || !candidateBefore(first[1], second[0]) {
		t.Fatalf("candidate order is not created_at/id descending: first=%+v second=%+v", first, second)
	}
}

func TestOutboxAddIfAbsentRejectsExistingSemanticResultReadyEvent(t *testing.T) {
	ctx := context.Background()
	outbox := NewOutboxRepo()
	jobID := uuid.New()
	existing := &domain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "job",
		AggregateID:   jobID,
		EventType:     outboxrelay.EventJobResultReady,
		Status:        domain.OutboxFailed,
	}
	if err := outbox.Add(ctx, existing); err != nil {
		t.Fatalf("add existing: %v", err)
	}
	created, err := outbox.AddIfAbsentByID(ctx, &domain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "job",
		AggregateID:   jobID,
		EventType:     outboxrelay.EventJobResultReady,
	})
	if err != nil || created {
		t.Fatalf("semantic AddIfAbsentByID = (%v, %v), want (false, nil)", created, err)
	}
	if events := outbox.Events(); len(events) != 1 || events[0].ID != existing.ID {
		t.Fatalf("semantic duplicate events = %+v, want original only", events)
	}
}

func memoryCanonicalReadyJob(key string) *domain.Job {
	accountID := uuid.New()
	return &domain.Job{
		ID:                uuid.New(),
		UserID:            accountID,
		AccountID:         accountID,
		Source:            "web",
		ResultMode:        domain.ResultModeAccountHistory,
		OperationType:     domain.OperationTextGenerate,
		Modality:          domain.ModalityText,
		Status:            domain.JobStatusResultReady,
		IdempotencyKey:    key + "-" + uuid.NewString(),
		OutputArtifactIDs: []uuid.UUID{uuid.New()},
	}
}

func candidateBefore(left, right *domain.Job) bool {
	if left.CreatedAt.Equal(right.CreatedAt) {
		return left.ID.String() > right.ID.String()
	}
	return left.CreatedAt.After(right.CreatedAt)
}
