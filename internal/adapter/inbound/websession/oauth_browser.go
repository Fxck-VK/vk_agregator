package websession

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/accountoauth"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountservice"
)

const oauthBindingCookie = "__Host-nh-oauth"

type BrowserOAuthService interface {
	Providers() []string
	Start(context.Context, string, string, string, string, string) (accountoauth.BrowserStart, error)
	Complete(context.Context, string, string, url.Values) (accountoauth.BrowserTransaction, domain.VerifiedAccountLogin, error)
}
type BrowserOAuthLogins interface {
	ResolveOrCreate(context.Context, domain.VerifiedAccountLogin) (domain.IdentityResolution, error)
}
type browserIdentityLinker interface {
	LinkVerifiedIdentity(context.Context, uuid.UUID, uuid.UUID, domain.VerifiedAccountLogin) (accountservice.AccountIdentitySafe, error)
}

func (h *Handler) browserOAuthStart(w http.ResponseWriter, r *http.Request) {
	if !h.requireOrigin(w, r) {
		return
	}
	h.beginOAuth(w, r, "login", "")
}
func (h *Handler) browserOAuthLinkStart(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFromContext(r.Context())
	h.beginOAuth(w, r, "link", principal.AccountID.String())
}
func (h *Handler) beginOAuth(w http.ResponseWriter, r *http.Request, intent, accountID string) {
	if h.deps.BrowserOAuth == nil || h.deps.OAuthLogins == nil || h.deps.Sessions == nil {
		writeError(w, 503, "authentication unavailable")
		return
	}
	var req struct {
		Locale string `json:"locale"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	binding := ""
	if cookie, err := r.Cookie(oauthBindingCookie); err == nil && len(cookie.Value) == 43 {
		binding = cookie.Value
	}
	if binding == "" {
		var err error
		binding, err = newCSRFToken()
		if err != nil {
			writeError(w, 503, "authentication unavailable")
			return
		}
	}
	start, err := h.deps.BrowserOAuth.Start(r.Context(), r.PathValue("provider"), binding, req.Locale, intent, accountID)
	if err != nil {
		writeError(w, 503, "authentication unavailable")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: oauthBindingCookie, Value: binding, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: int(accountoauth.BrowserTransactionTTL.Seconds()), Expires: time.Now().Add(accountoauth.BrowserTransactionTTL)})
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, start)
}

func (h *Handler) browserOAuthCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	failure := func(locale, intent string) {
		if locale != "en" {
			locale = "ru"
		}
		path := "/" + locale + "/login?oauth=failed"
		if intent == "link" {
			path = "/" + locale + "/app/profile?oauth=failed"
		}
		http.Redirect(w, r, path, 303)
	}
	if h.deps.BrowserOAuth == nil || h.deps.OAuthLogins == nil || h.deps.Sessions == nil {
		failure("ru", "login")
		return
	}
	binding, err := r.Cookie(oauthBindingCookie)
	if err != nil {
		failure("ru", "login")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, login, err := h.deps.BrowserOAuth.Complete(ctx, r.PathValue("provider"), binding.Value, r.URL.Query())
	if err != nil {
		failure(tx.Locale, tx.Intent)
		return
	}
	if tx.Intent == "link" {
		linker, ok := h.deps.Account.(browserIdentityLinker)
		if !ok || h.deps.Authenticator == nil {
			failure(tx.Locale, tx.Intent)
			return
		}
		cookie, err := r.Cookie(accessCookieName)
		if err != nil {
			failure(tx.Locale, tx.Intent)
			return
		}
		principal, err := h.deps.Authenticator.AuthenticateAccessToken(ctx, cookie.Value)
		if err != nil || principal.Validate() != nil || principal.AccountID.String() != tx.AccountID {
			failure(tx.Locale, tx.Intent)
			return
		}
		if _, err := linker.LinkVerifiedIdentity(ctx, principal.AccountID, principal.AccountID, login); err != nil {
			failure(tx.Locale, tx.Intent)
			return
		}
		http.Redirect(w, r, "/"+tx.Locale+"/app/profile?oauth=linked", 303)
		return
	}
	resolution, err := h.deps.OAuthLogins.ResolveOrCreate(ctx, login)
	if err != nil || resolution.AccountID == uuid.Nil {
		failure(tx.Locale, tx.Intent)
		return
	}
	tokens, err := h.deps.Sessions.IssueSession(ctx, resolution.AccountID, sessionMetadata(r, ""))
	if err != nil {
		failure(tx.Locale, tx.Intent)
		return
	}
	if err := h.setSessionCookies(w, tokens); err != nil {
		failure(tx.Locale, tx.Intent)
		return
	}
	_ = h.acceptPendingReferral(w, r, resolution.AccountID)
	http.Redirect(w, r, "/"+tx.Locale+"/app", 303)
}
