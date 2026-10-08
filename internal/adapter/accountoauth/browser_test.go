package accountoauth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"vk-ai-aggregator/internal/domain"
)

type browserMemoryStore struct{ data map[string]BrowserTransaction }

func (s *browserMemoryStore) Save(_ context.Context, key string, tx BrowserTransaction, _ time.Duration) error {
	s.data[key] = tx
	return nil
}

type browserRoundTrip func(*http.Request) (*http.Response, error)

func (f browserRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type browserProofAdapter struct {
	name  domain.IdentityProvider
	nonce string
}

func (a *browserProofAdapter) Provider() domain.IdentityProvider { return a.name }
func (a *browserProofAdapter) Verify(_ context.Context, req VerifyRequest) (domain.VerifiedAccountLogin, error) {
	if req.IDToken != "provider-signed-proof" || req.ExpectedNonce != a.nonce || a.nonce == "" {
		return domain.VerifiedAccountLogin{}, ErrInvalidAssertion
	}
	return domain.VerifiedAccountLogin{Method: domain.AccountLoginGoogle, ExternalID: "subject", Verified: true}, nil
}

func TestBrowserCodeExchangeKeepsSecretsServerSideAndConsumesProof(t *testing.T) {
	for _, name := range []string{"google", "apple", "telegram", "vk"} {
		t.Run(name, func(t *testing.T) {
			store := &browserMemoryStore{data: map[string]BrowserTransaction{}}
			proof := &browserProofAdapter{name: domain.IdentityProvider(name)}
			cfg := BrowserConfig{WebOrigin: "https://app.example.test", GoogleEnabled: true, GoogleClientID: "client", GoogleClientSecret: "secret", AppleClientID: "client", AppleClientSecret: "secret", TelegramClientID: "client", TelegramClientSecret: "secret", VKIDClientID: "client"}
			calls := 0
			var state string
			client := &http.Client{Transport: browserRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != "POST" || r.URL.RawQuery != "" {
					t.Fatal("credentials placed in URL")
				}
				r.ParseForm()
				body := `{"id_token":"provider-signed-proof"}`
				if strings.HasSuffix(r.URL.Path, "user_info") {
					if r.Form.Get("access_token") != "provider-access" {
						t.Fatal("missing provider proof")
					}
					body = `{"user":{"user_id":"12345"}}`
				} else {
					if r.Form.Get("code") != "code" || r.Form.Get("client_id") != "client" || r.Form.Get("redirect_uri") != "https://app.example.test/web/v1/auth/oauth/"+name+"/callback" {
						t.Fatal("bad exchange binding")
					}
					if name != "apple" && r.Form.Get("code_verifier") == "" {
						t.Fatal("missing PKCE")
					}
					if name == "telegram" {
						user, pass, ok := r.BasicAuth()
						if !ok || user != "client" || pass != "secret" {
							t.Fatal("missing confidential-client auth")
						}
					}
					if name == "vk" {
						body = `{"access_token":"provider-access","state":"` + state + `"}`
					}
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			b := NewBrowser(cfg, store, NewRegistry(proof), client)
			start, err := b.Start(context.Background(), name, "browser", "en", "login", "")
			if err != nil {
				t.Fatal(err)
			}
			state = start.State
			proof.nonce = store.data[stateKey(state)].Nonce
			q := url.Values{"state": {state}, "code": {"code"}, "device_id": {"device"}}
			if _, _, err := b.Complete(context.Background(), name, "wrong-browser", q); err == nil || calls != 0 {
				t.Fatal("unbound exchange started")
			}
			tx, login, err := b.Complete(context.Background(), name, "browser", q)
			if err != nil || !login.Verified || tx.Locale != "en" {
				t.Fatalf("completion failed: %v", err)
			}
			before := calls
			if _, _, err := b.Complete(context.Background(), name, "browser", q); err == nil || calls != before {
				t.Fatal("code replay accepted")
			}
		})
	}
}

func TestBrowserExpiredTransactionNeverExchangesCode(t *testing.T) {
	store := &browserMemoryStore{data: map[string]BrowserTransaction{}}
	b := NewBrowser(BrowserConfig{WebOrigin: "https://app.example.test", GoogleEnabled: true, GoogleClientID: "client", GoogleClientSecret: "secret"}, store, NewRegistry(), nil)
	start, _ := b.Start(context.Background(), "google", "browser", "ru", "login", "")
	tx := store.data[stateKey(start.State)]
	tx.ExpiresAt = time.Now().Add(-time.Minute)
	store.data[stateKey(start.State)] = tx
	if _, _, err := b.Complete(context.Background(), "google", "browser", url.Values{"state": {start.State}, "code": {"code"}}); !errors.Is(err, ErrInvalidAssertion) {
		t.Fatal("expired proof accepted")
	}
}
func (s *browserMemoryStore) Take(_ context.Context, key, binding string) (BrowserTransaction, error) {
	tx, ok := s.data[key]
	if !ok || tx.BindingHash != binding {
		return BrowserTransaction{}, ErrInvalidAssertion
	}
	delete(s.data, key)
	return tx, nil
}

func TestBrowserOAuthStartIsBoundAndFailClosed(t *testing.T) {
	store := &browserMemoryStore{data: map[string]BrowserTransaction{}}
	service := NewBrowser(BrowserConfig{WebOrigin: "https://app.example.test", GoogleEnabled: true, GoogleClientID: "client", GoogleClientSecret: "server-secret"}, store, NewRegistry(), nil)
	start, err := service.Start(context.Background(), "google", "binding", "ru", "login", "")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(start.URL)
	q := u.Query()
	if u.Host != "accounts.google.com" || q.Get("state") == "" || q.Get("nonce") == "" || q.Get("code_challenge_method") != "S256" || q.Get("redirect_uri") != "https://app.example.test/web/v1/auth/oauth/google/callback" || q.Get("client_secret") != "" {
		t.Fatal("unsafe authorization URL")
	}
	if _, err := store.Take(context.Background(), stateKey(start.State), "wrong"); !errors.Is(err, ErrInvalidAssertion) {
		t.Fatal("unbound browser accepted")
	}
	if _, err := store.Take(context.Background(), stateKey(start.State), bindingHash("binding")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Take(context.Background(), stateKey(start.State), bindingHash("binding")); !errors.Is(err, ErrInvalidAssertion) {
		t.Fatal("transaction replay accepted")
	}
	if _, err := service.Start(context.Background(), "apple", "binding", "ru", "login", ""); !errors.Is(err, ErrUnavailable) {
		t.Fatal("unconfigured provider enabled")
	}
}

func TestBrowserOAuthGoogleDisabledByDefault(t *testing.T) {
	store := &browserMemoryStore{data: map[string]BrowserTransaction{}}
	service := NewBrowser(BrowserConfig{WebOrigin: "https://app.example.test", GoogleClientID: "client", GoogleClientSecret: "server-secret"}, store, NewRegistry(), nil)

	if providers := service.Providers(); containsToken(providers, "google") {
		t.Fatalf("providers = %v, want google omitted", providers)
	}
	if _, err := service.Start(context.Background(), "google", "binding", "ru", "login", ""); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Start error = %v, want ErrUnavailable", err)
	}
}

func TestOIDCBrowserNonceMustMatch(t *testing.T) {
	verifier := &fakeOIDCVerifier{claims: OIDCClaims{Subject: "subject", Nonce: "other"}}
	a := NewOIDCAdapter(OIDCAdapterConfig{Provider: domain.IdentityProviderGoogle, Method: domain.AccountLoginGoogle, Issuers: []string{"issuer"}, Audience: []string{"client"}, Verifier: verifier})
	if _, err := a.Verify(context.Background(), VerifyRequest{Provider: domain.IdentityProviderGoogle, IDToken: "token", ExpectedNonce: "expected"}); !errors.Is(err, ErrInvalidAssertion) {
		t.Fatal("nonce mismatch accepted")
	}
	verifier.claims.Nonce = "expected"
	if _, err := a.Verify(context.Background(), VerifyRequest{Provider: domain.IdentityProviderGoogle, IDToken: "token", ExpectedNonce: "expected"}); err != nil {
		t.Fatal(err)
	}
}
