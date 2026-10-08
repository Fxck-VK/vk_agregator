package websession

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
	"vk-ai-aggregator/internal/service/accountregistration"
)

const registrationCookieName = "__Host-nh-signup"

type EmailRegistrationService interface {
	Request(context.Context, string, string) (string, error)
	Verify(context.Context, string, string, string) error
	Complete(context.Context, string, string, string, accountauth.SessionMetadata, func(accountauth.SessionTokens) error) (accountauth.SessionTokens, error)
}

func (h *Handler) emailRegistration(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !h.requireOrigin(w, r) {
		return
	}
	if !h.cfg.EmailDeliveryEnabled || h.deps.EmailRegistration == nil {
		writeError(w, 503, "registration unavailable")
		return
	}
	var req struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	binding := ""
	if cookie, err := r.Cookie(registrationCookieName); err == nil {
		binding = cookie.Value
	}
	switch r.URL.Path {
	case "/web/v1/auth/email/request-code":
		peer := r.RemoteAddr
		if host, _, err := net.SplitHostPort(peer); err == nil {
			peer = host
		}
		value, err := h.deps.EmailRegistration.Request(r.Context(), req.Email, peer)
		if err != nil {
			registrationError(w, err)
			return
		}
		http.SetCookie(w, sessionCookie(registrationCookieName, value, time.Now().Add(accountregistration.TTL)))
		writeJSON(w, 202, struct {
			Status           string `json:"status"`
			ExpiresInSeconds int    `json:"expires_in_seconds"`
		}{"verification_sent", int(accountregistration.TTL.Seconds())})
	case "/web/v1/auth/email/verify-code":
		if err := h.deps.EmailRegistration.Verify(r.Context(), binding, req.Email, req.Code); err != nil {
			registrationError(w, err)
			return
		}
		w.WriteHeader(204)
	case "/web/v1/auth/email/register":
		tokens, err := h.deps.EmailRegistration.Complete(r.Context(), binding, req.Email, req.Password, sessionMetadata(r, ""), func(tokens accountauth.SessionTokens) error {
			return h.setSessionCookies(w, tokens)
		})
		if err != nil {
			registrationError(w, err)
			return
		}
		cookie := sessionCookie(registrationCookieName, "", time.Unix(1, 0))
		cookie.MaxAge = -1
		http.SetCookie(w, cookie)
		w.Header().Set("X-NeiroHub-Account-ID", tokens.Session.AccountID.String())
		_ = h.acceptPendingReferral(w, r, tokens.Session.AccountID)
		writeJSON(w, 201, safeSessionResponse{Session: tokens.Session})
	}
}
func registrationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, accountregistration.ErrRateLimited), errors.Is(err, accountauth.ErrRateLimited):
		w.Header().Set("Retry-After", "900")
		writeError(w, 429, "registration rate limited")
	case errors.Is(err, accountregistration.ErrInvalidProof):
		writeError(w, 400, "invalid or expired verification")
	case errors.Is(err, accountauth.ErrWeakPassword), errors.Is(err, domain.ErrInvalidIdentity):
		writeError(w, 400, "invalid registration request")
	case errors.Is(err, domain.ErrConflict):
		writeError(w, 409, "email already linked")
	default:
		writeError(w, 503, "registration unavailable")
	}
}
