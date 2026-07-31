package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
)

func TestAccountSessionRepoRejectsDuplicateAccessHash(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewAccountSessionRepo()
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	accessExpiresAt := now.Add(15 * time.Minute)
	first := sessionForAccessHash("sha256:access", "sha256:refresh-one", now, accessExpiresAt)
	if _, err := repo.CreateSession(ctx, first); err != nil {
		t.Fatalf("create first session: %v", err)
	}
	second := sessionForAccessHash("sha256:access", "sha256:refresh-two", now, accessExpiresAt)
	if _, err := repo.CreateSession(ctx, second); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate access hash error = %v, want %v", err, domain.ErrConflict)
	}
}

func TestAccountSessionRepoRejectsDuplicateIDWithoutOverwritingMappings(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewAccountSessionRepo()
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	accessExpiresAt := now.Add(15 * time.Minute)
	first := sessionForAccessHash("sha256:access-one", "sha256:refresh-one", now, accessExpiresAt)
	if _, err := repo.CreateSession(ctx, first); err != nil {
		t.Fatalf("create first session: %v", err)
	}
	duplicate := sessionForAccessHash("sha256:access-two", "sha256:refresh-two", now, accessExpiresAt)
	duplicate.ID = first.ID
	if _, err := repo.CreateSession(ctx, duplicate); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate ID error = %v, want %v", err, domain.ErrConflict)
	}

	storedAccess, err := repo.FindSessionByAccessHash(ctx, first.AccessTokenHash)
	if err != nil {
		t.Fatalf("find original access mapping: %v", err)
	}
	storedRefresh, err := repo.FindSessionByRefreshHash(ctx, first.RefreshTokenHash)
	if err != nil {
		t.Fatalf("find original refresh mapping: %v", err)
	}
	if storedAccess.ID != first.ID || storedRefresh.ID != first.ID ||
		storedAccess.AccessTokenHash != first.AccessTokenHash ||
		storedRefresh.RefreshTokenHash != first.RefreshTokenHash {
		t.Fatalf("existing mappings were overwritten: access=%+v refresh=%+v", storedAccess, storedRefresh)
	}
	if _, err := repo.FindSessionByAccessHash(ctx, duplicate.AccessTokenHash); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("duplicate access mapping error = %v, want %v", err, domain.ErrNotFound)
	}
	if _, err := repo.FindSessionByRefreshHash(ctx, duplicate.RefreshTokenHash); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("duplicate refresh mapping error = %v, want %v", err, domain.ErrNotFound)
	}
}

func TestAccountSessionRepoRejectsIncompleteAccessPair(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewAccountSessionRepo()
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	session := sessionForAccessHash(" ", "sha256:refresh", now, now.Add(15*time.Minute))
	session.AccessExpiresAt = nil
	if _, err := repo.CreateSession(ctx, session); !errors.Is(err, domain.ErrInvalidIdentity) {
		t.Fatalf("incomplete access pair error = %v, want %v", err, domain.ErrInvalidIdentity)
	}
}

func sessionForAccessHash(accessHash, refreshHash string, now, accessExpiresAt time.Time) domain.AccountSession {
	return domain.AccountSession{
		ID:               uuid.New(),
		AccountID:        uuid.New(),
		AccessTokenHash:  accessHash,
		AccessExpiresAt:  &accessExpiresAt,
		RefreshTokenHash: refreshHash,
		DeviceID:         "sha256:device",
		IPHash:           "sha256:ip",
		UserAgentHash:    "sha256:ua",
		ExpiresAt:        now.Add(time.Hour),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}
