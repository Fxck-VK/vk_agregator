package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/provider/apimart"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/musicgeneration"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/videorouter"
)

func TestDEVSmokePublicControlsReachAdapterWithoutNetwork(t *testing.T) {
	if err := providermodels.ConfigureDEVSmoke("development", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = providermodels.ConfigureDEVSmoke("development", false) })
	registry := providermodels.RuntimeRegistry()
	prices, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if err := prices.AddSupplemental(registry.DEVSmokePrices()); err != nil {
		t.Fatal(err)
	}
	runtime, err := productcatalog.FromConfig(config.Config{Env: "development", FeatureDEVModelSmokeEnabled: true, FeatureVideoRouterEnabled: true, APIMartProviderEnabled: true, APIMartAPIKey: "test-key", APIMartBaseURL: "https://api.example.com"}, prices)
	if err != nil {
		t.Fatal(err)
	}
	var images []imagegeneration.PublicModel
	for _, m := range runtime.ImageModels() {
		images = append(images, imagegeneration.PublicModel{ID: m.ID, Name: m.Name, Enabled: true, Ready: true, QualityOptions: m.QualityOptions, DefaultQuality: m.DefaultQuality, MaxOutputCount: m.MaxOutputCount, AllowedAspectRatios: m.AllowedAspectRatios})
	}
	catalog := productcatalog.WorkspaceCatalog(productcatalog.WorkspaceConfig{ImageModels: images, VideoRoutes: runtime.VideoRoutes(), Pricing: prices, IncludePendingMedia: true})
	imageResolver := imagegeneration.NewResolver(images, prices)
	adapter := apimart.New(apimart.Config{})
	providers := NewRegistry(adapter)
	providers.ConfigureProviderMediaContracts(registry.ProviderMediaContracts(providermodels.MediaContractRuntime{}), false, false)
	checkRequest := func(id string, request domain.ProviderRequest) {
		t.Helper()
		provider, err := providers.ForRequest(context.Background(), request)
		if err != nil {
			t.Fatalf("%s route: %v", id, err)
		}
		if _, err := provider.Estimate(context.Background(), request); err != nil {
			t.Fatalf("%s estimate: %v", id, err)
		}
	}
	count := 0
	seen := map[string]bool{}
	for _, m := range catalog.Items {
		if seen[m.ID] {
			t.Fatalf("duplicate model %s", m.ID)
		}
		seen[m.ID] = true
		if !registry.IsDEVSmokeModel(m.ID) {
			continue
		}
		count++
		if m.Verification != "dev-smoke" {
			t.Fatalf("incorrect verification status for %s", m.ID)
		}
		if m.Kind == "audio" {
			enabled := 0
			for _, op := range m.Operations {
				if op.Enabled {
					enabled++
					if op.ID != "generate" {
						t.Fatal("advanced music operation enabled")
					}
				}
			}
			if enabled != 1 {
				t.Fatal("missing music generation")
			}
			params, quote, err := musicgeneration.Resolve(musicgeneration.Request{ModelID: m.ID, Music: domain.MusicRequest{Action: domain.MusicActionGenerate, Prompt: "Instrumental landscape"}}, registry)
			if err != nil {
				t.Fatalf("%s resolve: %v", m.ID, err)
			}
			raw, _ := json.Marshal(params)
			price, _ := json.Marshal(quote)
			p := processor{}
			request, err := p.buildMusicRequest(context.Background(), &domain.Job{ID: uuid.New(), AccountID: uuid.New(), OperationType: domain.OperationAudioMusic, Modality: domain.ModalityAudio, Params: raw, PricingSnapshot: price, CostEstimate: quote.InternalCredits}, 1)
			if err != nil {
				t.Fatalf("%s hydration: %v", m.ID, err)
			}
			checkRequest(m.ID, request)
			continue
		}
		for _, op := range m.Operations {
			if !op.Enabled {
				t.Fatal("smoke operation disabled")
			}
			if op.Video != nil {
				for _, v := range op.Video.Variants {
					params, _ := json.Marshal(map[string]any{"prompt": "A tree in the wind", "video_route_alias": m.ID, "resolution": v.Resolution, "duration_sec": v.DurationSec, "aspect_ratio": v.AspectRatio})
					resolved, err := runtime.VideoRouteCatalog.Resolve(context.Background(), videorouter.Request{Source: "web", Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, Params: params})
					if err != nil {
						t.Fatalf("%s resolve: %v", m.ID, err)
					}
					quote, err := pricingcatalog.MediaVideoCandidateQuote(m.ID, "", v.Resolution, v.DurationSec)
					if err != nil || resolved.InternalCostCredits != quote.InternalCredits {
						t.Fatalf("%s route price mismatch", m.ID)
					}
					job := &domain.Job{ID: uuid.New(), AccountID: uuid.New(), OperationType: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, Params: resolved.Params}
					p := processor{}
					request, err := p.buildRequest(context.Background(), job, 1)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := adapter.Estimate(context.Background(), request); err != nil {
						t.Fatalf("%s/%s/%ds hydration: %v", m.ID, v.Resolution, v.DurationSec, err)
					}
					checkRequest(m.ID, request)
				}
			} else if op.Image != nil {
				result, err := imageResolver.Resolve(imagegeneration.Request{ModelID: m.ID, Quality: op.Image.DefaultQuality, AspectRatio: "16:9", Prompt: "A tree"})
				if err != nil {
					t.Fatal(err)
				}
				w := result.Worker
				params, _ := json.Marshal(map[string]any{"prompt": "A tree", "provider": w.Provider, "model_code": w.ModelCode, "model_id": m.ID, "size": w.Size, "aspect_ratio": w.AspectRatio, "output_count": w.OutputCount, "resolution": w.Resolution})
				p := processor{}
				r, err := p.buildRequest(context.Background(), &domain.Job{ID: uuid.New(), AccountID: uuid.New(), OperationType: domain.OperationImageGenerate, Modality: domain.ModalityImage, Params: params}, 1)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := adapter.Estimate(context.Background(), r); err != nil {
					t.Fatalf("%s hydration: %v", m.ID, err)
				}
				checkRequest(m.ID, r)
			}
		}
	}
	if count != 16 {
		t.Fatalf("enabled candidates %d, want 16", count)
	}
}
