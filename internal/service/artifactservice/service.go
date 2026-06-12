// Package artifactservice persists media as Artifacts: it writes bytes to the
// object store and records normalized metadata in the artifact repository.
// Every media file becomes an Artifact before it can be delivered (invariant
// #7), so this is the single entry point used by workers and the media pipeline.
package artifactservice

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/tracing"
)

// maxRemoteBytes caps how much a remote download may pull, guarding memory.
const maxRemoteBytes = 256 << 20 // 256 MiB

var sensitiveURLPattern = regexp.MustCompile(`(?i)(https?|mock)://[^\s"'<>]+|data:[^\s"'<>]+`)

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

// Scanner inspects artifact bytes before they are stored and returns an error
// to reject unsafe content (malware, disallowed media). The default is no
// scanning; inject a real scanner (e.g. an antivirus or content-safety service)
// via WithScanner (audit ST1).
type Scanner interface {
	Scan(ctx context.Context, mediaType domain.MediaType, mimeType string, data []byte) error
}

// Service stores artifacts.
type Service struct {
	repo       domain.ArtifactRepository
	store      ObjectStore
	downloader Downloader
	scanner    Scanner
	bucket     string
	now        func() time.Time
}

// Option customizes a Service.
type Option func(*Service)

// WithDownloader overrides the remote downloader (defaults to a SSRF-hardened
// HTTP client).
func WithDownloader(d Downloader) Option {
	return func(s *Service) { s.downloader = d }
}

// NewHTTPDownloader returns the default SSRF-hardened remote downloader. It is
// exposed for wrappers that need to handle synthetic URLs while preserving the
// same HTTP egress policy for real provider URLs.
func NewHTTPDownloader() Downloader {
	return newHTTPDownloader()
}

// WithAllowedHosts restricts the default downloader to an egress allowlist of
// hostnames (case-insensitive). Empty means "any public host" (private/
// loopback/link-local addresses are still blocked).
func WithAllowedHosts(hosts ...string) Option {
	return func(s *Service) {
		if d, ok := s.downloader.(*httpDownloader); ok {
			d.setAllowedHosts(hosts)
		}
	}
}

// WithScanner installs a content scanner that runs on new artifact bytes before
// they are stored (audit ST1).
func WithScanner(sc Scanner) Option {
	return func(s *Service) { s.scanner = sc }
}

// New builds an artifact Service that stores bytes in the given bucket.
func New(repo domain.ArtifactRepository, store ObjectStore, bucket string, opts ...Option) *Service {
	s := &Service{
		repo:       repo,
		store:      store,
		downloader: newHTTPDownloader(),
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
	return s.saveBytes(ctx, ownerID, jobID, kind, domain.MediaTypeText, "text/plain; charset=utf-8", []byte(text), domain.ArtifactMediaMetadata{})
}

// SaveBytesArtifact stores raw bytes as an artifact of the given media type.
func (s *Service) SaveBytesArtifact(ctx context.Context, ownerID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, mimeType string, data []byte) (*domain.Artifact, error) {
	return s.saveBytes(ctx, ownerID, jobID, kind, mediaType, mimeType, data, domain.ArtifactMediaMetadata{})
}

// SaveBytesArtifactWithMetadata stores raw bytes with safe media facts already
// extracted by a worker-owned media pipeline.
func (s *Service) SaveBytesArtifactWithMetadata(ctx context.Context, ownerID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, mimeType string, data []byte, metadata domain.ArtifactMediaMetadata) (*domain.Artifact, error) {
	return s.saveBytes(ctx, ownerID, jobID, kind, mediaType, mimeType, data, metadata)
}

// SaveRemoteArtifact downloads a remote URL (e.g. a provider output) and stores
// it as an artifact. The content type from the response fills in an empty mime.
func (s *Service) SaveRemoteArtifact(ctx context.Context, ownerID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, url string) (*domain.Artifact, error) {
	return s.SaveRemoteArtifactWithMetadata(ctx, ownerID, jobID, kind, mediaType, url, domain.ArtifactMediaMetadata{})
}

// SaveRemoteArtifactWithMetadata downloads a provider output and stores it with
// safe metadata produced by the worker-owned media pipeline.
func (s *Service) SaveRemoteArtifactWithMetadata(ctx context.Context, ownerID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, url string, metadata domain.ArtifactMediaMetadata) (*domain.Artifact, error) {
	ctx, span := tracing.Start(ctx, "artifact.download",
		attribute.String("owner.id", ownerID.String()),
		attribute.String("artifact.kind", string(kind)),
		attribute.String("artifact.media_type", string(mediaType)),
	)
	if jobID != nil {
		span.SetAttributes(attribute.String("job.id", jobID.String()))
	}
	data, contentType, err := s.downloader.Download(ctx, url)
	if err != nil {
		err = safeDownloadError(err)
		tracing.RecordError(span, err)
		span.End()
		return nil, err
	}
	span.SetAttributes(attribute.Int("artifact.download_bytes", len(data)))
	span.End()
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return s.saveBytes(ctx, ownerID, jobID, kind, mediaType, contentType, data, metadata)
}

func safeDownloadError(err error) error {
	if err == nil {
		return nil
	}
	msg := sensitiveURLPattern.ReplaceAllString(err.Error(), "[redacted-url]")
	if strings.TrimSpace(msg) == "" {
		msg = "download failed"
	}
	return fmt.Errorf("artifactservice: download remote artifact: %s", msg)
}

// saveBytes computes the content hash, deduplicates by (owner, sha256), uploads
// the bytes and records the artifact metadata.
func (s *Service) saveBytes(ctx context.Context, ownerID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, mimeType string, data []byte, metadata domain.ArtifactMediaMetadata) (*domain.Artifact, error) {
	ctx, span := tracing.Start(ctx, "artifact.store",
		attribute.String("owner.id", ownerID.String()),
		attribute.String("artifact.kind", string(kind)),
		attribute.String("artifact.media_type", string(mediaType)),
		attribute.String("artifact.mime_type", mimeType),
		attribute.Int("artifact.size_bytes", len(data)),
	)
	defer span.End()
	if jobID != nil {
		span.SetAttributes(attribute.String("job.id", jobID.String()))
	}

	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])

	if existing, err := s.repo.GetBySHA256(ctx, ownerID, sha); err == nil {
		span.SetAttributes(attribute.Bool("artifact.dedup_hit", true))
		return existing, nil
	}

	// Scan new content before it is persisted or delivered (audit ST1).
	if s.scanner != nil {
		if err := s.scanner.Scan(ctx, mediaType, mimeType, data); err != nil {
			tracing.RecordError(span, err)
			return nil, fmt.Errorf("artifactservice: content scan rejected: %w", err)
		}
	}

	key := fmt.Sprintf("artifacts/%s/%s.%s", ownerID, sha, extFor(mediaType))
	if err := s.store.Put(ctx, s.bucket, key, data, mimeType); err != nil {
		tracing.RecordError(span, err)
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
	artifact.ApplyMediaMetadata(metadata)
	if err := s.repo.Create(ctx, artifact); err != nil {
		tracing.RecordError(span, err)
		return nil, fmt.Errorf("artifactservice: record artifact: %w", err)
	}
	span.SetAttributes(attribute.String("artifact.id", artifact.ID.String()))
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

// httpDownloader is the default Downloader backed by net/http. It is hardened
// against SSRF: only http/https are allowed, every URL (including redirect
// targets) is validated, and requests to private/loopback/link-local addresses
// are refused. An optional host allowlist narrows egress further.
type httpDownloader struct {
	client       *http.Client
	allowedHosts map[string]struct{}
	blockPrivate bool
}

// newHTTPDownloader builds the default SSRF-hardened downloader.
func newHTTPDownloader() *httpDownloader {
	d := &httpDownloader{blockPrivate: true}
	d.client = &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			return d.guard(req.URL)
		},
	}
	return d
}

func (d *httpDownloader) setAllowedHosts(hosts []string) {
	if len(hosts) == 0 {
		d.allowedHosts = nil
		return
	}
	m := make(map[string]struct{}, len(hosts))
	for _, h := range hosts {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			m[h] = struct{}{}
		}
	}
	d.allowedHosts = m
}

// guard validates a URL against the SSRF policy.
func (d *httpDownloader) guard(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("artifactservice: blocked url scheme %q", u.Scheme)
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return fmt.Errorf("artifactservice: missing host")
	}
	if len(d.allowedHosts) > 0 {
		if _, ok := d.allowedHosts[host]; !ok {
			return fmt.Errorf("artifactservice: host %q not in egress allowlist", host)
		}
	}
	if !d.blockPrivate {
		return nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("artifactservice: resolve %q: %w", host, err)
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("artifactservice: blocked non-public address %s", ip)
		}
	}
	return nil
}

// isBlockedIP reports whether an IP is in a range that must not be reached from
// the artifact downloader (SSRF protection).
func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() {
		return true
	}
	// 100.64.0.0/10 carrier-grade NAT (not covered by IsPrivate).
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return true
	}
	return false
}

func (d *httpDownloader) Download(ctx context.Context, rawURL string) ([]byte, string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", fmt.Errorf("artifactservice: parse url: %w", err)
	}
	if u.Scheme == "data" {
		return decodeDataURL(rawURL)
	}
	if err := d.guard(u); err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxRemoteBytes))
	if err != nil {
		return nil, "", err
	}
	return data, resp.Header.Get("Content-Type"), nil
}

func decodeDataURL(raw string) ([]byte, string, error) {
	const prefix = "data:"
	if !strings.HasPrefix(raw, prefix) {
		return nil, "", fmt.Errorf("artifactservice: invalid data url")
	}
	headerAndData := strings.SplitN(raw[len(prefix):], ",", 2)
	if len(headerAndData) != 2 {
		return nil, "", fmt.Errorf("artifactservice: malformed data url")
	}
	header := headerAndData[0]
	payload := headerAndData[1]
	contentType := "text/plain;charset=US-ASCII"
	if header != "" {
		parts := strings.Split(header, ";")
		if parts[0] != "" {
			contentType = parts[0]
		}
		if parts[len(parts)-1] == "base64" {
			data, err := base64.StdEncoding.DecodeString(payload)
			if err != nil {
				return nil, "", fmt.Errorf("artifactservice: decode data url: %w", err)
			}
			if len(data) > maxRemoteBytes {
				return nil, "", fmt.Errorf("artifactservice: data url too large")
			}
			return data, contentType, nil
		}
	}
	data := []byte(payload)
	if len(data) > maxRemoteBytes {
		return nil, "", fmt.Errorf("artifactservice: data url too large")
	}
	return data, contentType, nil
}
