package account

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/accountoauth"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/accountlink"
	"vk-ai-aggregator/internal/service/accountservice"
	"vk-ai-aggregator/internal/service/identityresolver"
)

func TestAccountAPIProfileAndIdentitiesAreSafe(t *testing.T) {
	h, service := newTestHandler(t)
	ctx := context.Background()

	vk, err := service.auth.ResolveVKID(ctx, 123456789)
	if err != nil {
		t.Fatalf("resolve vk: %v", err)
	}
	if _, err := service.account.LinkVerifiedIdentity(ctx, vk.AccountID, vk.AccountID, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginEmailPassword,
		ExternalID: "Owner.Name@Example.COM",
		Verified:   true,
	}); err != nil {
		t.Fatalf("link email: %v", err)
	}
	if _, err := service.account.LinkVerifiedIdentity(ctx, vk.AccountID, vk.AccountID, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginPhoneOTP,
		ExternalID: "+7 (999) 123-45-67",
		Verified:   true,
	}); err != nil {
		t.Fatalf("link phone: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/account/me", nil)
	req.Header.Set("X-VK-User-ID", "123456789")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	assertNoRawPII(t, rec.Body.String())

	req = httptest.NewRequest(http.MethodGet, "/account/identities", nil)
	req.Header.Set("X-VK-User-ID", "123456789")
	rec = httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	assertNoRawPII(t, rec.Body.String())
	if !strings.Contains(rec.Body.String(), "o***@example.com") || !strings.Contains(rec.Body.String(), "****4567") {
		t.Fatalf("safe labels missing from body: %s", rec.Body.String())
	}
}

func TestAccountAPIRejectsUnauthenticatedRequest(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/account/me", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAccountAPILaunchParamsTransportIsHeaderOnlyByDefault(t *testing.T) {
	h, _ := newTestHandler(t)
	const secret = "account-launch-transport-test-secret"
	h.cfg = Config{AppSecret: secret, LaunchParamsMaxAge: time.Hour}
	launchParams := signedAccountLaunchParams(t, 123456789, secret)

	queryReq := httptest.NewRequest(http.MethodGet, "/account/me?launch_params="+url.QueryEscape(launchParams), nil)
	queryRec := httptest.NewRecorder()
	h.Routes().ServeHTTP(queryRec, queryReq)
	if queryRec.Code != http.StatusUnauthorized {
		t.Fatalf("query launch params status = %d, want 401", queryRec.Code)
	}

	headerReq := httptest.NewRequest(http.MethodGet, "/account/me", nil)
	headerReq.Header.Set("X-Launch-Params", launchParams)
	headerRec := httptest.NewRecorder()
	h.Routes().ServeHTTP(headerRec, headerReq)
	if headerRec.Code != http.StatusOK {
		t.Fatalf("header launch params status = %d, want 200, body = %s", headerRec.Code, headerRec.Body.String())
	}
}

func TestAccountAPIQueryLaunchParamsRequireExplicitOptIn(t *testing.T) {
	h, _ := newTestHandler(t)
	const secret = "account-query-opt-in-test-secret"
	h.cfg = Config{
		AppSecret:              secret,
		LaunchParamsMaxAge:     time.Hour,
		AllowQueryLaunchParams: true,
	}
	launchParams := signedAccountLaunchParams(t, 123456789, secret)
	req := httptest.NewRequest(http.MethodGet, "/account/me?launch_params="+url.QueryEscape(launchParams), nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("explicit query opt-in status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAccountAPIUnlinkIdentity(t *testing.T) {
	h, service := newTestHandler(t)
	ctx := context.Background()
	vk, err := service.auth.ResolveVKID(ctx, 777)
	if err != nil {
		t.Fatalf("resolve vk: %v", err)
	}
	email, err := service.account.LinkVerifiedIdentity(ctx, vk.AccountID, vk.AccountID, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginEmailPassword,
		ExternalID: "owner@example.com",
		Verified:   true,
	})
	if err != nil {
		t.Fatalf("link email: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/account/identities/"+email.ID.String(), nil)
	req.SetPathValue("id", email.ID.String())
	req.Header.Set("X-VK-User-ID", "777")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body = %s", rec.Code, rec.Body.String())
	}
	items, err := service.account.ListIdentities(ctx, vk.AccountID, 10, 0)
	if err != nil {
		t.Fatalf("list identities: %v", err)
	}
	for _, item := range items {
		if item.ID == email.ID {
			t.Fatalf("identity was not unlinked: %+v", items)
		}
	}
}

func TestAccountAPILinkEndpointFailsClosedWithoutVerifiedProof(t *testing.T) {
	h, _ := newTestHandler(t)

	body := strings.NewReader(`{"method":"email_password","external_id":"owner@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/account/identities/link", body)
	req.Header.Set("X-VK-User-ID", "555")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionRequired {
		t.Fatalf("status = %d, want 428, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "owner@example.com") {
		t.Fatalf("link error leaked raw external id: %s", rec.Body.String())
	}
}

func TestAccountAPILinkEndpointDoesNotTrustClientVerifiedFlag(t *testing.T) {
	h, _ := newTestHandler(t)

	body := strings.NewReader(`{"method":"email_password","external_id":"owner@example.com","verified":true}`)
	req := httptest.NewRequest(http.MethodPost, "/account/identities/link", body)
	req.Header.Set("X-VK-User-ID", "555")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for unknown verified field, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "owner@example.com") {
		t.Fatalf("link error leaked raw external id: %s", rec.Body.String())
	}
}

func TestAccountAPIEmailLinkCodeFlow(t *testing.T) {
	h, service := newTestHandler(t)

	req := httptest.NewRequest(
		http.MethodPost,
		"/account/identities/email/request-code",
		strings.NewReader(`{"email":"Owner.Name@Example.COM"}`),
	)
	req.Header.Set("X-VK-User-ID", "555")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("request-code status = %d, want 202, body = %s", rec.Code, rec.Body.String())
	}
	if service.emailSender.code == "" {
		t.Fatal("verification code was not sent")
	}
	assertNoRawPII(t, rec.Body.String())

	verifyBody := fmt.Sprintf(`{"email":"owner.name@example.com","code":%q}`, service.emailSender.code)
	req = httptest.NewRequest(http.MethodPost, "/account/identities/email/verify", strings.NewReader(verifyBody))
	req.Header.Set("X-VK-User-ID", "555")
	rec = httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("verify status = %d, want 201, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "o***@example.com") {
		t.Fatalf("safe email label missing: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), service.emailSender.code) {
		t.Fatalf("verify response leaked code: %s", rec.Body.String())
	}
	assertNoRawPII(t, rec.Body.String())
}

func TestAccountAPIPhoneLinkOTPFlow(t *testing.T) {
	h, service := newTestHandler(t)

	req := httptest.NewRequest(
		http.MethodPost,
		"/account/identities/phone/request-otp",
		strings.NewReader(`{"phone":"+7 (999) 123-45-67"}`),
	)
	req.Header.Set("X-VK-User-ID", "555")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("request-otp status = %d, want 202, body = %s", rec.Code, rec.Body.String())
	}
	if service.emailSender.phoneCode == "" {
		t.Fatal("phone OTP was not sent")
	}
	assertNoRawPII(t, rec.Body.String())

	verifyBody := fmt.Sprintf(`{"phone":"+79991234567","code":%q}`, service.emailSender.phoneCode)
	req = httptest.NewRequest(http.MethodPost, "/account/identities/phone/verify", strings.NewReader(verifyBody))
	req.Header.Set("X-VK-User-ID", "555")
	rec = httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("verify status = %d, want 201, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "****4567") {
		t.Fatalf("safe phone label missing: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), service.emailSender.phoneCode) {
		t.Fatalf("verify response leaked OTP: %s", rec.Body.String())
	}
	assertNoRawPII(t, rec.Body.String())
}

func TestAccountAPIEmailLinkRejectsWrongCode(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(
		http.MethodPost,
		"/account/identities/email/request-code",
		strings.NewReader(`{"email":"owner@example.com"}`),
	)
	req.Header.Set("X-VK-User-ID", "555")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("request-code status = %d, want 202, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(
		http.MethodPost,
		"/account/identities/email/verify",
		strings.NewReader(`{"email":"owner@example.com","code":"000000"}`),
	)
	req.Header.Set("X-VK-User-ID", "555")
	rec = httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("verify status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
	assertNoRawPII(t, rec.Body.String())
}

func TestAccountAPIPhoneLinkRejectsWrongCode(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(
		http.MethodPost,
		"/account/identities/phone/request-otp",
		strings.NewReader(`{"phone":"+7 (999) 123-45-67"}`),
	)
	req.Header.Set("X-VK-User-ID", "555")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("request-otp status = %d, want 202, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(
		http.MethodPost,
		"/account/identities/phone/verify",
		strings.NewReader(`{"phone":"+79991234567","code":"000000"}`),
	)
	req.Header.Set("X-VK-User-ID", "555")
	rec = httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("verify status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
	assertNoRawPII(t, rec.Body.String())
}

func TestAccountAPIEmailLinkExpiredCode(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	h, service := newTestHandlerWithEmailConfig(t, accountlink.Config{
		CodeTTL: time.Minute,
		Now: func() time.Time {
			return now
		},
	})
	service.emailStore.SetNow(func() time.Time { return now })

	req := httptest.NewRequest(
		http.MethodPost,
		"/account/identities/email/request-code",
		strings.NewReader(`{"email":"owner@example.com"}`),
	)
	req.Header.Set("X-VK-User-ID", "555")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("request-code status = %d, want 202, body = %s", rec.Code, rec.Body.String())
	}

	now = now.Add(2 * time.Minute)
	req = httptest.NewRequest(
		http.MethodPost,
		"/account/identities/email/verify",
		strings.NewReader(fmt.Sprintf(`{"email":"owner@example.com","code":%q}`, service.emailSender.code)),
	)
	req.Header.Set("X-VK-User-ID", "555")
	rec = httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusGone {
		t.Fatalf("verify status = %d, want 410, body = %s", rec.Code, rec.Body.String())
	}
	assertNoRawPII(t, rec.Body.String())
}

func TestAccountAPIEmailLinkRequestRateLimited(t *testing.T) {
	h, _ := newTestHandlerWithEmailConfig(t, accountlink.Config{
		RequestLimit:  1,
		RequestWindow: time.Minute,
	})

	for i, want := range []int{http.StatusAccepted, http.StatusTooManyRequests} {
		req := httptest.NewRequest(
			http.MethodPost,
			"/account/identities/email/request-code",
			strings.NewReader(`{"email":"owner@example.com"}`),
		)
		req.Header.Set("X-VK-User-ID", "555")
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("request %d status = %d, want %d, body = %s", i+1, rec.Code, want, rec.Body.String())
		}
		assertNoRawPII(t, rec.Body.String())
	}
}

func TestAccountAPIPhoneLinkRequestRateLimited(t *testing.T) {
	h, _ := newTestHandlerWithEmailConfig(t, accountlink.Config{
		PhoneRequestLimit:  1,
		PhoneRequestWindow: time.Minute,
	})

	for i, want := range []int{http.StatusAccepted, http.StatusTooManyRequests} {
		req := httptest.NewRequest(
			http.MethodPost,
			"/account/identities/phone/request-otp",
			strings.NewReader(`{"phone":"+7 (999) 123-45-67"}`),
		)
		req.Header.Set("X-VK-User-ID", "555")
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("request %d status = %d, want %d, body = %s", i+1, rec.Code, want, rec.Body.String())
		}
		assertNoRawPII(t, rec.Body.String())
	}
}

func TestAccountAPISessionLifecycle(t *testing.T) {
	h, _ := newTestHandler(t)
	router := h.Routes()

	issueReq := httptest.NewRequest(http.MethodPost, "/account/sessions", strings.NewReader(`{"device_info":"Chrome on Windows"}`))
	issueReq.Header.Set("X-VK-User-ID", "555")
	issueReq.Header.Set("X-Forwarded-For", "203.0.113.10")
	issueReq.Header.Set("User-Agent", "UnitTest/1.0")
	issueRec := httptest.NewRecorder()
	router.ServeHTTP(issueRec, issueReq)
	if issueRec.Code != http.StatusCreated {
		t.Fatalf("issue status = %d, body = %s", issueRec.Code, issueRec.Body.String())
	}
	assertNoRawSessionMaterial(t, issueRec.Body.String())
	var issued accountauth.SessionTokens
	if err := json.Unmarshal(issueRec.Body.Bytes(), &issued); err != nil {
		t.Fatalf("decode issued session: %v", err)
	}
	if issued.AccessToken == "" || issued.RefreshToken == "" || issued.Session.ID == uuid.Nil {
		t.Fatalf("incomplete session response: %+v", issued)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/account/sessions", nil)
	listReq.Header.Set("X-VK-User-ID", "555")
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRec.Code, listRec.Body.String())
	}
	assertNoRawSessionMaterial(t, listRec.Body.String())

	refreshReq := httptest.NewRequest(http.MethodPost, "/account/sessions/refresh", strings.NewReader(fmt.Sprintf(`{"refresh_token":%q,"device_info":"Phone"}`, issued.RefreshToken)))
	refreshRec := httptest.NewRecorder()
	router.ServeHTTP(refreshRec, refreshReq)
	if refreshRec.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, body = %s", refreshRec.Code, refreshRec.Body.String())
	}
	var refreshed accountauth.SessionTokens
	if err := json.Unmarshal(refreshRec.Body.Bytes(), &refreshed); err != nil {
		t.Fatalf("decode refreshed session: %v", err)
	}
	if refreshed.RefreshToken == "" || refreshed.RefreshToken == issued.RefreshToken {
		t.Fatalf("refresh token was not rotated: old=%q new=%q", issued.RefreshToken, refreshed.RefreshToken)
	}

	revokeReq := httptest.NewRequest(http.MethodPost, "/account/sessions/"+refreshed.Session.ID.String()+"/revoke", nil)
	revokeReq.Header.Set("X-VK-User-ID", "555")
	revokeRec := httptest.NewRecorder()
	router.ServeHTTP(revokeRec, revokeReq)
	if revokeRec.Code != http.StatusOK {
		t.Fatalf("revoke status = %d, body = %s", revokeRec.Code, revokeRec.Body.String())
	}
	if !strings.Contains(revokeRec.Body.String(), `"revoked":true`) {
		t.Fatalf("revoke response did not mark session revoked: %s", revokeRec.Body.String())
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/account/sessions/logout", strings.NewReader(fmt.Sprintf(`{"refresh_token":%q}`, refreshed.RefreshToken)))
	logoutRec := httptest.NewRecorder()
	router.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, body = %s", logoutRec.Code, logoutRec.Body.String())
	}
}

func TestAccountAPIPasswordSetAndLogin(t *testing.T) {
	h, service := newTestHandler(t)
	ctx := context.Background()
	vk, err := service.auth.ResolveVKID(ctx, 555)
	if err != nil {
		t.Fatalf("resolve vk: %v", err)
	}
	if _, err := service.account.LinkVerifiedIdentity(ctx, vk.AccountID, vk.AccountID, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginEmailPassword,
		ExternalID: "owner@example.com",
		Verified:   true,
	}); err != nil {
		t.Fatalf("link email: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/account/password/set", strings.NewReader(`{"email":"owner@example.com","password":"correct horse battery staple"}`))
	req.Header.Set("X-VK-User-ID", "555")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("set password status = %d, body = %s", rec.Code, rec.Body.String())
	}
	credential, err := service.security.FindCredential(ctx, vk.AccountID, domain.AccountCredentialPassword)
	if err != nil {
		t.Fatalf("find credential: %v", err)
	}
	if strings.Contains(credential.SecretHash, "correct horse") || credential.SecretHash == "" {
		t.Fatalf("password was not stored as one-way hash: %q", credential.SecretHash)
	}

	req = httptest.NewRequest(http.MethodPost, "/account/password/login", strings.NewReader(`{"email":"owner@example.com","password":"correct horse battery staple","device_info":"browser"}`))
	rec = httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("login status = %d, body = %s", rec.Code, rec.Body.String())
	}
	assertNoRawPasswordMaterial(t, rec.Body.String())
	var tokens accountauth.SessionTokens
	if err := json.Unmarshal(rec.Body.Bytes(), &tokens); err != nil {
		t.Fatalf("decode password login tokens: %v", err)
	}
	if tokens.RefreshToken == "" || tokens.Session.AccountID != vk.AccountID {
		t.Fatalf("bad session from password login: %+v", tokens)
	}

	audits := service.security.AuditEntries()
	if len(audits) < 2 {
		t.Fatalf("expected password audit entries, got %+v", audits)
	}
	if audits[0].Action != domain.AccountLinkActionPasswordSet || audits[1].Action != domain.AccountLinkActionLogin {
		t.Fatalf("unexpected audit actions: %+v", audits)
	}
}

func TestAccountAPIPasswordLoginRejectsWrongPassword(t *testing.T) {
	h, service := newTestHandler(t)
	ctx := context.Background()
	vk, err := service.auth.ResolveVKID(ctx, 555)
	if err != nil {
		t.Fatalf("resolve vk: %v", err)
	}
	if _, err := service.account.LinkVerifiedIdentity(ctx, vk.AccountID, vk.AccountID, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginEmailPassword,
		ExternalID: "owner@example.com",
		Verified:   true,
	}); err != nil {
		t.Fatalf("link email: %v", err)
	}
	if err := service.auth.SetPasswordForVerifiedEmail(ctx, vk.AccountID, vk.AccountID, "owner@example.com", "correct horse battery staple"); err != nil {
		t.Fatalf("set password: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/account/password/login", strings.NewReader(`{"email":"owner@example.com","password":"wrong password"}`))
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("login status = %d, want 401, body = %s", rec.Code, rec.Body.String())
	}
	assertNoRawPasswordMaterial(t, rec.Body.String())
}

func TestAccountAPIPasswordResetUsesEmailCode(t *testing.T) {
	h, service := newTestHandler(t)
	ctx := context.Background()
	vk, err := service.auth.ResolveVKID(ctx, 555)
	if err != nil {
		t.Fatalf("resolve vk: %v", err)
	}
	if _, err := service.account.LinkVerifiedIdentity(ctx, vk.AccountID, vk.AccountID, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginEmailPassword,
		ExternalID: "owner@example.com",
		Verified:   true,
	}); err != nil {
		t.Fatalf("link email: %v", err)
	}
	if err := service.auth.SetPasswordForVerifiedEmail(ctx, vk.AccountID, vk.AccountID, "owner@example.com", "old password value"); err != nil {
		t.Fatalf("set password: %v", err)
	}
	oldTokens, err := service.auth.IssueSession(ctx, vk.AccountID, accountauth.SessionMetadata{DeviceInfo: "before-reset"})
	if err != nil {
		t.Fatalf("issue old session: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/account/password/request-reset", strings.NewReader(`{"email":"owner@example.com"}`))
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("request reset status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.emailSender.code == "" {
		t.Fatal("reset code was not sent")
	}
	assertNoRawPasswordMaterial(t, rec.Body.String())

	resetBody := fmt.Sprintf(`{"email":"owner@example.com","code":%q,"new_password":"new password value"}`, service.emailSender.code)
	req = httptest.NewRequest(http.MethodPost, "/account/password/reset", strings.NewReader(resetBody))
	rec = httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("reset status = %d, body = %s", rec.Code, rec.Body.String())
	}

	refreshBody := fmt.Sprintf(`{"refresh_token":%q}`, oldTokens.RefreshToken)
	req = httptest.NewRequest(http.MethodPost, "/account/sessions/refresh", strings.NewReader(refreshBody))
	rec = httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("old refresh status after reset = %d, want 401, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/account/password/login", strings.NewReader(`{"email":"owner@example.com","password":"old password value"}`))
	rec = httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("old password login status = %d, want 401, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/account/password/login", strings.NewReader(`{"email":"owner@example.com","password":"new password value"}`))
	rec = httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("new password login status = %d, body = %s", rec.Code, rec.Body.String())
	}
	audits := service.security.AuditEntries()
	foundReset := false
	for _, audit := range audits {
		if audit.Action == domain.AccountLinkActionPasswordReset {
			foundReset = true
		}
	}
	if !foundReset {
		t.Fatalf("password reset audit was not recorded: %+v", audits)
	}
}

func TestAccountAPIOAuthLoginIssuesSession(t *testing.T) {
	h, _ := newTestHandler(t)
	h.deps.OAuth = fakeOAuthVerifier{
		login: domain.VerifiedAccountLogin{
			Method:     domain.AccountLoginGoogle,
			ExternalID: "google-subject-123",
			Verified:   true,
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/account/oauth/login", strings.NewReader(`{"provider":"google","id_token":"raw-id-token","device_info":"browser"}`))
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("oauth login status = %d, body = %s", rec.Code, rec.Body.String())
	}
	assertNoRawOAuthMaterial(t, rec.Body.String())
	var tokens accountauth.SessionTokens
	if err := json.Unmarshal(rec.Body.Bytes(), &tokens); err != nil {
		t.Fatalf("decode oauth tokens: %v", err)
	}
	if tokens.RefreshToken == "" || tokens.Session.AccountID == uuid.Nil {
		t.Fatalf("bad oauth session: %+v", tokens)
	}
}

func TestAccountAPIOAuthLinkAttachesSafeIdentity(t *testing.T) {
	h, _ := newTestHandler(t)
	h.deps.OAuth = fakeOAuthVerifier{
		login: domain.VerifiedAccountLogin{
			Method:     domain.AccountLoginGoogle,
			ExternalID: "google-subject-456",
			Verified:   true,
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/account/oauth/link", strings.NewReader(`{"provider":"google","id_token":"raw-id-token"}`))
	req.Header.Set("X-VK-User-ID", "555")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("oauth link status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"provider":"google"`) || !strings.Contains(rec.Body.String(), `"label":"Google"`) {
		t.Fatalf("safe google identity missing: %s", rec.Body.String())
	}
	assertNoRawOAuthMaterial(t, rec.Body.String())
}

func TestAccountAPIOAuthLinkConflictRequiresMergeConfirmation(t *testing.T) {
	h, service := newTestHandler(t)
	ctx := context.Background()
	owner, err := service.auth.ResolveVKID(ctx, 1001)
	if err != nil {
		t.Fatalf("resolve owner vk: %v", err)
	}
	if _, err := service.account.LinkVerifiedIdentity(ctx, owner.AccountID, owner.AccountID, domain.VerifiedAccountLogin{
		Method:     domain.AccountLoginGoogle,
		ExternalID: "google-subject-conflict",
		Verified:   true,
	}); err != nil {
		t.Fatalf("link google to owner: %v", err)
	}
	h.deps.OAuth = fakeOAuthVerifier{
		login: domain.VerifiedAccountLogin{
			Method:     domain.AccountLoginGoogle,
			ExternalID: "google-subject-conflict",
			Verified:   true,
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/account/oauth/link", strings.NewReader(`{"provider":"google","id_token":"raw-id-token"}`))
	req.Header.Set("X-VK-User-ID", "2002")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("oauth conflict status = %d, want 409, body = %s", rec.Code, rec.Body.String())
	}
	assertNoRawOAuthMaterial(t, rec.Body.String())
	resolved, err := service.auth.ResolveVerifiedGoogleSubject(ctx, "google-subject-conflict")
	if err != nil {
		t.Fatalf("resolve google after conflict: %v", err)
	}
	if resolved.AccountID != owner.AccountID {
		t.Fatalf("conflict moved google identity to account %s, want %s", resolved.AccountID, owner.AccountID)
	}
}

func TestAccountAPIOAuthRejectsInvalidAssertionWithoutLeak(t *testing.T) {
	h, _ := newTestHandler(t)
	h.deps.OAuth = fakeOAuthVerifier{err: accountoauth.ErrInvalidAssertion}

	req := httptest.NewRequest(http.MethodPost, "/account/oauth/login", strings.NewReader(`{"provider":"google","id_token":"raw-id-token"}`))
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("oauth status = %d, want 401, body = %s", rec.Code, rec.Body.String())
	}
	assertNoRawOAuthMaterial(t, rec.Body.String())
}

type testServices struct {
	auth        *accountauth.Service
	account     *accountservice.Service
	emailStore  *accountlink.MemoryStore
	emailSender *captureEmailSender
	security    *memory.AccountSecurityRepo
}

func assertNoRawSessionMaterial(t *testing.T, body string) {
	t.Helper()
	for _, forbidden := range []string{
		"Chrome",
		"Windows",
		"203.0.113.10",
		"UnitTest",
		"refresh_token_hash",
		"device_id",
		"ip_hash",
		"user_agent_hash",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response leaked session material %q: %s", forbidden, body)
		}
	}
}

func assertNoRawPasswordMaterial(t *testing.T, body string) {
	t.Helper()
	for _, forbidden := range []string{
		"correct horse",
		"old password",
		"new password",
		"password value",
		"secret_hash",
		"pbkdf2_sha256",
		"refresh_token_hash",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response leaked password material %q: %s", forbidden, body)
		}
	}
}

func assertNoRawOAuthMaterial(t *testing.T, body string) {
	t.Helper()
	for _, forbidden := range []string{
		"raw-id-token",
		"google-subject",
		"auth_data",
		"id_token",
		"device_info",
		"browser",
		"refresh_token_hash",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response leaked oauth material %q: %s", forbidden, body)
		}
	}
}

func newTestHandler(t *testing.T) (*Handler, testServices) {
	return newTestHandlerWithEmailConfig(t, accountlink.Config{})
}

func newTestHandlerWithEmailConfig(t *testing.T, emailCfg accountlink.Config) (*Handler, testServices) {
	t.Helper()
	users := memory.NewUserRepo()
	identities := memory.NewAccountIdentityRepo()
	security := memory.NewAccountSecurityRepo()
	resolver := identityresolver.New(users, identities, nil)
	auth := accountauth.New(resolver,
		accountauth.WithSessionRepository(memory.NewAccountSessionRepo()),
		accountauth.WithCredentialRepository(security),
		accountauth.WithAccountAuditRepository(security),
	)
	accountSvc := accountservice.New(identities, auth)
	emailStore := accountlink.NewMemoryStore()
	emailSender := &captureEmailSender{}
	if emailCfg.HashSecret == "" {
		emailCfg.HashSecret = "test-email-link-secret"
	}
	emailLinker, err := accountlink.New(emailStore, emailSender, accountSvc, emailCfg)
	if err != nil {
		t.Fatalf("new account email linker: %v", err)
	}
	return NewHandler(Config{}, Deps{
		Identity:  resolver,
		Account:   accountSvc,
		Logins:    auth,
		Sessions:  auth,
		Passwords: auth,
		OAuth:     fakeOAuthVerifier{err: accountoauth.ErrUnavailable},
		Linker:    emailLinker,
	}), testServices{auth: auth, account: accountSvc, emailStore: emailStore, emailSender: emailSender, security: security}
}

type fakeOAuthVerifier struct {
	login domain.VerifiedAccountLogin
	err   error
}

func signedAccountLaunchParams(t *testing.T, vkUserID int64, secret string) string {
	t.Helper()
	values := url.Values{
		"vk_app_id":   {"123456"},
		"vk_platform": {"desktop_web"},
		"vk_ts":       {strconv.FormatInt(time.Now().Unix(), 10)},
		"vk_user_id":  {strconv.FormatInt(vkUserID, 10)},
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+url.QueryEscape(values.Get(key)))
	}
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(strings.Join(parts, "&"))); err != nil {
		t.Fatalf("sign launch params: %v", err)
	}
	values.Set("sign", base64.RawURLEncoding.EncodeToString(mac.Sum(nil)))
	return values.Encode()
}

func (f fakeOAuthVerifier) Verify(_ context.Context, _ accountoauth.VerifyRequest) (domain.VerifiedAccountLogin, error) {
	if f.err != nil {
		return domain.VerifiedAccountLogin{}, f.err
	}
	return f.login, nil
}

type captureEmailSender struct {
	email          string
	code           string
	expiresAt      time.Time
	phone          string
	phoneCode      string
	phoneExpiresAt time.Time
}

func (s *captureEmailSender) SendEmailLinkCode(_ context.Context, email, code string, expiresAt time.Time) error {
	s.email = email
	s.code = code
	s.expiresAt = expiresAt
	return nil
}

func (s *captureEmailSender) SendPhoneLinkOTP(_ context.Context, phone, code string, expiresAt time.Time) error {
	s.phone = phone
	s.phoneCode = code
	s.phoneExpiresAt = expiresAt
	return nil
}

func assertNoRawPII(t *testing.T, body string) {
	t.Helper()
	var decoded any
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("response is not json: %v; body=%s", err, body)
	}
	for _, forbidden := range []string{
		"Owner.Name",
		"owner.name@example.com",
		"Owner.Name@Example.COM",
		"owner@example.com",
		"123456789",
		"123-45",
		"9991234567",
		"+79991234567",
		"+7 (999) 123-45-67",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response leaked %q: %s", forbidden, body)
		}
	}
}
