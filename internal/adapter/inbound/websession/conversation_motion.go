package websession

import (
	"context"
	"errors"
	"io"
	"net/http"
	"slices"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/videoreference"
)

type MotionVideoInputUploader interface {
	Upload(context.Context, uuid.UUID, uuid.UUID, []byte) (*domain.Artifact, error)
}

func (h *Handler) motionReferencesEnabled() bool {
	return h.cfg.Environment == "development" && h.deps.MotionVideoInputs != nil && h.deps.InputArtifacts != nil && h.deps.InputObjects != nil
}

func (h *Handler) uploadMotionVideo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id := string(domain.VideoRouteKling26Motion)
	if !h.motionReferencesEnabled() || !h.isConversationMediaModel(id) || h.deps.WebChatMessageLimiter == nil {
		writeError(w, 503, "motion upload unavailable")
		return
	}
	if !h.requireReleaseModel(w, r, id, "video") {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	allowed, err := h.deps.WebChatMessageLimiter.Allow(r.Context(), "upload:"+principal.AccountID.String())
	if err != nil {
		writeError(w, 503, "attachment storage unavailable")
		return
	}
	if !allowed {
		writeError(w, 429, "upload rate limited")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, productcatalog.WebMotionVideoMaxBytes+(1<<20))
	// Spill multipart data above 1 MiB to a temporary file, then always remove it.
	err = r.ParseMultipartForm(1 << 20)
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			writeError(w, 413, "attachment too large")
		} else {
			writeError(w, 400, "invalid attachment")
		}
		return
	}
	if len(r.MultipartForm.File) != 1 || len(r.MultipartForm.File["file"]) != 1 {
		writeError(w, 400, "one video required")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, 400, "invalid attachment")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, productcatalog.WebMotionVideoMaxBytes+1))
	if len(data) > productcatalog.WebMotionVideoMaxBytes {
		writeError(w, 413, "attachment too large")
		return
	}
	if err != nil {
		writeError(w, 400, "invalid attachment")
		return
	}
	a, err := h.deps.MotionVideoInputs.Upload(r.Context(), uuid.Nil, principal.AccountID, data)
	if errors.Is(err, videoreference.ErrInvalid) {
		writeError(w, 400, "invalid motion reference video")
		return
	}
	if err != nil {
		writeError(w, 503, "video validation unavailable")
		return
	}
	seconds, err := videoreference.Validate(a, principal.AccountID, 30)
	if err != nil || a.ID == uuid.Nil || a.OwnerAccountID != principal.AccountID {
		writeError(w, 503, "video validation unavailable")
		return
	}
	writeJSON(w, 201, struct {
		ID          uuid.UUID `json:"artifact_id"`
		MIME        string    `json:"mime_type"`
		Size        int64     `json:"size_bytes"`
		DurationMS  int64     `json:"duration_ms"`
		DurationSec int       `json:"duration_sec"`
	}{a.ID, a.MimeType, a.SizeBytes, a.DurationMS, seconds})
}

// Duration is always re-derived from an owned, probed input before pricing or reservation.
func (h *Handler) prepareMotionInput(w http.ResponseWriter, r *http.Request, owner uuid.UUID, req *conversationGenerationRequest) bool {
	if req.ModelID != string(domain.VideoRouteKling26Motion) {
		return true
	}
	if !h.motionReferencesEnabled() {
		writeError(w, 503, "motion unavailable")
		return false
	}
	if !h.requireReleaseModel(w, r, req.ModelID, "video") {
		return false
	}
	maximum := 10
	if req.CharacterOrientation == "video" {
		maximum = 30
	} else if req.CharacterOrientation != "image" {
		writeError(w, 400, "invalid character orientation")
		return false
	}
	a, err := h.deps.InputArtifacts.GetArtifactForAccount(r.Context(), owner, req.ReferenceVideoArtifactID)
	if err != nil || a == nil || a.ID != req.ReferenceVideoArtifactID || a.OwnerAccountID != owner {
		writeError(w, 404, "reference video not found")
		return false
	}
	seconds, err := videoreference.Validate(a, owner, maximum)
	if err != nil || a.SizeBytes > productcatalog.WebMotionVideoMaxBytes || a.StorageBucket == "" || a.StorageKey == "" {
		writeError(w, 400, "invalid motion reference video")
		return false
	}
	if req.DurationSec != 0 && req.DurationSec != seconds {
		writeError(w, 400, "duration differs from reference video")
		return false
	}
	req.DurationSec = seconds
	return true
}

func (h *Handler) resolveMotionMedia(req conversationGenerationRequest) (domain.OperationType, domain.Modality, map[string]any, pricingcatalog.PricingSnapshot, error) {
	invalid := errors.New("invalid motion options")
	maximum := 10
	if req.CharacterOrientation == "video" {
		maximum = 30
	} else if req.CharacterOrientation != "image" {
		return "", "", nil, pricingcatalog.PricingSnapshot{}, invalid
	}
	if len(req.ReferenceArtifactIDs) != 1 || req.ReferenceArtifactIDs[0] == uuid.Nil || req.ReferenceVideoArtifactID == uuid.Nil || req.ReferenceArtifactIDs[0] == req.ReferenceVideoArtifactID || req.DurationSec < 3 || req.DurationSec > maximum || req.AspectRatio != "" || req.ImageQuality != "" || req.OutputCount != 0 || !providermodels.MediaPromptValid(req.ModelID, req.Prompt) {
		return "", "", nil, pricingcatalog.PricingSnapshot{}, invalid
	}
	for _, route := range h.conversationVideoRoutes() {
		if route.Alias != req.ModelID {
			continue
		}
		if !slices.Contains(route.AllowedResolutions, req.Resolution) || !slices.Contains(route.AllowedDurationsSec, req.DurationSec) {
			break
		}
		price, err := h.deps.ImagePricing.Snapshot(pricingcatalog.ProductKey{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, VideoRouteAlias: domain.VideoRouteKling26Motion, Resolution: req.Resolution, DurationSec: req.DurationSec})
		if err != nil || !price.Valid() {
			break
		}
		keep := true
		if req.KeepOriginalSound != nil {
			keep = *req.KeepOriginalSound
		}
		params := map[string]any{"prompt": req.Prompt, "model_id": route.Alias, "model_name": route.Name, "video_route_alias": route.Alias, "resolution": req.Resolution, "duration_sec": req.DurationSec, "reference_artifact_ids": req.ReferenceArtifactIDs, "reference_video_artifact_id": req.ReferenceVideoArtifactID, "character_orientation": req.CharacterOrientation, "keep_original_sound": keep}
		return domain.OperationVideoGenerate, domain.ModalityVideo, params, price, nil
	}
	return "", "", nil, pricingcatalog.PricingSnapshot{}, invalid
}

func conversationInputIDs(req conversationGenerationRequest) []uuid.UUID {
	ids := append([]uuid.UUID(nil), req.ReferenceArtifactIDs...)
	if req.ReferenceVideoArtifactID != uuid.Nil {
		ids = append(ids, req.ReferenceVideoArtifactID)
	}
	return ids
}

func (h *Handler) quoteMotionVideo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !h.motionReferencesEnabled() || !h.isConversationMediaModel(string(domain.VideoRouteKling26Motion)) {
		writeError(w, 404, "motion unavailable")
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	videoID, err := uuid.Parse(r.URL.Query().Get("reference_video_artifact_id"))
	if err != nil {
		writeError(w, 400, "invalid reference video")
		return
	}
	req := conversationGenerationRequest{ModelID: string(domain.VideoRouteKling26Motion), ReferenceVideoArtifactID: videoID, CharacterOrientation: r.URL.Query().Get("character_orientation"), Resolution: r.URL.Query().Get("resolution")}
	if !h.prepareMotionInput(w, r, principal.AccountID, &req) {
		return
	}
	for _, route := range h.conversationVideoRoutes() {
		if route.Alias != req.ModelID || !slices.Contains(route.AllowedResolutions, req.Resolution) || !slices.Contains(route.AllowedDurationsSec, req.DurationSec) {
			continue
		}
		price, err := h.deps.ImagePricing.Snapshot(pricingcatalog.ProductKey{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, VideoRouteAlias: domain.VideoRouteKling26Motion, Resolution: req.Resolution, DurationSec: req.DurationSec})
		if err == nil && price.Valid() {
			writeJSON(w, 200, struct {
				Credits     int64 `json:"credits"`
				DurationSec int   `json:"duration_sec"`
			}{price.InternalCredits, req.DurationSec})
			return
		}
	}
	writeError(w, 400, "invalid motion options")
}
