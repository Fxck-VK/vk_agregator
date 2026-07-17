package vkbot

import (
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
)

func TestMenuFeaturesUseRuntimeProductCatalogVisibility(t *testing.T) {
	runtimeCatalog, err := productcatalog.FromConfig(config.Config{
		PoYoProviderEnabled:                     true,
		PoYoAPIKey:                              "configured",
		PoYoBaseURL:                             "https://poyo.test",
		FeatureImageModelNanoBanana2Enabled:     true,
		FeatureVideoRouterEnabled:               true,
		FeatureVideoRouteKlingO3StandardEnabled: true,
	}, staticPricingCatalog(t))
	if err != nil {
		t.Fatalf("build runtime catalog: %v", err)
	}
	features := menuFeatures(config.Config{
		VKMenuImageEnabled:                true,
		VKMenuImageReferenceEnabled:       true,
		VKMenuVideoEnabled:                true,
		VKMenuVideoKling21Enabled:         true,
		VKMenuVideoKling21StartEnabled:    true,
		VKMenuVideoKling21ExamplesEnabled: true,
	}, runtimeCatalog)

	assertCommandVisible(t, features.DisabledCommands, domain.CommandMenuImage)
	assertCommandVisible(t, features.DisabledCommands, domain.CommandMenuImageNanoBanana2)
	assertCommandVisible(t, features.DisabledCommands, domain.CommandMenuImageReference)
	assertCommandVisible(t, features.DisabledCommands, domain.CommandMenuVideo)
	assertCommandVisible(t, features.DisabledCommands, domain.CommandMenuVideoKling21)
	assertCommandVisible(t, features.DisabledCommands, domain.CommandMenuVideoKling21Start)
	assertCommandVisible(t, features.DisabledCommands, domain.CommandMenuVideoKling21Examples)
	assertCommandEnabled(t, features.EnabledCommands, domain.CommandMenuVideoKling21Start)
}

func TestMenuFeaturesFailClosedWhenRuntimeCatalogHasNoPublicItems(t *testing.T) {
	runtimeCatalog, err := productcatalog.FromConfig(config.Config{
		PoYoProviderEnabled:                     true,
		FeatureImageModelNanoBanana2Enabled:     true,
		FeatureVideoRouterEnabled:               true,
		FeatureVideoRouteKlingO3StandardEnabled: true,
	}, staticPricingCatalog(t))
	if err != nil {
		t.Fatalf("build runtime catalog: %v", err)
	}
	features := menuFeatures(config.Config{
		VKMenuImageEnabled:                true,
		VKMenuImageReferenceEnabled:       true,
		VKMenuVideoEnabled:                true,
		VKMenuVideoKling21Enabled:         true,
		VKMenuVideoKling21StartEnabled:    true,
		VKMenuVideoKling21ExamplesEnabled: true,
	}, runtimeCatalog)

	assertCommandHidden(t, features.DisabledCommands, domain.CommandMenuImage)
	assertCommandHidden(t, features.DisabledCommands, domain.CommandMenuImageNanoBanana2)
	assertCommandHidden(t, features.DisabledCommands, domain.CommandMenuImageReference)
	assertCommandHidden(t, features.DisabledCommands, domain.CommandMenuVideo)
	assertCommandHidden(t, features.DisabledCommands, domain.CommandMenuVideoKling21)
	assertCommandHidden(t, features.DisabledCommands, domain.CommandMenuVideoKling21Start)
	assertCommandHidden(t, features.DisabledCommands, domain.CommandMenuVideoKling21Examples)
	if features.EnabledCommands[domain.CommandMenuVideoKling21Start] {
		t.Fatalf("unconfigured catalog must not explicitly enable %s", domain.CommandMenuVideoKling21Start)
	}
}

func TestMenuFeaturesDoNotFeatureGateImageReferenceCommand(t *testing.T) {
	runtimeCatalog, err := productcatalog.FromConfig(config.Config{
		PoYoProviderEnabled:                 true,
		PoYoAPIKey:                          "configured",
		PoYoBaseURL:                         "https://poyo.test",
		FeatureImageModelNanoBanana2Enabled: true,
	}, staticPricingCatalog(t))
	if err != nil {
		t.Fatalf("build runtime catalog: %v", err)
	}

	features := menuFeatures(config.Config{
		VKMenuImageEnabled:          true,
		VKMenuImageReferenceEnabled: false,
	}, runtimeCatalog)

	assertCommandVisible(t, features.DisabledCommands, domain.CommandMenuImage)
	if features.DisabledCommands[domain.CommandMenuImageReference] {
		t.Fatalf("%s must not be a separately visible feature gate; stale callbacks should flow through the normal photo menu", domain.CommandMenuImageReference)
	}
}

func TestMenuFeaturesExposeNanoBananaProFromAPIMartReadiness(t *testing.T) {
	runtimeCatalog, err := productcatalog.FromConfig(config.Config{
		APIMartProviderEnabled:                true,
		APIMartAPIKey:                         "configured",
		APIMartBaseURL:                        "https://apimart.test",
		FeatureImageModelNanoBananaProEnabled: true,
	}, staticPricingCatalog(t))
	if err != nil {
		t.Fatalf("build runtime catalog: %v", err)
	}

	features := menuFeatures(config.Config{
		VKMenuImageEnabled: true,
	}, runtimeCatalog)

	assertCommandVisible(t, features.DisabledCommands, domain.CommandMenuImage)
	assertCommandVisible(t, features.DisabledCommands, domain.CommandMenuImageText)
	assertCommandHidden(t, features.DisabledCommands, domain.CommandMenuImageNanoBanana2)
}

func TestMenuFeaturesKeepOfficialRunwayAndExposePoyoRunwayGen45(t *testing.T) {
	runtimeCatalog, err := productcatalog.FromConfig(config.Config{
		PoYoProviderEnabled:                     true,
		PoYoAPIKey:                              "configured",
		PoYoBaseURL:                             "https://poyo.test",
		RunwayProviderEnabled:                   true,
		RunwayMLAPISecret:                       "configured",
		RunwayMLBaseURL:                         "https://runway.test/v1",
		FeatureVideoRouterEnabled:               true,
		FeatureVideoRouteRunwayGen4TurboEnabled: true,
		FeatureVideoRouteRunwayGen45Enabled:     true,
	}, staticPricingCatalog(t))
	if err != nil {
		t.Fatalf("build runtime catalog: %v", err)
	}

	if findRuntimeVideoRoute(runtimeCatalog.VideoRoutes(), domain.VideoRouteRunwayGen4Turbo) == nil {
		t.Fatalf("official Runway route missing from runtime catalog: %+v", runtimeCatalog.VideoRoutes())
	}
	if route := findRuntimeVideoRoute(runtimeCatalog.VideoRoutes(), domain.VideoRouteRunwayGen45); route == nil {
		t.Fatalf("PoYo Runway Gen-4.5 route missing from runtime catalog: %+v", runtimeCatalog.VideoRoutes())
	} else if route.Name != "Runway Gen-4.5" ||
		!route.SupportsReferenceImage ||
		route.MaxReferenceImages != 1 ||
		route.RequiresStartImage {
		t.Fatalf("unexpected PoYo Runway Gen-4.5 public route: %+v", route)
	}

	features := menuFeatures(config.Config{
		VKMenuVideoEnabled:              true,
		VKMenuVideoSora2Enabled:         true,
		VKMenuVideoSora2StartEnabled:    true,
		VKMenuVideoSora2ExamplesEnabled: true,
	}, runtimeCatalog)
	assertCommandVisible(t, features.DisabledCommands, domain.CommandMenuVideo)
	assertCommandVisible(t, features.DisabledCommands, domain.CommandMenuVideoSora2Start)
	assertCommandEnabled(t, features.EnabledCommands, domain.CommandMenuVideoSora2Start)
}

func findRuntimeVideoRoute(routes []productcatalog.VideoRoute, alias domain.VideoRouteAlias) *productcatalog.VideoRoute {
	for i := range routes {
		if routes[i].Alias == string(alias) {
			return &routes[i]
		}
	}
	return nil
}

func staticPricingCatalog(t *testing.T) *pricingcatalog.Catalog {
	t.Helper()
	catalog, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatalf("build pricing catalog: %v", err)
	}
	return catalog
}

func assertCommandVisible(t *testing.T, disabled map[domain.CommandType]bool, command domain.CommandType) {
	t.Helper()
	if disabled[command] {
		t.Fatalf("expected %s to be visible", command)
	}
}

func assertCommandHidden(t *testing.T, disabled map[domain.CommandType]bool, command domain.CommandType) {
	t.Helper()
	if !disabled[command] {
		t.Fatalf("expected %s to be hidden", command)
	}
}

func assertCommandEnabled(t *testing.T, enabled map[domain.CommandType]bool, command domain.CommandType) {
	t.Helper()
	if !enabled[command] {
		t.Fatalf("expected %s to be explicitly enabled", command)
	}
}
