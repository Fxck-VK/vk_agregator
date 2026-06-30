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
	MiniAppChatModelID   = providermodels.PublicTextChatGPT
	MiniAppChatModelName = "NeiroHub Chat"

	MiniAppImageNanoBananaPro   = providermodels.PublicImageNanoBananaPro
	MiniAppImageGPTImage2       = providermodels.PublicImageGPTImage2
	MiniAppImageNanoBananaFlash = "nano_banana_flash"
	MiniAppImageNanoBanana2     = providermodels.PublicImageNanoBanana2
	MiniAppImageSeedream45      = "seedream_4_5"
	MiniAppImageSDXLTurbo       = "sdxl_turbo"
	MiniAppImageMock            = providermodels.LoadTestImageMock
	MiniAppVideoKling           = "kling"

	VKVideoPrunaAI = "prunaai"

	ModelCodePoYoNanoBanana2 = providermodels.ProviderModelPoYoNanoBanana2
	ModelCodeGemini3ProImage = providermodels.ProviderModelGemini3ProImage
	ModelCodeGPTImage2       = providermodels.ProviderModelGPTImage2
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
}

var miniAppDefaultModel = map[domain.OperationType]string{
	domain.OperationTextGenerate:  MiniAppChatModelID,
	domain.OperationImageGenerate: MiniAppImageNanoBananaPro,
	domain.OperationVideoGenerate: "",
}

var vkVideoModels = map[string]Model{}

func ResolveMiniAppModel(op domain.OperationType, raw string) (Model, bool) {
	modelID := strings.TrimSpace(raw)
	if modelID == "" {
		modelID = miniAppDefaultModel[op]
	}
	models := miniAppModels(op)
	model, ok := models[modelID]
	return model, ok
}

func MiniAppResponseModelID(model Model) string {
	if model.ExposeID {
		return model.ModelID
	}
	return ""
}

func NormalizeImageQuality(raw string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
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

func ListMiniAppModels(op domain.OperationType) []Model {
	ordered := orderedMiniAppModels(op)
	if len(ordered) == 0 {
		return nil
	}
	models := miniAppModels(op)
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

func miniAppModels(op domain.OperationType) map[string]Model {
	switch op {
	case domain.OperationTextGenerate:
		return miniAppTextModels()
	case domain.OperationImageGenerate:
		return miniAppImageModels()
	default:
		return map[string]Model{}
	}
}

func orderedMiniAppModels(op domain.OperationType) []Model {
	switch op {
	case domain.OperationTextGenerate:
		if model, ok := miniAppTextModels()[MiniAppChatModelID]; ok {
			return []Model{model}
		}
	case domain.OperationImageGenerate:
		registry := providermodels.StaticRegistry()
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

func miniAppTextModels() map[string]Model {
	models := map[string]Model{}
	for _, alias := range providermodels.StaticRegistry().TextAliasModels() {
		model := Model{
			ModelID:   alias.PublicID,
			ModelName: alias.DisplayName,
		}
		models[alias.PublicID] = model
		models[alias.DisplayName] = model
		models[alias.ProviderModelID] = model
	}
	if model, ok := models[MiniAppChatModelID]; ok {
		models["deepseek-v4-flash"] = model
	}
	return models
}

func miniAppImageModels() map[string]Model {
	registry := providermodels.StaticRegistry()
	models := map[string]Model{}
	for _, registryModel := range registry.PublicImageModels() {
		model := modelFromRegistryImage(registryModel)
		models[model.ModelID] = model
	}
	for _, registryModel := range registry.LoadTestImageModels {
		model := modelFromRegistryImage(registryModel)
		models[model.ModelID] = model
	}
	if model, ok := models[MiniAppImageNanoBananaPro]; ok {
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
	}
}
