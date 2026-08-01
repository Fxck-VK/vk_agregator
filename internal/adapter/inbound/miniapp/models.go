package miniapp

import (
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/modelcatalog"
)

const (
	miniAppChatModelID         = modelcatalog.MiniAppChatModelID
	miniAppChatPublicModelName = modelcatalog.MiniAppChatModelName
)

type miniAppModelSpec struct {
	modelcatalog.Model
	imageResolution *imagegeneration.Resolution
}

func resolveMiniAppModel(op domain.OperationType, raw string) (miniAppModelSpec, bool) {
	model, ok := modelcatalog.ResolveMiniAppModel(op, raw)
	return miniAppModelSpec{Model: model}, ok
}

func miniAppResponseModelID(model miniAppModelSpec) string {
	return modelcatalog.MiniAppResponseModelID(model.Model)
}

func miniAppModelFromImageResolution(resolution imagegeneration.Resolution) miniAppModelSpec {
	return miniAppModelSpec{
		Model: modelcatalog.Model{
			ModelID:   resolution.Worker.ModelID,
			ModelName: resolution.Worker.ModelName,
			Provider:  resolution.Worker.Provider,
			ModelCode: resolution.Worker.ModelCode,
			ExposeID:  true,
		},
		imageResolution: &resolution,
	}
}
