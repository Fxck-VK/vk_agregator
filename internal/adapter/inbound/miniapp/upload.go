package miniapp

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/service/artifactservice"
)

const (
	miniAppArtifactBucket           = "artifacts"
	defaultMaxMiniAppUploadBytes    = 20 << 20 // 20 MiB
	defaultMaxMiniAppImageDimension = 4096
	defaultMaxMiniAppImagePixels    = 4096 * 4096
	miniAppUploadFieldName          = "file"
	miniAppMultipartOverage         = 1 << 20
)

func (h *Handler) createArtifact(w http.ResponseWriter, r *http.Request) {
	resultLabel := "success"
	defer func() {
		metrics.ObserveProductEvent("miniapp", "artifact", "upload", "artifact_upload", "image", resultLabel)
	}()
	if h.deps.Artifacts == nil || h.deps.Objects == nil {
		resultLabel = "service_unavailable"
		writeError(w, http.StatusServiceUnavailable, "artifact storage unavailable")
		return
	}
	vkUserID, ok := vkUserIDFromCtx(r.Context())
	if !ok {
		resultLabel = "unauthorized"
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.cfg.ReferenceUploadsDisabled {
		resultLabel = "disabled"
		writeError(w, http.StatusServiceUnavailable, "reference_artifacts_unsupported")
		return
	}
	user, err := h.ensureUser(r.Context(), vkUserID)
	if err != nil {
		h.logger.Error("miniapp: ensure user failed", "error", err.Error())
		resultLabel = "error"
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	data, mimeType, metadata, status, errorCode, ok := h.readMiniAppUpload(w, r)
	if !ok {
		resultLabel = uploadResultLabel(errorCode)
		writeError(w, status, errorCode)
		return
	}

	saver := artifactservice.New(h.deps.Artifacts, h.deps.Objects, miniAppArtifactBucket)
	artifact, err := saver.SaveBytesArtifactWithMetadata(r.Context(), user.ID, nil, domain.ArtifactKindInput, domain.MediaTypeImage, mimeType, data, metadata)
	if err != nil {
		h.logger.Error("miniapp: upload artifact failed", "error", err.Error())
		resultLabel = "error"
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, ArtifactUploadDTO{ArtifactID: artifact.ID})
}

func (h *Handler) readMiniAppUpload(w http.ResponseWriter, r *http.Request) ([]byte, string, domain.ArtifactMediaMetadata, int, string, bool) {
	maxBytes := h.cfg.MaxUploadBytes
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+miniAppMultipartOverage)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "too large") {
			observeMiniAppUploadRejected(domain.JobErrMediaUploadTooLarge, "unknown", 0)
			return nil, "", domain.ArtifactMediaMetadata{}, http.StatusRequestEntityTooLarge, domain.JobErrMediaUploadTooLarge, false
		}
		observeMiniAppUploadRejected(domain.JobErrMediaUploadInvalid, "unknown", 0)
		return nil, "", domain.ArtifactMediaMetadata{}, http.StatusBadRequest, domain.JobErrMediaUploadInvalid, false
	}
	file, _, err := r.FormFile(miniAppUploadFieldName)
	if err != nil {
		observeMiniAppUploadRejected(domain.JobErrMediaUploadInvalid, "unknown", 0)
		return nil, "", domain.ArtifactMediaMetadata{}, http.StatusBadRequest, domain.JobErrMediaUploadInvalid, false
	}
	defer func() {
		_ = file.Close()
	}()

	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		observeMiniAppUploadRejected(domain.JobErrMediaUploadInvalid, "unknown", 0)
		return nil, "", domain.ArtifactMediaMetadata{}, http.StatusBadRequest, domain.JobErrMediaUploadInvalid, false
	}
	if len(data) == 0 {
		observeMiniAppUploadRejected(domain.JobErrMediaUploadInvalid, "unknown", 0)
		return nil, "", domain.ArtifactMediaMetadata{}, http.StatusBadRequest, domain.JobErrMediaUploadInvalid, false
	}
	mimeClass := miniAppDetectedMimeClass(data)
	if int64(len(data)) > maxBytes {
		observeMiniAppUploadRejected(domain.JobErrMediaUploadTooLarge, mimeClass, int64(len(data)))
		return nil, "", domain.ArtifactMediaMetadata{}, http.StatusRequestEntityTooLarge, domain.JobErrMediaUploadTooLarge, false
	}
	mimeType, ok := miniAppImageMime(data, h.cfg.ReferenceWebPEnabled)
	if !ok {
		observeMiniAppUploadRejected(domain.JobErrMediaUploadUnsupported, mimeClass, int64(len(data)))
		return nil, "", domain.ArtifactMediaMetadata{}, http.StatusBadRequest, domain.JobErrMediaUploadUnsupported, false
	}
	mimeClass = miniAppMimeClass(mimeType)
	metadata, pixels, valid, tooLarge := h.miniAppImageMetadata(data, mimeType)
	if !valid {
		observeMiniAppUploadRejected(domain.JobErrMediaUploadInvalid, mimeClass, int64(len(data)))
		return nil, "", domain.ArtifactMediaMetadata{}, http.StatusBadRequest, domain.JobErrMediaUploadInvalid, false
	}
	if tooLarge {
		observeMiniAppUploadRejected(domain.JobErrMediaUploadTooLarge, mimeClass, int64(len(data)))
		if pixels > 0 {
			metrics.ObserveMediaUploadPixels("miniapp", mimeClass, pixels)
		}
		return nil, "", domain.ArtifactMediaMetadata{}, http.StatusRequestEntityTooLarge, domain.JobErrMediaUploadTooLarge, false
	}
	metrics.ObserveMediaUploadValidation("miniapp", "accepted", "none", mimeClass)
	metrics.ObserveMediaUploadBytes("miniapp", mimeClass, int64(len(data)))
	if pixels > 0 {
		metrics.ObserveMediaUploadPixels("miniapp", mimeClass, pixels)
	}
	return data, mimeType, metadata, http.StatusOK, "", true
}

func miniAppImageMime(data []byte, allowWebP bool) (string, bool) {
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		if !allowWebP {
			return "", false
		}
		return "image/webp", true
	}
	switch detected := http.DetectContentType(data); detected {
	case "image/jpeg", "image/png":
		return detected, true
	default:
		return "", false
	}
}

func observeMiniAppUploadRejected(reason, mimeClass string, sizeBytes int64) {
	if mimeClass == "" {
		mimeClass = "unknown"
	}
	metrics.ObserveMediaUploadValidation("miniapp", "rejected", reason, mimeClass)
	if sizeBytes > 0 {
		metrics.ObserveMediaUploadBytes("miniapp", mimeClass, sizeBytes)
	}
}

func miniAppDetectedMimeClass(data []byte) string {
	if len(data) == 0 {
		return "unknown"
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "webp"
	}
	return miniAppMimeClass(http.DetectContentType(data))
}

func miniAppMimeClass(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg":
		return "jpeg"
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	case "":
		return "unknown"
	default:
		return "other"
	}
}

func (h *Handler) miniAppImageMetadata(data []byte, mimeType string) (domain.ArtifactMediaMetadata, int64, bool, bool) {
	switch mimeType {
	case "image/jpeg", "image/png":
	default:
		return domain.ArtifactMediaMetadata{}, 0, true, false
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return domain.ArtifactMediaMetadata{}, 0, false, false
	}
	pixels := int64(cfg.Width) * int64(cfg.Height)
	tooLarge := cfg.Width > h.cfg.MaxUploadImageWidth ||
		cfg.Height > h.cfg.MaxUploadImageHeight ||
		pixels > h.cfg.MaxUploadImagePixels
	return domain.ArtifactMediaMetadata{Width: cfg.Width, Height: cfg.Height, ProbeStatus: domain.MediaProbeSkipped}, pixels, true, tooLarge
}

func uploadResultLabel(errorCode string) string {
	switch errorCode {
	case domain.JobErrMediaUploadTooLarge:
		return "too_large"
	case domain.JobErrMediaUploadUnsupported:
		return "unsupported"
	default:
		return "invalid_upload"
	}
}
