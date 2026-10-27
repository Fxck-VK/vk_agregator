package accountauth_test

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"sync"
	"testing"
	"time"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
)

func TestRefreshRetainsStoredPersistenceAndShortAbsoluteDeadline(t *testing.T) {
	ctx := context.Background()
	start := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	for _, initial := range []bool{false, true} {
		t.Run(map[bool]string{false: "short", true: "persistent"}[initial], func(t *testing.T) {
			now := start
			repo := memory.NewAccountSessionRepo()
			service := accountauth.New(nil, accountauth.WithSessionRepository(repo), accountauth.WithClock(func() time.Time { return now }))
			choice := initial
			tokens, err := service.IssueSession(ctx, uuid.New(), accountauth.SessionMetadata{Persistent: &choice})
			if err != nil {
				t.Fatal(err)
			}
			requestChoice := !choice
			now = start.Add(23*time.Hour + 59*time.Minute)
			rotated, err := service.RefreshSession(ctx, tokens.RefreshToken, accountauth.SessionMetadata{Persistent: &requestChoice})
			if err != nil {
				t.Fatal(err)
			}
			if rotated.Persistent == nil || *rotated.Persistent != choice {
				t.Fatal("refresh accepted a request-supplied persistence change")
			}
			want := start.Add(24 * time.Hour)
			if choice {
				want = now.Add(30 * 24 * time.Hour)
			}
			if rotated.ExpiresAt != want.Format(time.RFC3339) {
				t.Fatalf("refreshed expiry = %s, want %s", rotated.ExpiresAt, want.Format(time.RFC3339))
			}
			if !choice {
				if rotated.AccessExpiresAt != want.Format(time.RFC3339) {
					t.Fatal("access token outlives absolute short-session deadline")
				}
				now = start.Add(24 * time.Hour)
				if _, err := service.RefreshSession(ctx, rotated.RefreshToken, accountauth.SessionMetadata{Persistent: &requestChoice}); !errors.Is(err, accountauth.ErrSessionExpired) {
					t.Fatalf("absolute expiry error = %v", err)
				}
			}
		})
	}
}

func TestRefreshCannotExtendStoredLongPolicyAfterConfigurationIncrease(t *testing.T) {
	ctx := context.Background()
	start := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	repo := memory.NewAccountSessionRepo()
	choice := true
	service := accountauth.New(nil, accountauth.WithSessionRepository(repo), accountauth.WithSessionTTL(7*24*time.Hour), accountauth.WithClock(func() time.Time { return start }))
	tokens, err := service.IssueSession(ctx, uuid.New(), accountauth.SessionMetadata{Persistent: &choice})
	if err != nil {
		t.Fatal(err)
	}
	now := start.Add(time.Hour)
	changed := accountauth.New(nil, accountauth.WithSessionRepository(repo), accountauth.WithSessionTTL(30*24*time.Hour), accountauth.WithClock(func() time.Time { return now }))
	rotated, err := changed.RefreshSession(ctx, tokens.RefreshToken, accountauth.SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	if rotated.ExpiresAt != now.Add(7*24*time.Hour).Format(time.RFC3339) {
		t.Fatal("refresh extended the stored session duration")
	}
}

type failingSessionWrite struct {
	*memory.AccountSessionRepo
	fail bool
}

func TestConcurrentRefreshHasExactlyOneWinner(t *testing.T) {
	repo := memory.NewAccountSessionRepo()
	service := accountauth.New(nil, accountauth.WithSessionRepository(repo))
	tokens, err := service.IssueSession(context.Background(), uuid.New(), accountauth.SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, err := service.RefreshSession(context.Background(), tokens.RefreshToken, accountauth.SessionMetadata{})
			results <- err
		}()
	}
	workers.Wait()
	close(results)
	winners := 0
	for err := range results {
		if err == nil {
			winners++
		} else if !errors.Is(err, accountauth.ErrInvalidSession) {
			t.Fatal(err)
		}
	}
	if winners != 1 {
		t.Fatalf("successful refreshes = %d, want 1", winners)
	}
}

func (r *failingSessionWrite) CreateSession(ctx context.Context, session domain.AccountSession) (*domain.AccountSession, error) {
	if r.fail {
		return nil, errors.New("temporary storage failure")
	}
	return r.AccountSessionRepo.CreateSession(ctx, session)
}
func (r *failingSessionWrite) RotateSession(ctx context.Context, oldHash string, session domain.AccountSession) (*domain.AccountSession, error) {
	return nil, errors.New("temporary storage failure")
}
func TestFailedRefreshDoesNotRevokeExistingSession(t *testing.T) {
	repo := &failingSessionWrite{AccountSessionRepo: memory.NewAccountSessionRepo()}
	service := accountauth.New(nil, accountauth.WithSessionRepository(repo))
	tokens, err := service.IssueSession(context.Background(), uuid.New(), accountauth.SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	repo.fail = true
	if _, err = service.RefreshSession(context.Background(), tokens.RefreshToken, accountauth.SessionMetadata{}); err == nil {
		t.Fatal("expected storage failure")
	}
	if _, err = service.AuthenticateAccessToken(context.Background(), tokens.AccessToken); err != nil {
		t.Fatalf("existing session was revoked by failed refresh: %v", err)
	}
}
