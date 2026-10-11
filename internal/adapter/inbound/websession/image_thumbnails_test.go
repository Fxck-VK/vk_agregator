package websession

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/resultservice"
)

type imageThumbnailStub struct {
	calls int
	err   error
}

func (s *imageThumbnailStub) GetThumbnail(context.Context, *domain.Artifact) ([]byte, error) {
	s.calls++
	return []byte("private thumbnail"), s.err
}

func thumbnailHandlerFixture(t *testing.T) (*Handler, *sessionStub, uuid.UUID, *domain.Artifact, *imageArtifactObjectStoreStub, *imageThumbnailStub) {
	t.Helper()
	h, _, sessions := newImageJobTestHandler(t)
	owner, jobID := uuid.New(), uuid.New()
	a := &domain.Artifact{ID: uuid.New(), OwnerAccountID: owner, JobID: &jobID, Kind: domain.ArtifactKindOutput, MediaType: domain.MediaTypeImage, MimeType: "image/png", Status: domain.ArtifactStatusReady, StorageBucket: "private", StorageKey: "original", SizeBytes: 8}
	h.deps.ImageArtifacts = &imageArtifactReaderStub{artifact: a}
	h.deps.ImageJobReader = &imageJobReaderStub{job: &domain.Job{ID: jobID, AccountID: owner, Source: "web", OperationType: domain.OperationImageGenerate, Modality: domain.ModalityImage, Status: domain.JobStatusSucceeded, CostEstimate: 5,
		Params: mustMarshalWebImageJobParams(t, webImageJobParams{Prompt: "image prompt", ModelID: "synthetic-model", ModelName: "model"})}}
	h.deps.ImageResults = &imageResultReaderStub{result: resultservice.Result{ID: jobID, Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, Status: domain.JobStatusSucceeded,
		Artifacts: []resultservice.ArtifactMetadata{{ID: a.ID, MediaType: domain.MediaTypeImage, MIMEType: "image/png", SizeBytes: 8}}}}
	objects := &imageArtifactObjectStoreStub{data: []byte("original")}
	thumbnail := &imageThumbnailStub{}
	h.deps.ImageArtifactURLSigner, h.deps.ImageThumbnails = objects, thumbnail
	return h, sessions, owner, a, objects, thumbnail
}

func TestWebImageThumbnailServesAuthorizedVariantWithoutReadingOriginal(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			h, sessions, owner, a, objects, thumbnail := thumbnailHandlerFixture(t)
			req := authenticatedConversationRequest(t, method, "/web/v1/image-artifacts/"+a.ID.String()+"?preview=1", sessions, owner)
			req.Header.Set("Range", "bytes=0-2")
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)
			if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/webp" || rec.Header().Get("Cache-Control") != "private, no-store" || rec.Header().Get("Content-Length") != "17" {
				t.Fatalf("status=%d headers=%v body=%q", rec.Code, rec.Header(), rec.Body.String())
			}
			if method == http.MethodGet && rec.Body.String() != "private thumbnail" || method == http.MethodHead && rec.Body.Len() != 0 {
				t.Fatalf("body=%q", rec.Body.String())
			}
			if objects.key != "" || thumbnail.calls != 1 || rec.Header().Get("Location") != "" {
				t.Fatal("thumbnail downloaded or redirected to the original")
			}
		})
	}
}

func TestWebImageThumbnailAuthorizesBeforeReadingPrivateVariant(t *testing.T) {
	for _, scenario := range []string{"foreign owner", "deleted", "excluded result", "unmoderated result", "other source", "unauthenticated"} {
		t.Run(scenario, func(t *testing.T) {
			h, sessions, owner, a, objects, thumbnail := thumbnailHandlerFixture(t)
			switch scenario {
			case "foreign owner":
				a.OwnerAccountID = uuid.New()
			case "deleted":
				a.Status = domain.ArtifactStatusDeleted
			case "excluded result":
				h.deps.ImageResults.(*imageResultReaderStub).result.Artifacts = nil
			case "unmoderated result":
				h.deps.ImageResults.(*imageResultReaderStub).result.Status = domain.JobStatusPrepared
			case "other source":
				h.deps.ImageJobReader.(*imageJobReaderStub).job.Source = "vk"
			}
			req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/image-artifacts/"+a.ID.String()+"?preview=1", sessions, owner)
			want := http.StatusNotFound
			if scenario == "unauthenticated" {
				req.Header.Del("Cookie")
				want = http.StatusUnauthorized
			}
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)
			if rec.Code != want || thumbnail.calls != 0 || objects.key != "" {
				t.Fatalf("status=%d thumbnail calls=%d original key=%q", rec.Code, thumbnail.calls, objects.key)
			}
		})
	}
}

func TestWebImageThumbnailFailureDoesNotReturnOriginalBytes(t *testing.T) {
	h, sessions, owner, a, objects, thumbnail := thumbnailHandlerFixture(t)
	thumbnail.err = errors.New("storage unavailable")
	req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/image-artifacts/"+a.ID.String()+"?preview=1", sessions, owner)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || objects.key != "" || thumbnail.calls != 1 {
		t.Fatalf("status=%d calls=%d original key=%q", rec.Code, thumbnail.calls, objects.key)
	}
}

func TestWebImageOriginalRemainsAvailableWithoutThumbnailService(t *testing.T) {
	h, sessions, owner, a, objects, _ := thumbnailHandlerFixture(t)
	h.deps.ImageThumbnails = nil
	req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/image-artifacts/"+a.ID.String(), sessions, owner)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != "original" || objects.key != "original" || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("original status=%d body=%q", rec.Code, rec.Body.String())
	}
}
