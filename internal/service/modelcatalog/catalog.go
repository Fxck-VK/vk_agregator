// Package modelcatalog owns server-side public model choices for inbound
// surfaces. Clients may send public IDs only; provider/model codes come from
// this catalog.
package modelcatalog

import (
	"math"
	"strings"

	"vk-ai-aggregator/internal/domain"
)

const (
	MiniAppChatModelID   = "chatgpt"
	MiniAppChatModelName = "ChatGPT"

	MiniAppImageNanoBananaPro   = "nano_banana_pro"
	MiniAppImageGPTImage2       = "gpt_image_2"
	MiniAppImageNanoBananaFlash = "nano_banana_flash"
	MiniAppImageNanoBanana2     = "nano_banana_2"
	MiniAppImageSeedream45      = "seedream_4_5"
	MiniAppImageSDXLTurbo       = "sdxl_turbo"
	MiniAppVideoKling           = "kling"

	VKVideoPrunaAI = "prunaai"
	VKVideoSora2   = "sora_2"

	ModelCodePoYoNanoBanana2 = "nano-banana-2-new"
	ModelCodeGemini3ProImage = "gemini-3-pro-image-preview"
	ModelCodeGPTImage2       = "gpt-image-2"
	ModelCodeSeedream45      = "ByteDance/Seedream-4.5"
	ModelCodeSDXLTurbo       = "stabilityai/sdxl-turbo"
	ModelCodePVideo          = "PrunaAI/p-video"
	ModelCodeSora2           = "sora-2"

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
	ProviderCostCredits    int64
	PriceMultiplier        float64
	MaxInternalCostCredits int64
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
			ProviderCostCredits:    5,
			PriceMultiplier:        2,
			MaxInternalCostCredits: 24,
			SupportsReferenceImage: true,
			MaxReferenceImages:     4,
		},
		MiniAppImageNanoBananaPro: {
			ModelID:                MiniAppImageNanoBananaPro,
			ModelName:              "Nano Banana Pro",
			Provider:               domain.ProviderAPIMart,
			ModelCode:              ModelCodeGemini3ProImage,
			ExposeID:               true,
			ProviderCostCredits:    10,
			PriceMultiplier:        2,
			MaxInternalCostCredits: 40,
			SupportsReferenceImage: true,
			MaxReferenceImages:     14,
		},
		MiniAppImageGPTImage2: {
			ModelID:                MiniAppImageGPTImage2,
			ModelName:              "GPT Image 2",
			Provider:               domain.ProviderAPIMart,
			ModelCode:              ModelCodeGPTImage2,
			ExposeID:               true,
			ProviderCostCredits:    10,
			PriceMultiplier:        2,
			MaxInternalCostCredits: 40,
			SupportsReferenceImage: true,
			MaxReferenceImages:     16,
		},
		MiniAppImageNanoBananaFlash: {
			ModelID:   MiniAppImageSDXLTurbo,
			ModelName: "Stability AI SDXL Turbo",
			Provider:  domain.ProviderDeepInfra,
			ModelCode: ModelCodeSDXLTurbo,
			ExposeID:  true,
		},
		MiniAppImageSeedream45: {
			ModelID:                MiniAppImageSeedream45,
			ModelName:              "ByteDance Seedream 4.5",
			Provider:               domain.ProviderDeepInfra,
			ModelCode:              ModelCodeSeedream45,
			ExposeID:               true,
			ProviderCostCredits:    10,
			PriceMultiplier:        1,
			MaxInternalCostCredits: 10,
		},
		MiniAppImageSDXLTurbo: {
			ModelID:                MiniAppImageSDXLTurbo,
			ModelName:              "Stability AI SDXL Turbo",
			Provider:               domain.ProviderDeepInfra,
			ModelCode:              ModelCodeSDXLTurbo,
			ExposeID:               true,
			ProviderCostCredits:    10,
			PriceMultiplier:        1,
			MaxInternalCostCredits: 10,
		},
		// Legacy public aliases remain accepted for older Mini App clients.
		"sdxl": {
			ModelID:                MiniAppImageSDXLTurbo,
			ModelName:              "Stability AI SDXL Turbo",
			Provider:               domain.ProviderDeepInfra,
			ModelCode:              ModelCodeSDXLTurbo,
			ExposeID:               true,
			ProviderCostCredits:    10,
			PriceMultiplier:        1,
			MaxInternalCostCredits: 10,
		},
		"kandinsky": {
			ModelID:                MiniAppImageNanoBananaPro,
			ModelName:              "Nano Banana Pro",
			Provider:               domain.ProviderAPIMart,
			ModelCode:              ModelCodeGemini3ProImage,
			ExposeID:               true,
			ProviderCostCredits:    10,
			PriceMultiplier:        2,
			MaxInternalCostCredits: 40,
			SupportsReferenceImage: true,
			MaxReferenceImages:     14,
		},
	},
	domain.OperationVideoGenerate: {
		MiniAppVideoKling: {
			ModelID:     MiniAppVideoKling,
			ModelName:   "Kling",
			Provider:    domain.ProviderDeepInfra,
			ModelCode:   ModelCodePVideo,
			ExposeID:    true,
			DurationSec: 5,
		},
	},
}

var miniAppDefaultModel = map[domain.OperationType]string{
	domain.OperationTextGenerate:  MiniAppChatModelID,
	domain.OperationImageGenerate: MiniAppImageNanoBananaPro,
	domain.OperationVideoGenerate: MiniAppVideoKling,
}

var miniAppModelOrder = map[domain.OperationType][]string{
	domain.OperationTextGenerate:  {MiniAppChatModelID},
	domain.OperationImageGenerate: {MiniAppImageNanoBanana2, MiniAppImageNanoBananaPro, MiniAppImageGPTImage2, MiniAppImageSeedream45, MiniAppImageSDXLTurbo},
	domain.OperationVideoGenerate: {MiniAppVideoKling},
}

var vkVideoModels = map[string]Model{
	VKVideoPrunaAI: {
		ModelID:     VKVideoPrunaAI,
		ModelName:   "PrunaAI",
		Provider:    domain.ProviderDeepInfra,
		ModelCode:   ModelCodePVideo,
		DurationSec: 5,
	},
	VKVideoSora2: {
		ModelID:     VKVideoSora2,
		ModelName:   "Sora 2",
		Provider:    domain.ProviderOpenAI,
		ModelCode:   ModelCodeSora2,
		DurationSec: 5,
	},
}

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
	quality, ok := NormalizeImageQuality(quality)
	if !ok {
		return model
	}
	if model.ModelID == MiniAppImageNanoBanana2 {
		switch quality {
		case ImageQuality2K:
			model.ProviderCostCredits = 8
		case ImageQuality4K:
			model.ProviderCostCredits = 12
		default:
			model.ProviderCostCredits = 5
		}
		if minCap := estimateInternalCost(model.ProviderCostCredits, model.PriceMultiplier); minCap > model.MaxInternalCostCredits {
			model.MaxInternalCostCredits = minCap
		}
	}
	return model
}

func estimateInternalCost(providerCostCredits int64, multiplier float64) int64 {
	if providerCostCredits <= 0 {
		return 0
	}
	if multiplier <= 0 {
		multiplier = 1
	}
	return int64(math.Ceil(float64(providerCostCredits) * multiplier))
}

func EstimateInternalCostCredits(model Model) int64 {
	cost := estimateInternalCost(model.ProviderCostCredits, model.PriceMultiplier)
	if cost <= 0 {
		return 0
	}
	if model.MaxInternalCostCredits > 0 && cost > model.MaxInternalCostCredits {
		return model.MaxInternalCostCredits
	}
	return cost
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
