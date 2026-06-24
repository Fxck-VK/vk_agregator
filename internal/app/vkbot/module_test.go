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
