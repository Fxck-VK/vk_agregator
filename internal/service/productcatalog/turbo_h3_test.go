package productcatalog_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/videorouter"
)

func TestTurboH3CatalogReadinessAndPricing(t *testing.T) {
	for _, tc := range []struct {
		alias             domain.VideoRouteAlias
		enable            func(*config.Config)
		name              string
		defaultResolution string
		minDuration       int
		resolutions       []string
		aspects           []string
		wantFiveSecond    map[string]int64
		wantProviderCost  map[string]int64
	}{
		{
			alias:             domain.VideoRouteKling30Turbo,
			enable:            func(cfg *config.Config) { cfg.FeatureAPIMartKling30TurboEnabled = true },
			name:              "Kling 3.0 Turbo",
			defaultResolution: "720p",
			minDuration:       3,
			resolutions:       []string{"720p", "1080p"},
			aspects:           []string{"16:9", "9:16", "1:1"},
			wantFiveSecond:    map[string]int64{"720p": 345, "1080p": 430},
			wantProviderCost:  map[string]int64{"720p": 6, "1080p": 8},
		},
		{
			alias:             domain.VideoRouteMiniMaxH3,
			enable:            func(cfg *config.Config) { cfg.FeatureAPIMartMiniMaxH3Enabled = true },
			name:              "MiniMax H3",
			defaultResolution: "2k",
			minDuration:       4,
			resolutions:       []string{"2k", "768p"},
			aspects:           []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"},
			wantFiveSecond:    map[string]int64{"768p": 175, "2k": 275},
			wantProviderCost:  map[string]int64{"768p": 3, "2k": 5},
		},
	} {
		t.Run(string(tc.alias), func(t *testing.T) {
			for _, readiness := range []struct {
				name    string
				mutate  func(*config.Config)
				pricing bool
			}{
				{name: "ready", mutate: tc.enable, pricing: true},
				{name: "feature disabled", mutate: func(*config.Config) {}, pricing: true},
				{name: "router disabled", mutate: func(cfg *config.Config) {
					tc.enable(cfg)
					cfg.FeatureVideoRouterEnabled = false
				}, pricing: true},
				{name: "provider disabled", mutate: func(cfg *config.Config) {
					tc.enable(cfg)
					cfg.APIMartProviderEnabled = false
				}, pricing: true},
				{name: "api key missing", mutate: func(cfg *config.Config) {
					tc.enable(cfg)
					cfg.APIMartAPIKey = ""
				}, pricing: true},
				{name: "base url missing", mutate: func(cfg *config.Config) {
					tc.enable(cfg)
					cfg.APIMartBaseURL = ""
				}, pricing: true},
				{name: "pricing missing", mutate: tc.enable, pricing: false},
			} {
				t.Run(readiness.name, func(t *testing.T) {
					cfg := config.Config{
						FeatureVideoRouterEnabled: true,
						APIMartProviderEnabled:    true,
						APIMartAPIKey:             "test",
						APIMartBaseURL:            "https://example.com",
					}
					readiness.mutate(&cfg)
					prices := emptyPricingCatalog(t)
					if readiness.pricing {
						prices = staticPricingCatalog(t)
					}
					catalog, err := productcatalog.FromConfig(cfg, prices)
					if err != nil {
						t.Fatal(err)
					}
					route := findRoute(catalog.VideoRoutes(), tc.alias)
					item := findItem(catalog.Catalog.Items(), string(tc.alias))
					if readiness.name != "ready" {
						if route != nil || item != nil {
							t.Fatalf("unready route exposed: route=%+v item=%+v", route, item)
						}
						return
					}
					if route == nil || item == nil {
						t.Fatalf("ready route missing: route=%+v item=%+v", route, item)
					}
					assertTurboH3PublicRoute(t, route, tc.name, tc.defaultResolution, tc.minDuration, tc.resolutions, tc.aspects, tc.wantFiveSecond[tc.defaultResolution])
					if item.EstimateCredits != route.EstimateCredits || item.SupportsAudio || item.RequiresReferenceVideo || !item.SupportsReferenceImage || item.MaxReferenceImages != 1 {
						t.Fatalf("public item mismatch: %+v", item)
					}
					assertNoPrivateProviderFields(t, catalog.Catalog.Items())

					for _, resolution := range tc.resolutions {
						params, _ := json.Marshal(map[string]any{
							"video_route_alias": tc.alias,
							"prompt":            "Synthetic scene",
							"duration_sec":      5,
							"resolution":        resolution,
						})
						resolved, err := catalog.VideoRouteCatalog.Resolve(context.Background(), videorouter.Request{
							Operation: domain.OperationVideoGenerate,
							Modality:  domain.ModalityVideo,
							Params:    params,
						})
						if err != nil {
							t.Fatal(err)
						}
						if resolved.InternalCostCredits != tc.wantFiveSecond[resolution] ||
							resolved.Snapshot.Provider != domain.ProviderAPIMart ||
							resolved.Snapshot.Resolution != resolution ||
							resolved.Snapshot.DurationSec != 5 ||
							resolved.Snapshot.ProviderCostCredits != tc.wantProviderCost[resolution] {
							t.Fatalf("incorrect resolved price or snapshot: %+v", resolved)
						}
					}
				})
			}
		})
	}
}

func TestTurboH3CatalogRejectsClosedInputs(t *testing.T) {
	for _, tc := range []struct {
		alias        domain.VideoRouteAlias
		enable       func(*config.Config)
		minDuration  int
		maxPrompt    int
		resolution   string
		validAspects []string
	}{
		{domain.VideoRouteKling30Turbo, func(cfg *config.Config) { cfg.FeatureAPIMartKling30TurboEnabled = true }, 3, 3072, "720p", []string{"16:9", "9:16", "1:1"}},
		{domain.VideoRouteMiniMaxH3, func(cfg *config.Config) { cfg.FeatureAPIMartMiniMaxH3Enabled = true }, 4, 7000, "2k", []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}},
	} {
		t.Run(string(tc.alias), func(t *testing.T) {
			cfg := config.Config{FeatureVideoRouterEnabled: true, APIMartProviderEnabled: true, APIMartAPIKey: "test", APIMartBaseURL: "https://example.com"}
			tc.enable(&cfg)
			catalog, err := productcatalog.FromConfig(cfg, staticPricingCatalog(t))
			if err != nil {
				t.Fatal(err)
			}
			params, _ := json.Marshal(map[string]any{
				"video_route_alias": tc.alias,
				"prompt":            "Synthetic scene",
				"duration_sec":      tc.minDuration,
				"resolution":        tc.resolution,
				"aspect_ratio":      tc.validAspects[len(tc.validAspects)-1],
			})
			resolved, err := catalog.VideoRouteCatalog.Resolve(context.Background(), videorouter.Request{
				Operation:        domain.OperationVideoGenerate,
				Modality:         domain.ModalityVideo,
				Params:           params,
				InputArtifactIDs: []uuid.UUID{uuid.New()},
			})
			if err != nil || resolved.Snapshot.AspectRatio != tc.validAspects[len(tc.validAspects)-1] {
				t.Fatalf("valid first-frame route rejected: resolved=%+v err=%v", resolved, err)
			}
			unsupported := []map[string]any{
				{"duration_sec": tc.minDuration - 1},
				{"duration_sec": 16},
				{"resolution": "4k"},
				{"aspect_ratio": "2:3"},
				{"video_audio": true},
				{"provider": "apimart"},
				{"model_code": "MiniMax-H3"},
				{"model_id": "MiniMax-H3"},
				{"provider_cost_credits": 1},
				{"internal_cost_credits": 1},
				{"first_frame_image": "https://example.com/first.png"},
				{"last_frame_image": "https://example.com/last.png"},
				{"image_urls": []string{"https://example.com/ref.png"}},
				{"video_urls": []string{"https://example.com/ref.mp4"}},
				{"audio_urls": []string{"https://example.com/ref.mp3"}},
				{"webhook": "https://example.com/hook"},
				{"prompt": makeLongPrompt(tc.maxPrompt + 1)},
			}
			for _, override := range unsupported {
				body := map[string]any{
					"video_route_alias": tc.alias,
					"prompt":            "Synthetic scene",
					"duration_sec":      5,
					"resolution":        tc.resolution,
				}
				for key, value := range override {
					body[key] = value
				}
				raw, _ := json.Marshal(body)
				_, err := catalog.VideoRouteCatalog.Resolve(context.Background(), videorouter.Request{
					Operation: domain.OperationVideoGenerate,
					Modality:  domain.ModalityVideo,
					Params:    raw,
				})
				if err == nil {
					t.Fatalf("unsupported input accepted: %+v", override)
				}
			}
			raw, _ := json.Marshal(map[string]any{
				"video_route_alias": tc.alias,
				"prompt":            "Synthetic scene",
				"duration_sec":      5,
				"resolution":        tc.resolution,
			})
			_, err = catalog.VideoRouteCatalog.Resolve(context.Background(), videorouter.Request{
				Operation:        domain.OperationVideoGenerate,
				Modality:         domain.ModalityVideo,
				Params:           raw,
				InputArtifactIDs: []uuid.UUID{uuid.New(), uuid.New()},
			})
			if err == nil {
				t.Fatal("multiple first-frame references accepted")
			}
		})
	}
}

func assertTurboH3PublicRoute(t *testing.T, route *productcatalog.VideoRoute, name, defaultResolution string, minDuration int, resolutions, aspects []string, estimate int64) {
	t.Helper()
	if route.Type != productcatalog.TypeVideo ||
		route.Name != name ||
		route.EstimateCredits != estimate ||
		!route.Enabled ||
		route.DefaultDurationSec != 5 ||
		route.DefaultResolution != defaultResolution ||
		route.SupportsAudio ||
		route.RequiresReferenceVideo ||
		!route.SupportsReferenceImage ||
		route.MaxReferenceImages != 1 {
		t.Fatalf("public route mismatch: %+v", route)
	}
	for duration := minDuration; duration <= 15; duration++ {
		if !hasTurboH3Int(route.AllowedDurationsSec, duration) {
			t.Fatalf("missing duration %d in %+v", duration, route.AllowedDurationsSec)
		}
	}
	assertTurboH3Strings(t, route.AllowedResolutions, resolutions)
	assertTurboH3Strings(t, route.AllowedAspectRatios, aspects)
}

func assertTurboH3Strings(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("values=%+v want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("values=%+v want %+v", got, want)
		}
	}
}

func hasTurboH3Int(values []int, want int) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func makeLongPrompt(chars int) string {
	buf := make([]rune, chars)
	for i := range buf {
		buf[i] = 'x'
	}
	return string(buf)
}
