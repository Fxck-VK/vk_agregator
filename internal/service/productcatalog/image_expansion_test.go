package productcatalog

import (
	"context"
	"testing"

	"vk-ai-aggregator/internal/adapter/provider/apimart"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestImageExpansionCatalogPricingAndAdapterAgree(t *testing.T) {
	if err := providermodels.ConfigureDEVSmoke("development", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = providermodels.ConfigureDEVSmoke("development", false) })
	prices, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if err := prices.AddSupplemental(providermodels.RuntimeRegistry().DEVSmokePrices()); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Env: "development", FeatureDEVModelSmokeEnabled: true, APIMartProviderEnabled: true, APIMartAPIKey: "fixture", APIMartBaseURL: "https://api.apimart.ai/v1"}
	runtime, err := FromConfig(cfg, prices)
	if err != nil {
		t.Fatal(err)
	}
	var images []imagegeneration.PublicModel
	for _, m := range runtime.ImageModels() {
		images = append(images, imagegeneration.PublicModel{ID: m.ID, Name: m.Name, Enabled: m.Enabled, Ready: m.Enabled, QualityOptions: m.QualityOptions, DefaultQuality: m.DefaultQuality, MaxOutputCount: m.MaxOutputCount, AllowedAspectRatios: m.AllowedAspectRatios})
	}
	list := WorkspaceCatalog(WorkspaceConfig{ImageModels: images, Pricing: prices, IncludePendingMedia: true})
	resolver := imagegeneration.NewResolver(images, prices)
	provider := apimart.New(apimart.Config{})
	for _, id := range []string{"seedream_5_0_flash", "z_image_turbo", "flux_2_max", "flux_2_flex", "qwen_image_3_pro"} {
		m := pendingByID(t, list, id)
		if m.Verification != "dev-smoke" || !m.Operations[0].Enabled || m.Operations[0].Image == nil {
			t.Fatalf("%s missing active image controls", id)
		}
		if m.Capabilities.Application.Image.Images.Support != providermodels.Unsupported {
			t.Fatal("unverified input enabled")
		}
		candidate, _ := providermodels.MediaCandidateByID(id)
		contract := providermodels.DraftMediaContract(candidate)
		controls := m.Operations[0].Image
		if len(contract.Operations[0].Image.Variants) != len(controls.QualityOptions)*len(controls.AllowedAspectRatios) {
			t.Fatal("draft does not cover advertised variants")
		}
		for _, quality := range controls.QualityOptions {
			for _, ratio := range controls.AllowedAspectRatios {
				req := imagegeneration.Request{Prompt: "Synthetic scene", ModelID: id, Quality: quality, AspectRatio: ratio, OutputCount: 1}
				resolved, err := resolver.Resolve(req)
				if err != nil {
					t.Fatalf("%s %s %s: %v", id, quality, ratio, err)
				}
				if resolved.PricingSnapshot.InternalCredits != controls.PriceByVariant[quality+":"+ratio] {
					t.Fatal("catalog and reservation prices differ")
				}
				w := resolved.Worker
				if err := imagegeneration.ValidatePricedRequest(resolved.PricingSnapshot, req, w.Provider, w.ModelCode, w.Resolution); err != nil {
					t.Fatal(err)
				}
				if _, err := provider.Estimate(context.Background(), domain.ProviderRequest{Provider: w.Provider, Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ModelCode: w.ModelCode, Prompt: req.Prompt, Resolution: w.Resolution, Size: w.Size, AspectRatio: w.AspectRatio, OutputCount: w.OutputCount}); err != nil {
					t.Fatalf("catalog option rejected by adapter: %v", err)
				}
				if imagegeneration.ValidatePricedRequest(resolved.PricingSnapshot, req, w.Provider, w.ModelCode, "unpaid-quality") == nil {
					t.Fatal("persisted quality tampering accepted")
				}
				req.OutputCount = 2
				if _, err := resolver.Resolve(req); err == nil {
					t.Fatal("batch accepted")
				}
				req.OutputCount = 1
				req.ReferenceCount = 1
				if _, err := resolver.Resolve(req); err == nil {
					t.Fatal("references accepted")
				}
			}
		}
	}
}
