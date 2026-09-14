// Package providermodels defines the central provider/model registry that will
// become the source of truth for public model ids, provider model ids, feature
// gates, readiness requirements, route limits and media policy metadata.
package providermodels

import (
	"fmt"
	"strings"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/modelcontract"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

const (
	ProviderFlagAPIMart  = "APIMART_PROVIDER_ENABLED"
	ProviderFlagPoYo     = "POYO_PROVIDER_ENABLED"
	ProviderFlagRunway   = "RUNWAY_PROVIDER_ENABLED"
	ProviderFlagLoadtest = "APP_ENV"

	ConfigKeyAPIMartAPIKey  = "APIMART_API_KEY" // #nosec G101 -- environment variable name only, never a credential value.
	ConfigKeyAPIMartBaseURL = "APIMART_BASE_URL"
	ConfigKeyPoYoAPIKey     = "POYO_API_KEY" // #nosec G101 -- environment variable name only, never a credential value.
	ConfigKeyPoYoBaseURL    = "POYO_BASE_URL"
	ConfigKeyRunwaySecret   = "RUNWAYML_API_SECRET" // #nosec G101 -- environment variable name only, never a credential value.
	ConfigKeyRunwayBaseURL  = "RUNWAYML_BASE_URL"
	ConfigKeyDeepInfraKey   = "DEEPINFRA_API_KEY"
	ConfigKeyDeepInfraURL   = "DEEPINFRA_BASE_URL"
)

// ProviderReadiness is static config metadata only. It stores env/config names,
// never values, and performs no provider calls.
type ProviderReadiness struct {
	ProviderEnabledFlag string
	RequiredConfigKeys  []string
	LoadTestOnly        bool
}

// Registry is the static provider/model registry.
type Registry struct {
	ModelAliases        []ProviderModelAlias
	Contracts           map[string]modelcontract.Contract
	onboardingError     error
	TextAliases         []TextAlias
	ImageModels         []ImageModel
	LoadTestImageModels []ImageModel
	AudioModels         []AudioModel
	VideoRouteModels    []VideoRoute
}

// StaticRegistry returns the current static provider/model registry.
func StaticRegistry() Registry {
	contracts, err := loadOnboardingContracts()
	return Registry{
		ModelAliases:        providerModelAliases(),
		Contracts:           contracts,
		onboardingError:     err,
		TextAliases:         textAliases(),
		ImageModels:         imageModels(),
		LoadTestImageModels: loadTestImageModels(),
		AudioModels:         audioModels(),
		VideoRouteModels:    videoRoutes(),
	}
}

func apimartReadiness() ProviderReadiness {
	return ProviderReadiness{
		ProviderEnabledFlag: ProviderFlagAPIMart,
		RequiredConfigKeys:  []string{ConfigKeyAPIMartAPIKey, ConfigKeyAPIMartBaseURL},
	}
}

func poyoReadiness() ProviderReadiness {
	return ProviderReadiness{
		ProviderEnabledFlag: ProviderFlagPoYo,
		RequiredConfigKeys:  []string{ConfigKeyPoYoAPIKey, ConfigKeyPoYoBaseURL},
	}
}

func runwayReadiness() ProviderReadiness {
	return ProviderReadiness{
		ProviderEnabledFlag: ProviderFlagRunway,
		RequiredConfigKeys:  []string{ConfigKeyRunwaySecret, ConfigKeyRunwayBaseURL},
	}
}

func mockReadiness() ProviderReadiness {
	return ProviderReadiness{
		ProviderEnabledFlag: ProviderFlagLoadtest,
		RequiredConfigKeys:  []string{"PROVIDER", "PROVIDER_CHAIN", "IMAGE_PROVIDER", "VIDEO_PROVIDER"},
		LoadTestOnly:        true,
	}
}

func (r Registry) ProviderReadiness() []ProviderReadiness {
	seen := map[string]struct{}{}
	var out []ProviderReadiness
	add := func(readiness ProviderReadiness) {
		key := readiness.ProviderEnabledFlag + "\x00" + strings.Join(readiness.RequiredConfigKeys, "\x00")
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, copyReadiness(readiness))
	}
	for _, alias := range r.TextAliases {
		add(alias.Readiness)
	}
	for _, model := range r.ImageModels {
		add(model.Readiness)
	}
	for _, model := range r.LoadTestImageModels {
		add(model.Readiness)
	}
	for _, model := range r.AudioModels {
		add(model.Readiness)
	}
	for _, route := range r.VideoRouteModels {
		add(route.Readiness)
	}
	return out
}

func (r Registry) Validate() error {
	seenImages := map[string]struct{}{}
	for _, model := range r.ImageModels {
		if err := validateImageModel(model, false); err != nil {
			return err
		}
		if _, exists := seenImages[model.PublicID]; exists {
			return fmt.Errorf("providermodels: duplicate image model %s", model.PublicID)
		}
		seenImages[model.PublicID] = struct{}{}
	}
	for _, model := range r.LoadTestImageModels {
		if err := validateImageModel(model, true); err != nil {
			return err
		}
		if _, exists := seenImages[model.PublicID]; exists {
			return fmt.Errorf("providermodels: duplicate image model %s", model.PublicID)
		}
		seenImages[model.PublicID] = struct{}{}
	}
	seenText := map[string]struct{}{}
	for _, alias := range r.TextAliases {
		if strings.TrimSpace(alias.PublicID) == "" || alias.Provider == "" || strings.TrimSpace(alias.ProviderModelID) == "" {
			return fmt.Errorf("providermodels: text alias %s missing provider metadata", alias.PublicID)
		}
		if _, exists := seenText[alias.PublicID]; exists {
			return fmt.Errorf("providermodels: duplicate text alias %s", alias.PublicID)
		}
		seenText[alias.PublicID] = struct{}{}
	}
	seenAudio := map[string]struct{}{}
	for _, model := range r.AudioModels {
		if err := validateAudioModel(model, false); err != nil {
			return err
		}
		if _, exists := seenAudio[model.PublicID]; exists {
			return fmt.Errorf("providermodels: duplicate audio model %s", model.PublicID)
		}
		seenAudio[model.PublicID] = struct{}{}
	}
	seenRoutes := map[domain.VideoRouteAlias]struct{}{}
	for _, route := range r.VideoRouteModels {
		if err := validateVideoRoute(route); err != nil {
			return err
		}
		if _, exists := seenRoutes[route.Alias]; exists {
			return fmt.Errorf("providermodels: duplicate video route %s", route.Alias)
		}
		seenRoutes[route.Alias] = struct{}{}
	}
	return r.ValidateOnboarding()
}

func (r Registry) ValidatePricingCoverage(catalog *pricingcatalog.Catalog, disabled []pricingcatalog.DisabledProductPrice) error {
	if catalog == nil {
		return fmt.Errorf("providermodels: pricing catalog is required")
	}
	disabledKeys := map[pricingcatalog.ProductKey]struct{}{}
	for _, price := range disabled {
		disabledKeys[price.Key.Normalize()] = struct{}{}
	}
	for _, model := range r.ImageModels {
		for _, key := range model.PricingKeys {
			if _, err := catalog.Lookup(key); err != nil {
				return fmt.Errorf("providermodels: image %s pricing key missing: %w", model.PublicID, err)
			}
		}
	}
	for _, model := range r.AudioModels {
		for _, key := range model.PricingKeys {
			if _, err := catalog.Lookup(key); err != nil {
				return fmt.Errorf("providermodels: audio %s pricing key missing: %w", model.PublicID, err)
			}
		}
	}
	for _, route := range r.VideoRouteModels {
		for _, key := range route.PricingKeys {
			if _, err := catalog.Lookup(key); err != nil {
				return fmt.Errorf("providermodels: route %s pricing key missing: %w", route.Alias, err)
			}
		}
		for _, key := range route.DisabledPricingKeys {
			if _, ok := disabledKeys[key.Normalize()]; !ok {
				return fmt.Errorf("providermodels: route %s disabled pricing key missing: %+v", route.Alias, key)
			}
		}
	}
	return nil
}

func copyReadiness(readiness ProviderReadiness) ProviderReadiness {
	readiness.RequiredConfigKeys = append([]string(nil), readiness.RequiredConfigKeys...)
	return readiness
}
