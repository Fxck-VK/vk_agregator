package websession

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/imagegeneration"
)

func TestWebFlux2PrepareAndReplayMPPrice(t *testing.T) {
	for _, tc := range []struct {
		quality string
		credits int64
	}{{"1MP", 15}, {"2MP", 25}, {"3MP", 30}, {"4MP", 40}} {
		t.Run(tc.quality, func(t *testing.T) {
			h, jobs, sessions := newImageJobTestHandler(t)
			h.cfg.ImageModels = []imagegeneration.PublicModel{{ID: "flux_2_pro", Name: "FLUX.2 Pro", Enabled: true, Ready: true, QualityOptions: []string{"1MP", "2MP", "3MP", "4MP"}, DefaultQuality: "1MP", MaxOutputCount: 1}}
			owner, key := uuid.New(), uuid.New()
			body := map[string]any{"prompt": "Synthetic image", "model_id": "flux_2_pro", "image_quality": tc.quality, "aspect_ratio": "9:21", "output_count": 1}
			req := safeImageMutationRequest(t, sessions, owner, http.MethodPost, "/web/v1/image-jobs/prepare", body)
			req.Header.Set("X-Idempotency-Key", key.String())
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)
			if rec.Code != http.StatusCreated || jobs.prepareCalls != 1 || jobs.prepareInput.AccountID != owner || jobs.prepareInput.CostEstimateCredits != tc.credits {
				t.Fatalf("incorrect prepared quote: status %d", rec.Code)
			}
			var params webImageJobParams
			if err := json.Unmarshal(jobs.prepareInput.Params, &params); err != nil {
				t.Fatal(err)
			}
			if params.Provider != domain.ProviderAPIMart || params.ModelCode != "flux-2-pro" || params.Resolution != tc.quality || params.AspectRatio != "9:21" {
				t.Fatal("incorrect worker snapshot")
			}
			assertSafeWebImagePreparation(t, rec.Body.Bytes(), jobs.prepared.ID, domain.JobStatusPrepared, tc.credits, 104)
			stored := newPreparedWebImageJobForReplay(t, owner, key, "Synthetic image", nil)
			stored.Params, stored.PricingSnapshot, stored.CostEstimate = jobs.prepareInput.Params, mustMarshalPricingSnapshot(jobs.prepareInput.PricingSnapshot), tc.credits
			h.deps.ImageJobIdempotency = &imageJobIdempotencyReaderStub{job: stored}
			changedPrice := &imagePricingSnapshotStub{snapshot: jobs.prepareInput.PricingSnapshot}
			changedPrice.snapshot.InternalCredits += 5
			h.deps.ImagePricing = changedPrice
			req = safeImageMutationRequest(t, sessions, owner, http.MethodPost, "/web/v1/image-jobs/prepare", body)
			req.Header.Set("X-Idempotency-Key", key.String())
			rec = httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)
			if rec.Code != http.StatusCreated || jobs.prepareCalls != 1 || changedPrice.calls != 0 {
				t.Fatal("replay created or repriced a 9:21 Job")
			}
			assertSafeWebImagePreparation(t, rec.Body.Bytes(), stored.ID, domain.JobStatusPrepared, tc.credits, 104)
		})
	}
}
