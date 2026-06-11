// Package referralservice owns shared VK referral business rules for the bot
// and future Mini App entry points.
package referralservice

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/service/billingservice"
)

const (
	defaultCodeLength = 10
)

const referralAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

// Config controls referral code generation and signup rewards.
type Config struct {
	CodeLength                  int
	ReferrerSignupRewardCredits int64
	ReferredSignupRewardCredits int64
	// RewardOnActivation gates rollout of signup rewards. When false, Activate
	// only marks the referral as activated; a later Activate with this flag
	// enabled can safely post the ledger reward once.
	RewardOnActivation bool
}

// Service provides shared referral operations.
type Service struct {
	repo    domain.ReferralRepository
	billing *billingservice.Service
	cfg     Config
	now     func() time.Time
	newCode func(int) (string, error)
}

// Option customizes a Service.
type Option func(*Service)

// WithClock overrides the time source.
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// WithCodeGenerator overrides code generation for tests.
func WithCodeGenerator(fn func(int) (string, error)) Option {
	return func(s *Service) { s.newCode = fn }
}

// New builds a referral service.
func New(repo domain.ReferralRepository, billing *billingservice.Service, cfg Config, opts ...Option) *Service {
	if cfg.CodeLength <= 0 {
		cfg.CodeLength = defaultCodeLength
	}
	s := &Service{
		repo:    repo,
		billing: billing,
		cfg:     cfg,
		now:     time.Now,
		newCode: generateCode,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// EnsureCode returns the user's single stable referral code, creating it if
// needed. Codes are random and do not expose vk_user_id or internal UUIDs.
func (s *Service) EnsureCode(ctx context.Context, userID uuid.UUID) (*domain.ReferralCode, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("referralservice: repository is required")
	}
	code, err := s.repo.GetCodeByUserID(ctx, userID)
	if err == nil {
		return code, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	for attempt := 0; attempt < 8; attempt++ {
		value, err := s.newCode(s.cfg.CodeLength)
		if err != nil {
			return nil, err
		}
		code = &domain.ReferralCode{UserID: userID, Code: value}
		if err := s.repo.CreateCode(ctx, code); err == nil {
			return code, nil
		} else if !errors.Is(err, domain.ErrConflict) {
			return nil, err
		}
		if existing, err := s.repo.GetCodeByUserID(ctx, userID); err == nil {
			return existing, nil
		}
	}
	return nil, errors.New("referralservice: could not allocate unique referral code")
}

// ApplyInput describes a referral code accepted from one VK surface.
type ApplyInput struct {
	Code           string
	ReferredUserID uuid.UUID
	Source         domain.ReferralSource
}

// ApplyResult reports how the referral code was handled.
type ApplyResult struct {
	Applied        bool
	AlreadyApplied bool
	InvalidCode    bool
	SelfReferral   bool
	Referral       *domain.Referral
}

// Apply records a referral relation if the code is valid and the user has not
// already been referred. It is idempotent per referred user and does not grant
// signup rewards; rewards are posted only by Activate.
func (s *Service) Apply(ctx context.Context, input ApplyInput) (ApplyResult, error) {
	if s == nil || s.repo == nil {
		metrics.ReferralRewards.WithLabelValues("service_unavailable").Inc()
		return ApplyResult{}, nil
	}
	codeValue := NormalizeCode(input.Code)
	if codeValue == "" || input.ReferredUserID == uuid.Nil {
		metrics.ReferralRewards.WithLabelValues("invalid_code").Inc()
		return ApplyResult{InvalidCode: true}, nil
	}
	code, err := s.repo.GetCode(ctx, codeValue)
	if errors.Is(err, domain.ErrNotFound) {
		metrics.ReferralRewards.WithLabelValues("invalid_code").Inc()
		return ApplyResult{InvalidCode: true}, nil
	}
	if err != nil {
		return ApplyResult{}, err
	}
	if code.UserID == input.ReferredUserID {
		metrics.ReferralRewards.WithLabelValues("self_referral").Inc()
		return ApplyResult{SelfReferral: true}, nil
	}
	if input.Source == "" {
		input.Source = domain.ReferralSourceVKBot
	}
	referral := &domain.Referral{
		ReferrerUserID: code.UserID,
		ReferredUserID: input.ReferredUserID,
		ReferralCode:   code.Code,
		Source:         input.Source,
		Status:         domain.ReferralStatusRegistered,
		RewardStatus:   domain.ReferralRewardPending,
	}
	if err := s.repo.CreateReferral(ctx, referral); err != nil {
		if !errors.Is(err, domain.ErrConflict) {
			return ApplyResult{}, err
		}
		existing, getErr := s.repo.GetReferralByReferredUserID(ctx, input.ReferredUserID)
		if getErr != nil {
			return ApplyResult{}, getErr
		}
		if existing.ReferrerUserID != code.UserID {
			metrics.ReferralRewards.WithLabelValues("already_applied_other_referrer").Inc()
			return ApplyResult{AlreadyApplied: true, Referral: existing}, nil
		}
		referral = existing
	} else {
		metrics.ReferralRewards.WithLabelValues("registered").Inc()
		return ApplyResult{Applied: true, Referral: referral}, nil
	}
	metrics.ReferralRewards.WithLabelValues("already_registered").Inc()
	return ApplyResult{AlreadyApplied: true, Referral: referral}, nil
}

// ActivateInput describes a product activation event for the referred user.
type ActivateInput struct {
	ReferredUserID uuid.UUID
	Source         domain.ReferralSource
}

// ActivateResult reports whether a referral was activated and rewarded.
type ActivateResult struct {
	Activated       bool
	Rewarded        bool
	AlreadyRewarded bool
	NotFound        bool
	Referral        *domain.Referral
}

// Activate marks the referred user's relation as activated and posts configured
// signup rewards through the billing ledger exactly once.
func (s *Service) Activate(ctx context.Context, input ActivateInput) (ActivateResult, error) {
	if s == nil || s.repo == nil || input.ReferredUserID == uuid.Nil {
		return ActivateResult{NotFound: true}, nil
	}
	referral, err := s.repo.GetReferralByReferredUserID(ctx, input.ReferredUserID)
	if errors.Is(err, domain.ErrNotFound) {
		return ActivateResult{NotFound: true}, nil
	}
	if err != nil {
		return ActivateResult{}, err
	}
	if referral.Status == domain.ReferralStatusRewarded || referral.RewardStatus == domain.ReferralRewardApplied {
		metrics.ReferralRewards.WithLabelValues("already_rewarded").Inc()
		return ActivateResult{AlreadyRewarded: true, Referral: referral}, nil
	}
	activatedAt := s.now()
	newlyActivated := false
	if referral.Status != domain.ReferralStatusActivated {
		if err := s.repo.MarkActivated(ctx, referral.ID, activatedAt); err != nil {
			return ActivateResult{}, err
		}
		referral.Status = domain.ReferralStatusActivated
		if referral.ActivatedAt == nil {
			referral.ActivatedAt = &activatedAt
		}
		newlyActivated = true
	}
	if !s.cfg.RewardOnActivation {
		metrics.ReferralRewards.WithLabelValues("activated_without_reward").Inc()
		return ActivateResult{Activated: newlyActivated, Referral: referral}, nil
	}
	if err := s.applySignupRewards(ctx, referral); err != nil {
		metrics.ReferralRewards.WithLabelValues("error").Inc()
		return ActivateResult{}, err
	}
	metrics.ReferralRewards.WithLabelValues("rewarded").Inc()
	return ActivateResult{Activated: newlyActivated, Rewarded: true, Referral: referral}, nil
}

// Stats returns the user's referral code and invited-user count.
func (s *Service) Stats(ctx context.Context, userID uuid.UUID) (*domain.ReferralCode, int, error) {
	code, stats, err := s.StatsDetailed(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	return code, stats.Total(), nil
}

// StatsDetailed returns the user's referral code and aggregate no-PII funnel
// counters for accepted invitations.
func (s *Service) StatsDetailed(ctx context.Context, userID uuid.UUID) (*domain.ReferralCode, domain.ReferralStats, error) {
	code, err := s.EnsureCode(ctx, userID)
	if err != nil {
		return nil, domain.ReferralStats{}, err
	}
	stats, err := s.repo.CountByReferrerStatus(ctx, userID)
	if err != nil {
		return nil, domain.ReferralStats{}, err
	}
	return code, stats, nil
}

func (s *Service) applySignupRewards(ctx context.Context, referral *domain.Referral) error {
	if referral == nil || referral.ID == uuid.Nil {
		return nil
	}
	if s.billing == nil {
		if s.cfg.ReferrerSignupRewardCredits > 0 || s.cfg.ReferredSignupRewardCredits > 0 {
			return errors.New("referralservice: billing service is required for referral rewards")
		}
		return s.repo.MarkRewardApplied(ctx, referral.ID, s.now())
	}
	if amount := s.cfg.ReferrerSignupRewardCredits; amount > 0 {
		if err := s.billing.Grant(ctx,
			referral.ReferrerUserID,
			amount,
			"referral:signup:referrer:"+referral.ID.String(),
			"referral signup reward",
		); err != nil {
			return err
		}
	}
	if amount := s.cfg.ReferredSignupRewardCredits; amount > 0 {
		if err := s.billing.Grant(ctx,
			referral.ReferredUserID,
			amount,
			"referral:signup:referred:"+referral.ID.String(),
			"referral signup bonus",
		); err != nil {
			return err
		}
	}
	rewardedAt := s.now()
	if err := s.repo.MarkRewardApplied(ctx, referral.ID, rewardedAt); err != nil {
		return err
	}
	referral.Status = domain.ReferralStatusRewarded
	referral.RewardStatus = domain.ReferralRewardApplied
	if referral.ActivatedAt == nil {
		referral.ActivatedAt = &rewardedAt
	}
	referral.RewardedAt = &rewardedAt
	return nil
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

func generateCode(length int) (string, error) {
	if length <= 0 {
		length = defaultCodeLength
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("referralservice: random code: %w", err)
	}
	out := make([]byte, length)
	for i, b := range buf {
		out[i] = referralAlphabet[int(b)%len(referralAlphabet)]
	}
	return string(out), nil
}
