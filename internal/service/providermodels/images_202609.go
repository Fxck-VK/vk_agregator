package providermodels

import "vk-ai-aggregator/internal/domain"

const (
	PublicImageGPTImage25Flare      = "gpt_image_2_5_flare"
	PublicImageGPTImage25Sunburst   = "gpt_image_2_5_sunburst"
	PublicImageSeedream50Lite       = "seedream_5_0_lite"
	PublicImageSeedream50Pro        = "seedream_5_0_pro"
	ProviderModelGPTImage25Flare    = "gpt-image-2.5-flare"
	ProviderModelGPTImage25Sunburst = "gpt-image-2.5-sunburst"
	ProviderModelSeedream50Lite     = "seedream-5-0-lite"
	ProviderModelSeedream50Pro      = "seedream-5-0-pro"
	FeatureImageGPTImage25Flare     = "FEATURE_APIMART_GPT_IMAGE_2_5_FLARE_ENABLED"
	FeatureImageGPTImage25Sunburst  = "FEATURE_APIMART_GPT_IMAGE_2_5_SUNBURST_ENABLED"
	FeatureImageSeedream50Lite      = "FEATURE_APIMART_SEEDREAM_5_0_LITE_ENABLED"
	FeatureImageSeedream50Pro       = "FEATURE_APIMART_SEEDREAM_5_0_PRO_ENABLED"
)

func GPTImage25Qualities() []string {
	var qualities []string
	for _, resolution := range []string{"1K", "2K", "4K"} {
		for _, quality := range []string{"medium", "low", "high", "xhigh", "max"} {
			qualities = append(qualities, resolution+"-"+quality)
		}
	}
	return qualities
}

func gptImage25Model(sunburst bool) ImageModel {
	id, name, code, flag := PublicImageGPTImage25Flare, "GPT Image 2.5 Flare", ProviderModelGPTImage25Flare, FeatureImageGPTImage25Flare
	if sunburst {
		id, name, code, flag = PublicImageGPTImage25Sunburst, "GPT Image 2.5 Sunburst", ProviderModelGPTImage25Sunburst, FeatureImageGPTImage25Sunburst
	}
	model := imageModelWithQualities(id, name, domain.ProviderAPIMart, code, flag, apimartReadiness(), GPTImage25Qualities(), 0)
	model.Limits.SupportsReferenceImage = false
	model.Limits.AllowedAspectRatios = []string{"1:1", "3:2", "2:3", "4:3", "3:4", "5:4", "4:5", "16:9", "9:16", "2:1", "1:2", "21:9", "9:21", "3:1", "1:3"}
	return model
}

func seedream50LiteModel() ImageModel {
	model := imageModelWithQualities(PublicImageSeedream50Lite, "Seedream 5.0 Lite", domain.ProviderAPIMart, ProviderModelSeedream50Lite, FeatureImageSeedream50Lite, apimartReadiness(), []string{"2K", "3K", "4K"}, 14)
	model.Limits.MaxOutputCount = 15
	model.Limits.AllowedAspectRatios = []string{"1:1", "4:3", "3:4", "16:9", "9:16", "3:2", "2:3", "21:9"}
	return model
}

func seedream50ProModel() ImageModel {
	model := imageModelWithQualities(PublicImageSeedream50Pro, "Seedream 5.0 Pro", domain.ProviderAPIMart, ProviderModelSeedream50Pro, FeatureImageSeedream50Pro, apimartReadiness(), []string{"1.5K", "1K", "2K"}, 10)
	model.Limits.MaxOutputCount = 1
	model.Limits.AllowedAspectRatios = []string{"1:1", "4:3", "3:4", "16:9", "9:16", "3:2", "2:3", "2:1", "1:2", "21:9"}
	return model
}

func IsGPTImage25Route(provider domain.ProviderName, model string) bool {
	return provider == domain.ProviderAPIMart && (model == ProviderModelGPTImage25Flare || model == ProviderModelGPTImage25Sunburst)
}

func IsNewAPIMartImageRoute(provider domain.ProviderName, model string) bool {
	return provider == domain.ProviderAPIMart && (IsGPTImage25Route(provider, model) || model == ProviderModelSeedream50Lite || model == ProviderModelSeedream50Pro)
}
