package websession

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/service/legalacceptance"
)

type LegalConsentService interface {
	ValidateRegistration(*legalacceptance.RegistrationNotice) error
	AcceptRegistration(context.Context, uuid.UUID, string, *legalacceptance.RegistrationNotice) error
	Status(context.Context, uuid.UUID) (legalacceptance.Status, error)
	Accept(context.Context, uuid.UUID, legalacceptance.AcceptInput) (legalacceptance.Status, error)
	UpdateMarketing(context.Context, uuid.UUID, legalacceptance.MarketingInput) (legalacceptance.Status, error)
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
	if errors.Is(err, legalacceptance.ErrVersion) {
		w.Header().Set("X-NeiroHub-Legal-Revision", "changed")
	}
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
