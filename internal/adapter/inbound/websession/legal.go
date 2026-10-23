package websession

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/service/legalacceptance"
)

type LegalConsentService interface {
	Status(context.Context, uuid.UUID) (legalacceptance.Status, error)
	Accept(context.Context, uuid.UUID, legalacceptance.AcceptInput) (legalacceptance.Status, error)
	UpdateMarketing(context.Context, uuid.UUID, legalacceptance.MarketingInput) (legalacceptance.Status, error)
}

// Consent is checked before new work or checkout, never before security,
// withdrawal, reading existing results or deleting user content.
func (h *Handler) requireCurrentLegal(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		needsChoice := r.Method == http.MethodPost && (path == "/web/v1/conversations" ||
			(strings.HasPrefix(path, "/web/v1/conversations/") && strings.HasSuffix(path, "/messages")) ||
			path == "/web/v1/payments/intents" || path == "/web/v1/input-artifacts" ||
			path == "/web/v1/music-inputs" || strings.HasPrefix(path, "/web/v1/image-jobs/") ||
			strings.HasPrefix(path, "/web/v1/video-jobs/") || strings.HasPrefix(path, "/web/v1/music-jobs/") ||
			strings.HasPrefix(path, "/web/v1/speech-jobs/"))
		if needsChoice && h.deps.Legal != nil {
			principal, _ := PrincipalFromContext(r.Context())
			status, err := h.deps.Legal.Status(r.Context(), principal.AccountID)
			if err != nil {
				writeError(w, http.StatusServiceUnavailable, "legal settings unavailable")
				return
			}
			if status.Published && !status.Accepted {
				writeError(w, http.StatusPreconditionRequired, "current terms must be accepted")
				return
			}
		}
		next(w, r)
	}
}

func (h *Handler) legalStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.deps.Legal == nil {
		writeError(w, 503, "legal settings unavailable")
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	value, err := h.deps.Legal.Status(r.Context(), principal.AccountID)
	if err != nil {
		legalError(w, err)
		return
	}
	writeJSON(w, 200, value)
}
func (h *Handler) legalChoice(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.deps.Legal == nil {
		writeError(w, 503, "legal settings unavailable")
		return
	}
	requestID, err := uuid.Parse(r.Header.Get("X-Idempotency-Key"))
	if err != nil || requestID == uuid.Nil {
		writeError(w, 400, "invalid idempotency key")
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	var out legalacceptance.Status
	if r.URL.Path == "/web/v1/account/legal/accept" {
		var in legalacceptance.AcceptInput
		if !decodeJSON(w, r, &in) {
			return
		}
		in.RequestID = requestID
		out, err = h.deps.Legal.Accept(r.Context(), principal.AccountID, in)
	} else {
		var in struct {
			RecipientVersion string `json:"recipient_version"`
			Version          string `json:"version"`
			Checksum         string `json:"checksum"`
			Locale           string `json:"locale"`
			OptIn            *bool  `json:"opt_in"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		if in.OptIn == nil {
			writeError(w, 400, "explicit choice required")
			return
		}
		out, err = h.deps.Legal.UpdateMarketing(r.Context(), principal.AccountID, legalacceptance.MarketingInput{RequestID: requestID, Version: in.Version, Checksum: in.Checksum, Locale: in.Locale, OptIn: *in.OptIn, RecipientVersion: in.RecipientVersion})
	}
	if err != nil {
		legalError(w, err)
		return
	}
	writeJSON(w, 200, out)
}
func legalError(w http.ResponseWriter, err error) {
	status := 503
	switch {
	case errors.Is(err, legalacceptance.ErrInvalid):
		status = 400
	case errors.Is(err, legalacceptance.ErrVersion), errors.Is(err, legalacceptance.ErrReplay):
		status = 409
	case errors.Is(err, legalacceptance.ErrEmail):
		status = 422
	}
	writeError(w, status, "legal choice could not be saved")
}
