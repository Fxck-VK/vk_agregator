package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

func TestEmailRolesPreserveLegacyBindingsAndBreakTimestampTies(t *testing.T) {
	at := time.Now()
	first := &domain.AccountIdentity{ID: uuid.MustParse("10000000-0000-4000-8000-000000000001"), Provider: domain.IdentityProviderEmail, CreatedAt: at, VerifiedAt: at}
	second := &domain.AccountIdentity{ID: uuid.MustParse("10000000-0000-4000-8000-000000000002"), Provider: domain.IdentityProviderEmail, CreatedAt: at, VerifiedAt: at}
	extra := &domain.AccountIdentity{ID: uuid.New(), Provider: domain.IdentityProviderEmail, CreatedAt: at.Add(time.Second), VerifiedAt: at}
	unverified := &domain.AccountIdentity{ID: uuid.New(), Provider: domain.IdentityProviderEmail, CreatedAt: at.Add(-time.Second)}
	roles := domain.AccountEmailRoles([]*domain.AccountIdentity{extra, second, unverified, first})
	if roles[first.ID] != domain.AccountEmailPrimary || roles[second.ID] != domain.AccountEmailBackup || roles[extra.ID] != domain.AccountEmailAdditional || len(roles) != 3 {
		t.Fatal("roles not stable")
	}
}
