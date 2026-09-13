package providermodels

import (
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

const (
	PublicTextGPT55          = "gpt_5_5"
	PublicTextClaudeOpus47   = "claude_opus_4_7"
	PublicTextGemini31Pro    = "gemini_3_1_pro"
	ModelGPT55               = "gpt-5-5"
	ModelClaudeOpus47        = "claude-opus-4-7"
	ModelGemini31Pro         = "gemini-3.1-pro"
	FeatureTextGPT55         = "FEATURE_TEXT_GPT_5_5_ENABLED"
	FeatureTextClaudeOpus47  = "FEATURE_TEXT_CLAUDE_OPUS_4_7_ENABLED"
	FeatureTextGemini31Pro   = "FEATURE_TEXT_GEMINI_3_1_PRO_ENABLED"
	PublicTextClaudeOpus48   = "claude_opus_4_8"
	ModelClaudeOpus48        = "claude-opus-4-8"
	FeatureTextClaudeOpus48  = "FEATURE_TEXT_CLAUDE_OPUS_4_8_ENABLED"
	PublicTextGPT56Terra     = "gpt_5_6_terra"
	ModelGPT56Terra          = "gpt-5-6-terra"
	FeatureTextGPT56Terra    = "FEATURE_TEXT_GPT_5_6_TERRA_ENABLED"
	PublicTextGPT6Astra      = "gpt_6_astra"
	ModelGPT6Astra           = "gpt-6-astra"
	FeatureTextGPT6Astra     = "FEATURE_TEXT_GPT_6_ASTRA_ENABLED"
	PublicTextClaudeOpus5    = "claude_opus_5"
	ModelClaudeOpus5         = "claude-opus-5"
	FeatureTextClaudeOpus5   = "FEATURE_TEXT_CLAUDE_OPUS_5_ENABLED"
	PublicTextGemini37Flash  = "gemini_3_7_flash"
	ModelGemini37Flash       = "gemini-3-7-flash-openai"
	FeatureTextGemini37Flash = "FEATURE_TEXT_GEMINI_3_7_FLASH_ENABLED"
	PublicTextClaudeFable51  = "claude_fable_5_1"
	ModelClaudeFable51       = "claude-fable-5.1"
	FeatureTextClaudeFable51 = "FEATURE_TEXT_CLAUDE_FABLE_5_1_ENABLED"
	PublicTextClaudeFable5   = "claude_fable_5"
	ModelClaudeFable5        = "claude-fable-5"
	FeatureTextClaudeFable5  = "FEATURE_TEXT_CLAUDE_FABLE_5_ENABLED"
	PublicTextGemini36Flash  = "gemini_3_6_flash"
	ModelGemini36Flash       = "gemini-3-6-flash-openai"
	FeatureTextGemini36Flash = "FEATURE_TEXT_GEMINI_3_6_FLASH_ENABLED"
)

func PaidTextModels() []TextAlias {
	var out []TextAlias
	for _, m := range []struct {
		id, name, model, flag string
		provider              domain.ProviderName
	}{
		{PublicTextGPT55, "GPT-5.5", ModelGPT55, FeatureTextGPT55, domain.ProviderKIE},
		{PublicTextClaudeOpus47, "Claude Opus 4.7", ModelClaudeOpus47, FeatureTextClaudeOpus47, domain.ProviderKIE},
		{PublicTextGemini31Pro, "Gemini 3.1 Pro", ModelGemini31Pro, FeatureTextGemini31Pro, domain.ProviderKIE},
		{PublicTextClaudeOpus48, "Claude Opus 4.8", ModelClaudeOpus48, FeatureTextClaudeOpus48, domain.ProviderKIE},
		{PublicTextGPT56Terra, "GPT 5.6 Terra", ModelGPT56Terra, FeatureTextGPT56Terra, domain.ProviderKIE},
		{PublicTextGPT6Astra, "GPT 6 Astra", ModelGPT6Astra, FeatureTextGPT6Astra, domain.ProviderKIE},
		{PublicTextClaudeOpus5, "Claude Opus 5", ModelClaudeOpus5, FeatureTextClaudeOpus5, domain.ProviderKIE},
		{PublicTextGemini37Flash, "Gemini 3.7 Flash", ModelGemini37Flash, FeatureTextGemini37Flash, domain.ProviderKIE},
		{PublicTextClaudeFable51, "Claude Fable 5.1", ModelClaudeFable51, FeatureTextClaudeFable51, domain.ProviderAPIMart},
		{PublicTextClaudeFable5, "Claude Fable 5", ModelClaudeFable5, FeatureTextClaudeFable5, domain.ProviderKIE},
		{PublicTextGemini36Flash, "Gemini 3.6 Flash", ModelGemini36Flash, FeatureTextGemini36Flash, domain.ProviderKIE},
	} {
		readiness := ProviderReadiness{ProviderEnabledFlag: "KIE_PROVIDER_ENABLED", RequiredConfigKeys: []string{"KIE_API_KEY", "KIE_BASE_URL"}}
		if m.provider == domain.ProviderAPIMart {
			readiness = ProviderReadiness{ProviderEnabledFlag: "APIMART_TEXT_LIMITS_VERIFIED", RequiredConfigKeys: []string{ConfigKeyAPIMartAPIKey, ConfigKeyAPIMartBaseURL}}
		}
		out = append(out, TextAlias{PublicID: m.id, DisplayName: m.name, Provider: m.provider, ProviderModelID: m.model, FeatureFlag: m.flag,
			Readiness:   readiness,
			PricingKeys: []pricingcatalog.ProductKey{{Operation: domain.OperationTextGenerate, Modality: domain.ModalityText, TextModelID: m.id}}})
	}
	return out
}

func PaidTextModel(id string) (TextAlias, bool) {
	for _, m := range PaidTextModels() {
		if m.PublicID == id {
			return m, true
		}
	}
	return TextAlias{}, false
}

func IsPaidTextRoute(provider domain.ProviderName, code string) bool {
	for _, m := range PaidTextModels() {
		if m.Provider == provider && m.ProviderModelID == code {
			return true
		}
	}
	return false
}
