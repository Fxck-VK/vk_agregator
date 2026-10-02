package websession

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

func configureVideoExpansionTest(t *testing.T, h *Handler) {
	t.Helper()
	if err := providermodels.ConfigureDEVSmoke("development", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = providermodels.ConfigureDEVSmoke("development", false) })
	prices, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if err := prices.AddSupplemental(providermodels.RuntimeRegistry().DEVSmokePrices()); err != nil {
		t.Fatal(err)
	}
	runtime, err := productcatalog.FromConfig(config.Config{Env: "development", FeatureDEVModelSmokeEnabled: true, FeatureVideoRouterEnabled: true, APIMartProviderEnabled: true, APIMartAPIKey: "test-key", APIMartBaseURL: "https://api.example.com"}, prices)
	if err != nil {
		t.Fatal(err)
	}
	h.cfg.VideoRoutes, h.deps.ImagePricing = runtime.VideoRoutes(), prices
}

func TestViduWebReferencesAndGeminiAutomaticQuote(t *testing.T) {
	h, conversations, sessions, jobs, _ := newWebConversationMessageTestHandler(t)
	configureVideoExpansionTest(t, h)
	store := &webInputStoreStub{}
	h.deps.InputArtifacts, h.deps.InputObjects = store, store
	owner := uuid.New()
	conv := seedWebMessageConversation(t, conversations, owner, domain.ConversationSourceWeb)
	var pngData bytes.Buffer
	_ = png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 128, 128)))
	artifact, _ := store.SaveAccountInputArtifact(context.Background(), owner, domain.MediaTypeImage, "image/png", pngData.Bytes())
	jobs.job = &domain.Job{ID: uuid.New(), Status: domain.JobStatusQueued}
	send := func(model string, ids []uuid.UUID, seconds int) *httptest.ResponseRecorder {
		payload, _ := json.Marshal(map[string]any{"prompt": "A paper boat on a pond", "model_id": model, "reference_artifact_ids": ids, "duration_sec": seconds})
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, safeWebConversationMessageRequest(t, sessions, owner, conv.ID, uuid.New(), string(payload)))
		return rec
	}
	if rec := send("vidu_q3", nil, 5); rec.Code != 400 || len(jobs.inputs) != 0 {
		t.Fatal("reference-only model accepted missing images")
	}
	var tooMany []uuid.UUID
	for range 8 {
		tooMany = append(tooMany, uuid.New())
	}
	if rec := send("vidu_q3", tooMany, 5); rec.Code != 400 || len(jobs.inputs) != 0 {
		t.Fatal("reference count bound not enforced before ownership lookup")
	}
	if rec := send("vidu_q3", []uuid.UUID{artifact.ID}, 5); rec.Code != 201 {
		t.Fatalf("Vidu reference submission: %d %s", rec.Code, rec.Body.String())
	}
	if len(jobs.inputs[0].InputArtifactIDs) != 1 || jobs.inputs[0].InputArtifactIDs[0] != artifact.ID || jobs.inputs[0].PricingSnapshot.InternalCredits != 240 {
		t.Fatal("Vidu quote or job binding lost")
	}
	store.artifact.OwnerAccountID = uuid.New()
	if rec := send("vidu_q3_mix", []uuid.UUID{artifact.ID}, 5); rec.Code != 404 || len(jobs.inputs) != 1 {
		t.Fatal("foreign image queued")
	}
	store.artifact.OwnerAccountID = owner
	if rec := send("vidu_q3", []uuid.UUID{artifact.ID, artifact.ID}, 5); rec.Code != 400 || len(jobs.inputs) != 1 {
		t.Fatal("duplicate image queued")
	}
	if rec := send("flux_3_video", []uuid.UUID{artifact.ID}, 5); rec.Code != 400 {
		t.Fatal("unwired video inputs accepted")
	}
	if rec := send("gemini_omni_flash_preview", nil, 5); rec.Code != 400 {
		t.Fatal("Gemini accepted manual duration")
	}
	if rec := send("gemini_omni_flash_preview", nil, 0); rec.Code != 201 {
		t.Fatalf("Gemini automatic submission: %d %s", rec.Code, rec.Body.String())
	}
	if jobs.inputs[len(jobs.inputs)-1].PricingSnapshot.InternalCredits != 530 {
		t.Fatal("incorrect bounded Gemini quote")
	}
}

func TestViduWebUploadValidatesGeometryAndSession(t *testing.T) {
	h, conversations, sessions, _, _ := newWebConversationMessageTestHandler(t)
	configureVideoExpansionTest(t, h)
	store := &webInputStoreStub{}
	h.deps.InputArtifacts, h.deps.InputObjects = store, store
	owner := uuid.New()
	conv := seedWebMessageConversation(t, conversations, owner, domain.ConversationSourceWeb)
	for _, tc := range []struct {
		width, height int
		csrf          bool
		status        int
	}{
		{128, 128, false, 403}, {127, 128, true, 400}, {128, 513, true, 400}, {513, 128, true, 400}, {128, 128, true, 201}, {128, 512, true, 201},
	} {
		var data, body bytes.Buffer
		if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, tc.width, tc.height))); err != nil {
			t.Fatal(err)
		}
		writer := multipart.NewWriter(&body)
		part, _ := writer.CreateFormFile("file", "synthetic.png")
		_, _ = part.Write(data.Bytes())
		_ = writer.Close()
		req := safeWebConversationMessageRequest(t, sessions, owner, conv.ID, uuid.New(), "")
		req.URL.Path, req.URL.RawQuery = "/web/v1/input-artifacts", "model_id=vidu_q3"
		req.Body, req.ContentLength = ioNopCloser(&body), int64(body.Len())
		req.Header.Set("Content-Type", writer.FormDataContentType())
		if !tc.csrf {
			req.Header.Del("X-CSRF-Token")
		}
		before := store.saves
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("%dx%d csrf=%t: status %d", tc.width, tc.height, tc.csrf, rec.Code)
		}
		if tc.status != 201 && store.saves != before {
			t.Fatal("rejected bytes persisted")
		}
	}
}
