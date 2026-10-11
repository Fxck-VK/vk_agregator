package webreferralservice_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/webreferralservice"
)

func TestSummaryReusesAccountCodeAndForcesRewardsDisabled(t *testing.T) {
	ctx := context.Background()
	accountID := uuid.New()
	repo := &fakeRepository{
		code: &domain.WebReferralCode{
			AccountID: accountID,
			Code:      "WEB2345678",
		},
		summary: webreferralservice.Summary{
			Visits:         4,
			Registered:     3,
			Activated:      2,
			Rewarded:       1,
			RewardsEnabled: true,
		},
	}
	svc := webreferralservice.New(repo)

	got, err := svc.Summary(ctx, accountID)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if got.Code != "WEB2345678" || got.Visits != 4 || got.Registered != 3 || got.Activated != 2 || got.Rewarded != 1 {
		t.Fatalf("unexpected summary: %+v", got)
	}
	if got.RewardsEnabled {
		t.Fatal("web referral rewards must stay disabled")
	}
	if repo.created != nil {
		t.Fatalf("existing account code should be reused, created %+v", repo.created)
	}
}

func TestSummaryGeneratesCodeAndRetriesUniqueConflict(t *testing.T) {
	ctx := context.Background()
	accountID := uuid.New()
	repo := &fakeRepository{
		codeErrs: []error{domain.ErrNotFound, domain.ErrNotFound},
		createErrs: []error{
			domain.ErrConflict,
			nil,
		},
	}
	generated := []string{"DUP2345678", "NEW2345678"}
	svc := webreferralservice.New(repo, webreferralservice.WithCodeGenerator(func(int) (string, error) {
		next := generated[0]
		generated = generated[1:]
		return next, nil
	}))

	got, err := svc.Summary(ctx, accountID)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if got.Code != "NEW2345678" {
		t.Fatalf("code = %q, want retry-generated code", got.Code)
	}
	if len(repo.createdCodes) != 2 || repo.createdCodes[0] != "DUP2345678" || repo.createdCodes[1] != "NEW2345678" {
		t.Fatalf("created codes = %+v, want duplicate then retry", repo.createdCodes)
	}
}

func TestStatisticsReadsCountersWithoutLookingUpOrCreatingCode(t *testing.T) {
	id := uuid.New()
	repo := &fakeRepository{summary: webreferralservice.Summary{Visits: 7, Registered: 2, Activated: 1, Rewarded: 1, RewardsEnabled: true}}
	got, err := webreferralservice.New(repo).Statistics(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Visits != 7 || got.Registered != 2 || got.Activated != 1 || got.Rewarded != 1 || got.RewardsEnabled {
		t.Fatalf("unexpected statistics: %+v", got)
	}
	if repo.codeReads != 0 || repo.created != nil || repo.summaryAccountID != id {
		t.Fatal("statistics must read only this account's counters")
	}
	if _, err := webreferralservice.New(repo).Statistics(context.Background(), uuid.Nil); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("nil account: %v", err)
	}
	if _, err := webreferralservice.New(nil).Statistics(context.Background(), id); err == nil {
		t.Fatal("missing repository must fail")
	}
	repo.summaryErr = errors.New("storage unavailable")
	if _, err := webreferralservice.New(repo).Statistics(context.Background(), id); !errors.Is(err, repo.summaryErr) {
		t.Fatalf("storage error: %v", err)
	}
}

func TestCaptureNormalizesCodeAndStoresFirstTouchVisit(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 8, 9, 30, 0, 0, time.UTC)
	tokenHash := strings.Repeat("a", 64)
	repo := &fakeRepository{}
	svc := webreferralservice.New(repo)

	if err := svc.Capture(ctx, " web2345678 ", tokenHash, now); err != nil {
		t.Fatalf("capture: %v", err)
	}
	if repo.visit.TokenHash != tokenHash || repo.visit.Code != "WEB2345678" {
		t.Fatalf("stored visit = %+v", repo.visit)
	}
	if !repo.visit.CreatedAt.Equal(now) {
		t.Fatalf("created_at = %s, want %s", repo.visit.CreatedAt, now)
	}
	if !repo.visit.ExpiresAt.Equal(now.Add(30 * 24 * time.Hour)) {
		t.Fatalf("expires_at = %s, want 30 day ttl", repo.visit.ExpiresAt)
	}
}

func TestCaptureRejectsInvalidTokenHash(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepository{}
	svc := webreferralservice.New(repo)

	err := svc.Capture(ctx, "WEB2345678", "not-a-sha256-hex", time.Now())
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("capture error = %v, want ErrForbidden", err)
	}
	if repo.visit.TokenHash != "" {
		t.Fatalf("invalid token should not reach repository: %+v", repo.visit)
	}
}

func TestAcceptDelegatesValidatedTokenAndAccount(t *testing.T) {
	ctx := context.Background()
	accountID := uuid.New()
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	tokenHash := strings.Repeat("b", 64)
	repo := &fakeRepository{}
	svc := webreferralservice.New(repo)

	if err := svc.Accept(ctx, tokenHash, accountID, now); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if repo.acceptTokenHash != tokenHash || repo.acceptAccountID != accountID || !repo.acceptNow.Equal(now) {
		t.Fatalf("accept args token=%q account=%s now=%s", repo.acceptTokenHash, repo.acceptAccountID, repo.acceptNow)
	}
}

func TestAcceptRejectsNilAccount(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepository{}
	svc := webreferralservice.New(repo)

	err := svc.Accept(ctx, strings.Repeat("c", 64), uuid.Nil, time.Now())
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("accept error = %v, want ErrForbidden", err)
	}
	if repo.acceptTokenHash != "" {
		t.Fatalf("nil account should not reach repository")
	}
}

type fakeRepository struct {
	codeReads        int
	summaryAccountID uuid.UUID
	summaryErr       error
	code             *domain.WebReferralCode
	codeErrs         []error
	summary          webreferralservice.Summary
	created          *domain.WebReferralCode
	createErrs       []error

	createdCodes []string

	visit domain.WebReferralVisit

	acceptTokenHash string
	acceptAccountID uuid.UUID
	acceptNow       time.Time
}

func (r *fakeRepository) CodeByAccountID(_ context.Context, accountID uuid.UUID) (*domain.WebReferralCode, error) {
	r.codeReads++
	if len(r.codeErrs) > 0 {
		err := r.codeErrs[0]
		r.codeErrs = r.codeErrs[1:]
		return nil, err
	}
	if r.code == nil {
		return nil, domain.ErrNotFound
	}
	copy := *r.code
	copy.AccountID = accountID
	return &copy, nil
}

func (r *fakeRepository) CreateCode(_ context.Context, code *domain.WebReferralCode) error {
	copy := *code
	r.created = &copy
	r.createdCodes = append(r.createdCodes, copy.Code)
	if len(r.createErrs) > 0 {
		err := r.createErrs[0]
		r.createErrs = r.createErrs[1:]
		if err != nil {
			return err
		}
	}
	r.code = &copy
	return nil
}

func (r *fakeRepository) Summary(_ context.Context, id uuid.UUID) (webreferralservice.Summary, error) {
	r.summaryAccountID = id
	return r.summary, r.summaryErr
}

func (r *fakeRepository) Capture(_ context.Context, visit domain.WebReferralVisit) error {
	r.visit = visit
	return nil
}

func (r *fakeRepository) Accept(_ context.Context, tokenHash string, accountID uuid.UUID, now time.Time) error {
	r.acceptTokenHash = tokenHash
	r.acceptAccountID = accountID
	r.acceptNow = now
	return nil
}
