package antispam

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

func TestMessageLimitUsesExistingUserProfile(t *testing.T) {
	now := fixedNow()
	svc := newTestService(t, DefaultConfig(), now, nil)
	user := existingUser(now)
	in := CheckInput{User: user, VKUserID: user.VKUserID, CommandType: domain.CommandShowMenu}

	for i := 0; i < 10; i++ {
		decision, err := svc.Check(context.Background(), in)
		if err != nil {
			t.Fatalf("check %d: %v", i, err)
		}
		if !decision.Allowed {
			t.Fatalf("check %d denied: %+v", i, decision)
		}
	}

	decision, err := svc.Check(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed || decision.Kind != DecisionCooldown {
		t.Fatalf("decision = %+v, want cooldown denial", decision)
	}
	if decision.Message == "" || decision.RetryAfter <= 0 {
		t.Fatalf("cooldown decision should include user message and retry-after: %+v", decision)
	}
}

func TestNewUsersHaveStricterLimits(t *testing.T) {
	now := fixedNow()
	svc := newTestService(t, DefaultConfig(), now, nil)
	user := newUser(now)
	in := CheckInput{User: user, VKUserID: user.VKUserID, CommandType: domain.CommandShowMenu}

	for i := 0; i < 5; i++ {
		decision, err := svc.Check(context.Background(), in)
		if err != nil {
			t.Fatalf("check %d: %v", i, err)
		}
		if !decision.Allowed {
			t.Fatalf("check %d denied: %+v", i, decision)
		}
	}

	decision, err := svc.Check(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed || decision.Kind != DecisionCooldown {
		t.Fatalf("decision = %+v, want new-user cooldown denial", decision)
	}
}

func TestGPTLimitIsSeparateFromMessageLimit(t *testing.T) {
	now := fixedNow()
	svc := newTestService(t, DefaultConfig(), now, nil)
	user := existingUser(now)
	in := CheckInput{
		User:        user,
		VKUserID:    user.VKUserID,
		CommandType: domain.CommandTextAsk,
		Operation:   domain.OperationTextGenerate,
		CreatesJob:  true,
	}

	for i := 0; i < 3; i++ {
		decision, err := svc.Check(context.Background(), in)
		if err != nil {
			t.Fatalf("check %d: %v", i, err)
		}
		if !decision.Allowed {
			t.Fatalf("check %d denied: %+v", i, decision)
		}
	}

	decision, err := svc.Check(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed || decision.Kind != DecisionCooldown {
		t.Fatalf("decision = %+v, want GPT cooldown denial", decision)
	}
}

func TestRepeatedViolationsBecomeTemporaryBlock(t *testing.T) {
	now := fixedNow()
	cfg := DefaultConfig()
	cfg.MessageLimit = 1
	cfg.ViolationLimit = 3
	svc := newTestService(t, cfg, now, nil)
	user := existingUser(now)
	in := CheckInput{User: user, VKUserID: user.VKUserID, CommandType: domain.CommandShowMenu}

	first, err := svc.Check(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Allowed {
		t.Fatalf("first check denied: %+v", first)
	}

	for i := 0; i < 2; i++ {
		decision, err := svc.Check(context.Background(), in)
		if err != nil {
			t.Fatalf("violation %d: %v", i, err)
		}
		if decision.Kind != DecisionCooldown {
			t.Fatalf("violation %d decision = %+v, want cooldown", i, decision)
		}
	}

	decision, err := svc.Check(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed || decision.Kind != DecisionTemporaryBlock {
		t.Fatalf("decision = %+v, want temporary block", decision)
	}
}

func TestActiveGPTJobsBlockQueueFlood(t *testing.T) {
	now := fixedNow()
	jobs := &fakeActiveJobs{count: 2}
	svc := newTestService(t, DefaultConfig(), now, jobs)
	user := existingUser(now)
	in := CheckInput{
		User:        user,
		VKUserID:    user.VKUserID,
		CommandType: domain.CommandTextAsk,
		Operation:   domain.OperationTextGenerate,
		CreatesJob:  true,
	}

	decision, err := svc.Check(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed || decision.Kind != DecisionActiveJobs {
		t.Fatalf("decision = %+v, want active-jobs denial", decision)
	}
}

func newTestService(t *testing.T, cfg Config, now time.Time, jobs ActiveJobCounter) *Service {
	t.Helper()
	store := newMemoryStore(now)
	if jobs == nil {
		jobs = &fakeActiveJobs{}
	}
	return New(store, jobs, cfg).WithClock(func() time.Time { return store.now })
}

func fixedNow() time.Time {
	return time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
}

func existingUser(now time.Time) *domain.User {
	return &domain.User{
		ID:          uuid.New(),
		VKUserID:    123,
		FirstSeenAt: now.Add(-5 * time.Hour),
		CreatedAt:   now.Add(-5 * time.Hour),
	}
}

func newUser(now time.Time) *domain.User {
	return &domain.User{
		ID:          uuid.New(),
		VKUserID:    456,
		FirstSeenAt: now.Add(-30 * time.Minute),
		CreatedAt:   now.Add(-30 * time.Minute),
	}
}

type fakeActiveJobs struct {
	count int
	err   error
}

func (f *fakeActiveJobs) CountActiveByUserOperation(context.Context, uuid.UUID, domain.OperationType) (int, error) {
	return f.count, f.err
}

type memoryStore struct {
	now     time.Time
	entries map[string]memoryEntry
}

type memoryEntry struct {
	count     int64
	expiresAt time.Time
}

func newMemoryStore(now time.Time) *memoryStore {
	return &memoryStore{now: now, entries: map[string]memoryEntry{}}
}

func (s *memoryStore) Increment(_ context.Context, key string, window time.Duration) (int64, time.Duration, error) {
	entry := s.entry(key)
	if entry.count == 0 {
		entry.expiresAt = s.now.Add(window)
	}
	entry.count++
	s.entries[key] = entry
	return entry.count, entry.expiresAt.Sub(s.now), nil
}

func (s *memoryStore) TTL(_ context.Context, key string) (time.Duration, error) {
	entry := s.entry(key)
	if entry.count == 0 {
		return 0, nil
	}
	return entry.expiresAt.Sub(s.now), nil
}

func (s *memoryStore) SetTTL(_ context.Context, key string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	s.entries[key] = memoryEntry{count: 1, expiresAt: s.now.Add(ttl)}
	return nil
}

func (s *memoryStore) entry(key string) memoryEntry {
	entry := s.entries[key]
	if entry.expiresAt.IsZero() || !s.now.Before(entry.expiresAt) {
		delete(s.entries, key)
		return memoryEntry{}
	}
	return entry
}
