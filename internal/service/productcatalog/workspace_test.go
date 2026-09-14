package productcatalog_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"vk-ai-aggregator/internal/domain"

	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/textgeneration"
	"vk-ai-aggregator/internal/service/videorouter"
)

func TestWorkspaceCatalogUsesOneTypedAndSafeList(t *testing.T) {
	prices := staticPricingCatalog(t)
	r := providermodels.StaticRegistry()
	im, _ := r.PublicImageModel(providermodels.PublicImageNanoBanana2)
	c := productcatalog.WorkspaceCatalog(productcatalog.WorkspaceConfig{
		TextModels: textgeneration.Models(nil, prices), Pricing: prices,
		ImageModels: []imagegeneration.PublicModel{{ID: im.PublicID, Name: im.DisplayName, Enabled: true, Ready: true, QualityOptions: im.Limits.AllowedQualities, DefaultQuality: im.Limits.AllowedQualities[0], SupportsReferenceImage: im.Limits.SupportsReferenceImage, MaxReferenceImages: im.Limits.MaxReferenceImages, MaxOutputCount: im.Limits.MaxOutputCount, AllowedAspectRatios: im.Limits.AllowedAspectRatios}},
	})
	if c.SchemaVersion != 1 || len(c.Items) != 2 || c.DefaultModelID != providermodels.PublicTextChatGPT {
		t.Fatalf("incomplete catalog: %+v", c)
	}
	seen := map[string]bool{}
	for _, model := range c.Items {
		if seen[model.ID] || model.Name == "" || model.Description == "" || len(model.Categories) == 0 || model.Verification != "legacy-unverified" || len(model.Operations) != 1 {
			t.Fatalf("incomplete model: %+v", model)
		}
		seen[model.ID] = true
		op := model.Operations[0]
		if !op.Enabled || op.Kind != model.Kind || op.Inputs.Images.Enabled {
			t.Fatalf("unsafe operation: %+v", op)
		}
		if model.Kind == "image" && (op.Image == nil || len(op.Image.QualityOptions) == 0 || op.Image.DefaultAspectRatio == "" || op.Image.MaxOutputCount != 4) {
			t.Fatalf("image options missing: %+v", op)
		}
		if model.Kind == "text" && (!slices.Contains(model.Categories, "text") || !slices.Contains(model.Categories, "study-work")) {
			t.Fatalf("text categories missing: %+v", model)
		}
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"provider_model_id", "registry_fingerprint", "endpoint", "api_key", "contract_digest", "checked_at"} {
		if strings.Contains(string(raw), private) {
			t.Fatalf("private field leaked: %s", private)
		}
	}
}

func TestEveryPreviewOptionResolvesWithoutCallingProviders(t *testing.T) {
	catalog, err := productcatalog.WorkspacePreviewCatalog()
	if err != nil {
		t.Fatal(err)
	}
	prices := staticPricingCatalog(t)
	registry := providermodels.StaticRegistry()
	vcfg := videorouter.Config{RouterEnabled: true, Providers: map[domain.ProviderName]videorouter.ProviderConfig{}, EnabledRoutes: map[domain.VideoRouteAlias]bool{}}
	for _, route := range registry.VideoRoutes() {
		vcfg.Providers[route.Provider] = videorouter.ProviderConfig{Enabled: true, APIKeyConfigured: true, BaseURLConfigured: true}
		vcfg.EnabledRoutes[route.Alias] = true
	}
	videoResolver, err := videorouter.NewCatalog(vcfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range catalog.Items {
		for _, op := range model.Operations {
			if op.Image != nil {
				m, _ := registry.PublicImageModel(model.ID)
				resolver := imagegeneration.NewResolver([]imagegeneration.PublicModel{{ID: model.ID, Name: model.Name, Enabled: true, Ready: true, QualityOptions: op.Image.QualityOptions, DefaultQuality: op.Image.DefaultQuality, MaxOutputCount: op.Image.MaxOutputCount, SupportsReferenceImage: m.Limits.SupportsReferenceImage, MaxReferenceImages: m.Limits.MaxReferenceImages}}, prices)
				for key, cost := range op.Image.PriceByVariant {
					quality, ratio, _ := strings.Cut(key, ":")
					result, err := resolver.Resolve(imagegeneration.Request{ModelID: model.ID, Quality: quality, AspectRatio: ratio})
					if err != nil || result.PricingSnapshot.InternalCredits != cost {
						t.Errorf("advertised image variant %s/%s does not resolve: %v", model.ID, key, err)
					}
				}
			}
			if op.Video != nil {
				for _, variant := range op.Video.Variants {
					if variant.FPS != nil || variant.Audio != nil {
						t.Errorf("legacy output facts must remain unknown: %s", model.ID)
					}
					params, _ := json.Marshal(map[string]any{"prompt": "Synthetic catalog validation", "video_route_alias": model.ID, "resolution": variant.Resolution, "duration_sec": variant.DurationSec, "aspect_ratio": variant.AspectRatio})
					result, err := videoResolver.Resolve(context.Background(), videorouter.Request{Source: "web", Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, Params: params})
					if err != nil || !result.Resolved {
						t.Errorf("advertised video variant %s/%+v does not resolve: %v", model.ID, variant, err)
					}
				}
			}
		}
	}
}

func TestWorkspaceCatalogDoesNotInventUnavailableOrDuplicateModels(t *testing.T) {
	chat := textgeneration.Models(nil, nil)[0]
	c := productcatalog.WorkspaceCatalog(productcatalog.WorkspaceConfig{TextModels: []textgeneration.PublicModel{chat, chat}, ImageModels: []imagegeneration.PublicModel{{ID: "unavailable", Enabled: true, Ready: false}}})
	if len(c.Items) != 1 || c.Items[0].ID != chat.ID {
		t.Fatalf("wrong available catalog: %+v", c)
	}
}
