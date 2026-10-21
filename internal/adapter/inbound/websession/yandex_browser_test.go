package websession

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/accountoauth"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/accountservice"
	"vk-ai-aggregator/internal/service/identityresolver"
)

type yandexBrowserStore struct {
	data map[string]accountoauth.BrowserTransaction
}

func (s *yandexBrowserStore) Save(_ context.Context, key string, tx accountoauth.BrowserTransaction, _ time.Duration) error {
	s.data[key] = tx
	return nil
}
func (s *yandexBrowserStore) Take(_ context.Context, key, binding string) (accountoauth.BrowserTransaction, error) {
	tx, ok := s.data[key]
	if !ok || tx.BindingHash != binding {
		return accountoauth.BrowserTransaction{}, accountoauth.ErrInvalidAssertion
	}
	delete(s.data, key)
	return tx, nil
}

type yandexHTTPTransport func(*http.Request) (*http.Response, error)

func (f yandexHTTPTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestYandexBrowserRoutesIssueCookiesAndBindLinkToCurrentAccount(t *testing.T) {
	ctx := context.Background()
	h, _, sessions := newTestHandler(t)
	repo := memory.NewAccountIdentityRepo()
	auth := accountauth.New(identityresolver.New(memory.NewUserRepo(), repo, nil), accountauth.WithProfileRepository(repo))
	h.deps.OAuthLogins = auth
	h.deps.Account = accountservice.New(repo, auth, accountservice.WithProfileRepository(repo))
	calls := 0
	h.deps.BrowserOAuth = accountoauth.NewBrowser(accountoauth.BrowserConfig{WebOrigin: "https://app.example.test", YandexClientID: "client", YandexClientSecret: "synthetic-secret"}, &yandexBrowserStore{data: map[string]accountoauth.BrowserTransaction{}}, accountoauth.NewRegistry(), &http.Client{Transport: yandexHTTPTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		body := `{"access_token":"synthetic-access"}`
		if r.URL.Host == "login.yandex.ru" {
			body = `{"id":"Stable-Subject","client_id":"client","first_name":"Тестовый","last_name":"Пользователь"}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})})
	start := func(link bool, access *http.Cookie) (url.Values, *http.Cookie) {
		t.Helper()
		path := "/web/v1/auth/oauth/yandex/start"
		if link {
			path = "/web/v1/account/oauth/yandex/start"
		}
		req := httptest.NewRequest("POST", path, strings.NewReader(`{"locale":"en"}`))
		req.Header.Set("Origin", "https://app.example.test")
		if access != nil {
			req.AddCookie(access)
			req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
			req.Header.Set("X-CSRF-Token", "csrf")
		}
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatal("OAuth start failed", rec.Code)
		}
		var value struct {
			URL string `json:"authorization_url"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &value); err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(value.URL)
		if err != nil || u.Host != "oauth.yandex.ru" {
			t.Fatal("invalid provider URL")
		}
		cookie := cookieMap(rec.Result().Cookies())[oauthBindingCookie]
		if cookie == nil || !cookie.Secure || !cookie.HttpOnly || cookie.Domain != "" {
			t.Fatal("unsafe browser binding")
		}
		return url.Values{"state": {u.Query().Get("state")}, "code": {"synthetic-code"}}, cookie
	}
	callback := func(q url.Values, binding, access *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/web/v1/auth/oauth/yandex/callback?"+q.Encode(), nil)
		if binding != nil {
			req.AddCookie(binding)
		}
		if access != nil {
			req.AddCookie(access)
		}
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)
		return rec
	}
	q, binding := start(false, nil)
	if rec := callback(q, nil, nil); rec.Header().Get("Location") != "/ru/login?oauth=failed" || calls != 0 {
		t.Fatal("unbound browser accepted")
	}
	rec := callback(q, binding, nil)
	if rec.Code != 303 || rec.Header().Get("Location") != "/en/app" || calls != 2 {
		t.Fatal("login callback failed")
	}
	cookies := cookieMap(rec.Result().Cookies())
	for _, key := range []string{accessCookieName, refreshCookieName} {
		if c := cookies[key]; c == nil || !c.HttpOnly || !c.Secure || c.Domain != "" || c.SameSite != http.SameSiteLaxMode {
			t.Fatal("unsafe session cookie")
		}
	}
	principal, err := sessions.AuthenticateAccessToken(ctx, cookies[accessCookieName].Value)
	if err != nil || principal.AccountID == uuid.Nil {
		t.Fatal("issued session invalid")
	}
	meRequest := httptest.NewRequest("GET", "/web/v1/me", nil)
	meRequest.AddCookie(cookies[accessCookieName])
	meResponse := httptest.NewRecorder()
	h.Routes().ServeHTTP(meResponse, meRequest)
	var me struct {
		DisplayName string `json:"display_name"`
	}
	if err := json.Unmarshal(meResponse.Body.Bytes(), &me); err != nil || meResponse.Code != 200 || me.DisplayName != "Тестовый Пользователь" {
		t.Fatal("authenticated profile did not expose the Yandex name", err)
	}
	if repeat := callback(q, binding, nil); repeat.Header().Get("Location") != "/ru/login?oauth=failed" || calls != 2 {
		t.Fatal("callback replay accepted")
	}
	q, binding = start(true, cookies[accessCookieName])
	foreign, err := sessions.IssueSession(ctx, uuid.New(), accountauth.SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	if rec := callback(q, binding, &http.Cookie{Name: accessCookieName, Value: foreign.AccessToken}); rec.Header().Get("Location") != "/en/app/profile?oauth=failed" {
		t.Fatal("link accepted a different logged-in account")
	}
	q, binding = start(true, cookies[accessCookieName])
	if rec := callback(q, binding, cookies[accessCookieName]); rec.Header().Get("Location") != "/en/app/profile?oauth=linked" {
		t.Fatal("owned link failed")
	}
	profile, err := h.deps.Account.Profile(ctx, principal.AccountID)
	if err != nil || len(profile.IdentityRefs) != 1 || profile.IdentityRefs[0].Provider != domain.IdentityProviderYandex {
		t.Fatal("binding changed account ownership", err)
	}
}
