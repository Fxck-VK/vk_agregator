package memory

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

func TestMemoryOutboxClaimLeaseOwnership(t *testing.T) {
	ctx := context.Background()
	repo := NewOutboxRepo()
	now := time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC)
	leaseUntil := now.Add(time.Minute)
	event := &domain.OutboxEvent{
		AggregateType: "job",
		AggregateID:   uuid.New(),
		EventType:     "event.job.created",
		NextAttemptAt: now,
	}
	if err := repo.Add(ctx, event); err != nil {
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
			claimed, err := repo.ClaimPending(ctx, owner, now, leaseUntil, 1)
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
	if first == nil || first.Status != domain.OutboxStatusProcessing || first.ClaimToken == nil ||
		first.ClaimOwner == "" || first.LeaseUntil == nil || !first.LeaseUntil.Equal(leaseUntil) {
		t.Fatalf("first claim = %+v, want processing claim with owner/token/lease", first)
	}

	reclaimed, err := repo.ClaimPending(ctx, "relay-c", leaseUntil, leaseUntil.Add(time.Minute), 1)
	if err != nil || len(reclaimed) != 1 {
		t.Fatalf("reclaim = %d, %v; want 1, nil", len(reclaimed), err)
	}
	if reclaimed[0].ClaimToken == nil || *reclaimed[0].ClaimToken == *first.ClaimToken {
		t.Fatalf("reclaimed token = %v, want a fresh token", reclaimed[0].ClaimToken)
	}
	staleToken := *first.ClaimToken
	if updated, err := repo.MarkPublishedClaimed(ctx, event.ID, staleToken, now); err != nil || updated {
		t.Fatalf("stale mark = %v, %v; want false, nil", updated, err)
	}
	if updated, err := repo.RetryClaimed(ctx, event.ID, staleToken, now, "stale"); err != nil || updated {
		t.Fatalf("stale retry = %v, %v; want false, nil", updated, err)
	}
	if updated, err := repo.FailClaimed(ctx, event.ID, staleToken, now, "stale"); err != nil || updated {
		t.Fatalf("stale fail = %v, %v; want false, nil", updated, err)
	}
}

func TestMemoryOutboxClaimResolveTransitions(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 31, 9, 0, 0, 0, time.UTC)
	longErrorCode := strings.Repeat("x", 256)

	t.Run("retry clears lease and increments attempts", func(t *testing.T) {
		repo := NewOutboxRepo()
		event := addReadyMemoryOutboxEvent(t, repo, now)
		claimed, err := repo.ClaimPending(ctx, "relay-a", now, now.Add(time.Minute), 1)
		if err != nil || len(claimed) != 1 || claimed[0].ClaimToken == nil {
			t.Fatalf("claim = %d, %v; want claimed token", len(claimed), err)
		}
		nextAttemptAt := now.Add(2 * time.Minute)
		updated, err := repo.RetryClaimed(ctx, event.ID, *claimed[0].ClaimToken, nextAttemptAt, longErrorCode)
		if err != nil || !updated {
			t.Fatalf("retry = %v, %v; want true, nil", updated, err)
		}
		stored := memoryOutboxEventByID(t, repo, event.ID)
		if stored.Status != domain.OutboxPending || stored.Attempts != 1 || !stored.NextAttemptAt.Equal(nextAttemptAt) {
			t.Fatalf("retry state = %+v, want pending attempt 1 at next attempt", stored)
		}
		if stored.ClaimToken != nil || stored.ClaimOwner != "" || stored.LeaseUntil != nil {
			t.Fatalf("retry lease fields = %v/%q/%v, want cleared", stored.ClaimToken, stored.ClaimOwner, stored.LeaseUntil)
		}
		if len([]rune(stored.LastErrorCode)) > 128 {
			t.Fatalf("retry error code length = %d, want <= 128", len([]rune(stored.LastErrorCode)))
		}
	})

	t.Run("terminal failure clears lease and records bounded error", func(t *testing.T) {
		repo := NewOutboxRepo()
		event := addReadyMemoryOutboxEvent(t, repo, now)
		claimed, err := repo.ClaimPending(ctx, "relay-a", now, now.Add(time.Minute), 1)
		if err != nil || len(claimed) != 1 || claimed[0].ClaimToken == nil {
			t.Fatalf("claim = %d, %v; want claimed token", len(claimed), err)
		}
		failedAt := now.Add(30 * time.Second)
		updated, err := repo.FailClaimed(ctx, event.ID, *claimed[0].ClaimToken, failedAt, longErrorCode)
		if err != nil || !updated {
			t.Fatalf("fail = %v, %v; want true, nil", updated, err)
		}
		stored := memoryOutboxEventByID(t, repo, event.ID)
		if stored.Status != domain.OutboxFailed || stored.Attempts != 1 || stored.FailedAt == nil || !stored.FailedAt.Equal(failedAt) {
			t.Fatalf("failed state = %+v, want failed attempt 1 with failed_at", stored)
		}
		if stored.ClaimToken != nil || stored.ClaimOwner != "" || stored.LeaseUntil != nil {
			t.Fatalf("failure lease fields = %v/%q/%v, want cleared", stored.ClaimToken, stored.ClaimOwner, stored.LeaseUntil)
		}
		if len([]rune(stored.LastErrorCode)) > 128 {
			t.Fatalf("failure error code length = %d, want <= 128", len([]rune(stored.LastErrorCode)))
		}
	})

	t.Run("published rows never reappear", func(t *testing.T) {
		repo := NewOutboxRepo()
		event := addReadyMemoryOutboxEvent(t, repo, now)
		claimed, err := repo.ClaimPending(ctx, "relay-a", now, now.Add(time.Minute), 1)
		if err != nil || len(claimed) != 1 || claimed[0].ClaimToken == nil {
			t.Fatalf("claim = %d, %v; want claimed token", len(claimed), err)
		}
		updated, err := repo.MarkPublishedClaimed(ctx, event.ID, *claimed[0].ClaimToken, now.Add(time.Second))
		if err != nil || !updated {
			t.Fatalf("publish = %v, %v; want true, nil", updated, err)
		}
		again, err := repo.ClaimPending(ctx, "relay-b", now.Add(24*time.Hour), now.Add(25*time.Hour), 1)
		if err != nil || len(again) != 0 {
			t.Fatalf("claim published = %d, %v; want 0, nil", len(again), err)
		}
	})
}

func TestMemoryOutboxHealthSnapshotReturnsOnlySafeAggregates(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	repo := NewOutboxRepo()
	for range 4 {
		addReadyMemoryOutboxEvent(t, repo, now)
	}

	claimed, err := repo.ClaimPending(ctx, "relay-a", now, now.Add(time.Minute), 3)
	if err != nil || len(claimed) != 3 {
		t.Fatalf("claim three events = (%d, %v), want (3, nil)", len(claimed), err)
	}
	if ok, err := repo.MarkPublishedClaimed(ctx, claimed[0].ID, *claimed[0].ClaimToken, now); err != nil || !ok {
		t.Fatalf("publish first claim = (%v, %v), want (true, nil)", ok, err)
	}
	if ok, err := repo.RetryClaimed(ctx, claimed[1].ID, *claimed[1].ClaimToken, now.Add(time.Hour), "retry"); err != nil || !ok {
		t.Fatalf("retry second claim = (%v, %v), want (true, nil)", ok, err)
	}
	if ok, err := repo.FailClaimed(ctx, claimed[2].ID, *claimed[2].ClaimToken, now, "failed"); err != nil || !ok {
		t.Fatalf("fail third claim = (%v, %v), want (true, nil)", ok, err)
	}
	remaining, err := repo.ClaimPending(ctx, "relay-b", now, now.Add(time.Second), 1)
	if err != nil || len(remaining) != 1 {
		t.Fatalf("claim remaining event = (%d, %v), want (1, nil)", len(remaining), err)
	}

	snapshot, err := repo.OutboxHealthSnapshot(ctx, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("health snapshot: %v", err)
	}
	if snapshot.Pending != 1 ||
		snapshot.Processing != 1 ||
		snapshot.Failed != 1 ||
		snapshot.ExpiredLeases != 1 ||
		snapshot.OldestPendingCreatedAt == nil {
		t.Fatalf("safe outbox health snapshot = %+v, want pending/processing/failed/expired 1 and oldest timestamp", snapshot)
	}
}

func addReadyMemoryOutboxEvent(t *testing.T, repo *OutboxRepo, now time.Time) *domain.OutboxEvent {
	t.Helper()
	event := &domain.OutboxEvent{
		AggregateType: "job",
		AggregateID:   uuid.New(),
		EventType:     "event.job.created",
		NextAttemptAt: now,
	}
	if err := repo.Add(context.Background(), event); err != nil {
		t.Fatalf("add: %v", err)
	}
	return event
}

func memoryOutboxEventByID(t *testing.T, repo *OutboxRepo, id uuid.UUID) domain.OutboxEvent {
	t.Helper()
	for _, event := range repo.Events() {
		if event.ID == id {
			return event
		}
	}
	t.Fatalf("outbox event %s not found", id)
	return domain.OutboxEvent{}
}
