package miniapp

import (
	"fmt"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/modelcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
)

func TestMiniAppImageModelsExposeOnlyPublicCatalogFields(t *testing.T) {
	cfg := config.Config{
		APIMartProviderEnabled:                true,
		APIMartAPIKey:                         "configured",
		APIMartBaseURL:                        "https://apimart.test",
		PoYoProviderEnabled:                   true,
		PoYoAPIKey:                            "configured",
		PoYoBaseURL:                           "https://poyo.test",
		FeatureImageModelNanoBanana2Enabled:   true,
		FeatureImageModelNanoBananaProEnabled: true,
		FeatureImageModelGPTImage2Enabled:     true,
	}
	runtimeCatalog, err := productcatalog.FromConfig(cfg)
	if err != nil {
		t.Fatalf("build product catalog: %v", err)
	}
	models := miniAppImageModels(runtimeCatalog.Catalog)
	if len(models) != 3 {
		t.Fatalf("expected enabled public image models, got %+v", models)
	}

	var sawNanoBanana2 bool
	for _, model := range models {
		if model.Type != "image" || model.ID == "" || model.Name == "" || model.Description == "" || !model.Enabled || model.EstimateCredits <= 0 {
			t.Fatalf("missing public image catalog fields: %+v", model)
		}
		serialized := strings.ToLower(fmt.Sprintf("%+v", model))
		for _, private := range []string{"model_code", "provider", "nano-banana-2-new", "gemini-3-pro-image-preview"} {
			if strings.Contains(serialized, private) {
				t.Fatalf("image catalog leaked private field %q: %+v", private, model)
			}
		}
		if model.ID == modelcatalog.MiniAppImageNanoBanana2 {
			sawNanoBanana2 = true
			if model.DefaultQuality != modelcatalog.ImageQuality1K || len(model.QualityOptions) != 3 {
				t.Fatalf("missing Nano Banana 2 quality options: %+v", model)
			}
			if !model.SupportsReferenceImage || model.MaxReferenceImages != 4 {
				t.Fatalf("missing Nano Banana 2 reference limits: %+v", model)
			}
		}
	}
	if !sawNanoBanana2 {
		t.Fatal("Nano Banana 2 public model was not exposed")
	}
}

func TestMiniAppVideoRoutesExposeOnlyPublicCatalogFields(t *testing.T) {
	cfg := config.Config{
		FeatureVideoRouterEnabled:               true,
		PoYoProviderEnabled:                     true,
		PoYoAPIKey:                              "configured",
		PoYoBaseURL:                             "https://poyo.test",
		FeatureVideoRouteKlingO3StandardEnabled: true,
	}
	runtimeCatalog, err := productcatalog.FromConfig(cfg)
	if err != nil {
		t.Fatalf("build product catalog: %v", err)
	}
	routes := miniAppVideoRoutes(runtimeCatalog.Catalog)
	if len(routes) != 1 {
		t.Fatalf("expected one public video route, got %+v", routes)
	}
	route := routes[0]
	if route.Type != "video" || route.Alias != string(domain.VideoRouteKlingO3Standard) || route.Name == "" || route.Description == "" || !route.Enabled || route.EstimateCredits <= 0 {
		t.Fatalf("missing public video route fields: %+v", route)
	}
	if len(route.AllowedDurationsSec) == 0 || len(route.AllowedAspectRatios) == 0 || route.DefaultDurationSec == 0 {
		t.Fatalf("missing public video route constraints: %+v", route)
	}
	serialized := strings.ToLower(fmt.Sprintf("%+v", route))
	for _, private := range []string{"model_code", "provider", "provider_model_id", "kling-o3/standard"} {
		if strings.Contains(serialized, private) {
			t.Fatalf("video route leaked private field %q: %+v", private, route)
		}
	}
}
