package account

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountlink"
	"vk-ai-aggregator/internal/service/accountservice"
)

type backupEmailLinker interface {
	RequestBackupEmailCode(context.Context, uuid.UUID, uuid.UUID, string) (accountlink.RequestResult, error)
	VerifyBackupEmailCode(context.Context, uuid.UUID, uuid.UUID, string, string) (accountservice.AccountIdentitySafe, error)
}

func writeEmailLinkError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, domain.ErrAccountEmailLimit):
		writeError(w, 409, "email_limit_reached")
	case errors.Is(err, domain.ErrAccountBackupEmailChanged):
		writeError(w, 409, "backup_email_changed")
	case errors.Is(err, domain.ErrAccountEmailAlreadyLinked):
		writeError(w, 409, "email_already_linked")
	default:
		writeError(w, statusForError(err), fallback)
	}
}

func (h *Handler) backupEmailAction(w http.ResponseWriter, r *http.Request, verify bool) {
	linker, ok := h.deps.Linker.(backupEmailLinker)
	if !ok {
		writeError(w, 503, "email verification unavailable")
		return
	}
	accountID, ok := accountIDFromContext(r.Context())
	if !ok {
		writeError(w, 401, "unauthorized")
		return
	}
	var req struct {
		IdentityID uuid.UUID `json:"identity_id"`
		Email      string    `json:"email"`
		Code       string    `json:"code,omitempty"`
	}
	if !decodeRequiredJSON(w, r, &req, "invalid email verification request") {
		return
	}
	if !verify {
		if req.Code != "" {
			writeError(w, 400, "invalid email verification request")
			return
		}
		result, err := linker.RequestBackupEmailCode(r.Context(), accountID, req.IdentityID, req.Email)
		if err != nil {
			writeEmailLinkError(w, err, "email verification unavailable")
			return
		}
		writeJSON(w, 202, result)
		return
	}
	identity, err := linker.VerifyBackupEmailCode(r.Context(), accountID, req.IdentityID, req.Email, req.Code)
	if err != nil {
		writeEmailLinkError(w, err, "email verification failed")
		return
	}
	writeJSON(w, 200, identity)
}
