package websession

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/modelcatalog"
)

func TestWebGrokImageCatalogAndPrepare(t *testing.T) {
	for _, tc := range []struct{ id, model, resolution string }{
		{"grok_image_1_5", "grok-imagine-1.5-apimart", ""},
		{"grok_image_2_0", "grok-imagine-2.0-ext", "quality"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			h, jobs, sessions := newImageJobTestHandler(t)
			model, ok := modelcatalog.ResolvePublicModel(domain.OperationImageGenerate, tc.id)
			if !ok {
				t.Fatal("missing model")
			}
			h.cfg.ImageModels = []imagegeneration.PublicModel{{ID: tc.id, Name: model.ModelName, Enabled: true, Ready: true, QualityOptions: []string{"standard"}, DefaultQuality: "standard", AllowedAspectRatios: model.AllowedAspectRatios, MaxOutputCount: 1}}
			owner := uuid.New()
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, authenticatedConversationRequest(t, http.MethodGet, "/web/v1/image-models", sessions, owner))
			var catalog struct {
				Items []safeImageModel `json:"items"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &catalog); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusOK || len(catalog.Items) != 1 || !reflect.DeepEqual(catalog.Items[0].AllowedAspectRatios, model.AllowedAspectRatios) || catalog.Items[0].PriceByQuality["standard"] != 10 {
				t.Fatal("wrong public catalog limits or price")
			}
			for _, ratio := range []string{"1:1", "21:9"} {
				req := safeImageMutationRequest(t, sessions, owner, http.MethodPost, "/web/v1/image-jobs/prepare", map[string]any{"prompt": "Synthetic test image", "model_id": tc.id, "image_quality": "standard", "aspect_ratio": ratio, "output_count": 1})
				rec = httptest.NewRecorder()
				h.Routes().ServeHTTP(rec, req)
				if ratio == "21:9" {
					if rec.Code != http.StatusBadRequest || jobs.prepareCalls != 1 {
						t.Fatal("invalid ratio reached Job prepare")
					}
					continue
				}
				if rec.Code != http.StatusCreated || jobs.prepareCalls != 1 || jobs.prepareInput.AccountID != owner || jobs.prepareInput.CostEstimateCredits != 10 || jobs.prepareInput.PricingSnapshot.InternalCredits != 10 {
					t.Fatal("wrong owner or server quote")
				}
				var params webImageJobParams
				if err := json.Unmarshal(jobs.prepareInput.Params, &params); err != nil {
					t.Fatal(err)
				}
				if params.Provider != domain.ProviderAPIMart || params.ModelCode != tc.model || params.ModelID != tc.id || params.Resolution != tc.resolution {
					t.Fatal("wrong private Job snapshot")
				}
				assertSafeWebImagePreparation(t, rec.Body.Bytes(), jobs.prepared.ID, domain.JobStatusPrepared, 10, 104)
				for _, private := range []string{"provider", "model_code", "apimart", "floor", "multiplier"} {
					if bytes.Contains(bytes.ToLower(rec.Body.Bytes()), []byte(private)) {
						t.Fatalf("private field %s in public DTO", private)
					}
				}
			}
		})
	}
}
