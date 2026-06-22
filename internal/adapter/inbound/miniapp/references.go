package miniapp

import (
	"bytes"
	"context"
	"image"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

const maxReferenceArtifacts = 4

func (h *Handler) validateReferenceArtifacts(w http.ResponseWriter, r *http.Request, userID uuid.UUID, op domain.OperationType, ids []uuid.UUID) bool {
	if op != domain.OperationImageGenerate && op != domain.OperationVideoGenerate {
		writeError(w, http.StatusBadRequest, "reference_artifacts require image_generate or video_generate")
		return false
	}
	if len(ids) > maxReferenceArtifacts {
		writeError(w, http.StatusBadRequest, "too many reference artifacts")
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

func (h *Handler) videoAspectRatioFromReferenceArtifacts(ctx context.Context, userID uuid.UUID, route VideoRouteDTO, ids []uuid.UUID) string {
	if h.deps.Artifacts == nil || len(ids) == 0 || len(route.AllowedAspectRatios) == 0 {
		return ""
	}
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		artifact, err := h.deps.Artifacts.GetByID(ctx, id)
		if err != nil || artifact == nil || artifact.OwnerUserID != userID {
			continue
		}
		if artifact.Kind != domain.ArtifactKindInput || artifact.MediaType != domain.MediaTypeImage || artifact.Status != domain.ArtifactStatusReady {
			continue
		}
		width, height := artifact.Width, artifact.Height
		if (width <= 0 || height <= 0) && h.deps.Objects != nil && artifact.StorageBucket != "" && artifact.StorageKey != "" {
			if data, err := h.deps.Objects.GetObject(ctx, artifact.StorageBucket, artifact.StorageKey); err == nil {
				if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
					width, height = cfg.Width, cfg.Height
				}
			}
		}
		if aspect := allowedAspectRatioForDimensions(width, height, route.AllowedAspectRatios); aspect != "" {
			return aspect
		}
	}
	return ""
}

func allowedAspectRatioForDimensions(width, height int, allowed []string) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	target := float64(width) / float64(height)
	orientation := aspectOrientation(width, height)
	if value := closestAllowedAspectRatio(target, orientation, allowed); value != "" {
		return value
	}
	return closestAllowedAspectRatio(target, 0, allowed)
}

func closestAllowedAspectRatio(target float64, orientation int, allowed []string) string {
	best := ""
	bestScore := math.MaxFloat64
	for _, raw := range allowed {
		value := strings.TrimSpace(raw)
		width, height, ok := parseAspectRatio(value)
		if !ok {
			continue
		}
		if orientation != 0 && aspectOrientation(width, height) != orientation {
			continue
		}
		ratio := float64(width) / float64(height)
		score := math.Abs(math.Log(target / ratio))
		if score < bestScore {
			best = value
			bestScore = score
		}
	}
	return best
}

func aspectOrientation(width, height int) int {
	if width > height {
		return 1
	}
	if height > width {
		return -1
	}
	return 0
}

func parseAspectRatio(value string) (int, int, bool) {
	left, right, ok := strings.Cut(strings.TrimSpace(value), ":")
	if !ok {
		return 0, 0, false
	}
	width, err := strconv.Atoi(strings.TrimSpace(left))
	if err != nil || width <= 0 {
		return 0, 0, false
	}
	height, err := strconv.Atoi(strings.TrimSpace(right))
	if err != nil || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}
