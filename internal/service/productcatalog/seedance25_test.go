package productcatalog_test

import (
	"context"
	"testing"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/videorouter"
)

func TestSeedance25CatalogReadinessAndResolutionCosts(t *testing.T) {
	base := config.Config{FeatureVideoRouterEnabled: true, FeatureAPIMartSeedance25Enabled: true, APIMartProviderEnabled: true, APIMartAPIKey: "test", APIMartBaseURL: "https://example.com"}
	for _, name := range []string{"ready", "disabled", "no key", "no provider", "no router"} {
		t.Run(name, func(t *testing.T) {
			cfg := base
			switch name {
			case "disabled":
				cfg.FeatureAPIMartSeedance25Enabled = false
			case "no key":
				cfg.APIMartAPIKey = ""
			case "no provider":
				cfg.APIMartProviderEnabled = false
			case "no router":
				cfg.FeatureVideoRouterEnabled = false
			}
			catalog, err := productcatalog.FromConfig(cfg, staticPricingCatalog(t))
			if err != nil {
				t.Fatal(err)
			}
			route := findRoute(catalog.VideoRoutes(), domain.VideoRouteSeedance25)
			if name != "ready" {
				if route != nil {
					t.Fatal("unconfigured route exposed")
				}
				return
			}
			if route == nil || route.Name != "Seedance 2.5" || route.EstimateCredits != 290 || len(route.AllowedDurationsSec) != 4 || len(route.AllowedResolutions) != 3 {
				t.Fatal("incomplete public route")
			}
			assertNoPrivateProviderFields(t, catalog.Catalog.Items())
			for _, tc := range []struct {
				resolution string
				credits    int64
			}{{"480p", 29}, {"720p", 65}, {"1080p", 116}} {
				params := []byte(`{"video_route_alias":"video_seedance_2_5","duration_sec":30,"resolution":"` + tc.resolution + `"}`)
				resolved, err := catalog.VideoRouteCatalog.Resolve(context.Background(), videorouter.Request{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, Params: params})
				if err != nil {
					t.Fatal(err)
				}
				if resolved.Snapshot.Provider != domain.ProviderAPIMart || resolved.Snapshot.ProviderModelID != "seedance-2.5" || resolved.Snapshot.ProviderCostCredits != tc.credits {
					t.Fatal("incorrect worker route or resolution-specific spend estimate")
				}
			}
		})
	}
}
