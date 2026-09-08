package websession

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/imagegeneration"
)

func TestWebQwenImagePrepareUsesPrivateRouteAndServerPrice(t *testing.T) {
	for _, quality := range []string{"1K", "2K"} {
		t.Run(quality, func(t *testing.T) {
			h, jobs, sessions := newImageJobTestHandler(t)
			h.cfg.ImageModels = []imagegeneration.PublicModel{{ID: "qwen_image_3", Name: "Qwen Image 3.0", Enabled: true, Ready: true, QualityOptions: []string{"1K", "2K"}, DefaultQuality: "1K", SupportsReferenceImage: true, MaxReferenceImages: 3}}
			owner := uuid.New()
			req := safeImageMutationRequest(t, sessions, owner, http.MethodPost, "/web/v1/image-jobs/prepare", map[string]string{"prompt": "Synthetic test image", "model_id": "qwen_image_3", "image_quality": quality})
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)
			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d", rec.Code)
			}
			if jobs.prepareCalls != 1 || jobs.prepareInput.AccountID != owner || jobs.prepareInput.CostEstimateCredits != 15 || jobs.prepareInput.PricingSnapshot.InternalCredits != 15 {
				t.Fatal("wrong owner or server quote")
			}
			var params webImageJobParams
			if err := json.Unmarshal(jobs.prepareInput.Params, &params); err != nil {
				t.Fatal(err)
			}
			if params.Provider != domain.ProviderAPIMart || params.ModelCode != "qwen-image-3.0" || params.ModelID != "qwen_image_3" || params.Resolution != quality {
				t.Fatal("wrong private Job snapshot")
			}
			assertSafeWebImagePreparation(t, rec.Body.Bytes(), jobs.prepared.ID, domain.JobStatusPrepared, 15, 104)
			for _, secret := range []string{"provider", "model_code", "apimart", "qwen-image-3.0", "floor", "multiplier"} {
				if bytes.Contains(bytes.ToLower(rec.Body.Bytes()), []byte(secret)) {
					t.Fatalf("private field %s in public DTO", secret)
				}
			}
		})
	}
}
