// Package accountlink verifies method-specific account identity ownership
// before calling the shared account boundary.
package accountlink

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountservice"
)

const (
	defaultEmailCodeTTL       = 10 * time.Minute
	defaultEmailCodeDigits    = 6
	defaultPhoneOTPTTL        = 10 * time.Minute
	defaultPhoneOTPDigits     = 6
	defaultRequestLimit       = 3
	defaultRequestWindow      = 15 * time.Minute
	defaultVerifyLimit        = 5
	defaultVerifyWindow       = 15 * time.Minute
	defaultHashSecret         = "accountlink-development-secret"
	maxCodeDigits             = 10
	minCodeDigits             = 4
	emailChallengeKeyPrefix   = "accountlink:email:challenge:"
	emailRequestRateKeyPrefix = "accountlink:email:request:"
	emailVerifyRateKeyPrefix  = "accountlink:email:verify:"
	phoneChallengeKeyPrefix   = "accountlink:phone:challenge:"
	phoneRequestRateKeyPrefix = "accountlink:phone:request:"
	phoneVerifyRateKeyPrefix  = "accountlink:phone:verify:"
)

var (
	ErrMissingDependency      = errors.New("accountlink: missing dependency")
	ErrRateLimited            = errors.New("accountlink: rate limited")
	ErrInvalidEmail           = domain.ErrInvalidIdentity
	ErrInvalidPhone           = domain.ErrInvalidIdentity
	ErrInvalidCode            = errors.New("accountlink: invalid code")
	ErrExpiredCode            = errors.New("accountlink: expired code")
	ErrDeliveryUnavailable    = errors.New("accountlink: email delivery unavailable")
	ErrSMSDeliveryUnavailable = errors.New("accountlink: phone delivery unavailable")
	errInvalidConfiguration   = errors.New("accountlink: invalid configuration")
)

// Store persists short-lived method-specific verification challenges.
type Store interface {
	SaveChallenge(ctx context.Context, key string, challenge Challenge, ttl time.Duration) error
	LoadChallenge(ctx context.Context, key string) (Challenge, error)
	DeleteChallenge(ctx context.Context, key string) error
	Increment(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

// Sender sends the verification code out of band. Implementations must not log
// or expose raw codes outside their delivery channel.
type Sender interface {
	SendEmailLinkCode(ctx context.Context, email, code string, expiresAt time.Time) error
	SendPhoneLinkOTP(ctx context.Context, phone, code string, expiresAt time.Time) error
}

// AccountService is the safe account boundary used after identity ownership is
// verified.
type AccountService interface {
	LinkVerifiedIdentity(ctx context.Context, actorAccountID, accountID uuid.UUID, login domain.VerifiedAccountLogin) (accountservice.AccountIdentitySafe, error)
}

// Config controls email and phone verification.
type Config struct {
	CodeTTL       time.Duration
	CodeDigits    int
	RequestLimit  int
	RequestWindow time.Duration
	VerifyLimit   int
	VerifyWindow  time.Duration

	PhoneCodeTTL       time.Duration
	PhoneCodeDigits    int
	PhoneRequestLimit  int
	PhoneRequestWindow time.Duration
	PhoneVerifyLimit   int
	PhoneVerifyWindow  time.Duration

	HashSecret string
	Now        func() time.Time
}

// Challenge is the stored method-specific verification state. It contains only
// hashed values and the owning account id.
type Challenge struct {
	AccountID    uuid.UUID `json:"account_id"`
	IdentityHash string    `json:"identity_hash"`
	CodeHash     string    `json:"code_hash"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// RequestResult is safe to return from request-code endpoints.
type RequestResult struct {
	Status           string `json:"status"`
	ExpiresInSeconds int64  `json:"expires_in_seconds"`
}

// Service owns request-code and verify-code flows.
type Service struct {
	store   Store
	sender  Sender
	account AccountService
	cfg     Config
}

// New builds a method-specific identity linker.
func New(store Store, sender Sender, account AccountService, cfg Config) (*Service, error) {
	cfg = normalizeConfig(cfg)
	if cfg.CodeTTL <= 0 || cfg.RequestWindow <= 0 || cfg.VerifyWindow <= 0 ||
		cfg.CodeDigits < minCodeDigits || cfg.CodeDigits > maxCodeDigits ||
		cfg.PhoneCodeTTL <= 0 || cfg.PhoneRequestWindow <= 0 || cfg.PhoneVerifyWindow <= 0 ||
		cfg.PhoneCodeDigits < minCodeDigits || cfg.PhoneCodeDigits > maxCodeDigits {
		return nil, errInvalidConfiguration
	}
	return &Service{
		store:   store,
		sender:  sender,
		account: account,
		cfg:     cfg,
	}, nil
}

// RequestEmailCode creates a short-lived code and sends it to the supplied
// email after rate limiting the current account/email pair.
func (s *Service) RequestEmailCode(ctx context.Context, accountID uuid.UUID, email string) (RequestResult, error) {
	if s == nil || s.store == nil || s.sender == nil {
		return RequestResult{}, ErrMissingDependency
	}
	if accountID == uuid.Nil {
		return RequestResult{}, domain.ErrInvalidIdentity
	}
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return RequestResult{}, err
	}
	emailHash := hashIdentity(normalizedEmail)
	if err := s.checkLimit(ctx, requestRateKey(accountID, emailHash), s.cfg.RequestLimit, s.cfg.RequestWindow); err != nil {
		return RequestResult{}, err
	}
	code, err := generateNumericCode(s.cfg.CodeDigits)
	if err != nil {
		return RequestResult{}, err
	}
	now := s.cfg.Now()
	expiresAt := now.Add(s.cfg.CodeTTL)
	key := challengeKey(accountID, emailHash)
	challenge := Challenge{
		AccountID:    accountID,
		IdentityHash: emailHash,
		CodeHash:     s.codeHash(accountID, normalizedEmail, code),
		ExpiresAt:    expiresAt,
	}
	if err := s.store.SaveChallenge(ctx, key, challenge, s.cfg.CodeTTL); err != nil {
		return RequestResult{}, err
	}
	if err := s.sender.SendEmailLinkCode(ctx, normalizedEmail, code, expiresAt); err != nil {
		_ = s.store.DeleteChallenge(ctx, key)
		return RequestResult{}, err
	}
	return RequestResult{
		Status:           "verification_sent",
		ExpiresInSeconds: int64(s.cfg.CodeTTL.Seconds()),
	}, nil
}

// RequestPhoneOTP creates a short-lived OTP and sends it to the supplied phone
// after rate limiting the current account/phone pair.
func (s *Service) RequestPhoneOTP(ctx context.Context, accountID uuid.UUID, phone string) (RequestResult, error) {
	if s == nil || s.store == nil || s.sender == nil {
		return RequestResult{}, ErrMissingDependency
	}
	if accountID == uuid.Nil {
		return RequestResult{}, domain.ErrInvalidIdentity
	}
	normalizedPhone, err := normalizePhone(phone)
	if err != nil {
		return RequestResult{}, err
	}
	phoneHash := hashIdentity(normalizedPhone)
	if err := s.checkLimit(ctx, phoneRequestRateKey(accountID, phoneHash), s.cfg.PhoneRequestLimit, s.cfg.PhoneRequestWindow); err != nil {
		return RequestResult{}, err
	}
	code, err := generateNumericCode(s.cfg.PhoneCodeDigits)
	if err != nil {
		return RequestResult{}, err
	}
	now := s.cfg.Now()
	expiresAt := now.Add(s.cfg.PhoneCodeTTL)
	key := phoneChallengeKey(accountID, phoneHash)
	challenge := Challenge{
		AccountID:    accountID,
		IdentityHash: phoneHash,
		CodeHash:     s.codeHash(accountID, normalizedPhone, code),
		ExpiresAt:    expiresAt,
	}
	if err := s.store.SaveChallenge(ctx, key, challenge, s.cfg.PhoneCodeTTL); err != nil {
		return RequestResult{}, err
	}
	if err := s.sender.SendPhoneLinkOTP(ctx, normalizedPhone, code, expiresAt); err != nil {
		_ = s.store.DeleteChallenge(ctx, key)
		return RequestResult{}, err
	}
	return RequestResult{
		Status:           "verification_sent",
		ExpiresInSeconds: int64(s.cfg.PhoneCodeTTL.Seconds()),
	}, nil
}

// VerifyEmailCode validates a code and links the verified email identity to
// the current account.
func (s *Service) VerifyEmailCode(ctx context.Context, accountID uuid.UUID, email, code string) (accountservice.AccountIdentitySafe, error) {
	if s == nil || s.store == nil || s.account == nil {
		return accountservice.AccountIdentitySafe{}, ErrMissingDependency
	}
	if accountID == uuid.Nil {
		return accountservice.AccountIdentitySafe{}, domain.ErrInvalidIdentity
	}
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return accountservice.AccountIdentitySafe{}, err
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return accountservice.AccountIdentitySafe{}, ErrInvalidCode
	}
	emailHash := hashIdentity(normalizedEmail)
	if err := s.checkLimit(ctx, verifyRateKey(accountID, emailHash), s.cfg.VerifyLimit, s.cfg.VerifyWindow); err != nil {
		return accountservice.AccountIdentitySafe{}, err
	}
	key := challengeKey(accountID, emailHash)
	challenge, err := s.store.LoadChallenge(ctx, key)
	if err != nil {
		return accountservice.AccountIdentitySafe{}, err
	}
	if challenge.AccountID != accountID || challenge.IdentityHash != emailHash {
		return accountservice.AccountIdentitySafe{}, ErrInvalidCode
	}
	if !challenge.ExpiresAt.IsZero() && s.cfg.Now().After(challenge.ExpiresAt) {
		_ = s.store.DeleteChallenge(ctx, key)
		return accountservice.AccountIdentitySafe{}, ErrExpiredCode
	}
	if !hmac.Equal([]byte(challenge.CodeHash), []byte(s.codeHash(accountID, normalizedEmail, code))) {
		return accountservice.AccountIdentitySafe{}, ErrInvalidCode
	}
	_ = s.store.DeleteChallenge(ctx, key)
	return s.account.LinkVerifiedIdentity(ctx, accountID, accountID, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginEmailPassword,
		ExternalID: normalizedEmail,
		Verified:   true,
	})
}

// VerifyPhoneOTP validates an OTP and links the verified phone identity to the
// current account.
func (s *Service) VerifyPhoneOTP(ctx context.Context, accountID uuid.UUID, phone, code string) (accountservice.AccountIdentitySafe, error) {
	if s == nil || s.store == nil || s.account == nil {
		return accountservice.AccountIdentitySafe{}, ErrMissingDependency
	}
	if accountID == uuid.Nil {
		return accountservice.AccountIdentitySafe{}, domain.ErrInvalidIdentity
	}
	normalizedPhone, err := normalizePhone(phone)
	if err != nil {
		return accountservice.AccountIdentitySafe{}, err
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return accountservice.AccountIdentitySafe{}, ErrInvalidCode
	}
	phoneHash := hashIdentity(normalizedPhone)
	if err := s.checkLimit(ctx, phoneVerifyRateKey(accountID, phoneHash), s.cfg.PhoneVerifyLimit, s.cfg.PhoneVerifyWindow); err != nil {
		return accountservice.AccountIdentitySafe{}, err
	}
	key := phoneChallengeKey(accountID, phoneHash)
	challenge, err := s.store.LoadChallenge(ctx, key)
	if err != nil {
		return accountservice.AccountIdentitySafe{}, err
	}
	if challenge.AccountID != accountID || challenge.IdentityHash != phoneHash {
		return accountservice.AccountIdentitySafe{}, ErrInvalidCode
	}
	if !challenge.ExpiresAt.IsZero() && s.cfg.Now().After(challenge.ExpiresAt) {
		_ = s.store.DeleteChallenge(ctx, key)
		return accountservice.AccountIdentitySafe{}, ErrExpiredCode
	}
	if !hmac.Equal([]byte(challenge.CodeHash), []byte(s.codeHash(accountID, normalizedPhone, code))) {
		return accountservice.AccountIdentitySafe{}, ErrInvalidCode
	}
	_ = s.store.DeleteChallenge(ctx, key)
	return s.account.LinkVerifiedIdentity(ctx, accountID, accountID, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginPhoneOTP,
		ExternalID: normalizedPhone,
		Verified:   true,
	})
}

func (s *Service) checkLimit(ctx context.Context, key string, limit int, window time.Duration) error {
	if limit <= 0 {
		return nil
	}
	count, err := s.store.Increment(ctx, key, window)
	if err != nil {
		return err
	}
	if count > int64(limit) {
		return ErrRateLimited
	}
	return nil
}

func (s *Service) codeHash(accountID uuid.UUID, normalizedIdentity, code string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.HashSecret))
	_, _ = mac.Write([]byte(accountID.String()))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(normalizedIdentity))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(strings.TrimSpace(code)))
	return hex.EncodeToString(mac.Sum(nil))
}

func normalizeConfig(cfg Config) Config {
	if cfg.CodeTTL <= 0 {
		cfg.CodeTTL = defaultEmailCodeTTL
	}
	if cfg.CodeDigits <= 0 {
		cfg.CodeDigits = defaultEmailCodeDigits
	}
	if cfg.RequestLimit < 0 {
		cfg.RequestLimit = 0
	}
	if cfg.RequestLimit == 0 {
		cfg.RequestLimit = defaultRequestLimit
	}
	if cfg.RequestWindow <= 0 {
		cfg.RequestWindow = defaultRequestWindow
	}
	if cfg.VerifyLimit < 0 {
		cfg.VerifyLimit = 0
	}
	if cfg.VerifyLimit == 0 {
		cfg.VerifyLimit = defaultVerifyLimit
	}
	if cfg.VerifyWindow <= 0 {
		cfg.VerifyWindow = defaultVerifyWindow
	}
	if cfg.PhoneCodeTTL <= 0 {
		cfg.PhoneCodeTTL = defaultPhoneOTPTTL
	}
	if cfg.PhoneCodeDigits <= 0 {
		cfg.PhoneCodeDigits = defaultPhoneOTPDigits
	}
	if cfg.PhoneRequestLimit < 0 {
		cfg.PhoneRequestLimit = 0
	}
	if cfg.PhoneRequestLimit == 0 {
		cfg.PhoneRequestLimit = defaultRequestLimit
	}
	if cfg.PhoneRequestWindow <= 0 {
		cfg.PhoneRequestWindow = defaultRequestWindow
	}
	if cfg.PhoneVerifyLimit < 0 {
		cfg.PhoneVerifyLimit = 0
	}
	if cfg.PhoneVerifyLimit == 0 {
		cfg.PhoneVerifyLimit = defaultVerifyLimit
	}
	if cfg.PhoneVerifyWindow <= 0 {
		cfg.PhoneVerifyWindow = defaultVerifyWindow
	}
	if strings.TrimSpace(cfg.HashSecret) == "" {
		cfg.HashSecret = defaultHashSecret
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return cfg
}

func normalizeEmail(email string) (string, error) {
	email = strings.TrimSpace(email)
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", ErrInvalidEmail
	}
	return domain.NormalizeExternalIdentity(domain.IdentityProviderEmail, addr.Address)
}

func normalizePhone(phone string) (string, error) {
	normalized, err := domain.NormalizeExternalIdentity(domain.IdentityProviderPhone, phone)
	if err != nil {
		return "", ErrInvalidPhone
	}
	digits := normalized
	if strings.HasPrefix(digits, "+") {
		digits = strings.TrimPrefix(digits, "+")
	}
	if len(digits) < 7 || len(digits) > 15 {
		return "", ErrInvalidPhone
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return "", ErrInvalidPhone
		}
	}
	if strings.Contains(digits, "+") {
		return "", ErrInvalidPhone
	}
	return normalized, nil
}

func hashIdentity(normalizedIdentity string) string {
	sum := sha256.Sum256([]byte(normalizedIdentity))
	return hex.EncodeToString(sum[:])
}

func challengeKey(accountID uuid.UUID, emailHash string) string {
	return emailChallengeKeyPrefix + accountID.String() + ":" + emailHash
}

func requestRateKey(accountID uuid.UUID, emailHash string) string {
	return emailRequestRateKeyPrefix + accountID.String() + ":" + emailHash
}

func verifyRateKey(accountID uuid.UUID, emailHash string) string {
	return emailVerifyRateKeyPrefix + accountID.String() + ":" + emailHash
}

func phoneChallengeKey(accountID uuid.UUID, phoneHash string) string {
	return phoneChallengeKeyPrefix + accountID.String() + ":" + phoneHash
}

func phoneRequestRateKey(accountID uuid.UUID, phoneHash string) string {
	return phoneRequestRateKeyPrefix + accountID.String() + ":" + phoneHash
}

func phoneVerifyRateKey(accountID uuid.UUID, phoneHash string) string {
	return phoneVerifyRateKeyPrefix + accountID.String() + ":" + phoneHash
}

func generateNumericCode(digits int) (string, error) {
	max := int64(math.Pow10(digits))
	n, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", digits, n.Int64()), nil
}

// DisabledSender is the safe default until an SMTP/provider sender is wired.
type DisabledSender struct{}

// SendEmailLinkCode returns ErrDeliveryUnavailable without exposing the code.
func (DisabledSender) SendEmailLinkCode(context.Context, string, string, time.Time) error {
	return ErrDeliveryUnavailable
}

// SendPhoneLinkOTP returns ErrSMSDeliveryUnavailable without exposing the OTP.
func (DisabledSender) SendPhoneLinkOTP(context.Context, string, string, time.Time) error {
	return ErrSMSDeliveryUnavailable
}
