package websession

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"net/http"
	"vk-ai-aggregator/internal/domain"
)

type WebPromotionService interface {
	QuotePromotion(context.Context, uuid.UUID, string) (*domain.PromotionQuote, error)
}

func (h *Handler) quoteWebPromotion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	service, ok := h.deps.Subscriptions.(WebPromotionService)
	if !h.cfg.PaymentsEnabled || !ok {
		writeError(w, 503, "promotions unavailable")
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	owner, _ := PrincipalFromContext(r.Context())
	if h.deps.PaymentCreateLimiter != nil {
		allowed, e := h.deps.PaymentCreateLimiter.Allow(r.Context(), owner.AccountID.String())
		if e != nil {
			writeError(w, 503, "promotions unavailable")
			return
		}
		if !allowed {
			writeError(w, 429, "too many requests")
			return
		}
	}
	q, e := service.QuotePromotion(r.Context(), owner.AccountID, body.Code)
	if e != nil {
		switch {
		case errors.Is(e, domain.ErrPromotionInvalid):
			writeError(w, 400, "invalid promotion code")
		case errors.Is(e, domain.ErrConflict), errors.Is(e, domain.ErrNotFound):
			writeError(w, 409, "promotion unavailable for this purchase")
		default:
			writeError(w, 503, "promotions unavailable")
		}
		return
	}
	writeJSON(w, 200, q)
}
