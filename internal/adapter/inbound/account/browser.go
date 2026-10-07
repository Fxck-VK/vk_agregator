package account

import (
	"context"
	"net/http"

	"vk-ai-aggregator/internal/domain"
)

// ServePasswordRecovery exposes only proof-based password recovery. The browser
// adapter must validate Origin before calling this method. It never issues tokens.
func (h *Handler) ServePasswordRecovery(w http.ResponseWriter, r *http.Request) {
	switch r.Method + " " + r.URL.Path {
	case "POST /web/v1/auth/password/request-reset":
		h.requestPasswordReset(w, r)
	case "POST /web/v1/auth/password/reset":
		h.resetPassword(w, r)
	default:
		http.NotFound(w, r)
	}
}

// ServeBrowserAccountAction is an internal bridge, never an HTTP mount. Its
// caller supplies the principal established from the browser cookie; request
// headers and body account IDs cannot establish an identity here.
func (h *Handler) ServeBrowserAccountAction(w http.ResponseWriter, r *http.Request, principal domain.RequestPrincipal) {
	if principal.Validate() != nil {
		writeError(w, 401, "unauthorized")
		return
	}
	r = r.WithContext(context.WithValue(r.Context(), ctxAccountIDKey, principal.AccountID))
	r = r.WithContext(context.WithValue(r.Context(), ctxBrowserPrincipalKey, principal))
	switch r.Method + " " + r.URL.Path {
	case "POST /web/v1/account/identities/email/request-code":
		h.requestEmailCode(w, r)
	case "POST /web/v1/account/identities/email/verify":
		h.verifyEmailCode(w, r)
	case "POST /web/v1/account/identities/email/backup/request-code":
		h.backupEmailAction(w, r, false)
	case "POST /web/v1/account/identities/email/backup/verify":
		h.backupEmailAction(w, r, true)
	case "POST /web/v1/account/identities/phone/request-otp":
		h.requestPhoneOTP(w, r)
	case "POST /web/v1/account/identities/phone/verify":
		h.verifyPhoneOTP(w, r)
	case "POST /web/v1/account/password/set":
		h.setPassword(w, r)
	default:
		if r.Method == "DELETE" && r.PathValue("id") != "" && r.URL.Path == "/web/v1/account/identities/"+r.PathValue("id") {
			h.unlinkIdentity(w, r)
			return
		}
		http.NotFound(w, r)
	}
}
