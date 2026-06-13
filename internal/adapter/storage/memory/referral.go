package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// ReferralRepo is an in-memory domain.ReferralRepository.
type ReferralRepo struct {
	mu             sync.Mutex
	codesByUser    map[uuid.UUID]domain.ReferralCode
	codesByValue   map[string]uuid.UUID
	referralsByID  map[uuid.UUID]domain.Referral
	referredToID   map[uuid.UUID]uuid.UUID
	referrerToRefs map[uuid.UUID][]uuid.UUID
}

// NewReferralRepo builds an empty ReferralRepo.
func NewReferralRepo() *ReferralRepo {
	return &ReferralRepo{
		codesByUser:    map[uuid.UUID]domain.ReferralCode{},
		codesByValue:   map[string]uuid.UUID{},
		referralsByID:  map[uuid.UUID]domain.Referral{},
		referredToID:   map[uuid.UUID]uuid.UUID{},
		referrerToRefs: map[uuid.UUID][]uuid.UUID{},
	}
}

var _ domain.ReferralRepository = (*ReferralRepo)(nil)

func (r *ReferralRepo) GetCodeByUserID(_ context.Context, userID uuid.UUID) (*domain.ReferralCode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	code, ok := r.codesByUser[userID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &code, nil
}

func (r *ReferralRepo) GetCode(_ context.Context, codeValue string) (*domain.ReferralCode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	userID, ok := r.codesByValue[codeValue]
	if !ok {
		return nil, domain.ErrNotFound
	}
	code := r.codesByUser[userID]
	return &code, nil
}

func (r *ReferralRepo) CreateCode(_ context.Context, code *domain.ReferralCode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.codesByUser[code.UserID]; ok {
		return domain.ErrConflict
	}
	if _, ok := r.codesByValue[code.Code]; ok {
		return domain.ErrConflict
	}
	if code.ID == uuid.Nil {
		code.ID = uuid.New()
	}
	now := time.Now()
	code.CreatedAt, code.UpdatedAt = now, now
	r.codesByUser[code.UserID] = *code
	r.codesByValue[code.Code] = code.UserID
	return nil
}

func (r *ReferralRepo) CreateReferral(_ context.Context, referral *domain.Referral) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if referral.ReferrerUserID == referral.ReferredUserID {
		return domain.ErrConflict
	}
	if _, ok := r.referredToID[referral.ReferredUserID]; ok {
		return domain.ErrConflict
	}
	if referral.ID == uuid.Nil {
		referral.ID = uuid.New()
	}
	if referral.Source == "" {
		referral.Source = domain.ReferralSourceVKBot
	}
	if referral.RewardStatus == "" {
		referral.RewardStatus = domain.ReferralRewardPending
	}
	if referral.Status == "" {
		referral.Status = domain.ReferralStatusRegistered
	}
	now := time.Now()
	referral.CreatedAt, referral.UpdatedAt = now, now
	if referral.FirstSeenAt.IsZero() {
		referral.FirstSeenAt = now
	}
	r.referralsByID[referral.ID] = *referral
	r.referredToID[referral.ReferredUserID] = referral.ID
	r.referrerToRefs[referral.ReferrerUserID] = append(r.referrerToRefs[referral.ReferrerUserID], referral.ID)
	return nil
}

func (r *ReferralRepo) GetReferralByReferredUserID(_ context.Context, userID uuid.UUID) (*domain.Referral, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.referredToID[userID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	referral := r.referralsByID[id]
	return &referral, nil
}

func (r *ReferralRepo) CountByReferrer(_ context.Context, referrerUserID uuid.UUID) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.referrerToRefs[referrerUserID]), nil
}

func (r *ReferralRepo) CountByReferrerStatus(_ context.Context, referrerUserID uuid.UUID) (domain.ReferralStats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var stats domain.ReferralStats
	for _, id := range r.referrerToRefs[referrerUserID] {
		referral := r.referralsByID[id]
		addReferralStatus(&stats, referral)
	}
	return stats, nil
}

func (r *ReferralRepo) StatsByReferralCode(_ context.Context, code string) (domain.ReferralCodeStats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.codesByValue[code]; !ok {
		return domain.ReferralCodeStats{}, domain.ErrNotFound
	}
	stats := r.statsByCodeLocked(code)
	return domain.ReferralCodeStats{Code: code, Stats: stats}, nil
}

func (r *ReferralRepo) ListSuspiciousReferralCodes(_ context.Context, filter domain.ReferralSuspiciousFilter) ([]domain.ReferralCodeStats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	var out []domain.ReferralCodeStats
	for code := range r.codesByValue {
		stats := r.statsByCodeLocked(code)
		if stats.Total() == 0 {
			continue
		}
		if filter.MinRegistered > 0 && stats.RegisteredCount >= filter.MinRegistered ||
			filter.MinTotal > 0 && stats.Total() >= filter.MinTotal {
			out = append(out, domain.ReferralCodeStats{Code: code, Stats: stats})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		left, right := out[i].Stats, out[j].Stats
		if left.Total() != right.Total() {
			return left.Total() > right.Total()
		}
		if left.RegisteredCount != right.RegisteredCount {
			return left.RegisteredCount > right.RegisteredCount
		}
		return out[i].Code < out[j].Code
	})
	if len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, nil
}

func (r *ReferralRepo) ReferralStatusDistribution(_ context.Context) (domain.ReferralStats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var stats domain.ReferralStats
	for _, referral := range r.referralsByID {
		addReferralStatus(&stats, referral)
	}
	return stats, nil
}

func (r *ReferralRepo) statsByCodeLocked(code string) domain.ReferralStats {
	var stats domain.ReferralStats
	for _, referral := range r.referralsByID {
		if referral.ReferralCode != code {
			continue
		}
		addReferralStatus(&stats, referral)
	}
	return stats
}

func addReferralStatus(stats *domain.ReferralStats, referral domain.Referral) {
	switch referral.Status {
	case domain.ReferralStatusRewarded:
		stats.RewardedCount++
	case domain.ReferralStatusActivated:
		stats.ActivatedCount++
	default:
		if referral.RewardStatus == domain.ReferralRewardApplied {
			stats.RewardedCount++
		} else {
			stats.RegisteredCount++
		}
	}
}

func (r *ReferralRepo) MarkActivated(_ context.Context, referralID uuid.UUID, activatedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	referral, ok := r.referralsByID[referralID]
	if !ok {
		return domain.ErrNotFound
	}
	if referral.Status == domain.ReferralStatusRewarded {
		return domain.ErrNotFound
	}
	if referral.Status == "" || referral.Status == domain.ReferralStatusRegistered {
		referral.Status = domain.ReferralStatusActivated
	}
	if referral.ActivatedAt == nil {
		referral.ActivatedAt = &activatedAt
	}
	referral.UpdatedAt = time.Now()
	r.referralsByID[referralID] = referral
	return nil
}

func (r *ReferralRepo) MarkRewardApplied(_ context.Context, referralID uuid.UUID, rewardedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	referral, ok := r.referralsByID[referralID]
	if !ok {
		return domain.ErrNotFound
	}
	referral.Status = domain.ReferralStatusRewarded
	referral.RewardStatus = domain.ReferralRewardApplied
	if referral.ActivatedAt == nil {
		referral.ActivatedAt = &rewardedAt
	}
	referral.RewardedAt = &rewardedAt
	referral.UpdatedAt = time.Now()
	r.referralsByID[referralID] = referral
	return nil
}
