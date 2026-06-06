package memory

import (
	"context"
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
	now := time.Now()
	referral.CreatedAt, referral.UpdatedAt = now, now
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

func (r *ReferralRepo) MarkRewardApplied(_ context.Context, referralID uuid.UUID, rewardedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	referral, ok := r.referralsByID[referralID]
	if !ok {
		return domain.ErrNotFound
	}
	referral.RewardStatus = domain.ReferralRewardApplied
	referral.RewardedAt = &rewardedAt
	referral.UpdatedAt = time.Now()
	r.referralsByID[referralID] = referral
	return nil
}
