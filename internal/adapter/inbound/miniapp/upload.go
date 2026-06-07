package miniapp

import (
	"io"
	"net/http"
	"strings"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/artifactservice"
)

const (
	miniAppArtifactBucket   = "artifacts"
	maxMiniAppUploadBytes   = 20 << 20 // 20 MiB
	miniAppUploadFieldName  = "file"
	miniAppMultipartOverage = 1 << 20
)

func (h *Handler) createArtifact(w http.ResponseWriter, r *http.Request) {
	if h.deps.Artifacts == nil || h.deps.Objects == nil {
		writeError(w, http.StatusServiceUnavailable, "artifact storage unavailable")
		return
	}
	vkUserID, ok := vkUserIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	user, err := h.ensureUser(r.Context(), vkUserID)
	if err != nil {
		h.logger.Error("miniapp: ensure user failed", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	data, mimeType, status, ok := readMiniAppUpload(w, r)
	if !ok {
		writeError(w, status, uploadErrorMessage(status))
		return
	}

	saver := artifactservice.New(h.deps.Artifacts, h.deps.Objects, miniAppArtifactBucket)
	artifact, err := saver.SaveBytesArtifact(r.Context(), user.ID, nil, domain.ArtifactKindInput, domain.MediaTypeImage, mimeType, data)
	if err != nil {
		h.logger.Error("miniapp: upload artifact failed", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, ArtifactUploadDTO{ArtifactID: artifact.ID})
}

func readMiniAppUpload(w http.ResponseWriter, r *http.Request) ([]byte, string, int, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxMiniAppUploadBytes+miniAppMultipartOverage)
	if err := r.ParseMultipartForm(maxMiniAppUploadBytes); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "too large") {
			return nil, "", http.StatusRequestEntityTooLarge, false
		}
		return nil, "", http.StatusBadRequest, false
	}
	file, _, err := r.FormFile(miniAppUploadFieldName)
	if err != nil {
		return nil, "", http.StatusBadRequest, false
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxMiniAppUploadBytes+1))
	if err != nil {
		return nil, "", http.StatusBadRequest, false
	}
	if len(data) == 0 {
		return nil, "", http.StatusBadRequest, false
	}
	if len(data) > maxMiniAppUploadBytes {
		return nil, "", http.StatusRequestEntityTooLarge, false
	}
	mimeType, ok := miniAppImageMime(data)
	if !ok {
		return nil, "", http.StatusBadRequest, false
	}
	return data, mimeType, http.StatusOK, true
}

func miniAppImageMime(data []byte) (string, bool) {
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp", true
	}
	switch detected := http.DetectContentType(data); detected {
	case "image/jpeg", "image/png":
		return detected, true
	default:
		return "", false
	}
}

func uploadErrorMessage(status int) string {
	switch status {
	case http.StatusRequestEntityTooLarge:
		return "file too large"
	case http.StatusBadRequest:
		return "invalid image upload"
	default:
		return "invalid image upload"
	}
}
