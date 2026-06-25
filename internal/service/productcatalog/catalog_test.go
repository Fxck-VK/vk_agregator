package productcatalog_test

import (
	"encoding/json"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/modelcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/videorouter"
)

func TestCatalogBuildsOnlyPublicEnabledItems(t *testing.T) {
	videoCatalog := newVideoCatalog(t)
	catalog := productcatalog.New(productcatalog.Config{
		ImageProviderReady: map[domain.ProviderName]bool{
			domain.ProviderAPIMart: true,
			domain.ProviderPoYo:    true,
		},
		EnabledImageModels: map[string]bool{
			modelcatalog.MiniAppImageNanoBanana2:   true,
			modelcatalog.MiniAppImageNanoBananaPro: true,
			modelcatalog.MiniAppImageGPTImage2:     true,
		},
		VideoRoutes:    videoCatalog.PublicRoutes(),
		PricingCatalog: staticPricingCatalog(t),
	})

	items := catalog.Items()
	if len(items) != 4 {
		t.Fatalf("expected 3 image models and 1 video route, got %+v", items)
	}
	for _, item := range items {
		if item.ID == "" || item.Type == "" || item.Name == "" || item.Description == "" || !item.Enabled || item.EstimateCredits <= 0 {
			t.Fatalf("missing public product fields: %+v", item)
		}
	}

	nano := findItem(items, modelcatalog.MiniAppImageNanoBanana2)
	if nano == nil {
		t.Fatalf("Nano Banana 2 missing from public catalog: %+v", items)
	}
	assertImageQualityOptions(t, "Nano Banana 2", nano.DefaultQuality, nano.QualityOptions)
	if !nano.SupportsReferenceImage || nano.MaxReferenceImages != 4 {
		t.Fatalf("Nano Banana 2 reference limits missing: %+v", nano)
	}
	pro := findItem(items, modelcatalog.MiniAppImageNanoBananaPro)
	if pro == nil {
		t.Fatalf("Nano Banana Pro missing from public catalog: %+v", items)
	}
	assertImageQualityOptions(t, "Nano Banana Pro", pro.DefaultQuality, pro.QualityOptions)
	gptImage2 := findItem(items, modelcatalog.MiniAppImageGPTImage2)
	if gptImage2 == nil {
		t.Fatalf("GPT Image 2 missing from public catalog: %+v", items)
	}
	assertImageQualityOptions(t, "GPT Image 2", gptImage2.DefaultQuality, gptImage2.QualityOptions)

	video := findItem(items, string(domain.VideoRouteKlingO3Standard))
	if video == nil {
		t.Fatalf("Kling video route missing from public catalog: %+v", items)
	}
	if video.Type != productcatalog.TypeVideo || video.Alias != string(domain.VideoRouteKlingO3Standard) || len(video.AllowedDurationsSec) == 0 || video.DefaultDurationSec == 0 {
		t.Fatalf("video route constraints missing: %+v", video)
	}

	assertNoPrivateProviderFields(t, items)
}

func TestCatalogFailsClosedForDisabledOrUnconfiguredModels(t *testing.T) {
	catalog := productcatalog.New(productcatalog.Config{
		ImageProviderReady: map[domain.ProviderName]bool{
			domain.ProviderAPIMart: false,
			domain.ProviderPoYo:    true,
		},
		EnabledImageModels: map[string]bool{
			modelcatalog.MiniAppImageNanoBanana2:   false,
			modelcatalog.MiniAppImageNanoBananaPro: true,
			modelcatalog.MiniAppImageGPTImage2:     true,
		},
		PricingCatalog: staticPricingCatalog(t),
	})

	if images := catalog.ImageModels(); len(images) != 0 {
		t.Fatalf("disabled/unconfigured image models leaked: %+v", images)
	}
	if items := catalog.Items(); len(items) != 0 {
		t.Fatalf("disabled/unconfigured product items leaked: %+v", items)
	}
}

func TestCatalogReturnsDefensiveCopies(t *testing.T) {
	catalog := productcatalog.New(productcatalog.Config{
		ImageProviderReady: map[domain.ProviderName]bool{domain.ProviderPoYo: true},
		EnabledImageModels: map[string]bool{
			modelcatalog.MiniAppImageNanoBanana2: true,
		},
		PricingCatalog: staticPricingCatalog(t),
	})

	images := catalog.ImageModels()
	if len(images) != 1 || len(images[0].QualityOptions) == 0 {
		t.Fatalf("expected image quality options, got %+v", images)
	}
	images[0].QualityOptions[0] = "mutated"

	items := catalog.Items()
	if len(items) != 1 || len(items[0].QualityOptions) == 0 {
		t.Fatalf("expected item quality options, got %+v", items)
	}
	items[0].QualityOptions[0] = "mutated"

	again := catalog.ImageModels()
	if again[0].QualityOptions[0] != modelcatalog.ImageQuality1K {
		t.Fatalf("catalog returned mutable image quality slice: %+v", again[0])
	}
}

func newVideoCatalog(t *testing.T) *videorouter.Catalog {
	t.Helper()
	catalog, err := videorouter.NewCatalog(videorouter.Config{
		RouterEnabled: true,
		Providers: map[domain.ProviderName]videorouter.ProviderConfig{
			domain.ProviderPoYo: {
				Enabled:           true,
				RequireAPIKey:     true,
				APIKeyConfigured:  true,
				RequireBaseURL:    true,
				BaseURLConfigured: true,
			},
		},
		EnabledRoutes: map[domain.VideoRouteAlias]bool{
			domain.VideoRouteKlingO3Standard: true,
		},
	})
	if err != nil {
		t.Fatalf("new video catalog: %v", err)
	}
	return catalog
}

func findItem(items []productcatalog.Item, id string) *productcatalog.Item {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

func assertImageQualityOptions(t *testing.T, name, defaultQuality string, options []string) {
	t.Helper()
	want := []string{modelcatalog.ImageQuality1K, modelcatalog.ImageQuality2K, modelcatalog.ImageQuality4K}
	if defaultQuality != modelcatalog.ImageQuality1K || len(options) != len(want) {
		t.Fatalf("%s quality options missing: default=%q options=%+v", name, defaultQuality, options)
	}
	for i, option := range want {
		if options[i] != option {
			t.Fatalf("%s quality option %d = %q, want %q; options=%+v", name, i, options[i], option, options)
		}
	}
}

func assertNoPrivateProviderFields(t *testing.T, items []productcatalog.Item) {
	t.Helper()
	raw, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("marshal items: %v", err)
	}
	serialized := strings.ToLower(string(raw))
	for _, private := range []string{
		"provider",
		"provider_cost",
		"model_code",
		"provider_model_id",
		"provider_native_model_id",
		"api_key",
		"authorization",
		"auth_header",
		"bearer",
		"resolved_snapshot",
		"raw_provider_payload",
		"private_url",
		"prompt",
		"price",
		"floor",
		"floor_amount",
		"floor_unit",
		"multiplier",
		"cost_estimate",
		"provider_cost_credits",
		"price_multiplier",
		"max_internal_cost_credits",
		"nano-banana-2",
		"gemini-3-pro-image-preview",
		"gpt-image-2",
		"kling-o3/standard",
	} {
		if strings.Contains(serialized, private) {
			t.Fatalf("public catalog leaked private field %q: %s", private, serialized)
		}
	}
}
