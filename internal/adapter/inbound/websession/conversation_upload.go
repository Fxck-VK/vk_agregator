package websession

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"slices"
	"strconv"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/productcatalog"
)

func (h *Handler) quoteConversationImage(w http.ResponseWriter, r *http.Request) {
	count, err := strconv.Atoi(r.URL.Query().Get("reference_count"))
	outputs, outputErr := strconv.Atoi(r.URL.Query().Get("output_count"))
	if err != nil || outputErr != nil || count < 1 || count > 16 || !h.imageReferencesEnabled(r.URL.Query().Get("model_id")) {
		writeError(w, 400, "invalid image options")
		return
	}
	_, _, _, price, err := h.resolveConversationMedia(conversationGenerationRequest{ModelID: r.URL.Query().Get("model_id"), ImageQuality: r.URL.Query().Get("image_quality"), AspectRatio: r.URL.Query().Get("aspect_ratio"), OutputCount: outputs, ReferenceArtifactIDs: make([]uuid.UUID, count)})
	if err != nil {
		writeError(w, 400, "invalid image options")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, struct {
		Credits int64 `json:"credits"`
	}{price.InternalCredits})
}

// History references come from the persisted account-owned job, never URLs or
// identifiers in user text. Each image is still authorized on delivery.
func (h *Handler) attachConversationInputs(ctx context.Context, owner, conversationID, jobID uuid.UUID, message *safeConversationMessage) {
	if h.deps.ImageJobReader == nil || h.deps.InputArtifacts == nil {
		return
	}
	job, err := h.deps.ImageJobReader.GetByIDForAccount(ctx, owner, jobID)
	if err != nil || !ownedConversationJob(job, owner, conversationID) {
		return
	}
	var params struct {
		IDs []uuid.UUID `json:"reference_artifact_ids"`
	}
	if json.Unmarshal(job.Params, &params) != nil || len(params.IDs) > 16 {
		return
	}
	for _, id := range params.IDs {
		a, err := h.deps.InputArtifacts.GetArtifactForAccount(ctx, owner, id)
		if err == nil && validWebInput(a, owner) && a.ID == id {
			message.InputImages = append(message.InputImages, id)
		}
	}
}

// Storage and deduplication remain in artifactservice, scoped to the session account.
type InputArtifactService interface {
	SaveAccountInputArtifact(context.Context, uuid.UUID, domain.MediaType, string, []byte) (*domain.Artifact, error)
	GetArtifactForAccount(context.Context, uuid.UUID, uuid.UUID) (*domain.Artifact, error)
}

func (h *Handler) imageReferencesEnabled(modelID string) bool {
	if h.deps.InputArtifacts == nil || h.deps.InputObjects == nil {
		return false
	}
	for _, m := range h.cfg.ImageModels {
		if m.ID == modelID {
			return m.Enabled && m.Ready && m.SupportsReferenceImage && m.MaxReferenceImages > 0
		}
	}
	return false
}

// These transport limits match the existing image-reference worker. Native
// support or client MIME alone never admits an input.
func webInputImage(data []byte) (string, image.Config, error) {
	mime := http.DetectContentType(data)
	if len(data) == 0 || len(data) > productcatalog.WebReferenceMaxBytes || !slices.Contains([]string{"image/png", "image/jpeg"}, mime) {
		return "", image.Config{}, errors.New("invalid image")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > productcatalog.WebReferenceMaxDimension || cfg.Height > productcatalog.WebReferenceMaxDimension {
		return "", image.Config{}, errors.New("invalid image dimensions")
	}
	if _, _, err = image.Decode(bytes.NewReader(data)); err != nil {
		return "", image.Config{}, errors.New("invalid image")
	}
	return mime, cfg, nil
}

func (h *Handler) uploadConversationInput(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, _ := PrincipalFromContext(r.Context())
	if h.deps.InputArtifacts == nil || h.deps.InputObjects == nil || h.deps.WebChatMessageLimiter == nil {
		writeError(w, 503, "attachment storage unavailable")
		return
	}
	if !h.imageReferencesEnabled(r.URL.Query().Get("model_id")) {
		writeError(w, 400, "attachments unsupported for model")
		return
	}
	allowed, err := h.deps.WebChatMessageLimiter.Allow(r.Context(), "upload:"+principal.AccountID.String())
	if err != nil {
		writeError(w, 503, "attachment storage unavailable")
		return
	}
	if !allowed {
		writeError(w, 429, "upload rate limited")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, productcatalog.WebReferenceMaxBytes+(1<<20))
	if err := r.ParseMultipartForm(productcatalog.WebReferenceMaxBytes); err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			writeError(w, 413, "attachment too large")
		} else {
			writeError(w, 400, "invalid attachment")
		}
		return
	}
	defer r.MultipartForm.RemoveAll()
	if len(r.MultipartForm.File["file"]) != 1 || len(r.MultipartForm.File) != 1 {
		writeError(w, 400, "one attachment required")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, 400, "invalid attachment")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, productcatalog.WebReferenceMaxBytes+1))
	if len(data) > productcatalog.WebReferenceMaxBytes {
		writeError(w, 413, "attachment too large")
		return
	}
	mime, cfg, validationErr := webInputImage(data)
	if err != nil || validationErr != nil {
		writeError(w, 400, "invalid image attachment")
		return
	}
	artifact, err := h.deps.InputArtifacts.SaveAccountInputArtifact(r.Context(), principal.AccountID, domain.MediaTypeImage, mime, data)
	if err != nil || !validWebInput(artifact, principal.AccountID) {
		writeError(w, 503, "attachment upload failed")
		return
	}
	writeJSON(w, http.StatusCreated, struct {
		ID     uuid.UUID `json:"artifact_id"`
		MIME   string    `json:"mime_type"`
		Size   int       `json:"size_bytes"`
		Width  int       `json:"width"`
		Height int       `json:"height"`
	}{artifact.ID, mime, len(data), cfg.Width, cfg.Height})
}

func validWebInput(a *domain.Artifact, owner uuid.UUID) bool {
	return a != nil && a.ID != uuid.Nil && a.OwnerAccountID == owner && a.Kind == domain.ArtifactKindInput && a.MediaType == domain.MediaTypeImage && a.Status == domain.ArtifactStatusReady && a.SizeBytes > 0 && a.SizeBytes <= productcatalog.WebReferenceMaxBytes && a.StorageBucket != "" && a.StorageKey != ""
}

func (h *Handler) validateConversationInputs(w http.ResponseWriter, r *http.Request, owner uuid.UUID, req conversationGenerationRequest) bool {
	if len(req.ReferenceArtifactIDs) == 0 {
		return true
	}
	if !h.imageReferencesEnabled(req.ModelID) || len(req.ReferenceArtifactIDs) > 16 {
		writeError(w, 400, "attachments unsupported for model")
		return false
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range req.ReferenceArtifactIDs {
		if id == uuid.Nil || seen[id] {
			writeError(w, 400, "invalid attachment ids")
			return false
		}
		seen[id] = true
		a, err := h.deps.InputArtifacts.GetArtifactForAccount(r.Context(), owner, id)
		if err != nil || a == nil || a.ID != id || a.OwnerAccountID != owner {
			writeError(w, 404, "attachment not found")
			return false
		}
		if !validWebInput(a, owner) {
			writeError(w, 400, "invalid attachment")
			return false
		}
		// Recheck stored bytes, including legacy inputs that predate web validation.
		data, err := h.deps.InputObjects.GetObject(r.Context(), a.StorageBucket, a.StorageKey)
		mime, _, invalid := webInputImage(data)
		if err != nil || invalid != nil || mime != a.MimeType || int64(len(data)) != a.SizeBytes {
			writeError(w, 400, "invalid attachment")
			return false
		}
	}
	return true
}

func (h *Handler) getConversationInput(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("artifactID"))
	if err != nil || h.deps.InputArtifacts == nil || h.deps.InputObjects == nil {
		writeError(w, 404, "attachment not found")
		return
	}
	a, err := h.deps.InputArtifacts.GetArtifactForAccount(r.Context(), principal.AccountID, id)
	if err != nil || !validWebInput(a, principal.AccountID) || a.ID != id {
		writeError(w, 404, "attachment not found")
		return
	}
	data, err := h.deps.InputObjects.GetObject(r.Context(), a.StorageBucket, a.StorageKey)
	mime, _, invalid := webInputImage(data)
	if err != nil || invalid != nil || mime != a.MimeType {
		writeError(w, 404, "attachment not found")
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(data)
}
