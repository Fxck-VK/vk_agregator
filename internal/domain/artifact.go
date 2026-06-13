package domain

import (
	"strings"
	"time"
	"unicode"

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

// ArtifactLifecycleClass describes retention/dedup policy for stored media.
// Values are bounded and internal; do not expose them as public identifiers.
type ArtifactLifecycleClass string

const (
	ArtifactLifecycleTempUpload       ArtifactLifecycleClass = "temp_upload"
	ArtifactLifecycleInputReference   ArtifactLifecycleClass = "input_reference"
	ArtifactLifecycleProviderOriginal ArtifactLifecycleClass = "provider_original"
	ArtifactLifecycleDeliveryVariant  ArtifactLifecycleClass = "delivery_variant"
	ArtifactLifecycleFailedDeleted    ArtifactLifecycleClass = "failed_deleted"
)

// Valid reports whether the lifecycle class is one of the known values.
func (c ArtifactLifecycleClass) Valid() bool {
	switch c {
	case ArtifactLifecycleTempUpload,
		ArtifactLifecycleInputReference,
		ArtifactLifecycleProviderOriginal,
		ArtifactLifecycleDeliveryVariant,
		ArtifactLifecycleFailedDeleted:
		return true
	default:
		return false
	}
}

// NormalizeArtifactLifecycleClass fills a conservative lifecycle class from
// artifact shape while rejecting arbitrary persisted values.
func NormalizeArtifactLifecycleClass(class ArtifactLifecycleClass, kind ArtifactKind, mediaType MediaType, status ArtifactStatus) ArtifactLifecycleClass {
	if status == ArtifactStatusFailed || status == ArtifactStatusDeleted {
		return ArtifactLifecycleFailedDeleted
	}
	if class.Valid() {
		return class
	}
	if kind == ArtifactKindInput && mediaType == MediaTypeImage {
		return ArtifactLifecycleInputReference
	}
	if kind == ArtifactKindOutput || kind == ArtifactKindIntermediate {
		return ArtifactLifecycleProviderOriginal
	}
	return ArtifactLifecycleTempUpload
}

// MediaCleanupKind identifies which stored media object a maintenance cleanup
// candidate points at. It is bounded and safe for internal branching only.
type MediaCleanupKind string

const (
	MediaCleanupOriginal MediaCleanupKind = "original"
	MediaCleanupVariant  MediaCleanupKind = "variant"
)

// MediaCleanupClass mirrors artifact lifecycle policy for cleanup decisions.
type MediaCleanupClass string

const (
	MediaCleanupTempUpload       MediaCleanupClass = "temp_upload"
	MediaCleanupInputReference   MediaCleanupClass = "input_reference"
	MediaCleanupProviderOriginal MediaCleanupClass = "provider_original"
	MediaCleanupDeliveryVariant  MediaCleanupClass = "delivery_variant"
	MediaCleanupFailedDeleted    MediaCleanupClass = "failed_deleted"
)

// MediaCleanupPolicy contains per-class cutoffs. A zero cutoff disables that
// class so production can retain active history while still purging failures.
type MediaCleanupPolicy struct {
	TempUploadCutoff       time.Time
	InputReferenceCutoff   time.Time
	ProviderOriginalCutoff time.Time
	DeliveryVariantCutoff  time.Time
	FailedDeletedCutoff    time.Time
}

// Enabled reports whether at least one media cleanup class is enabled.
func (p MediaCleanupPolicy) Enabled() bool {
	return !p.TempUploadCutoff.IsZero() ||
		!p.InputReferenceCutoff.IsZero() ||
		!p.ProviderOriginalCutoff.IsZero() ||
		!p.DeliveryVariantCutoff.IsZero() ||
		!p.FailedDeletedCutoff.IsZero()
}

// MediaCleanupCandidate is a storage object that maintenance may delete. It
// intentionally carries only internal ids and storage coordinates; callers must
// never expose it through public DTOs or logs.
type MediaCleanupCandidate struct {
	Kind          MediaCleanupKind
	CleanupClass  MediaCleanupClass
	ArtifactID    uuid.UUID
	VariantID     uuid.UUID
	VariantType   VariantType
	MediaType     MediaType
	StorageBucket string
	StorageKey    string
	SizeBytes     int64
}

// MediaProbeStatus is the sanitized state of media technical metadata
// extraction. It intentionally stores only bounded status, not raw ffprobe
// output or private storage paths.
type MediaProbeStatus string

const (
	// MediaProbeUnknown means no probe decision has been recorded yet.
	MediaProbeUnknown MediaProbeStatus = "unknown"
	// MediaProbePending means a probe task is planned or running.
	MediaProbePending MediaProbeStatus = "pending"
	// MediaProbePassed means safe media facts were extracted and accepted.
	MediaProbePassed MediaProbeStatus = "passed"
	// MediaProbeFailed means probing failed or media facts were rejected.
	MediaProbeFailed MediaProbeStatus = "failed"
	// MediaProbeSkipped means probing was intentionally skipped, usually
	// because the media pipeline is disabled for this environment.
	MediaProbeSkipped MediaProbeStatus = "skipped"
)

// Valid reports whether the probe status is one of the known safe values.
func (s MediaProbeStatus) Valid() bool {
	switch s {
	case MediaProbeUnknown, MediaProbePending, MediaProbePassed, MediaProbeFailed, MediaProbeSkipped:
		return true
	default:
		return false
	}
}

// NormalizeMediaProbeStatus coerces an empty or unknown probe status to
// MediaProbeUnknown so storage layers never persist arbitrary status strings.
func NormalizeMediaProbeStatus(status MediaProbeStatus) MediaProbeStatus {
	if status.Valid() {
		return status
	}
	return MediaProbeUnknown
}

// ArtifactMediaMetadata contains safe, normalized media facts. It must not
// contain private storage paths, raw probe output, provider payloads or prompts.
type ArtifactMediaMetadata struct {
	Width       int
	Height      int
	DurationMS  int64
	Codec       string
	Container   string
	BitrateBPS  int64
	ProbeStatus MediaProbeStatus
}

// Normalize returns a bounded copy safe to store and expose only through
// internal metadata paths.
func (m ArtifactMediaMetadata) Normalize() ArtifactMediaMetadata {
	out := ArtifactMediaMetadata{
		Codec:       normalizeMediaToken(m.Codec),
		Container:   normalizeMediaToken(m.Container),
		ProbeStatus: NormalizeMediaProbeStatus(m.ProbeStatus),
	}
	if m.Width > 0 {
		out.Width = m.Width
	}
	if m.Height > 0 {
		out.Height = m.Height
	}
	if m.DurationMS > 0 {
		out.DurationMS = m.DurationMS
	}
	if m.BitrateBPS > 0 {
		out.BitrateBPS = m.BitrateBPS
	}
	return out
}

func normalizeMediaToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-' || r == '.' || r == '+' || r == ',':
			b.WriteRune(r)
		case unicode.IsSpace(r):
			b.WriteByte('_')
		}
		if b.Len() >= 96 {
			break
		}
	}
	return strings.Trim(b.String(), "_.,+-")
}

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
	// ValidationPolicyVersion scopes safe reuse decisions for input media.
	ValidationPolicyVersion string `json:"-"`
	// LifecycleClass controls retention and reuse policy for stored bytes.
	LifecycleClass ArtifactLifecycleClass `json:"-"`
	// SizeBytes is the size of the original bytes.
	SizeBytes int64 `json:"size_bytes"`
	// Width is the pixel width for image/video, 0 otherwise.
	Width int `json:"width,omitempty"`
	// Height is the pixel height for image/video, 0 otherwise.
	Height int `json:"height,omitempty"`
	// DurationMS is the duration in milliseconds for video/audio.
	DurationMS int64 `json:"duration_ms,omitempty"`
	// Codec is the sanitized primary codec name from media probing.
	Codec string `json:"codec,omitempty"`
	// Container is the sanitized container/format name from media probing.
	Container string `json:"container,omitempty"`
	// BitrateBPS is the probed aggregate bitrate in bits per second.
	BitrateBPS int64 `json:"bitrate_bps,omitempty"`
	// ProbeStatus is the sanitized media-probe lifecycle status.
	ProbeStatus MediaProbeStatus `json:"probe_status,omitempty"`
	// Status is the artifact lifecycle state.
	Status ArtifactStatus `json:"status"`
	// CreatedAt is the row creation timestamp.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the last mutation timestamp.
	UpdatedAt time.Time `json:"updated_at"`
}

// ApplyMediaMetadata overlays safe media facts on the artifact.
func (a *Artifact) ApplyMediaMetadata(metadata ArtifactMediaMetadata) {
	m := metadata.Normalize()
	if m.Width > 0 {
		a.Width = m.Width
	}
	if m.Height > 0 {
		a.Height = m.Height
	}
	if m.DurationMS > 0 {
		a.DurationMS = m.DurationMS
	}
	if m.Codec != "" {
		a.Codec = m.Codec
	}
	if m.Container != "" {
		a.Container = m.Container
	}
	if m.BitrateBPS > 0 {
		a.BitrateBPS = m.BitrateBPS
	}
	a.ProbeStatus = m.ProbeStatus
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
	// Codec is the sanitized primary codec name from media probing.
	Codec string `json:"codec,omitempty"`
	// Container is the sanitized container/format name from media probing.
	Container string `json:"container,omitempty"`
	// BitrateBPS is the probed aggregate bitrate in bits per second.
	BitrateBPS int64 `json:"bitrate_bps,omitempty"`
	// ProbeStatus is the sanitized media-probe lifecycle status.
	ProbeStatus MediaProbeStatus `json:"probe_status,omitempty"`
	// LifecycleClass controls retention policy for variant bytes.
	LifecycleClass ArtifactLifecycleClass `json:"-"`
	// CreatedAt is the row creation timestamp.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the last mutation timestamp.
	UpdatedAt time.Time `json:"updated_at"`
}

// ApplyMediaMetadata overlays safe media facts on the variant.
func (v *ArtifactVariant) ApplyMediaMetadata(metadata ArtifactMediaMetadata) {
	m := metadata.Normalize()
	if m.Width > 0 {
		v.Width = m.Width
	}
	if m.Height > 0 {
		v.Height = m.Height
	}
	if m.DurationMS > 0 {
		v.DurationMS = m.DurationMS
	}
	if m.Codec != "" {
		v.Codec = m.Codec
	}
	if m.Container != "" {
		v.Container = m.Container
	}
	if m.BitrateBPS > 0 {
		v.BitrateBPS = m.BitrateBPS
	}
	v.ProbeStatus = m.ProbeStatus
}
