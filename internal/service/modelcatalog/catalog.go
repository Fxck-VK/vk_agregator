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
	MiniAppImageNanoBananaFlash = "nano_banana_flash"
	MiniAppVideoKling           = "kling"

	VKVideoPrunaAI = "prunaai"
	VKVideoSora2   = "sora_2"

	ModelCodeSeedream45 = "ByteDance/Seedream-4.5"
	ModelCodeSDXLTurbo  = "stabilityai/sdxl-turbo"
	ModelCodePVideo     = "PrunaAI/p-video"
	ModelCodeSora2      = "sora-2"
)

// Model is the private server-side model spec selected for a user-facing
// public ID.
type Model struct {
	ModelID     string
	ModelName   string
	Provider    domain.ProviderName
	ModelCode   string
	ExposeID    bool
	DurationSec int
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
		MiniAppImageNanoBananaPro: {
			ModelID:   MiniAppImageNanoBananaPro,
			ModelName: "Nano Banana Pro",
			Provider:  domain.ProviderDeepInfra,
			ModelCode: ModelCodeSeedream45,
			ExposeID:  true,
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
			ModelID:   "kandinsky",
			ModelName: "Nano Banana Pro",
			Provider:  domain.ProviderDeepInfra,
			ModelCode: ModelCodeSeedream45,
			ExposeID:  true,
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

func ResolveVKVideoModel(raw string) (Model, bool) {
	modelID := strings.TrimSpace(raw)
	model, ok := vkVideoModels[modelID]
	return model, ok
}
