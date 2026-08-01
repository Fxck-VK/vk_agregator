package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/uow"
	"vk-ai-aggregator/internal/service/outboxrelay"
	"vk-ai-aggregator/internal/service/resultreadyreconciliation"
)

func TestOutboxExactExistsAndDeterministicInsertPostgresIntegration(t *testing.T) {
	ctx := context.Background()
	pool := preparedAccountIntegrationPool(t, ctx)
	outbox := postgres.NewOutboxRepository(pool)
	aggregateID := uuid.New()
	event := &domain.OutboxEvent{
		ID:            uuid.NewSHA1(uuid.NameSpaceURL, []byte("outbox-reconciliation-integration:"+aggregateID.String())),
		AggregateType: "job",
		AggregateID:   aggregateID,
		EventType:     outboxrelay.EventJobResultReady,
		Payload:       json.RawMessage(`{"job_id":"00000000-0000-0000-0000-000000000000"}`),
	}

	created, err := outbox.AddIfAbsentByID(ctx, event)
	if err != nil || !created {
		t.Fatalf("first AddIfAbsentByID = (%v, %v), want (true, nil)", created, err)
	}
	created, err = outbox.AddIfAbsentByID(ctx, event)
	if err != nil || created {
		t.Fatalf("second AddIfAbsentByID = (%v, %v), want (false, nil)", created, err)
	}
	exists, err := outbox.ExistsForAggregateEvent(ctx, "job", aggregateID, outboxrelay.EventJobResultReady)
	if err != nil || !exists {
		t.Fatalf("pending ExistsForAggregateEvent = (%v, %v), want (true, nil)", exists, err)
	}
	if err := outbox.MarkPublished(ctx, event.ID, testNow()); err != nil {
		t.Fatalf("mark published: %v", err)
	}
	exists, err = outbox.ExistsForAggregateEvent(ctx, "job", aggregateID, outboxrelay.EventJobResultReady)
	if err != nil || !exists {
		t.Fatalf("published ExistsForAggregateEvent = (%v, %v), want (true, nil)", exists, err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE id = $1`, event.ID).Scan(&count); err != nil {
		t.Fatalf("count deterministic rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("deterministic rows = %d, want 1", count)
	}
}

func TestPostgresOutboxClaimLeaseOwnership(t *testing.T) {
	ctx := context.Background()
	pool := preparedAccountIntegrationPool(t, ctx)
	outbox := postgres.NewOutboxRepository(pool)
	now := testNow()
	leaseUntil := now.Add(time.Minute)
	event := &domain.OutboxEvent{
		AggregateType: "job",
		AggregateID:   uuid.New(),
		EventType:     outboxrelay.EventJobResultReady,
		NextAttemptAt: now,
	}
	if err := outbox.Add(ctx, event); err != nil {
		t.Fatalf("add: %v", err)
	}

	start := make(chan struct{})
	results := make(chan []*domain.OutboxEvent, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, owner := range []string{"relay-a", "relay-b"} {
		owner := owner
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			claimed, err := outbox.ClaimPending(ctx, owner, now, leaseUntil, 1)
			results <- claimed
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
	}
	var first *domain.OutboxEvent
	total := 0
	for claimed := range results {
		total += len(claimed)
		if len(claimed) == 1 {
			first = claimed[0]
		}
	}
	if total != 1 {
		t.Fatalf("total concurrently claimed = %d, want 1", total)
	}
	if first == nil || first.ClaimToken == nil || first.Status != domain.OutboxStatusProcessing {
		t.Fatalf("first claim = %+v, want processing claim with token", first)
	}

	reclaimed, err := outbox.ClaimPending(ctx, "relay-c", leaseUntil, leaseUntil.Add(time.Minute), 1)
	if err != nil || len(reclaimed) != 1 || reclaimed[0].ClaimToken == nil {
		t.Fatalf("reclaim = %d, %v; want claimed token", len(reclaimed), err)
	}
	staleToken := *first.ClaimToken
	if updated, err := outbox.MarkPublishedClaimed(ctx, event.ID, staleToken, now); err != nil || updated {
		t.Fatalf("stale mark = %v, %v; want false, nil", updated, err)
	}
	if updated, err := outbox.RetryClaimed(ctx, event.ID, staleToken, now, "stale"); err != nil || updated {
		t.Fatalf("stale retry = %v, %v; want false, nil", updated, err)
	}
	if updated, err := outbox.FailClaimed(ctx, event.ID, staleToken, now, "stale"); err != nil || updated {
		t.Fatalf("stale fail = %v, %v; want false, nil", updated, err)
	}
}

func TestPostgresOutboxClaimResolveTransitions(t *testing.T) {
	ctx := context.Background()
	pool := preparedAccountIntegrationPool(t, ctx)
	outbox := postgres.NewOutboxRepository(pool)
	now := testNow()
	longErrorCode := strings.Repeat("x", 256)

	event := &domain.OutboxEvent{
		AggregateType: "job",
		AggregateID:   uuid.New(),
		EventType:     outboxrelay.EventJobResultReady,
		NextAttemptAt: now,
	}
	if err := outbox.Add(ctx, event); err != nil {
		t.Fatalf("add retry event: %v", err)
	}
	claimed, err := outbox.ClaimPending(ctx, "relay-a", now, now.Add(time.Minute), 1)
	if err != nil || len(claimed) != 1 || claimed[0].ClaimToken == nil {
		t.Fatalf("claim retry event = %d, %v; want claimed token", len(claimed), err)
	}
	nextAttemptAt := now.Add(2 * time.Minute)
	updated, err := outbox.RetryClaimed(ctx, event.ID, *claimed[0].ClaimToken, nextAttemptAt, longErrorCode)
	if err != nil || !updated {
		t.Fatalf("retry = %v, %v; want true, nil", updated, err)
	}
	var status domain.OutboxStatus
	var attempts int
	var claimToken *uuid.UUID
	var claimOwner *string
	var leaseUntil *time.Time
	var lastErrorCode string
	if err := pool.QueryRow(ctx, `
		SELECT status, attempts, claim_token, claim_owner, lease_until, last_error_code
		FROM outbox_events
		WHERE id = $1
	`, event.ID).Scan(&status, &attempts, &claimToken, &claimOwner, &leaseUntil, &lastErrorCode); err != nil {
		t.Fatalf("read retry state: %v", err)
	}
	if status != domain.OutboxPending || attempts != 1 || claimToken != nil || claimOwner != nil || leaseUntil != nil {
		t.Fatalf("retry state = %s/%d/%v/%v/%v, want pending/1/nil/nil/nil", status, attempts, claimToken, claimOwner, leaseUntil)
	}
	if len([]rune(lastErrorCode)) > 128 {
		t.Fatalf("retry error code length = %d, want <= 128", len([]rune(lastErrorCode)))
	}

	claimed, err = outbox.ClaimPending(ctx, "relay-b", nextAttemptAt, nextAttemptAt.Add(time.Minute), 1)
	if err != nil || len(claimed) != 1 || claimed[0].ClaimToken == nil {
		t.Fatalf("reclaim retry event = %d, %v; want claimed token", len(claimed), err)
	}
	failedAt := nextAttemptAt.Add(30 * time.Second)
	updated, err = outbox.FailClaimed(ctx, event.ID, *claimed[0].ClaimToken, failedAt, longErrorCode)
	if err != nil || !updated {
		t.Fatalf("fail = %v, %v; want true, nil", updated, err)
	}
	var failedAtStored *time.Time
	if err := pool.QueryRow(ctx, `
		SELECT status, attempts, claim_token, claim_owner, lease_until, last_error_code, failed_at
		FROM outbox_events
		WHERE id = $1
	`, event.ID).Scan(&status, &attempts, &claimToken, &claimOwner, &leaseUntil, &lastErrorCode, &failedAtStored); err != nil {
		t.Fatalf("read failure state: %v", err)
	}
	if status != domain.OutboxFailed || attempts != 2 || claimToken != nil || claimOwner != nil || leaseUntil != nil ||
		failedAtStored == nil || !failedAtStored.Equal(failedAt) {
		t.Fatalf("failure state = %s/%d/%v/%v/%v/%v, want failed/2/nil/nil/nil/%s",
			status, attempts, claimToken, claimOwner, leaseUntil, failedAtStored, failedAt)
	}
	if len([]rune(lastErrorCode)) > 128 {
		t.Fatalf("failure error code length = %d, want <= 128", len([]rune(lastErrorCode)))
	}

	published := &domain.OutboxEvent{
		AggregateType: "job",
		AggregateID:   uuid.New(),
		EventType:     outboxrelay.EventJobResultReady,
		NextAttemptAt: now,
	}
	if err := outbox.Add(ctx, published); err != nil {
		t.Fatalf("add published event: %v", err)
	}
	claimed, err = outbox.ClaimPending(ctx, "relay-a", now, now.Add(time.Minute), 1)
	if err != nil || len(claimed) != 1 || claimed[0].ClaimToken == nil || claimed[0].ID != published.ID {
		t.Fatalf("claim published event = %+v, %v; want event %s", claimed, err, published.ID)
	}
	updated, err = outbox.MarkPublishedClaimed(ctx, published.ID, *claimed[0].ClaimToken, now.Add(time.Second))
	if err != nil || !updated {
		t.Fatalf("publish = %v, %v; want true, nil", updated, err)
	}
	again, err := outbox.ClaimPending(ctx, "relay-c", now.Add(24*time.Hour), now.Add(25*time.Hour), 10)
	if err != nil || len(again) != 0 {
		t.Fatalf("claim terminal rows = %d, %v; want 0, nil", len(again), err)
	}
}

func TestReconciliationPostgresUOWRollsBackStagedEvents(t *testing.T) {
	ctx := context.Background()
	pool := preparedAccountIntegrationPool(t, ctx)
	jobs := postgres.NewJobRepository(pool)
	accountID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO accounts (id) VALUES ($1)`, accountID); err != nil {
		t.Fatalf("insert account: %v", err)
	}
	for i := range 2 {
		job := postgresReadyJob(accountID, "rollback-"+string(rune('a'+i)))
		if err := jobs.Create(ctx, job); err != nil {
			t.Fatalf("create ready job %d: %v", i, err)
		}
	}
	insertFailure := errors.New("injected postgres outbox failure")
	manager := &failAfterOutboxInsertManager{
		delegate: postgres.NewUnitOfWork(pool),
		err:      insertFailure,
	}

	if _, err := resultreadyreconciliation.New(
		postgres.NewResultReadyCandidateRepository(pool),
		allowResultReadyReadiness{},
		manager,
	).Reconcile(ctx, 10); !errors.Is(err, insertFailure) {
		t.Fatalf("reconcile error = %v, want injected insert failure", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM outbox_events
		WHERE event_type = $1
	`, outboxrelay.EventJobResultReady).Scan(&count); err != nil {
		t.Fatalf("count rolled-back events: %v", err)
	}
	if count != 0 {
		t.Fatalf("result-ready events after rollback = %d, want 0", count)
	}
}

func TestReconciliationPostgresHonorsRandomIDSemanticEvent(t *testing.T) {
	ctx := context.Background()
	pool := preparedAccountIntegrationPool(t, ctx)
	jobs := postgres.NewJobRepository(pool)
	outbox := postgres.NewOutboxRepository(pool)
	accountID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO accounts (id) VALUES ($1)`, accountID); err != nil {
		t.Fatalf("insert account: %v", err)
	}
	job := postgresReadyJob(accountID, "random-existing")
	if err := jobs.Create(ctx, job); err != nil {
		t.Fatalf("create ready job: %v", err)
	}
	existing := &domain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "job",
		AggregateID:   job.ID,
		EventType:     outboxrelay.EventJobResultReady,
		Payload:       json.RawMessage(`{"historical":true}`),
		Status:        domain.OutboxPublished,
	}
	if err := outbox.Add(ctx, existing); err != nil {
		t.Fatalf("add random-ID event: %v", err)
	}

	result, err := resultreadyreconciliation.New(
		postgres.NewResultReadyCandidateRepository(pool),
		allowResultReadyReadiness{},
		postgres.NewUnitOfWork(pool),
	).Reconcile(ctx, 10)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if result != (resultreadyreconciliation.Result{}) {
		t.Fatalf("result = %+v, want semantic anti-join to exclude existing row", result)
	}
	var count int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM outbox_events
		WHERE aggregate_type = 'job' AND aggregate_id = $1 AND event_type = $2
	`, job.ID, outboxrelay.EventJobResultReady).Scan(&count); err != nil {
		t.Fatalf("count semantic events: %v", err)
	}
	if count != 1 {
		t.Fatalf("semantic events = %d, want 1", count)
	}
}

func postgresReadyJob(accountID uuid.UUID, key string) *domain.Job {
	return &domain.Job{
		ID:             uuid.New(),
		AccountID:      accountID,
		Source:         "web",
		ResultMode:     domain.ResultModeAccountHistory,
		OperationType:  domain.OperationTextGenerate,
		Modality:       domain.ModalityText,
		Status:         domain.JobStatusResultReady,
		IdempotencyKey: key + "-" + uuid.NewString(),
		CorrelationID:  "postgres-reconcile",
	}
}

type failAfterOutboxInsertManager struct {
	delegate uow.Manager
	err      error
}

type allowResultReadyReadiness struct{}

func (allowResultReadyReadiness) RequireCompletionReady(
	context.Context,
	uuid.UUID,
	uuid.UUID,
) error {
	return nil
}

func (m *failAfterOutboxInsertManager) Within(
	ctx context.Context,
	fn func(context.Context, uow.Repositories) error,
) error {
	return m.delegate.Within(ctx, func(ctx context.Context, repos uow.Repositories) error {
		repos.Outbox = &failAfterOneInsertOutbox{
			OutboxRepository: repos.Outbox,
			err:              m.err,
		}
		return fn(ctx, repos)
	})
}

type failAfterOneInsertOutbox struct {
	domain.OutboxRepository
	inserted int
	err      error
}

func (r *failAfterOneInsertOutbox) AddIfAbsentByID(
	ctx context.Context,
	event *domain.OutboxEvent,
) (bool, error) {
	if r.inserted == 1 {
		return false, r.err
	}
	created, err := r.OutboxRepository.AddIfAbsentByID(ctx, event)
	if created {
		r.inserted++
	}
	return created, err
}

func testNow() time.Time {
	return time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
}
