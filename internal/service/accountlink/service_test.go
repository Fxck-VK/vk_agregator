package accountlink_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/accountlink"
	"vk-ai-aggregator/internal/service/accountservice"
	"vk-ai-aggregator/internal/service/identityresolver"
)

func TestRequestVerifyEmailLinksIdentityAndWritesAudit(t *testing.T) {
	ctx := context.Background()
	linker, sender, identities, accountID := newLinkFixture(t)

	result, err := linker.RequestEmailCode(ctx, accountID, "Owner.Name@Example.COM")
	if err != nil {
		t.Fatalf("request email code: %v", err)
	}
	if result.Status != "verification_sent" || result.ExpiresInSeconds <= 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if sender.email != "owner.name@example.com" || sender.emailCode == "" {
		t.Fatalf("email sender did not receive normalized email/code: %+v", sender)
	}

	identity, err := linker.VerifyEmailCode(ctx, accountID, "owner.name@example.com", sender.emailCode)
	if err != nil {
		t.Fatalf("verify email code: %v", err)
	}
	if identity.Provider != domain.IdentityProviderEmail || identity.AccountID != accountID || identity.Label != "o***@example.com" {
		t.Fatalf("unexpected linked identity: %+v", identity)
	}
	assertAuditContainsLinkedIdentity(t, identities.AuditEntries(), accountID, domain.IdentityProviderEmail, identity.ID)
}

func TestRequestVerifyPhoneLinksIdentityAndWritesAudit(t *testing.T) {
	ctx := context.Background()
	linker, sender, identities, accountID := newLinkFixture(t)

	result, err := linker.RequestPhoneOTP(ctx, accountID, "+7 (999) 123-45-67")
	if err != nil {
		t.Fatalf("request phone otp: %v", err)
	}
	if result.Status != "verification_sent" || result.ExpiresInSeconds <= 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if sender.phone != "+79991234567" || sender.phoneCode == "" {
		t.Fatalf("phone sender did not receive normalized phone/code: %+v", sender)
	}

	identity, err := linker.VerifyPhoneOTP(ctx, accountID, "+79991234567", sender.phoneCode)
	if err != nil {
		t.Fatalf("verify phone otp: %v", err)
	}
	if identity.Provider != domain.IdentityProviderPhone || identity.AccountID != accountID || !strings.HasSuffix(identity.Label, "4567") {
		t.Fatalf("unexpected linked identity: %+v", identity)
	}
	assertAuditContainsLinkedIdentity(t, identities.AuditEntries(), accountID, domain.IdentityProviderPhone, identity.ID)
}

func newLinkFixture(t *testing.T) (*accountlink.Service, *capturingSender, *memory.AccountIdentityRepo, uuid.UUID) {
	t.Helper()
	identities := memory.NewAccountIdentityRepo()
	resolver := identityresolver.New(memory.NewUserRepo(), identities, nil)
	auth := accountauth.New(resolver)
	account := accountservice.New(identities, auth)
	owner, err := auth.ResolveVKID(context.Background(), 123456789)
	if err != nil {
		t.Fatalf("resolve owner: %v", err)
	}
	sender := &capturingSender{}
	linker, err := accountlink.New(accountlink.NewMemoryStore(), sender, account, accountlink.Config{
		CodeTTL:         time.Minute,
		CodeDigits:      6,
		PhoneCodeTTL:    time.Minute,
		PhoneCodeDigits: 6,
		HashSecret:      "test-secret",
	})
	if err != nil {
		t.Fatalf("new linker: %v", err)
	}
	return linker, sender, identities, owner.AccountID
}

func assertAuditContainsLinkedIdentity(t *testing.T, audits []domain.AccountLinkAuditEntry, accountID uuid.UUID, provider domain.IdentityProvider, identityID uuid.UUID) {
	t.Helper()
	found := false
	for _, audit := range audits {
		if audit.AccountID == accountID &&
			audit.Action == domain.AccountLinkActionLinked &&
			audit.Provider == provider &&
			audit.IdentityID != nil &&
			*audit.IdentityID == identityID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing linked audit for provider=%s identity=%s: %+v", provider, identityID, audits)
	}
	encoded, err := json.Marshal(audits)
	if err != nil {
		t.Fatalf("marshal audits: %v", err)
	}
	for _, forbidden := range []string{"owner.name@example.com", "+79991234567", "9991234567", "123-45"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("audit leaked raw identity %q: %s", forbidden, encoded)
		}
	}
}

type capturingSender struct {
	email     string
	emailCode string
	phone     string
	phoneCode string
}

func (s *capturingSender) SendEmailLinkCode(_ context.Context, email, code string, _ time.Time) error {
	s.email = email
	s.emailCode = code
	return nil
}

func (s *capturingSender) SendPhoneLinkOTP(_ context.Context, phone, code string, _ time.Time) error {
	s.phone = phone
	s.phoneCode = code
	return nil
}
