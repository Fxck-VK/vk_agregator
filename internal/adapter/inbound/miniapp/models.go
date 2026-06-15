package miniapp

import (
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/modelcatalog"
)

const (
	miniAppChatModelID         = modelcatalog.MiniAppChatModelID
	miniAppChatPublicModelName = modelcatalog.MiniAppChatModelName
)

type miniAppModelSpec = modelcatalog.Model

func resolveMiniAppModel(op domain.OperationType, raw string) (miniAppModelSpec, bool) {
	return modelcatalog.ResolveMiniAppModel(op, raw)
}

func miniAppResponseModelID(model miniAppModelSpec) string {
	return modelcatalog.MiniAppResponseModelID(model)
}
