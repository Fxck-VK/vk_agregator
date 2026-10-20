package accountoauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func yandexTestBrowser(transport http.RoundTripper) *Browser {
	return NewBrowser(BrowserConfig{WebOrigin: "https://app.example.test", YandexClientID: "client", YandexClientSecret: "synthetic-secret"}, &browserMemoryStore{data: map[string]BrowserTransaction{}}, NewRegistry(), &http.Client{Transport: transport})
}

func TestYandexBrowserAvailabilityRequiresServerCredentialsAndHTTPS(t *testing.T) {
	for _, cfg := range []BrowserConfig{
		{WebOrigin: "https://app.example.test", YandexClientID: "client"},
		{WebOrigin: "https://app.example.test", YandexClientSecret: "secret"},
		{WebOrigin: "http://app.example.test", YandexClientID: "client", YandexClientSecret: "secret"},
	} {
		b := NewBrowser(cfg, &browserMemoryStore{data: map[string]BrowserTransaction{}}, NewRegistry(), nil)
		if _, err := b.Start(context.Background(), "yandex", "browser", "ru", "login", ""); !errors.Is(err, ErrUnavailable) {
			t.Fatal("unconfigured Yandex enabled")
		}
		for _, name := range b.Providers() {
			if name == "yandex" {
				t.Fatal("unavailable Yandex advertised")
			}
		}
	}
	if names := yandexTestBrowser(nil).Providers(); len(names) != 1 || names[0] != "yandex" {
		t.Fatal("configured Yandex not advertised")
	}
}

func TestYandexBrowserLoginAndLinkAreBoundAndSingleUse(t *testing.T) {
	for _, intent := range []string{"login", "link"} {
		t.Run(intent, func(t *testing.T) {
			var challenge string
			calls := 0
			b := yandexTestBrowser(browserRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				body := `{"access_token":"synthetic-access"}`
				switch calls {
				case 1:
					if r.Method != "POST" || r.URL.String() != "https://oauth.yandex.ru/token" {
						t.Fatal("unexpected token endpoint")
					}
					if err := r.ParseForm(); err != nil {
						t.Fatal(err)
					}
					f := r.PostForm
					sum := sha256.Sum256([]byte(f.Get("code_verifier")))
					if f.Get("grant_type") != "authorization_code" || f.Get("code") != "synthetic-code" || f.Get("client_id") != "client" || f.Get("client_secret") != "synthetic-secret" || f.Get("redirect_uri") != "https://app.example.test/web/v1/auth/oauth/yandex/callback" || base64.RawURLEncoding.EncodeToString(sum[:]) != challenge {
						t.Fatal("invalid confidential PKCE exchange")
					}
				case 2:
					if r.Method != "GET" || r.URL.String() != "https://login.yandex.ru/info?format=json" || r.Header.Get("Authorization") != "OAuth synthetic-access" {
						t.Fatal("unsafe profile request")
					}
					body = `{"id":"Yandex-Subject","client_id":"client","default_email":"ignored@example.test"}`
				default:
					t.Fatal("extra provider request")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			}))
			ctx := context.Background()
			start, err := b.Start(ctx, "yandex", "browser", "ru", intent, "account")
			if err != nil {
				t.Fatal(err)
			}
			u, _ := url.Parse(start.URL)
			q := u.Query()
			challenge = q.Get("code_challenge")
			if u.Scheme != "https" || u.Host != "oauth.yandex.ru" || u.Path != "/authorize" || q.Get("scope") != "login:info" || q.Get("force_confirm") != "yes" || q.Get("code_challenge_method") != "S256" || challenge == "" || q.Get("nonce") != "" || q.Get("client_secret") != "" {
				t.Fatal("unsafe authorize request")
			}
			callback := url.Values{"state": {start.State}, "code": {"synthetic-code"}}
			if _, _, err := b.Complete(ctx, "yandex", "foreign-browser", callback); !errors.Is(err, ErrInvalidAssertion) || calls != 0 {
				t.Fatal("wrong browser reached provider")
			}
			tx, login, err := b.Complete(ctx, "yandex", "browser", callback)
			if err != nil || !login.Verified || login.Method != domain.AccountLoginMethod("yandex") || login.ExternalID != "Yandex-Subject" || tx.Intent != intent || tx.AccountID != "account" || calls != 2 {
				t.Fatal("verified subject not resolved", err)
			}
			if _, _, err := b.Complete(ctx, "yandex", "browser", callback); !errors.Is(err, ErrInvalidAssertion) || calls != 2 {
				t.Fatal("replay accepted")
			}
		})
	}
}

func TestYandexBrowserRejectsInvalidProfileAndToken(t *testing.T) {
	for _, tc := range []struct {
		name, token, profile string
		status               int
	}{
		{"missing token", `{}`, `{}`, 200},
		{"token error", `{"access_token":"synthetic","error":"denied"}`, `{}`, 200},
		{"invalid header", `{"access_token":"bad\r\nheader"}`, `{}`, 200},
		{"missing subject", `{"access_token":"synthetic"}`, `{"client_id":"client"}`, 200},
		{"wrong client", `{"access_token":"synthetic"}`, `{"id":"subject","client_id":"another-app"}`, 200},
		{"missing client", `{"access_token":"synthetic"}`, `{"id":"subject"}`, 200},
		{"whitespace subject", `{"access_token":"synthetic"}`, `{"id":"bad subject","client_id":"client"}`, 200},
		{"profile error", `{"access_token":"synthetic"}`, `{"id":"subject","client_id":"client","error":"denied"}`, 200},
		{"malformed profile", `{"access_token":"synthetic"}`, `not-json`, 200},
		{"redirect", `{"access_token":"synthetic"}`, `{}`, 302},
		{"unauthorized", `{"access_token":"synthetic"}`, `{}`, 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := yandexTestBrowser(browserRoundTrip(func(r *http.Request) (*http.Response, error) {
				body, status := tc.token, 200
				if r.URL.Host == "login.yandex.ru" {
					body, status = tc.profile, tc.status
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Location": {"https://untrusted.example.test/"}}}, nil
			}))
			start, err := b.Start(context.Background(), "yandex", "browser", "ru", "login", "")
			if err != nil {
				t.Fatal(err)
			}
			_, login, err := b.Complete(context.Background(), "yandex", "browser", url.Values{"state": {start.State}, "code": {"synthetic-code"}})
			if !errors.Is(err, ErrInvalidAssertion) || login.Verified {
				t.Fatal("untrusted Yandex response accepted")
			}
		})
	}
}
