package productcatalog_test

import (
	"testing"

	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
)

func TestMidjourneyCatalogAndPerCallPrices(t *testing.T) {
	t.Setenv("FEATURE_APIMART_MIDJOURNEY_V7_ENABLED", "true")
	cfg := config.Load()
	cfg.APIMartProviderEnabled = true
	cfg.APIMartAPIKey = "test-key"
	cfg.APIMartBaseURL = "https://example.com/v1"
	prices := staticPricingCatalog(t)
	runtime, err := productcatalog.FromConfig(cfg, prices)
	if err != nil {
		t.Fatal(err)
	}
	model := findImage(runtime.ImageModels(), "midjourney_v7")
	if model == nil {
		t.Fatal("Midjourney missing")
	}
	assertNoPrivateProviderFields(t, runtime.Catalog.Items())
	if model.MaxOutputCount != 1 || model.MaxReferenceImages != 4 || model.DefaultQuality != "relax" {
		t.Fatal("incorrect Imagine limits")
	}
	resolver := imagegeneration.NewResolver([]imagegeneration.PublicModel{{ID: model.ID, Enabled: true, Ready: true, QualityOptions: model.QualityOptions, DefaultQuality: model.DefaultQuality, SupportsReferenceImage: true, MaxReferenceImages: 4, MaxOutputCount: 1}}, prices)
	for _, tc := range []struct {
		speed         string
		floor, retail int64
	}{{"relax", 450400, 30}, {"fast", 550400, 35}, {"turbo", 1000000, 60}} {
		resolved, err := resolver.Resolve(imagegeneration.Request{ModelID: "midjourney_v7", Quality: tc.speed, AspectRatio: "16:9", ReferenceCount: 4})
		if err != nil {
			t.Fatal(err)
		}
		if resolved.Worker.ModelCode != "midjourney" || resolved.Worker.Resolution != tc.speed || resolved.PricingSnapshot.Floor.Amount != tc.floor || resolved.PricingSnapshot.InternalCredits != tc.retail {
			t.Fatalf("wrong tariff/route for %s", tc.speed)
		}
	}
	for _, req := range []imagegeneration.Request{{ModelID: "midjourney_v7", OutputCount: 4}, {ModelID: "midjourney_v7", Quality: "1K"}, {ModelID: "midjourney_v7", ReferenceCount: 5}} {
		if _, err := resolver.Resolve(req); err == nil {
			t.Fatal("unpriced Imagine request accepted")
		}
	}
	for _, prompt := range []string{"scene --repeat 4", "a {red,blue} bird", "scene --v 8.2"} {
		if _, err := resolver.ResolvePublic(imagegeneration.Request{ModelID: "midjourney_v7", Prompt: prompt}); err != imagegeneration.ErrUnsupportedPromptOptions {
			t.Fatal("native options accepted before job creation")
		}
	}
	cfg.APIMartAPIKey = ""
	runtime, err = productcatalog.FromConfig(cfg, prices)
	if err != nil {
		t.Fatal(err)
	}
	if findImage(runtime.ImageModels(), "midjourney_v7") != nil {
		t.Fatal("unready model exposed")
	}
	assertNoPrivateProviderFields(t, runtime.Catalog.Items())
}

func TestMidjourneyUnavailableWithoutFlagProviderOrPrice(t *testing.T) {
	for _, scenario := range []string{"disabled", "provider disabled", "missing price"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("FEATURE_APIMART_MIDJOURNEY_V7_ENABLED", "true")
			cfg := config.Load()
			cfg.APIMartProviderEnabled, cfg.APIMartAPIKey, cfg.APIMartBaseURL = true, "test-key", "https://example.com/v1"
			prices := staticPricingCatalog(t)
			switch scenario {
			case "disabled":
				cfg.FeatureAPIMartMidjourneyV7Enabled = false
			case "provider disabled":
				cfg.APIMartProviderEnabled = false
			case "missing price":
				prices, _ = pricingcatalog.NewCatalog(nil)
			}
			runtime, err := productcatalog.FromConfig(cfg, prices)
			if err != nil {
				t.Fatal(err)
			}
			if findImage(runtime.ImageModels(), "midjourney_v7") != nil {
				t.Fatal("unavailable Midjourney exposed")
			}
		})
	}
}
