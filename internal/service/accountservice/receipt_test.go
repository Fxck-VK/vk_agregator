package accountservice_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountservice"
)

type receiptIdentityStore struct {
	rows      []*domain.AccountIdentity
	err       error
	accountID uuid.UUID
}

func (s *receiptIdentityStore) ListIdentitiesByAccount(_ context.Context, accountID uuid.UUID, limit, offset int) ([]*domain.AccountIdentity, error) {
	s.accountID = accountID
	if s.err != nil {
		return nil, s.err
	}
	if offset >= len(s.rows) {
		return nil, nil
	}
	return s.rows[offset:min(offset+limit, len(s.rows))], nil
}

func TestVerifiedReceiptEmailUsesOnlyOwnedVerifiedEmail(t *testing.T) {
	owner := uuid.New()
	valid := &domain.AccountIdentity{AccountID: owner, Provider: domain.IdentityProviderEmail, NormalizedID: "buyer@example.test", VerifiedAt: time.Now()}
	for _, tc := range []struct {
		name   string
		change func(*domain.AccountIdentity)
	}{
		{"unverified", func(i *domain.AccountIdentity) { i.VerifiedAt = time.Time{} }},
		{"foreign", func(i *domain.AccountIdentity) { i.AccountID = uuid.New() }},
		{"google subject", func(i *domain.AccountIdentity) { i.Provider = domain.IdentityProviderGoogle }},
		{"invalid", func(i *domain.AccountIdentity) { i.NormalizedID = "not-email" }},
		{"display name", func(i *domain.AccountIdentity) { i.NormalizedID = "Buyer <buyer@example.test>" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invalid := *valid
			tc.change(&invalid)
			store := &receiptIdentityStore{rows: []*domain.AccountIdentity{nil, &invalid}}
			service := accountservice.New(store, nil)
			if email, err := service.VerifiedReceiptEmail(context.Background(), owner); email != "" || !errors.Is(err, accountservice.ErrReceiptEmailUnavailable) {
				t.Fatal("invalid identity accepted for receipt")
			}
			store.rows = append(store.rows, valid)
			if email, err := service.VerifiedReceiptEmail(context.Background(), owner); err != nil || email != valid.NormalizedID || store.accountID != owner {
				t.Fatal("owned verified email not resolved")
			}
		})
	}
}

func TestVerifiedReceiptEmailHandlesPaginationAndStoreFailure(t *testing.T) {
	owner := uuid.New()
	store := &receiptIdentityStore{rows: make([]*domain.AccountIdentity, 100)}
	store.rows = append(store.rows, &domain.AccountIdentity{AccountID: owner, Provider: domain.IdentityProviderEmail, NormalizedID: "buyer@example.test", VerifiedAt: time.Now()})
	service := accountservice.New(store, nil)
	if email, err := service.VerifiedReceiptEmail(context.Background(), owner); err != nil || email == "" {
		t.Fatal("second page ignored")
	}
	store.err = errors.New("store unavailable")
	if _, err := service.VerifiedReceiptEmail(context.Background(), owner); !errors.Is(err, store.err) {
		t.Fatal("store failure hidden")
	}
	if _, err := service.VerifiedReceiptEmail(context.Background(), uuid.Nil); !errors.Is(err, domain.ErrInvalidIdentity) {
		t.Fatal("nil account accepted")
	}
}
