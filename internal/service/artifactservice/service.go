// Package artifactservice persists media as Artifacts: it writes bytes to the
// object store and records normalized metadata in the artifact repository.
// Every media file becomes an Artifact before it can be delivered (invariant
// #7), so this is the single entry point used by workers and the media pipeline.
package artifactservice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// maxRemoteBytes caps how much a remote download may pull, guarding memory.
const maxRemoteBytes = 256 << 20 // 256 MiB

// ObjectStore is the minimal blob storage contract the service needs. It is
// satisfied by the S3/MinIO adapter and by in-memory test doubles.
type ObjectStore interface {
	Put(ctx context.Context, bucket, key string, data []byte, contentType string) error
}

// Downloader fetches remote content. It is abstracted so remote downloads can
// be faked in tests and hardened (SSRF allowlist) in production.
type Downloader interface {
	Download(ctx context.Context, url string) (data []byte, contentType string, err error)
}

// Service stores artifacts.
type Service struct {
	repo       domain.ArtifactRepository
	store      ObjectStore
	downloader Downloader
	bucket     string
	now        func() time.Time
}

// Option customizes a Service.
type Option func(*Service)

// WithDownloader overrides the remote downloader (defaults to an HTTP client).
func WithDownloader(d Downloader) Option {
	return func(s *Service) { s.downloader = d }
}

// New builds an artifact Service that stores bytes in the given bucket.
func New(repo domain.ArtifactRepository, store ObjectStore, bucket string, opts ...Option) *Service {
	s := &Service{
		repo:       repo,
		store:      store,
		downloader: &httpDownloader{client: http.DefaultClient},
		bucket:     bucket,
		now:        time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// SaveTextArtifact stores a text payload as an artifact.
func (s *Service) SaveTextArtifact(ctx context.Context, ownerID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, text string) (*domain.Artifact, error) {
	return s.saveBytes(ctx, ownerID, jobID, kind, domain.MediaTypeText, "text/plain; charset=utf-8", []byte(text))
}

// SaveBytesArtifact stores raw bytes as an artifact of the given media type.
func (s *Service) SaveBytesArtifact(ctx context.Context, ownerID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, mimeType string, data []byte) (*domain.Artifact, error) {
	return s.saveBytes(ctx, ownerID, jobID, kind, mediaType, mimeType, data)
}

// SaveRemoteArtifact downloads a remote URL (e.g. a provider output) and stores
// it as an artifact. The content type from the response fills in an empty mime.
func (s *Service) SaveRemoteArtifact(ctx context.Context, ownerID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, url string) (*domain.Artifact, error) {
	data, contentType, err := s.downloader.Download(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("artifactservice: download %s: %w", url, err)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return s.saveBytes(ctx, ownerID, jobID, kind, mediaType, contentType, data)
}

// saveBytes computes the content hash, deduplicates by (owner, sha256), uploads
// the bytes and records the artifact metadata.
func (s *Service) saveBytes(ctx context.Context, ownerID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, mimeType string, data []byte) (*domain.Artifact, error) {
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])

	if existing, err := s.repo.GetBySHA256(ctx, ownerID, sha); err == nil {
		return existing, nil
	}

	key := fmt.Sprintf("artifacts/%s/%s.%s", ownerID, sha, extFor(mediaType))
	if err := s.store.Put(ctx, s.bucket, key, data, mimeType); err != nil {
		return nil, fmt.Errorf("artifactservice: store object: %w", err)
	}

	artifact := &domain.Artifact{
		ID:            uuid.New(),
		OwnerUserID:   ownerID,
		JobID:         jobID,
		Kind:          kind,
		MediaType:     mediaType,
		MimeType:      mimeType,
		StorageBucket: s.bucket,
		StorageKey:    key,
		SHA256:        sha,
		SizeBytes:     int64(len(data)),
		Status:        domain.ArtifactStatusReady,
	}
	if err := s.repo.Create(ctx, artifact); err != nil {
		return nil, fmt.Errorf("artifactservice: record artifact: %w", err)
	}
	return artifact, nil
}

func extFor(mediaType domain.MediaType) string {
	switch mediaType {
	case domain.MediaTypeText:
		return "txt"
	case domain.MediaTypeImage:
		return "png"
	case domain.MediaTypeVideo:
		return "mp4"
	case domain.MediaTypeAudio:
		return "mp3"
	case domain.MediaTypeDocument:
		return "bin"
	default:
		return "bin"
	}
}

// httpDownloader is the default Downloader backed by net/http.
type httpDownloader struct {
	client *http.Client
}

func (d *httpDownloader) Download(ctx context.Context, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxRemoteBytes))
	if err != nil {
		return nil, "", err
	}
	return data, resp.Header.Get("Content-Type"), nil
}
