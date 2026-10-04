package websession

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/accountregistration"
)

func TestEmailRegistrationPublicWritesRejectForeignOrigin(t *testing.T) {
	h, _, _ := newTestHandler(t)
	for _, action := range []string{"request-code", "verify-code", "register"} {
		req := httptest.NewRequest("POST", "/web/v1/auth/email/"+action, strings.NewReader(`{"email":"member@example.test"}`))
		req.Header.Set("Origin", "https://evil.example.test")
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s: got %d, want 403", action, rec.Code)
		}
	}
}

type signupServiceSpy struct {
	sessions  *sessionStub
	accountID uuid.UUID
	binding   string
	fail      error
}

func (s *signupServiceSpy) Request(context.Context, string, string) (string, error) {
	return strings.Repeat("b", 43), s.fail
}
func (s *signupServiceSpy) Verify(_ context.Context, binding, email, code string) error {
	s.binding = binding
	if s.fail != nil {
		return s.fail
	}
	if len(binding) != 43 {
		return accountregistration.ErrInvalidProof
	}
	return nil
}
func (s *signupServiceSpy) Complete(ctx context.Context, binding, email, password string, meta accountauth.SessionMetadata, finalize func(accountauth.SessionTokens) error) (accountauth.SessionTokens, error) {
	s.binding = binding
	if s.fail != nil {
		return accountauth.SessionTokens{}, s.fail
	}
	tokens, err := s.sessions.IssueSession(ctx, s.accountID, meta)
	if err == nil {
		err = finalize(tokens)
	}
	return tokens, err
}

func TestEmailRegistrationUsesHttpOnlyProofAndExistingSessionCookies(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	spy := &signupServiceSpy{sessions: sessions, accountID: uuid.New()}
	h.deps.EmailRegistration = spy
	h.cfg.EmailDeliveryEnabled = true
	send := func(path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/web/v1/auth/email/"+path, strings.NewReader(body))
		req.Header.Set("Origin", "https://app.example.test")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)
		return rec
	}
	rec := send("request-code", `{"email":"new@example.test"}`, nil)
	if rec.Code != 202 || rec.Header().Get("Cache-Control") != "no-store" || strings.Contains(rec.Body.String(), "new@example.test") {
		t.Fatal("unsafe accepted response")
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("code request must not issue account session")
	}
	proof := cookies[0]
	if proof.Name != registrationCookieName || !proof.HttpOnly || !proof.Secure || proof.Path != "/" || proof.Domain != "" || proof.MaxAge > 600 {
		t.Fatal("unsafe registration cookie")
	}
	rec = send("verify-code", `{"email":"new@example.test","code":"123456"}`, nil)
	if rec.Code != 400 {
		t.Fatal("missing cookie accepted")
	}
	rec = send("verify-code", `{"email":"new@example.test","code":"123456"}`, proof)
	if rec.Code != 204 || spy.binding != proof.Value {
		t.Fatal("proof cookie not forwarded")
	}
	rec = send("register", `{"email":"new@example.test","password":"strong-password","account_id":"00000000-0000-4000-8000-000000000001"}`, proof)
	if rec.Code != 400 {
		t.Fatal("client account ID accepted")
	}
	rec = send("register", `{"email":"new@example.test","password":"strong-password"}`, proof)
	if rec.Code != 201 || strings.Contains(rec.Body.String(), "strong-password") || strings.Contains(rec.Body.String(), "access_token") {
		t.Fatal("unsafe signup response")
	}
	var access *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == accessCookieName {
			access = cookie
		}
		if cookie.Name == registrationCookieName && cookie.MaxAge != -1 {
			t.Fatal("proof cookie not expired")
		}
	}
	if access == nil || !access.HttpOnly || !access.Secure {
		t.Fatal("session not issued")
	}
	principal, err := sessions.AuthenticateAccessToken(context.Background(), access.Value)
	if err != nil || principal.AccountID != spy.accountID {
		t.Fatal("signup did not use shared account session")
	}
}

func TestEmailRegistrationErrorsAreClassifiedWithoutLeakingDetails(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	spy := &signupServiceSpy{sessions: sessions}
	h.deps.EmailRegistration = spy
	h.cfg.EmailDeliveryEnabled = true
	for _, item := range []struct {
		err    error
		status int
	}{{accountregistration.ErrInvalidProof, 400}, {accountregistration.ErrRateLimited, 429}, {domain.ErrConflict, 409}, {errors.New("private SMTP detail"), 503}} {
		spy.fail = item.err
		req := httptest.NewRequest("POST", "/web/v1/auth/email/verify-code", strings.NewReader(`{"email":"new@example.test","code":"123456"}`))
		req.Header.Set("Origin", "https://app.example.test")
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)
		if rec.Code != item.status || strings.Contains(rec.Body.String(), "private SMTP") {
			t.Fatalf("unsafe status: %d", rec.Code)
		}
	}
}

func TestEmailRegistrationUnavailableWithoutDeliveryConfiguration(t *testing.T) {
	h, _, _ := newTestHandler(t)
	req := httptest.NewRequest("POST", "/web/v1/auth/email/request-code", strings.NewReader(`{"email":"member@example.test"}`))
	req.Header.Set("Origin", "https://app.example.test")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, httptest.NewRequest("GET", "/web/v1/auth/methods", nil))
	if !strings.Contains(rec.Body.String(), `"registration":false`) {
		t.Fatal("signup capability must fail closed")
	}
}
