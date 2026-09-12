// Package providerreference serves short-lived signed provider media reads.
package providerreference

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/videoreference"
)

const (
	PathPrefix      = "/provider-references/"
	MaxTTL          = time.Hour
	DefaultTTL      = MaxTTL
	MinSecretLength = 32
)

var (
	ErrInvalidConfig = errors.New("providerreference: invalid config")
	ErrInvalidURL    = errors.New("providerreference: invalid url")
	ErrForbidden     = errors.New("providerreference: forbidden")
	ErrNotFound      = errors.New("providerreference: not found")
	ErrTooLarge      = errors.New("providerreference: object too large")
)

// ObjectStore is the current project object-read API.
type ObjectStore interface {
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
}

// Service signs and serves provider reference URLs.
type Service struct {
	baseURL   url.URL
	secret    []byte
	jobs      domain.JobRepository
	artifacts domain.ArtifactRepository
	objects   ObjectStore
	now       func() time.Time
}

// New validates gateway configuration and dependencies.
func New(baseURL, secret string, jobs domain.JobRepository, artifacts domain.ArtifactRepository, objects ObjectStore) (*Service, error) {
	base, err := parseBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if err := ValidateConfig(baseURL, secret); err != nil || jobs == nil || artifacts == nil || objects == nil {
		return nil, ErrInvalidConfig
	}
	return &Service{
		baseURL:   *base,
		secret:    []byte(secret),
		jobs:      jobs,
		artifacts: artifacts,
		objects:   objects,
		now:       time.Now,
	}, nil
}

// ValidateConfig verifies public gateway signing configuration without requiring
// repository or object-store dependencies.
func ValidateConfig(baseURL, secret string) error {
	if _, err := parseBaseURL(baseURL); err != nil {
		return ErrInvalidConfig
	}
	if len([]byte(secret)) < MinSecretLength {
		return ErrInvalidConfig
	}
	return nil
}

// URL signs a short-lived provider URL for one job-bound reference artifact.
func (s *Service) URL(jobID, artifactID uuid.UUID) (string, error) {
	if s == nil || jobID == uuid.Nil || artifactID == uuid.Nil {
		return "", ErrInvalidURL
	}
	expires := s.now().Add(DefaultTTL).Unix()
	expiresValue := strconv.FormatInt(expires, 10)
	path := referencePath(jobID, artifactID)

	u := s.baseURL
	u.Path = path
	u.RawPath = ""
	q := url.Values{}
	q.Set("expires", expiresValue)
	q.Set("signature", s.signature(path, expiresValue))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// ServeHTTP verifies a signed URL, authorizes job/artifact binding and serves bytes.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if s == nil {
		writeSafeError(w, http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeSafeError(w, http.StatusMethodNotAllowed)
		return
	}
	jobID, artifactID, err := parseReferencePath(r.URL.Path)
	if err != nil {
		writeSafeError(w, http.StatusNotFound)
		return
	}
	if err := s.verifyRequest(r.URL.Path, r.URL.Query()); err != nil {
		writeSafeError(w, http.StatusForbidden)
		return
	}

	ctx := r.Context()
	job, err := s.jobs.GetByID(ctx, jobID)
	if err != nil {
		writeSafeError(w, statusForLoadError(err))
		return
	}
	artifact, err := s.artifacts.GetByID(ctx, artifactID)
	if err != nil {
		writeSafeError(w, statusForLoadError(err))
		return
	}
	params, err := parseJobParams(job.Params)
	if err != nil {
		writeSafeError(w, http.StatusForbidden)
		return
	}
	if err := authorize(job, artifact, artifactID, params); err != nil {
		writeSafeError(w, statusForAuthError(err))
		return
	}

	data, err := s.objects.GetObject(ctx, artifact.StorageBucket, artifact.StorageKey)
	if err != nil {
		writeSafeError(w, statusForLoadError(err))
		return
	}
	if int64(len(data)) > videoreference.MaxBytes {
		writeSafeError(w, http.StatusRequestEntityTooLarge)
		return
	}

	w.Header().Set("Content-Type", artifact.MimeType)
	modTime := artifact.UpdatedAt
	if modTime.IsZero() {
		modTime = artifact.CreatedAt
	}
	if modTime.IsZero() {
		modTime = s.now()
	}
	http.ServeContent(w, r, artifact.ID.String()+".mp4", modTime, bytes.NewReader(data))
}

func (s *Service) verifyRequest(path string, query url.Values) error {
	expiresValues, ok := query["expires"]
	if !ok || len(expiresValues) != 1 {
		return ErrForbidden
	}
	signatureValues, ok := query["signature"]
	if !ok || len(signatureValues) != 1 || len(query) != 2 {
		return ErrForbidden
	}
	expires, err := strconv.ParseInt(expiresValues[0], 10, 64)
	if err != nil {
		return ErrForbidden
	}
	now := s.now().Unix()
	if expires <= now || expires > s.now().Add(MaxTTL).Unix() {
		return ErrForbidden
	}
	got, err := hex.DecodeString(signatureValues[0])
	if err != nil || len(got) != sha256.Size {
		return ErrForbidden
	}
	want, err := hex.DecodeString(s.signature(path, expiresValues[0]))
	if err != nil || subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrForbidden
	}
	return nil
}

func (s *Service) signature(path, expires string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(path))
	mac.Write([]byte("\n"))
	mac.Write([]byte(expires))
	return hex.EncodeToString(mac.Sum(nil))
}

func authorize(job *domain.Job, artifact *domain.Artifact, artifactID uuid.UUID, params jobParams) error {
	if job == nil || artifact == nil {
		return ErrNotFound
	}
	if job.AccountID == uuid.Nil || artifact.OwnerAccountID != job.AccountID {
		return ErrNotFound
	}
	if job.OperationType != domain.OperationVideoGenerate || job.Modality != domain.ModalityVideo {
		return ErrForbidden
	}
	if !statusAllowsProviderDownload(job.Status) {
		return ErrForbidden
	}
	if !isMotionRoute(params) {
		return ErrForbidden
	}
	if !containsUUID(job.InputArtifactIDs, artifactID) || !params.referenceMatches(artifactID) {
		return ErrNotFound
	}
	maxDurationSec := params.ResolvedVideoRoute.DurationSec
	if maxDurationSec <= 0 {
		maxDurationSec = videoreference.MaxDurationSec
	}
	if _, err := videoreference.Validate(artifact, job.AccountID, maxDurationSec); err != nil {
		if errors.Is(err, videoreference.ErrNotFound) {
			return ErrNotFound
		}
		return ErrForbidden
	}
	if strings.TrimSpace(artifact.StorageBucket) == "" || strings.TrimSpace(artifact.StorageKey) == "" {
		return ErrNotFound
	}
	return nil
}

func statusAllowsProviderDownload(status domain.JobStatus) bool {
	switch status {
	case domain.JobStatusDispatchingProvider,
		domain.JobStatusProviderSubmitted,
		domain.JobStatusProviderPending,
		domain.JobStatusProviderProcessing:
		return true
	default:
		return false
	}
}

type jobParams struct {
	VideoRouteAlias          string                    `json:"video_route_alias"`
	ReferenceVideoArtifactID string                    `json:"reference_video_artifact_id"`
	ResolvedVideoRoute       domain.VideoRouteSnapshot `json:"resolved_video_route"`
}

func parseJobParams(raw json.RawMessage) (jobParams, error) {
	if len(raw) == 0 {
		return jobParams{}, ErrForbidden
	}
	var params jobParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return jobParams{}, ErrForbidden
	}
	return params, nil
}

func isMotionRoute(params jobParams) bool {
	routeAlias := domain.VideoRouteAlias(strings.TrimSpace(params.VideoRouteAlias))
	snapshotAlias := params.ResolvedVideoRoute.Alias
	if routeAlias != "" && routeAlias != domain.VideoRouteKling26Motion {
		return false
	}
	if snapshotAlias != "" && snapshotAlias != domain.VideoRouteKling26Motion {
		return false
	}
	return routeAlias == domain.VideoRouteKling26Motion || snapshotAlias == domain.VideoRouteKling26Motion
}

func (p jobParams) referenceMatches(artifactID uuid.UUID) bool {
	for _, value := range []string{p.ReferenceVideoArtifactID, p.ResolvedVideoRoute.ReferenceVideoArtifactID} {
		id, err := uuid.Parse(strings.TrimSpace(value))
		if err == nil && id == artifactID {
			return true
		}
	}
	return false
}

func containsUUID(values []uuid.UUID, want uuid.UUID) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func referencePath(jobID, artifactID uuid.UUID) string {
	return PathPrefix + jobID.String() + "/" + artifactID.String() + ".mp4"
}

func parseReferencePath(path string) (uuid.UUID, uuid.UUID, error) {
	rest := strings.TrimPrefix(path, PathPrefix)
	if rest == path {
		return uuid.Nil, uuid.Nil, ErrNotFound
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || !strings.HasSuffix(parts[1], ".mp4") {
		return uuid.Nil, uuid.Nil, ErrNotFound
	}
	jobID, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, uuid.Nil, ErrNotFound
	}
	artifactID, err := uuid.Parse(strings.TrimSuffix(parts[1], ".mp4"))
	if err != nil {
		return uuid.Nil, uuid.Nil, ErrNotFound
	}
	return jobID, artifactID, nil
}

func parseBaseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, ErrInvalidConfig
	}
	if u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, ErrInvalidConfig
	}
	if path := strings.Trim(u.EscapedPath(), "/"); path != "" {
		return nil, ErrInvalidConfig
	}
	if !publicHost(u.Hostname()) {
		return nil, ErrInvalidConfig
	}
	u.Path = ""
	u.RawPath = ""
	return u, nil
}

func publicHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsGlobalUnicast() &&
			!ip.IsPrivate() &&
			!ip.IsLoopback() &&
			!ip.IsLinkLocalUnicast() &&
			!ip.IsLinkLocalMulticast() &&
			!ip.IsMulticast() &&
			!ip.IsUnspecified()
	}
	if !strings.Contains(host, ".") || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if !validDNSLabel(label) {
			return false
		}
	}
	return true
}

func validDNSLabel(label string) bool {
	if len(label) == 0 || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return false
	}
	for _, r := range label {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	return true
}

func statusForLoadError(err error) int {
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, ErrNotFound) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

func statusForAuthError(err error) int {
	if errors.Is(err, ErrNotFound) {
		return http.StatusNotFound
	}
	return http.StatusForbidden
}

func writeSafeError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}
