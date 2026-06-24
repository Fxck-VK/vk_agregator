package miniapp

import (
	"fmt"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/modelcatalog"
	"vk-ai-aggregator/internal/service/pricingcatalog"
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
	runtimeCatalog, err := productcatalog.FromConfig(cfg, staticPricingCatalog(t))
	if err != nil {
		t.Fatalf("build product catalog: %v", err)
	}
	models := miniAppImageModels(runtimeCatalog.Catalog)
	if len(models) != 3 {
		t.Fatalf("expected enabled public image models, got %+v", models)
	}

	sawQualityOptions := map[string]bool{}
	for _, model := range models {
		if model.Type != "image" || model.ID == "" || model.Name == "" || model.Description == "" || !model.Enabled || model.EstimateCredits <= 0 {
			t.Fatalf("missing public image catalog fields: %+v", model)
		}
		serialized := strings.ToLower(fmt.Sprintf("%+v", model))
		for _, private := range []string{"model_code", "provider", "nano-banana-2-new", "gemini-3-pro-image-preview", "gpt-image-2"} {
			if strings.Contains(serialized, private) {
				t.Fatalf("image catalog leaked private field %q: %+v", private, model)
			}
		}
		switch model.ID {
		case modelcatalog.MiniAppImageNanoBanana2, modelcatalog.MiniAppImageGPTImage2:
			if model.DefaultQuality != modelcatalog.ImageQuality1K || len(model.QualityOptions) != 3 {
				t.Fatalf("missing image quality options: %+v", model)
			}
			sawQualityOptions[model.ID] = true
		case modelcatalog.MiniAppImageNanoBananaPro:
			if model.DefaultQuality != modelcatalog.ImageQuality1K || len(model.QualityOptions) != 2 ||
				model.QualityOptions[0] != modelcatalog.ImageQuality1K ||
				model.QualityOptions[1] != modelcatalog.ImageQuality4K {
				t.Fatalf("missing priced image quality options: %+v", model)
			}
			sawQualityOptions[model.ID] = true
		}
		if model.ID == modelcatalog.MiniAppImageNanoBanana2 {
			if !model.SupportsReferenceImage || model.MaxReferenceImages != 4 {
				t.Fatalf("missing Nano Banana 2 reference limits: %+v", model)
			}
		}
	}
	for _, id := range []string{modelcatalog.MiniAppImageNanoBanana2, modelcatalog.MiniAppImageNanoBananaPro, modelcatalog.MiniAppImageGPTImage2} {
		if !sawQualityOptions[id] {
			t.Fatalf("%s public model was not exposed with quality options", id)
		}
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
	runtimeCatalog, err := productcatalog.FromConfig(cfg, staticPricingCatalog(t))
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

func staticPricingCatalog(t *testing.T) *pricingcatalog.Catalog {
	t.Helper()
	catalog, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatalf("build pricing catalog: %v", err)
	}
	return catalog
}
