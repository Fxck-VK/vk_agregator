package websession

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
)

type failingPasswordLogin struct{ err error }

type legacyOnlyPasswordLogin struct{ PasswordService }

func TestBrowserPasswordLoginFailsClosedWithoutCombinedOperation(t *testing.T) {
	h, passwords, _ := newTestHandler(t)
	h.deps.Passwords = legacyOnlyPasswordLogin{PasswordService: passwords}
	r := httptest.NewRequest(http.MethodPost, "/web/v1/auth/password/login", strings.NewReader(`{"email":"member@example.test","password":"synthetic-password"}`))
	r.Header.Set("Origin", "https://app.example.test")
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable || passwords.calls != 0 || len(w.Result().Cookies()) != 0 {
		t.Fatal("browser login used split password/session fallback")
	}
}

func (s failingPasswordLogin) AuthenticateEmailPasswordSession(context.Context, string, string, accountauth.SessionMetadata) (accountauth.SessionTokens, error) {
	return accountauth.SessionTokens{}, s.err
}

func (s failingPasswordLogin) AuthenticateEmailPassword(context.Context, string, string) (domain.IdentityResolution, error) {
	return domain.IdentityResolution{}, s.err
}

func TestPasswordLoginLogsOnlySafeFailureMetadata(t *testing.T) {
	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	h, _, _ := newTestHandler(t)
	h.deps.Passwords = failingPasswordLogin{err: errors.New("synthetic-sensitive-internal-detail")}
	r := httptest.NewRequest(http.MethodPost, "/web/v1/auth/password/login", strings.NewReader(`{"email":"member@example.test","password":"synthetic-password"}`))
	r.Header.Set("Origin", "https://app.example.test")
	r.Header.Set("X-Request-ID", "40000000-0000-4000-8000-000000000001")
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, r)
	for _, required := range []string{"web_request_failed", "password_authentication", "40000000-0000-4000-8000-000000000001", `"status":503`, "duration_ms"} {
		if !strings.Contains(logged.String(), required) {
			t.Fatalf("missing safe diagnostic field %q", required)
		}
	}
	for _, forbidden := range []string{"member@example.test", "synthetic-password", "synthetic-sensitive-internal-detail"} {
		if strings.Contains(logged.String(), forbidden) {
			t.Fatal("login diagnostic exposed sensitive request or internal data")
		}
	}
}

func TestPasswordLoginClassifiesFailures(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status int
	}{
		{"invalid credentials", accountauth.ErrInvalidPasswordLogin, http.StatusUnauthorized},
		{"rate limit", accountauth.ErrRateLimited, http.StatusTooManyRequests},
		{"credential store unavailable", accountauth.ErrPasswordStoreUnavailable, http.StatusServiceUnavailable},
		{"unexpected storage failure", errors.New("synthetic-sensitive-internal-detail"), http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			h, _, _ := newTestHandler(t)
			h.deps.Passwords = failingPasswordLogin{err: test.err}
			r := httptest.NewRequest(http.MethodPost, "/web/v1/auth/password/login", strings.NewReader(`{"email":"member@example.test","password":"synthetic-password"}`))
			r.Header.Set("Origin", "https://app.example.test")
			r.Header.Set("X-Request-ID", "40000000-0000-4000-8000-000000000001")
			w := httptest.NewRecorder()
			h.Routes().ServeHTTP(w, r)
			if w.Code != test.status {
				t.Fatalf("status = %d, want %d", w.Code, test.status)
			}
			if w.Header().Get("X-Request-ID") != r.Header.Get("X-Request-ID") {
				t.Fatal("login correlation id was lost")
			}
			for _, forbidden := range []string{"member@example.test", "synthetic-password", "synthetic-sensitive-internal-detail"} {
				if strings.Contains(w.Body.String(), forbidden) {
					t.Fatal("login error exposed sensitive request or internal data")
				}
			}
		})
	}
}
