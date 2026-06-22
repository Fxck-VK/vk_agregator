package productcatalog_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/modelcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/videorouter"
)

func TestFromConfigBuildsPublicCatalogFromServerReadiness(t *testing.T) {
	runtimeCatalog, err := productcatalog.FromConfig(config.Config{
		APIMartProviderEnabled:                  true,
		APIMartAPIKey:                           "configured",
		APIMartBaseURL:                          "https://apimart.test",
		PoYoProviderEnabled:                     true,
		PoYoAPIKey:                              "configured",
		PoYoBaseURL:                             "https://poyo.test",
		DeepInfraAPIKey:                         "configured",
		DeepInfraBaseURL:                        "https://deepinfra.test",
		FeatureImageModelNanoBanana2Enabled:     true,
		FeatureImageModelNanoBananaProEnabled:   true,
		FeatureImageModelGPTImage2Enabled:       true,
		FeatureVideoRouterEnabled:               true,
		FeatureVideoRouteKlingO3StandardEnabled: true,
	})
	if err != nil {
		t.Fatalf("build runtime catalog: %v", err)
	}
	if runtimeCatalog.Catalog == nil || runtimeCatalog.VideoRouteCatalog == nil {
		t.Fatalf("missing runtime catalogs: %+v", runtimeCatalog)
	}
	for _, id := range []string{
		modelcatalog.MiniAppImageNanoBanana2,
		modelcatalog.MiniAppImageNanoBananaPro,
		modelcatalog.MiniAppImageGPTImage2,
		modelcatalog.MiniAppImageSeedream45,
		modelcatalog.MiniAppImageSDXLTurbo,
	} {
		if findImage(runtimeCatalog.ImageModels(), id) == nil {
			t.Fatalf("public image %q missing from runtime catalog: %+v", id, runtimeCatalog.ImageModels())
		}
	}
	if !runtimeCatalog.ImageReferenceEnabled {
		t.Fatalf("expected image reference enabled from public reference-capable models")
	}
	if findRoute(runtimeCatalog.VideoRoutes(), domain.VideoRouteKlingO3Standard) == nil {
		t.Fatalf("public video route missing from runtime catalog: %+v", runtimeCatalog.VideoRoutes())
	}
	assertNoPrivateProviderFields(t, runtimeCatalog.Catalog.Items())
}

func TestFromConfigFailsClosedForUnconfiguredProviders(t *testing.T) {
	runtimeCatalog, err := productcatalog.FromConfig(config.Config{
		PoYoProviderEnabled:                     true,
		FeatureImageModelNanoBanana2Enabled:     true,
		FeatureVideoRouterEnabled:               true,
		FeatureVideoRouteKlingO3StandardEnabled: true,
	})
	if err != nil {
		t.Fatalf("build runtime catalog: %v", err)
	}
	if len(runtimeCatalog.ImageModels()) != 0 {
		t.Fatalf("unconfigured image provider leaked models: %+v", runtimeCatalog.ImageModels())
	}
	if len(runtimeCatalog.VideoRoutes()) != 0 {
		t.Fatalf("unconfigured video provider leaked routes: %+v", runtimeCatalog.VideoRoutes())
	}
	if runtimeCatalog.ImageReferenceEnabled {
		t.Fatalf("image reference must fail closed without public reference-capable models")
	}

	params, _ := json.Marshal(map[string]any{
		"video_route_alias": string(domain.VideoRouteKlingO3Standard),
		"duration_sec":      5,
	})
	_, err = runtimeCatalog.VideoRouteCatalog.Resolve(context.Background(), videorouter.Request{
		Source:    "test",
		Operation: domain.OperationVideoGenerate,
		Modality:  domain.ModalityVideo,
		Params:    params,
	})
	if !errors.Is(err, videorouter.ErrProviderUnconfigured) {
		t.Fatalf("hidden unconfigured route must still reject server-side, got %v", err)
	}
}

func TestFromConfigDeepInfraImagesUseProviderLevelReadiness(t *testing.T) {
	runtimeCatalog, err := productcatalog.FromConfig(config.Config{
		DeepInfraAPIKey:  "configured",
		DeepInfraBaseURL: "https://deepinfra.test",
	})
	if err != nil {
		t.Fatalf("build runtime catalog: %v", err)
	}
	if findImage(runtimeCatalog.ImageModels(), modelcatalog.MiniAppImageSeedream45) == nil ||
		findImage(runtimeCatalog.ImageModels(), modelcatalog.MiniAppImageSDXLTurbo) == nil {
		t.Fatalf("legacy DeepInfra images must be visible when provider-level readiness is configured: %+v", runtimeCatalog.ImageModels())
	}
	assertNoPrivateProviderFields(t, runtimeCatalog.Catalog.Items())

	runtimeCatalog, err = productcatalog.FromConfig(config.Config{
		DeepInfraAPIKey: "configured",
	})
	if err != nil {
		t.Fatalf("build runtime catalog without DeepInfra base URL: %v", err)
	}
	if findImage(runtimeCatalog.ImageModels(), modelcatalog.MiniAppImageSeedream45) != nil ||
		findImage(runtimeCatalog.ImageModels(), modelcatalog.MiniAppImageSDXLTurbo) != nil {
		t.Fatalf("legacy DeepInfra images must fail closed without key/base URL readiness: %+v", runtimeCatalog.ImageModels())
	}
}

func findImage(models []productcatalog.ImageModel, id string) *productcatalog.ImageModel {
	for i := range models {
		if models[i].ID == id {
			return &models[i]
		}
	}
	return nil
}

func findRoute(routes []productcatalog.VideoRoute, alias domain.VideoRouteAlias) *productcatalog.VideoRoute {
	for i := range routes {
		if routes[i].Alias == string(alias) {
			return &routes[i]
		}
	}
	return nil
}
