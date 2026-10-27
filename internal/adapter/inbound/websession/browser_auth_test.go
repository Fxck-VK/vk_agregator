package websession

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/accountlink"
	"vk-ai-aggregator/internal/service/browserauth"
)

type browserAuthTestStore struct {
	sync.Mutex
	rows   map[string]accountlink.Challenge
	counts map[string]int64
}

func (s *browserAuthTestStore) SaveChallenge(_ context.Context, key string, row accountlink.Challenge, _ time.Duration) error {
	s.Lock()
	defer s.Unlock()
	s.rows[key] = row
	return nil
}
func (s *browserAuthTestStore) LoadChallenge(_ context.Context, key string) (accountlink.Challenge, error) {
	s.Lock()
	defer s.Unlock()
	row, ok := s.rows[key]
	if !ok {
		return row, accountlink.ErrInvalidCode
	}
	return row, nil
}
func (s *browserAuthTestStore) ConsumeChallenge(_ context.Context, key string, expected accountlink.Challenge) error {
	s.Lock()
	defer s.Unlock()
	row, ok := s.rows[key]
	if !ok || row != expected || !row.ExpiresAt.After(time.Now()) {
		return accountlink.ErrInvalidCode
	}
	delete(s.rows, key)
	return nil
}
func (s *browserAuthTestStore) DeleteChallenge(_ context.Context, key string) error {
	s.Lock()
	defer s.Unlock()
	delete(s.rows, key)
	return nil
}
func (s *browserAuthTestStore) Increment(_ context.Context, key string, _ time.Duration) (int64, error) {
	s.Lock()
	defer s.Unlock()
	s.counts[key]++
	return s.counts[key], nil
}

type browserAuthTestSender struct {
	code, link string
	calls      int
}

func (s *browserAuthTestSender) SendEmailLinkCode(_ context.Context, _, code string, _ time.Time) error {
	s.code = code
	s.calls++
	return nil
}
func (s *browserAuthTestSender) SendPasswordResetLink(_ context.Context, _, link string, _ time.Time) error {
	s.link = link
	s.calls++
	return nil
}

type browserAuthTestVerifier struct {
	proof                                   accountauth.PasswordProof
	sessions                                *sessionStub
	captureCalls, completeCalls, resetCalls int
}

func (a *browserAuthTestVerifier) CapturePasswordProof(_ context.Context, email, password string) (accountauth.PasswordProof, error) {
	a.captureCalls++
	if email != "member@example.test" || password != "synthetic-password" {
		return accountauth.PasswordProof{}, accountauth.ErrInvalidPasswordLogin
	}
	p := a.proof
	p.PasswordVerified = true
	return p, nil
}
func (a *browserAuthTestVerifier) CaptureRecoveryProof(_ context.Context, email string) (accountauth.PasswordProof, error) {
	if email != "member@example.test" {
		return accountauth.PasswordProof{}, accountauth.ErrInvalidPasswordLogin
	}
	return a.proof, nil
}
func (a *browserAuthTestVerifier) ValidatePasswordProof(_ context.Context, p accountauth.PasswordProof) error {
	if p.EmailBinding.ID != a.proof.EmailBinding.ID || p.ExpectedHash != a.proof.ExpectedHash {
		return accountauth.ErrInvalidPasswordLogin
	}
	return nil
}
func (a *browserAuthTestVerifier) CompletePasswordProof(ctx context.Context, p accountauth.PasswordProof, meta accountauth.SessionMetadata) (accountauth.SessionTokens, error) {
	if !p.PasswordVerified {
		return accountauth.SessionTokens{}, accountauth.ErrInvalidPasswordLogin
	}
	if err := a.ValidatePasswordProof(ctx, p); err != nil {
		return accountauth.SessionTokens{}, err
	}
	a.completeCalls++
	return a.sessions.IssueSession(ctx, p.EmailBinding.AccountID, meta)
}
func (a *browserAuthTestVerifier) ResetPasswordProof(ctx context.Context, p accountauth.PasswordProof, _ string) error {
	if err := a.ValidatePasswordProof(ctx, p); err != nil {
		return err
	}
	a.resetCalls++
	a.proof.ExpectedHash = "replacement-synthetic-verifier"
	return nil
}

func newBrowserAuthTestHandler(t *testing.T) (*Handler, *browserAuthTestVerifier, *browserAuthTestSender) {
	t.Helper()
	h, _, sessions := newTestHandler(t)
	now := time.Now().UTC()
	auth := &browserAuthTestVerifier{sessions: sessions, proof: accountauth.PasswordProof{EmailBinding: domain.AccountIdentity{ID: uuid.New(), AccountID: uuid.New(), Provider: domain.IdentityProviderEmail, ExternalID: "member@example.test", NormalizedID: "member@example.test", VerifiedAt: now, UpdatedAt: now}, ExpectedHash: "private-synthetic-verifier"}}
	sender := &browserAuthTestSender{}
	service, err := browserauth.New(&browserAuthTestStore{rows: map[string]accountlink.Challenge{}, counts: map[string]int64{}}, sender, auth, "https://app.example.test", strings.Repeat("test-secret-material-", 3))
	if err != nil {
		t.Fatal(err)
	}
	h.deps.BrowserAuth = service
	h.cfg.EmailDeliveryEnabled = true
	h.cfg.RequireBrowserEmailProof = true
	return h, auth, sender
}

func browserAuthRequest(t *testing.T, h *Handler, method, path, origin string, body map[string]any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	return browserAuthRequestWithBinding(t, h, method, path, origin, body, "", cookies...)
}

func browserAuthRequestWithBinding(t *testing.T, h *Handler, method, path, origin string, body map[string]any, binding string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "browser-auth-test-agent")
	if binding != "" {
		req.Header.Set("X-NeiroHub-Reset-Binding", binding)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	for _, cookie := range cookies {
		if cookie != nil {
			req.AddCookie(cookie)
		}
	}
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	return rec
}

func assertPrivateBrowserAuthResponse(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	for _, private := range []string{"member@example.test", "synthetic-password", "private-synthetic-verifier", "secret_hash", "expected_hash", "access_token", "refresh_token", "email_binding"} {
		if strings.Contains(rec.Body.String(), private) {
			t.Fatalf("browser auth response exposes private field %q", private)
		}
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("browser auth response is cacheable")
	}
}

func assertSafeBrowserProofCookie(t *testing.T, cookie *http.Cookie) {
	t.Helper()
	if cookie == nil || !cookie.HttpOnly || !cookie.Secure || cookie.Path != "/" || cookie.Domain != "" || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("unsafe or missing browser proof cookie")
	}
}

func browserAuthResetToken(t *testing.T, h *Handler, sender *browserAuthTestSender, peer string) string {
	t.Helper()
	if err := h.deps.BrowserAuth.RequestReset(context.Background(), "member@example.test", "ru", peer); err != nil {
		t.Fatal(err)
	}
	link, err := url.Parse(sender.link)
	if err != nil {
		t.Fatal("reset link invalid")
	}
	fragment, err := url.ParseQuery(link.Fragment)
	if err != nil {
		t.Fatal("reset fragment invalid")
	}
	token := fragment.Get("token")
	if token == "" || link.RawQuery != "" {
		t.Fatal("reset token was not kept in URL fragment")
	}
	return token
}

func assertBrowserResetBinding(t *testing.T, rec *httptest.ResponseRecorder, token string) string {
	t.Helper()
	var metadata struct {
		Binding string `json:"binding"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &metadata) != nil {
		t.Fatalf("reset metadata failed: %d", rec.Code)
	}
	digest := sha256.Sum256([]byte(token))
	if metadata.Binding != hex.EncodeToString(digest[:]) || strings.Contains(rec.Body.String(), token) {
		t.Fatal("reset metadata did not expose only its public concurrency binding")
	}
	assertPrivateBrowserAuthResponse(t, rec)
	return metadata.Binding
}

func TestBrowserAuthPublicWritesRejectForeignOriginBeforeServices(t *testing.T) {
	h, auth, sender := newBrowserAuthTestHandler(t)
	for _, path := range []string{"/web/v1/auth/password/login", "/web/v1/auth/password/verify-login", "/web/v1/auth/password/resend-login", "/web/v1/auth/password/request-link", "/web/v1/auth/password/prepare-reset", "/web/v1/auth/password/reset-link", "/web/v1/auth/forget"} {
		for _, origin := range []string{"", "https://foreign.example.test"} {
			rec := browserAuthRequest(t, h, http.MethodPost, path, origin, map[string]any{"email": "member@example.test", "password": "synthetic-password"})
			if rec.Code != http.StatusForbidden {
				t.Fatalf("origin accepted for %s: %d", path, rec.Code)
			}
			if len(rec.Result().Cookies()) != 0 {
				t.Fatalf("foreign origin changed cookies for %s", path)
			}
		}
	}
	if auth.captureCalls != 0 || auth.completeCalls != 0 || sender.calls != 0 {
		t.Fatal("service called before origin admission")
	}
}

func TestBrowserAuthPasswordChallengeIssuesNoSessionUntilCodeThenUsesShortCookies(t *testing.T) {
	h, auth, sender := newBrowserAuthTestHandler(t)
	login := browserAuthRequest(t, h, http.MethodPost, "/web/v1/auth/password/login", "https://app.example.test", map[string]any{"email": "member@example.test", "password": "synthetic-password", "persistent": false})
	if login.Code != http.StatusAccepted || auth.completeCalls != 0 || sender.calls != 1 {
		t.Fatalf("expected challenge without session, status=%d", login.Code)
	}
	assertPrivateBrowserAuthResponse(t, login)
	cookies := cookieMap(login.Result().Cookies())
	if cookies[accessCookieName] != nil || cookies[refreshCookieName] != nil {
		t.Fatal("password alone created session cookies")
	}
	assertSafeBrowserProofCookie(t, cookies[browserBindingCookie])
	assertSafeBrowserProofCookie(t, cookies[browserChallengeCookie])
	wrong := browserAuthRequest(t, h, http.MethodPost, "/web/v1/auth/password/verify-login", "https://app.example.test", map[string]any{"code": "not-a-code"}, cookies[browserBindingCookie], cookies[browserChallengeCookie])
	if wrong.Code != http.StatusBadRequest || auth.completeCalls != 0 {
		t.Fatal("incorrect code issued a session")
	}
	verified := browserAuthRequest(t, h, http.MethodPost, "/web/v1/auth/password/verify-login", "https://app.example.test", map[string]any{"code": sender.code}, cookies[browserBindingCookie], cookies[browserChallengeCookie])
	if verified.Code != http.StatusCreated || auth.completeCalls != 1 {
		t.Fatalf("code did not complete login, status=%d", verified.Code)
	}
	assertPrivateBrowserAuthResponse(t, verified)
	done := cookieMap(verified.Result().Cookies())
	for _, name := range []string{accessCookieName, refreshCookieName, csrfCookieName} {
		cookie := done[name]
		if cookie == nil || cookie.Value == "" || cookie.MaxAge != 0 || !cookie.Expires.IsZero() {
			t.Fatalf("short-session cookie %s persisted after browser close", name)
		}
	}
	assertSafeBrowserProofCookie(t, done[accessCookieName])
	assertSafeBrowserProofCookie(t, done[refreshCookieName])
	assertSafeBrowserProofCookie(t, done[browserTrustCookie])
	assertSafeBrowserProofCookie(t, done[browserHintCookie])
	if done[csrfCookieName].HttpOnly {
		t.Fatal("CSRF cookie unreadable")
	}
	if done[browserChallengeCookie] == nil || done[browserChallengeCookie].MaxAge != -1 {
		t.Fatal("used challenge cookie retained")
	}
	principal, err := auth.sessions.AuthenticateAccessToken(context.Background(), done[accessCookieName].Value)
	if err != nil || principal.AccountID != auth.proof.EmailBinding.AccountID {
		t.Fatal("verified cookie does not authenticate shared account session")
	}
}

func TestBrowserAuthRememberedResponseIsMaskedAndForgetExpiresReceipts(t *testing.T) {
	h, _, sender := newBrowserAuthTestHandler(t)
	login := browserAuthRequest(t, h, http.MethodPost, "/web/v1/auth/password/login", "https://app.example.test", map[string]any{"email": "member@example.test", "password": "synthetic-password", "persistent": true})
	first := cookieMap(login.Result().Cookies())
	verified := browserAuthRequest(t, h, http.MethodPost, "/web/v1/auth/password/verify-login", "https://app.example.test", map[string]any{"code": sender.code}, first[browserBindingCookie], first[browserChallengeCookie])
	if verified.Code != http.StatusCreated {
		t.Fatal("verification failed")
	}
	done := cookieMap(verified.Result().Cookies())
	if done[refreshCookieName] == nil || done[refreshCookieName].MaxAge <= 0 || done[refreshCookieName].Expires.IsZero() {
		t.Fatal("persistent session is not persistent")
	}
	hint := browserAuthRequest(t, h, http.MethodGet, "/web/v1/auth/remembered", "", nil, done[browserHintCookie])
	if hint.Code != http.StatusOK || !strings.Contains(hint.Body.String(), `"email_hint":"m***@example.test"`) {
		t.Fatal("remembered hint missing")
	}
	assertPrivateBrowserAuthResponse(t, hint)
	forgot := browserAuthRequest(t, h, http.MethodPost, "/web/v1/auth/forget", "https://app.example.test", nil, first[browserBindingCookie], done[browserHintCookie], done[browserTrustCookie])
	if forgot.Code != http.StatusNoContent {
		t.Fatal("forget failed")
	}
	cleared := cookieMap(forgot.Result().Cookies())
	for _, name := range []string{browserHintCookie, browserTrustCookie, browserChallengeCookie, browserBindingCookie} {
		if cleared[name] == nil || cleared[name].MaxAge != -1 || cleared[name].Value != "" {
			t.Fatalf("forget retained %s", name)
		}
	}
	assertPrivateBrowserAuthResponse(t, forgot)
}

func TestBrowserAuthRequiredProofFailsClosedWhenCoordinatorMissing(t *testing.T) {
	h, passwords, _ := newTestHandler(t)
	h.cfg.RequireBrowserEmailProof = true
	rec := browserAuthRequest(t, h, http.MethodPost, "/web/v1/auth/password/login", "https://app.example.test", map[string]any{"email": "member@example.test", "password": "synthetic-password"})
	if rec.Code != http.StatusServiceUnavailable || passwords.calls != 0 {
		t.Fatal("required browser proof fell back to password-only login")
	}
	if cookieMap(rec.Result().Cookies())[accessCookieName] != nil {
		t.Fatal("missing coordinator created session")
	}
	methods := browserAuthRequest(t, h, http.MethodGet, "/web/v1/auth/methods", "", nil)
	if methods.Code != http.StatusOK || !strings.Contains(methods.Body.String(), `"password":false`) {
		t.Fatal("unavailable proof login advertised as available")
	}
}

func TestBrowserAuthResetPrepareKeepsTokenHttpOnlyAndStatusHasNoSecretJSON(t *testing.T) {
	h, auth, sender := newBrowserAuthTestHandler(t)
	token := browserAuthResetToken(t, h, sender, "local-test-peer")
	prepared := browserAuthRequest(t, h, http.MethodPost, "/web/v1/auth/password/prepare-reset", "https://app.example.test", map[string]any{"token": token})
	binding := assertBrowserResetBinding(t, prepared, token)
	proof := cookieMap(prepared.Result().Cookies())[browserResetCookie]
	assertSafeBrowserProofCookie(t, proof)
	if proof.Value != token || proof.MaxAge <= 0 || proof.MaxAge > int(browserauth.ResetTTL.Seconds()) {
		t.Fatal("reset cookie lost token bounds")
	}
	status := browserAuthRequestWithBinding(t, h, http.MethodGet, "/web/v1/auth/password/reset-status", "", nil, binding, proof)
	if assertBrowserResetBinding(t, status, token) != binding {
		t.Fatal("reset status changed the concurrency binding")
	}
	reset := browserAuthRequestWithBinding(t, h, http.MethodPost, "/web/v1/auth/password/reset-link", "https://app.example.test", map[string]any{"new_password": "synthetic-replacement-password"}, binding, proof)
	if reset.Code != http.StatusNoContent || auth.resetCalls != 1 {
		t.Fatalf("reset did not complete: %d", reset.Code)
	}
	assertPrivateBrowserAuthResponse(t, reset)
	cleared := cookieMap(reset.Result().Cookies())
	for _, name := range []string{accessCookieName, refreshCookieName, browserResetCookie, browserTrustCookie, browserChallengeCookie} {
		if cleared[name] == nil || cleared[name].MaxAge != -1 {
			t.Fatalf("reset retained %s", name)
		}
	}
	status = browserAuthRequest(t, h, http.MethodGet, "/web/v1/auth/password/reset-status", "", nil, proof)
	if status.Code != http.StatusBadRequest {
		t.Fatal("consumed reset proof still active")
	}
}

func TestBrowserAuthResetRejectsAnotherTabsCookieWithoutChangingPassword(t *testing.T) {
	h, auth, sender := newBrowserAuthTestHandler(t)
	tokenA := browserAuthResetToken(t, h, sender, "local-tab-a")
	preparedA := browserAuthRequest(t, h, http.MethodPost, "/web/v1/auth/password/prepare-reset", "https://app.example.test", map[string]any{"token": tokenA})
	bindingA := assertBrowserResetBinding(t, preparedA, tokenA)
	// Advance the fake verifier to an independent account before opening its
	// link in tab B. The real browser shares this newer HttpOnly cookie with A.
	auth.proof.EmailBinding.ID = uuid.New()
	auth.proof.EmailBinding.AccountID = uuid.New()
	tokenB := browserAuthResetToken(t, h, sender, "local-tab-b")
	preparedB := browserAuthRequest(t, h, http.MethodPost, "/web/v1/auth/password/prepare-reset", "https://app.example.test", map[string]any{"token": tokenB})
	bindingB := assertBrowserResetBinding(t, preparedB, tokenB)
	proofB := cookieMap(preparedB.Result().Cookies())[browserResetCookie]
	if bindingA == bindingB {
		t.Fatal("independent reset links reused a concurrency binding")
	}
	status := browserAuthRequestWithBinding(t, h, http.MethodGet, "/web/v1/auth/password/reset-status", "", nil, bindingA, proofB)
	if status.Code != http.StatusBadRequest {
		t.Fatal("status accepted another tab's reset cookie")
	}
	assertPrivateBrowserAuthResponse(t, status)
	for _, binding := range []string{"", bindingA} {
		reset := browserAuthRequestWithBinding(t, h, http.MethodPost, "/web/v1/auth/password/reset-link", "https://app.example.test", map[string]any{"new_password": "synthetic-replacement-password"}, binding, proofB)
		if reset.Code != http.StatusBadRequest || auth.resetCalls != 0 {
			t.Fatal("missing or mismatched tab binding changed a password")
		}
		assertPrivateBrowserAuthResponse(t, reset)
	}
	reset := browserAuthRequestWithBinding(t, h, http.MethodPost, "/web/v1/auth/password/reset-link", "https://app.example.test", map[string]any{"new_password": "synthetic-replacement-password"}, bindingB, proofB)
	if reset.Code != http.StatusNoContent || auth.resetCalls != 1 {
		t.Fatal("rejecting tab A consumed tab B's valid reset proof")
	}
}
