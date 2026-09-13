package productcatalog_test

import (
	"testing"

	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/productcatalog"
)

func TestSeedream5CatalogAndQuotes(t *testing.T) {
	t.Setenv("FEATURE_APIMART_SEEDREAM_5_0_LITE_ENABLED", "true")
	t.Setenv("FEATURE_APIMART_SEEDREAM_5_0_PRO_ENABLED", "true")
	cfg := config.Load()
	cfg.APIMartProviderEnabled, cfg.APIMartAPIKey, cfg.APIMartBaseURL = true, "test-key", "https://example.com/v1"
	prices := staticPricingCatalog(t)
	runtime, err := productcatalog.FromConfig(cfg, prices)
	if err != nil {
		t.Fatal(err)
	}
	assertNoPrivateProviderFields(t, runtime.Catalog.Items())
	for _, tc := range []struct {
		id, code, quality string
		refs, count       int
		retail            int64
	}{
		{"seedream_5_0_lite", "seedream-5-0-lite", "2K", 0, 1, 20},
		{"seedream_5_0_lite", "seedream-5-0-lite", "3K", 1, 14, 280},
		{"seedream_5_0_lite", "seedream-5-0-lite", "4K", 14, 1, 20},
		{"seedream_5_0_pro", "seedream-5-0-pro", "1K", 0, 1, 20},
		{"seedream_5_0_pro", "seedream-5-0-pro", "1.5K", 1, 1, 20},
		{"seedream_5_0_pro", "seedream-5-0-pro", "2K", 1, 1, 40},
		{"seedream_5_0_pro", "seedream-5-0-pro", "1.5K", 10, 1, 30},
		{"seedream_5_0_pro", "seedream-5-0-pro", "2K", 10, 1, 50},
	} {
		model := findImage(runtime.ImageModels(), tc.id)
		if model == nil {
			t.Fatalf("missing %s", tc.id)
		}
		resolver := imagegeneration.NewResolver([]imagegeneration.PublicModel{{ID: model.ID, Enabled: true, Ready: true, QualityOptions: model.QualityOptions, DefaultQuality: model.DefaultQuality, SupportsReferenceImage: model.SupportsReferenceImage, MaxReferenceImages: model.MaxReferenceImages, MaxOutputCount: model.MaxOutputCount, AllowedAspectRatios: model.AllowedAspectRatios}}, prices)
		got, err := resolver.Resolve(imagegeneration.Request{ModelID: tc.id, Quality: tc.quality, AspectRatio: "16:9", ReferenceCount: tc.refs, OutputCount: tc.count})
		if err != nil {
			t.Fatal(err)
		}
		if got.Worker.ModelCode != tc.code || got.Worker.Resolution != tc.quality || got.PricingSnapshot.InternalCredits != tc.retail {
			t.Fatalf("wrong route/price for %s %s", tc.id, tc.quality)
		}
		for _, request := range []imagegeneration.Request{
			{ModelID: tc.id, Quality: tc.quality, ReferenceCount: 14, OutputCount: 2},
			{ModelID: tc.id, Quality: tc.quality, AspectRatio: "9:21"},
			{ModelID: tc.id, Quality: "auto"},
		} {
			if _, err := resolver.Resolve(request); err == nil {
				t.Fatalf("unpriced shape accepted: %+v", request)
			}
		}
	}
}
