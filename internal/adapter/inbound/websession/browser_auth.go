package websession

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/accountlink"
	"vk-ai-aggregator/internal/service/browserauth"
)

const browserBindingCookie = "__Host-nh-browser"
const browserTrustCookie = "__Host-nh-trusted"
const browserHintCookie = "__Host-nh-last-account"
const browserChallengeCookie = "__Host-nh-login-proof"
const browserResetCookie = "__Host-nh-reset-proof"
const resetBindingHeader = "X-NeiroHub-Reset-Binding"

func resetBinding(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func cookieValue(r *http.Request, name string) string {
	if c, e := r.Cookie(name); e == nil {
		return c.Value
	}
	return ""
}
func clearAuthCookie(w http.ResponseWriter, name string) {
	c := sessionCookie(name, "", time.Unix(1, 0))
	c.MaxAge = -1
	http.SetCookie(w, c)
}
func browserBinding(w http.ResponseWriter, r *http.Request) (string, error) {
	if value := cookieValue(r, browserBindingCookie); len(value) == 43 {
		return value, nil
	}
	value, err := newCSRFToken()
	if err == nil {
		http.SetCookie(w, sessionCookie(browserBindingCookie, value, time.Now().Add(browserauth.RememberTTL)))
	}
	return value, err
}
func (h *Handler) browserPasswordLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.deps.BrowserAuth == nil {
		writeError(w, 503, "authentication unavailable")
		return
	}
	var req struct {
		Email      string `json:"email"`
		Password   string `json:"password"`
		Persistent *bool  `json:"persistent,omitempty"`
		Remembered bool   `json:"remembered,omitempty"`
	}
	if !decodeJSON(w, r, &req) || req.Password == "" {
		writeError(w, 400, "invalid request")
		return
	}
	if req.Remembered {
		var err error
		req.Email, err = h.deps.BrowserAuth.RememberedEmail(cookieValue(r, browserHintCookie))
		if err != nil {
			writeError(w, 401, "invalid credentials")
			return
		}
	}
	binding, err := browserBinding(w, r)
	if err != nil {
		writeError(w, 503, "authentication unavailable")
		return
	}
	meta := sessionMetadata(r, "")
	meta.Persistent = req.Persistent
	out, err := h.deps.BrowserAuth.Begin(r.Context(), req.Email, req.Password, binding, cookieValue(r, browserTrustCookie), r.UserAgent(), meta)
	if err != nil {
		browserAuthError(w, err)
		return
	}
	h.writeBrowserAuthOutcome(w, r, out)
}
func (h *Handler) writeBrowserAuthOutcome(w http.ResponseWriter, r *http.Request, out browserauth.Outcome) {
	if out.Challenge != "" {
		http.SetCookie(w, sessionCookie(browserChallengeCookie, out.Challenge, time.Now().Add(browserauth.ChallengeTTL)))
		writeJSON(w, 202, struct {
			Status  string `json:"status"`
			Expires int    `json:"expires_in_seconds"`
		}{"verification_required", 600})
		return
	}
	if out.Tokens == nil {
		writeError(w, 503, "authentication unavailable")
		return
	}
	if err := h.setSessionCookies(w, *out.Tokens); err != nil {
		writeError(w, 503, "authentication unavailable")
		return
	}
	clearAuthCookie(w, browserChallengeCookie)
	http.SetCookie(w, sessionCookie(browserTrustCookie, out.Trusted, time.Now().Add(browserauth.RememberTTL)))
	http.SetCookie(w, sessionCookie(browserHintCookie, out.Remembered, time.Now().Add(browserauth.RememberTTL)))
	w.Header().Set("X-NeiroHub-Account-ID", out.Tokens.Session.AccountID.String())
	writeJSON(w, 201, safeSessionResponse{Session: out.Tokens.Session})
}
func (h *Handler) browserAuthAction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != "GET" && !h.requireOrigin(w, r) {
		return
	}
	if h.deps.BrowserAuth == nil {
		writeError(w, 503, "authentication unavailable")
		return
	}
	switch r.URL.Path {
	case "/web/v1/auth/remembered":
		hint, provider, err := h.deps.BrowserAuth.HintInfo(cookieValue(r, browserHintCookie))
		if err != nil {
			writeJSON(w, 200, struct {
				EmailHint string `json:"email_hint"`
			}{""})
			return
		}
		writeJSON(w, 200, struct {
			EmailHint string `json:"email_hint"`
			Provider  string `json:"provider,omitempty"`
		}{hint, provider})
	case "/web/v1/auth/forget":
		for _, name := range []string{browserHintCookie, browserTrustCookie, browserChallengeCookie, browserBindingCookie} {
			clearAuthCookie(w, name)
		}
		w.WriteHeader(204)
	case "/web/v1/auth/password/verify-login":
		var req struct {
			Code string `json:"code"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		out, err := h.deps.BrowserAuth.Verify(r.Context(), cookieValue(r, browserChallengeCookie), cookieValue(r, browserBindingCookie), req.Code, r.UserAgent(), sessionMetadata(r, ""))
		if err != nil {
			browserAuthError(w, err)
			return
		}
		h.writeBrowserAuthOutcome(w, r, out)
	case "/web/v1/auth/password/resend-login":
		if err := h.deps.BrowserAuth.Resend(r.Context(), cookieValue(r, browserChallengeCookie), cookieValue(r, browserBindingCookie), r.UserAgent()); err != nil {
			browserAuthError(w, err)
			return
		}
		w.WriteHeader(202)
	case "/web/v1/auth/password/request-link":
		var req struct {
			Email  string `json:"email"`
			Locale string `json:"locale"`
		}
		if !decodeJSON(w, r, &req) || len(req.Email) > 254 || strings.TrimSpace(req.Email) == "" {
			writeError(w, 400, "invalid request")
			return
		}
		// Identical asynchronous response for known/unknown accounts and delivery
		// outcomes; a bounded worker prevents reset delivery exhausting HTTP workers.
		select {
		case h.recoverySlots <- struct{}{}:
			peer := r.RemoteAddr
			if host, _, err := net.SplitHostPort(peer); err == nil {
				peer = host
			}
			go func() {
				defer func() { <-h.recoverySlots }()
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				_ = h.deps.BrowserAuth.RequestReset(ctx, req.Email, req.Locale, peer)
			}()
		default:
		}
		writeJSON(w, 202, struct {
			Status string `json:"status"`
		}{"verification_sent"})
	case "/web/v1/auth/password/prepare-reset":
		var req struct {
			Token string `json:"token"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		expires, err := h.deps.BrowserAuth.InspectReset(r.Context(), req.Token)
		if err != nil {
			clearAuthCookie(w, browserResetCookie)
			browserAuthError(w, err)
			return
		}
		http.SetCookie(w, sessionCookie(browserResetCookie, req.Token, expires))
		writeJSON(w, 200, struct {
			Binding string `json:"binding"`
		}{resetBinding(req.Token)})
	case "/web/v1/auth/password/reset-status":
		token := cookieValue(r, browserResetCookie)
		if expected := r.Header.Get(resetBindingHeader); expected != "" && !constantTimeEqual(expected, resetBinding(token)) {
			writeError(w, 400, "reset context changed")
			return
		}
		if _, err := h.deps.BrowserAuth.InspectReset(r.Context(), token); err != nil {
			browserAuthError(w, err)
			return
		}
		writeJSON(w, 200, struct {
			Binding string `json:"binding"`
		}{resetBinding(token)})
	case "/web/v1/auth/password/reset-link":
		var req struct {
			NewPassword string `json:"new_password"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		token := cookieValue(r, browserResetCookie)
		if !constantTimeEqual(r.Header.Get(resetBindingHeader), resetBinding(token)) {
			writeError(w, 400, "reset context changed")
			return
		}
		if err := h.deps.BrowserAuth.Reset(r.Context(), token, req.NewPassword); err != nil {
			browserAuthError(w, err)
			return
		}
		h.expireSessionCookies(w)
		for _, name := range []string{browserResetCookie, browserTrustCookie, browserChallengeCookie} {
			clearAuthCookie(w, name)
		}
		w.WriteHeader(204)
	}
}
func browserAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, browserauth.ErrProof):
		writeError(w, 400, "invalid or expired verification")
	case errors.Is(err, accountauth.ErrInvalidPasswordLogin):
		writeError(w, 401, "invalid credentials")
	case errors.Is(err, accountauth.ErrWeakPassword):
		writeError(w, 400, "invalid password")
	case errors.Is(err, accountlink.ErrRateLimited) || errors.Is(err, accountauth.ErrRateLimited):
		w.Header().Set("Retry-After", "900")
		writeError(w, 429, "authentication rate limited")
	default:
		writeError(w, 503, "authentication unavailable")
	}
}
