package postgres_test

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"sync"
	"testing"
	"time"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
)

func TestAtomicSessionRotationPostgres(t *testing.T) {
	ctx := context.Background()
	// This helper creates and cleans up its own random schema, never live tables.
	pool := conversationManagementIntegrationPool(t, ctx)
	_, err := pool.Exec(ctx, `CREATE TABLE account_sessions (
 id UUID PRIMARY KEY, account_id UUID NOT NULL REFERENCES accounts(id), identity_id UUID,
 access_token_hash TEXT UNIQUE, access_expires_at TIMESTAMPTZ, refresh_token_hash TEXT NOT NULL UNIQUE,
 device_id TEXT NOT NULL, ip_hash TEXT NOT NULL, user_agent_hash TEXT NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL, revoked_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	accountID := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO accounts(id) VALUES($1)", accountID); err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewAccountSessionRepository(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	accessExpires := now.Add(15 * time.Minute)
	makeSession := func() domain.AccountSession {
		return domain.AccountSession{ID: uuid.New(), AccountID: accountID, AccessTokenHash: "sha256:" + uuid.NewString(), AccessExpiresAt: &accessExpires,
			RefreshTokenHash: "sha256:" + uuid.NewString(), DeviceID: "sha256:synthetic", IPHash: "sha256:synthetic", UserAgentHash: "sha256:synthetic",
			CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(time.Hour)}
	}
	old, err := repo.CreateSession(ctx, makeSession())
	if err != nil {
		t.Fatal(err)
	}
	duplicate := makeSession()
	duplicate.ID = old.ID
	if _, err := repo.RotateSession(ctx, old.RefreshTokenHash, duplicate); err == nil {
		t.Fatal("expected insert conflict")
	}
	unchanged, err := repo.FindSessionByRefreshHash(ctx, old.RefreshTokenHash)
	if err != nil || unchanged.RevokedAt != nil {
		t.Fatal("failed insert committed a revocation")
	}
	var group sync.WaitGroup
	results := make(chan error, 4)
	for i := 0; i < 4; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := repo.RotateSession(ctx, old.RefreshTokenHash, makeSession())
			results <- err
		}()
	}
	group.Wait()
	close(results)
	winners := 0
	for err := range results {
		if err == nil {
			winners++
		} else if !errors.Is(err, domain.ErrNotFound) {
			t.Fatal(err)
		}
	}
	if winners != 1 {
		t.Fatalf("successful rotations = %d, want 1", winners)
	}
}
