package websession

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/legalacceptance"
)

type legalStub struct {
	status legalacceptance.Status
	actor  uuid.UUID
	calls  int
	input  legalacceptance.AcceptInput
}

func (s *legalStub) Status(_ context.Context, id uuid.UUID) (legalacceptance.Status, error) {
	s.actor = id
	return s.status, nil
}
func (s *legalStub) Accept(_ context.Context, id uuid.UUID, in legalacceptance.AcceptInput) (legalacceptance.Status, error) {
	s.actor = id
	s.calls++
	s.input = in
	return legalacceptance.Status{}, nil
}
func (s *legalStub) UpdateMarketing(_ context.Context, id uuid.UUID, _ legalacceptance.MarketingInput) (legalacceptance.Status, error) {
	s.actor = id
	s.calls++
	return legalacceptance.Status{}, nil
}
func TestLegalChoicesRequireCookieOriginCSRFAndUseServerAccount(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	svc := &legalStub{}
	h.deps.Legal = svc
	account := uuid.New()
	tokens, err := sessions.IssueSession(context.Background(), account, accountauth.SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		auth, csrf bool
		origin     string
		want       int
	}{
		{"anonymous", false, true, "https://app.example.test", 401},
		{"missing csrf", true, false, "https://app.example.test", 403},
		{"foreign origin", true, true, "https://other.example.test", 403},
		{"valid", true, true, "https://app.example.test", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/web/v1/account/legal/accept", strings.NewReader(`{"accepted":true,"version":"v1","checksum":"hash","locale":"ru"}`))
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("X-Idempotency-Key", uuid.NewString())
			req.Header.Set("X-Account-ID", uuid.NewString())
			if tc.auth {
				req.AddCookie(&http.Cookie{Name: "nh_access", Value: tokens.AccessToken})
			}
			if tc.csrf {
				req.AddCookie(&http.Cookie{Name: "nh_csrf", Value: "proof"})
				req.Header.Set("X-CSRF-Token", "proof")
			}
			before := svc.calls
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status %d want %d", rec.Code, tc.want)
			}
			if tc.want != 200 && svc.calls != before {
				t.Fatal("choice recorded before authentication/CSRF")
			}
			if tc.want == 200 && (svc.actor != account || !svc.input.Accepted || svc.input.MarketingOptIn) {
				t.Fatal("untrusted owner or automatic marketing")
			}
		})
	}
	req := httptest.NewRequest("GET", "/web/v1/account/legal", nil)
	req.AddCookie(&http.Cookie{Name: "nh_access", Value: tokens.AccessToken})
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("private legal state cached or inaccessible")
	}
}

func TestLegalAcceptanceGuardsGenerationButNotSecurityOrWithdrawal(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	svc := &legalStub{status: legalacceptance.Status{Metadata: legalacceptance.Metadata{Published: true}}}
	h.deps.Legal = svc
	tokens, err := sessions.IssueSession(context.Background(), uuid.New(), accountauth.SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path                string
		accepted, published bool
		want                int
	}{
		{"/web/v1/image-jobs/prepare", false, true, 428},
		{"/web/v1/payments/intents", false, true, 428},
		{"/web/v1/conversations", false, true, 428},
		{"/web/v1/account/legal/marketing", false, true, 204},
		{"/web/v1/account/password/set", false, true, 204},
		{"/web/v1/image-jobs/prepare", true, true, 204},
		{"/web/v1/image-jobs/prepare", false, false, 204},
	} {
		svc.status.Accepted, svc.status.Published = tc.accepted, tc.published
		req := httptest.NewRequest("POST", tc.path, nil)
		req.Header.Set("Origin", "https://app.example.test")
		req.AddCookie(&http.Cookie{Name: "nh_access", Value: tokens.AccessToken})
		req.AddCookie(&http.Cookie{Name: "nh_csrf", Value: "proof"})
		req.Header.Set("X-CSRF-Token", "proof")
		rec := httptest.NewRecorder()
		h.requireUnsafePrincipal(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("%s: got %d want %d", tc.path, rec.Code, tc.want)
		}
	}
}
