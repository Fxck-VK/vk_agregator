package accountoauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"vk-ai-aggregator/internal/domain"
)

const BrowserTransactionTTL = 10 * time.Minute

// BrowserTransaction contains short-lived server-only OAuth material. Never log it.
type BrowserTransaction struct {
	Provider    string    `json:"provider"`
	BindingHash string    `json:"binding_hash"`
	Verifier    string    `json:"verifier"`
	Nonce       string    `json:"nonce"`
	Locale      string    `json:"locale"`
	Intent      string    `json:"intent"`
	AccountID   string    `json:"account_id,omitempty"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type BrowserStore interface {
	Save(context.Context, string, BrowserTransaction, time.Duration) error
	// Take must check the binding and consume once atomically.
	Take(context.Context, string, string) (BrowserTransaction, error)
}

type BrowserConfig struct {
	WebOrigin                              string
	GoogleClientID, GoogleClientSecret     string
	AppleClientID, AppleClientSecret       string
	TelegramClientID, TelegramClientSecret string
	VKIDClientID                           string
}

type browserProvider struct {
	id, secret, authorize, token, scope string
	pkce, basic                         bool
}
type Browser struct {
	cfg      BrowserConfig
	store    BrowserStore
	registry *Registry
	client   *http.Client
}
type BrowserStart struct {
	URL   string `json:"authorization_url"`
	State string `json:"-"`
}

func NewBrowser(cfg BrowserConfig, store BrowserStore, registry *Registry, client *http.Client) *Browser {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Browser{cfg, store, registry, &copyClient}
}

func (b *Browser) provider(name string) (browserProvider, bool) {
	if b == nil || b.store == nil || b.registry == nil {
		return browserProvider{}, false
	}
	origin, err := url.Parse(b.cfg.WebOrigin)
	if err != nil || origin.Scheme != "https" || origin.Host == "" || origin.User != nil || (origin.Path != "" && origin.Path != "/") || origin.RawQuery != "" || origin.Fragment != "" {
		return browserProvider{}, false
	}
	var p browserProvider
	switch name {
	case "google":
		p = browserProvider{b.cfg.GoogleClientID, b.cfg.GoogleClientSecret, "https://accounts.google.com/o/oauth2/v2/auth", "https://oauth2.googleapis.com/token", "openid", true, false}
	case "apple":
		p = browserProvider{b.cfg.AppleClientID, b.cfg.AppleClientSecret, "https://appleid.apple.com/auth/authorize", "https://appleid.apple.com/auth/token", "", false, false}
	case "telegram":
		p = browserProvider{b.cfg.TelegramClientID, b.cfg.TelegramClientSecret, "https://oauth.telegram.org/auth", "https://oauth.telegram.org/token", "openid", true, true}
	case "vk":
		p = browserProvider{id: b.cfg.VKIDClientID, authorize: "https://id.vk.ru/authorize", token: "https://id.vk.ru/oauth2/auth", pkce: true}
	default:
		return p, false
	}
	return p, p.id != "" && (name == "vk" || p.secret != "")
}

func (b *Browser) Providers() []string {
	out := []string{}
	for _, name := range []string{"google", "apple", "vk", "telegram"} {
		if _, ok := b.provider(name); ok {
			out = append(out, name)
		}
	}
	return out
}

func (b *Browser) callback(name string) string {
	return strings.TrimRight(b.cfg.WebOrigin, "/") + "/web/v1/auth/oauth/" + name + "/callback"
}
func browserSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
func bindingHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func stateKey(value string) string { return "accountoauth:browser:" + bindingHash(value) }

func (b *Browser) Start(ctx context.Context, name, binding, locale, intent, accountID string) (BrowserStart, error) {
	p, ok := b.provider(name)
	if !ok {
		return BrowserStart{}, ErrUnavailable
	}
	if binding == "" || (intent != "login" && intent != "link") || (intent == "link" && accountID == "") {
		return BrowserStart{}, ErrInvalidAssertion
	}
	if locale != "en" {
		locale = "ru"
	}
	state, err := browserSecret()
	if err != nil {
		return BrowserStart{}, err
	}
	verifier, err := browserSecret()
	if err != nil {
		return BrowserStart{}, err
	}
	nonce, err := browserSecret()
	if err != nil {
		return BrowserStart{}, err
	}
	tx := BrowserTransaction{name, bindingHash(binding), verifier, nonce, locale, intent, accountID, time.Now().Add(BrowserTransactionTTL)}
	if err := b.store.Save(ctx, stateKey(state), tx, BrowserTransactionTTL); err != nil {
		return BrowserStart{}, ErrUnavailable
	}
	q := url.Values{"client_id": {p.id}, "redirect_uri": {b.callback(name)}, "response_type": {"code"}, "state": {state}}
	if p.scope != "" {
		q.Set("scope", p.scope)
	}
	if name != "vk" {
		q.Set("nonce", nonce)
	}
	if name == "apple" {
		q.Set("response_mode", "query")
	}
	if p.pkce {
		sum := sha256.Sum256([]byte(verifier))
		q.Set("code_challenge", base64.RawURLEncoding.EncodeToString(sum[:]))
		q.Set("code_challenge_method", "S256")
	}
	return BrowserStart{p.authorize + "?" + q.Encode(), state}, nil
}

// Complete consumes the browser-bound transaction before any token exchange.
// Provider responses remain server-side and are reduced to a verified subject.
func (b *Browser) Complete(ctx context.Context, name, binding string, query url.Values) (BrowserTransaction, domain.VerifiedAccountLogin, error) {
	empty := domain.VerifiedAccountLogin{}
	p, ok := b.provider(name)
	if !ok {
		return BrowserTransaction{}, empty, ErrUnavailable
	}
	state := query.Get("state")
	if len(state) != 43 || binding == "" {
		return BrowserTransaction{}, empty, ErrInvalidAssertion
	}
	tx, err := b.store.Take(ctx, stateKey(state), bindingHash(binding))
	if err != nil {
		return BrowserTransaction{}, empty, ErrInvalidAssertion
	}
	if tx.Provider != name || !time.Now().Before(tx.ExpiresAt) || query.Get("error") != "" {
		return tx, empty, ErrInvalidAssertion
	}
	code := query.Get("code")
	if code == "" || len(code) > 4096 {
		return tx, empty, ErrInvalidAssertion
	}
	form := url.Values{"grant_type": {"authorization_code"}, "client_id": {p.id}, "redirect_uri": {b.callback(name)}, "code": {code}}
	if p.pkce {
		form.Set("code_verifier", tx.Verifier)
	}
	if p.secret != "" && !p.basic {
		form.Set("client_secret", p.secret)
	}
	if name == "vk" {
		if query.Get("device_id") == "" || len(query.Get("device_id")) > 512 {
			return tx, empty, ErrInvalidAssertion
		}
		form.Set("device_id", query.Get("device_id"))
		form.Set("state", state)
	}
	var tokens struct {
		IDToken     string `json:"id_token"`
		AccessToken string `json:"access_token"`
		State       string `json:"state"`
		Error       string `json:"error"`
	}
	if err := b.post(ctx, p.token, form, p, &tokens); err != nil || tokens.Error != "" {
		return tx, empty, ErrInvalidAssertion
	}
	if name == "vk" {
		if tokens.AccessToken == "" || subtle.ConstantTimeCompare([]byte(tokens.State), []byte(state)) != 1 {
			return tx, empty, ErrInvalidAssertion
		}
		var info struct {
			User struct {
				ID string `json:"user_id"`
			} `json:"user"`
			Error string `json:"error"`
		}
		if err := b.post(ctx, "https://id.vk.ru/oauth2/user_info", url.Values{"client_id": {p.id}, "access_token": {tokens.AccessToken}}, browserProvider{}, &info); err != nil || info.Error != "" {
			return tx, empty, ErrInvalidAssertion
		}
		id, err := strconv.ParseInt(info.User.ID, 10, 64)
		if err != nil || id <= 0 {
			return tx, empty, ErrInvalidAssertion
		}
		return tx, domain.VerifiedAccountLogin{Method: domain.AccountLoginVKID, ExternalID: info.User.ID, Verified: true}, nil
	}
	login, err := b.registry.Verify(ctx, VerifyRequest{Provider: domain.IdentityProvider(name), IDToken: tokens.IDToken, ExpectedNonce: tx.Nonce})
	return tx, login, err
}

func (b *Browser) post(ctx context.Context, endpoint string, form url.Values, provider browserProvider, out any) error {
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return ErrUnavailable
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if provider.basic {
		req.SetBasicAuth(provider.id, provider.secret)
	}
	response, err := b.client.Do(req)
	if err != nil {
		return ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return ErrInvalidAssertion
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(out); err != nil {
		return ErrInvalidAssertion
	}
	return nil
}
