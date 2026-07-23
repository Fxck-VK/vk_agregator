package providermodels_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/modelcatalog"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/videorouter"
)

func TestRegistryIncludesCurrentPricedPublicImageModels(t *testing.T) {
	registry := providermodels.StaticRegistry()

	wantIDs := []string{
		modelcatalog.MiniAppImageNanoBanana2,
		modelcatalog.MiniAppImageNanoBananaPro,
		modelcatalog.MiniAppImageGPTImage2,
	}
	for _, publicID := range wantIDs {
		model, ok := registry.PublicImageModel(publicID)
		if !ok {
			t.Fatalf("public image model %s missing from registry", publicID)
		}
		catalogModel, ok := modelcatalog.ResolveMiniAppModel(domain.OperationImageGenerate, publicID)
		if !ok {
			t.Fatalf("modelcatalog no longer resolves %s", publicID)
		}
		if model.Provider != catalogModel.Provider || model.ProviderModelID != catalogModel.ModelCode {
			t.Fatalf("%s provider mapping = %s/%s, want %s/%s", publicID, model.Provider, model.ProviderModelID, catalogModel.Provider, catalogModel.ModelCode)
		}
		if model.FeatureFlag == "" {
			t.Fatalf("%s missing feature flag", publicID)
		}
		if model.Readiness.ProviderEnabledFlag == "" || len(model.Readiness.RequiredConfigKeys) == 0 {
			t.Fatalf("%s missing provider readiness requirements: %+v", publicID, model.Readiness)
		}
		if model.Limits.MaxReferenceImages != catalogModel.MaxReferenceImages || !model.Limits.SupportsReferenceImage {
			t.Fatalf("%s reference limits = %+v, want max refs %d and reference support", publicID, model.Limits, catalogModel.MaxReferenceImages)
		}
		if !reflect.DeepEqual(model.Limits.AllowedQualities, []string{modelcatalog.ImageQuality1K, modelcatalog.ImageQuality2K, modelcatalog.ImageQuality4K}) {
			t.Fatalf("%s qualities = %#v", publicID, model.Limits.AllowedQualities)
		}
		if len(model.PricingKeys) != 3 {
			t.Fatalf("%s pricing keys = %d, want 3", publicID, len(model.PricingKeys))
		}
		for _, key := range model.PricingKeys {
			if !key.Valid() || key.ImageModelID != publicID {
				t.Fatalf("%s invalid pricing key: %+v", publicID, key)
			}
		}
	}

	if _, ok := registry.PublicImageModel(modelcatalog.MiniAppImageMock); ok {
		t.Fatalf("loadtest-only mock image must not be in priced public image registry")
	}
}

func TestRegistryIncludesTextAliasAndLoadTestImageSeparately(t *testing.T) {
	registry := providermodels.StaticRegistry()

	text, ok := registry.TextAlias(modelcatalog.MiniAppChatModelID)
	if !ok {
		t.Fatalf("text alias %s missing", modelcatalog.MiniAppChatModelID)
	}
	if text.Provider != domain.ProviderDeepInfra || text.ProviderModelID == "" {
		t.Fatalf("text alias provider mapping is incomplete: %+v", text)
	}
	if text.FeatureFlag != "" || len(text.PricingKeys) != 0 {
		t.Fatalf("text alias should not pretend to have product pricing or feature flag: %+v", text)
	}

	mock, ok := registry.LoadTestImageModel(modelcatalog.MiniAppImageMock)
	if !ok {
		t.Fatalf("loadtest image model %s missing", modelcatalog.MiniAppImageMock)
	}
	if !mock.LoadTestOnly || mock.Provider != domain.ProviderMock || len(mock.PricingKeys) != 0 {
		t.Fatalf("mock image should be tracked as loadtest-only and unpriced: %+v", mock)
	}
}

func TestRegistryGPTImage2ReferenceLimitIsIsolated(t *testing.T) {
	registry := providermodels.StaticRegistry()

	gpt, ok := registry.PublicImageModel(modelcatalog.MiniAppImageGPTImage2)
	if !ok {
		t.Fatal("GPT Image 2 image model missing")
	}
	if gpt.Provider != domain.ProviderAPIMart || gpt.ProviderModelID != providermodels.ProviderModelGPTImage2 {
		t.Fatalf("GPT Image 2 provider mapping drifted: %+v", gpt)
	}
	if !gpt.Limits.SupportsReferenceImage || gpt.Limits.MaxReferenceImages != 16 {
		t.Fatalf("GPT Image 2 reference limits = %+v, want max refs 16", gpt.Limits)
	}

	stableImageLimits := map[string]int{
		modelcatalog.MiniAppImageNanoBananaPro: 14,
	}
	for publicID, want := range stableImageLimits {
		model, ok := registry.PublicImageModel(publicID)
		if !ok {
			t.Fatalf("%s image model missing", publicID)
		}
		if model.Limits.MaxReferenceImages != want {
			t.Fatalf("%s max reference images = %d, want unchanged %d", publicID, model.Limits.MaxReferenceImages, want)
		}
	}

	stableVideoLimits := map[domain.VideoRouteAlias]int{
		domain.VideoRouteRunwayGen4Turbo: 1,
		domain.VideoRouteRunwayGen45:     1,
	}
	for alias, want := range stableVideoLimits {
		route, ok := registry.VideoRoute(alias)
		if !ok {
			t.Fatalf("%s video route missing", alias)
		}
		if route.Limits.MaxReferenceImages != want {
			t.Fatalf("%s max reference images = %d, want unchanged %d", alias, route.Limits.MaxReferenceImages, want)
		}
	}
}

func TestRegistryNanoBanana2UsesPoyoSourceContract(t *testing.T) {
	registry := providermodels.StaticRegistry()

	model, ok := registry.PublicImageModel(modelcatalog.MiniAppImageNanoBanana2)
	if !ok {
		t.Fatal("Nano Banana 2 image model missing")
	}
	if model.Provider != domain.ProviderPoYo {
		t.Fatalf("Nano Banana 2 provider = %s, want %s", model.Provider, domain.ProviderPoYo)
	}
	if model.ProviderModelID != providermodels.ProviderModelPoYoNanoBanana2 {
		t.Fatalf("Nano Banana 2 provider model = %q, want %q", model.ProviderModelID, providermodels.ProviderModelPoYoNanoBanana2)
	}
	if model.ProviderModelID != "nano-banana-2-new" {
		t.Fatalf("Nano Banana 2 provider model = %q, want source-doc nano-banana-2-new", model.ProviderModelID)
	}
	if !model.Limits.SupportsReferenceImage || model.Limits.MaxReferenceImages != 14 {
		t.Fatalf("Nano Banana 2 reference limits = %+v, want optional refs max 14", model.Limits)
	}
}

func TestRegistryNanoBananaProUsesAPIMartGemini3ProContract(t *testing.T) {
	registry := providermodels.StaticRegistry()

	model, ok := registry.PublicImageModel(modelcatalog.MiniAppImageNanoBananaPro)
	if !ok {
		t.Fatal("Nano Banana Pro image model missing")
	}
	if model.Provider != domain.ProviderAPIMart {
		t.Fatalf("Nano Banana Pro provider = %s, want %s", model.Provider, domain.ProviderAPIMart)
	}
	if model.ProviderModelID != providermodels.ProviderModelGemini3ProImage {
		t.Fatalf("Nano Banana Pro provider model = %q, want %q", model.ProviderModelID, providermodels.ProviderModelGemini3ProImage)
	}
	if model.Readiness.ProviderEnabledFlag != providermodels.ProviderFlagAPIMart {
		t.Fatalf("Nano Banana Pro readiness flag = %q, want %q", model.Readiness.ProviderEnabledFlag, providermodels.ProviderFlagAPIMart)
	}
	if !reflect.DeepEqual(model.Readiness.RequiredConfigKeys, []string{providermodels.ConfigKeyAPIMartAPIKey, providermodels.ConfigKeyAPIMartBaseURL}) {
		t.Fatalf("Nano Banana Pro readiness keys = %#v", model.Readiness.RequiredConfigKeys)
	}
	if !model.Limits.SupportsReferenceImage || model.Limits.MaxReferenceImages != 14 {
		t.Fatalf("Nano Banana Pro reference limits = %+v, want optional refs max 14", model.Limits)
	}

	unchangedImages := map[string]struct {
		provider domain.ProviderName
		modelID  string
		maxRefs  int
	}{
		modelcatalog.MiniAppImageNanoBanana2: {provider: domain.ProviderPoYo, modelID: providermodels.ProviderModelPoYoNanoBanana2, maxRefs: 14},
		modelcatalog.MiniAppImageGPTImage2:   {provider: domain.ProviderAPIMart, modelID: providermodels.ProviderModelGPTImage2, maxRefs: 16},
	}
	for publicID, want := range unchangedImages {
		got, ok := registry.PublicImageModel(publicID)
		if !ok {
			t.Fatalf("%s image model missing", publicID)
		}
		if got.Provider != want.provider || got.ProviderModelID != want.modelID || got.Limits.MaxReferenceImages != want.maxRefs {
			t.Fatalf("%s drifted: provider=%s model=%s maxRefs=%d, want %s/%s/%d",
				publicID, got.Provider, got.ProviderModelID, got.Limits.MaxReferenceImages, want.provider, want.modelID, want.maxRefs)
		}
	}
}

func TestRegistrySeedream45UsesPoyoSourceContract(t *testing.T) {
	registry := providermodels.StaticRegistry()

	model, ok := registry.PublicImageModel(modelcatalog.MiniAppImageSeedream45)
	if !ok {
		t.Fatal("Seedream 4.5 image model missing")
	}
	if model.PublicID != providermodels.PublicImageSeedream45 {
		t.Fatalf("Seedream 4.5 public id = %q, want %q", model.PublicID, providermodels.PublicImageSeedream45)
	}
	if model.Provider != domain.ProviderPoYo {
		t.Fatalf("Seedream 4.5 provider = %s, want %s", model.Provider, domain.ProviderPoYo)
	}
	if model.ProviderModelID != providermodels.ProviderModelPoYoSeedream45 {
		t.Fatalf("Seedream 4.5 provider model = %q, want %q", model.ProviderModelID, providermodels.ProviderModelPoYoSeedream45)
	}
	if model.FeatureFlag != providermodels.FeatureImageSeedream45 {
		t.Fatalf("Seedream 4.5 feature flag = %q, want %q", model.FeatureFlag, providermodels.FeatureImageSeedream45)
	}
	if model.Readiness.ProviderEnabledFlag != providermodels.ProviderFlagPoYo {
		t.Fatalf("Seedream 4.5 readiness flag = %q, want %q", model.Readiness.ProviderEnabledFlag, providermodels.ProviderFlagPoYo)
	}
	if !reflect.DeepEqual(model.Readiness.RequiredConfigKeys, []string{providermodels.ConfigKeyPoYoAPIKey, providermodels.ConfigKeyPoYoBaseURL}) {
		t.Fatalf("Seedream 4.5 readiness keys = %#v", model.Readiness.RequiredConfigKeys)
	}
	if !model.Limits.SupportsReferenceImage || model.Limits.MaxReferenceImages != 10 {
		t.Fatalf("Seedream 4.5 reference limits = %+v, want optional refs max 10", model.Limits)
	}
	if !reflect.DeepEqual(model.Limits.AllowedQualities, []string{modelcatalog.ImageQuality2K, modelcatalog.ImageQuality4K}) {
		t.Fatalf("Seedream 4.5 qualities = %#v, want 2K/4K only", model.Limits.AllowedQualities)
	}
	if len(model.PricingKeys) != 2 {
		t.Fatalf("Seedream 4.5 pricing keys = %d, want 2", len(model.PricingKeys))
	}
	for _, key := range model.PricingKeys {
		if key.ImageModelID != providermodels.PublicImageSeedream45 {
			t.Fatalf("Seedream 4.5 pricing key model = %q", key.ImageModelID)
		}
		if key.Quality == pricingcatalog.ImageQuality1K {
			t.Fatalf("Seedream 4.5 must not include 1K pricing key: %+v", key)
		}
	}
}

func TestRegistryVideoRoutesMatchCurrentRouterSpecs(t *testing.T) {
	registry := providermodels.StaticRegistry()
	routes := registry.VideoRoutes()
	specs := videorouter.DefaultRouteSpecs()
	if len(routes) != len(specs) {
		t.Fatalf("video routes = %d, want %d", len(routes), len(specs))
	}

	for _, want := range specs {
		route, ok := registry.VideoRoute(want.Alias)
		if !ok {
			t.Fatalf("video route %s missing", want.Alias)
		}
		if !reflect.DeepEqual(route.Spec, want) {
			t.Fatalf("route spec %s drifted:\ngot  %+v\nwant %+v", want.Alias, route.Spec, want)
		}
		if route.FeatureFlag == "" {
			t.Fatalf("route %s missing feature flag", want.Alias)
		}
		if route.Provider != domain.ProviderMock && (route.Readiness.ProviderEnabledFlag == "" || len(route.Readiness.RequiredConfigKeys) == 0) {
			t.Fatalf("route %s missing provider readiness requirements: %+v", want.Alias, route.Readiness)
		}
		if route.MediaContract.ModelClass != want.ModelClass || route.MediaContract.Modality != domain.ModalityVideo {
			t.Fatalf("route %s media contract class incomplete: %+v", want.Alias, route.MediaContract)
		}
	}
}

func TestRegistryKeepsOfficialRunwayAndAddsPoyoRunwayGen45Separately(t *testing.T) {
	registry := providermodels.StaticRegistry()

	official, ok := registry.VideoRoute(domain.VideoRouteRunwayGen4Turbo)
	if !ok {
		t.Fatal("official Runway Gen-4 Turbo route missing")
	}
	if official.Provider != domain.ProviderRunway ||
		official.ProviderModelID != "gen4_turbo" ||
		official.ModelClass != "runway_gen4_turbo" ||
		official.FeatureFlag != providermodels.FeatureVideoRunwayGen4Turbo ||
		!official.Limits.RequiresStartImage ||
		!official.Limits.SupportsReferenceImage ||
		official.Limits.MaxReferenceImages != 1 {
		t.Fatalf("official Runway route drifted: %+v", official)
	}
	if len(official.PricingKeys) == 0 {
		t.Fatalf("official Runway route lost active pricing keys: %+v", official)
	}
	if !reflect.DeepEqual(official.Limits.AllowedDurationsSec, []int{5, 10}) {
		t.Fatalf("official Runway durations = %#v, want 5/10", official.Limits.AllowedDurationsSec)
	}
	if len(official.PricingKeys) != 2 {
		t.Fatalf("official Runway pricing keys = %d, want 2", len(official.PricingKeys))
	}

	poyo, ok := registry.VideoRoute(domain.VideoRouteRunwayGen45)
	if !ok {
		t.Fatal("PoYo Runway Gen-4.5 route missing")
	}
	if poyo.Provider != domain.ProviderPoYo ||
		poyo.ProviderModelID != "runway-gen-4.5" ||
		poyo.ModelClass != "runway_gen4_5" ||
		poyo.FeatureFlag != providermodels.FeatureVideoRunwayGen45 ||
		poyo.Limits.RequiresStartImage ||
		!poyo.Limits.SupportsReferenceImage ||
		poyo.Limits.MaxReferenceImages != 1 {
		t.Fatalf("PoYo Runway Gen-4.5 route metadata drifted: %+v", poyo)
	}
	if !reflect.DeepEqual(poyo.Limits.AllowedDurationsSec, []int{5, 10}) {
		t.Fatalf("PoYo Runway Gen-4.5 durations = %#v, want 5/10", poyo.Limits.AllowedDurationsSec)
	}
	if !reflect.DeepEqual(poyo.Limits.AllowedAspectRatios, []string{"16:9", "9:16", "4:3", "3:4", "1:1", "21:9"}) {
		t.Fatalf("PoYo Runway Gen-4.5 aspect ratios = %#v", poyo.Limits.AllowedAspectRatios)
	}
	if len(poyo.PricingKeys) != 4 {
		t.Fatalf("PoYo Runway Gen-4.5 pricing keys = %d, want 4", len(poyo.PricingKeys))
	}
	if poyo.Spec.ProviderCostCreditsPerSecond <= 0 ||
		poyo.Spec.MaxProviderCostCredits <= 0 ||
		poyo.Spec.MaxInternalCostCredits <= 0 {
		t.Fatalf("PoYo Runway Gen-4.5 must be routable with bounded provider cost: %+v", poyo.Spec)
	}
}

func TestRegistryPricingCoverageMatchesCurrentCatalogs(t *testing.T) {
	registry := providermodels.StaticRegistry()
	catalog, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatalf("pricing catalog: %v", err)
	}
	if err := registry.Validate(); err != nil {
		t.Fatalf("registry validate: %v", err)
	}
	if err := registry.ValidatePricingCoverage(catalog, pricingcatalog.DisabledStaticProductPrices()); err != nil {
		t.Fatalf("pricing coverage: %v", err)
	}
}

func TestRegistryValidationFailsClosedForMissingProviderMetadata(t *testing.T) {
	registry := providermodels.Registry{
		ImageModels: []providermodels.ImageModel{
			{
				PublicID:    "broken_image",
				FeatureFlag: "FEATURE_BROKEN_IMAGE_ENABLED",
				PricingKeys: []pricingcatalog.ProductKey{{
					Operation:    domain.OperationImageGenerate,
					Modality:     domain.ModalityImage,
					ImageModelID: "broken_image",
					Quality:      pricingcatalog.ImageQuality1K,
				}},
			},
		},
	}
	if err := registry.Validate(); err == nil {
		t.Fatal("expected missing provider/model metadata to fail closed")
	}
}

func TestRegistryReadinessContainsOnlyEnvNamesNotValues(t *testing.T) {
	registry := providermodels.StaticRegistry()
	for _, readiness := range registry.ProviderReadiness() {
		for _, value := range append([]string{readiness.ProviderEnabledFlag}, readiness.RequiredConfigKeys...) {
			if value == "" {
				continue
			}
			if strings.Contains(value, "=") || strings.Contains(value, "://") || strings.Contains(strings.ToLower(value), "bearer ") {
				t.Fatalf("readiness value looks like a secret/config value, not an env name: %q", value)
			}
			if value != strings.ToUpper(value) {
				t.Fatalf("readiness value should be an env name: %q", value)
			}
		}
	}
}

func TestRegistryValidationReportsMissingPricingKeys(t *testing.T) {
	registry := providermodels.Registry{
		ImageModels: []providermodels.ImageModel{
			{
				PublicID:        "priced_without_key",
				Provider:        domain.ProviderAPIMart,
				ProviderModelID: "provider-model",
				FeatureFlag:     "FEATURE_PRICED_WITHOUT_KEY_ENABLED",
				Readiness: providermodels.ProviderReadiness{
					ProviderEnabledFlag: "APIMART_PROVIDER_ENABLED",
					RequiredConfigKeys:  []string{"APIMART_API_KEY", "APIMART_BASE_URL"},
				},
			},
		},
	}
	err := registry.Validate()
	if err == nil || !strings.Contains(fmt.Sprint(err), "pricing") {
		t.Fatalf("expected missing pricing keys error, got %v", err)
	}
}
