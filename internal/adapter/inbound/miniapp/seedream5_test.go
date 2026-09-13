package miniapp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"testing"
	miniapp "vk-ai-aggregator/internal/adapter/inbound/miniapp"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/productcatalog"
)

func TestNewAPIMartImagesMiniAppPriceMatchesWorker(t *testing.T) {
	runtime, err := productcatalog.FromConfig(config.Config{APIMartProviderEnabled: true, APIMartAPIKey: "test-key", APIMartBaseURL: "https://example.test/v1",
		FeatureAPIMartGPTImage25FlareEnabled: true, FeatureAPIMartGPTImage25SunburstEnabled: true, FeatureAPIMartSeedream50LiteEnabled: true, FeatureAPIMartSeedream50ProEnabled: true}, staticPricingCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range runtime.ImageModels() {
		t.Run(model.ID, func(t *testing.T) {
			fixture := newTestFixtureWithConfig("", nil, func(cfg *miniapp.Config) {
				cfg.ImageModels = []miniapp.ImageModelDTO{{ID: model.ID, Name: model.Name, Enabled: true, QualityOptions: model.QualityOptions, DefaultQuality: model.DefaultQuality, SupportsReferenceImage: model.SupportsReferenceImage, MaxReferenceImages: model.MaxReferenceImages}}
			})
			body, _ := json.Marshal(map[string]string{"operation": "image_generate", "model_id": model.ID, "image_quality": model.DefaultQuality, "prompt": "A mountain"})
			request := httptest.NewRequest(http.MethodPost, "/miniapp/jobs", bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Launch-Params", devLaunchParams(777))
			request.Header.Set("X-Idempotency-Key", model.ID)
			response := httptest.NewRecorder()
			fixture.handler.Routes().ServeHTTP(response, request)
			if response.Code != http.StatusCreated {
				t.Fatalf("create failed: %d %s", response.Code, response.Body.String())
			}
			var result struct {
				ID uuid.UUID `json:"id"`
			}
			_ = json.Unmarshal(response.Body.Bytes(), &result)
			job, err := fixture.jobRepo.GetByID(context.Background(), result.ID)
			if err != nil {
				t.Fatal(err)
			}
			var params struct{ Size, AspectRatio string }
			_ = json.Unmarshal(job.Params, &params)
			if job.CostReserved != 20 || params.Size != "1:1" {
				t.Fatal("miniapp quote and execution dimensions differ")
			}
		})
	}
}
