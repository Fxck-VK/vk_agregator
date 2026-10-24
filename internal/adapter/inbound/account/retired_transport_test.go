package account

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
)

func TestAccountAPIRejectsRetiredVKLaunchAuthentication(t *testing.T) {
	h, _ := newTestHandler(t)
	for _, header := range []string{"X-VK-User-ID", "X-Launch-Params"} {
		r := httptest.NewRequest(http.MethodGet, "/account/me", nil)
		r.Header.Set(header, "123456789")
		w := httptest.NewRecorder()
		h.Routes().ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("retired %s established an account: status %d", header, w.Code)
		}
	}
}

func authorizeAccountRequest(t *testing.T, h *Handler, request *http.Request, vkID string) {
	t.Helper()
	owner, err := h.deps.Identity.ResolveOrCreate(context.Background(), domain.IdentityProviderVK, vkID)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := h.deps.Sessions.IssueSession(context.Background(), owner.AccountID, accountauth.SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
}

func TestAccountAPIAcceptsExistingVerifiedAccountSession(t *testing.T) {
	h, svc := newTestHandler(t)
	owner, err := svc.auth.ResolveVKID(context.Background(), 123456789)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := svc.auth.IssueSession(context.Background(), owner.AccountID, accountauth.SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/account/me", nil)
	r.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("verified account session rejected: status %d", w.Code)
	}
}
