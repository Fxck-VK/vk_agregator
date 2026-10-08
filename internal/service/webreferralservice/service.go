// Package webreferralservice owns account-native web referral rules.
package webreferralservice

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

const (
	defaultCodeLength = 10
	defaultVisitTTL   = 30 * 24 * time.Hour
)

const referralAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

// Summary is the no-PII account-owned web referral funnel returned to the web
// handler. Stage counters are cumulative: Registered includes Activated and
// Rewarded, and Activated includes Rewarded.
type Summary struct {
	Code           string `json:"code"`
	Visits         int    `json:"visits"`
	Registered     int    `json:"registered"`
	Activated      int    `json:"activated"`
	Rewarded       int    `json:"rewarded"`
	RewardsEnabled bool   `json:"rewards_enabled"`
}

// Repository is the storage boundary used by Service.
type Repository interface {
	CodeByAccountID(ctx context.Context, accountID uuid.UUID) (*domain.WebReferralCode, error)
	CreateCode(ctx context.Context, code *domain.WebReferralCode) error
	Summary(ctx context.Context, accountID uuid.UUID) (Summary, error)
	Capture(ctx context.Context, visit domain.WebReferralVisit) error
	Accept(ctx context.Context, tokenHash string, accountID uuid.UUID, now time.Time) error
}

// Service provides account-native web referral operations.
type Service struct {
	repo       Repository
	visitTTL   time.Duration
	codeLength int
	newCode    func(int) (string, error)
}

// Option customizes a Service.
type Option func(*Service)

// WithCodeGenerator overrides referral-code generation for tests.
func WithCodeGenerator(fn func(int) (string, error)) Option {
	return func(s *Service) {
		if fn != nil {
			s.newCode = fn
		}
	}
}

// WithVisitTTL overrides the referral visit cookie lifetime.
func WithVisitTTL(ttl time.Duration) Option {
	return func(s *Service) {
		if ttl > 0 {
			s.visitTTL = ttl
		}
	}
}

// New builds a web referral service.
func New(repo Repository, opts ...Option) *Service {
	s := &Service{
		repo:       repo,
		visitTTL:   defaultVisitTTL,
		codeLength: defaultCodeLength,
		newCode:    generateCode,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Summary returns the stable account referral code and aggregate funnel
// counters. Web referral rewards remain disabled until a product decision
// enables account-native reward qualification.
func (s *Service) Summary(ctx context.Context, accountID uuid.UUID) (Summary, error) {
	if s == nil || s.repo == nil {
		return Summary{}, errors.New("webreferralservice: repository is required")
	}
	if accountID == uuid.Nil {
		return Summary{}, domain.ErrForbidden
	}
	code, err := s.ensureCode(ctx, accountID)
	if err != nil {
		return Summary{}, err
	}
	summary, err := s.repo.Summary(ctx, accountID)
	if err != nil {
		return Summary{}, err
	}
	summary.Code = code.Code
	summary.RewardsEnabled = false
	return summary, nil
}

// Capture records the first referral visit for a hashed browser token.
func (s *Service) Capture(ctx context.Context, code, tokenHash string, now time.Time) error {
	if s == nil || s.repo == nil {
		return errors.New("webreferralservice: repository is required")
	}
	code = NormalizeCode(code)
	if code == "" {
		return domain.ErrNotFound
	}
	tokenHash, ok := normalizeTokenHash(tokenHash)
	if !ok {
		return domain.ErrForbidden
	}
	if now.IsZero() {
		now = time.Now()
	}
	return s.repo.Capture(ctx, domain.WebReferralVisit{
		TokenHash: tokenHash,
		Code:      code,
		CreatedAt: now,
		ExpiresAt: now.Add(s.visitTTL),
	})
}

// Accept consumes a captured token for a newly created account. It is
// idempotent for the same token/account pair and returns typed errors for safe,
// non-blocking auth integration.
func (s *Service) Accept(ctx context.Context, tokenHash string, accountID uuid.UUID, now time.Time) error {
	if s == nil || s.repo == nil {
		return errors.New("webreferralservice: repository is required")
	}
	tokenHash, ok := normalizeTokenHash(tokenHash)
	if !ok || accountID == uuid.Nil {
		return domain.ErrForbidden
	}
	if now.IsZero() {
		now = time.Now()
	}
	return s.repo.Accept(ctx, tokenHash, accountID, now)
}

func (s *Service) ensureCode(ctx context.Context, accountID uuid.UUID) (*domain.WebReferralCode, error) {
	code, err := s.repo.CodeByAccountID(ctx, accountID)
	if err == nil {
		return code, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	for attempt := 0; attempt < 8; attempt++ {
		value, err := s.newCode(s.codeLength)
		if err != nil {
			return nil, err
		}
		code = &domain.WebReferralCode{AccountID: accountID, Code: value}
		if err := s.repo.CreateCode(ctx, code); err == nil {
			return code, nil
		} else if !errors.Is(err, domain.ErrConflict) {
			return nil, err
		}
		if existing, err := s.repo.CodeByAccountID(ctx, accountID); err == nil {
			return existing, nil
		} else if !errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
	}
	return nil, errors.New("webreferralservice: could not allocate unique referral code")
}

// NormalizeCode validates the public referral-code alphabet and returns the
// canonical uppercase value. Invalid values return an empty string.
func NormalizeCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) < 4 || len(value) > 64 {
		return ""
	}
	for _, r := range value {
		if !strings.ContainsRune(referralAlphabet, r) && r != '_' && r != '-' {
			return ""
		}
	}
	return value
}

func normalizeTokenHash(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 64 {
		return "", false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return "", false
		}
	}
	return value, true
}

func generateCode(length int) (string, error) {
	if length <= 0 {
		length = defaultCodeLength
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("webreferralservice: random code: %w", err)
	}
	out := make([]byte, length)
	for i, b := range buf {
		out[i] = referralAlphabet[int(b)%len(referralAlphabet)]
	}
	return string(out), nil
}
