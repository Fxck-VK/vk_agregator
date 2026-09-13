package providermodels

import (
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

const (
	ProviderModelOmni11Flash    = "gemini-omni-1.1-flash"
	ProviderModelOmni11FlashExt = "gemini-omni-1.1-flash-ext"
	FeatureVideoOmni11Flash     = "FEATURE_APIMART_OMNI_1_1_FLASH_ENABLED"
	FeatureVideoOmni11FlashExt  = "FEATURE_APIMART_OMNI_1_1_FLASH_EXT_ENABLED"
)

func IsOmniVideoRoute(provider domain.ProviderName, model string) bool {
	return provider == domain.ProviderAPIMart && (model == ProviderModelOmni11Flash || model == ProviderModelOmni11FlashExt)
}

func omniVideoRoute(ext bool) VideoRoute {
	spec := domain.VideoRouteSpec{
		Alias:                                  domain.VideoRouteOmni11Flash,
		Provider:                               domain.ProviderAPIMart,
		ProviderModelID:                        ProviderModelOmni11Flash,
		ModelClass:                             "gemini_omni_1_1_flash",
		InputModes:                             []domain.VideoInputMode{domain.VideoInputText, domain.VideoInputImage, domain.VideoInputReference},
		AutomaticDuration:                      true,
		AllowedDurationsSec:                    []int{10},
		AllowedResolutions:                     []string{"720p", "1080p", "360p", "4k"},
		AllowedAspectRatios:                    []string{"16:9", "9:16"},
		SupportsReferenceImage:                 true,
		MaxReferenceImages:                     10,
		ProviderCostMicrosByResolutionDuration: pricingcatalog.OmniVideoProviderFloors(ext),
		MaxProviderCostCredits:                 27,
		MaxInternalCostCredits:                 1620,
		PriceMultiplier:                        60,
	}
	flag := FeatureVideoOmni11Flash
	if ext {
		spec.Alias = domain.VideoRouteOmni11FlashExt
		spec.ProviderModelID = ProviderModelOmni11FlashExt
		spec.ModelClass = "gemini_omni_1_1_flash_ext"
		spec.AutomaticDuration = false
		spec.AllowedDurationsSec = []int{6, 4, 8, 10}
		spec.MaxReferenceImages = 3
		spec.AllowedReferenceImageCounts = []int{0, 1, 3}
		spec.MaxProviderCostCredits = 9
		spec.MaxInternalCostCredits = 540
		flag = FeatureVideoOmni11FlashExt
	}
	return videoRoute(spec, flag, apimartReadiness(), videoPricingKeys(spec.Alias, spec.AllowedResolutions, spec.AllowedDurationsSec), nil, false)
}
