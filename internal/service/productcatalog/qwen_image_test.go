package productcatalog_test

import (
	"errors"
	"reflect"
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestQwenImageCatalogAndTrustedQuote(t *testing.T) {
	t.Setenv("FEATURE_APIMART_QWEN_IMAGE_3_ENABLED", "true")
	cfg := config.Load()
	cfg.APIMartProviderEnabled, cfg.APIMartAPIKey, cfg.APIMartBaseURL = true, "test-key", "https://example.com/v1"
	prices := staticPricingCatalog(t)
	runtime, err := productcatalog.FromConfig(cfg, prices)
	if err != nil {
		t.Fatal(err)
	}
	model := findImage(runtime.ImageModels(), "qwen_image_3")
	if model == nil {
		t.Fatal("Qwen missing from configured public catalog")
	}
	if model.MaxOutputCount != 1 {
		t.Fatal("Qwen public output count must stay at one")
	}
	registered, ok := providermodels.StaticRegistry().PublicImageModel("qwen_image_3")
	if !ok || registered.ProviderModelID != "qwen-image-3.0" || registered.Provider != domain.ProviderAPIMart || registered.Limits.MaxReferenceImages != 3 || !reflect.DeepEqual(registered.Limits.AllowedQualities, []string{"1K", "2K"}) {
		t.Fatal("wrong Qwen registry contract")
	}
	resolver := imagegeneration.NewResolver([]imagegeneration.PublicModel{{ID: "qwen_image_3", Enabled: true, Ready: true, QualityOptions: registered.Limits.AllowedQualities, DefaultQuality: "1K", SupportsReferenceImage: true, MaxReferenceImages: 3}}, prices)
	for _, quality := range []string{"1K", "2K"} {
		result, err := resolver.Resolve(imagegeneration.Request{ModelID: "qwen_image_3", Quality: quality, ReferenceCount: 3})
		if err != nil {
			t.Fatal(err)
		}
		if result.Worker.ModelCode != "qwen-image-3.0" || result.Worker.Provider != domain.ProviderAPIMart || result.Worker.Resolution != quality || result.Worker.Size != "1:1" {
			t.Fatal("wrong trusted worker request")
		}
		price := result.PricingSnapshot
		if price.InternalCredits != 15 || price.Floor.Amount != 205712 || price.Floor.Unit != pricingcatalog.FloorUnitAPIMartCredits || price.InternalCreditCap != 15 {
			t.Fatalf("wrong exact Qwen tariff: %+v", price)
		}
	}
	for _, tc := range []struct {
		req  imagegeneration.Request
		want error
	}{
		{imagegeneration.Request{ModelID: "qwen_image_3", Quality: "4K"}, imagegeneration.ErrUnsupportedQuality},
		{imagegeneration.Request{ModelID: "qwen_image_3", ReferenceCount: 4}, imagegeneration.ErrReferenceLimit},
		{imagegeneration.Request{ModelID: "qwen-image-3.0"}, imagegeneration.ErrPublicModelUnavailable},
		{imagegeneration.Request{ModelID: "qwen-image-3.0-pro"}, imagegeneration.ErrPublicModelUnavailable},
	} {
		if _, err := resolver.Resolve(tc.req); !errors.Is(err, tc.want) {
			t.Fatalf("resolve: %v, want %v", err, tc.want)
		}
	}
	assertNoPrivateProviderFields(t, runtime.Catalog.Items())
}

func TestQwenImageReadinessFailsClosed(t *testing.T) {
	for _, name := range []string{"flag absent", "flag false", "provider off", "missing key", "missing base", "missing tariff"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("FEATURE_APIMART_QWEN_IMAGE_3_ENABLED", "true")
			if name == "flag absent" {
				t.Setenv("FEATURE_APIMART_QWEN_IMAGE_3_ENABLED", "")
			}
			if name == "flag false" {
				t.Setenv("FEATURE_APIMART_QWEN_IMAGE_3_ENABLED", "false")
			}
			cfg := config.Load()
			cfg.APIMartProviderEnabled, cfg.APIMartAPIKey, cfg.APIMartBaseURL = true, "test-key", "https://example.com/v1"
			switch name {
			case "provider off":
				cfg.APIMartProviderEnabled = false
			case "missing key":
				cfg.APIMartAPIKey = ""
			case "missing base":
				cfg.APIMartBaseURL = ""
			}
			prices := staticPricingCatalog(t)
			if name == "missing tariff" {
				var err error
				prices, err = pricingcatalog.NewCatalog(nil)
				if err != nil {
					t.Fatal(err)
				}
			}
			runtime, err := productcatalog.FromConfig(cfg, prices)
			if err != nil {
				t.Fatal(err)
			}
			if findImage(runtime.ImageModels(), "qwen_image_3") != nil {
				t.Fatal("unready Qwen exposed")
			}
		})
	}
}
