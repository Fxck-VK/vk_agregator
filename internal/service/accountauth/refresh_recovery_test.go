package accountauth_test

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"sync"
	"testing"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
)

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
