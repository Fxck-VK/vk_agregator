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
