// Package modelcatalog owns server-side public model choices for inbound
// surfaces. Clients may send public IDs only; provider/model codes come from
// this catalog.
package modelcatalog

import (
	"strings"

	"vk-ai-aggregator/internal/domain"
)

const (
	MiniAppChatModelID   = "chatgpt"
	MiniAppChatModelName = "NeiroHub Chat"

	MiniAppImageNanoBananaPro   = "nano_banana_pro"
	MiniAppImageGPTImage2       = "gpt_image_2"
	MiniAppImageNanoBananaFlash = "nano_banana_flash"
	MiniAppImageNanoBanana2     = "nano_banana_2"
	MiniAppImageSeedream45      = "seedream_4_5"
	MiniAppImageSDXLTurbo       = "sdxl_turbo"
	MiniAppImageMock            = "mock_image"
	MiniAppVideoKling           = "kling"

	VKVideoPrunaAI = "prunaai"

	ModelCodePoYoNanoBanana2 = "nano-banana-2-new"
	ModelCodeGemini3ProImage = "gemini-3-pro-image-preview"
	ModelCodeGPTImage2       = "gpt-image-2"
	ModelCodeSeedream45      = "ByteDance/Seedream-4.5"
	ModelCodeSDXLTurbo       = "stabilityai/sdxl-turbo"
	ModelCodeMockImage       = "mock-image"
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

var miniAppModels = map[domain.OperationType]map[string]Model{
	domain.OperationTextGenerate: {
		MiniAppChatModelID: {
			ModelID:   MiniAppChatModelID,
			ModelName: MiniAppChatModelName,
		},
		MiniAppChatModelName: {
			ModelID:   MiniAppChatModelID,
			ModelName: MiniAppChatModelName,
		},
		"deepseek-v4-flash": {
			ModelID:   MiniAppChatModelID,
			ModelName: MiniAppChatModelName,
		},
		"deepseek-ai/DeepSeek-V4-Flash": {
			ModelID:   MiniAppChatModelID,
			ModelName: MiniAppChatModelName,
		},
	},
	domain.OperationImageGenerate: {
		MiniAppImageNanoBanana2: {
			ModelID:                MiniAppImageNanoBanana2,
			ModelName:              "Nano Banana 2",
			Provider:               domain.ProviderPoYo,
			ModelCode:              ModelCodePoYoNanoBanana2,
			ExposeID:               true,
			SupportsReferenceImage: true,
			MaxReferenceImages:     4,
		},
		MiniAppImageNanoBananaPro: {
			ModelID:                MiniAppImageNanoBananaPro,
			ModelName:              "Nano Banana Pro",
			Provider:               domain.ProviderAPIMart,
			ModelCode:              ModelCodeGemini3ProImage,
			ExposeID:               true,
			SupportsReferenceImage: true,
			MaxReferenceImages:     14,
		},
		MiniAppImageGPTImage2: {
			ModelID:                MiniAppImageGPTImage2,
			ModelName:              "GPT Image 2",
			Provider:               domain.ProviderAPIMart,
			ModelCode:              ModelCodeGPTImage2,
			ExposeID:               true,
			SupportsReferenceImage: true,
			MaxReferenceImages:     16,
		},
		MiniAppImageMock: {
			ModelID:   MiniAppImageMock,
			ModelName: "Mock Image Loadtest",
			Provider:  domain.ProviderMock,
			ModelCode: ModelCodeMockImage,
			ExposeID:  true,
		},
		"kandinsky": {
			ModelID:                MiniAppImageNanoBananaPro,
			ModelName:              "Nano Banana Pro",
			Provider:               domain.ProviderAPIMart,
			ModelCode:              ModelCodeGemini3ProImage,
			ExposeID:               true,
			SupportsReferenceImage: true,
			MaxReferenceImages:     14,
		},
	},
	domain.OperationVideoGenerate: {},
}

var miniAppDefaultModel = map[domain.OperationType]string{
	domain.OperationTextGenerate:  MiniAppChatModelID,
	domain.OperationImageGenerate: MiniAppImageNanoBananaPro,
	domain.OperationVideoGenerate: "",
}

var miniAppModelOrder = map[domain.OperationType][]string{
	domain.OperationTextGenerate:  {MiniAppChatModelID},
	domain.OperationImageGenerate: {MiniAppImageNanoBanana2, MiniAppImageNanoBananaPro, MiniAppImageGPTImage2, MiniAppImageMock},
	domain.OperationVideoGenerate: {},
}

var vkVideoModels = map[string]Model{}

func ResolveMiniAppModel(op domain.OperationType, raw string) (Model, bool) {
	modelID := strings.TrimSpace(raw)
	if modelID == "" {
		modelID = miniAppDefaultModel[op]
	}
	models, ok := miniAppModels[op]
	if !ok {
		return Model{}, false
	}
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
	models, ok := miniAppModels[op]
	if !ok {
		return nil
	}
	out := make([]Model, 0, len(models))
	seen := map[string]struct{}{}
	for _, modelID := range miniAppModelOrder[op] {
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
