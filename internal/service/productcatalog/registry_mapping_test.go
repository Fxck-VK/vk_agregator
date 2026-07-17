package productcatalog

import (
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestRegistryConfigMappingsCoverCurrentProviderModels(t *testing.T) {
	if err := validateRegistryConfigMappings(providermodels.StaticRegistry()); err != nil {
		t.Fatalf("current registry has unmapped config metadata: %v", err)
	}
}

func TestRegistryConfigMappingsCoverSeedream45FeatureFlag(t *testing.T) {
	registry := providermodels.Registry{
		ImageModels: []providermodels.ImageModel{
			{
				PublicID:        providermodels.PublicImageSeedream45,
				Provider:        domain.ProviderPoYo,
				ProviderModelID: providermodels.ProviderModelPoYoSeedream45,
				FeatureFlag:     providermodels.FeatureImageSeedream45,
				Readiness: providermodels.ProviderReadiness{
					ProviderEnabledFlag: providermodels.ProviderFlagPoYo,
					RequiredConfigKeys: []string{
						providermodels.ConfigKeyPoYoAPIKey,
						providermodels.ConfigKeyPoYoBaseURL,
					},
				},
			},
		},
	}

	if err := validateRegistryConfigMappings(registry); err != nil {
		t.Fatalf("Seedream 4.5 registry config mapping missing: %v", err)
	}
}

func TestRegistryConfigMappingsRejectUnknownFeatureFlagProviderFlagAndConfigKey(t *testing.T) {
	registry := providermodels.Registry{
		ImageModels: []providermodels.ImageModel{
			{
				PublicID:        "unknown_image",
				Provider:        domain.ProviderAPIMart,
				ProviderModelID: "provider-model",
				FeatureFlag:     "FEATURE_UNKNOWN_IMAGE_ENABLED",
				Readiness: providermodels.ProviderReadiness{
					ProviderEnabledFlag: "UNKNOWN_PROVIDER_ENABLED",
					RequiredConfigKeys:  []string{"UNKNOWN_API_KEY"},
				},
			},
		},
	}

	err := validateRegistryConfigMappings(registry)
	if err == nil {
		t.Fatal("expected unmapped registry config metadata to fail")
	}
	message := err.Error()
	for _, want := range []string{"FEATURE_UNKNOWN_IMAGE_ENABLED", "UNKNOWN_PROVIDER_ENABLED", "UNKNOWN_API_KEY"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error %q did not mention %q", message, want)
		}
	}
}
