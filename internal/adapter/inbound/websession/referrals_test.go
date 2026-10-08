package websession

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/webreferralservice"
)

func TestReferralVisitFailsClosedWithoutService(t *testing.T) {
	h, _, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/web/v1/referrals/visit", strings.NewReader(`{"code":"INVITE1234"}`))
	req.Header.Set("Origin", "https://app.example.test")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want503", rec.Code)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("invitation responses must not be cached")
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("failed capture created a cookie")
	}
}

type referralStub struct {
	code, hash string
	accountID  uuid.UUID
	acceptErr  error
	calls      int
}

func (s *referralStub) Summary(_ context.Context, id uuid.UUID) (webreferralservice.Summary, error) {
	s.accountID = id
	return webreferralservice.Summary{Code: "INVITE1234", Visits: 7, Registered: 2}, nil
}
func (s *referralStub) Capture(_ context.Context, code, hash string, _ time.Time) error {
	s.code, s.hash = code, hash
	s.calls++
	return nil
}
func (s *referralStub) Accept(_ context.Context, hash string, id uuid.UUID, _ time.Time) error {
	s.hash, s.accountID = hash, id
	s.calls++
	return s.acceptErr
}

type referralLimiter struct{ allowed bool }

func (l referralLimiter) Allow(context.Context, string) (bool, error) { return l.allowed, nil }

type referralClientBudget struct{ seen map[string]int }

func (l *referralClientBudget) Allow(_ context.Context, key string) (bool, error) {
	l.seen[key]++
	return l.seen[key] <= 2, nil
}

func TestReferralClientLimitDoesNotStarveAnotherBrowser(t *testing.T) {
	h, _, _ := newTestHandler(t)
	svc := &referralStub{}
	clients := &referralClientBudget{seen: map[string]int{}}
	h.deps.Referrals = svc
	h.deps.ReferralVisitLimiter = referralLimiter{true}
	h.deps.ReferralVisitClientLimiter = clients
	for i, token := range []string{strings.Repeat("A", 43), strings.Repeat("A", 43), strings.Repeat("A", 43), strings.Repeat("B", 43)} {
		req := httptest.NewRequest("POST", "/web/v1/referrals/visit", strings.NewReader(`{"code":"INVITE1234"}`))
		req.Header.Set("Origin", "https://app.example.test")
		req.AddCookie(&http.Cookie{Name: referralCookieName, Value: token})
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)
		want := 204
		if i == 2 {
			want = 429
		}
		if rec.Code != want {
			t.Fatalf("browser request %d: status=%d, want=%d", i, rec.Code, want)
		}
	}
	if svc.calls != 3 || len(clients.seen) != 2 {
		t.Fatal("per-browser budget did not isolate clients")
	}
	for key := range clients.seen {
		if strings.Contains(key, strings.Repeat("A", 43)) || strings.Contains(key, strings.Repeat("B", 43)) {
			t.Fatal("raw token used as limiter key")
		}
	}
}

func TestReferralCaptureStoresOnlyHashAndSecureFirstTouchCookie(t *testing.T) {
	h, _, _ := newTestHandler(t)
	svc := &referralStub{}
	h.deps.Referrals, h.deps.ReferralVisitLimiter = svc, referralLimiter{true}
	h.deps.ReferralVisitClientLimiter = referralLimiter{true}
	req := httptest.NewRequest("POST", "/web/v1/referrals/visit", strings.NewReader(`{"code":"INVITE1234"}`))
	req.Header.Set("Origin", "https://app.example.test")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != 204 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	cookie := cookieMap(rec.Result().Cookies())[referralCookieName]
	if cookie == nil || !cookie.Secure || !cookie.HttpOnly || cookie.Domain != "" || cookie.Path != "/" || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("unsafe invitation cookie")
	}
	if svc.hash != referralTokenHash(cookie.Value) || len(svc.hash) != 64 || strings.Contains(svc.hash, cookie.Value) {
		t.Fatal("storage did not receive only token hash")
	}
	repeated := httptest.NewRequest("POST", "/web/v1/referrals/visit", strings.NewReader(`{"code":"OTHER12345"}`))
	repeated.Header.Set("Origin", "https://app.example.test")
	repeated.AddCookie(cookie)
	repeatedRec := httptest.NewRecorder()
	h.Routes().ServeHTTP(repeatedRec, repeated)
	if repeatedRec.Code != 204 || len(repeatedRec.Result().Cookies()) != 0 || svc.hash != referralTokenHash(cookie.Value) {
		t.Fatal("repeat extended or replaced the first-touch binding")
	}
}

func TestReferralCaptureRateLimitPrecedesDurableWrite(t *testing.T) {
	h, _, _ := newTestHandler(t)
	svc := &referralStub{}
	h.deps.Referrals, h.deps.ReferralVisitLimiter = svc, referralLimiter{false}
	h.deps.ReferralVisitClientLimiter = referralLimiter{true}
	req := httptest.NewRequest("POST", "/web/v1/referrals/visit", strings.NewReader(`{"code":"INVITE1234"}`))
	req.Header.Set("Origin", "https://app.example.test")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != 429 || svc.calls != 0 || len(rec.Result().Cookies()) != 0 {
		t.Fatal("rate limit failed before storage")
	}
}

func TestReferralAcceptanceUsesCookiePrincipalAndRetriesOutage(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		status  int
		expires bool
	}{
		{"accepted", nil, 204, true}, {"old account", domain.ErrForbidden, 204, true}, {"expired", domain.ErrExpired, 204, true}, {"unavailable", errors.New("storage outage"), 503, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, sessions := newTestHandler(t)
			svc := &referralStub{acceptErr: tc.err}
			h.deps.Referrals = svc
			id := uuid.New()
			tokens, err := sessions.IssueSession(context.Background(), id, accountauth.SessionMetadata{})
			if err != nil {
				t.Fatal(err)
			}
			cookieRec := httptest.NewRecorder()
			if err := h.setSessionCookies(cookieRec, tokens); err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest("POST", "/web/v1/referrals/accept?account_id="+uuid.NewString(), nil)
			req.Header.Set("Origin", "https://app.example.test")
			req.Header.Set("X-Account-ID", uuid.NewString())
			for _, cookie := range cookieRec.Result().Cookies() {
				req.AddCookie(cookie)
				if cookie.Name == csrfCookieName {
					req.Header.Set("X-CSRF-Token", cookie.Value)
				}
			}
			token := strings.Repeat("A", 43)
			req.AddCookie(&http.Cookie{Name: referralCookieName, Value: token})
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)
			if rec.Code != tc.status || svc.accountID != id || svc.hash != referralTokenHash(token) {
				t.Fatalf("status=%d ownership/hash failed", rec.Code)
			}
			cookie := cookieMap(rec.Result().Cookies())[referralCookieName]
			if tc.expires && (cookie == nil || cookie.MaxAge != -1) {
				t.Fatal("terminal intent not cleared")
			}
			if !tc.expires && cookie != nil {
				t.Fatal("outage destroyed retry intent")
			}
		})
	}
}

func TestReferralVisitRejectsCrossOriginBeforeService(t *testing.T) {
	h, _, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/web/v1/referrals/visit", strings.NewReader(`{"code":"INVITE1234"}`))
	req.Header.Set("Origin", "https://other.example.test")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want403", rec.Code)
	}
}

func TestReferralSummaryRequiresCookieSession(t *testing.T) {
	h, _, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/web/v1/referrals", nil)
	req.Header.Set("X-Account-ID", "forged")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want401", rec.Code)
	}
}

func TestReferralAcceptanceRequiresCSRF(t *testing.T) {
	h, _, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/web/v1/referrals/accept", nil)
	req.Header.Set("Origin", "https://app.example.test")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want403", rec.Code)
	}
}
