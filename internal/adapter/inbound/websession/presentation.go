package websession

import (
	"net/http"
	"slices"

	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/textgeneration"
)

var presentationTextModels = []string{
	providermodels.PublicTextGemini37Flash,
	providermodels.PublicTextGemini36Flash,
	"qwen_3_7_flash",
}

func presentationTextModel(id string) bool {
	return slices.Contains(presentationTextModels, id)
}

func (h *Handler) requirePresentationPreview(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Cache-Control", "no-store")
	if h.releaseProfile(r) == "dev" {
		return true
	}
	writeError(w, http.StatusNotFound, "presentation unavailable")
	return false
}

func (h *Handler) quotePresentation(w http.ResponseWriter, r *http.Request) {
	if !h.requirePresentationPreview(w, r) {
		return
	}
	for _, id := range presentationTextModels {
		if !h.releaseModelAllowed(r, id, "text") {
			continue
		}
		model, quote, err := textgeneration.Resolve(id, "presentation pricing", h.cfg.TextModels, h.deps.ImagePricing)
		if err != nil {
			continue
		}
		quote, err = pricingcatalog.PresentationTextSnapshot(quote)
		if err != nil {
			continue
		}
		writeJSON(w, http.StatusOK, struct {
			ID      string `json:"model_id"`
			Name    string `json:"model_name"`
			Credits int64  `json:"credits"`
		}{model.ModelID, model.ModelName, quote.InternalCredits})
		return
	}
	writeError(w, http.StatusServiceUnavailable, "presentation unavailable")
}

func (h *Handler) generatePresentation(w http.ResponseWriter, r *http.Request) {
	if h.requirePresentationPreview(w, r) {
		h.createTextOrMediaMessage(w, r, true)
	}
}
