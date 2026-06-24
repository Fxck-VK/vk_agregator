package productcatalog

import (
	"errors"
	"strings"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/modelcatalog"
	"vk-ai-aggregator/internal/service/pricingcatalog"
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
	return videorouter.NewCatalog(videorouter.Config{
		RouterEnabled: cfg.FeatureVideoRouterEnabled,
		Providers: map[domain.ProviderName]videorouter.ProviderConfig{
			domain.ProviderAPIMart: {
				Enabled:           cfg.APIMartProviderEnabled,
				RequireAPIKey:     true,
				APIKeyConfigured:  strings.TrimSpace(cfg.APIMartAPIKey) != "",
				RequireBaseURL:    true,
				BaseURLConfigured: strings.TrimSpace(cfg.APIMartBaseURL) != "",
			},
			domain.ProviderPoYo: {
				Enabled:           cfg.PoYoProviderEnabled,
				RequireAPIKey:     true,
				APIKeyConfigured:  strings.TrimSpace(cfg.PoYoAPIKey) != "",
				RequireBaseURL:    true,
				BaseURLConfigured: strings.TrimSpace(cfg.PoYoBaseURL) != "",
			},
			domain.ProviderRunway: {
				Enabled:           cfg.RunwayProviderEnabled,
				RequireAPIKey:     true,
				APIKeyConfigured:  strings.TrimSpace(cfg.RunwayMLAPISecret) != "",
				RequireBaseURL:    true,
				BaseURLConfigured: strings.TrimSpace(cfg.RunwayMLBaseURL) != "",
			},
			domain.ProviderMock: {
				Enabled: mockVideoProviderReadyFromConfig(cfg),
			},
		},
		EnabledRoutes: map[domain.VideoRouteAlias]bool{
			domain.VideoRouteHailuo23Fast:     cfg.FeatureVideoRouteHailuo23FastEnabled,
			domain.VideoRouteHailuo23Standard: cfg.FeatureVideoRouteHailuo23StandardEnabled,
			domain.VideoRouteKlingO3Standard:  cfg.FeatureVideoRouteKlingO3StandardEnabled,
			domain.VideoRouteRunwayGen4Turbo:  cfg.FeatureVideoRouteRunwayGen4TurboEnabled,
			domain.VideoRouteSeedance20Fast:   cfg.FeatureVideoRouteSeedance20FastEnabled,
			domain.VideoRouteRunwayGen45:      cfg.FeatureVideoRouteRunwayGen45Enabled,
			domain.VideoRouteMockTextToVideo:  cfg.FeatureVideoRouteMockTextToVideoEnabled,
		},
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
	return map[domain.ProviderName]bool{
		domain.ProviderAPIMart:   apimartReadyFromConfig(cfg),
		domain.ProviderPoYo:      poyoReadyFromConfig(cfg),
		domain.ProviderDeepInfra: deepInfraReadyFromConfig(cfg),
		domain.ProviderMock:      mockImageProviderReadyFromConfig(cfg),
	}
}

func enabledImageModelsFromConfig(cfg config.Config) map[string]bool {
	return map[string]bool{
		modelcatalog.MiniAppImageNanoBanana2:   cfg.FeatureImageModelNanoBanana2Enabled,
		modelcatalog.MiniAppImageNanoBananaPro: cfg.FeatureImageModelNanoBananaProEnabled,
		modelcatalog.MiniAppImageGPTImage2:     cfg.FeatureImageModelGPTImage2Enabled,
		modelcatalog.MiniAppImageSeedream45:    true,
		modelcatalog.MiniAppImageSDXLTurbo:     true,
		modelcatalog.MiniAppImageMock:          cfg.FeatureImageModelMockEnabled,
	}
}

func apimartReadyFromConfig(cfg config.Config) bool {
	return cfg.APIMartProviderEnabled &&
		strings.TrimSpace(cfg.APIMartAPIKey) != "" &&
		strings.TrimSpace(cfg.APIMartBaseURL) != ""
}

func poyoReadyFromConfig(cfg config.Config) bool {
	return cfg.PoYoProviderEnabled &&
		strings.TrimSpace(cfg.PoYoAPIKey) != "" &&
		strings.TrimSpace(cfg.PoYoBaseURL) != ""
}

func deepInfraReadyFromConfig(cfg config.Config) bool {
	return strings.TrimSpace(cfg.DeepInfraAPIKey) != "" &&
		strings.TrimSpace(cfg.DeepInfraBaseURL) != ""
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
