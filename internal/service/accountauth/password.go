package accountauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/pbkdf2"

	"vk-ai-aggregator/internal/domain"
)

const (
	passwordMinBytes                      = 8
	passwordMaxBytes                      = 256
	passwordSaltBytes                     = 16
	passwordHashBytes                     = 32
	passwordMaxEncodedHashBytes           = 256
	passwordHashAlgorithm                 = "pbkdf2_sha256" // #nosec G101 -- legacy password hash algorithm identifier, not a credential.
	passwordLegacyPBKDF2Iterations        = 120000
	passwordMaxPBKDF2Iterations           = 600000
	passwordCurrentHashAlgorithm          = "argon2id" // #nosec G101 -- password hash algorithm identifier, not a credential.
	passwordArgonVersion                  = argon2.Version
	passwordArgonMemoryKiB         uint32 = 19 * 1024
	passwordArgonTime              uint32 = 2
	passwordArgonThreads           uint8  = 1
	passwordMaxArgonMemoryKiB      uint64 = 64 * 1024
	passwordMaxArgonTime           uint64 = 10
	passwordMaxArgonThreads        uint64 = 4
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
	ok, needsRehash, err := verifyPasswordWithUpgrade(password, credential.SecretHash)
	if err != nil || !ok {
		return domain.IdentityResolution{}, ErrInvalidPasswordLogin
	}
	if needsRehash {
		if err := s.upsertPassword(ctx, accountID, password); err != nil {
			return domain.IdentityResolution{}, err
		}
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
	sum := argon2.IDKey(
		[]byte(password),
		salt,
		passwordArgonTime,
		passwordArgonMemoryKiB,
		passwordArgonThreads,
		passwordHashBytes,
	)
	return fmt.Sprintf(
		"$%s$v=%d$m=%d,t=%d,p=%d$%s$%s",
		passwordCurrentHashAlgorithm,
		passwordArgonVersion,
		passwordArgonMemoryKiB,
		passwordArgonTime,
		passwordArgonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(sum),
	), nil
}

func verifyPassword(password, encoded string) (bool, error) {
	ok, _, err := verifyPasswordWithUpgrade(password, encoded)
	return ok, err
}

func verifyPasswordWithUpgrade(password, encoded string) (bool, bool, error) {
	if validatePassword(password) != nil {
		return false, false, nil
	}
	if len(encoded) == 0 || len(encoded) > passwordMaxEncodedHashBytes {
		return false, false, errors.New("accountauth: invalid password hash size")
	}
	if strings.HasPrefix(encoded, "$"+passwordCurrentHashAlgorithm+"$") {
		return verifyArgon2idPassword(password, encoded)
	}
	if strings.HasPrefix(encoded, passwordHashAlgorithm+"$") {
		ok, err := verifyLegacyPBKDF2Password(password, encoded)
		return ok, ok && err == nil, err
	}
	return false, false, errors.New("accountauth: unsupported password hash")
}

func verifyLegacyPBKDF2Password(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != passwordHashAlgorithm {
		return false, errors.New("accountauth: unsupported password hash")
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations < passwordLegacyPBKDF2Iterations || iterations > passwordMaxPBKDF2Iterations {
		return false, errors.New("accountauth: invalid password hash iterations")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) != passwordSaltBytes {
		return false, errors.New("accountauth: invalid password hash salt")
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(expected) != passwordHashBytes {
		return false, errors.New("accountauth: invalid password hash digest")
	}
	actual := pbkdf2.Key([]byte(password), salt, iterations, len(expected), sha256.New)
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func verifyArgon2idPassword(password, encoded string) (bool, bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != passwordCurrentHashAlgorithm {
		return false, false, errors.New("accountauth: invalid argon2id hash format")
	}
	version, err := parsePasswordHashUint(parts[2], "v=")
	if err != nil || version != uint64(passwordArgonVersion) {
		return false, false, errors.New("accountauth: invalid argon2id version")
	}
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return false, false, errors.New("accountauth: invalid argon2id parameters")
	}
	memory, memoryErr := parsePasswordHashUint32(params[0], "m=")
	timeCost, timeErr := parsePasswordHashUint32(params[1], "t=")
	threads, threadsErr := parsePasswordHashUint8(params[2], "p=")
	if memoryErr != nil || timeErr != nil || threadsErr != nil ||
		memory == 0 || memory > uint32(passwordMaxArgonMemoryKiB) ||
		timeCost == 0 || timeCost > uint32(passwordMaxArgonTime) ||
		threads == 0 || threads > uint8(passwordMaxArgonThreads) {
		return false, false, errors.New("accountauth: unsafe argon2id parameters")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != passwordSaltBytes {
		return false, false, errors.New("accountauth: invalid argon2id salt")
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) != passwordHashBytes {
		return false, false, errors.New("accountauth: invalid argon2id digest")
	}
	actual := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, passwordHashBytes)
	ok := subtle.ConstantTimeCompare(actual, expected) == 1
	needsRehash := ok && (memory != passwordArgonMemoryKiB ||
		timeCost != passwordArgonTime || threads != passwordArgonThreads)
	return ok, needsRehash, nil
}

func parsePasswordHashUint(raw, prefix string) (uint64, error) {
	value, err := passwordHashParameterValue(raw, prefix)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(value, 10, 64)
}

func parsePasswordHashUint32(raw, prefix string) (uint32, error) {
	value, err := passwordHashParameterValue(raw, prefix)
	if err != nil {
		return 0, err
	}
	var parsed uint32
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return 0, errors.New("accountauth: invalid password hash parameter")
	}
	return parsed, nil
}

func parsePasswordHashUint8(raw, prefix string) (uint8, error) {
	value, err := passwordHashParameterValue(raw, prefix)
	if err != nil {
		return 0, err
	}
	var parsed uint8
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return 0, errors.New("accountauth: invalid password hash parameter")
	}
	return parsed, nil
}

func passwordHashParameterValue(raw, prefix string) (string, error) {
	if !strings.HasPrefix(raw, prefix) {
		return "", errors.New("accountauth: invalid password hash parameter")
	}
	value := strings.TrimPrefix(raw, prefix)
	if value == "" || len(value) > 10 {
		return "", errors.New("accountauth: invalid password hash parameter")
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return "", errors.New("accountauth: invalid password hash parameter")
		}
	}
	return value, nil
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
