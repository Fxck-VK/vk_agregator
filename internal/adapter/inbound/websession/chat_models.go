package websession

import (
	"net/http"
	"sort"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/modelcatalog"
)

type publicChatModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (h *Handler) listChatModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if _, ok := PrincipalFromContext(r.Context()); !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	defaultModel, ok := modelcatalog.ResolvePublicModel(domain.OperationTextGenerate, "")
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "chat models unavailable")
		return
	}
	items := make([]publicChatModel, 0)
	for _, model := range modelcatalog.ListMiniAppModels(domain.OperationTextGenerate) {
		items = append(items, publicChatModel{ID: model.ModelID, Name: model.ModelName})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ID == defaultModel.ModelID {
			return items[j].ID != defaultModel.ModelID
		}
		if items[j].ID == defaultModel.ModelID {
			return false
		}
		return items[i].ID < items[j].ID
	})
	writeJSON(w, http.StatusOK, struct {
		Items          []publicChatModel `json:"items"`
		DefaultModelID string            `json:"default_model_id"`
	}{Items: items, DefaultModelID: defaultModel.ModelID})
}
