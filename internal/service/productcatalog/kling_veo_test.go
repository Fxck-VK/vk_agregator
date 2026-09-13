package productcatalog_test

import (
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/productcatalog"
)

func TestKlingVeoCatalogFlagsExposePricedPublicRoutes(t *testing.T) {
	for _, tc := range klingVeoCatalogCases() {
		t.Run(string(tc.alias), func(t *testing.T) {
			cfg := baseKlingVeoAPIMartConfig()
			tc.enable(&cfg)
			runtime, err := productcatalog.FromConfig(cfg, staticPricingCatalog(t))
			if err != nil {
				t.Fatalf("build runtime catalog: %v", err)
			}

			routes := runtime.VideoRoutes()
			if len(routes) != 1 {
				t.Fatalf("visible routes = %d, want only %s: %+v", len(routes), tc.alias, routes)
			}
			route := findRoute(routes, tc.alias)
			if route == nil {
				t.Fatalf("%s route missing from %+v", tc.alias, routes)
			}
			assertKlingVeoCatalogRoute(t, route, tc)

			item := findItem(runtime.Catalog.Items(), string(tc.alias))
			if item == nil {
				t.Fatalf("%s item missing from %+v", tc.alias, runtime.Catalog.Items())
			}
			if item.Alias != string(tc.alias) ||
				item.EstimateCredits != tc.estimate ||
				item.SupportsAudio != tc.supportsAudio ||
				item.RequiresReferenceVideo != tc.requiresVideo ||
				item.SupportsReferenceImage != tc.supportsImages ||
				item.MaxReferenceImages != tc.maxImages {
				t.Fatalf("%s item mismatch: %+v", tc.alias, item)
			}
			assertNoPrivateProviderFields(t, runtime.Catalog.Items())
		})
	}
}

func TestKlingVeoCatalogFlagsHideUnreadyOrUnpricedRoutes(t *testing.T) {
	for _, routeCase := range klingVeoCatalogCases() {
		for _, hiddenCase := range []struct {
			name    string
			mutate  func(*config.Config)
			pricing bool
		}{
			{name: "feature disabled", mutate: func(*config.Config) {}, pricing: true},
			{name: "router disabled", mutate: func(cfg *config.Config) {
				routeCase.enable(cfg)
				cfg.FeatureVideoRouterEnabled = false
			}, pricing: true},
			{name: "provider disabled", mutate: func(cfg *config.Config) {
				routeCase.enable(cfg)
				cfg.APIMartProviderEnabled = false
			}, pricing: true},
			{name: "api key missing", mutate: func(cfg *config.Config) {
				routeCase.enable(cfg)
				cfg.APIMartAPIKey = ""
			}, pricing: true},
			{name: "base url missing", mutate: func(cfg *config.Config) {
				routeCase.enable(cfg)
				cfg.APIMartBaseURL = ""
			}, pricing: true},
			{name: "pricing missing", mutate: routeCase.enable, pricing: false},
		} {
			t.Run(string(routeCase.alias)+"/"+hiddenCase.name, func(t *testing.T) {
				cfg := baseKlingVeoAPIMartConfig()
				hiddenCase.mutate(&cfg)
				prices := emptyPricingCatalog(t)
				if hiddenCase.pricing {
					prices = staticPricingCatalog(t)
				}
				runtime, err := productcatalog.FromConfig(cfg, prices)
				if err != nil {
					t.Fatalf("build runtime catalog: %v", err)
				}
				if route := findRoute(runtime.VideoRoutes(), routeCase.alias); route != nil {
					t.Fatalf("%s exposed despite %s: %+v", routeCase.alias, hiddenCase.name, route)
				}
				if item := findItem(runtime.Catalog.Items(), string(routeCase.alias)); item != nil {
					t.Fatalf("%s item exposed despite %s: %+v", routeCase.alias, hiddenCase.name, item)
				}
			})
		}
	}
}

type klingVeoCatalogCase struct {
	alias           domain.VideoRouteAlias
	enable          func(*config.Config)
	estimate        int64
	defaultDuration int
	defaultRes      string
	maxDuration     int
	requiresVideo   bool
	requiresStart   bool
	supportsAudio   bool
	supportsImages  bool
	maxImages       int
}

func klingVeoCatalogCases() []klingVeoCatalogCase {
	return []klingVeoCatalogCase{
		{
			alias:           domain.VideoRouteKlingV3,
			enable:          func(cfg *config.Config) { cfg.FeatureAPIMartKlingV3Enabled = true },
			estimate:        205,
			defaultDuration: 5,
			defaultRes:      "720p",
			maxDuration:     15,
			supportsAudio:   true,
			supportsImages:  true,
			maxImages:       2,
		},
		{
			alias:           domain.VideoRouteKling26Motion,
			enable:          func(cfg *config.Config) { cfg.FeatureAPIMartKling26MotionEnabled = true },
			estimate:        105,
			defaultDuration: 3,
			defaultRes:      "std",
			maxDuration:     30,
			requiresVideo:   true,
			requiresStart:   true,
			supportsImages:  true,
			maxImages:       1,
		},
		{
			alias:           domain.VideoRouteVeo31Lite,
			enable:          func(cfg *config.Config) { cfg.FeatureAPIMartVeo31LiteEnabled = true },
			estimate:        45,
			defaultDuration: 8,
			defaultRes:      "720p",
			maxDuration:     8,
		},
		{
			alias:           domain.VideoRouteVeo31Fast,
			enable:          func(cfg *config.Config) { cfg.FeatureAPIMartVeo31FastEnabled = true },
			estimate:        85,
			defaultDuration: 8,
			defaultRes:      "720p",
			maxDuration:     8,
			supportsImages:  true,
			maxImages:       3,
		},
		{
			alias:           domain.VideoRouteVeo31Quality,
			enable:          func(cfg *config.Config) { cfg.FeatureAPIMartVeo31QualityEnabled = true },
			estimate:        600,
			defaultDuration: 8,
			defaultRes:      "720p",
			maxDuration:     8,
			supportsImages:  true,
			maxImages:       2,
		},
	}
}

func baseKlingVeoAPIMartConfig() config.Config {
	return config.Config{
		FeatureVideoRouterEnabled: true,
		APIMartProviderEnabled:    true,
		APIMartAPIKey:             "test",
		APIMartBaseURL:            "https://example.com",
	}
}

func assertKlingVeoCatalogRoute(t *testing.T, route *productcatalog.VideoRoute, want klingVeoCatalogCase) {
	t.Helper()
	if route.Type != productcatalog.TypeVideo ||
		route.Alias != string(want.alias) ||
		route.EstimateCredits != want.estimate ||
		!route.Enabled ||
		route.DefaultDurationSec != want.defaultDuration ||
		route.DefaultResolution != want.defaultRes ||
		route.RequiresReferenceVideo != want.requiresVideo ||
		route.RequiresStartImage != want.requiresStart ||
		route.SupportsAudio != want.supportsAudio ||
		route.SupportsReferenceImage != want.supportsImages ||
		route.MaxReferenceImages != want.maxImages {
		t.Fatalf("%s route mismatch: %+v", want.alias, route)
	}
	if !hasDuration(route.AllowedDurationsSec, want.defaultDuration) || !hasDuration(route.AllowedDurationsSec, want.maxDuration) {
		t.Fatalf("%s durations missing expected bounds: %+v", want.alias, route.AllowedDurationsSec)
	}
	if !hasResolution(route.AllowedResolutions, want.defaultRes) {
		t.Fatalf("%s resolutions missing %q: %+v", want.alias, want.defaultRes, route.AllowedResolutions)
	}
}

func hasDuration(values []int, want int) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func hasResolution(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
