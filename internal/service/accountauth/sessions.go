package accountauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

const (
	defaultSessionTTL        = 30 * 24 * time.Hour
	defaultAccessTokenTTL    = 15 * time.Minute
	defaultSessionTokenBytes = 32
)

// ErrSessionStoreUnavailable is returned when session support is not wired.
var ErrSessionStoreUnavailable = errors.New("accountauth: session store unavailable")

// ErrInvalidSession is returned for missing, revoked or unknown session tokens.
var ErrInvalidSession = errors.New("accountauth: invalid session")

// ErrSessionExpired is returned when a refresh token belongs to an expired session.
var ErrSessionExpired = errors.New("accountauth: session expired")

// SessionMetadata contains request metadata that must be persisted only as
// hashes. DeviceInfo may be a browser/mobile device fingerprint supplied by the
// client; IP and UserAgent come from the edge/backend request.
type SessionMetadata struct {
	IdentityID *uuid.UUID
	DeviceInfo string
	IP         string
	UserAgent  string
}

// AccountSessionSafe is safe to return to web/mobile clients. It intentionally
// omits refresh token hash, device hash, IP hash and user-agent hash.
type AccountSessionSafe struct {
	ID         uuid.UUID  `json:"id"`
	AccountID  uuid.UUID  `json:"account_id"`
	IdentityID *uuid.UUID `json:"identity_id,omitempty"`
	CreatedAt  string     `json:"created_at"`
	UpdatedAt  string     `json:"updated_at"`
	ExpiresAt  string     `json:"expires_at"`
	Revoked    bool       `json:"revoked"`
}

// SessionTokens contains a newly issued access/refresh token pair. Raw tokens
// are returned only to the caller and are never stored by AccountAuth.
type SessionTokens struct {
	AccessToken     string             `json:"access_token"`
	RefreshToken    string             `json:"refresh_token"`
	AccessExpiresAt string             `json:"access_expires_at"`
	ExpiresAt       string             `json:"expires_at"`
	Session         AccountSessionSafe `json:"session"`
}

// WithSessionRepository enables Web/Mobile session persistence.
func WithSessionRepository(repo domain.AccountSessionRepository) Option {
	return func(s *Service) {
		s.sessions = repo
	}
}

// WithSessionTTL overrides refresh-token lifetime. Non-positive values keep
// the production-safe default.
func WithSessionTTL(ttl time.Duration) Option {
	return func(s *Service) {
		if ttl > 0 {
			s.sessionTTL = ttl
		}
	}
}

// WithAccessTokenTTL overrides access-token lifetime. Non-positive values keep
// the production-safe default.
func WithAccessTokenTTL(ttl time.Duration) Option {
	return func(s *Service) {
		if ttl > 0 {
			s.accessTokenTTL = ttl
		}
	}
}

// WithClock is for deterministic tests.
func WithClock(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

// IssueSession creates an account session and returns raw tokens once.
func (s *Service) IssueSession(ctx context.Context, accountID uuid.UUID, meta SessionMetadata) (SessionTokens, error) {
	if s == nil || s.sessions == nil {
		return SessionTokens{}, ErrSessionStoreUnavailable
	}
	if accountID == uuid.Nil {
		return SessionTokens{}, domain.ErrInvalidIdentity
	}
	accessToken, err := randomToken(defaultSessionTokenBytes)
	if err != nil {
		return SessionTokens{}, err
	}
	refreshToken, err := randomToken(defaultSessionTokenBytes)
	if err != nil {
		return SessionTokens{}, err
	}
	now := s.currentTime()
	expiresAt := now.Add(s.effectiveSessionTTL())
	accessExpiresAt := now.Add(s.effectiveAccessTokenTTL())
	if accessExpiresAt.After(expiresAt) {
		accessExpiresAt = expiresAt
	}
	session := domain.AccountSession{
		ID:               uuid.New(),
		AccountID:        accountID,
		IdentityID:       normalizedIdentityID(meta.IdentityID),
		AccessTokenHash:  hashSecret("access", accessToken),
		AccessExpiresAt:  &accessExpiresAt,
		RefreshTokenHash: hashSecret("refresh", refreshToken),
		DeviceID:         hashSecret("device", nonEmpty(meta.DeviceInfo)),
		IPHash:           hashSecret("ip", nonEmpty(meta.IP)),
		UserAgentHash:    hashSecret("ua", nonEmpty(meta.UserAgent)),
		CreatedAt:        now,
		UpdatedAt:        now,
		ExpiresAt:        expiresAt,
	}
	created, err := s.sessions.CreateSession(ctx, session)
	if err != nil {
		return SessionTokens{}, err
	}
	return SessionTokens{
		AccessToken:     accessToken,
		RefreshToken:    refreshToken,
		AccessExpiresAt: formatSessionTime(*created.AccessExpiresAt),
		ExpiresAt:       formatSessionTime(created.ExpiresAt),
		Session:         SafeSessionDTO(created),
	}, nil
}

// AuthenticateAccessToken resolves a valid short-lived access secret into the
// canonical account principal. Legacy refresh-only rows have no access hash and
// therefore cannot authorize generic requests.
func (s *Service) AuthenticateAccessToken(ctx context.Context, accessToken string) (domain.RequestPrincipal, error) {
	if s == nil || s.sessions == nil {
		return domain.RequestPrincipal{}, ErrSessionStoreUnavailable
	}
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return domain.RequestPrincipal{}, ErrInvalidSession
	}
	session, err := s.sessions.FindSessionByAccessHash(ctx, hashSecret("access", accessToken))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.RequestPrincipal{}, ErrInvalidSession
	}
	if err != nil {
		return domain.RequestPrincipal{}, err
	}
	now := s.currentTime()
	if session.RevokedAt != nil {
		return domain.RequestPrincipal{}, ErrInvalidSession
	}
	if session.AccessExpiresAt == nil || !session.AccessExpiresAt.After(now) {
		return domain.RequestPrincipal{}, ErrSessionExpired
	}
	principal := domain.RequestPrincipal{
		AccountID: session.AccountID,
		SessionID: session.ID,
		Method:    domain.AuthenticationMethodAccountSession,
	}
	if err := principal.Validate(); err != nil {
		return domain.RequestPrincipal{}, err
	}
	return principal, nil
}

// RefreshSession revokes the supplied refresh token and issues a new session.
func (s *Service) RefreshSession(ctx context.Context, refreshToken string, meta SessionMetadata) (SessionTokens, error) {
	if s == nil || s.sessions == nil {
		return SessionTokens{}, ErrSessionStoreUnavailable
	}
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return SessionTokens{}, ErrInvalidSession
	}
	now := s.currentTime()
	old, err := s.sessions.FindSessionByRefreshHash(ctx, hashSecret("refresh", refreshToken))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return SessionTokens{}, ErrInvalidSession
		}
		return SessionTokens{}, err
	}
	if old.RevokedAt != nil {
		return SessionTokens{}, ErrInvalidSession
	}
	if !old.ExpiresAt.After(now) {
		return SessionTokens{}, ErrSessionExpired
	}
	if _, err := s.sessions.RevokeSessionByRefreshHash(ctx, old.RefreshTokenHash, now); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return SessionTokens{}, ErrInvalidSession
		}
		return SessionTokens{}, err
	}
	if meta.IdentityID == nil {
		meta.IdentityID = old.IdentityID
	}
	return s.IssueSession(ctx, old.AccountID, meta)
}

// RevokeSession revokes one active session owned by accountID.
func (s *Service) RevokeSession(ctx context.Context, accountID, sessionID uuid.UUID) (AccountSessionSafe, error) {
	if s == nil || s.sessions == nil {
		return AccountSessionSafe{}, ErrSessionStoreUnavailable
	}
	if accountID == uuid.Nil || sessionID == uuid.Nil {
		return AccountSessionSafe{}, domain.ErrInvalidIdentity
	}
	revoked, err := s.sessions.RevokeSession(ctx, accountID, sessionID, s.currentTime())
	if err != nil {
		return AccountSessionSafe{}, err
	}
	return SafeSessionDTO(revoked), nil
}

// Logout revokes a session by refresh token. Unknown tokens are treated as a
// no-op so logout remains idempotent and does not reveal token existence.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if s == nil || s.sessions == nil {
		return ErrSessionStoreUnavailable
	}
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil
	}
	_, err := s.sessions.RevokeSessionByRefreshHash(ctx, hashSecret("refresh", refreshToken), s.currentTime())
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	return err
}

// ListActiveSessions returns safe active sessions for one account.
func (s *Service) ListActiveSessions(ctx context.Context, accountID uuid.UUID, limit int) ([]AccountSessionSafe, error) {
	if s == nil || s.sessions == nil {
		return nil, ErrSessionStoreUnavailable
	}
	if accountID == uuid.Nil {
		return nil, domain.ErrInvalidIdentity
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.sessions.ListActiveSessionsByAccount(ctx, accountID, s.currentTime(), limit)
	if err != nil {
		return nil, err
	}
	out := make([]AccountSessionSafe, 0, len(rows))
	for _, row := range rows {
		out = append(out, SafeSessionDTO(row))
	}
	return out, nil
}

// SafeSessionDTO strips all hashes and formats timestamps consistently.
func SafeSessionDTO(session *domain.AccountSession) AccountSessionSafe {
	if session == nil {
		return AccountSessionSafe{}
	}
	return AccountSessionSafe{
		ID:         session.ID,
		AccountID:  session.AccountID,
		IdentityID: session.IdentityID,
		CreatedAt:  formatSessionTime(session.CreatedAt),
		UpdatedAt:  formatSessionTime(session.UpdatedAt),
		ExpiresAt:  formatSessionTime(session.ExpiresAt),
		Revoked:    session.RevokedAt != nil,
	}
}

func (s *Service) effectiveSessionTTL() time.Duration {
	if s == nil || s.sessionTTL <= 0 {
		return defaultSessionTTL
	}
	return s.sessionTTL
}

func (s *Service) effectiveAccessTokenTTL() time.Duration {
	if s == nil || s.accessTokenTTL <= 0 {
		return defaultAccessTokenTTL
	}
	return s.accessTokenTTL
}

func (s *Service) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func randomToken(size int) (string, error) {
	if size <= 0 {
		size = defaultSessionTokenBytes
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashSecret(scope, value string) string {
	sum := sha256.Sum256([]byte(scope + ":" + value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func nonEmpty(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}

func normalizedIdentityID(identityID *uuid.UUID) *uuid.UUID {
	if identityID == nil || *identityID == uuid.Nil {
		return nil
	}
	id := *identityID
	return &id
}

func formatSessionTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
