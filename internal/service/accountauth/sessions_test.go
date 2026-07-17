package accountauth_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/identityresolver"
)

func TestIssueRefreshRevokeSessionStoresOnlyHashes(t *testing.T) {
	ctx := context.Background()
	sessionRepo := memory.NewAccountSessionRepo()
	service := accountauth.New(
		identityresolver.New(memory.NewUserRepo(), memory.NewAccountIdentityRepo(), nil),
		accountauth.WithSessionRepository(sessionRepo),
		accountauth.WithSessionTTL(time.Hour),
	)
	accountID := uuid.New()

	tokens, err := service.IssueSession(ctx, accountID, accountauth.SessionMetadata{
		DeviceInfo: "Chrome on Windows",
		IP:         "127.0.0.1",
		UserAgent:  "UnitTest/1.0",
	})
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("tokens were not issued: %+v", tokens)
	}
	if tokens.Session.AccountID != accountID || tokens.Session.ID == uuid.Nil {
		t.Fatalf("unexpected safe session: %+v", tokens.Session)
	}

	rawJSON, err := json.Marshal(tokens.Session)
	if err != nil {
		t.Fatalf("marshal safe session: %v", err)
	}
	for _, forbidden := range []string{"Chrome", "127.0.0.1", "UnitTest", "refresh_token_hash", "device_id", "ip_hash", "user_agent_hash"} {
		if strings.Contains(string(rawJSON), forbidden) {
			t.Fatalf("safe session leaked %q: %s", forbidden, rawJSON)
		}
	}

	stored, err := sessionRepo.FindSessionByRefreshHash(ctx, "sha256:"+hashForTest("refresh", tokens.RefreshToken))
	if err != nil {
		t.Fatalf("find stored session: %v", err)
	}
	if stored.RefreshTokenHash == tokens.RefreshToken ||
		strings.Contains(stored.DeviceID, "Chrome") ||
		strings.Contains(stored.IPHash, "127.0.0.1") ||
		strings.Contains(stored.UserAgentHash, "UnitTest") {
		t.Fatalf("session stored raw sensitive material: %+v", stored)
	}

	refreshed, err := service.RefreshSession(ctx, tokens.RefreshToken, accountauth.SessionMetadata{DeviceInfo: "Phone"})
	if err != nil {
		t.Fatalf("refresh session: %v", err)
	}
	if refreshed.RefreshToken == "" || refreshed.RefreshToken == tokens.RefreshToken {
		t.Fatalf("refresh token was not rotated: old=%q new=%q", tokens.RefreshToken, refreshed.RefreshToken)
	}
	if _, err := service.RefreshSession(ctx, tokens.RefreshToken, accountauth.SessionMetadata{}); !errors.Is(err, accountauth.ErrInvalidSession) {
		t.Fatalf("old refresh error = %v, want invalid session", err)
	}

	revoked, err := service.RevokeSession(ctx, accountID, refreshed.Session.ID)
	if err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	if !revoked.Revoked {
		t.Fatalf("session was not marked revoked: %+v", revoked)
	}
}

func TestSessionExpiryAndLogout(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	sessionRepo := memory.NewAccountSessionRepo()
	service := accountauth.New(
		identityresolver.New(memory.NewUserRepo(), memory.NewAccountIdentityRepo(), nil),
		accountauth.WithSessionRepository(sessionRepo),
		accountauth.WithSessionTTL(time.Second),
		accountauth.WithClock(func() time.Time { return now }),
	)
	accountID := uuid.New()
	tokens, err := service.IssueSession(ctx, accountID, accountauth.SessionMetadata{})
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	expired := accountauth.New(
		identityresolver.New(memory.NewUserRepo(), memory.NewAccountIdentityRepo(), nil),
		accountauth.WithSessionRepository(sessionRepo),
		accountauth.WithSessionTTL(time.Second),
		accountauth.WithClock(func() time.Time { return now.Add(2 * time.Second) }),
	)
	if _, err := expired.RefreshSession(ctx, tokens.RefreshToken, accountauth.SessionMetadata{}); !errors.Is(err, accountauth.ErrSessionExpired) {
		t.Fatalf("expired refresh error = %v, want %v", err, accountauth.ErrSessionExpired)
	}

	fresh, err := service.IssueSession(ctx, accountID, accountauth.SessionMetadata{})
	if err != nil {
		t.Fatalf("issue second session: %v", err)
	}
	if err := service.Logout(ctx, fresh.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if err := service.Logout(ctx, fresh.RefreshToken); err != nil {
		t.Fatalf("logout should be idempotent: %v", err)
	}
	if _, err := service.RefreshSession(ctx, fresh.RefreshToken, accountauth.SessionMetadata{}); !errors.Is(err, accountauth.ErrInvalidSession) {
		t.Fatalf("logged out refresh error = %v, want invalid session", err)
	}
}

func TestSessionServiceRequiresRepository(t *testing.T) {
	service := accountauth.New(identityresolver.New(memory.NewUserRepo(), memory.NewAccountIdentityRepo(), nil))
	if _, err := service.IssueSession(context.Background(), uuid.New(), accountauth.SessionMetadata{}); !errors.Is(err, accountauth.ErrSessionStoreUnavailable) {
		t.Fatalf("issue error = %v, want session store unavailable", err)
	}
}

func hashForTest(scope, value string) string {
	sum := sha256.Sum256([]byte(scope + ":" + value))
	return hex.EncodeToString(sum[:])
}
