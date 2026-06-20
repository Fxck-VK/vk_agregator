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
	MiniAppChatModelName = "ChatGPT"

	MiniAppImageNanoBananaPro   = "nano_banana_pro"
	MiniAppImageGPTImage2       = "gpt_image_2"
	MiniAppImageNanoBananaFlash = "nano_banana_flash"
	MiniAppImageNanoBanana2     = "nano_banana_2"
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
			MaxInternalCostCredits: 20,
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
			ModelID:   MiniAppImageNanoBananaFlash,
			ModelName: "Nano Banana Flash",
			Provider:  domain.ProviderDeepInfra,
			ModelCode: ModelCodeSDXLTurbo,
			ExposeID:  true,
		},
		// Legacy public aliases remain accepted for older Mini App clients.
		"sdxl": {
			ModelID:   "sdxl",
			ModelName: "Nano Banana Flash",
			Provider:  domain.ProviderDeepInfra,
			ModelCode: ModelCodeSDXLTurbo,
			ExposeID:  true,
		},
		"kandinsky": {
			ModelID:                "kandinsky",
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
	domain.OperationImageGenerate: {MiniAppImageNanoBanana2, MiniAppImageNanoBananaPro, MiniAppImageGPTImage2, MiniAppImageNanoBananaFlash},
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
