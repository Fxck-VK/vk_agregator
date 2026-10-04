package account

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
)

func TestBrowserBackupEmailRecoversSameAccountAndSharedPassword(t *testing.T) {
	h, services := newTestHandler(t)
	ctx := context.Background()
	owner, err := services.auth.ResolveOrCreate(ctx, domain.VerifiedAccountLogin{Method: domain.AccountLoginEmailPassword, ExternalID: "owner@example.test", Verified: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := services.auth.SetPasswordForVerifiedEmail(ctx, owner.AccountID, owner.AccountID, "owner@example.test", "original-password"); err != nil {
		t.Fatal(err)
	}
	principal := domain.RequestPrincipal{AccountID: owner.AccountID, SessionID: uuid.New(), Method: domain.AuthenticationMethodAccountSession}
	request := func(path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRecorder()
		h.ServeBrowserAccountAction(r, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)), principal)
		return r
	}
	if r := request("/web/v1/account/identities/email/request-code", `{"email":"backup@example.test"}`); r.Code != http.StatusAccepted {
		t.Fatalf("backup code request: %d", r.Code)
	}
	linkCode := services.emailSender.waitEmailCode(t)
	if r := request("/web/v1/account/identities/email/verify", fmt.Sprintf(`{"email":"backup@example.test","code":%q}`, linkCode)); r.Code != http.StatusCreated {
		t.Fatalf("backup verification: %d", r.Code)
	} else if strings.Contains(r.Body.String(), "backup@example.test") || strings.Contains(r.Body.String(), linkCode) {
		t.Fatal("verification response exposed raw email or proof")
	}
	for _, email := range []string{"owner@example.test", "backup@example.test"} {
		resolution, err := services.auth.AuthenticateEmailPassword(ctx, email, "original-password")
		if err != nil || resolution.AccountID != owner.AccountID {
			t.Fatal("linked email did not retain the original account and password")
		}
	}
	reset := func(code string) *httptest.ResponseRecorder {
		r := httptest.NewRecorder()
		h.ServePasswordRecovery(r, httptest.NewRequest(http.MethodPost, "/web/v1/auth/password/reset", strings.NewReader(fmt.Sprintf(`{"email":"backup@example.test","code":%q,"new_password":"replacement-password"}`, code))))
		return r
	}
	if r := reset(linkCode); r.Code >= 200 && r.Code < 300 {
		t.Fatal("used backup-link code reset the password")
	}
	session, err := services.auth.IssueSession(ctx, owner.AccountID, accountauth.SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	services.emailSender.mu.Lock()
	services.emailSender.code = ""
	services.emailSender.mu.Unlock()
	r := httptest.NewRecorder()
	h.ServePasswordRecovery(r, httptest.NewRequest(http.MethodPost, "/web/v1/auth/password/request-reset", strings.NewReader(`{"email":"backup@example.test"}`)))
	if r.Code != http.StatusAccepted {
		t.Fatalf("backup recovery request: %d", r.Code)
	}
	code := services.emailSender.waitEmailCode(t)
	services.emailSender.mu.RLock()
	destination := services.emailSender.email
	services.emailSender.mu.RUnlock()
	if destination != "backup@example.test" {
		t.Fatal("recovery code was not delivered to the backup address")
	}
	if r := reset("invalid-code"); r.Code >= 200 && r.Code < 300 {
		t.Fatal("invalid recovery proof was accepted")
	}
	if _, err := services.auth.AuthenticateEmailPassword(ctx, "owner@example.test", "original-password"); err != nil {
		t.Fatal("invalid proof changed the original password")
	}
	if r := reset(code); r.Code != http.StatusNoContent {
		t.Fatalf("backup password reset: %d", r.Code)
	} else if r.Body.Len() != 0 {
		t.Fatal("password recovery returned credential material")
	}
	for _, email := range []string{"owner@example.test", "backup@example.test"} {
		resolution, err := services.auth.AuthenticateEmailPassword(ctx, email, "replacement-password")
		if err != nil || resolution.AccountID != owner.AccountID {
			t.Fatal("backup recovery did not retain the original account")
		}
		if _, err := services.auth.AuthenticateEmailPassword(ctx, email, "original-password"); !errors.Is(err, accountauth.ErrInvalidPasswordLogin) {
			t.Fatal("old password remains usable")
		}
	}
	if _, err := services.auth.AuthenticateAccessToken(ctx, session.AccessToken); err == nil {
		t.Fatal("backup recovery did not revoke the old session")
	}
	if _, err := services.auth.RefreshSession(ctx, session.RefreshToken, accountauth.SessionMetadata{}); err == nil {
		t.Fatal("backup recovery left the old refresh token usable")
	}
	if r := reset(code); r.Code >= 200 && r.Code < 300 {
		t.Fatal("recovery code was reusable")
	}
	profile, err := services.account.Profile(ctx, owner.AccountID)
	if err != nil || len(profile.IdentityRefs) != 2 || profile.AccountID != owner.AccountID {
		t.Fatal("backup recovery changed or duplicated linked identities")
	}
}

func TestBrowserUnconfirmedBackupEmailCannotRecover(t *testing.T) {
	h, services := newTestHandler(t)
	ctx := context.Background()
	owner, err := services.auth.ResolveOrCreate(ctx, domain.VerifiedAccountLogin{Method: domain.AccountLoginEmailPassword, ExternalID: "owner@example.test", Verified: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := services.auth.SetPasswordForVerifiedEmail(ctx, owner.AccountID, owner.AccountID, "owner@example.test", "original-password"); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	h.ServeBrowserAccountAction(r, httptest.NewRequest(http.MethodPost, "/web/v1/account/identities/email/request-code", strings.NewReader(`{"email":"unconfirmed@example.test"}`)), domain.RequestPrincipal{AccountID: owner.AccountID, SessionID: uuid.New(), Method: domain.AuthenticationMethodAccountSession})
	if r.Code != http.StatusAccepted {
		t.Fatalf("link code request: %d", r.Code)
	}
	code := services.emailSender.waitEmailCode(t)
	r = httptest.NewRecorder()
	h.ServePasswordRecovery(r, httptest.NewRequest(http.MethodPost, "/web/v1/auth/password/reset", strings.NewReader(fmt.Sprintf(`{"email":"unconfirmed@example.test","code":%q,"new_password":"replacement-password"}`, code))))
	if r.Code != http.StatusBadRequest {
		t.Fatalf("unconfirmed address recovered an account: %d", r.Code)
	}
	if _, err := h.deps.Identity.Resolve(ctx, domain.IdentityProviderEmail, "unconfirmed@example.test"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("recovery attached the unconfirmed address")
	}
	if _, err := services.auth.AuthenticateEmailPassword(ctx, "owner@example.test", "original-password"); err != nil {
		t.Fatal("unconfirmed address changed the owner's password")
	}
}

func TestBrowserBackupEmailCannotMoveFromAnotherAccount(t *testing.T) {
	h, services := newTestHandler(t)
	ctx := context.Background()
	owner, err := services.auth.ResolveOrCreate(ctx, domain.VerifiedAccountLogin{Method: domain.AccountLoginEmailPassword, ExternalID: "owner@example.test", Verified: true})
	if err != nil {
		t.Fatal(err)
	}
	other, err := services.auth.ResolveOrCreate(ctx, domain.VerifiedAccountLogin{Method: domain.AccountLoginEmailPassword, ExternalID: "occupied@example.test", Verified: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.deps.Linker.RequestEmailCode(ctx, owner.AccountID, "occupied@example.test"); err != nil {
		t.Fatal(err)
	}
	code := services.emailSender.waitEmailCode(t)
	r := httptest.NewRecorder()
	h.ServeBrowserAccountAction(r, httptest.NewRequest(http.MethodPost, "/web/v1/account/identities/email/verify", strings.NewReader(fmt.Sprintf(`{"email":"occupied@example.test","code":%q}`, code))), domain.RequestPrincipal{AccountID: owner.AccountID, SessionID: uuid.New(), Method: domain.AuthenticationMethodAccountSession})
	if r.Code != http.StatusConflict {
		t.Fatalf("conflicting backup accepted: %d", r.Code)
	}
	resolved, err := h.deps.Identity.Resolve(ctx, domain.IdentityProviderEmail, "occupied@example.test")
	if err != nil || resolved != other.AccountID {
		t.Fatal("backup verification moved another account's identity")
	}
}

func TestBrowserRemovedBackupCannotRecoverWithPreviouslyIssuedCode(t *testing.T) {
	h, services := newTestHandler(t)
	ctx := context.Background()
	owner, err := services.auth.ResolveOrCreate(ctx, domain.VerifiedAccountLogin{Method: domain.AccountLoginEmailPassword, ExternalID: "owner@example.test", Verified: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := services.auth.SetPasswordForVerifiedEmail(ctx, owner.AccountID, owner.AccountID, "owner@example.test", "original-password"); err != nil {
		t.Fatal(err)
	}
	backup, err := services.account.LinkVerifiedIdentity(ctx, owner.AccountID, owner.AccountID, domain.VerifiedAccountLogin{Method: domain.AccountLoginEmailPassword, ExternalID: "backup@example.test", Verified: true})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	h.ServePasswordRecovery(r, httptest.NewRequest(http.MethodPost, "/web/v1/auth/password/request-reset", strings.NewReader(`{"email":"backup@example.test"}`)))
	if r.Code != http.StatusAccepted {
		t.Fatalf("backup recovery request: %d", r.Code)
	}
	code := services.emailSender.waitEmailCode(t)
	if err := services.account.UnlinkIdentity(ctx, owner.AccountID, owner.AccountID, backup.ID); err != nil {
		t.Fatal(err)
	}
	r = httptest.NewRecorder()
	h.ServePasswordRecovery(r, httptest.NewRequest(http.MethodPost, "/web/v1/auth/password/reset", strings.NewReader(fmt.Sprintf(`{"email":"backup@example.test","code":%q,"new_password":"replacement-password"}`, code))))
	if r.Code != http.StatusBadRequest {
		t.Fatalf("removed backup recovered the account: %d", r.Code)
	}
	if _, err := services.auth.AuthenticateEmailPassword(ctx, "owner@example.test", "original-password"); err != nil {
		t.Fatal("removed backup changed the password")
	}
	if _, err := h.deps.Identity.Resolve(ctx, domain.IdentityProviderEmail, "backup@example.test"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("recovery reattached the removed backup")
	}
}

func TestBrowserRemovedBackupDuringRecoveryIsNotReattached(t *testing.T) {
	h, services := newTestHandler(t)
	ctx := context.Background()
	owner, err := services.auth.ResolveOrCreate(ctx, domain.VerifiedAccountLogin{Method: domain.AccountLoginEmailPassword, ExternalID: "owner@example.test", Verified: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := services.auth.SetPasswordForVerifiedEmail(ctx, owner.AccountID, owner.AccountID, "owner@example.test", "original-password"); err != nil {
		t.Fatal(err)
	}
	backup, err := services.account.LinkVerifiedIdentity(ctx, owner.AccountID, owner.AccountID, domain.VerifiedAccountLogin{Method: domain.AccountLoginEmailPassword, ExternalID: "backup@example.test", Verified: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.deps.Linker.RequestEmailCode(ctx, owner.AccountID, "backup@example.test"); err != nil {
		t.Fatal(err)
	}
	code := services.emailSender.waitEmailCode(t)
	resolver := h.deps.Identity
	h.deps.Identity = identityResolverOverride{IdentityResolver: resolver, resolve: func(ctx context.Context, provider domain.IdentityProvider, email string) (uuid.UUID, error) {
		id, err := resolver.Resolve(ctx, provider, email)
		if err == nil {
			err = services.account.UnlinkIdentity(ctx, owner.AccountID, owner.AccountID, backup.ID)
		}
		return id, err
	}}
	r := httptest.NewRecorder()
	h.ServePasswordRecovery(r, httptest.NewRequest(http.MethodPost, "/web/v1/auth/password/reset", strings.NewReader(fmt.Sprintf(`{"email":"backup@example.test","code":%q,"new_password":"replacement-password"}`, code))))
	if _, err := resolver.Resolve(ctx, domain.IdentityProviderEmail, "backup@example.test"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("recovery reattached a concurrently removed backup email")
	}
	if r.Code >= 200 && r.Code < 300 {
		t.Fatal("recovery accepted a concurrently removed email")
	}
	if _, err := services.auth.AuthenticateEmailPassword(ctx, "owner@example.test", "original-password"); err != nil {
		t.Fatal("concurrently removed backup changed the password")
	}
}
