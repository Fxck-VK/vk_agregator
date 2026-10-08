package websession

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/webreferralservice"
)

const referralCookieName = "__Host-nh-referral"
const referralCookieTTL = 30 * 24 * time.Hour

var referralTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

type WebReferrals interface {
	Summary(context.Context, uuid.UUID) (webreferralservice.Summary, error)
	Capture(context.Context, string, string, time.Time) error
	Accept(context.Context, string, uuid.UUID, time.Time) error
}

func referralCookieToken(r *http.Request) string {
	cookie, err := r.Cookie(referralCookieName)
	if err == nil && referralTokenPattern.MatchString(cookie.Value) {
		return cookie.Value
	}
	return ""
}
func referralTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (h *Handler) referralSummary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.deps.Referrals == nil {
		writeError(w, 503, "referrals unavailable")
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	summary, err := h.deps.Referrals.Summary(ctx, principal.AccountID)
	if err != nil {
		writeError(w, 503, "referrals unavailable")
		return
	}
	writeJSON(w, 200, summary)
}

func (h *Handler) referralVisit(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !h.requireOrigin(w, r) {
		return
	}
	if h.deps.Referrals == nil || h.deps.ReferralVisitLimiter == nil || h.deps.ReferralVisitClientLimiter == nil {
		writeError(w, 503, "referrals unavailable")
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	token := referralCookieToken(r)
	fresh := token == ""
	if fresh {
		var err error
		token, err = newCSRFToken()
		if err != nil {
			writeError(w, 503, "referrals unavailable")
			return
		}
	}
	// A browser that has exhausted its quota never consumes the shared budget.
	allowed, err := h.deps.ReferralVisitClientLimiter.Allow(ctx, referralTokenHash(token))
	if err != nil {
		writeError(w, 503, "referrals unavailable")
		return
	}
	if !allowed {
		w.Header().Set("Retry-After", "60")
		writeError(w, 429, "referrals rate limited")
		return
	}
	// Resetting cookies can bypass the browser bucket, so retain a global cap.
	allowed, err = h.deps.ReferralVisitLimiter.Allow(ctx, "web_referral_visits")
	if err != nil {
		writeError(w, 503, "referrals unavailable")
		return
	}
	if !allowed {
		w.Header().Set("Retry-After", "60")
		writeError(w, 429, "referrals rate limited")
		return
	}
	now := time.Now().UTC()
	if err := h.deps.Referrals.Capture(ctx, req.Code, referralTokenHash(token), now); err != nil {
		if errors.Is(err, domain.ErrExpired) {
			cookie := sessionCookie(referralCookieName, "", time.Unix(1, 0))
			cookie.MaxAge = -1
			http.SetCookie(w, cookie)
		}
		switch {
		case errors.Is(err, domain.ErrNotFound), errors.Is(err, domain.ErrInvalidIdentity), errors.Is(err, domain.ErrForbidden), errors.Is(err, domain.ErrExpired):
			writeError(w, 400, "invalid invitation")
		default:
			writeError(w, 503, "referrals unavailable")
		}
		return
	}
	if fresh {
		http.SetCookie(w, sessionCookie(referralCookieName, token, now.Add(referralCookieTTL)))
	}
	w.WriteHeader(204)
}

func (h *Handler) acceptPendingReferral(w http.ResponseWriter, r *http.Request, accountID uuid.UUID) error {
	token := referralCookieToken(r)
	if token == "" {
		return nil
	}
	if h.deps.Referrals == nil {
		return errors.New("referrals unavailable")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	err := h.deps.Referrals.Accept(ctx, referralTokenHash(token), accountID, time.Now().UTC())
	// A previous account, self-invitation or expired intent is terminal. An outage
	// leaves the cookie intact so a later authenticated POST can retry safely.
	if err == nil || errors.Is(err, domain.ErrConflict) || errors.Is(err, domain.ErrForbidden) || errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrInvalidIdentity) || errors.Is(err, domain.ErrExpired) {
		cookie := sessionCookie(referralCookieName, "", time.Unix(1, 0))
		cookie.MaxAge = -1
		http.SetCookie(w, cookie)
		return nil
	}
	return err
}

func (h *Handler) referralAccept(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, _ := PrincipalFromContext(r.Context())
	if err := h.acceptPendingReferral(w, r, principal.AccountID); err != nil {
		writeError(w, 503, "referrals unavailable")
		return
	}
	w.WriteHeader(204)
}
