package miniapp

import (
	"strings"

	"vk-ai-aggregator/internal/domain"
)

const (
	miniAppChatModelID         = "chatgpt"
	miniAppChatPublicModelName = "ChatGPT"

	miniAppImageModelNanoBananaPro   = "nano_banana_pro"
	miniAppImageModelNanoBananaFlash = "nano_banana_flash"

	miniAppModelCodeSeedream45 = "ByteDance/Seedream-4.5"
	miniAppModelCodeSDXLTurbo  = "stabilityai/sdxl-turbo"
	miniAppModelCodePVideo     = "PrunaAI/p-video"
)

type miniAppModelSpec struct {
	ModelID   string
	ModelName string
	ModelCode string
	ExposeID  bool
}

var miniAppModels = map[domain.OperationType]map[string]miniAppModelSpec{
	domain.OperationTextGenerate: {
		miniAppChatModelID: {
			ModelID:   miniAppChatModelID,
			ModelName: miniAppChatPublicModelName,
		},
		miniAppChatPublicModelName: {
			ModelID:   miniAppChatModelID,
			ModelName: miniAppChatPublicModelName,
		},
		"deepseek-v4-flash": {
			ModelID:   miniAppChatModelID,
			ModelName: miniAppChatPublicModelName,
		},
		"deepseek-ai/DeepSeek-V4-Flash": {
			ModelID:   miniAppChatModelID,
			ModelName: miniAppChatPublicModelName,
		},
	},
	domain.OperationImageGenerate: {
		miniAppImageModelNanoBananaPro: {
			ModelID:   miniAppImageModelNanoBananaPro,
			ModelName: "Nano Banana Pro",
			ModelCode: miniAppModelCodeSeedream45,
			ExposeID:  true,
		},
		miniAppImageModelNanoBananaFlash: {
			ModelID:   miniAppImageModelNanoBananaFlash,
			ModelName: "Nano Banana Flash",
			ModelCode: miniAppModelCodeSDXLTurbo,
			ExposeID:  true,
		},
		// Legacy public aliases remain accepted for older Mini App clients.
		"sdxl": {
			ModelID:   "sdxl",
			ModelName: "Nano Banana Flash",
			ModelCode: miniAppModelCodeSDXLTurbo,
			ExposeID:  true,
		},
		"kandinsky": {
			ModelID:   "kandinsky",
			ModelName: "Nano Banana Pro",
			ModelCode: miniAppModelCodeSeedream45,
			ExposeID:  true,
		},
	},
	domain.OperationVideoGenerate: {
		"kling": {
			ModelID:   "kling",
			ModelName: "Kling",
			ModelCode: miniAppModelCodePVideo,
			ExposeID:  true,
		},
	},
}

var miniAppDefaultModel = map[domain.OperationType]string{
	domain.OperationTextGenerate:  miniAppChatModelID,
	domain.OperationImageGenerate: miniAppImageModelNanoBananaPro,
	domain.OperationVideoGenerate: "kling",
}

func resolveMiniAppModel(op domain.OperationType, raw string) (miniAppModelSpec, bool) {
	modelID := strings.TrimSpace(raw)
	if modelID == "" {
		modelID = miniAppDefaultModel[op]
	}
	models, ok := miniAppModels[op]
	if !ok {
		return miniAppModelSpec{}, false
	}
	model, ok := models[modelID]
	return model, ok
}

func miniAppResponseModelID(model miniAppModelSpec) string {
	if model.ExposeID {
		return model.ModelID
	}
	return ""
}
