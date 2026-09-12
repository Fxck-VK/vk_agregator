package providerreference

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/videoreference"
)

type testFixture struct {
	ctx       context.Context
	jobs      *memory.JobRepo
	artifacts *memory.ArtifactRepo
	objects   *memory.ObjectStore
	service   *Service
	now       time.Time
	job       *domain.Job
	artifact  *domain.Artifact
	body      []byte
}

func TestServeHTTPValidRead(t *testing.T) {
	f := newFixture(t)
	rec := f.serve(t, http.MethodGet, f.mustURL(t))
	if rec.Header().Get("Cache-Control") != "private, no-store" || rec.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("private reference cache policy missing")
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != string(f.body) {
		t.Fatalf("body = %q, want %q", rec.Body.String(), string(f.body))
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "video/mp4") {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestServeHTTPRangeRead(t *testing.T) {
	f := newFixture(t)
	req := httptest.NewRequest(http.MethodGet, f.mustURL(t), nil)
	req.Header.Set("Range", "bytes=1-3")
	rec := httptest.NewRecorder()

	f.service.ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != string(f.body[1:4]) {
		t.Fatalf("range body = %q", rec.Body.String())
	}
}

func TestServeHTTPValidHead(t *testing.T) {
	f := newFixture(t)
	rec := f.serve(t, http.MethodHead, f.mustURL(t))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("HEAD returned body %q", rec.Body.String())
	}
}

func TestServeHTTPRejectsTamperedSignature(t *testing.T) {
	f := newFixture(t)
	u := parseTestURL(t, f.mustURL(t))
	q := u.Query()
	q.Set("signature", "00"+q.Get("signature")[2:])
	u.RawQuery = q.Encode()

	rec := f.serve(t, http.MethodGet, u.String())
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestServeHTTPRejectsExpiredToken(t *testing.T) {
	f := newFixture(t)
	signed := f.mustURL(t)
	f.service.now = func() time.Time { return f.now.Add(DefaultTTL + time.Second) }

	rec := f.serve(t, http.MethodGet, signed)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestServeHTTPRejectsUnownedArtifact(t *testing.T) {
	f := newFixture(t)
	f.artifact.OwnerAccountID = uuid.New()
	if err := f.artifacts.Update(f.ctx, f.artifact); err != nil {
		t.Fatalf("update artifact: %v", err)
	}

	rec := f.serve(t, http.MethodGet, f.mustURL(t))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestServeHTTPRejectsNotBoundArtifact(t *testing.T) {
	f := newFixture(t)
	f.job.InputArtifactIDs = nil
	if err := f.jobs.Update(f.ctx, f.job); err != nil {
		t.Fatalf("update job: %v", err)
	}

	rec := f.serve(t, http.MethodGet, f.mustURL(t))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestServeHTTPRejectsMissingRows(t *testing.T) {
	f := newFixture(t)
	signed, err := f.service.URL(uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("url: %v", err)
	}

	rec := f.serve(t, http.MethodGet, signed)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestServeHTTPRejectsTerminalJob(t *testing.T) {
	f := newFixture(t)
	if err := f.jobs.UpdateStatus(f.ctx, f.job.ID, domain.JobStatusProviderSubmitted, domain.JobStatusSucceeded, "", ""); err != nil {
		t.Fatalf("update job status: %v", err)
	}

	rec := f.serve(t, http.MethodGet, f.mustURL(t))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestNewRejectsUnsafeConfig(t *testing.T) {
	jobs := memory.NewJobRepo()
	artifacts := memory.NewArtifactRepo()
	objects := memory.NewObjectStore()
	for name, tc := range map[string]struct {
		baseURL string
		secret  string
	}{
		"http":       {baseURL: "http://provider.example.com", secret: strings.Repeat("s", MinSecretLength)},
		"localhost":  {baseURL: "https://localhost", secret: strings.Repeat("s", MinSecretLength)},
		"private_ip": {baseURL: "https://10.0.0.1", secret: strings.Repeat("s", MinSecretLength)},
		"short_key":  {baseURL: "https://provider.example.com", secret: "short"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := New(tc.baseURL, tc.secret, jobs, artifacts, objects); err == nil {
				t.Fatal("New succeeded, want error")
			}
		})
	}
}

func newFixture(t *testing.T) *testFixture {
	t.Helper()
	ctx := context.Background()
	jobs := memory.NewJobRepo()
	artifacts := memory.NewArtifactRepo()
	objects := memory.NewObjectStore()
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	service, err := New("https://provider.example.com", strings.Repeat("s", MinSecretLength), jobs, artifacts, objects)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	service.now = func() time.Time { return now }

	accountID := uuid.New()
	artifactID := uuid.New()
	artifact := &domain.Artifact{
		ID:             artifactID,
		OwnerAccountID: accountID,
		Kind:           domain.ArtifactKindInput,
		MediaType:      domain.MediaTypeVideo,
		MimeType:       "video/mp4",
		StorageBucket:  "artifacts",
		StorageKey:     "provider/ref.mp4",
		SizeBytes:      18,
		Width:          1280,
		Height:         720,
		DurationMS:     5000,
		Codec:          "h264",
		Container:      "mp4",
		BitrateBPS:     4_000_000,
		ProbeStatus:    domain.MediaProbePassed,
		Status:         domain.ArtifactStatusReady,
	}
	if err := artifacts.Create(ctx, artifact); err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	body := []byte("0123456789abcdef")
	if err := objects.Put(ctx, artifact.StorageBucket, artifact.StorageKey, body, artifact.MimeType); err != nil {
		t.Fatalf("put object: %v", err)
	}

	params := motionParams(t, artifactID)
	job := &domain.Job{
		ID:               uuid.New(),
		UserID:           accountID,
		AccountID:        accountID,
		Source:           "test",
		ResultMode:       domain.ResultModeAccountHistory,
		OperationType:    domain.OperationVideoGenerate,
		Modality:         domain.ModalityVideo,
		Status:           domain.JobStatusProviderSubmitted,
		IdempotencyKey:   uuid.NewString(),
		InputArtifactIDs: []uuid.UUID{artifactID},
		Params:           params,
	}
	if err := jobs.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	return &testFixture{
		ctx:       ctx,
		jobs:      jobs,
		artifacts: artifacts,
		objects:   objects,
		service:   service,
		now:       now,
		job:       job,
		artifact:  artifact,
		body:      body,
	}
}

func motionParams(t *testing.T, artifactID uuid.UUID) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"video_route_alias":           string(domain.VideoRouteKling26Motion),
		"reference_video_artifact_id": artifactID.String(),
		"resolved_video_route": domain.VideoRouteSnapshot{
			Alias:                    domain.VideoRouteKling26Motion,
			Provider:                 domain.ProviderAPIMart,
			ProviderModelID:          "kling-v2-6-motion-control",
			ModelClass:               "kling_2_6_motion_control",
			DurationSec:              videoreference.MaxDurationSec,
			ReferenceVideoArtifactID: artifactID.String(),
			InternalCostCredits:      1,
		},
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	return raw
}

func (f *testFixture) mustURL(t *testing.T) string {
	t.Helper()
	signed, err := f.service.URL(f.job.ID, f.artifact.ID)
	if err != nil {
		t.Fatalf("URL: %v", err)
	}
	return signed
}

func (f *testFixture) serve(t *testing.T, method, signed string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, signed, nil)
	rec := httptest.NewRecorder()
	f.service.ServeHTTP(rec, req)
	return rec
}

func parseTestURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	return u
}
