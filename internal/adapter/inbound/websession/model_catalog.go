package websession

import (
	"net/http"
	"vk-ai-aggregator/internal/service/productcatalog"
)

func (h *Handler) workspaceModels() productcatalog.WorkspaceModelList {
	return productcatalog.WorkspaceCatalog(productcatalog.WorkspaceConfig{IncludePendingMedia: true, ImageReferenceUploads: h.deps.InputArtifacts != nil && h.deps.InputObjects != nil, TextModels: h.availableTextModels(), ImageModels: h.cfg.ImageModels, VideoRoutes: h.conversationVideoRoutes(), Pricing: h.deps.ImagePricing})
}

func (h *Handler) listModels(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, h.workspaceModels())
}
