package websession

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
)

type unavailableAuth struct {
	err error
	*accountauth.Service
}

func (a unavailableAuth) AuthenticateAccessToken(context.Context, string) (domain.RequestPrincipal, error) {
	return domain.RequestPrincipal{}, a.err
}
func (a unavailableAuth) RefreshSession(context.Context, string, accountauth.SessionMetadata) (accountauth.SessionTokens, error) {
	return accountauth.SessionTokens{}, a.err
}

func TestSessionFailuresPreserveAuthenticationMeaning(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"expired", accountauth.ErrSessionExpired, 401}, {"revoked", accountauth.ErrInvalidSession, 401},
		{"repository failure", errors.New("private database details"), 503},
	} {
		for _, path := range []string{"/web/v1/conversations", "/web/v1/auth/refresh"} {
			t.Run(tc.name+path, func(t *testing.T) {
				auth := unavailableAuth{err: tc.err}
				handler := NewHandler(Config{WebOrigin: "https://app.example.test"}, Deps{Authenticator: auth, Sessions: auth})
				method := http.MethodGet
				if path == "/web/v1/auth/refresh" {
					method = http.MethodPost
				}
				req := httptest.NewRequest(method, path, nil)
				req.AddCookie(&http.Cookie{Name: "nh_access", Value: "synthetic"})
				req.AddCookie(&http.Cookie{Name: "nh_refresh", Value: "synthetic"})
				req.AddCookie(&http.Cookie{Name: "nh_csrf", Value: "synthetic"})
				req.Header.Set("Origin", "https://app.example.test")
				req.Header.Set("X-CSRF-Token", "synthetic")
				rec := httptest.NewRecorder()
				handler.Routes().ServeHTTP(rec, req)
				if rec.Code != tc.status {
					t.Fatalf("status = %d, want %d", rec.Code, tc.status)
				}
			})
		}
	}
}
