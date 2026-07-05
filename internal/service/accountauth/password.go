package accountauth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

const (
	passwordMinBytes      = 8
	passwordMaxBytes      = 256
	passwordSaltBytes     = 16
	passwordHashBytes     = 32
	passwordIterations    = 120000
	passwordHashAlgorithm = "pbkdf2_sha256"
)

var (
	// ErrPasswordStoreUnavailable is returned when password login is not wired.
	ErrPasswordStoreUnavailable = errors.New("accountauth: password store unavailable")
	// ErrInvalidPasswordLogin intentionally covers unknown email, missing
	// credential and wrong password so callers cannot enumerate accounts.
	ErrInvalidPasswordLogin = errors.New("accountauth: invalid email or password")
	// ErrWeakPassword is returned for passwords outside accepted bounds.
	ErrWeakPassword = errors.New("accountauth: weak password")
)

// WithCredentialRepository enables email/password credential persistence.
func WithCredentialRepository(repo domain.AccountCredentialRepository) Option {
	return func(s *Service) {
		s.credentials = repo
	}
}

// WithAccountAuditRepository enables explicit account security audit writes.
func WithAccountAuditRepository(repo domain.AccountLinkAuditRepository) Option {
	return func(s *Service) {
		s.audit = repo
	}
}

// SetPasswordForVerifiedEmail enables password login for an email identity
// already linked to the authenticated account.
func (s *Service) SetPasswordForVerifiedEmail(ctx context.Context, actorAccountID, accountID uuid.UUID, email, password string) error {
	if actorAccountID == uuid.Nil || accountID == uuid.Nil || actorAccountID != accountID {
		return domain.ErrAccountIdentityOwnershipRequired
	}
	normalizedEmail, err := s.verifyLinkedEmail(ctx, accountID, email)
	if err != nil {
		return err
	}
	if err := s.checkRateLimit(ctx, "password_set", domain.IdentityProviderEmail, normalizedEmail); err != nil {
		return err
	}
	if err := s.upsertPassword(ctx, accountID, password); err != nil {
		return err
	}
	return s.recordAudit(ctx, accountID, &actorAccountID, domain.AccountLinkActionPasswordSet, domain.IdentityProviderEmail)
}

// ResetPasswordForVerifiedEmail updates a password after the caller has
// verified email ownership through an out-of-band code.
func (s *Service) ResetPasswordForVerifiedEmail(ctx context.Context, accountID uuid.UUID, email, password string) error {
	if accountID == uuid.Nil {
		return domain.ErrInvalidIdentity
	}
	normalizedEmail, err := s.verifyLinkedEmail(ctx, accountID, email)
	if err != nil {
		return err
	}
	if err := s.checkRateLimit(ctx, "password_reset", domain.IdentityProviderEmail, normalizedEmail); err != nil {
		return err
	}
	if err := s.upsertPassword(ctx, accountID, password); err != nil {
		return err
	}
	if s.sessions != nil {
		if _, err := s.sessions.RevokeAllSessions(ctx, accountID, s.currentTime()); err != nil {
			return err
		}
	}
	return s.recordAudit(ctx, accountID, nil, domain.AccountLinkActionPasswordReset, domain.IdentityProviderEmail)
}

// AuthenticateEmailPassword verifies an email/password pair and returns the
// canonical account id. It never creates accounts; password login must sit on
// top of an existing verified email identity.
func (s *Service) AuthenticateEmailPassword(ctx context.Context, email, password string) (domain.IdentityResolution, error) {
	if s == nil || s.resolver == nil {
		return domain.IdentityResolution{}, errors.New("accountauth: identity resolver is required")
	}
	if s.credentials == nil {
		return domain.IdentityResolution{}, ErrPasswordStoreUnavailable
	}
	_, externalID, normalizedEmail, err := providerIdentity(domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginEmailPassword,
		ExternalID: email,
		Verified:   true,
	})
	if err != nil {
		return domain.IdentityResolution{}, ErrInvalidPasswordLogin
	}
	if err := s.checkRateLimit(ctx, "password_login", domain.IdentityProviderEmail, normalizedEmail); err != nil {
		return domain.IdentityResolution{}, err
	}
	accountID, err := s.resolver.Resolve(ctx, domain.IdentityProviderEmail, externalID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.IdentityResolution{}, ErrInvalidPasswordLogin
		}
		return domain.IdentityResolution{}, err
	}
	credential, err := s.credentials.FindCredential(ctx, accountID, domain.AccountCredentialPassword)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.IdentityResolution{}, ErrInvalidPasswordLogin
		}
		return domain.IdentityResolution{}, err
	}
	ok, err := verifyPassword(password, credential.SecretHash)
	if err != nil || !ok {
		return domain.IdentityResolution{}, ErrInvalidPasswordLogin
	}
	if err := s.recordAudit(ctx, accountID, nil, domain.AccountLinkActionLogin, domain.IdentityProviderEmail); err != nil {
		return domain.IdentityResolution{}, err
	}
	return domain.IdentityResolution{AccountID: accountID}, nil
}

func (s *Service) verifyLinkedEmail(ctx context.Context, accountID uuid.UUID, email string) (string, error) {
	if s == nil || s.resolver == nil {
		return "", errors.New("accountauth: identity resolver is required")
	}
	if s.credentials == nil {
		return "", ErrPasswordStoreUnavailable
	}
	_, externalID, normalizedEmail, err := providerIdentity(domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginEmailPassword,
		ExternalID: email,
		Verified:   true,
	})
	if err != nil {
		return "", err
	}
	resolvedAccountID, err := s.resolver.Resolve(ctx, domain.IdentityProviderEmail, externalID)
	if err != nil {
		return "", err
	}
	if resolvedAccountID != accountID {
		return "", domain.ErrAccountIdentityOwnershipRequired
	}
	return normalizedEmail, nil
}

func (s *Service) upsertPassword(ctx context.Context, accountID uuid.UUID, password string) error {
	if s == nil || s.credentials == nil {
		return ErrPasswordStoreUnavailable
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	now := s.currentTime()
	_, err = s.credentials.UpsertCredential(ctx, domain.AccountCredential{
		ID:             uuid.New(),
		AccountID:      accountID,
		CredentialType: domain.AccountCredentialPassword,
		SecretHash:     hash,
		ChangedAt:      &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	return err
}

func (s *Service) recordAudit(ctx context.Context, accountID uuid.UUID, actorAccountID *uuid.UUID, action domain.AccountLinkAction, provider domain.IdentityProvider) error {
	if s == nil || s.audit == nil {
		return nil
	}
	var actor *uuid.UUID
	if actorAccountID != nil && *actorAccountID != uuid.Nil {
		id := *actorAccountID
		actor = &id
	}
	return s.audit.RecordAccountAudit(ctx, domain.AccountLinkAuditEntry{
		ID:             uuid.New(),
		AccountID:      accountID,
		ActorAccountID: actor,
		Action:         action,
		Provider:       provider,
		CreatedAt:      s.currentTime(),
	})
}

func hashPassword(password string) (string, error) {
	if err := validatePassword(password); err != nil {
		return "", err
	}
	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := pbkdf2SHA256([]byte(password), salt, passwordIterations, passwordHashBytes)
	return strings.Join([]string{
		passwordHashAlgorithm,
		strconv.Itoa(passwordIterations),
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(sum),
	}, "$"), nil
}

func verifyPassword(password, encoded string) (bool, error) {
	if validatePassword(password) != nil {
		return false, nil
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != passwordHashAlgorithm {
		return false, errors.New("accountauth: unsupported password hash")
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false, errors.New("accountauth: invalid password hash iterations")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) == 0 {
		return false, errors.New("accountauth: invalid password hash salt")
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(expected) == 0 {
		return false, errors.New("accountauth: invalid password hash digest")
	}
	actual := pbkdf2SHA256([]byte(password), salt, iterations, len(expected))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func validatePassword(password string) error {
	if len(password) < passwordMinBytes || len(password) > passwordMaxBytes {
		return ErrWeakPassword
	}
	if strings.TrimSpace(password) == "" {
		return ErrWeakPassword
	}
	return nil
}

func pbkdf2SHA256(password, salt []byte, iterations, keyLen int) []byte {
	hashLen := sha256.Size
	numBlocks := (keyLen + hashLen - 1) / hashLen
	out := make([]byte, 0, numBlocks*hashLen)
	for block := 1; block <= numBlocks; block++ {
		u := pbkdf2Block(password, salt, iterations, block)
		out = append(out, u...)
	}
	return out[:keyLen]
}

func pbkdf2Block(password, salt []byte, iterations, block int) []byte {
	mac := hmac.New(sha256.New, password)
	_, _ = mac.Write(salt)
	_, _ = mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
	u := mac.Sum(nil)
	out := make([]byte, len(u))
	copy(out, u)
	for i := 1; i < iterations; i++ {
		mac = hmac.New(sha256.New, password)
		_, _ = mac.Write(u)
		u = mac.Sum(nil)
		for j := range out {
			out[j] ^= u[j]
		}
	}
	return out
}
