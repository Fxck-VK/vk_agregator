package videorouter_test

import (
	"context"
	"testing"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/videorouter"
)

func TestInjectedEconomyCataloguePreservesCaseInsensitiveResolution(t *testing.T) {
	prices, _ := pricingcatalog.NewStaticCatalog()
	c, err := videorouter.NewCatalog(videorouter.Config{RouterEnabled: true, Pricing: prices, EnabledRoutes: map[domain.VideoRouteAlias]bool{domain.VideoRouteKling30Turbo: true, domain.VideoRouteMiniMaxH3: true}, Providers: map[domain.ProviderName]videorouter.ProviderConfig{domain.ProviderAPIMart: {Enabled: true, APIKeyConfigured: true, BaseURLConfigured: true}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id   domain.VideoRouteAlias
		res  string
		want int64
	}{{domain.VideoRouteKling30Turbo, "720P", 250}, {domain.VideoRouteKling30Turbo, "1080P", 250}, {domain.VideoRouteMiniMaxH3, "2K", 375}} {
		r, err := c.Resolve(context.Background(), videorouter.Request{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, Params: rawJSON(t, map[string]any{"video_route_alias": tc.id, "prompt": "Offline economy test", "resolution": tc.res, "duration_sec": 5})})
		if err != nil || r.InternalCostCredits != tc.want {
			t.Fatalf("%s/%s quote%d err%v", tc.id, tc.res, r.InternalCostCredits, err)
		}
	}
}
