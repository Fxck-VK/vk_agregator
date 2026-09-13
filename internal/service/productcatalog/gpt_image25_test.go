package productcatalog_test

import (
	"strings"
	"testing"

	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/productcatalog"
)

func TestGPTImage25BoundedQuotes(t *testing.T) {
	t.Setenv("FEATURE_APIMART_GPT_IMAGE_2_5_FLARE_ENABLED", "true")
	t.Setenv("FEATURE_APIMART_GPT_IMAGE_2_5_SUNBURST_ENABLED", "true")
	cfg := config.Load()
	cfg.APIMartProviderEnabled, cfg.APIMartAPIKey, cfg.APIMartBaseURL = true, "test-key", "https://example.com/v1"
	prices := staticPricingCatalog(t)
	runtime, err := productcatalog.FromConfig(cfg, prices)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"gpt_image_2_5_flare", "gpt_image_2_5_sunburst"} {
		model := findImage(runtime.ImageModels(), id)
		if model == nil {
			t.Fatalf("missing %s", id)
		}
		resolver := imagegeneration.NewResolver([]imagegeneration.PublicModel{{ID: id, Enabled: true, Ready: true, QualityOptions: model.QualityOptions, DefaultQuality: model.DefaultQuality, MaxOutputCount: model.MaxOutputCount, AllowedAspectRatios: model.AllowedAspectRatios}}, prices)
		for _, tc := range []struct {
			quality string
			credits int64
		}{{"1K-low", 20}, {"1K-medium", 25}, {"1K-high", 65}, {"1K-xhigh", 105}, {"1K-max", 215}} {
			got, err := resolver.Resolve(imagegeneration.Request{Prompt: "A landscape", ModelID: id, Quality: tc.quality, AspectRatio: "1:1", OutputCount: 2})
			if err != nil {
				t.Fatal(err)
			}
			if got.Worker.Resolution != "1K" || got.PricingSnapshot.InternalCredits != tc.credits {
				t.Fatalf("wrong quote for %s", tc.quality)
			}
		}
		for _, req := range []imagegeneration.Request{{Quality: "8K-medium"}, {Quality: "4K-auto"}, {AspectRatio: "auto"}, {ReferenceCount: 1}, {Prompt: strings.Repeat("a", 4097)}} {
			req.ModelID = id
			if _, err := resolver.Resolve(req); err == nil {
				t.Fatal("unpriced GPT request accepted")
			}
		}
		for _, tc := range []struct {
			quality, ratio string
			floor, credits int64
		}{
			{"2K-medium", "1:1", 39840, 25},
			{"4K-max", "1:1", 587688, 355},
			{"1K-medium", "16:9", 25152, 20},
			{"4K-max", "16:9", 338640, 205},
		} {
			got, err := resolver.Resolve(imagegeneration.Request{ModelID: id, Quality: tc.quality, AspectRatio: tc.ratio})
			if err != nil || got.PricingSnapshot.Floor.Amount != tc.floor || got.PricingSnapshot.InternalCredits != tc.credits {
				t.Fatalf("wrong ratio quote %s/%s: %v", tc.quality, tc.ratio, err)
			}
		}
	}
}
