package account

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
)

func TestBrowserPasswordReplacementRequiresCurrentPassword(t *testing.T) {
	for _, proof := range []string{"", "incorrect-password", "original-password"} {
		t.Run(proof, func(t *testing.T) {
			h, services := newTestHandler(t)
			ctx := context.Background()
			account, err := services.auth.ResolveVerifiedEmailPassword(ctx, "member@example.test")
			if err != nil {
				t.Fatal(err)
			}
			if err := services.auth.SetPasswordForVerifiedEmail(ctx, account.AccountID, account.AccountID, "member@example.test", "original-password"); err != nil {
				t.Fatal(err)
			}
			body := map[string]string{"email": "member@example.test", "password": "replacement-password"}
			if proof != "" {
				body["current_password"] = proof
			}
			payload, _ := json.Marshal(body)
			session, err := services.auth.IssueSession(ctx, account.AccountID, accountauth.SessionMetadata{})
			if err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			h.ServeBrowserAccountAction(rec, httptest.NewRequest("POST", "/web/v1/account/password/set", strings.NewReader(string(payload))), domain.RequestPrincipal{AccountID: account.AccountID, SessionID: session.Session.ID, Method: domain.AuthenticationMethodAccountSession})
			if proof == "original-password" {
				if rec.Code != 204 {
					t.Fatalf("confirmed change status %d, want 204", rec.Code)
				}
				if _, err := services.auth.AuthenticateEmailPassword(ctx, "member@example.test", "replacement-password"); err != nil {
					t.Fatal("new password not accepted")
				}
				if _, err := services.auth.AuthenticateEmailPassword(ctx, "member@example.test", "original-password"); err == nil {
					t.Fatal("old password still accepted")
				}
			} else {
				if rec.Code < 400 {
					t.Fatal("unconfirmed password replacement succeeded")
				}
				if _, err := services.auth.AuthenticateEmailPassword(ctx, "member@example.test", "original-password"); err != nil {
					t.Fatal("rejected change replaced original password")
				}
				if _, err := services.auth.AuthenticateEmailPassword(ctx, "member@example.test", "replacement-password"); err == nil {
					t.Fatal("rejected password accepted")
				}
			}
			if strings.Contains(rec.Body.String(), "original-password") || strings.Contains(rec.Body.String(), "replacement-password") {
				t.Fatal("response contains credentials")
			}
		})
	}
}

func TestBrowserPasswordChangePreservesPrincipalSessionOnly(t *testing.T) {
	h, services := newTestHandler(t)
	ctx := context.Background()
	owner, _ := services.auth.ResolveVerifiedEmailPassword(ctx, "member@example.test")
	if err := services.auth.SetPasswordForVerifiedEmail(ctx, owner.AccountID, owner.AccountID, "member@example.test", "original-password"); err != nil {
		t.Fatal(err)
	}
	current, _ := services.auth.IssueSession(ctx, owner.AccountID, accountauth.SessionMetadata{})
	other, _ := services.auth.IssueSession(ctx, owner.AccountID, accountauth.SessionMetadata{})
	payload := `{"email":"member@example.test","current_password":"original-password","password":"replacement-password","session_id":"` + other.Session.ID.String() + `"}`
	rec := httptest.NewRecorder()
	h.ServeBrowserAccountAction(rec, httptest.NewRequest("POST", "/web/v1/account/password/set", strings.NewReader(payload)), domain.RequestPrincipal{AccountID: owner.AccountID, SessionID: current.Session.ID, Method: domain.AuthenticationMethodAccountSession})
	// Unknown body fields fail validation; they can never select a preserved session.
	if rec.Code < 400 {
		t.Fatal("body session authority accepted")
	}
	payload = `{"email":"member@example.test","current_password":"original-password","password":"replacement-password"}`
	rec = httptest.NewRecorder()
	h.ServeBrowserAccountAction(rec, httptest.NewRequest("POST", "/web/v1/account/password/set", strings.NewReader(payload)), domain.RequestPrincipal{AccountID: owner.AccountID, SessionID: current.Session.ID, Method: domain.AuthenticationMethodAccountSession})
	if rec.Code != 204 {
		t.Fatalf("password change status: %d", rec.Code)
	}
	if _, err := services.auth.AuthenticateAccessToken(ctx, current.AccessToken); err != nil {
		t.Fatal("server principal session revoked")
	}
	if _, err := services.auth.AuthenticateAccessToken(ctx, other.AccessToken); err == nil {
		t.Fatal("other session preserved")
	}
}

type legacyOnlyPasswordService struct{ PasswordService }

func TestAccountPasswordLoginFailsClosedWithoutCombinedOperation(t *testing.T) {
	h, services := newTestHandler(t)
	h.deps.Passwords = legacyOnlyPasswordService{PasswordService: services.auth}
	w := httptest.NewRecorder()
	h.passwordLogin(w, httptest.NewRequest("POST", "/account/auth/password/login", strings.NewReader(`{"email":"member@example.test","password":"synthetic-password"}`)))
	if w.Code != 503 {
		t.Fatal("token login used split password/session fallback")
	}
}
