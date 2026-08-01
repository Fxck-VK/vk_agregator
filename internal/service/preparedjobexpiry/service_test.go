package preparedjobexpiry

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
)

func TestReconcileAccountExpiresOnlyDueOwnedWebImagePreparations(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	owner := uuid.New()
	foreign := uuid.New()
	jobs := memory.NewJobRepo()

	dueOwned := expiryJob(owner, "due-owned", now.Add(-time.Second))
	for _, job := range []*domain.Job{
		dueOwned,
		expiryJob(owner, "future-owned", now.Add(time.Second)),
		expiryJob(foreign, "due-foreign", now.Add(-time.Second)),
		{
			ID:             uuid.New(),
			AccountID:      owner,
			Source:         "miniapp",
			OperationType:  domain.OperationImageGenerate,
			Modality:       domain.ModalityImage,
			Status:         domain.JobStatusPrepared,
			IdempotencyKey: "due-miniapp",
			ExpiresAt:      timePtr(now.Add(-time.Second)),
		},
	} {
		if err := jobs.Create(ctx, job); err != nil {
			t.Fatalf("create job %q: %v", job.IdempotencyKey, err)
		}
	}

	service := New(memory.NewPreparedWebImageExpiryRepository(jobs), WithClock(func() time.Time { return now }))
	result, err := service.ReconcileAccount(ctx, owner, 10)
	if err != nil {
		t.Fatalf("reconcile owner: %v", err)
	}
	if result.Expired != 1 || result.HasMore {
		t.Fatalf("result = %+v, want one expired job", result)
	}

	stored, err := jobs.GetByIDForAccount(ctx, owner, dueOwned.ID)
	if err != nil {
		t.Fatalf("read expired job: %v", err)
	}
	if stored.Status != domain.JobStatusExpired || stored.ErrorCode != domain.PreparedConfirmationExpiredCode || stored.ErrorMessage != domain.PreparedConfirmationExpiredMessage {
		t.Fatalf("expired job = status:%s code:%q message:%q", stored.Status, stored.ErrorCode, stored.ErrorMessage)
	}
	for _, id := range []uuid.UUID{
		mustJobID(t, jobs, owner, "future-owned"),
		mustJobID(t, jobs, foreign, "due-foreign"),
		mustJobID(t, jobs, owner, "due-miniapp"),
	} {
		stored, err := jobs.GetByID(ctx, id)
		if err != nil {
			t.Fatalf("read untouched job: %v", err)
		}
		if stored.Status != domain.JobStatusPrepared || stored.ErrorCode != "" || stored.ErrorMessage != "" {
			t.Fatalf("untouched job %s = %+v", id, stored)
		}
	}
}

func TestReconcileIsBoundedAndCanDrainInPages(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	jobs := memory.NewJobRepo()
	for i := 0; i < 3; i++ {
		if err := jobs.Create(ctx, expiryJob(uuid.New(), fmt.Sprintf("due-global-%d", i), now.Add(-time.Minute))); err != nil {
			t.Fatalf("create due job %d: %v", i, err)
		}
	}
	service := New(memory.NewPreparedWebImageExpiryRepository(jobs), WithClock(func() time.Time { return now }))

	first, err := service.Reconcile(ctx, 2)
	if err != nil {
		t.Fatalf("first global reconcile: %v", err)
	}
	if first.Expired != 2 || !first.HasMore {
		t.Fatalf("first result = %+v, want two expired with more", first)
	}
	second, err := service.Reconcile(ctx, 2)
	if err != nil {
		t.Fatalf("second global reconcile: %v", err)
	}
	if second.Expired != 1 || second.HasMore {
		t.Fatalf("second result = %+v, want final expired job", second)
	}
}

func TestConcurrentReconciliationClaimsPreparedJobsOnlyOnce(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	jobs := memory.NewJobRepo()
	for i := 0; i < 2; i++ {
		if err := jobs.Create(ctx, expiryJob(uuid.New(), fmt.Sprintf("due-concurrent-%d", i), now.Add(-time.Minute))); err != nil {
			t.Fatalf("create due job %d: %v", i, err)
		}
	}
	service := New(memory.NewPreparedWebImageExpiryRepository(jobs), WithClock(func() time.Time { return now }))
	type reconciliationResult struct {
		result Result
		err    error
	}
	results := make(chan reconciliationResult, 2)
	for range 2 {
		go func() {
			result, err := service.Reconcile(ctx, 1)
			results <- reconciliationResult{result: result, err: err}
		}()
	}

	totalExpired := 0
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Fatalf("concurrent reconcile: %v", result.err)
		}
		totalExpired += result.result.Expired
	}
	if totalExpired != 2 {
		t.Fatalf("concurrent expired total = %d, want each due job claimed once", totalExpired)
	}
}

func TestReconcileJobUsesExactAccountAndJobScope(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	owner := uuid.New()
	foreign := uuid.New()
	jobs := memory.NewJobRepo()
	due := expiryJob(owner, "due-targeted", now.Add(-time.Second))
	if err := jobs.Create(ctx, due); err != nil {
		t.Fatalf("create due job: %v", err)
	}
	service := New(memory.NewPreparedWebImageExpiryRepository(jobs), WithClock(func() time.Time { return now }))

	changed, err := service.ReconcileJob(ctx, foreign, due.ID)
	if err != nil {
		t.Fatalf("foreign targeted reconcile: %v", err)
	}
	if changed {
		t.Fatal("foreign account changed an owned job")
	}
	stored, err := jobs.GetByID(ctx, due.ID)
	if err != nil || stored.Status != domain.JobStatusPrepared {
		t.Fatalf("foreign target result = %+v, %v", stored, err)
	}

	changed, err = service.ReconcileJob(ctx, owner, due.ID)
	if err != nil || !changed {
		t.Fatalf("owner targeted reconcile = changed:%t err:%v", changed, err)
	}
	stored, err = jobs.GetByID(ctx, due.ID)
	if err != nil || stored.Status != domain.JobStatusExpired {
		t.Fatalf("owner target result = %+v, %v", stored, err)
	}
}

func expiryJob(accountID uuid.UUID, key string, expiresAt time.Time) *domain.Job {
	return &domain.Job{
		ID:             uuid.New(),
		AccountID:      accountID,
		Source:         "web",
		OperationType:  domain.OperationImageGenerate,
		Modality:       domain.ModalityImage,
		Status:         domain.JobStatusPrepared,
		IdempotencyKey: key,
		ExpiresAt:      timePtr(expiresAt),
	}
}

func timePtr(value time.Time) *time.Time { return &value }

func mustJobID(t *testing.T, jobs *memory.JobRepo, accountID uuid.UUID, key string) uuid.UUID {
	t.Helper()
	job, err := jobs.GetByIdempotencyKeyForAccount(context.Background(), accountID, key)
	if err != nil {
		t.Fatalf("read job %q: %v", key, err)
	}
	return job.ID
}
