package websession

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/textgeneration"
	releasepolicy "vk-ai-aggregator/web/platform/release"
)

// The immutable baseline preserves existing admission, including legacy
// contracts. Manual DEV registry additions never enter this allowlist.
var productionReleaseModels = func() map[string]bool {
	models := map[string]bool{}
	for _, binding := range providermodels.StaticRegistry().Bindings() {
		models[binding.PublicID] = true
	}
	return models
}()

func (h *Handler) releaseProfile(r *http.Request) string {
	return releasepolicy.Profile(h.cfg.Environment, r.Header.Get(releasepolicy.Header))
}

func releaseModelFeature(id, kind string) string {
	switch kind {
	case "text":
		return "chat"
	case "image":
		return "images"
	case "video":
		return "videos"
	case "audio":
		switch id {
		case "whisper_1":
			return "transcription"
		case "gpt_4o_mini_tts":
			return "tts"
		default:
			return "music"
		}
	default:
		return ""
	}
}

func (h *Handler) releaseModelAllowed(r *http.Request, id, kind string) bool {
	profile := h.releaseProfile(r)
	if id == string(domain.VideoRouteKling26Motion) && profile != "dev" {
		return false
	}
	if !releasepolicy.Enabled(releaseModelFeature(id, kind), profile) {
		return false
	}
	return profile == "dev" || productionReleaseModels[id]
}

func (h *Handler) requireReleaseModel(w http.ResponseWriter, r *http.Request, id, kind string) bool {
	if h.releaseModelAllowed(r, strings.TrimSpace(id), kind) {
		return true
	}
	w.Header().Set("Cache-Control", "no-store")
	writeError(w, http.StatusForbidden, "operation unavailable in this release")
	return false
}

func (h *Handler) releaseTextModels(r *http.Request) []textgeneration.PublicModel {
	items := []textgeneration.PublicModel{}
	for _, model := range h.availableTextModels() {
		if h.releaseModelAllowed(r, model.ID, "text") {
			items = append(items, model)
		}
	}
	return items
}

func (h *Handler) releaseCatalog(r *http.Request) productcatalog.WorkspaceModelList {
	catalog := h.workspaceModels()
	if h.releaseProfile(r) == "dev" {
		return catalog
	}
	items := []productcatalog.WorkspaceModel{}
	for _, model := range catalog.Items {
		if model.Verification == "pending-verification" || model.Verification == "dev-smoke" || !h.releaseModelAllowed(r, model.ID, model.Kind) {
			continue
		}
		items = append(items, model)
	}
	catalog.Items = items
	if !h.releaseModelAllowed(r, catalog.DefaultModelID, "text") {
		catalog.DefaultModelID = ""
	}
	return catalog
}

// Activation/retry receive only a job ID. Resolve the owned stored model before
// invoking an orchestrator that may reserve credits or queue provider work.
func (h *Handler) requireReleaseImageJob(w http.ResponseWriter, r *http.Request, accountID, jobID uuid.UUID) bool {
	if h.releaseProfile(r) == "dev" {
		return true
	}
	if h.deps.ImageJobReader == nil {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return false
	}
	job, err := h.deps.ImageJobReader.GetByIDForAccount(r.Context(), accountID, jobID)
	if errors.Is(err, domain.ErrNotFound) || err == nil && (job == nil || job.ID != jobID || job.AccountID != accountID || job.Source != "web") {
		writeError(w, http.StatusNotFound, "image job not found")
		return false
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return false
	}
	safe, ok := newSafeImageJob(job)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "image generation unavailable")
		return false
	}
	return h.requireReleaseModel(w, r, safe.ModelID, "image")
}
