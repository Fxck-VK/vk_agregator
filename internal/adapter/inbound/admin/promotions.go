package admin

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"io"
	"net/http"
	"time"
	"vk-ai-aggregator/internal/domain"
)

type promotionConfigRequest struct {
	Code      string     `json:"code"`
	Percent   int        `json:"percent"`
	Enabled   bool       `json:"enabled"`
	Revision  int64      `json:"revision"`
	StartsAt  time.Time  `json:"starts_at"`
	EndsAt    *time.Time `json:"ends_at"`
	MaxUses   int64      `json:"max_uses"`
	PlanCodes []string   `json:"plan_codes"`
}

func (h *Handler) listPromotions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.deps.Promotions == nil {
		writeError(w, 503, "promotions unavailable")
		return
	}
	limit, offset := parsePagination(r)
	items, e := h.deps.Promotions.List(r.Context(), limit+1, offset)
	if e != nil {
		writePromotionError(w, e)
		return
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	writeJSON(w, 200, struct {
		Items   []domain.Promotion `json:"items"`
		HasMore bool               `json:"has_more"`
	}{items, more})
}
func (h *Handler) mutatePromotion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.deps.Promotions == nil {
		writeError(w, 503, "promotions unavailable")
		return
	}
	key, e := uuid.Parse(r.Header.Get("X-Idempotency-Key"))
	if e != nil || key == uuid.Nil {
		writeError(w, 400, "invalid idempotency key")
		return
	}
	var id uuid.UUID
	if r.PathValue("id") != "" {
		id, e = uuid.Parse(r.PathValue("id"))
		if e != nil || id == uuid.Nil {
			writeError(w, 400, "invalid promotion id")
			return
		}
	}
	var body promotionConfigRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&body); e != nil {
		writeError(w, 400, "invalid promotion details")
		return
	}
	if e = decoder.Decode(&struct{}{}); !errors.Is(e, io.EOF) {
		writeError(w, 400, "invalid promotion details")
		return
	}
	p := &domain.Promotion{ID: id, Code: body.Code, Percent: body.Percent, Enabled: body.Enabled, Revision: body.Revision, StartsAt: body.StartsAt, EndsAt: body.EndsAt, MaxUses: body.MaxUses, PlanCodes: body.PlanCodes}
	result, e := h.deps.Promotions.Mutate(r.Context(), operatorIdentityFromContext(r.Context()).ActorRef, key, p)
	if e != nil {
		writePromotionError(w, e)
		return
	}
	writeJSON(w, 200, result)
}
func writePromotionError(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, domain.ErrPromotionInvalid):
		writeError(w, 400, "invalid promotion details")
	case errors.Is(e, domain.ErrConflict):
		writeError(w, 409, "promotion revision or request conflicts")
	case errors.Is(e, domain.ErrNotFound):
		writeError(w, 404, "promotion not found")
	default:
		writeError(w, 503, "promotions unavailable")
	}
}
