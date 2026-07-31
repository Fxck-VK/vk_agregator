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
	"vk-ai-aggregator/internal/domain"
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

func TestRefreshSessionConsumesRefreshTokenOnlyOnceConcurrently(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	readsReady := make(chan struct{}, 2)
	releaseReads := make(chan struct{})
	repo := &refreshReadBarrier{
		AccountSessionRepository: memory.NewAccountSessionRepo(),
		readsReady:               readsReady,
		releaseReads:             releaseReads,
	}
	service := accountauth.New(
		identityresolver.New(memory.NewUserRepo(), memory.NewAccountIdentityRepo(), nil),
		accountauth.WithSessionRepository(repo),
		accountauth.WithClock(func() time.Time { return now }),
	)
	tokens, err := service.IssueSession(ctx, uuid.New(), accountauth.SessionMetadata{})
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}

	results := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := service.RefreshSession(ctx, tokens.RefreshToken, accountauth.SessionMetadata{})
			results <- err
		}()
	}
	for range 2 {
		<-readsReady
	}
	close(releaseReads)

	var succeeded, invalid int
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, accountauth.ErrInvalidSession):
			invalid++
		default:
			t.Fatalf("concurrent refresh error = %v, want success or %v", err, accountauth.ErrInvalidSession)
		}
	}
	if succeeded != 1 || invalid != 1 {
		t.Fatalf("concurrent refresh results: succeeded=%d invalid=%d, want 1 each", succeeded, invalid)
	}
}

func TestSessionServiceAuthenticatesIssuedAccessToken(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	repo := memory.NewAccountSessionRepo()
	service := accountauth.New(
		identityresolver.New(memory.NewUserRepo(), memory.NewAccountIdentityRepo(), nil),
		accountauth.WithSessionRepository(repo),
		accountauth.WithClock(func() time.Time { return now }),
	)
	accountID := uuid.New()

	tokens, err := service.IssueSession(ctx, accountID, accountauth.SessionMetadata{})
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	principal, err := service.AuthenticateAccessToken(ctx, tokens.AccessToken)
	if err != nil {
		t.Fatalf("authenticate access token: %v", err)
	}
	if principal.AccountID != accountID ||
		principal.SessionID != tokens.Session.ID ||
		principal.Method != domain.AuthenticationMethodAccountSession {
		t.Fatalf("unexpected principal: %+v", principal)
	}

	stored, err := repo.FindSessionByAccessHash(ctx, "sha256:"+hashForTest("access", tokens.AccessToken))
	if err != nil {
		t.Fatalf("find stored access session: %v", err)
	}
	if stored.AccessTokenHash == tokens.AccessToken || strings.Contains(stored.AccessTokenHash, tokens.AccessToken) {
		t.Fatalf("stored raw access token: %+v", stored)
	}
	if stored.AccessExpiresAt == nil || !stored.AccessExpiresAt.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("access expiry = %v, want %v", stored.AccessExpiresAt, now.Add(15*time.Minute))
	}
}

func TestSessionServiceRejectsRotatedAndExpiredAccessTokens(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	repo := memory.NewAccountSessionRepo()
	service := accountauth.New(
		identityresolver.New(memory.NewUserRepo(), memory.NewAccountIdentityRepo(), nil),
		accountauth.WithSessionRepository(repo),
		accountauth.WithAccessTokenTTL(time.Minute),
		accountauth.WithClock(func() time.Time { return now }),
	)
	tokens, err := service.IssueSession(ctx, uuid.New(), accountauth.SessionMetadata{})
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	refreshed, err := service.RefreshSession(ctx, tokens.RefreshToken, accountauth.SessionMetadata{})
	if err != nil {
		t.Fatalf("refresh session: %v", err)
	}
	if _, err := service.AuthenticateAccessToken(ctx, tokens.AccessToken); !errors.Is(err, accountauth.ErrInvalidSession) {
		t.Fatalf("rotated access error = %v, want %v", err, accountauth.ErrInvalidSession)
	}
	if _, err := service.AuthenticateAccessToken(ctx, refreshed.AccessToken); err != nil {
		t.Fatalf("authenticate refreshed access token: %v", err)
	}

	expired := accountauth.New(
		identityresolver.New(memory.NewUserRepo(), memory.NewAccountIdentityRepo(), nil),
		accountauth.WithSessionRepository(repo),
		accountauth.WithAccessTokenTTL(time.Minute),
		accountauth.WithClock(func() time.Time { return now.Add(time.Minute) }),
	)
	if _, err := expired.AuthenticateAccessToken(ctx, refreshed.AccessToken); !errors.Is(err, accountauth.ErrSessionExpired) {
		t.Fatalf("expired access error = %v, want %v", err, accountauth.ErrSessionExpired)
	}
}

func TestSessionAccessExpiryDoesNotOutliveRefreshSession(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	service := accountauth.New(
		identityresolver.New(memory.NewUserRepo(), memory.NewAccountIdentityRepo(), nil),
		accountauth.WithSessionRepository(memory.NewAccountSessionRepo()),
		accountauth.WithSessionTTL(time.Second),
		accountauth.WithClock(func() time.Time { return now }),
	)
	tokens, err := service.IssueSession(ctx, uuid.New(), accountauth.SessionMetadata{})
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	if tokens.AccessExpiresAt != now.Add(time.Second).Format(time.RFC3339) {
		t.Fatalf("access expiry = %q, want %q", tokens.AccessExpiresAt, now.Add(time.Second).Format(time.RFC3339))
	}
}

func TestSessionServiceAllowsLegacyRefreshOnlySessionWithoutAccessAuthentication(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	legacyRefresh := "legacy-refresh-token"
	repo := memory.NewAccountSessionRepo()
	legacy := domain.AccountSession{
		ID:               uuid.New(),
		AccountID:        uuid.New(),
		RefreshTokenHash: "sha256:" + hashForTest("refresh", legacyRefresh),
		DeviceID:         "sha256:device",
		IPHash:           "sha256:ip",
		UserAgentHash:    "sha256:ua",
		ExpiresAt:        now.Add(time.Hour),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if _, err := repo.CreateSession(ctx, legacy); err != nil {
		t.Fatalf("create legacy session: %v", err)
	}
	service := accountauth.New(
		identityresolver.New(memory.NewUserRepo(), memory.NewAccountIdentityRepo(), nil),
		accountauth.WithSessionRepository(repo),
		accountauth.WithClock(func() time.Time { return now }),
	)
	if _, err := service.AuthenticateAccessToken(ctx, "legacy-access-token"); !errors.Is(err, accountauth.ErrInvalidSession) {
		t.Fatalf("legacy access authentication error = %v, want %v", err, accountauth.ErrInvalidSession)
	}
	if _, err := service.RefreshSession(ctx, legacyRefresh, accountauth.SessionMetadata{}); err != nil {
		t.Fatalf("refresh legacy session: %v", err)
	}
}

func hashForTest(scope, value string) string {
	sum := sha256.Sum256([]byte(scope + ":" + value))
	return hex.EncodeToString(sum[:])
}

type refreshReadBarrier struct {
	domain.AccountSessionRepository
	readsReady   chan<- struct{}
	releaseReads <-chan struct{}
}

func (r *refreshReadBarrier) FindSessionByRefreshHash(ctx context.Context, refreshTokenHash string) (*domain.AccountSession, error) {
	session, err := r.AccountSessionRepository.FindSessionByRefreshHash(ctx, refreshTokenHash)
	if err != nil {
		return nil, err
	}
	r.readsReady <- struct{}{}
	<-r.releaseReads
	return session, nil
}
