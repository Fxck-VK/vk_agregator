package productcatalog_test

import (
	"errors"
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestGrokImageCatalogAndPricing(t *testing.T) {
	for _, tc := range []struct {
		id, model, flag, resolution string
		refs                        int
	}{
		{"grok_image_1_5", "grok-imagine-1.5-apimart", "FEATURE_APIMART_GROK_IMAGE_1_5_ENABLED", "", 1},
		{"grok_image_2_0", "grok-imagine-2.0-ext", "FEATURE_APIMART_GROK_IMAGE_2_0_ENABLED", "quality", 0},
	} {
		t.Run(tc.id, func(t *testing.T) {
			t.Setenv(tc.flag, "true")
			cfg := config.Load()
			cfg.APIMartProviderEnabled, cfg.APIMartAPIKey, cfg.APIMartBaseURL = true, "test-key", "https://example.com/v1"
			prices := staticPricingCatalog(t)
			runtime, err := productcatalog.FromConfig(cfg, prices)
			if err != nil {
				t.Fatal(err)
			}
			model := findImage(runtime.ImageModels(), tc.id)
			if model == nil {
				t.Fatal("Grok missing from public catalog")
			}
			if model.MaxOutputCount != 1 || model.MaxReferenceImages != tc.refs || model.SupportsReferenceImage != (tc.refs > 0) || model.DefaultQuality != "standard" {
				t.Fatal("incorrect public limits")
			}
			registered, ok := providermodels.StaticRegistry().PublicImageModel(tc.id)
			if !ok || registered.ProviderModelID != tc.model {
				t.Fatal("incorrect provider route")
			}
			resolver := imagegeneration.NewResolver([]imagegeneration.PublicModel{{ID: tc.id, Enabled: true, Ready: true, QualityOptions: []string{"standard"}, DefaultQuality: "standard", SupportsReferenceImage: tc.refs > 0, MaxReferenceImages: tc.refs, MaxOutputCount: 1}}, prices)
			resolved, err := resolver.Resolve(imagegeneration.Request{ModelID: tc.id, ReferenceCount: tc.refs, AspectRatio: "16:9"})
			if err != nil {
				t.Fatal(err)
			}
			if resolved.Worker.ModelCode != tc.model || resolved.Worker.Resolution != tc.resolution || resolved.Worker.Provider != domain.ProviderAPIMart || resolved.Public.ImageQuality != "standard" {
				t.Fatal("incorrect trusted worker parameters")
			}
			price := resolved.PricingSnapshot
			if price.InternalCredits != 10 || price.InternalCreditCap != 10 || price.Floor.Amount != 150000 || price.Floor.Unit != pricingcatalog.FloorUnitAPIMartCredits {
				t.Fatal("incorrect Grok tariff snapshot")
			}
			for _, bad := range []struct {
				request imagegeneration.Request
				want    error
			}{
				{imagegeneration.Request{ModelID: tc.id, Quality: "1K"}, imagegeneration.ErrUnsupportedQuality},
				{imagegeneration.Request{ModelID: tc.id, OutputCount: 2}, imagegeneration.ErrOutputCountLimit},
				{imagegeneration.Request{ModelID: tc.id, ReferenceCount: tc.refs + 1}, nil},
				{imagegeneration.Request{ModelID: tc.id, AspectRatio: "21:9"}, imagegeneration.ErrUnsupportedAspectRatio},
				{imagegeneration.Request{ModelID: tc.model}, imagegeneration.ErrPublicModelUnavailable},
			} {
				_, err := resolver.Resolve(bad.request)
				if err == nil || (bad.want != nil && !errors.Is(err, bad.want)) {
					t.Fatalf("request was not correctly rejected: %v", err)
				}
			}
			assertNoPrivateProviderFields(t, runtime.Catalog.Items())
		})
	}
}

func TestGrokImageReadiness(t *testing.T) {
	for _, id := range []string{"grok_image_1_5", "grok_image_2_0"} {
		for _, scenario := range []string{"disabled", "provider disabled", "missing key", "missing tariff"} {
			t.Run(id+"/"+scenario, func(t *testing.T) {
				t.Setenv("FEATURE_APIMART_GROK_IMAGE_1_5_ENABLED", "true")
				t.Setenv("FEATURE_APIMART_GROK_IMAGE_2_0_ENABLED", "true")
				if scenario == "disabled" {
					flag := "FEATURE_APIMART_GROK_IMAGE_1_5_ENABLED"
					if id == "grok_image_2_0" {
						flag = "FEATURE_APIMART_GROK_IMAGE_2_0_ENABLED"
					}
					t.Setenv(flag, "false")
				}
				cfg := config.Load()
				cfg.APIMartProviderEnabled, cfg.APIMartAPIKey, cfg.APIMartBaseURL = true, "test-key", "https://example.com/v1"
				if scenario == "provider disabled" {
					cfg.APIMartProviderEnabled = false
				}
				if scenario == "missing key" {
					cfg.APIMartAPIKey = ""
				}
				prices := staticPricingCatalog(t)
				if scenario == "missing tariff" {
					prices, _ = pricingcatalog.NewCatalog(nil)
				}
				runtime, err := productcatalog.FromConfig(cfg, prices)
				if err != nil {
					t.Fatal(err)
				}
				if findImage(runtime.ImageModels(), id) != nil {
					t.Fatal("unready Grok exposed")
				}
			})
		}
	}
}
