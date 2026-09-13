package providermodels

import (
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

const (
	ProviderModelKling30Turbo = "kling-3.0-turbo"
	ProviderModelMiniMaxH3    = "MiniMax-H3"
	FeatureVideoKling30Turbo  = "FEATURE_APIMART_KLING_3_0_TURBO_ENABLED"
	FeatureVideoMiniMaxH3     = "FEATURE_APIMART_MINIMAX_H3_ENABLED"
)

func IsTurboH3VideoRoute(provider domain.ProviderName, model string) bool {
	return provider == domain.ProviderAPIMart && (model == ProviderModelKling30Turbo || model == ProviderModelMiniMaxH3)
}

func turboH3VideoRoute(h3 bool) VideoRoute {
	spec := domain.VideoRouteSpec{
		Alias: domain.VideoRouteKling30Turbo, Provider: domain.ProviderAPIMart,
		ProviderModelID: ProviderModelKling30Turbo, ModelClass: "kling_3_0_turbo",
		InputModes:          []domain.VideoInputMode{domain.VideoInputText, domain.VideoInputImage},
		AllowedDurationsSec: []int{5}, AllowedResolutions: []string{"720p", "1080p"},
		AllowedAspectRatios:    []string{"16:9", "9:16", "1:1"},
		SupportsReferenceImage: true, MaxReferenceImages: 1,
		ProviderCostMicrosPerSecondByResolution: pricingcatalog.Kling30TurboProviderRates(),
		MaxProviderCostCredits:                  22, MaxInternalCostCredits: 1320, PriceMultiplier: 60,
	}
	flag, minimum := FeatureVideoKling30Turbo, 3
	if h3 {
		spec.Alias, spec.ProviderModelID, spec.ModelClass = domain.VideoRouteMiniMaxH3, ProviderModelMiniMaxH3, "minimax_h3"
		spec.AllowedResolutions = []string{"2k", "768p"}
		spec.AllowedAspectRatios = []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}
		spec.ProviderCostMicrosPerSecondByResolution = pricingcatalog.MiniMaxH3ProviderRates()
		spec.MaxProviderCostCredits, spec.MaxInternalCostCredits = 14, 840
		flag, minimum = FeatureVideoMiniMaxH3, 4
	}
	for n := minimum; n <= 15; n++ {
		if n != 5 {
			spec.AllowedDurationsSec = append(spec.AllowedDurationsSec, n)
		}
	}
	return videoRoute(spec, flag, apimartReadiness(), videoPricingKeys(spec.Alias, spec.AllowedResolutions, spec.AllowedDurationsSec), nil, false)
}
