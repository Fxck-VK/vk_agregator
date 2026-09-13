// Package videoreference validates and stores owned reference videos.
package videoreference

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/artifactservice"
	"vk-ai-aggregator/internal/service/mediaprobe"
)

const (
	DefaultArtifactBucket       = "artifacts"
	MaxBytes                    = 100 << 20
	MinDurationSec              = 3
	MaxDurationSec              = 30
	MaxDimension                = 4096
	MaxBitrateBPS         int64 = 80_000_000
	ProbeTimeout                = 10 * time.Second
)

var (
	ErrNotFound    = errors.New("videoreference: not found")
	ErrInvalid     = errors.New("videoreference: invalid")
	ErrProbeFailed = errors.New("videoreference: probe failed")
	ErrStoreFailed = errors.New("videoreference: store failed")
)

// Prober extracts safe video metadata from server-side bytes.
type Prober interface {
	ProbeVideo(ctx context.Context, data []byte, sizeBytes int64) (domain.ArtifactMediaMetadata, error)
}

// Service stores uploaded reference videos only after byte and probe checks.
type Service struct {
	repo    domain.ArtifactRepository
	objects artifactservice.ObjectStore
	probe   Prober
}

// NewProber returns the default Motion reference video prober.
func NewProber(path string) Prober {
	return mediaprobe.NewFFProbe(mediaprobe.Config{
		FFProbePath:            path,
		MaxVideoSizeBytes:      MaxBytes,
		MaxVideoDurationSec:    MaxDurationSec,
		MaxVideoWidth:          MaxDimension,
		MaxVideoHeight:         MaxDimension,
		MaxVideoBitrate:        MaxBitrateBPS,
		AllowedVideoContainers: []string{"mp4", "mov"},
		AllowedVideoCodecs:     []string{"h264", "hevc"},
		Timeout:                ProbeTimeout,
	})
}

// New builds a reference-video service.
func New(repo domain.ArtifactRepository, objects artifactservice.ObjectStore, probe Prober) *Service {
	return &Service{repo: repo, objects: objects, probe: probe}
}

// Upload validates bytes, probes metadata, then persists a private input video artifact.
func (s *Service) Upload(ctx context.Context, userID, accountID uuid.UUID, data []byte) (*domain.Artifact, error) {
	if s == nil || s.repo == nil || s.objects == nil || s.probe == nil {
		return nil, invalid("service_unavailable")
	}
	if userID == uuid.Nil && accountID == uuid.Nil {
		return nil, invalid("owner_required")
	}
	if accountID == uuid.Nil {
		accountID = userID
	}
	if len(data) == 0 {
		return nil, invalid("empty")
	}
	if int64(len(data)) > MaxBytes {
		return nil, invalid("too_large")
	}
	mimeType, ok := referenceVideoMIME(data)
	if !ok {
		return nil, invalid("unsupported_container")
	}

	metadata, err := s.probe.ProbeVideo(ctx, data, int64(len(data)))
	if err != nil {
		return nil, probeError(err)
	}
	metadata = metadata.Normalize()
	if _, err := validateMetadata(metadata, MaxDurationSec); err != nil {
		return nil, err
	}

	saver := artifactservice.New(s.repo, s.objects, DefaultArtifactBucket)
	artifact, err := saver.SaveBytesArtifactWithMetadataForAccount(ctx, userID, accountID, nil, domain.ArtifactKindInput, domain.MediaTypeVideo, mimeType, data, metadata)
	if err != nil {
		return nil, ErrStoreFailed
	}
	return artifact, nil
}

// Validate checks a stored artifact using server-side fields only and returns
// ceil(duration_ms/1000) for fixed-tariff provider requests.
func Validate(artifact *domain.Artifact, accountID uuid.UUID, maxDurationSec int) (int, error) {
	if artifact == nil || accountID == uuid.Nil || artifactOwner(artifact) != accountID {
		return 0, ErrNotFound
	}
	if artifact.Kind != domain.ArtifactKindInput {
		return 0, invalid("not_input")
	}
	if artifact.MediaType != domain.MediaTypeVideo {
		return 0, invalid("not_video")
	}
	if artifact.Status != domain.ArtifactStatusReady {
		return 0, invalid("not_ready")
	}
	if artifact.SizeBytes <= 0 || artifact.SizeBytes > MaxBytes {
		return 0, invalid("size")
	}
	if !allowedMIME(artifact.MimeType) {
		return 0, invalid("mime")
	}
	return validateMetadata(domain.ArtifactMediaMetadata{
		Width:       artifact.Width,
		Height:      artifact.Height,
		DurationMS:  artifact.DurationMS,
		Codec:       artifact.Codec,
		Container:   artifact.Container,
		BitrateBPS:  artifact.BitrateBPS,
		ProbeStatus: artifact.ProbeStatus,
	}, maxDurationSec)
}

func validateMetadata(metadata domain.ArtifactMediaMetadata, maxDurationSec int) (int, error) {
	metadata = metadata.Normalize()
	if metadata.ProbeStatus != domain.MediaProbePassed {
		return 0, invalid("probe_not_passed")
	}
	if metadata.Width <= 0 || metadata.Height <= 0 || metadata.Width > MaxDimension || metadata.Height > MaxDimension {
		return 0, invalid("dimensions")
	}
	if !allowedCodec(metadata.Codec) {
		return 0, invalid("codec")
	}
	if !allowedContainer(metadata.Container) {
		return 0, invalid("container")
	}
	if metadata.BitrateBPS <= 0 || metadata.BitrateBPS > MaxBitrateBPS {
		return 0, invalid("bitrate")
	}
	maxDurationSec = normalizeMaxDuration(maxDurationSec)
	minDurationMS := int64(MinDurationSec) * 1000
	maxDurationMS := int64(maxDurationSec) * 1000
	if metadata.DurationMS < minDurationMS {
		return 0, invalid("duration_too_short")
	}
	if metadata.DurationMS > maxDurationMS {
		return 0, invalid("duration_too_long")
	}
	return int((metadata.DurationMS + 999) / 1000), nil
}

func normalizeMaxDuration(maxDurationSec int) int {
	if maxDurationSec <= 0 || maxDurationSec > MaxDurationSec {
		return MaxDurationSec
	}
	if maxDurationSec < MinDurationSec {
		return MinDurationSec
	}
	return maxDurationSec
}

func artifactOwner(artifact *domain.Artifact) uuid.UUID {
	if artifact.OwnerAccountID != uuid.Nil {
		return artifact.OwnerAccountID
	}
	return artifact.OwnerUserID
}

func invalid(reason string) error {
	if strings.TrimSpace(reason) == "" {
		return ErrInvalid
	}
	return fmt.Errorf("%w: %s", ErrInvalid, reason)
}

func probeError(err error) error {
	var safe mediaprobe.ProbeError
	if !errors.As(err, &safe) {
		return ErrProbeFailed
	}
	switch safe.Reason {
	case "probe_failed", "probe_timeout", "ffprobe_not_configured", "probe_json_invalid":
		return ErrProbeFailed
	default:
		return invalid(safe.Reason)
	}
}

func allowedMIME(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "video/mp4", "video/quicktime":
		return true
	default:
		return false
	}
}

func allowedCodec(value string) bool {
	switch compactToken(value) {
	case "h264", "avc1", "hevc", "h265", "hvc1", "hev1":
		return true
	default:
		return false
	}
}

func allowedContainer(value string) bool {
	switch compactToken(value) {
	case "mp4", "mov", "quicktime", "qt":
		return true
	default:
		return false
	}
}

func compactToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func referenceVideoMIME(data []byte) (string, bool) {
	brands, ok := bmffBrands(data)
	if !ok {
		return "", false
	}
	quickTime := false
	for _, brand := range brands {
		if brand == "qt  " {
			quickTime = true
			continue
		}
		if acceptedMP4Brand(brand) {
			return "video/mp4", true
		}
	}
	if quickTime {
		return "video/quicktime", true
	}
	return "", false
}

func bmffBrands(data []byte) ([]string, bool) {
	if len(data) < 16 || !bytes.Equal(data[4:8], []byte("ftyp")) {
		return nil, false
	}
	boxSize := int(binary.BigEndian.Uint32(data[:4]))
	if boxSize == 1 || boxSize < 16 {
		return nil, false
	}
	if boxSize > len(data) {
		return nil, false
	}
	if boxSize == 0 {
		boxSize = len(data)
	}
	brands := []string{string(data[8:12])}
	for offset := 16; offset+4 <= boxSize; offset += 4 {
		brands = append(brands, string(data[offset:offset+4]))
	}
	return brands, true
}

func acceptedMP4Brand(brand string) bool {
	switch brand {
	case "isom", "iso2", "iso3", "iso4", "iso5", "iso6", "mp41", "mp42", "avc1", "hvc1", "hev1", "M4V ":
		return true
	default:
		return false
	}
}
