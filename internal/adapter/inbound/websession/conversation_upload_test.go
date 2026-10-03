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
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/modelcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
)

type webInputStoreStub struct {
	artifact    *domain.Artifact
	artifacts   map[uuid.UUID]*domain.Artifact
	data        []byte
	objects     map[string][]byte
	objectReads int
	saves       int
}

func (s *webInputStoreStub) SaveAccountInputArtifact(_ context.Context, owner uuid.UUID, media domain.MediaType, mime string, data []byte) (*domain.Artifact, error) {
	s.saves++
	s.data = append([]byte(nil), data...)
	s.artifact = &domain.Artifact{ID: uuid.New(), OwnerAccountID: owner, Kind: domain.ArtifactKindInput, MediaType: media, MimeType: mime, Status: domain.ArtifactStatusReady, SizeBytes: int64(len(data)), StorageBucket: "artifacts", StorageKey: "synthetic"}
	s.addArtifact(s.artifact, data)
	return s.artifact, nil
}
func (s *webInputStoreStub) GetArtifactForAccount(_ context.Context, owner, id uuid.UUID) (*domain.Artifact, error) {
	if s.artifacts != nil {
		a := s.artifacts[id]
		if a == nil || a.OwnerAccountID != owner {
			return nil, domain.ErrNotFound
		}
		return a, nil
	}
	if s.artifact == nil || s.artifact.ID != id || s.artifact.OwnerAccountID != owner {
		return nil, domain.ErrNotFound
	}
	return s.artifact, nil
}
func (s *webInputStoreStub) GetObject(_ context.Context, bucket, key string) ([]byte, error) {
	s.objectReads++
	if s.objects != nil {
		data, ok := s.objects[bucket+"/"+key]
		if !ok {
			return nil, domain.ErrNotFound
		}
		return data, nil
	}
	return s.data, nil
}

func (s *webInputStoreStub) addArtifact(a *domain.Artifact, data []byte) {
	if s.artifacts == nil {
		s.artifacts = map[uuid.UUID]*domain.Artifact{}
	}
	if s.objects == nil {
		s.objects = map[string][]byte{}
	}
	s.artifacts[a.ID] = a
	s.objects[a.StorageBucket+"/"+a.StorageKey] = append([]byte(nil), data...)
}

func (s *webInputStoreStub) addSizedArtifact(owner uuid.UUID, size int64) uuid.UUID {
	id := uuid.New()
	a := &domain.Artifact{ID: id, OwnerAccountID: owner, Kind: domain.ArtifactKindInput, MediaType: domain.MediaTypeImage, MimeType: "image/png", Status: domain.ArtifactStatusReady, SizeBytes: size, StorageBucket: "artifacts", StorageKey: id.String()}
	s.addArtifact(a, []byte("not read before aggregate reject"))
	return id
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
	if store.objectReads != 1 {
		t.Fatalf("reference byte recheck reads = %d, want 1", store.objectReads)
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

func TestConversationReferencesRejectAggregateSizeBeforeObjectRead(t *testing.T) {
	h, conversations, sessions, jobs, _ := newWebConversationMessageTestHandler(t)
	images, _, _ := newImageJobTestHandler(t)
	h.deps.ImagePricing = images.deps.ImagePricing
	h.cfg.ImageModels = []imagegeneration.PublicModel{gptImage2ReferencePublicModel(t)}
	store := &webInputStoreStub{}
	h.deps.InputArtifacts, h.deps.InputObjects = store, store
	owner := uuid.New()
	conv := seedWebMessageConversation(t, conversations, owner, domain.ConversationSourceWeb)
	jobs.job = &domain.Job{ID: uuid.New(), Status: domain.JobStatusQueued}

	ids := make([]uuid.UUID, 13)
	for i := range ids {
		ids[i] = store.addSizedArtifact(owner, productcatalog.WebReferenceMaxBytes)
	}
	rec := submitConversationMedia(t, h, sessions, owner, conv.ID, map[string]any{
		"prompt":                 "Synthetic aggregate references",
		"model_id":               modelcatalog.MiniAppImageGPTImage2,
		"image_quality":          modelcatalog.ImageQuality1K,
		"aspect_ratio":           "1:1",
		"reference_artifact_ids": ids,
	})
	if rec.Code != http.StatusBadRequest || store.objectReads != 0 || len(jobs.inputs) != 0 {
		t.Fatalf("aggregate reject = status %d object reads %d jobs %d body %s", rec.Code, store.objectReads, len(jobs.inputs), rec.Body.String())
	}
}

func TestConversationReferenceAggregateLimitBoundary(t *testing.T) {
	atLimit := make([]*domain.Artifact, 0, 13)
	for i := 0; i < 12; i++ {
		atLimit = append(atLimit, &domain.Artifact{SizeBytes: productcatalog.WebReferenceMaxBytes})
	}
	atLimit = append(atLimit, &domain.Artifact{SizeBytes: 16 << 20})
	if !conversationReferencesWithinTotalByteLimit(modelcatalog.MiniAppImageGPTImage2, atLimit) {
		t.Fatal("GPT Image 2 references at 256 MiB rejected")
	}
	overLimit := append(append([]*domain.Artifact(nil), atLimit...), &domain.Artifact{SizeBytes: 1})
	if conversationReferencesWithinTotalByteLimit(modelcatalog.MiniAppImageGPTImage2, overLimit) {
		t.Fatal("GPT Image 2 references over 256 MiB accepted")
	}
}

func gptImage2ReferencePublicModel(t *testing.T) imagegeneration.PublicModel {
	t.Helper()
	model, ok := modelcatalog.ResolvePublicModel(domain.OperationImageGenerate, modelcatalog.MiniAppImageGPTImage2)
	if !ok {
		t.Fatal("GPT Image 2 public model missing")
	}
	return imagegeneration.PublicModel{
		ID:                     model.ModelID,
		Name:                   model.ModelName,
		Enabled:                true,
		Ready:                  true,
		QualityOptions:         []string{modelcatalog.ImageQuality1K},
		DefaultQuality:         modelcatalog.ImageQuality1K,
		SupportsReferenceImage: true,
		MaxReferenceImages:     model.MaxReferenceImages,
		MaxOutputCount:         1,
		AllowedAspectRatios:    append([]string(nil), model.AllowedAspectRatios...),
	}
}

type readCloser struct{ *bytes.Buffer }

func (readCloser) Close() error              { return nil }
func ioNopCloser(b *bytes.Buffer) readCloser { return readCloser{b} }
