package websession

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

type webInputStoreStub struct {
	artifact *domain.Artifact
	data     []byte
	saves    int
}

func (s *webInputStoreStub) SaveAccountInputArtifact(_ context.Context, owner uuid.UUID, media domain.MediaType, mime string, data []byte) (*domain.Artifact, error) {
	s.saves++
	s.data = append([]byte(nil), data...)
	s.artifact = &domain.Artifact{ID: uuid.New(), OwnerAccountID: owner, Kind: domain.ArtifactKindInput, MediaType: media, MimeType: mime, Status: domain.ArtifactStatusReady, SizeBytes: int64(len(data)), StorageBucket: "artifacts", StorageKey: "synthetic"}
	return s.artifact, nil
}
func (s *webInputStoreStub) GetArtifactForAccount(_ context.Context, owner, id uuid.UUID) (*domain.Artifact, error) {
	if s.artifact == nil || s.artifact.ID != id || s.artifact.OwnerAccountID != owner {
		return nil, domain.ErrNotFound
	}
	return s.artifact, nil
}
func (s *webInputStoreStub) GetObject(context.Context, string, string) ([]byte, error) {
	return s.data, nil
}

func TestConversationUploadAndReferenceSubmission(t *testing.T) {
	h, conversations, sessions, jobs, _ := newWebConversationMessageTestHandler(t)
	images, _, _ := newImageJobTestHandler(t)
	h.cfg.ImageModels, h.deps.ImagePricing = images.cfg.ImageModels, images.deps.ImagePricing
	store := &webInputStoreStub{}
	h.deps.InputArtifacts, h.deps.InputObjects = store, store
	owner := uuid.New()
	conv := seedWebMessageConversation(t, conversations, owner, domain.ConversationSourceWeb)
	var pngData bytes.Buffer
	_ = png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	upload := func(payload []byte, model string, authorized bool) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, _ := writer.CreateFormFile("file", "synthetic.png")
		_, _ = part.Write(payload)
		_ = writer.Close()
		req := safeWebConversationMessageRequest(t, sessions, owner, conv.ID, uuid.New(), "")
		req.URL.Path = "/web/v1/input-artifacts"
		req.URL.RawQuery = "model_id=" + model
		req.Body = http.NoBody
		req.Body = ioNopCloser(&body)
		req.ContentLength = int64(body.Len())
		req.Header.Set("Content-Type", writer.FormDataContentType())
		if !authorized {
			req.Header.Del("X-CSRF-Token")
		}
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)
		return rec
	}
	if rec := upload(pngData.Bytes(), "nano_banana_2", false); rec.Code != 403 {
		t.Fatalf("csrf %d", rec.Code)
	}
	if rec := upload([]byte("not an image"), "nano_banana_2", true); rec.Code != 400 || store.saves != 0 {
		t.Fatal("invalid upload saved")
	}
	if rec := upload(pngData.Bytes(), "chatgpt", true); rec.Code != 400 || store.saves != 0 {
		t.Fatal("unsupported model accepted")
	}
	rec := upload(pngData.Bytes(), "nano_banana_2", true)
	if rec.Code != 201 || store.saves != 1 {
		t.Fatalf("upload %d %s", rec.Code, rec.Body.String())
	}
	jobs.job = &domain.Job{ID: uuid.New(), Status: domain.JobStatusQueued}
	payload := map[string]any{"prompt": "Synthetic reference", "model_id": "nano_banana_2", "image_quality": "2K", "reference_artifact_ids": []uuid.UUID{store.artifact.ID}}
	send := func() *httptest.ResponseRecorder {
		raw, _ := json.Marshal(payload)
		r := httptest.NewRecorder()
		h.Routes().ServeHTTP(r, safeWebConversationMessageRequest(t, sessions, owner, conv.ID, uuid.New(), string(raw)))
		return r
	}
	if rec = send(); rec.Code != 201 || len(jobs.inputs) != 1 {
		t.Fatalf("reference submit %d %s", rec.Code, rec.Body.String())
	}
	var params struct {
		References []uuid.UUID `json:"reference_artifact_ids"`
	}
	_ = json.Unmarshal(jobs.inputs[0].Params, &params)
	if len(params.References) != 1 || params.References[0] != store.artifact.ID {
		t.Fatal("reference lost before worker")
	}
	quote := httptest.NewRecorder()
	h.Routes().ServeHTTP(quote, authenticatedConversationRequest(t, http.MethodGet, "/web/v1/image-reference-quote?model_id=nano_banana_2&image_quality=2K&aspect_ratio=16:9&output_count=1&reference_count=1", sessions, owner))
	var price struct {
		Credits int64 `json:"credits"`
	}
	_ = json.Unmarshal(quote.Body.Bytes(), &price)
	if quote.Code != 200 || price.Credits != jobs.inputs[0].PricingSnapshot.InternalCredits {
		t.Fatalf("reference quote differs from queued price: %d", quote.Code)
	}
	for _, who := range []uuid.UUID{owner, uuid.New()} {
		preview := httptest.NewRecorder()
		h.Routes().ServeHTTP(preview, authenticatedConversationRequest(t, http.MethodGet, "/web/v1/input-artifacts/"+store.artifact.ID.String(), sessions, who))
		if who == owner {
			if preview.Code != 200 || !bytes.Equal(preview.Body.Bytes(), pngData.Bytes()) {
				t.Fatal("owned preview missing")
			}
		} else if preview.Code != 404 {
			t.Fatal("foreign preview exposed")
		}
	}
	store.artifact.OwnerAccountID = uuid.New()
	if rec = send(); rec.Code != 404 || len(jobs.inputs) != 1 {
		t.Fatal("foreign reference queued")
	}
	store.artifact.OwnerAccountID = owner
	payload["reference_artifact_ids"] = []uuid.UUID{store.artifact.ID, store.artifact.ID}
	if rec = send(); rec.Code != 400 || len(jobs.inputs) != 1 {
		t.Fatal("duplicate references queued")
	}
	payload["model_id"] = "chatgpt"
	payload["image_quality"] = ""
	payload["reference_artifact_ids"] = []uuid.UUID{store.artifact.ID}
	if rec = send(); rec.Code != 400 || len(jobs.inputs) != 1 {
		t.Fatal("text silently ignored reference")
	}
}

type readCloser struct{ *bytes.Buffer }

func (readCloser) Close() error              { return nil }
func ioNopCloser(b *bytes.Buffer) readCloser { return readCloser{b} }
