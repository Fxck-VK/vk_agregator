package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/outboxrelay"
)

func TestResultReadyCandidatesPostgresAntiJoinCursorAndPagePlusOne(t *testing.T) {
	ctx := context.Background()
	pool := preparedAccountIntegrationPool(t, ctx)
	jobs := postgres.NewJobRepository(pool)
	outbox := postgres.NewOutboxRepository(pool)
	accountID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO accounts (id) VALUES ($1)`, accountID); err != nil {
		t.Fatalf("insert account: %v", err)
	}

	base := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	ready := make([]*domain.Job, 0, 4)
	for i := range 4 {
		job := postgresCandidateReadyJob(accountID, "candidate-"+string(rune('a'+i)))
		if err := jobs.Create(ctx, job); err != nil {
			t.Fatalf("create ready job %d: %v", i, err)
		}
		createdAt := base.Add(-time.Duration(i) * time.Minute)
		if _, err := pool.Exec(ctx, `UPDATE jobs SET created_at = $2 WHERE id = $1`, job.ID, createdAt); err != nil {
			t.Fatalf("set ready job %d created_at: %v", i, err)
		}
		job.CreatedAt = createdAt
		ready = append(ready, job)
	}
	existing := &domain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "job",
		AggregateID:   ready[1].ID,
		EventType:     outboxrelay.EventJobResultReady,
		Payload:       json.RawMessage(`{"malformed_historical":true}`),
		Status:        domain.OutboxFailed,
	}
	if err := outbox.Add(ctx, existing); err != nil {
		t.Fatalf("add semantic event: %v", err)
	}
	legacy := postgresCandidateReadyJob(accountID, "legacy")
	legacy.ResultMode = domain.ResultModeLegacyUnknown
	if err := jobs.Create(ctx, legacy); err != nil {
		t.Fatalf("create legacy job: %v", err)
	}
	ownerless := postgresCandidateReadyJob(uuid.Nil, "ownerless")
	ownerless.ResultMode = domain.ResultModeExternalPush
	ownerless.DeliveryTarget = &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "123"}
	if err := jobs.Create(ctx, ownerless); err != nil {
		t.Fatalf("create ownerless external-push job: %v", err)
	}

	repo := postgres.NewResultReadyCandidateRepository(pool)
	first, hasMore, err := repo.ListMissingCanonicalResultReady(ctx, 2, nil)
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(first) != 2 || !hasMore {
		t.Fatalf("first page len/hasMore = %d/%v, want 2/true", len(first), hasMore)
	}
	if first[0].ID != ready[0].ID || first[1].ID != ready[2].ID {
		t.Fatalf("first page ids = %s/%s, want newest missing %s/%s",
			first[0].ID, first[1].ID, ready[0].ID, ready[2].ID)
	}
	cursor := &domain.JobCursor{CreatedAt: first[1].CreatedAt, ID: first[1].ID}
	second, hasMore, err := repo.ListMissingCanonicalResultReady(ctx, 2, cursor)
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if len(second) != 1 || hasMore || second[0].ID != ready[3].ID {
		t.Fatalf("second page = %+v hasMore=%v, want oldest missing %s only", second, hasMore, ready[3].ID)
	}
}

func postgresCandidateReadyJob(accountID uuid.UUID, key string) *domain.Job {
	return &domain.Job{
		ID:                uuid.New(),
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
