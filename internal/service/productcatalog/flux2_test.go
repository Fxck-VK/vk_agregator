package productcatalog_test

import (
	"testing"

	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
)

func TestFlux2ProCatalogAndPrices(t *testing.T) {
	t.Setenv("FEATURE_APIMART_FLUX_2_PRO_ENABLED", "true")
	cfg := config.Load()
	cfg.APIMartProviderEnabled, cfg.APIMartAPIKey, cfg.APIMartBaseURL = true, "test-key", "https://example.com/v1"
	prices := staticPricingCatalog(t)
	runtime, err := productcatalog.FromConfig(cfg, prices)
	if err != nil {
		t.Fatal(err)
	}
	model := findImage(runtime.ImageModels(), "flux_2_pro")
	if model == nil {
		t.Fatal("FLUX.2 Pro missing")
	}
	assertNoPrivateProviderFields(t, runtime.Catalog.Items())
	if model.MaxOutputCount != 1 || model.SupportsReferenceImage || model.MaxReferenceImages != 0 || model.DefaultQuality != "1MP" {
		t.Fatal("incorrect text-to-image limits")
	}
	resolver := imagegeneration.NewResolver([]imagegeneration.PublicModel{{ID: model.ID, Enabled: true, Ready: true, QualityOptions: model.QualityOptions, DefaultQuality: model.DefaultQuality, MaxOutputCount: 1, AllowedAspectRatios: model.AllowedAspectRatios}}, prices)
	for _, tc := range []struct {
		resolution    string
		floor, retail int64
	}{{"1MP", 240000, 15}, {"2MP", 360000, 25}, {"3MP", 480000, 30}, {"4MP", 600000, 40}} {
		for _, ratio := range []string{"1:1", "4:3", "3:4", "16:9", "9:16", "3:2", "2:3", "21:9", "9:21"} {
			resolved, err := resolver.Resolve(imagegeneration.Request{ModelID: model.ID, Quality: tc.resolution, AspectRatio: ratio})
			if err != nil {
				t.Fatal(err)
			}
			if resolved.Worker.ModelCode != "flux-2-pro" || resolved.Worker.Resolution != tc.resolution || resolved.Worker.AspectRatio != ratio || resolved.PricingSnapshot.Floor.Amount != tc.floor || resolved.PricingSnapshot.InternalCredits != tc.retail {
				t.Fatalf("wrong price or route for %s", tc.resolution)
			}
		}
	}
	for _, req := range []imagegeneration.Request{{Quality: "1K"}, {Quality: "2K"}, {Quality: "4K"}, {ReferenceCount: 1}, {OutputCount: 2}, {AspectRatio: "auto"}, {AspectRatio: "4:5"}, {AspectRatio: "1024x1024"}} {
		req.ModelID = model.ID
		if _, err := resolver.Resolve(req); err == nil {
			t.Fatal("unpriced request accepted")
		}
	}
}

func TestFlux2ProUnavailableWithoutFlagProviderOrPrice(t *testing.T) {
	for _, scenario := range []string{"disabled", "provider disabled", "missing key", "missing price"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("FEATURE_APIMART_FLUX_2_PRO_ENABLED", "true")
			cfg := config.Load()
			cfg.APIMartProviderEnabled, cfg.APIMartAPIKey, cfg.APIMartBaseURL = true, "test-key", "https://example.com/v1"
			prices := staticPricingCatalog(t)
			switch scenario {
			case "disabled":
				cfg.FeatureAPIMartFlux2ProEnabled = false
			case "provider disabled":
				cfg.APIMartProviderEnabled = false
			case "missing key":
				cfg.APIMartAPIKey = ""
			case "missing price":
				prices, _ = pricingcatalog.NewCatalog(nil)
			}
			runtime, err := productcatalog.FromConfig(cfg, prices)
			if err != nil {
				t.Fatal(err)
			}
			if findImage(runtime.ImageModels(), "flux_2_pro") != nil {
				t.Fatal("unavailable FLUX.2 exposed")
			}
		})
	}
}
