package productcatalog

import (
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/textgeneration"
)

func textModelsFromConfig(cfg config.Config, r RuntimeCatalog) []textgeneration.PublicModel {
	var ids []string
	for _, m := range providermodels.StaticRegistry().TextAliasModels() {
		if m.PublicID == providermodels.PublicTextChatGPT {
			continue
		}
		if featureFlagEnabled(cfg, m.FeatureFlag) && providerReadyFromReadiness(cfg, m.Readiness) {
			ids = append(ids, m.PublicID)
		}
	}
	return textgeneration.Models(ids, r.PricingCatalog)
}
