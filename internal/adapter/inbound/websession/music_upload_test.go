package websession

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/mediaprobe"
)

func TestUploadMusicInputStoresPrivateAccountAudio(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	accountID := uuid.New()
	artifactID := uuid.New()
	saver := &musicInputSaverStub{artifact: &domain.Artifact{ID: artifactID}}
	prober := &musicInputProberStub{metadata: musicInputMetadata("mp3", "mp3", 12345)}
	limiter := &imagePrepareLimiterStub{allowed: true}
	h.deps.MusicInputArtifacts = saver
	h.deps.MusicInputProber = prober
	h.deps.ImageJobPrepareLimiter = limiter

	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, musicUploadRequest(t, sessions, accountID, "audio/mpeg", validMP3Bytes()))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if len(limiter.keys) != 1 || limiter.keys[0] != "account:"+accountID.String() {
		t.Fatalf("limiter keys = %#v", limiter.keys)
	}
	if saver.userID != uuid.Nil || saver.accountID != accountID || saver.jobID != nil || saver.kind != domain.ArtifactKindInput || saver.mediaType != domain.MediaTypeAudio {
		t.Fatalf("unexpected save scope: %+v", saver)
	}
	if saver.mimeType != "audio/mpeg" || !bytes.Equal(saver.data, validMP3Bytes()) || saver.metadata.DurationMS != 12345 {
		t.Fatalf("unexpected saved bytes/metadata: mime=%q meta=%+v", saver.mimeType, saver.metadata)
	}
	var response musicInputUploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ArtifactID != artifactID || response.MIMEType != "audio/mpeg" || response.DurationMS != 12345 || response.SizeBytes != int64(len(validMP3Bytes())) {
		t.Fatalf("response = %+v", response)
	}
}

func TestUploadMusicInputRequiresCSRF(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	req := musicUploadRequest(t, sessions, uuid.New(), "audio/mpeg", validMP3Bytes())
	req.Header.Del("X-CSRF-Token")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestUploadMusicInputFailsClosedWithoutDependencies(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, musicUploadRequest(t, sessions, uuid.New(), "audio/mpeg", validMP3Bytes()))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestUploadMusicInputUsesSharedRateLimiter(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	h.deps.MusicInputArtifacts = &musicInputSaverStub{artifact: &domain.Artifact{ID: uuid.New()}}
	h.deps.MusicInputProber = &musicInputProberStub{metadata: musicInputMetadata("mp3", "mp3", 1000)}
	h.deps.ImageJobPrepareLimiter = &imagePrepareLimiterStub{allowed: false}
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, musicUploadRequest(t, sessions, uuid.New(), "audio/mpeg", validMP3Bytes()))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
}

func TestUploadMusicInputRejectsMIMEMismatchBeforeProbe(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	prober := &musicInputProberStub{metadata: musicInputMetadata("mp3", "mp3", 1000)}
	h.deps.MusicInputArtifacts = &musicInputSaverStub{artifact: &domain.Artifact{ID: uuid.New()}}
	h.deps.MusicInputProber = prober
	h.deps.ImageJobPrepareLimiter = &imagePrepareLimiterStub{allowed: true}
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, musicUploadRequest(t, sessions, uuid.New(), "audio/aac", validMP3Bytes()))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if prober.calls != 0 {
		t.Fatalf("probe calls = %d, want 0", prober.calls)
	}
}

func TestUploadMusicInputRejectsProbeFailure(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	h.deps.MusicInputArtifacts = &musicInputSaverStub{artifact: &domain.Artifact{ID: uuid.New()}}
	h.deps.MusicInputProber = &musicInputProberStub{err: mediaprobe.ProbeError{Reason: "audio_codec_not_allowed"}}
	h.deps.ImageJobPrepareLimiter = &imagePrepareLimiterStub{allowed: true}
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, musicUploadRequest(t, sessions, uuid.New(), "audio/mpeg", validMP3Bytes()))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func musicUploadRequest(t *testing.T, sessions *sessionStub, accountID uuid.UUID, contentType string, body []byte) *http.Request {
	t.Helper()
	req := authenticatedConversationRequest(t, http.MethodPost, "/web/v1/music-inputs", sessions, accountID)
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Origin", "https://app.example.test")
	req.Header.Set("X-CSRF-Token", "csrf")
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
	return req
}

func validMP3Bytes() []byte { return []byte{'I', 'D', '3', 4, 0, 0, 0, 0, 0, 0, 'd', 'a', 't', 'a'} }

func musicInputMetadata(container, codec string, duration int64) domain.ArtifactMediaMetadata {
	return domain.ArtifactMediaMetadata{Container: container, Codec: codec, DurationMS: duration, BitrateBPS: 128000, ProbeStatus: domain.MediaProbePassed}
}

type musicInputSaverStub struct {
	artifact  *domain.Artifact
	err       error
	userID    uuid.UUID
	accountID uuid.UUID
	jobID     *uuid.UUID
	kind      domain.ArtifactKind
	mediaType domain.MediaType
	mimeType  string
	data      []byte
	metadata  domain.ArtifactMediaMetadata
}

func (s *musicInputSaverStub) SaveBytesArtifactWithMetadataForAccount(_ context.Context, userID, accountID uuid.UUID, jobID *uuid.UUID, kind domain.ArtifactKind, mediaType domain.MediaType, mimeType string, data []byte, metadata domain.ArtifactMediaMetadata) (*domain.Artifact, error) {
	s.userID = userID
	s.accountID = accountID
	s.jobID = jobID
	s.kind = kind
	s.mediaType = mediaType
	s.mimeType = mimeType
	s.data = append([]byte(nil), data...)
	s.metadata = metadata
	if s.err != nil {
		return nil, s.err
	}
	if s.artifact == nil {
		return nil, errors.New("missing artifact")
	}
	return s.artifact, nil
}

type musicInputProberStub struct {
	metadata domain.ArtifactMediaMetadata
	err      error
	calls    int
}

func (p *musicInputProberStub) ProbeAudio(_ context.Context, _ []byte, _ int64) (domain.ArtifactMediaMetadata, error) {
	p.calls++
	return p.metadata, p.err
}
