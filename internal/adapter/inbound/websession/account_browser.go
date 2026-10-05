package websession

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountauth"
)

type BrowserAccountActions interface {
	ServePasswordRecovery(http.ResponseWriter, *http.Request)
	ServeBrowserAccountAction(http.ResponseWriter, *http.Request, domain.RequestPrincipal)
}

type BrowserSessionManager interface {
	ListActiveSessions(context.Context, uuid.UUID, int) ([]accountauth.AccountSessionSafe, error)
	RevokeSession(context.Context, uuid.UUID, uuid.UUID) (accountauth.AccountSessionSafe, error)
}

func (h *Handler) accountMethods(w http.ResponseWriter, r *http.Request) {
	providers := []string{}
	if h.deps.BrowserOAuth != nil {
		providers = h.deps.BrowserOAuth.Providers()
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, struct {
		Registration bool     `json:"registration"`
		Password     bool     `json:"password"`
		Recovery     bool     `json:"recovery"`
		EmailLink    bool     `json:"email_link"`
		PhoneLink    bool     `json:"phone_link"`
		Providers    []string `json:"providers"`
	}{h.cfg.EmailDeliveryEnabled && h.deps.EmailRegistration != nil, h.deps.Passwords != nil, h.cfg.EmailDeliveryEnabled && h.deps.AccountActions != nil, h.cfg.EmailDeliveryEnabled && h.deps.AccountActions != nil, h.cfg.PhoneDeliveryEnabled && h.deps.AccountActions != nil, providers})
}

func (h *Handler) browserPasswordRecovery(w http.ResponseWriter, r *http.Request) {
	if !h.requireOrigin(w, r) {
		return
	}
	if !h.cfg.EmailDeliveryEnabled || h.deps.AccountActions == nil {
		writeError(w, 503, "password recovery unavailable")
		return
	}
	h.deps.AccountActions.ServePasswordRecovery(w, r)
}

func (h *Handler) browserAccountAction(w http.ResponseWriter, r *http.Request) {
	if h.deps.AccountActions == nil {
		writeError(w, 503, "account unavailable")
		return
	}
	if (r.URL.Path == "/web/v1/account/identities/email/request-code" || r.URL.Path == "/web/v1/account/identities/email/verify" || r.URL.Path == "/web/v1/account/identities/email/backup/request-code" || r.URL.Path == "/web/v1/account/identities/email/backup/verify") && !h.cfg.EmailDeliveryEnabled {
		writeError(w, 503, "verification unavailable")
		return
	}
	if (r.URL.Path == "/web/v1/account/identities/phone/request-otp" || r.URL.Path == "/web/v1/account/identities/phone/verify") && !h.cfg.PhoneDeliveryEnabled {
		writeError(w, 503, "verification unavailable")
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	h.deps.AccountActions.ServeBrowserAccountAction(w, r, principal)
}

func (h *Handler) browserSessions(w http.ResponseWriter, r *http.Request) {
	manager, ok := h.deps.Sessions.(BrowserSessionManager)
	if !ok {
		writeError(w, 503, "sessions unavailable")
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	items, err := manager.ListActiveSessions(r.Context(), principal.AccountID, 100)
	if err != nil {
		writeError(w, 503, "sessions unavailable")
		return
	}
	type item struct {
		accountauth.AccountSessionSafe
		Current bool `json:"current"`
	}
	out := make([]item, 0, len(items))
	for _, session := range items {
		out = append(out, item{session, session.ID == principal.SessionID})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, struct {
		Items []item `json:"items"`
	}{out})
}

func (h *Handler) browserRevokeSession(w http.ResponseWriter, r *http.Request) {
	manager, ok := h.deps.Sessions.(BrowserSessionManager)
	if !ok {
		writeError(w, 503, "sessions unavailable")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil || id == uuid.Nil {
		writeError(w, 400, "invalid session")
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	_, err = manager.RevokeSession(r.Context(), principal.AccountID, id)
	if err != nil {
		status := 503
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, accountauth.ErrInvalidSession) {
			status = 404
		}
		writeError(w, status, "session revocation failed")
		return
	}
	if id == principal.SessionID {
		h.expireSessionCookies(w)
	}
	w.WriteHeader(204)
}
