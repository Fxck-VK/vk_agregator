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
	"errors"
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

// ReferenceImageValidationPolicyVersion scopes safe reuse of uploaded
// reference images. Bump it when validation/sanitization rules become stricter.
const ReferenceImageValidationPolicyVersion = "image_reference_v1"

var sensitiveURLPattern = regexp.MustCompile(`(?i)(https?|mock)://[^\s"'<>]+|data:[^\s"'<>]+`)

var errRemoteArtifactTooLarge = errors.New("artifactservice: remote artifact too large")

// ContentScanError marks scanner rejections as non-retryable provider content
// failures for callers that need billing-safe delivery decisions.
type ContentScanError struct {
	Err error
}

func (e ContentScanError) Error() string {
	if e.Err == nil {
		return "artifactservice: content scan rejected"
	}
	return "artifactservice: content scan rejected: " + e.Err.Error()
}

func (e ContentScanError) Unwrap() error {
	return e.Err
}

func (e ContentScanError) ProviderErrorClass() domain.ProviderErrorClass {
	return domain.ProviderErrContentRejected
}

// ObjectStore is the minimal blob storage contract the service needs. It is
// satisfied by the S3/MinIO adapter and by in-memory test doubles.
type ObjectStore interface {
	Put(ctx context.Context, bucket, key string, data []byte, contentType string) error
}

type objectReader interface {
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
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

// EnsureArtifactScanned re-checks the stored bytes before a persisted output
// artifact is reused by a retry or recovery path. Local/test wiring without a
// scanner remains a no-op; production configuration requires a scanner.
func (s *Service) EnsureArtifactScanned(ctx context.Context, artifactID uuid.UUID) error {
	if s.scanner == nil {
		return nil
	}
	artifact, err := s.repo.GetByID(ctx, artifactID)
	if err != nil {
		return fmt.Errorf("artifactservice: load artifact for rescan: %w", err)
	}
	if artifact.Status != domain.ArtifactStatusReady {
		return fmt.Errorf("artifactservice: artifact is not ready for reuse")
	}
	return s.scanStoredObject(ctx, artifact.MediaType, artifact.MimeType, artifact.StorageBucket, artifact.StorageKey)
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
	return s.SaveTextArtifactForAccount(ctx, ownerID, ownerID, jobID, kind, text)
}

// SaveTextArtifactForAccount stores a text payload for a canonical account
// owner while retaining the legacy channel user id for compatibility.
func (s *Service) SaveTextArtifactForAccount(ctx context.Context, userID, accountID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, text string) (*domain.Artifact, error) {
	return s.saveBytes(ctx, userID, accountID, jobID, kind, domain.MediaTypeText, "text/plain; charset=utf-8", []byte(text), domain.ArtifactMediaMetadata{})
}

// SaveBytesArtifact stores raw bytes as an artifact of the given media type.
func (s *Service) SaveBytesArtifact(ctx context.Context, ownerID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, mimeType string, data []byte) (*domain.Artifact, error) {
	return s.SaveBytesArtifactForAccount(ctx, ownerID, ownerID, jobID, kind, mediaType, mimeType, data)
}

// SaveBytesArtifactForAccount stores raw bytes for a canonical account owner.
func (s *Service) SaveBytesArtifactForAccount(ctx context.Context, userID, accountID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, mimeType string, data []byte) (*domain.Artifact, error) {
	return s.saveBytes(ctx, userID, accountID, jobID, kind, mediaType, mimeType, data, domain.ArtifactMediaMetadata{})
}

// SaveBytesArtifactWithMetadata stores raw bytes with safe media facts already
// extracted by a worker-owned media pipeline.
func (s *Service) SaveBytesArtifactWithMetadata(ctx context.Context, ownerID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, mimeType string, data []byte, metadata domain.ArtifactMediaMetadata) (*domain.Artifact, error) {
	return s.SaveBytesArtifactWithMetadataForAccount(ctx, ownerID, ownerID, jobID, kind, mediaType, mimeType, data, metadata)
}

// SaveBytesArtifactWithMetadataForAccount stores raw bytes with metadata for a
// canonical account owner.
func (s *Service) SaveBytesArtifactWithMetadataForAccount(ctx context.Context, userID, accountID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, mimeType string, data []byte, metadata domain.ArtifactMediaMetadata) (*domain.Artifact, error) {
	return s.saveBytes(ctx, userID, accountID, jobID, kind, mediaType, mimeType, data, metadata)
}

// SaveAccountInputArtifact stores a browser-neutral account-owned input image.
// It records no legacy user provenance and has no job link; later preparation
// may reference it through the strict account-scoped repository APIs.
func (s *Service) SaveAccountInputArtifact(ctx context.Context, accountID uuid.UUID, mediaType domain.MediaType, mimeType string, data []byte) (*domain.Artifact, error) {
	if accountID == uuid.Nil {
		return nil, fmt.Errorf("artifactservice: account id is required")
	}
	if mediaType != domain.MediaTypeImage {
		return nil, fmt.Errorf("artifactservice: account input must be an image")
	}
	return s.saveAccountInputBytes(ctx, accountID, mediaType, mimeType, data)
}

// GetArtifactForAccount returns an artifact only for its exact canonical owner.
// A foreign or legacy-only artifact is intentionally indistinguishable from a
// missing one.
func (s *Service) GetArtifactForAccount(ctx context.Context, accountID, artifactID uuid.UUID) (*domain.Artifact, error) {
	if accountID == uuid.Nil {
		return nil, domain.ErrNotFound
	}
	return s.repo.GetByIDForAccount(ctx, accountID, artifactID)
}

// FindReusableInputReferenceForAccount is the strict canonical-owner reuse API
// for account-native input uploads.
func (s *Service) FindReusableInputReferenceForAccount(ctx context.Context, accountID uuid.UUID, sha256, mimeType string) (*domain.Artifact, error) {
	if accountID == uuid.Nil {
		return nil, domain.ErrNotFound
	}
	return s.repo.FindReusableInputReferenceForAccount(ctx, accountID, sha256, ReferenceImageValidationPolicyVersion, mimeType)
}

// SaveVariantWithMetadata stores a derived rendition of an existing artifact.
// The variant row is idempotent by (artifact_id, variant_type): retrying the
// same worker step returns the existing row instead of creating duplicates.
func (s *Service) SaveVariantWithMetadata(ctx context.Context, artifact *domain.Artifact, variantType domain.VariantType, mimeType string, data []byte, metadata domain.ArtifactMediaMetadata) (*domain.ArtifactVariant, error) {
	if artifact == nil || artifact.ID == uuid.Nil {
		return nil, fmt.Errorf("artifactservice: variant parent missing")
	}
	if existing, err := s.findVariant(ctx, artifact.ID, variantType); err == nil {
		if err := s.scanStoredObject(ctx, artifact.MediaType, existing.MimeType, existing.StorageBucket, existing.StorageKey); err != nil {
			return nil, err
		}
		return existing, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	if s.scanner != nil {
		if err := s.scanner.Scan(ctx, artifact.MediaType, mimeType, data); err != nil {
			return nil, ContentScanError{Err: err}
		}
	}

	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	ownerID := artifact.OwnerAccountID
	if ownerID == uuid.Nil {
		ownerID = artifact.OwnerUserID
	}
	key := fmt.Sprintf("artifacts/%s/%s/%s-%s.%s", ownerID, artifact.ID, variantType, sha, extFor(artifact.MediaType))
	if err := s.store.Put(ctx, s.bucket, key, data, mimeType); err != nil {
		return nil, fmt.Errorf("artifactservice: store variant object: %w", err)
	}

	variant := &domain.ArtifactVariant{
		ID:             uuid.New(),
		ArtifactID:     artifact.ID,
		VariantType:    variantType,
		StorageBucket:  s.bucket,
		StorageKey:     key,
		MimeType:       mimeType,
		SizeBytes:      int64(len(data)),
		LifecycleClass: domain.ArtifactLifecycleDeliveryVariant,
	}
	variant.ApplyMediaMetadata(metadata)
	if err := s.repo.AddVariant(ctx, variant); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			existing, findErr := s.findVariant(ctx, artifact.ID, variantType)
			if findErr != nil {
				return nil, findErr
			}
			if scanErr := s.scanStoredObject(ctx, artifact.MediaType, existing.MimeType, existing.StorageBucket, existing.StorageKey); scanErr != nil {
				return nil, scanErr
			}
			return existing, nil
		}
		return nil, fmt.Errorf("artifactservice: record variant: %w", err)
	}
	return variant, nil
}

func (s *Service) scanStoredObject(ctx context.Context, mediaType domain.MediaType, mimeType, bucket, key string) error {
	if s.scanner == nil {
		return nil
	}
	reader, ok := s.store.(objectReader)
	if !ok {
		return fmt.Errorf("artifactservice: object store cannot read bytes for rescan")
	}
	data, err := reader.GetObject(ctx, bucket, key)
	if err != nil {
		return fmt.Errorf("artifactservice: read stored bytes for rescan: %w", err)
	}
	if err := s.scanner.Scan(ctx, mediaType, mimeType, data); err != nil {
		return ContentScanError{Err: err}
	}
	return nil
}

func (s *Service) findVariant(ctx context.Context, artifactID uuid.UUID, variantType domain.VariantType) (*domain.ArtifactVariant, error) {
	variants, err := s.repo.ListVariants(ctx, artifactID)
	if err != nil {
		return nil, err
	}
	for _, variant := range variants {
		if variant != nil && variant.VariantType == variantType {
			return variant, nil
		}
	}
	return nil, domain.ErrNotFound
}

// SaveRemoteArtifact downloads a remote URL (e.g. a provider output) and stores
// it as an artifact. The content type from the response fills in an empty mime.
func (s *Service) SaveRemoteArtifact(ctx context.Context, ownerID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, url string) (*domain.Artifact, error) {
	return s.SaveRemoteArtifactForAccount(ctx, ownerID, ownerID, jobID, kind, mediaType, url)
}

// SaveRemoteArtifactForAccount downloads a remote URL and stores it for a
// canonical account owner.
func (s *Service) SaveRemoteArtifactForAccount(ctx context.Context, userID, accountID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, url string) (*domain.Artifact, error) {
	return s.SaveRemoteArtifactWithMetadataForAccount(ctx, userID, accountID, jobID, kind, mediaType, url, domain.ArtifactMediaMetadata{})
}

// SaveRemoteArtifactWithMetadata downloads a provider output and stores it with
// safe metadata produced by the worker-owned media pipeline.
func (s *Service) SaveRemoteArtifactWithMetadata(ctx context.Context, ownerID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, url string, metadata domain.ArtifactMediaMetadata) (*domain.Artifact, error) {
	return s.SaveRemoteArtifactWithMetadataForAccount(ctx, ownerID, ownerID, jobID, kind, mediaType, url, metadata)
}

// SaveRemoteArtifactWithMetadataForAccount downloads a provider output and
// stores it for a canonical account owner.
func (s *Service) SaveRemoteArtifactWithMetadataForAccount(ctx context.Context, userID, accountID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, url string, metadata domain.ArtifactMediaMetadata) (*domain.Artifact, error) {
	ownerID := artifactOwnerID(userID, accountID)
	ctx, span := tracing.Start(ctx, "artifact.download",
		attribute.String("owner.id", ownerID.String()),
		attribute.String("owner.user_id", userID.String()),
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
	return s.saveBytes(ctx, userID, ownerID, jobID, kind, mediaType, contentType, data, metadata)
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

// saveBytes computes the content hash, reuses only policy-compatible input
// reference images, uploads new bytes and records artifact metadata.
func (s *Service) saveBytes(ctx context.Context, userID, accountID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, mimeType string, data []byte, metadata domain.ArtifactMediaMetadata) (*domain.Artifact, error) {
	ownerID := artifactOwnerID(userID, accountID)
	ctx, span := tracing.Start(ctx, "artifact.store",
		attribute.String("owner.id", ownerID.String()),
		attribute.String("owner.user_id", userID.String()),
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
	lifecycleClass, validationPolicyVersion := classifyArtifact(kind, mediaType, domain.ArtifactStatusReady)

	if validationPolicyVersion != "" {
		if existing, err := s.repo.FindReusableInputReference(ctx, ownerID, sha, validationPolicyVersion, mimeType); err == nil {
			span.SetAttributes(attribute.Bool("artifact.dedup_hit", true))
			return existing, nil
		} else if !errors.Is(err, domain.ErrNotFound) {
			tracing.RecordError(span, err)
			return nil, err
		}
	}

	// Scan new content before it is persisted or delivered (audit ST1).
	if s.scanner != nil {
		if err := s.scanner.Scan(ctx, mediaType, mimeType, data); err != nil {
			scanErr := ContentScanError{Err: err}
			tracing.RecordError(span, scanErr)
			return nil, scanErr
		}
	}

	artifactID := uuid.New()
	key := fmt.Sprintf("artifacts/%s/%s-%s.%s", ownerID, artifactID, sha, extFor(mediaType))
	if err := s.store.Put(ctx, s.bucket, key, data, mimeType); err != nil {
		tracing.RecordError(span, err)
		return nil, fmt.Errorf("artifactservice: store object: %w", err)
	}

	artifact := &domain.Artifact{
		ID:                      artifactID,
		OwnerUserID:             userID,
		OwnerAccountID:          ownerID,
		JobID:                   jobID,
		Kind:                    kind,
		MediaType:               mediaType,
		MimeType:                mimeType,
		StorageBucket:           s.bucket,
		StorageKey:              key,
		SHA256:                  sha,
		ValidationPolicyVersion: validationPolicyVersion,
		LifecycleClass:          lifecycleClass,
		SizeBytes:               int64(len(data)),
		Status:                  domain.ArtifactStatusReady,
	}
	artifact.ApplyMediaMetadata(metadata)
	if err := s.repo.Create(ctx, artifact); err != nil {
		tracing.RecordError(span, err)
		return nil, fmt.Errorf("artifactservice: record artifact: %w", err)
	}
	span.SetAttributes(attribute.String("artifact.id", artifact.ID.String()))
	return artifact, nil
}

func (s *Service) saveAccountInputBytes(ctx context.Context, accountID uuid.UUID, mediaType domain.MediaType, mimeType string, data []byte) (*domain.Artifact, error) {
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	lifecycleClass, validationPolicyVersion := classifyArtifact(domain.ArtifactKindInput, mediaType, domain.ArtifactStatusReady)
	if existing, err := s.repo.FindReusableInputReferenceForAccount(ctx, accountID, sha, validationPolicyVersion, mimeType); err == nil {
		return existing, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if s.scanner != nil {
		if err := s.scanner.Scan(ctx, mediaType, mimeType, data); err != nil {
			return nil, ContentScanError{Err: err}
		}
	}
	artifactID := uuid.New()
	key := fmt.Sprintf("artifacts/%s/%s-%s.%s", accountID, artifactID, sha, extFor(mediaType))
	if err := s.store.Put(ctx, s.bucket, key, data, mimeType); err != nil {
		return nil, fmt.Errorf("artifactservice: store object: %w", err)
	}
	artifact := &domain.Artifact{
		ID:                      artifactID,
		OwnerUserID:             uuid.Nil,
		OwnerAccountID:          accountID,
		Kind:                    domain.ArtifactKindInput,
		MediaType:               mediaType,
		MimeType:                mimeType,
		StorageBucket:           s.bucket,
		StorageKey:              key,
		SHA256:                  sha,
		ValidationPolicyVersion: validationPolicyVersion,
		LifecycleClass:          lifecycleClass,
		SizeBytes:               int64(len(data)),
		Status:                  domain.ArtifactStatusReady,
	}
	if err := s.repo.Create(ctx, artifact); err != nil {
		return nil, fmt.Errorf("artifactservice: record artifact: %w", err)
	}
	return artifact, nil
}

func artifactOwnerID(userID, accountID uuid.UUID) uuid.UUID {
	if accountID != uuid.Nil {
		return accountID
	}
	return userID
}

func classifyArtifact(kind domain.ArtifactKind, mediaType domain.MediaType, status domain.ArtifactStatus) (domain.ArtifactLifecycleClass, string) {
	class := domain.NormalizeArtifactLifecycleClass("", kind, mediaType, status)
	if class == domain.ArtifactLifecycleInputReference {
		return class, ReferenceImageValidationPolicyVersion
	}
	return class, ""
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
	resolver     ipResolver
	dialContext  dialContextFunc
}

type ipResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type defaultIPResolver struct{}

func (defaultIPResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IPAddr{{IP: ip}}, nil
	}
	return net.DefaultResolver.LookupIPAddr(ctx, host)
}

type dialContextFunc func(ctx context.Context, network, address string) (net.Conn, error)

// newHTTPDownloader builds the default SSRF-hardened downloader.
func newHTTPDownloader() *httpDownloader {
	baseDialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	d := &httpDownloader{
		blockPrivate: true,
		resolver:     defaultIPResolver{},
		dialContext:  baseDialer.DialContext,
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = d.dialPublicContext
	d.client = &http.Client{
		Timeout:   60 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			return d.guard(req.Context(), req.URL)
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

func (d *httpDownloader) dialPublicContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("artifactservice: parse dial address %q: %w", address, err)
	}
	if err := d.guardHost(host); err != nil {
		return nil, err
	}
	if !d.blockPrivate {
		return d.dial(ctx, network, address)
	}
	ips, err := d.publicIPs(ctx, host)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, ip := range ips {
		if !networkAllowsIP(network, ip) {
			continue
		}
		conn, err := d.dial(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, fmt.Errorf("artifactservice: dial public address for %q: %w", host, lastErr)
	}
	return nil, fmt.Errorf("artifactservice: no public address for %q matches network %q", host, network)
}

func (d *httpDownloader) dial(ctx context.Context, network, address string) (net.Conn, error) {
	if d.dialContext != nil {
		return d.dialContext(ctx, network, address)
	}
	var dialer net.Dialer
	return dialer.DialContext(ctx, network, address)
}

// guard validates a URL against the SSRF policy.
func (d *httpDownloader) guard(ctx context.Context, u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("artifactservice: blocked url scheme %q", u.Scheme)
	}
	host := strings.ToLower(u.Hostname())
	if err := d.guardHost(host); err != nil {
		return err
	}
	if !d.blockPrivate {
		return nil
	}
	_, err := d.publicIPs(ctx, host)
	return err
}

func (d *httpDownloader) guardHost(host string) error {
	host = strings.ToLower(host)
	if host == "" {
		return fmt.Errorf("artifactservice: missing host")
	}
	if len(d.allowedHosts) > 0 {
		if _, ok := d.allowedHosts[host]; !ok {
			return fmt.Errorf("artifactservice: host %q not in egress allowlist", host)
		}
	}
	return nil
}

func (d *httpDownloader) publicIPs(ctx context.Context, host string) ([]net.IP, error) {
	resolver := d.resolver
	if resolver == nil {
		resolver = defaultIPResolver{}
	}
	addrs, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("artifactservice: resolve %q: %w", host, err)
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		if addr.IP == nil {
			continue
		}
		if isBlockedIP(addr.IP) {
			return nil, fmt.Errorf("artifactservice: blocked non-public address %s", addr.IP)
		}
		ips = append(ips, addr.IP)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("artifactservice: resolve %q: no ip addresses", host)
	}
	return ips, nil
}

func networkAllowsIP(network string, ip net.IP) bool {
	switch network {
	case "tcp4":
		return ip.To4() != nil
	case "tcp6":
		return ip.To4() == nil
	default:
		return true
	}
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
	if err := d.guard(ctx, u); err != nil {
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
	data, err := readRemoteBody(resp.Body, maxRemoteBytes)
	if err != nil {
		return nil, "", err
	}
	return data, resp.Header.Get("Content-Type"), nil
}

func readRemoteBody(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errRemoteArtifactTooLarge
	}
	return data, nil
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
