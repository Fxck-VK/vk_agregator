package providermodels

import (
	"fmt"
	"strings"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

const (
	PublicImageNanoBanana2   = "nano_banana_2"
	PublicImageNanoBananaPro = "nano_banana_pro"
	PublicImageGPTImage2     = "gpt_image_2"
	PublicImageQwenImage3    = "qwen_image_3"
	PublicImageGrokImage15   = "grok_image_1_5"
	PublicImageGrokImage20   = "grok_image_2_0"
	PublicImageMidjourneyV7  = "midjourney_v7"
	PublicImageFlux2Pro      = "flux_2_pro"
	PublicImageSeedream45    = "seedream_4_5"
	LoadTestImageMock        = "mock_image"

	ProviderModelPoYoNanoBanana2    = "nano-banana-2-new"
	ProviderModelPoYoNanoBananaPro  = "nano-banana-pro"
	ProviderModelPoYoSeedream45     = "seedream-4.5"
	ProviderModelPoYoSeedream45Edit = "seedream-4.5-edit"
	ProviderModelGemini3ProImage    = "gemini-3-pro-image-preview"
	ProviderModelGPTImage2          = "gpt-image-2"
	ProviderModelQwenImage3         = "qwen-image-3.0"
	ProviderModelGrokImage15        = "grok-imagine-1.5-apimart"
	ProviderModelGrokImage20        = "grok-imagine-2.0-ext"
	ProviderModelMidjourneyV7       = "midjourney"
	ProviderModelFlux2Pro           = "flux-2-pro"
	ProviderModelMockImage          = "mock-image"

	FeatureImageNanoBanana2   = "FEATURE_IMAGE_MODEL_NANO_BANANA_2_ENABLED"
	FeatureImageNanoBananaPro = "FEATURE_IMAGE_MODEL_NANO_BANANA_PRO_ENABLED"
	FeatureImageGPTImage2     = "FEATURE_IMAGE_MODEL_GPT_IMAGE_2_ENABLED"
	FeatureImageQwenImage3    = "FEATURE_APIMART_QWEN_IMAGE_3_ENABLED"
	FeatureImageGrokImage15   = "FEATURE_APIMART_GROK_IMAGE_1_5_ENABLED"
	FeatureImageGrokImage20   = "FEATURE_APIMART_GROK_IMAGE_2_0_ENABLED"
	FeatureImageMidjourneyV7  = "FEATURE_APIMART_MIDJOURNEY_V7_ENABLED"
	FeatureImageFlux2Pro      = "FEATURE_APIMART_FLUX_2_PRO_ENABLED"
	FeatureImageSeedream45    = "FEATURE_IMAGE_MODEL_SEEDREAM_4_5_ENABLED"
	FeatureImageMock          = "FEATURE_IMAGE_MODEL_MOCK_ENABLED"
)

var defaultImageAspectRatios = []string{"16:9", "1:1", "21:9", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16"}

// ImageLimits describes public image request/media bounds for one model.
type ImageLimits struct {
	AllowedQualities       []string
	AllowedAspectRatios    []string
	SupportsReferenceImage bool
	MaxReferenceImages     int
	MaxOutputCount         int
}

// ImageModel maps one priced public image model id to its provider metadata.
type ImageModel struct {
	PublicID        string
	DisplayName     string
	Provider        domain.ProviderName
	ProviderModelID string
	FeatureFlag     string
	Readiness       ProviderReadiness
	Limits          ImageLimits
	PricingKeys     []pricingcatalog.ProductKey
	LoadTestOnly    bool
}

func imageModels() []ImageModel {
	return []ImageModel{
		gptImage25Model(false),
		gptImage25Model(true),
		seedream50LiteModel(),
		seedream50ProModel(),
		midjourneyV7Model(),
		flux2ProModel(),
		imageModel(PublicImageNanoBanana2, "Nano Banana 2", domain.ProviderPoYo, ProviderModelPoYoNanoBanana2, FeatureImageNanoBanana2, poyoReadiness(), 14),
		imageModel(PublicImageNanoBananaPro, "Nano Banana Pro", domain.ProviderAPIMart, ProviderModelGemini3ProImage, FeatureImageNanoBananaPro, apimartReadiness(), 14),
		imageModel(PublicImageGPTImage2, "GPT Image 2", domain.ProviderAPIMart, ProviderModelGPTImage2, FeatureImageGPTImage2, apimartReadiness(), 16),
		qwenImage3Model(),
		grokImageModel(PublicImageGrokImage15, "Grok Imagine 1.5", ProviderModelGrokImage15, FeatureImageGrokImage15, 1),
		grokImageModel(PublicImageGrokImage20, "Grok Imagine 2.0", ProviderModelGrokImage20, FeatureImageGrokImage20, 0),
		seedream45Model(),
	}
}

func loadTestImageModels() []ImageModel {
	return []ImageModel{
		{
			PublicID:        LoadTestImageMock,
			DisplayName:     "Mock Image Loadtest",
			Provider:        domain.ProviderMock,
			ProviderModelID: ProviderModelMockImage,
			FeatureFlag:     FeatureImageMock,
			Readiness:       mockReadiness(),
			Limits: ImageLimits{
				MaxOutputCount: 4,
			},
			LoadTestOnly: true,
		},
	}
}

func qwenImage3Model() ImageModel {
	model := imageModelWithQualities(PublicImageQwenImage3, "Qwen Image 3.0", domain.ProviderAPIMart, ProviderModelQwenImage3, FeatureImageQwenImage3, apimartReadiness(), []string{
		pricingcatalog.ImageQuality1K,
		pricingcatalog.ImageQuality2K,
	}, 3)
	model.Limits.MaxOutputCount = 1
	return model
}

func grokImageModel(publicID, name, providerModelID, flag string, maxRefs int) ImageModel {
	model := imageModelWithQualities(publicID, name, domain.ProviderAPIMart, providerModelID, flag, apimartReadiness(), []string{pricingcatalog.ImageQualityStandard}, maxRefs)
	model.Limits.MaxOutputCount = 1
	model.Limits.SupportsReferenceImage = maxRefs > 0
	model.Limits.AllowedAspectRatios = []string{"16:9", "1:1", "2:3", "3:2", "9:16"}
	if publicID == PublicImageGrokImage20 {
		model.Limits.AllowedAspectRatios = []string{"16:9", "1:1", "2:3", "3:2", "3:4", "4:3", "9:16"}
	}
	return model
}

func imageModel(publicID, displayName string, provider domain.ProviderName, providerModelID, featureFlag string, readiness ProviderReadiness, maxRefs int) ImageModel {
	qualities := []string{pricingcatalog.ImageQuality1K, pricingcatalog.ImageQuality2K, pricingcatalog.ImageQuality4K}
	return imageModelWithQualities(publicID, displayName, provider, providerModelID, featureFlag, readiness, qualities, maxRefs)
}

func imageModelWithQualities(publicID, displayName string, provider domain.ProviderName, providerModelID, featureFlag string, readiness ProviderReadiness, qualities []string, maxRefs int) ImageModel {
	keys := make([]pricingcatalog.ProductKey, 0, len(qualities))
	for _, quality := range qualities {
		keys = append(keys, pricingcatalog.ProductKey{
			Operation:    domain.OperationImageGenerate,
			Modality:     domain.ModalityImage,
			ImageModelID: publicID,
			Quality:      quality,
		})
	}
	return ImageModel{
		PublicID:        publicID,
		DisplayName:     displayName,
		Provider:        provider,
		ProviderModelID: providerModelID,
		FeatureFlag:     featureFlag,
		Readiness:       readiness,
		Limits: ImageLimits{
			AllowedQualities:       append([]string(nil), qualities...),
			SupportsReferenceImage: true,
			MaxReferenceImages:     maxRefs,
			MaxOutputCount:         4,
		},
		PricingKeys: keys,
	}
}

func seedream45Model() ImageModel {
	model := imageModelWithQualities(PublicImageSeedream45, "Seedream 4.5", domain.ProviderPoYo, ProviderModelPoYoSeedream45, FeatureImageSeedream45, poyoReadiness(), []string{
		pricingcatalog.ImageQuality2K,
		pricingcatalog.ImageQuality4K,
	}, 10)
	return model
}

func midjourneyV7Model() ImageModel {
	model := imageModelWithQualities(PublicImageMidjourneyV7, "Midjourney V7", domain.ProviderAPIMart, ProviderModelMidjourneyV7, FeatureImageMidjourneyV7, apimartReadiness(), []string{"relax", "fast", "turbo"}, 4)
	model.Limits.MaxOutputCount = 1
	return model
}

func flux2ProModel() ImageModel {
	model := imageModelWithQualities(PublicImageFlux2Pro, "FLUX.2 Pro", domain.ProviderAPIMart, ProviderModelFlux2Pro, FeatureImageFlux2Pro, apimartReadiness(), []string{"1MP", "2MP", "3MP", "4MP"}, 0)
	model.Limits.MaxOutputCount = 1
	model.Limits.SupportsReferenceImage = false
	model.Limits.AllowedAspectRatios = []string{"16:9", "1:1", "4:3", "3:4", "9:16", "3:2", "2:3", "21:9", "9:21"}
	return model
}

// PublicImageModels returns priced public image models. Loadtest-only models are
// exposed separately through LoadTestImageModels to avoid accidental public sale.
func (r Registry) PublicImageModels() []ImageModel {
	return copyImageModels(r.ImageModels)
}

func (r Registry) PublicImageModel(publicID string) (ImageModel, bool) {
	for _, model := range r.ImageModels {
		if model.PublicID == publicID {
			return copyImageModel(model), true
		}
	}
	return ImageModel{}, false
}

func (r Registry) LoadTestImageModel(publicID string) (ImageModel, bool) {
	for _, model := range r.LoadTestImageModels {
		if model.PublicID == publicID {
			return copyImageModel(model), true
		}
	}
	return ImageModel{}, false
}

func validateImageModel(model ImageModel, loadTestOnly bool) error {
	if strings.TrimSpace(model.PublicID) == "" {
		return fmt.Errorf("providermodels: image public id is required")
	}
	if model.Provider == "" || strings.TrimSpace(model.ProviderModelID) == "" {
		return fmt.Errorf("providermodels: image %s missing provider metadata", model.PublicID)
	}
	if strings.TrimSpace(model.FeatureFlag) == "" {
		return fmt.Errorf("providermodels: image %s missing feature flag", model.PublicID)
	}
	if !loadTestOnly && len(model.PricingKeys) == 0 {
		return fmt.Errorf("providermodels: image %s missing pricing keys", model.PublicID)
	}
	if !loadTestOnly && len(model.Limits.AllowedQualities) == 0 {
		return fmt.Errorf("providermodels: image %s missing quality limits", model.PublicID)
	}
	if !loadTestOnly && model.Limits.SupportsReferenceImage && model.Limits.MaxReferenceImages <= 0 {
		return fmt.Errorf("providermodels: image %s missing reference limits", model.PublicID)
	}
	if !loadTestOnly && model.Limits.MaxOutputCount <= 0 {
		return fmt.Errorf("providermodels: image %s missing output count limit", model.PublicID)
	}
	if !loadTestOnly && len(copyImageModel(model).Limits.AllowedAspectRatios) == 0 {
		return fmt.Errorf("providermodels: image %s missing aspect ratio limits", model.PublicID)
	}
	return nil
}

func copyImageModels(in []ImageModel) []ImageModel {
	out := make([]ImageModel, 0, len(in))
	for _, model := range in {
		out = append(out, copyImageModel(model))
	}
	return out
}

func copyImageModel(model ImageModel) ImageModel {
	model.Readiness = copyReadiness(model.Readiness)
	model.Limits = copyImageLimits(model.Limits)
	if len(model.Limits.AllowedAspectRatios) == 0 && !model.LoadTestOnly {
		// Legacy declarations omitted ratios. Resolve the existing adapter bounds
		// at the public boundary; keep the admitted declaration byte-stable.
		switch model.PublicID {
		case PublicImageQwenImage3:
			model.Limits.AllowedAspectRatios = []string{"1:1", "4:3", "3:4", "16:9", "9:16", "3:2", "2:3"}
		case PublicImageSeedream45:
			model.Limits.AllowedAspectRatios = []string{"1:1", "4:3", "3:4", "16:9", "9:16", "3:2", "2:3", "21:9"}
		case PublicImageNanoBanana2, PublicImageNanoBananaPro, PublicImageGPTImage2, PublicImageMidjourneyV7:
			model.Limits.AllowedAspectRatios = append([]string(nil), defaultImageAspectRatios...)
		}
	}
	model.PricingKeys = append([]pricingcatalog.ProductKey(nil), model.PricingKeys...)
	return model
}

func copyImageLimits(limits ImageLimits) ImageLimits {
	limits.AllowedQualities = append([]string(nil), limits.AllowedQualities...)
	limits.AllowedAspectRatios = append([]string(nil), limits.AllowedAspectRatios...)
	return limits
}
