// Package modelcatalog owns server-side public model choices for inbound
// surfaces. Clients may send public IDs only; provider/model codes come from
// this catalog.
package modelcatalog

import (
	"strings"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/providermodels"
)

const (
	PublicChatModelID   = providermodels.PublicTextChatGPT
	PublicChatModelName = "NeiroHub Chat"

	PublicImageNanoBananaPro      = providermodels.PublicImageNanoBananaPro
	PublicImageGPTImage2          = providermodels.PublicImageGPTImage2
	PublicImageGPTImage25Flare    = providermodels.PublicImageGPTImage25Flare
	PublicImageGPTImage25Sunburst = providermodels.PublicImageGPTImage25Sunburst
	PublicImageSeedream50Lite     = providermodels.PublicImageSeedream50Lite
	PublicImageSeedream50Pro      = providermodels.PublicImageSeedream50Pro
	PublicImageQwenImage3         = providermodels.PublicImageQwenImage3
	PublicImageMidjourneyV7       = providermodels.PublicImageMidjourneyV7
	PublicImageFlux2Pro           = providermodels.PublicImageFlux2Pro
	PublicImageGrokImage15        = providermodels.PublicImageGrokImage15
	PublicImageGrokImage20        = providermodels.PublicImageGrokImage20
	PublicImageNanoBananaFlash    = "nano_banana_flash"
	PublicImageNanoBanana2        = providermodels.PublicImageNanoBanana2
	PublicImageSeedream45         = providermodels.PublicImageSeedream45
	PublicImageSDXLTurbo          = "sdxl_turbo"
	PublicImageMock               = providermodels.LoadTestImageMock
	PublicVideoKling              = "kling"

	VKVideoPrunaAI = "prunaai"

	ModelCodePoYoNanoBanana2 = providermodels.ProviderModelPoYoNanoBanana2
	ModelCodeGemini3ProImage = providermodels.ProviderModelGemini3ProImage
	ModelCodeGPTImage2       = providermodels.ProviderModelGPTImage2
	ModelCodeQwenImage3      = providermodels.ProviderModelQwenImage3
	ModelCodeSeedream45      = "ByteDance/Seedream-4.5"
	ModelCodeSDXLTurbo       = "stabilityai/sdxl-turbo"
	ModelCodeMockImage       = providermodels.ProviderModelMockImage
	ModelCodePVideo          = "PrunaAI/p-video"

	ImageQuality1K = "1K"
	ImageQuality2K = "2K"
	ImageQuality4K = "4K"
)

// Model is the private server-side model spec selected for a user-facing
// public ID.
type Model struct {
	ModelID                string
	ModelName              string
	Provider               domain.ProviderName
	ModelCode              string
	ExposeID               bool
	DurationSec            int
	SupportsReferenceImage bool
	MaxReferenceImages     int
	MaxOutputCount         int
	AllowedAspectRatios    []string
}

var publicDefaultModel = map[domain.OperationType]string{
	domain.OperationTextGenerate:  PublicChatModelID,
	domain.OperationImageGenerate: PublicImageNanoBananaPro,
	domain.OperationVideoGenerate: "",
}

var vkVideoModels = map[string]Model{}

// ResolvePublicModel resolves a server-trusted public product identifier to its
// private execution model shared by the website and VK bot.
func ResolvePublicModel(op domain.OperationType, raw string) (Model, bool) {
	modelID := strings.TrimSpace(raw)
	if modelID == "" {
		modelID = publicDefaultModel[op]
	}
	models := publicModels(op)
	model, ok := models[modelID]
	return model, ok
}

// PublicResponseModelID returns only explicitly public identifiers.
func PublicResponseModelID(model Model) string {
	if model.ExposeID {
		return model.ModelID
	}
	return ""
}

func NormalizeImageQuality(raw string) (string, bool) {
	if resolution, quality, ok := strings.Cut(strings.TrimSpace(raw), "-"); ok {
		resolution, quality = strings.ToUpper(resolution), strings.ToLower(quality)
		if resolution == "1K" || resolution == "2K" || resolution == "4K" {
			switch quality {
			case "low", "medium", "high", "xhigh", "max":
				return resolution + "-" + quality, true
			}
		}
	}
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "1.5K", "3K":
		return strings.ToUpper(strings.TrimSpace(raw)), true
	case "1MP", "2MP", "3MP", "4MP":
		return strings.ToUpper(strings.TrimSpace(raw)), true
	case "RELAX", "FAST", "TURBO":
		return strings.ToLower(strings.TrimSpace(raw)), true
	case "STANDARD":
		return "standard", true
	case ImageQuality1K:
		return ImageQuality1K, true
	case ImageQuality2K:
		return ImageQuality2K, true
	case ImageQuality4K:
		return ImageQuality4K, true
	default:
		return "", false
	}
}

func ApplyImageQuality(model Model, quality string) Model {
	// Quality is a public request/pricing dimension. Provider routing stays
	// model-only; pricingcatalog owns generation prices.
	_, _ = NormalizeImageQuality(quality)
	return model
}

func ListPublicModels(op domain.OperationType) []Model {
	ordered := orderedPublicModels(op)
	if len(ordered) == 0 {
		return nil
	}
	models := publicModels(op)
	out := make([]Model, 0, len(ordered)+len(models))
	seen := map[string]struct{}{}
	for _, orderedModel := range ordered {
		modelID := orderedModel.ModelID
		model, ok := models[modelID]
		if !ok {
			continue
		}
		out = append(out, model)
		seen[model.ModelID] = struct{}{}
	}
	for _, model := range models {
		if _, ok := seen[model.ModelID]; ok {
			continue
		}
		out = append(out, model)
		seen[model.ModelID] = struct{}{}
	}
	return out
}

func ResolveVKVideoModel(raw string) (Model, bool) {
	modelID := strings.TrimSpace(raw)
	model, ok := vkVideoModels[modelID]
	return model, ok
}

func publicModels(op domain.OperationType) map[string]Model {
	switch op {
	case domain.OperationTextGenerate:
		return publicTextModels()
	case domain.OperationImageGenerate:
		return publicImageModels()
	default:
		return map[string]Model{}
	}
}

func orderedPublicModels(op domain.OperationType) []Model {
	switch op {
	case domain.OperationTextGenerate:
		if model, ok := publicTextModels()[PublicChatModelID]; ok {
			return []Model{model}
		}
	case domain.OperationImageGenerate:
		registry := providermodels.RuntimeRegistry()
		models := make([]Model, 0, len(registry.PublicImageModels())+len(registry.LoadTestImageModels))
		for _, registryModel := range registry.PublicImageModels() {
			models = append(models, modelFromRegistryImage(registryModel))
		}
		for _, registryModel := range registry.LoadTestImageModels {
			models = append(models, modelFromRegistryImage(registryModel))
		}
		return models
	}
	return nil
}

func publicTextModels() map[string]Model {
	models := map[string]Model{}
	for _, alias := range providermodels.RuntimeRegistry().TextAliasModels() {
		model := Model{
			ModelID:   alias.PublicID,
			ModelName: alias.DisplayName,
		}
		models[alias.PublicID] = model
		if providermodels.IsPaidTextRoute(alias.Provider, alias.ProviderModelID) {
			model.Provider, model.ModelCode, model.ExposeID = alias.Provider, alias.ProviderModelID, true
			models[alias.PublicID] = model
			continue
		}
		models[alias.DisplayName] = model
		models[alias.ProviderModelID] = model
	}
	if model, ok := models[PublicChatModelID]; ok {
		models["deepseek-v4-flash"] = model
	}
	return models
}

func publicImageModels() map[string]Model {
	registry := providermodels.RuntimeRegistry()
	models := map[string]Model{}
	for _, registryModel := range registry.PublicImageModels() {
		model := modelFromRegistryImage(registryModel)
		models[model.ModelID] = model
	}
	for _, registryModel := range registry.LoadTestImageModels {
		model := modelFromRegistryImage(registryModel)
		models[model.ModelID] = model
	}
	if model, ok := models[PublicImageNanoBananaPro]; ok {
		models["kandinsky"] = model
	}
	return models
}

func modelFromRegistryImage(registryModel providermodels.ImageModel) Model {
	return Model{
		ModelID:                registryModel.PublicID,
		ModelName:              registryModel.DisplayName,
		Provider:               registryModel.Provider,
		ModelCode:              registryModel.ProviderModelID,
		ExposeID:               true,
		SupportsReferenceImage: registryModel.Limits.SupportsReferenceImage,
		MaxReferenceImages:     registryModel.Limits.MaxReferenceImages,
		MaxOutputCount:         registryModel.Limits.MaxOutputCount,
		AllowedAspectRatios:    append([]string(nil), registryModel.Limits.AllowedAspectRatios...),
	}
}
