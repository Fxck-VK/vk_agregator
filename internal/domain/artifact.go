package domain

import (
	"time"

	"github.com/google/uuid"
)

// MediaType is the high-level kind of media an artifact holds.
type MediaType string

const (
	// MediaTypeImage is a still image.
	MediaTypeImage MediaType = "image"
	// MediaTypeVideo is a video.
	MediaTypeVideo MediaType = "video"
	// MediaTypeAudio is an audio file.
	MediaTypeAudio MediaType = "audio"
	// MediaTypeText is a text payload.
	MediaTypeText MediaType = "text"
	// MediaTypeDocument is an arbitrary document/file.
	MediaTypeDocument MediaType = "document"
)

// Valid reports whether the media type is one of the known types.
func (m MediaType) Valid() bool {
	switch m {
	case MediaTypeImage, MediaTypeVideo, MediaTypeAudio, MediaTypeText, MediaTypeDocument:
		return true
	default:
		return false
	}
}

// ArtifactKind describes the role an artifact plays in a job's lifecycle.
type ArtifactKind string

const (
	// ArtifactKindInput is user-supplied input media.
	ArtifactKindInput ArtifactKind = "input"
	// ArtifactKindIntermediate is produced between workflow steps.
	ArtifactKindIntermediate ArtifactKind = "intermediate"
	// ArtifactKindOutput is the final result delivered to the user.
	ArtifactKindOutput ArtifactKind = "output"
)

// ArtifactStatus is the lifecycle state of an artifact's stored bytes.
type ArtifactStatus string

const (
	// ArtifactStatusPending means metadata exists but bytes are not stored yet.
	ArtifactStatusPending ArtifactStatus = "pending"
	// ArtifactStatusStored means bytes are durably stored.
	ArtifactStatusStored ArtifactStatus = "stored"
	// ArtifactStatusScanning means the file is being safety/malware scanned.
	ArtifactStatusScanning ArtifactStatus = "scanning"
	// ArtifactStatusReady means stored, scanned and usable.
	ArtifactStatusReady ArtifactStatus = "ready"
	// ArtifactStatusFailed means ingestion or scanning failed.
	ArtifactStatusFailed ArtifactStatus = "failed"
	// ArtifactStatusDeleted means the bytes were purged.
	ArtifactStatusDeleted ArtifactStatus = "deleted"
)

// VariantType is the rendition of an artifact for a specific purpose, e.g. a
// preview or a VK-ready upload format.
type VariantType string

const (
	// VariantOriginal is the unmodified source rendition.
	VariantOriginal VariantType = "original"
	// VariantPreview is a lightweight preview rendition.
	VariantPreview VariantType = "preview"
	// VariantThumbnail is a small thumbnail.
	VariantThumbnail VariantType = "thumbnail"
	// VariantVKPhoto is a rendition prepared for VK photo upload.
	VariantVKPhoto VariantType = "vk_photo"
	// VariantVKDoc is a rendition prepared for VK document upload.
	VariantVKDoc VariantType = "vk_doc"
	// VariantVKVideo is a rendition prepared for VK video upload.
	VariantVKVideo VariantType = "vk_video"
)

// Artifact is the canonical record of a media file. Every media file is an
// Artifact (invariant #7) and must be stored before delivery.
type Artifact struct {
	// ID is the internal primary key.
	ID uuid.UUID `json:"id"`
	// OwnerUserID is the user that owns the artifact.
	OwnerUserID uuid.UUID `json:"owner_user_id"`
	// JobID is the job that produced or consumed the artifact, if any.
	JobID *uuid.UUID `json:"job_id,omitempty"`
	// Kind is the role of the artifact (input/intermediate/output).
	Kind ArtifactKind `json:"kind"`
	// MediaType is the high-level media kind.
	MediaType MediaType `json:"media_type"`
	// MimeType is the precise content type, verified by sniffing.
	MimeType string `json:"mime_type"`
	// StorageBucket is the object storage bucket name.
	StorageBucket string `json:"storage_bucket"`
	// StorageKey is the object storage key of the original bytes.
	StorageKey string `json:"storage_key"`
	// PublicURL is an optional public/signed URL, when applicable.
	PublicURL string `json:"public_url,omitempty"`
	// SHA256 is the hex content hash for dedup and integrity.
	SHA256 string `json:"sha256"`
	// SizeBytes is the size of the original bytes.
	SizeBytes int64 `json:"size_bytes"`
	// Width is the pixel width for image/video, 0 otherwise.
	Width int `json:"width,omitempty"`
	// Height is the pixel height for image/video, 0 otherwise.
	Height int `json:"height,omitempty"`
	// DurationMS is the duration in milliseconds for video/audio.
	DurationMS int64 `json:"duration_ms,omitempty"`
	// Status is the artifact lifecycle state.
	Status ArtifactStatus `json:"status"`
	// CreatedAt is the row creation timestamp.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the last mutation timestamp.
	UpdatedAt time.Time `json:"updated_at"`
}

// ArtifactVariant is a derived rendition of an Artifact stored separately, used
// to keep the original immutable while serving purpose-built copies.
type ArtifactVariant struct {
	// ID is the internal primary key.
	ID uuid.UUID `json:"id"`
	// ArtifactID is the parent artifact.
	ArtifactID uuid.UUID `json:"artifact_id"`
	// VariantType is the rendition purpose.
	VariantType VariantType `json:"variant_type"`
	// StorageBucket is the object storage bucket of the variant.
	StorageBucket string `json:"storage_bucket"`
	// StorageKey is the object storage key of the variant bytes.
	StorageKey string `json:"storage_key"`
	// MimeType is the precise content type of the variant.
	MimeType string `json:"mime_type"`
	// SizeBytes is the variant size in bytes.
	SizeBytes int64 `json:"size_bytes"`
	// Width is the pixel width of the variant, 0 if not applicable.
	Width int `json:"width,omitempty"`
	// Height is the pixel height of the variant, 0 if not applicable.
	Height int `json:"height,omitempty"`
	// DurationMS is the variant duration in milliseconds, 0 if not applicable.
	DurationMS int64 `json:"duration_ms,omitempty"`
	// CreatedAt is the row creation timestamp.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the last mutation timestamp.
	UpdatedAt time.Time `json:"updated_at"`
}
