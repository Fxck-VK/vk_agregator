// Package accountauth maps verified login assertions to canonical accounts.
// It does not verify passwords, OTPs or OAuth tokens itself; those checks live
// in provider-specific adapters before this service is called.
package accountauth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// Service is the single entry point for current and future login methods.
type Service struct {
	resolver    domain.IdentityResolver
	limiter     RateLimiter
	sessions    domain.AccountSessionRepository
	credentials domain.AccountCredentialRepository
	audit       domain.AccountLinkAuditRepository
	sessionTTL  time.Duration
	now         func() time.Time
}

// RateLimiter is the optional shared limiter for login/link flows.
type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

// ErrRateLimited is returned when login/link/unlink throttling blocks a flow.
var ErrRateLimited = errors.New("accountauth: rate limited")

// Option customizes the account auth service.
type Option func(*Service)

// New builds an account auth service over the canonical identity resolver.
func New(resolver domain.IdentityResolver, opts ...Option) *Service {
	s := &Service{resolver: resolver}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WithLimiter enables shared throttling for login/link/unlink flows.
func WithLimiter(limiter RateLimiter) Option {
	return func(s *Service) {
		s.limiter = limiter
	}
}

// ResolveOrCreate returns the account for a verified login assertion.
func (s *Service) ResolveOrCreate(ctx context.Context, login domain.VerifiedAccountLogin) (domain.IdentityResolution, error) {
	if s == nil || s.resolver == nil {
		return domain.IdentityResolution{}, errors.New("accountauth: identity resolver is required")
	}
	if !login.Verified {
		return domain.IdentityResolution{}, domain.ErrUnverifiedLogin
	}
	provider, externalID, normalizedID, err := providerIdentity(login)
	if err != nil {
		return domain.IdentityResolution{}, err
	}
	if err := s.checkRateLimit(ctx, "login", provider, normalizedID); err != nil {
		return domain.IdentityResolution{}, err
	}
	resolution, err := s.resolver.ResolveOrCreate(ctx, provider, externalID)
	if err != nil {
		return domain.IdentityResolution{}, err
	}
	if resolution.AccountID == uuid.Nil {
		return domain.IdentityResolution{}, errors.New("accountauth: resolver returned empty account id")
	}
	return resolution, nil
}

// LinkVerifiedIdentity binds a verified identity to an existing account. The
// current actor must be the account owner until a separate audited operator
// flow exists.
func (s *Service) LinkVerifiedIdentity(ctx context.Context, actorAccountID, accountID uuid.UUID, login domain.VerifiedAccountLogin) (*domain.AccountIdentity, error) {
	if s == nil || s.resolver == nil {
		return nil, errors.New("accountauth: identity resolver is required")
	}
	if actorAccountID == uuid.Nil || accountID == uuid.Nil || actorAccountID != accountID {
		return nil, domain.ErrAccountIdentityOwnershipRequired
	}
	if !login.Verified {
		return nil, domain.ErrUnverifiedLogin
	}
	provider, externalID, normalizedID, err := providerIdentity(login)
	if err != nil {
		return nil, err
	}
	if err := s.checkRateLimit(ctx, "link", provider, normalizedID); err != nil {
		return nil, err
	}
	return s.resolver.LinkIdentity(ctx, accountID, provider, externalID)
}

// UnlinkIdentity removes an identity from the current account. The current
// actor must be the account owner until a separate audited operator flow exists.
func (s *Service) UnlinkIdentity(ctx context.Context, actorAccountID, accountID, identityID uuid.UUID) error {
	if s == nil || s.resolver == nil {
		return errors.New("accountauth: identity resolver is required")
	}
	if actorAccountID == uuid.Nil || accountID == uuid.Nil || identityID == uuid.Nil || actorAccountID != accountID {
		return domain.ErrAccountIdentityOwnershipRequired
	}
	if err := s.checkRateLimit(ctx, "unlink", "", accountID.String()+":"+identityID.String()); err != nil {
		return err
	}
	return s.resolver.UnlinkIdentity(ctx, accountID, identityID)
}

// MergeAccounts is intentionally blocked until a dedicated confirmed and
// audited merge flow is implemented.
func (s *Service) MergeAccounts(_ context.Context, confirmed bool, sourceAccountID, targetAccountID uuid.UUID) error {
	if !confirmed || sourceAccountID == uuid.Nil || targetAccountID == uuid.Nil || sourceAccountID == targetAccountID {
		return domain.ErrAccountMergeRequiresConfirmation
	}
	return errors.New("accountauth: account merge flow is not implemented")
}

// ResolveVerifiedEmailPassword resolves an already password-verified email.
func (s *Service) ResolveVerifiedEmailPassword(ctx context.Context, email string) (domain.IdentityResolution, error) {
	return s.ResolveOrCreate(ctx, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginEmailPassword,
		ExternalID: email,
		Verified:   true,
	})
}

// ResolveVerifiedPhoneOTP resolves an already OTP-verified phone number.
func (s *Service) ResolveVerifiedPhoneOTP(ctx context.Context, phone string) (domain.IdentityResolution, error) {
	return s.ResolveOrCreate(ctx, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginPhoneOTP,
		ExternalID: phone,
		Verified:   true,
	})
}

// ResolveVerifiedGoogleSubject resolves a verified Google OAuth subject.
func (s *Service) ResolveVerifiedGoogleSubject(ctx context.Context, subject string) (domain.IdentityResolution, error) {
	return s.ResolveOrCreate(ctx, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginGoogle,
		ExternalID: subject,
		Verified:   true,
	})
}

// ResolveVerifiedAppleSubject resolves a verified Apple OAuth subject.
func (s *Service) ResolveVerifiedAppleSubject(ctx context.Context, subject string) (domain.IdentityResolution, error) {
	return s.ResolveOrCreate(ctx, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginApple,
		ExternalID: subject,
		Verified:   true,
	})
}

// ResolveTelegramID resolves a Telegram user id after Telegram auth checks.
func (s *Service) ResolveTelegramID(ctx context.Context, telegramID int64) (domain.IdentityResolution, error) {
	return s.ResolveOrCreate(ctx, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginTelegram,
		ExternalID: strconv.FormatInt(telegramID, 10),
		Verified:   true,
	})
}

// ResolveVKID resolves a VK user id after VK callback or VK ID verification.
func (s *Service) ResolveVKID(ctx context.Context, vkUserID int64) (domain.IdentityResolution, error) {
	return s.ResolveOrCreate(ctx, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginVKID,
		ExternalID: strconv.FormatInt(vkUserID, 10),
		Verified:   true,
	})
}

func providerIdentity(login domain.VerifiedAccountLogin) (domain.IdentityProvider, string, string, error) {
	method := domain.AccountLoginMethod(strings.ToLower(strings.TrimSpace(string(login.Method))))
	externalID := strings.TrimSpace(login.ExternalID)
	if externalID == "" {
		return "", "", "", domain.ErrInvalidIdentity
	}

	var provider domain.IdentityProvider
	switch method {
	case domain.AccountLoginEmailPassword:
		if err := validateEmail(externalID); err != nil {
			return "", "", "", err
		}
		// Password proof is stored as credentials. The login identity is the
		// verified email so deposits/history can follow the same account.
		provider = domain.IdentityProviderEmail
	case domain.AccountLoginPhoneOTP:
		if err := validatePhone(externalID); err != nil {
			return "", "", "", err
		}
		provider = domain.IdentityProviderPhone
	case domain.AccountLoginGoogle:
		provider = domain.IdentityProviderGoogle
	case domain.AccountLoginApple:
		provider = domain.IdentityProviderApple
	case domain.AccountLoginTelegram:
		if err := validatePositiveInt(externalID); err != nil {
			return "", "", "", err
		}
		provider = domain.IdentityProviderTelegram
	case domain.AccountLoginVKID:
		if err := validatePositiveInt(externalID); err != nil {
			return "", "", "", err
		}
		provider = domain.IdentityProviderVK
	default:
		return "", "", "", domain.ErrInvalidIdentity
	}

	normalizedID, err := domain.NormalizeExternalIdentity(provider, externalID)
	if err != nil {
		return "", "", "", err
	}
	return provider, externalID, normalizedID, nil
}

func validateEmail(value string) error {
	addr, err := mail.ParseAddress(value)
	if err != nil || addr.Address != strings.TrimSpace(value) {
		return domain.ErrInvalidIdentity
	}
	return nil
}

func validatePhone(value string) error {
	normalized, err := domain.NormalizeExternalIdentity(domain.IdentityProviderPhone, value)
	if err != nil {
		return err
	}
	digits := 0
	for _, r := range normalized {
		if unicode.IsDigit(r) {
			digits++
			continue
		}
		if r == '+' && digits == 0 {
			continue
		}
		return domain.ErrInvalidIdentity
	}
	if digits < 10 || digits > 15 {
		return domain.ErrInvalidIdentity
	}
	return nil
}

func validatePositiveInt(value string) error {
	id, err := strconv.ParseInt(strings.TrimLeft(strings.TrimSpace(value), "+"), 10, 64)
	if err != nil || id <= 0 {
		return domain.ErrInvalidIdentity
	}
	return nil
}

func (s *Service) checkRateLimit(ctx context.Context, scope string, provider domain.IdentityProvider, normalizedID string) error {
	if s.limiter == nil {
		return nil
	}
	allowed, err := s.limiter.Allow(ctx, rateLimitKey(scope, provider, normalizedID))
	if err != nil {
		return err
	}
	if !allowed {
		return ErrRateLimited
	}
	return nil
}

func rateLimitKey(scope string, provider domain.IdentityProvider, normalizedID string) string {
	sum := sha256.Sum256([]byte(scope + ":" + string(provider) + ":" + normalizedID))
	return "accountauth:" + scope + ":" + hex.EncodeToString(sum[:])
}
