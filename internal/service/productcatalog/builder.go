package productcatalog

import (
	"errors"
	"fmt"
	"strings"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/videorouter"
)

// RuntimeCatalog is the single config-derived product catalog used by inbound
// surfaces. It performs no provider calls; readiness is based only on server
// config, provider kill switches, route/model flags and required key/base URL
// presence.
type RuntimeCatalog struct {
	Catalog               *Catalog
	VideoRouteCatalog     *videorouter.Catalog
	PricingCatalog        *pricingcatalog.Catalog
	ImageReferenceEnabled bool
}

func FromConfig(cfg config.Config, pricingCatalog *pricingcatalog.Catalog) (RuntimeCatalog, error) {
	if pricingCatalog == nil {
		return RuntimeCatalog{}, errors.New("productcatalog: pricing catalog is required")
	}
	if err := validateRegistryConfigMappings(providermodels.StaticRegistry()); err != nil {
		return RuntimeCatalog{}, err
	}
	videoCatalog, err := VideoRouteCatalogFromConfig(cfg)
	var publicVideoRoutes []videorouter.PublicRoute
	if err == nil && videoCatalog != nil {
		publicVideoRoutes = videoCatalog.PublicRoutes()
	}
	catalog := New(Config{
		ImageProviderReady: imageProviderReadyFromConfig(cfg),
		EnabledImageModels: enabledImageModelsFromConfig(cfg),
		VideoRoutes:        publicVideoRoutes,
		PricingCatalog:     pricingCatalog,
	})
	return RuntimeCatalog{
		Catalog:               catalog,
		VideoRouteCatalog:     videoCatalog,
		PricingCatalog:        pricingCatalog,
		ImageReferenceEnabled: catalogHasReferenceImageModel(catalog),
	}, err
}

func VideoRouteCatalogFromConfig(cfg config.Config) (*videorouter.Catalog, error) {
	registry := providermodels.StaticRegistry()
	if err := validateRegistryConfigMappings(registry); err != nil {
		return nil, err
	}
	providers := make(map[domain.ProviderName]videorouter.ProviderConfig)
	enabledRoutes := make(map[domain.VideoRouteAlias]bool)
	for _, route := range registry.VideoRoutes() {
		enabledRoutes[route.Alias] = featureFlagEnabled(cfg, route.FeatureFlag)
		if _, ok := providers[route.Provider]; ok {
			continue
		}
		providers[route.Provider] = videoProviderConfigFromRegistry(cfg, route)
	}
	return videorouter.NewCatalog(videorouter.Config{
		RouterEnabled: featureFlagEnabled(cfg, providermodels.FeatureVideoRouter),
		Providers:     providers,
		EnabledRoutes: enabledRoutes,
	})
}

func (r RuntimeCatalog) ImageModels() []ImageModel {
	if r.Catalog == nil {
		return nil
	}
	return r.Catalog.ImageModels()
}

func (r RuntimeCatalog) VideoRoutes() []VideoRoute {
	if r.Catalog == nil {
		return nil
	}
	return r.Catalog.VideoRoutes()
}

func imageProviderReadyFromConfig(cfg config.Config) map[domain.ProviderName]bool {
	registry := providermodels.StaticRegistry()
	ready := make(map[domain.ProviderName]bool)
	for _, model := range registry.PublicImageModels() {
		ready[model.Provider] = ready[model.Provider] || providerReadyFromReadiness(cfg, model.Readiness)
	}
	for _, model := range registry.LoadTestImageModels {
		if model.Provider == domain.ProviderMock {
			ready[model.Provider] = ready[model.Provider] || mockImageProviderReadyFromConfig(cfg)
			continue
		}
		ready[model.Provider] = ready[model.Provider] || providerReadyFromReadiness(cfg, model.Readiness)
	}
	return ready
}

func enabledImageModelsFromConfig(cfg config.Config) map[string]bool {
	registry := providermodels.StaticRegistry()
	enabled := make(map[string]bool)
	for _, model := range registry.PublicImageModels() {
		enabled[model.PublicID] = featureFlagEnabled(cfg, model.FeatureFlag)
	}
	for _, model := range registry.LoadTestImageModels {
		enabled[model.PublicID] = featureFlagEnabled(cfg, model.FeatureFlag)
	}
	return enabled
}

func mockVideoProviderReadyFromConfig(cfg config.Config) bool {
	if !cfg.IsLoadTest() || !strings.EqualFold(strings.TrimSpace(cfg.Provider), string(domain.ProviderMock)) {
		return false
	}
	if !optionalProviderIsMock(cfg.VideoProvider) {
		return false
	}
	for _, provider := range cfg.ProviderChain {
		if !optionalProviderIsMock(provider) {
			return false
		}
	}
	return true
}

func mockImageProviderReadyFromConfig(cfg config.Config) bool {
	if !cfg.IsLoadTest() || !strings.EqualFold(strings.TrimSpace(cfg.Provider), string(domain.ProviderMock)) {
		return false
	}
	if !optionalProviderIsMock(cfg.ImageProvider) {
		return false
	}
	for _, provider := range cfg.ProviderChain {
		if !optionalProviderIsMock(provider) {
			return false
		}
	}
	return true
}

func videoProviderConfigFromRegistry(cfg config.Config, route providermodels.VideoRoute) videorouter.ProviderConfig {
	if route.Readiness.LoadTestOnly && route.Provider == domain.ProviderMock {
		return videorouter.ProviderConfig{Enabled: mockVideoProviderReadyFromConfig(cfg)}
	}
	apiKeyConfigured := requiredConfigGroupConfigured(cfg, route.Readiness.RequiredConfigKeys, configKeyIsAPIKey)
	baseURLConfigured := requiredConfigGroupConfigured(cfg, route.Readiness.RequiredConfigKeys, configKeyIsBaseURL)
	return videorouter.ProviderConfig{
		Enabled:           providerEnabledFromFlag(cfg, route.Readiness.ProviderEnabledFlag),
		RequireAPIKey:     requiredConfigGroupPresent(route.Readiness.RequiredConfigKeys, configKeyIsAPIKey),
		APIKeyConfigured:  apiKeyConfigured,
		RequireBaseURL:    requiredConfigGroupPresent(route.Readiness.RequiredConfigKeys, configKeyIsBaseURL),
		BaseURLConfigured: baseURLConfigured,
	}
}

func providerReadyFromReadiness(cfg config.Config, readiness providermodels.ProviderReadiness) bool {
	if readiness.LoadTestOnly {
		return false
	}
	if !providerEnabledFromFlag(cfg, readiness.ProviderEnabledFlag) {
		return false
	}
	for _, key := range readiness.RequiredConfigKeys {
		if strings.TrimSpace(configValueByKey(cfg, key)) == "" {
			return false
		}
	}
	return true
}

func providerEnabledFromFlag(cfg config.Config, flag string) bool {
	enabled, ok := providerFlagValue(cfg, flag)
	return ok && enabled
}

func providerFlagValue(cfg config.Config, flag string) (bool, bool) {
	switch flag {
	case providermodels.ProviderFlagAPIMart:
		return cfg.APIMartProviderEnabled, true
	case providermodels.ProviderFlagPoYo:
		return cfg.PoYoProviderEnabled, true
	case providermodels.ProviderFlagRunway:
		return cfg.RunwayProviderEnabled, true
	default:
		return false, false
	}
}

func featureFlagEnabled(cfg config.Config, flag string) bool {
	enabled, ok := featureFlagValue(cfg, flag)
	return ok && enabled
}

func featureFlagValue(cfg config.Config, flag string) (bool, bool) {
	switch flag {
	case providermodels.FeatureImageNanoBanana2:
		return cfg.FeatureImageModelNanoBanana2Enabled, true
	case providermodels.FeatureImageNanoBananaPro:
		return cfg.FeatureImageModelNanoBananaProEnabled, true
	case providermodels.FeatureImageGPTImage2:
		return cfg.FeatureImageModelGPTImage2Enabled, true
	case providermodels.FeatureImageMock:
		return cfg.FeatureImageModelMockEnabled, true
	case providermodels.FeatureVideoRouter:
		return cfg.FeatureVideoRouterEnabled, true
	case providermodels.FeatureVideoHailuo23Fast:
		return cfg.FeatureVideoRouteHailuo23FastEnabled, true
	case providermodels.FeatureVideoHailuo23Standard:
		return cfg.FeatureVideoRouteHailuo23StandardEnabled, true
	case providermodels.FeatureVideoKlingO3Standard:
		return cfg.FeatureVideoRouteKlingO3StandardEnabled, true
	case providermodels.FeatureVideoRunwayGen4Turbo:
		return cfg.FeatureVideoRouteRunwayGen4TurboEnabled, true
	case providermodels.FeatureVideoSeedance20Fast:
		return cfg.FeatureVideoRouteSeedance20FastEnabled, true
	case providermodels.FeatureVideoRunwayGen45:
		return cfg.FeatureVideoRouteRunwayGen45Enabled, true
	case providermodels.FeatureVideoMockTextToVideo:
		return cfg.FeatureVideoRouteMockTextToVideoEnabled, true
	case providermodels.FeatureVideoResellerExperiment:
		return cfg.FeatureVideoRouteResellerExperimentsEnabled, true
	default:
		return false, false
	}
}

func requiredConfigGroupPresent(keys []string, match func(string) bool) bool {
	for _, key := range keys {
		if match(key) {
			return true
		}
	}
	return false
}

func requiredConfigGroupConfigured(cfg config.Config, keys []string, match func(string) bool) bool {
	hasMatch := false
	for _, key := range keys {
		if !match(key) {
			continue
		}
		hasMatch = true
		if strings.TrimSpace(configValueByKey(cfg, key)) == "" {
			return false
		}
	}
	return hasMatch
}

func configKeyIsAPIKey(key string) bool {
	return strings.HasSuffix(key, "_API_KEY") || strings.HasSuffix(key, "_API_SECRET")
}

func configKeyIsBaseURL(key string) bool {
	return strings.HasSuffix(key, "_BASE_URL")
}

func configValueByKey(cfg config.Config, key string) string {
	value, _ := configValue(cfg, key)
	return value
}

func configValue(cfg config.Config, key string) (string, bool) {
	switch key {
	case providermodels.ConfigKeyAPIMartAPIKey:
		return cfg.APIMartAPIKey, true
	case providermodels.ConfigKeyAPIMartBaseURL:
		return cfg.APIMartBaseURL, true
	case providermodels.ConfigKeyPoYoAPIKey:
		return cfg.PoYoAPIKey, true
	case providermodels.ConfigKeyPoYoBaseURL:
		return cfg.PoYoBaseURL, true
	case providermodels.ConfigKeyRunwaySecret:
		return cfg.RunwayMLAPISecret, true
	case providermodels.ConfigKeyRunwayBaseURL:
		return cfg.RunwayMLBaseURL, true
	case providermodels.ConfigKeyDeepInfraKey:
		return cfg.DeepInfraAPIKey, true
	case providermodels.ConfigKeyDeepInfraURL:
		return cfg.DeepInfraBaseURL, true
	default:
		return "", false
	}
}

func validateRegistryConfigMappings(registry providermodels.Registry) error {
	var missing []string
	requireFeatureFlag := func(scope, flag string) {
		flag = strings.TrimSpace(flag)
		if flag == "" {
			return
		}
		if _, ok := featureFlagValue(config.Config{}, flag); !ok {
			missing = append(missing, fmt.Sprintf("%s feature flag %q", scope, flag))
		}
	}
	requireReadiness := func(scope string, readiness providermodels.ProviderReadiness) {
		if readiness.LoadTestOnly {
			return
		}
		if flag := strings.TrimSpace(readiness.ProviderEnabledFlag); flag != "" {
			if _, ok := providerFlagValue(config.Config{}, flag); !ok {
				missing = append(missing, fmt.Sprintf("%s provider flag %q", scope, flag))
			}
		}
		for _, key := range readiness.RequiredConfigKeys {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			if _, ok := configValue(config.Config{}, key); !ok {
				missing = append(missing, fmt.Sprintf("%s config key %q", scope, key))
			}
		}
	}

	for _, alias := range registry.TextAliasModels() {
		requireFeatureFlag("text "+alias.PublicID, alias.FeatureFlag)
		requireReadiness("text "+alias.PublicID, alias.Readiness)
	}
	for _, model := range registry.PublicImageModels() {
		requireFeatureFlag("image "+model.PublicID, model.FeatureFlag)
		requireReadiness("image "+model.PublicID, model.Readiness)
	}
	for _, model := range registry.LoadTestImageModels {
		requireFeatureFlag("image "+model.PublicID, model.FeatureFlag)
		requireReadiness("image "+model.PublicID, model.Readiness)
	}
	for _, route := range registry.VideoRoutes() {
		requireFeatureFlag("video "+string(route.Alias), route.RouterFeatureFlag)
		requireFeatureFlag("video "+string(route.Alias), route.FeatureFlag)
		requireReadiness("video "+string(route.Alias), route.Readiness)
	}
	if len(missing) > 0 {
		return fmt.Errorf("productcatalog: provider registry config mapping missing: %s", strings.Join(missing, "; "))
	}
	return nil
}

func optionalProviderIsMock(provider string) bool {
	provider = strings.TrimSpace(provider)
	return provider == "" || strings.EqualFold(provider, string(domain.ProviderMock))
}

func catalogHasReferenceImageModel(catalog *Catalog) bool {
	if catalog == nil {
		return false
	}
	for _, model := range catalog.ImageModels() {
		if model.Enabled && model.SupportsReferenceImage {
			return true
		}
	}
	return false
}
