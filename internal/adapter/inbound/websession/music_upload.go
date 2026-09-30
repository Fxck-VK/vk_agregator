package websession

import (
	"context"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/mediaprobe"
)

// MusicInputArtifactSaver persists private account-owned music input bytes after
// local media validation. Implementations must not call providers.
type MusicInputArtifactSaver interface {
	SaveBytesArtifactWithMetadataForAccount(ctx context.Context, userID, accountID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, mimeType string, data []byte, metadata domain.ArtifactMediaMetadata) (*domain.Artifact, error)
}

// MusicInputProber extracts bounded audio metadata from raw upload bytes.
type MusicInputProber interface {
	ProbeAudio(ctx context.Context, data []byte, sizeBytes int64) (domain.ArtifactMediaMetadata, error)
}

type musicInputUploadResponse struct {
	ArtifactID uuid.UUID `json:"artifact_id"`
	MIMEType   string    `json:"mime_type"`
	DurationMS int64     `json:"duration_ms"`
	SizeBytes  int64     `json:"size_bytes"`
}

func (h *Handler) uploadMusicInput(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.deps.MusicInputArtifacts == nil || h.deps.MusicInputProber == nil || h.deps.ImageJobPrepareLimiter == nil {
		writeError(w, http.StatusServiceUnavailable, "music upload unavailable")
		return
	}
	allowed, err := h.deps.ImageJobPrepareLimiter.Allow(r.Context(), "account:"+principal.AccountID.String())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "music upload unavailable")
		return
	}
	if !allowed {
		writeError(w, http.StatusTooManyRequests, "music upload rate limited")
		return
	}

	headerMIME, ok := musicInputHeaderMIME(r.Header.Get("Content-Type"))
	if !ok {
		writeError(w, http.StatusBadRequest, "unsupported music input")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, mediaprobe.MaxMusicInputBytes+1)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "music input too large")
		return
	}
	if int64(len(data)) <= 0 || int64(len(data)) > mediaprobe.MaxMusicInputBytes {
		writeError(w, http.StatusBadRequest, "invalid music input")
		return
	}
	sniffedMIME, ok := mediaprobe.SniffMusicInputMIME(data)
	if !ok || sniffedMIME != headerMIME {
		writeError(w, http.StatusBadRequest, "music input mime mismatch")
		return
	}
	metadata, err := h.deps.MusicInputProber.ProbeAudio(r.Context(), data, int64(len(data)))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid music input")
		return
	}
	if err := mediaprobe.ValidateMusicInputArtifact(musicInputValidationArtifact(principal.AccountID, headerMIME, int64(len(data)), metadata), principal.AccountID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid music input")
		return
	}
	artifact, err := h.deps.MusicInputArtifacts.SaveBytesArtifactWithMetadataForAccount(r.Context(), uuid.Nil, principal.AccountID, nil, domain.ArtifactKindInput, domain.MediaTypeAudio, headerMIME, data, metadata)
	if err != nil || artifact == nil || artifact.ID == uuid.Nil {
		writeError(w, http.StatusServiceUnavailable, "music upload unavailable")
		return
	}
	writeJSON(w, http.StatusCreated, musicInputUploadResponse{ArtifactID: artifact.ID, MIMEType: headerMIME, DurationMS: metadata.Normalize().DurationMS, SizeBytes: int64(len(data))})
}

func musicInputHeaderMIME(value string) (string, bool) {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	if err != nil {
		return "", false
	}
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "audio/mpeg":
		return "audio/mpeg", true
	case "audio/aac":
		return "audio/aac", true
	case "audio/wav", "audio/x-wav":
		return "audio/wav", true
	default:
		return "", false
	}
}

func musicInputValidationArtifact(accountID uuid.UUID, mimeType string, size int64, metadata domain.ArtifactMediaMetadata) *domain.Artifact {
	metadata = metadata.Normalize()
	return &domain.Artifact{
		ID:             uuid.New(),
		OwnerAccountID: accountID,
		Kind:           domain.ArtifactKindInput,
		MediaType:      domain.MediaTypeAudio,
		MimeType:       mimeType,
		StorageBucket:  "validation",
		StorageKey:     "validation",
		SizeBytes:      size,
		DurationMS:     metadata.DurationMS,
		Codec:          metadata.Codec,
		Container:      metadata.Container,
		BitrateBPS:     metadata.BitrateBPS,
		ProbeStatus:    metadata.ProbeStatus,
		Status:         domain.ArtifactStatusReady,
	}
}
