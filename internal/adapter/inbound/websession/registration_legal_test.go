package websession

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/service/legalacceptance"
)

type registrationLegalRepo struct {
	records []legalacceptance.Record
	fail    error
}

func (s *registrationLegalRepo) State(context.Context, uuid.UUID, legalacceptance.Metadata) (legalacceptance.State, error) {
	return legalacceptance.State{}, s.fail
}
func (s *registrationLegalRepo) Record(_ context.Context, in legalacceptance.Record) error {
	if s.fail != nil {
		return s.fail
	}
	s.records = append(s.records, in)
	return nil
}

func TestRegistrationRecordsDisplayedTermsBeforeIssuingCookies(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	account := uuid.New()
	spy := &signupServiceSpy{sessions: sessions, accountID: account}
	h.deps.EmailRegistration = spy
	h.cfg.EmailDeliveryEnabled = true
	repo := &registrationLegalRepo{}
	h.deps.Legal = legalacceptance.New(repo, legalacceptance.Metadata{Published: true, Version: "v1", Checksum: strings.Repeat("a", 64)})
	send := func(version string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/web/v1/auth/email/register", strings.NewReader(`{"email":"new@example.test","password":"strong-password","registration":{"version":"`+version+`","checksum":"`+strings.Repeat("a", 64)+`","locale":"ru"}}`))
		r.Header.Set("Origin", "https://app.example.test")
		r.AddCookie(&http.Cookie{Name: registrationCookieName, Value: strings.Repeat("b", 43)})
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, r)
		return rec
	}
	rec := send("v1")
	if rec.Code != 201 {
		t.Fatalf("got %d want 201", rec.Code)
	}
	if len(repo.records) != 1 || repo.records[0].AccountID != account || repo.records[0].MarketingOptIn || repo.records[0].Metadata.Version != "v1" {
		t.Fatal("registration evidence missing or fabricated")
	}
	firstID := repo.records[0].RequestID
	rec = send("v1")
	if rec.Code != 201 || repo.records[len(repo.records)-1].RequestID != firstID {
		t.Fatal("registration retry must reuse evidence idempotency")
	}
	spy.binding = ""
	rec = send("old")
	if rec.Code != 409 || spy.binding != "" || len(rec.Result().Cookies()) != 0 {
		t.Fatalf("stale notice reached signup or cookies: %d", rec.Code)
	}
	if rec.Header().Get("X-NeiroHub-Legal-Revision") != "changed" {
		t.Fatal("revision conflict indistinguishable from existing email")
	}
	repo.fail = errors.New("synthetic storage failure")
	rec = send("v1")
	if rec.Code != 503 || len(rec.Result().Cookies()) != 0 {
		t.Fatal("issued cookies despite evidence storage failure")
	}
}
