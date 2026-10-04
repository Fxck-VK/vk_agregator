package account

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
)

func TestBrowserCannotUnlinkLastLoginWhenOnlyPhoneBindingWouldRemain(t *testing.T) {
	h, services := newTestHandler(t)
	account, _ := services.auth.ResolveOrCreate(context.Background(), domain.VerifiedAccountLogin{Method: domain.AccountLoginEmailPassword, ExternalID: "member@example.test", Verified: true})
	services.account.LinkVerifiedIdentity(context.Background(), account.AccountID, account.AccountID, domain.VerifiedAccountLogin{Method: domain.AccountLoginPhoneOTP, ExternalID: "+79991234567", Verified: true})
	refs, _ := services.account.ListIdentities(context.Background(), account.AccountID, 50, 0)
	for _, ref := range refs {
		if ref.Provider == domain.IdentityProviderEmail {
			r := httptest.NewRequest("DELETE", "/web/v1/account/identities/"+ref.ID.String(), nil)
			r.SetPathValue("id", ref.ID.String())
			rec := httptest.NewRecorder()
			h.ServeBrowserAccountAction(rec, r, domain.RequestPrincipal{AccountID: account.AccountID, SessionID: uuid.New(), Method: domain.AuthenticationMethodAccountSession})
			if rec.Code != 409 {
				t.Fatalf("last browser login removed: status %d", rec.Code)
			}
		}
	}
}

func TestBrowserRecoveryUsesExistingAccountAndRevokesSessions(t *testing.T) {
	h, services := newTestHandler(t)
	login := domain.VerifiedAccountLogin{Method: domain.AccountLoginEmailPassword, ExternalID: "owner@example.test", Verified: true}
	account, err := services.auth.ResolveOrCreate(context.Background(), login)
	if err != nil {
		t.Fatal(err)
	}
	if err := services.auth.SetPasswordForVerifiedEmail(context.Background(), account.AccountID, account.AccountID, login.ExternalID, "old-password"); err != nil {
		t.Fatal(err)
	}
	session, _ := services.auth.IssueSession(context.Background(), account.AccountID, accountauth.SessionMetadata{})
	request := httptest.NewRequest("POST", "/web/v1/auth/password/request-reset", strings.NewReader(`{"email":"owner@example.test"}`))
	known := httptest.NewRecorder()
	h.ServePasswordRecovery(known, request)
	unknown := httptest.NewRecorder()
	h.ServePasswordRecovery(unknown, httptest.NewRequest("POST", "/web/v1/auth/password/request-reset", strings.NewReader(`{"email":"unknown@example.test"}`)))
	if known.Code != 202 || known.Code != unknown.Code || known.Body.String() != unknown.Body.String() {
		t.Fatal("email existence disclosed")
	}
	code := services.emailSender.waitEmailCode(t)
	rec := httptest.NewRecorder()
	h.ServePasswordRecovery(rec, httptest.NewRequest("POST", "/web/v1/auth/password/reset", strings.NewReader(`{"email":"owner@example.test","code":"`+code+`","new_password":"new-password"}`)))
	if rec.Code != 204 {
		t.Fatalf("reset: %d", rec.Code)
	}
	if _, err := services.auth.AuthenticateEmailPassword(context.Background(), login.ExternalID, "new-password"); err != nil {
		t.Fatal("new password not usable")
	}
	if _, err := services.auth.AuthenticateAccessToken(context.Background(), session.AccessToken); err == nil {
		t.Fatal("old session still valid")
	}
	if strings.Contains(rec.Body.String(), code) || strings.Contains(rec.Body.String(), "password") {
		t.Fatal("proof leaked")
	}
}
