package miniapp

import (
	"net/http"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

func (h *Handler) validateReferenceArtifacts(w http.ResponseWriter, r *http.Request, userID uuid.UUID, op domain.OperationType, ids []uuid.UUID) bool {
	if op != domain.OperationImageGenerate {
		writeError(w, http.StatusBadRequest, "reference_artifacts require image_generate")
		return false
	}
	if h.deps.Artifacts == nil {
		writeError(w, http.StatusServiceUnavailable, "artifact storage unavailable")
		return false
	}
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			writeError(w, http.StatusBadRequest, "invalid reference artifact id")
			return false
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		artifact, err := h.deps.Artifacts.GetByID(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "not found")
			return false
		}
		if artifact.OwnerUserID != userID {
			writeError(w, http.StatusNotFound, "not found")
			return false
		}
		if artifact.Kind != domain.ArtifactKindInput || artifact.MediaType != domain.MediaTypeImage || artifact.Status != domain.ArtifactStatusReady {
			writeError(w, http.StatusBadRequest, "invalid reference artifact")
			return false
		}
	}
	return true
}
