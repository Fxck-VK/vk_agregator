package websession

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

type ArtifactDeletionService interface {
	Delete(context.Context, uuid.UUID, uuid.UUID) error
}

func (h *Handler) deleteArtifact(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, 401, "unauthorized")
		return
	}
	id, err := uuid.Parse(r.PathValue("artifactID"))
	if err != nil || id == uuid.Nil {
		writeError(w, 400, "invalid artifact id")
		return
	}
	if h.deps.ArtifactDeletion == nil {
		writeError(w, 503, "file deletion unavailable")
		return
	}
	if h.deps.ArtifactDeleteLimiter != nil {
		allowed, err := h.deps.ArtifactDeleteLimiter.Allow(r.Context(), principal.AccountID.String())
		if err != nil {
			writeError(w, 503, "file deletion unavailable")
			return
		}
		if !allowed {
			writeError(w, 429, "rate_limited")
			return
		}
	}
	err = h.deps.ArtifactDeletion.Delete(r.Context(), principal.AccountID, id)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, 404, "file not found")
	case errors.Is(err, domain.ErrConflict):
		writeError(w, 409, "file_in_use")
	case err != nil:
		writeError(w, 503, "file deletion unavailable")
	default:
		w.WriteHeader(http.StatusAccepted)
	}
}
