package websession

import (
	"net/http"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/textgeneration"
)

func (h *Handler) listChatModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if _, ok := PrincipalFromContext(r.Context()); !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Items          []textgeneration.PublicModel `json:"items"`
		DefaultModelID string                       `json:"default_model_id"`
	}{Items: h.availableTextModels(), DefaultModelID: providermodels.PublicTextChatGPT})
}

func (h *Handler) availableTextModels() []textgeneration.PublicModel {
	var ids []string
	for _, model := range h.cfg.TextModels {
		if model.ID != providermodels.PublicTextChatGPT {
			ids = append(ids, model.ID)
		}
	}
	return textgeneration.Models(ids, h.deps.ImagePricing)
}
