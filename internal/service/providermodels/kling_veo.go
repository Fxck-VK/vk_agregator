package providermodels

import (
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

const (
	FeatureVideoKlingV3       = "FEATURE_APIMART_KLING_V3_ENABLED"
	FeatureVideoKling26Motion = "FEATURE_APIMART_KLING_2_6_MOTION_CONTROL_ENABLED"
	FeatureVideoVeo31Fast     = "FEATURE_APIMART_VEO_3_1_FAST_ENABLED"
	FeatureVideoVeo31Quality  = "FEATURE_APIMART_VEO_3_1_QUALITY_ENABLED"
	FeatureVideoVeo31Lite     = "FEATURE_APIMART_VEO_3_1_LITE_ENABLED"
)

func IsKlingVeoVideoRoute(provider domain.ProviderName, model string) bool {
	if provider != domain.ProviderAPIMart {
		return false
	}
	switch model {
	case "kling-v3", "kling-v2-6-motion-control", "veo3.1-fast", "veo3.1-quality", "veo3.1-lite":
		return true
	}
	return false
}

func klingVeoRoutes() []VideoRoute {
	durations := []int{5}
	for n := 3; n <= 15; n++ {
		if n != 5 {
			durations = append(durations, n)
		}
	}
	kling := domain.VideoRouteSpec{Alias: domain.VideoRouteKlingV3, Provider: domain.ProviderAPIMart, ProviderModelID: "kling-v3", ModelClass: "kling_v3",
		InputModes: []domain.VideoInputMode{domain.VideoInputText, domain.VideoInputImage}, AllowedDurationsSec: durations, AllowedResolutions: []string{"720p", "1080p", "4k"}, AllowedAspectRatios: []string{"16:9", "9:16", "1:1"},
		SupportsReferenceImage: true, MaxReferenceImages: 2, SupportsAudio: true, ProviderCostMicrosPerSecondByResolution: pricingcatalog.KlingV3ProviderRates(false), ProviderCostMicrosPerSecondWithAudioByResolution: pricingcatalog.KlingV3ProviderRates(true), MaxProviderCostCredits: 65, MaxInternalCostCredits: 3900, PriceMultiplier: 60}
	keys := videoPricingKeys(kling.Alias, kling.AllowedResolutions, durations)
	for _, key := range append([]pricingcatalog.ProductKey(nil), keys...) {
		key.Quality = pricingcatalog.VideoQualityAudio
		keys = append(keys, key)
	}
	out := []VideoRoute{videoRoute(kling, FeatureVideoKlingV3, apimartReadiness(), keys, nil, false)}
	motion := domain.VideoRouteSpec{Alias: domain.VideoRouteKling26Motion, Provider: domain.ProviderAPIMart, ProviderModelID: "kling-v2-6-motion-control", ModelClass: "kling_2_6_motion_control",
		InputModes: []domain.VideoInputMode{domain.VideoInputReference}, AllowedResolutions: []string{"std", "pro"}, AllowedAspectRatios: []string{"16:9", "9:16", "1:1", "4:3", "3:4"}, SupportsReferenceImage: true, RequiresStartImage: true, MaxReferenceImages: 1, RequiresReferenceVideo: true, SupportsReferenceVideo: true,
		ProviderCostMicrosPerSecondByResolution: pricingcatalog.KlingMotionProviderRates(), MaxProviderCostCredits: 28, MaxInternalCostCredits: 1680, PriceMultiplier: 60}
	for n := 3; n <= 30; n++ {
		motion.AllowedDurationsSec = append(motion.AllowedDurationsSec, n)
	}
	motion.AllowedAspectRatios = nil // Provider derives framing from the uploaded image/video.
	out = append(out, videoRoute(motion, FeatureVideoKling26Motion, apimartReadiness(), videoPricingKeys(motion.Alias, motion.AllowedResolutions, motion.AllowedDurationsSec), nil, false))
	for _, entry := range []struct {
		alias       domain.VideoRouteAlias
		model, flag string
		refs        int
	}{
		{domain.VideoRouteVeo31Fast, "veo3.1-fast", FeatureVideoVeo31Fast, 3}, {domain.VideoRouteVeo31Quality, "veo3.1-quality", FeatureVideoVeo31Quality, 2}, {domain.VideoRouteVeo31Lite, "veo3.1-lite", FeatureVideoVeo31Lite, 0},
	} {
		spec := domain.VideoRouteSpec{Alias: entry.alias, Provider: domain.ProviderAPIMart, ProviderModelID: entry.model, ModelClass: string(entry.alias), InputModes: []domain.VideoInputMode{domain.VideoInputText}, AllowedDurationsSec: []int{8}, AllowedResolutions: []string{"720p", "1080p", "4k"}, AllowedAspectRatios: []string{"16:9", "9:16"}, SupportsReferenceImage: entry.refs > 0, MaxReferenceImages: entry.refs,
			ProviderCostMicrosByResolutionDuration: pricingcatalog.VeoProviderFloors(entry.alias), MaxProviderCostCredits: 15, MaxInternalCostCredits: 900, PriceMultiplier: 60}
		if entry.refs > 0 {
			spec.InputModes = append(spec.InputModes, domain.VideoInputImage)
		}
		out = append(out, videoRoute(spec, entry.flag, apimartReadiness(), videoPricingKeys(spec.Alias, spec.AllowedResolutions, spec.AllowedDurationsSec), nil, false))
	}
	return out
}
