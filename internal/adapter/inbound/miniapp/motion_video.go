package miniapp

import (
	"errors"
	"github.com/google/uuid"
	"net/http"
	"slices"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/videoreference"
)

func validateVideoOptions(w http.ResponseWriter, route VideoRouteDTO, req *CreateJobRequest) bool {
	valid := !req.VideoAudio || route.SupportsAudio
	if route.RequiresReferenceVideo {
		if req.CharacterOrientation == "" {
			req.CharacterOrientation = "image"
		}
		valid = valid && req.ReferenceVideoArtifactID != uuid.Nil && !slices.Contains(req.ReferenceArtifactIDs, req.ReferenceVideoArtifactID) && (req.CharacterOrientation == "image" || req.CharacterOrientation == "video")
	} else {
		valid = valid && req.ReferenceVideoArtifactID == uuid.Nil && req.CharacterOrientation == "" && req.KeepOriginalSound == nil
	}
	if !valid {
		writeError(w, http.StatusBadRequest, "invalid video options")
	}
	return valid
}

func videoInputArtifactIDs(req CreateJobRequest) []uuid.UUID {
	ids := append([]uuid.UUID(nil), req.ReferenceArtifactIDs...)
	if req.ReferenceVideoArtifactID != uuid.Nil {
		ids = append(ids, req.ReferenceVideoArtifactID)
	}
	return ids
}

// Duration is derived from the owned file on both estimate and creation.
func (h *Handler) resolveMotionReference(w http.ResponseWriter, r *http.Request, owner uuid.UUID, req *CreateJobRequest) bool {
	if req.ReferenceVideoArtifactID == uuid.Nil {
		return true
	}
	if h.deps.Artifacts == nil {
		writeError(w, http.StatusServiceUnavailable, "artifact storage unavailable")
		return false
	}
	artifact, err := h.deps.Artifacts.GetByID(r.Context(), req.ReferenceVideoArtifactID)
	if err != nil {
		writeError(w, http.StatusNotFound, "reference artifact not found")
		return false
	}
	maximum := 30
	if req.CharacterOrientation == "image" {
		maximum = 10
	}
	duration, err := videoreference.Validate(artifact, owner, maximum)
	if errors.Is(err, videoreference.ErrNotFound) {
		writeError(w, http.StatusNotFound, "reference artifact not found")
		return false
	}
	if err != nil || (req.DurationSec != 0 && req.DurationSec != duration) {
		writeError(w, http.StatusBadRequest, "invalid reference video duration")
		return false
	}
	req.DurationSec = duration
	return true
}

func (h *Handler) createVideoArtifact(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.videoRouteByAlias(string(domain.VideoRouteKling26Motion)); !ok || h.cfg.ReferenceUploadsDisabled || h.deps.VideoReferenceProber == nil || h.deps.Objects == nil || h.deps.Artifacts == nil {
		writeError(w, http.StatusServiceUnavailable, "reference_artifacts_unsupported")
		return
	}
	vkUserID, ok := vkUserIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	user, err := h.ensureUser(r.Context(), vkUserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, videoreference.MaxBytes+miniAppMultipartOverage)
	data, err := readMiniAppMultipartFile(r, videoreference.MaxBytes)
	if err != nil {
		status := http.StatusBadRequest
		if isMiniAppUploadTooLarge(err) {
			status = http.StatusRequestEntityTooLarge
		}
		writeError(w, status, "invalid reference video upload")
		return
	}
	artifact, err := videoreference.New(h.deps.Artifacts, h.deps.Objects, h.deps.VideoReferenceProber).Upload(r.Context(), user.ID, user.EffectiveAccountID(), data)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid reference video; use MP4 or MOV, 3–30 seconds, up to 100 MB")
		return
	}
	writeJSON(w, http.StatusCreated, struct {
		ArtifactID  uuid.UUID `json:"artifact_id"`
		DurationSec int       `json:"duration_sec"`
	}{artifact.ID, int((artifact.DurationMS + 999) / 1000)})
}
