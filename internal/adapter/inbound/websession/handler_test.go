package websession

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/accountservice"
)

func TestPasswordLoginSetsSafeHostOnlyCookies(t *testing.T) {
	h, passwords, _ := newTestHandler(t)
	accountID := uuid.New()
	passwords.resolution = domain.IdentityResolution{AccountID: accountID}

	req := httptest.NewRequest(http.MethodPost, "/web/v1/auth/password/login", strings.NewReader(`{"email":"owner@example.test","password":"correct horse battery staple"}`))
	req.Header.Set("Origin", "https://app.example.test")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Domain != "" || cookie.Path != "/" || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
			t.Fatalf("unsafe cookie attributes: %+v", cookie)
		}
		if cookie.Name == "nh_csrf" && cookie.HttpOnly {
			t.Fatal("CSRF cookie must be readable by browser code")
		}
		if (cookie.Name == "nh_access" || cookie.Name == "nh_refresh") && !cookie.HttpOnly {
			t.Fatal("session cookie must be HttpOnly")
		}
	}
	if got := rec.Body.String(); strings.Contains(got, "access_token") || strings.Contains(got, "refresh_token") {
		t.Fatalf("raw tokens serialized: %s", got)
	}
}

func TestRefreshAndLogoutRequireOriginAndCSRFPriorToSessionService(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	refresh := "refresh-secret"

	for _, test := range []struct {
		name       string
		path       string
		origin     string
		csrf       string
		headerCSRF string
	}{
		{name: "cross origin", path: "/web/v1/auth/refresh", origin: "https://evil.example.test"},
		{name: "missing csrf", path: "/web/v1/auth/logout", origin: "https://app.example.test"},
		{name: "mismatched csrf", path: "/web/v1/auth/refresh", origin: "https://app.example.test", csrf: "correct", headerCSRF: "wrong"},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, test.path, nil)
			req.Header.Set("Origin", test.origin)
			req.AddCookie(&http.Cookie{Name: "nh_refresh", Value: refresh})
			if test.csrf != "" {
				req.AddCookie(&http.Cookie{Name: "nh_csrf", Value: test.csrf})
				req.Header.Set("X-CSRF-Token", test.headerCSRF)
			}
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", rec.Code)
			}
			if sessions.refreshCalls != 0 || sessions.logoutCalls != 0 {
				t.Fatal("session service called before origin/CSRF validation")
			}
		})
	}
}

func TestRefreshLogoutAndMeUseCookiePrincipalOnly(t *testing.T) {
	h, passwords, sessions := newTestHandler(t)
	accountID := uuid.New()
	passwords.resolution = domain.IdentityResolution{AccountID: accountID}

	login := httptest.NewRequest(http.MethodPost, "/web/v1/auth/password/login", strings.NewReader(`{"email":"owner@example.test","password":"pw"}`))
	login.Header.Set("Origin", "https://app.example.test")
	loginRec := httptest.NewRecorder()
	h.Routes().ServeHTTP(loginRec, login)
	if loginRec.Code != http.StatusCreated {
		t.Fatalf("login status = %d", loginRec.Code)
	}
	cookies := cookieMap(loginRec.Result().Cookies())

	me := httptest.NewRequest(http.MethodGet, "/web/v1/me?access_token=ignored", nil)
	me.Header.Set("Authorization", "Bearer forged")
	me.Header.Set("X-Account-ID", uuid.NewString())
	me.AddCookie(cookies["nh_access"])
	meRec := httptest.NewRecorder()
	h.Routes().ServeHTTP(meRec, me)
	if meRec.Code != http.StatusOK {
		t.Fatalf("me status = %d, body = %s", meRec.Code, meRec.Body.String())
	}
	if got := meRec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if h.deps.Account.(*profileStub).lastAccountID != accountID {
		t.Fatal("profile used client-controlled account identity")
	}

	refresh := httptest.NewRequest(http.MethodPost, "/web/v1/auth/refresh", nil)
	refresh.Header.Set("Origin", "https://app.example.test")
	refresh.Header.Set("X-CSRF-Token", cookies["nh_csrf"].Value)
	refresh.AddCookie(cookies["nh_refresh"])
	refresh.AddCookie(cookies["nh_csrf"])
	refreshRec := httptest.NewRecorder()
	h.Routes().ServeHTTP(refreshRec, refresh)
	if refreshRec.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, body = %s", refreshRec.Code, refreshRec.Body.String())
	}
	if strings.Contains(refreshRec.Body.String(), "token") {
		t.Fatalf("refresh serialized token: %s", refreshRec.Body.String())
	}

	rotated := cookieMap(refreshRec.Result().Cookies())
	logout := httptest.NewRequest(http.MethodPost, "/web/v1/auth/logout", nil)
	logout.Header.Set("Origin", "https://app.example.test")
	logout.Header.Set("X-CSRF-Token", rotated["nh_csrf"].Value)
	logout.AddCookie(rotated["nh_refresh"])
	logout.AddCookie(rotated["nh_csrf"])
	logoutRec := httptest.NewRecorder()
	h.Routes().ServeHTTP(logoutRec, logout)
	if logoutRec.Code != http.StatusNoContent || sessions.logoutCalls != 1 {
		t.Fatalf("logout status/calls = %d/%d", logoutRec.Code, sessions.logoutCalls)
	}
	for _, cookie := range logoutRec.Result().Cookies() {
		if cookie.MaxAge >= 0 {
			t.Fatalf("cookie %q was not expired", cookie.Name)
		}
	}
}

func TestLogoutClearsCookiesWhenSessionRevocationFails(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	sessions.logoutErr = errors.New("database unavailable")
	req := httptest.NewRequest(http.MethodPost, "/web/v1/auth/logout", nil)
	req.Header.Set("Origin", "https://app.example.test")
	req.Header.Set("X-CSRF-Token", "csrf")
	req.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "refresh"})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code < http.StatusInternalServerError {
		t.Fatalf("status = %d, want non-success 5xx", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "database unavailable") {
		t.Fatalf("response exposes logout error: %s", rec.Body.String())
	}
	for _, name := range []string{accessCookieName, refreshCookieName, csrfCookieName} {
		cookie, ok := cookieMap(rec.Result().Cookies())[name]
		if !ok || cookie.MaxAge >= 0 {
			t.Fatalf("cookie %q was not expired: %+v", name, cookie)
		}
	}
}

func TestMeRejectsBearerAndQueryIdentityWithoutAccessCookie(t *testing.T) {
	h, _, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/web/v1/me?access_token=forged", nil)
	req.Header.Set("Authorization", "Bearer forged")
	req.Header.Set("X-Account-ID", uuid.NewString())
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestConversationListRequiresAccessCookie(t *testing.T) {
	h, _, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/web/v1/conversations", nil)
	rec := httptest.NewRecorder()

	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestConversationListUsesCookiePrincipalAccountOnly(t *testing.T) {
	h, conversations, sessions := newConversationTestHandler(t)
	accountID := uuid.New()
	otherAccountID := uuid.New()
	owned := &domain.Conversation{AccountID: accountID, Source: domain.ConversationSourceWeb, ExternalThreadID: "owned", Title: "Owned"}
	foreign := &domain.Conversation{AccountID: otherAccountID, Source: domain.ConversationSourceWeb, ExternalThreadID: "foreign", Title: "Foreign"}
	for _, conversation := range []*domain.Conversation{owned, foreign} {
		if err := conversations.CreateConversation(context.Background(), conversation); err != nil {
			t.Fatalf("seed conversation: %v", err)
		}
	}

	req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/conversations?limit=20", sessions, accountID)
	req.Header.Set("X-Account-ID", otherAccountID.String())
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	items := assertSafeConversationList(t, rec.Body.Bytes())
	if len(items) != 1 || items[0] != owned.ID {
		t.Fatalf("conversation ids = %v, want [%s]", items, owned.ID)
	}
}

func TestConversationListReturnsEmptyItemsArray(t *testing.T) {
	h, _, sessions := newConversationTestHandler(t)
	req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/conversations", sessions, uuid.New())
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := assertSafeConversationList(t, rec.Body.Bytes()); len(got) != 0 {
		t.Fatalf("conversation count = %d, want 0", len(got))
	}
}

func TestConversationListRejectsMalformedLimit(t *testing.T) {
	for _, rawLimit := range []string{"", "0", "51", "-1", "not-a-number"} {
		t.Run(rawLimit, func(t *testing.T) {
			h, _, sessions := newConversationTestHandler(t)
			req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/conversations?limit="+rawLimit, sessions, uuid.New())
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestConversationCreateRejectsOriginAndCSRFFailuresBeforeRepository(t *testing.T) {
	for _, test := range []struct {
		name       string
		origin     string
		csrfCookie string
		csrfHeader string
	}{
		{name: "missing origin", csrfCookie: "csrf", csrfHeader: "csrf"},
		{name: "wrong origin", origin: "https://evil.example.test", csrfCookie: "csrf", csrfHeader: "csrf"},
		{name: "missing csrf", origin: "https://app.example.test"},
		{name: "mismatched csrf", origin: "https://app.example.test", csrfCookie: "csrf", csrfHeader: "wrong"},
	} {
		t.Run(test.name, func(t *testing.T) {
			h, conversations, sessions := newConversationTestHandler(t)
			spy := &conversationRepositorySpy{ConversationRepository: conversations}
			h.deps.Conversations = spy
			req := authenticatedConversationRequest(t, http.MethodPost, "/web/v1/conversations", sessions, uuid.New())
			req.Header.Set("X-Idempotency-Key", uuid.NewString())
			if test.origin != "" {
				req.Header.Set("Origin", test.origin)
			}
			if test.csrfCookie != "" {
				req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: test.csrfCookie})
			}
			if test.csrfHeader != "" {
				req.Header.Set("X-CSRF-Token", test.csrfHeader)
			}
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if spy.createCalls != 0 || spy.referenceCalls != 0 {
				t.Fatal("conversation repository called before origin/CSRF validation")
			}
		})
	}
}

func TestConversationCreateRejectsInvalidIdempotencyKey(t *testing.T) {
	h, conversations, sessions := newConversationTestHandler(t)
	spy := &conversationRepositorySpy{ConversationRepository: conversations}
	h.deps.Conversations = spy
	req := safeConversationCreateRequest(t, sessions, uuid.New(), "not-a-uuid")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if spy.createCalls != 0 {
		t.Fatal("conversation repository called for invalid idempotency key")
	}
}

func TestConversationCreateRequiresIdempotencyHeader(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*http.Request, string)
	}{
		{name: "missing", mutate: func(req *http.Request, _ string) { req.Header.Del("X-Idempotency-Key") }},
		{name: "query only", mutate: func(req *http.Request, key string) {
			req.Header.Del("X-Idempotency-Key")
			req.URL.RawQuery = "idempotency_key=" + key
		}},
		{name: "body only", mutate: func(req *http.Request, key string) {
			req.Header.Del("X-Idempotency-Key")
			req.Body = io.NopCloser(strings.NewReader(`{"idempotency_key":"` + key + `"}`))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			h, conversations, sessions := newConversationTestHandler(t)
			spy := &conversationRepositorySpy{ConversationRepository: conversations}
			h.deps.Conversations = spy
			req := safeConversationCreateRequest(t, sessions, uuid.New(), uuid.NewString())
			test.mutate(req, uuid.NewString())
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if spy.createCalls != 0 || spy.referenceCalls != 0 {
				t.Fatal("conversation repository called without an idempotency header")
			}
		})
	}
}

func TestConversationInputValidationPrecedesUnavailableRepository(t *testing.T) {
	t.Run("list limit", func(t *testing.T) {
		h, _, sessions := newTestHandler(t)
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, authenticatedConversationRequest(t, http.MethodGet, "/web/v1/conversations?limit=0", sessions, uuid.New()))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("idempotency key", func(t *testing.T) {
		h, _, sessions := newTestHandler(t)
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, safeConversationCreateRequest(t, sessions, uuid.New(), "not-a-uuid"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
	})
}

func TestConversationCreateReturnsCreatedThenExistingForSameIdempotencyKey(t *testing.T) {
	h, conversations, sessions := newConversationTestHandler(t)
	accountID := uuid.New()
	idempotencyKey := uuid.NewString()

	first := httptest.NewRecorder()
	h.Routes().ServeHTTP(first, safeConversationCreateRequest(t, sessions, accountID, idempotencyKey))
	if first.Code != http.StatusCreated {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}
	if got := first.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("first Cache-Control = %q, want no-store", got)
	}
	firstID := safeConversationDTO(t, first.Body.Bytes())

	replay := httptest.NewRecorder()
	h.Routes().ServeHTTP(replay, safeConversationCreateRequest(t, sessions, accountID, idempotencyKey))
	if replay.Code != http.StatusOK {
		t.Fatalf("replay status = %d, body = %s", replay.Code, replay.Body.String())
	}
	if replayID := safeConversationDTO(t, replay.Body.Bytes()); replayID != firstID {
		t.Fatalf("replay id = %s, want %s", replayID, firstID)
	}

	stored, err := conversations.GetActiveByReference(context.Background(), domain.ConversationRef{
		AccountID:        accountID,
		Source:           domain.ConversationSourceWeb,
		ExternalThreadID: idempotencyKey,
	})
	if err != nil {
		t.Fatalf("load stored conversation: %v", err)
	}
	if stored.ID != firstID || stored.AccountID != accountID || stored.UserID != uuid.Nil || stored.Source != domain.ConversationSourceWeb || stored.Status != domain.ConversationActive || stored.Title != "" || stored.VKPeerID != 0 {
		t.Fatalf("stored conversation = %#v, want account-owned empty active web conversation", stored)
	}
}

func TestConversationDTOBodyContainsNoForbiddenInternalFields(t *testing.T) {
	h, conversations, sessions := newConversationTestHandler(t)
	accountID := uuid.New()
	conversation := &domain.Conversation{AccountID: accountID, Source: domain.ConversationSourceWeb, ExternalThreadID: "private-thread", Title: "Safe title"}
	if err := conversations.CreateConversation(context.Background(), conversation); err != nil {
		t.Fatalf("seed conversation: %v", err)
	}

	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, authenticatedConversationRequest(t, http.MethodGet, "/web/v1/conversations", sessions, accountID))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	_ = assertSafeConversationList(t, rec.Body.Bytes())
}

func TestPasswordLoginRequiresExactOriginBeforeVerifier(t *testing.T) {
	for _, test := range []struct {
		name   string
		origin string
	}{
		{name: "missing Origin"},
		{name: "wrong Origin", origin: "https://evil.example.test"},
	} {
		t.Run(test.name, func(t *testing.T) {
			h, passwords, _ := newTestHandler(t)
			passwords.resolution = domain.IdentityResolution{AccountID: uuid.New()}
			req := httptest.NewRequest(http.MethodPost, "/web/v1/auth/password/login", strings.NewReader(`{"email":"owner@example.test","password":"pw"}`))
			if test.origin != "" {
				req.Header.Set("Origin", test.origin)
			}
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", rec.Code)
			}
			if passwords.calls != 0 {
				t.Fatal("password verifier called before origin validation")
			}
		})
	}
}

func TestMeRejectsEmptyExpiredAndRevokedAccessCookies(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	assertMeUnauthorized(t, h, "")

	revoked, err := sessions.IssueSession(context.Background(), uuid.New(), accountauth.SessionMetadata{})
	if err != nil {
		t.Fatalf("issue revoked session: %v", err)
	}
	if err := sessions.Logout(context.Background(), revoked.RefreshToken); err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	assertMeUnauthorized(t, h, revoked.AccessToken)

	now := time.Now().UTC()
	expiring := accountauth.New(nil,
		accountauth.WithSessionRepository(memory.NewAccountSessionRepo()),
		accountauth.WithSessionTTL(time.Hour),
		accountauth.WithAccessTokenTTL(time.Minute),
		accountauth.WithClock(func() time.Time { return now }),
	)
	expired, err := expiring.IssueSession(context.Background(), uuid.New(), accountauth.SessionMetadata{})
	if err != nil {
		t.Fatalf("issue expiring session: %v", err)
	}
	now = now.Add(2 * time.Minute)
	expiredHandler := NewHandler(Config{WebOrigin: "https://app.example.test"}, Deps{Authenticator: expiring, Account: &profileStub{}})
	assertMeUnauthorized(t, expiredHandler, expired.AccessToken)
}

func assertMeUnauthorized(t *testing.T, h *Handler, token string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/web/v1/me", nil)
	req.AddCookie(&http.Cookie{Name: "nh_access", Value: token})
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

type conversationRepositorySpy struct {
	domain.ConversationRepository
	createCalls    int
	referenceCalls int
}

func (s *conversationRepositorySpy) CreateConversation(ctx context.Context, conversation *domain.Conversation) error {
	s.createCalls++
	return s.ConversationRepository.CreateConversation(ctx, conversation)
}

func (s *conversationRepositorySpy) GetActiveByReference(ctx context.Context, ref domain.ConversationRef) (*domain.Conversation, error) {
	s.referenceCalls++
	return s.ConversationRepository.GetActiveByReference(ctx, ref)
}

func newConversationTestHandler(t *testing.T) (*Handler, *memory.ConversationRepo, *sessionStub) {
	t.Helper()
	h, _, sessions := newTestHandler(t)
	conversations := memory.NewConversationRepo()
	h.deps.Conversations = conversations
	return h, conversations, sessions
}

func authenticatedConversationRequest(t *testing.T, method, path string, sessions *sessionStub, accountID uuid.UUID) *http.Request {
	t.Helper()
	tokens, err := sessions.IssueSession(context.Background(), accountID, accountauth.SessionMetadata{})
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	req := httptest.NewRequest(method, path, nil)
	req.AddCookie(&http.Cookie{Name: accessCookieName, Value: tokens.AccessToken})
	return req
}

func safeConversationCreateRequest(t *testing.T, sessions *sessionStub, accountID uuid.UUID, idempotencyKey string) *http.Request {
	t.Helper()
	req := authenticatedConversationRequest(t, http.MethodPost, "/web/v1/conversations", sessions, accountID)
	req.Header.Set("Origin", "https://app.example.test")
	req.Header.Set("X-CSRF-Token", "csrf")
	req.Header.Set("X-Idempotency-Key", idempotencyKey)
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
	return req
}

func assertSafeConversationList(t *testing.T, body []byte) []uuid.UUID {
	t.Helper()
	var response map[string]json.RawMessage
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response) != 1 || response["items"] == nil {
		t.Fatalf("response fields = %v, want only items", response)
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(response["items"], &items); err != nil {
		t.Fatalf("decode items: %v", err)
	}
	if items == nil {
		t.Fatal("items must be an empty array, not null")
	}
	ids := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		ids = append(ids, safeConversationFields(t, item))
	}
	return ids
}

func safeConversationDTO(t *testing.T, body []byte) uuid.UUID {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatalf("decode conversation DTO: %v", err)
	}
	return safeConversationFields(t, fields)
}

func safeConversationFields(t *testing.T, fields map[string]json.RawMessage) uuid.UUID {
	t.Helper()
	forbidden := []string{"account_id", "user_id", "source", "external_thread_id", "vk_peer_id", "messages", "message_data"}
	for _, field := range forbidden {
		if _, ok := fields[field]; ok {
			t.Fatalf("response exposes forbidden field %q: %v", field, fields)
		}
	}
	if len(fields) != 4 || fields["id"] == nil || fields["title"] == nil || fields["created_at"] == nil || fields["updated_at"] == nil {
		t.Fatalf("response fields = %v, want id, title, created_at, updated_at", fields)
	}
	var idRaw, title string
	if err := json.Unmarshal(fields["id"], &idRaw); err != nil {
		t.Fatalf("decode id: %v", err)
	}
	id, err := uuid.Parse(idRaw)
	if err != nil || id == uuid.Nil {
		t.Fatalf("id = %q, want UUID: %v", idRaw, err)
	}
	if err := json.Unmarshal(fields["title"], &title); err != nil {
		t.Fatalf("decode title: %v", err)
	}
	for field, raw := range map[string]json.RawMessage{"created_at": fields["created_at"], "updated_at": fields["updated_at"]} {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatalf("decode %s: %v", field, err)
		}
		if _, err := time.Parse(time.RFC3339, value); err != nil {
			t.Fatalf("%s = %q, want RFC3339: %v", field, value, err)
		}
	}
	return id
}

type passwordStub struct {
	resolution domain.IdentityResolution
	calls      int
}

func (s *passwordStub) AuthenticateEmailPassword(context.Context, string, string) (domain.IdentityResolution, error) {
	s.calls++
	if s.resolution.AccountID == uuid.Nil {
		return domain.IdentityResolution{}, errors.New("invalid credentials")
	}
	return s.resolution, nil
}

type profileStub struct{ lastAccountID uuid.UUID }

func (s *profileStub) Profile(_ context.Context, accountID uuid.UUID) (accountservice.AccountProfile, error) {
	s.lastAccountID = accountID
	return accountservice.AccountProfile{AccountID: accountID}, nil
}

type sessionStub struct {
	*accountauth.Service
	refreshCalls int
	logoutCalls  int
	logoutErr    error
}

func (s *sessionStub) RefreshSession(ctx context.Context, refresh string, meta accountauth.SessionMetadata) (accountauth.SessionTokens, error) {
	s.refreshCalls++
	return s.Service.RefreshSession(ctx, refresh, meta)
}

func (s *sessionStub) Logout(ctx context.Context, refresh string) error {
	s.logoutCalls++
	if s.logoutErr != nil {
		return s.logoutErr
	}
	return s.Service.Logout(ctx, refresh)
}

func newTestHandler(t *testing.T) (*Handler, *passwordStub, *sessionStub) {
	t.Helper()
	repo := memory.NewAccountSessionRepo()
	sessions := &sessionStub{Service: accountauth.New(nil, accountauth.WithSessionRepository(repo), accountauth.WithSessionTTL(time.Hour), accountauth.WithAccessTokenTTL(time.Hour))}
	passwords := &passwordStub{}
	profiles := &profileStub{}
	return NewHandler(Config{WebOrigin: "https://app.example.test"}, Deps{Passwords: passwords, Sessions: sessions, Authenticator: sessions, Account: profiles}), passwords, sessions
}

func cookieMap(cookies []*http.Cookie) map[string]*http.Cookie {
	out := make(map[string]*http.Cookie, len(cookies))
	for _, cookie := range cookies {
		out[cookie.Name] = cookie
	}
	return out
}

func TestSafeSessionResponseIsJSONObject(t *testing.T) {
	h, passwords, _ := newTestHandler(t)
	passwords.resolution = domain.IdentityResolution{AccountID: uuid.New()}
	req := httptest.NewRequest(http.MethodPost, "/web/v1/auth/password/login", strings.NewReader(`{"email":"owner@example.test","password":"pw"}`))
	req.Header.Set("Origin", "https://app.example.test")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := response["session"]; !ok {
		t.Fatalf("safe session DTO missing: %v", response)
	}
}
