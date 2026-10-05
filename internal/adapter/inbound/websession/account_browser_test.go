package websession

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
)

func TestBrowserRecoveryRejectsCrossOrigin(t *testing.T) {
	h, _, _ := newTestHandler(t)
	for _, path := range []string{"/web/v1/auth/password/request-reset", "/web/v1/auth/password/reset"} {
		req := httptest.NewRequest("POST", path, strings.NewReader(`{"email":"member@example.test"}`))
		req.Header.Set("Origin", "https://evil.example.test")
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s status %d, want 403", path, rec.Code)
		}
	}
}

func TestBrowserAccountWritesRequireCSRF(t *testing.T) {
	h, _, _ := newTestHandler(t)
	for _, path := range []string{"/web/v1/account/identities/email/request-code", "/web/v1/account/identities/email/backup/request-code", "/web/v1/account/identities/email/backup/verify", "/web/v1/account/password/set", "/web/v1/account/sessions/00000000-0000-4000-8000-000000000001/revoke"} {
		req := httptest.NewRequest("POST", path, strings.NewReader(`{}`))
		req.Header.Set("Origin", "https://app.example.test")
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s status %d, want 403", path, rec.Code)
		}
	}
}

func TestBrowserMethodsFailClosedWithoutConfiguration(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, httptest.NewRequest("GET", "/web/v1/auth/methods", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"recovery":false`) || strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("unsafe methods: %d %s", rec.Code, rec.Body.String())
	}
}

func TestBackupEmailBrowserRoutesRequireCookieOriginCSRFAndDelivery(t *testing.T) {
	for _, path := range []string{"/web/v1/account/identities/email/backup/request-code", "/web/v1/account/identities/email/backup/verify"} {
		t.Run(path, func(t *testing.T) {
			h, _, sessions := newTestHandler(t)
			spy := &browserActionsSpy{}
			h.deps.AccountActions = spy
			accountID := uuid.New()
			tokens, _ := sessions.IssueSession(context.Background(), accountID, accountauth.SessionMetadata{})
			request := func(origin string, cookie, csrf bool) *httptest.ResponseRecorder {
				req := httptest.NewRequest("POST", path, strings.NewReader(`{}`))
				req.Header.Set("Origin", origin)
				if cookie {
					req.AddCookie(&http.Cookie{Name: accessCookieName, Value: tokens.AccessToken})
				}
				if csrf {
					req.Header.Set("X-CSRF-Token", "csrf")
					req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
				}
				req.Header.Set("X-Account-ID", uuid.NewString())
				rec := httptest.NewRecorder()
				h.Routes().ServeHTTP(rec, req)
				return rec
			}
			if request("https://evil.example.test", true, true).Code != 403 {
				t.Fatal("cross origin accepted")
			}
			if request("https://app.example.test", true, false).Code != 403 {
				t.Fatal("missing CSRF accepted")
			}
			if request("https://app.example.test", false, true).Code != 401 {
				t.Fatal("missing cookie accepted")
			}
			if request("https://app.example.test", true, true).Code != 503 || spy.actor != uuid.Nil {
				t.Fatal("disabled delivery accepted")
			}
			h.cfg.EmailDeliveryEnabled = true
			if request("https://app.example.test", true, true).Code != 204 || spy.actor != accountID {
				t.Fatal("cookie owner not forwarded")
			}
		})
	}
}

type browserActionsSpy struct {
	actor      uuid.UUID
	recoveries int
}

func (s *browserActionsSpy) ServePasswordRecovery(w http.ResponseWriter, r *http.Request) {
	s.recoveries++
	w.WriteHeader(202)
}
func (s *browserActionsSpy) ServeBrowserAccountAction(w http.ResponseWriter, r *http.Request, p domain.RequestPrincipal) {
	s.actor = p.AccountID
	w.WriteHeader(204)
}

func TestBrowserAccountActionsUseCookieAccountAndRecoveryDoesNotRequireLogin(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	spy := &browserActionsSpy{}
	h.deps.AccountActions = spy
	h.cfg.EmailDeliveryEnabled = true
	accountID := uuid.New()
	tokens, _ := sessions.IssueSession(context.Background(), accountID, accountauth.SessionMetadata{})
	req := httptest.NewRequest("POST", "/web/v1/account/identities/email/request-code", strings.NewReader(`{"email":"member@example.test"}`))
	req.Header.Set("Origin", "https://app.example.test")
	req.Header.Set("X-Account-ID", uuid.NewString())
	req.Header.Set("X-CSRF-Token", "csrf")
	req.AddCookie(&http.Cookie{Name: accessCookieName, Value: tokens.AccessToken})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != 204 || spy.actor != accountID {
		t.Fatal("client account trusted or cookie principal lost")
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/web/v1/auth/password/request-reset", strings.NewReader(`{"email":"member@example.test"}`))
	req.Header.Set("Origin", "https://app.example.test")
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != 202 || spy.recoveries != 1 {
		t.Fatal("recovery depends on expired session")
	}
}

func TestBrowserSessionListAndRevokeEnforceOwnership(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	accountID := uuid.New()
	own, _ := sessions.IssueSession(context.Background(), accountID, accountauth.SessionMetadata{})
	other, _ := sessions.IssueSession(context.Background(), uuid.New(), accountauth.SessionMetadata{})
	req := httptest.NewRequest("GET", "/web/v1/account/sessions", nil)
	req.AddCookie(&http.Cookie{Name: accessCookieName, Value: own.AccessToken})
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	var data struct {
		Items []struct {
			ID      uuid.UUID `json:"id"`
			Current bool      `json:"current"`
		}
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &data) != nil || len(data.Items) != 1 || data.Items[0].ID != own.Session.ID || !data.Items[0].Current {
		t.Fatal("session list unsafe")
	}
	for _, test := range []struct {
		id       uuid.UUID
		expected int
	}{{other.Session.ID, 404}, {own.Session.ID, 204}} {
		req = httptest.NewRequest("POST", "/web/v1/account/sessions/"+test.id.String()+"/revoke", nil)
		req.Header.Set("Origin", "https://app.example.test")
		req.Header.Set("X-CSRF-Token", "csrf")
		req.AddCookie(&http.Cookie{Name: accessCookieName, Value: own.AccessToken})
		req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
		rec = httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)
		if rec.Code != test.expected {
			t.Fatalf("revoke status %d, want %d", rec.Code, test.expected)
		}
		if test.id == own.Session.ID && len(rec.Result().Cookies()) != 3 {
			t.Fatal("current revoked session cookies retained")
		}
	}
	if _, err := sessions.AuthenticateAccessToken(context.Background(), other.AccessToken); err != nil {
		t.Fatal("other account revoked")
	}
}
