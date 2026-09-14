package providermodels

import (
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

const (
	PublicTextChatGPT       = "chatgpt"
	ProviderModelDeepSeekV4 = "deepseek-ai/DeepSeek-V4-Flash"
)

// TextAlias is the public text model alias mapped to the hidden provider model.
type TextAlias struct {
	PublicID        string
	DisplayName     string
	Provider        domain.ProviderName
	ProviderModelID string
	FeatureFlag     string
	Readiness       ProviderReadiness
	PricingKeys     []pricingcatalog.ProductKey
}

func textAliases() []TextAlias {
	return append([]TextAlias{
		{
			PublicID:        PublicTextChatGPT,
			DisplayName:     "NeiroHub Chat",
			Provider:        domain.ProviderDeepInfra,
			ProviderModelID: ProviderModelDeepSeekV4,
			Readiness: ProviderReadiness{
				RequiredConfigKeys: []string{ConfigKeyDeepInfraKey, ConfigKeyDeepInfraURL},
			},
		},
	}, PaidTextModels()...)
}

func (r Registry) TextAlias(publicID string) (TextAlias, bool) {
	for _, alias := range r.TextAliases {
		if alias.PublicID == publicID {
			return copyTextAlias(alias), true
		}
	}
	return TextAlias{}, false
}

func (r Registry) TextAliasModels() []TextAlias {
	out := make([]TextAlias, 0, len(r.TextAliases))
	for _, alias := range r.TextAliases {
		out = append(out, copyTextAlias(alias))
	}
	return out
}

func copyTextAlias(alias TextAlias) TextAlias {
	alias.Readiness = copyReadiness(alias.Readiness)
	alias.PricingKeys = append([]pricingcatalog.ProductKey(nil), alias.PricingKeys...)
	return alias
}
