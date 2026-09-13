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

func TestOmniVideoCatalogReadinessAndBounds(t *testing.T) {
	for _, ext := range []bool{false, true} {
		alias := domain.VideoRouteOmni11Flash
		if ext {
			alias = domain.VideoRouteOmni11FlashExt
		}
		t.Run(string(alias), func(t *testing.T) {
			for _, readiness := range []string{"ready", "disabled", "no key", "no provider", "no router"} {
				t.Run(readiness, func(t *testing.T) {
					cfg := config.Config{FeatureVideoRouterEnabled: true, FeatureAPIMartOmni11FlashEnabled: !ext, FeatureAPIMartOmni11FlashExtEnabled: ext, APIMartProviderEnabled: true, APIMartAPIKey: "test", APIMartBaseURL: "https://example.com"}
					switch readiness {
					case "disabled":
						cfg.FeatureAPIMartOmni11FlashEnabled = false
						cfg.FeatureAPIMartOmni11FlashExtEnabled = false
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
					route := findRoute(catalog.VideoRoutes(), alias)
					if readiness != "ready" {
						if route != nil {
							t.Fatal("unconfigured route exposed")
						}
						return
					}
					wantDuration, wantPrice, maxRefs := 10, int64(530), 10
					if ext {
						wantDuration, wantPrice, maxRefs = 6, 180, 3
					}
					if route == nil || route.AutomaticDuration == ext || route.DefaultDurationSec != wantDuration || route.EstimateCredits != wantPrice || route.MaxReferenceImages != maxRefs {
						t.Fatalf("incorrect public route: %+v", route)
					}
					assertNoPrivateProviderFields(t, catalog.Catalog.Items())
					for _, count := range []int{0, 1, 2, 3, 10, 11} {
						ids := make([]uuid.UUID, count)
						for i := range ids {
							ids[i] = uuid.New()
						}
						params, _ := json.Marshal(map[string]any{"video_route_alias": alias, "resolution": "4k"})
						resolved, err := catalog.VideoRouteCatalog.Resolve(context.Background(), videorouter.Request{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, Params: params, InputArtifactIDs: ids})
						valid := count <= 10
						if ext {
							valid = count == 0 || count == 1 || count == 3
						}
						if valid != (err == nil) {
							t.Fatalf("count=%d valid=%v error=%v", count, valid, err)
						}
						if valid && (resolved.Snapshot.Provider != domain.ProviderAPIMart || resolved.Snapshot.DurationSec != wantDuration) {
							t.Fatal("incorrect route snapshot")
						}
					}
					for _, options := range []map[string]any{{"duration_sec": 5}, {"resolution": "480p"}, {"aspect_ratio": "1:1"}, {"provider": "apimart"}, {"reference_video_url": "https://example.com/video.mp4"}} {
						options["video_route_alias"] = alias
						params, _ := json.Marshal(options)
						_, err := catalog.VideoRouteCatalog.Resolve(context.Background(), videorouter.Request{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, Params: params})
						if err == nil {
							t.Fatal("unsupported selection accepted")
						}
					}
				})
			}
		})
	}
}
