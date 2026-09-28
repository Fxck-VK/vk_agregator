package providermodels

import (
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/modelcontract"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

const TextCandidateCheckedAt = "2026-09-28"
const APIMartChatDocumentation = "https://docs.apimart.ai/ru/api-reference/texts/general/chat-completions"

// TextCandidate is an exact documented route awaiting live admission. Provider
// token limits are facts from the pricing API; zero means unpublished, not unlimited.
type TextCandidate struct {
	PublicID, Name, ModelCode                       string
	ProviderMaxInputTokens, ProviderMaxOutputTokens int
}

func TextCandidates() []TextCandidate {
	return []TextCandidate{
		{"gpt_5", "GPT-5", "gpt-5", 128000, 16000},
		{"gpt_5_1", "GPT-5.1", "gpt-5.1", 128000, 16000},
		{"gpt_5_chat_latest", "GPT-5 Chat Latest", "gpt-5-chat-latest", 0, 0},
		{"gpt_5_mini", "GPT-5 Mini", "gpt-5-mini", 128000, 16000},
		{"claude_opus_4_6", "Claude Opus 4.6", "claude-opus-4-6", 1000000, 1000000},
		{"claude_sonnet_4_6", "Claude Sonnet 4.6", "claude-sonnet-4-6", 128000, 16000},
		{"claude_opus_4_5_20251101", "Claude Opus 4.5 (20251101)", "claude-opus-4-5-20251101", 0, 0},
		{"gemini_3_5_flash", "Gemini 3.5 Flash", "gemini-3.5-flash", 0, 0},
		{"gemini_3_1_pro_preview", "Gemini 3.1 Pro Preview", "gemini-3.1-pro-preview", 0, 0},
		{"gemini_3_pro_preview", "Gemini 3 Pro Preview", "gemini-3-pro-preview", 0, 0},
		{"gemini_3_pro_preview_thinking", "Gemini 3 Pro Preview Thinking", "gemini-3-pro-preview-thinking", 0, 0},
		{"gemini_3_flash_preview", "Gemini 3 Flash Preview", "gemini-3-flash-preview", 0, 0},
		{"gemini_2_5_pro", "Gemini 2.5 Pro", "gemini-2.5-pro", 0, 0},
		{"gemini_2_5_flash", "Gemini 2.5 Flash", "gemini-2.5-flash", 0, 0},
		{"gemini_2_5_flash_lite", "Gemini 2.5 Flash Lite", "gemini-2.5-flash-lite", 0, 0},
		{"deepseek_v4_pro", "DeepSeek V4 Pro", "deepseek-v4-pro", 1000000, 393216},
		{"deepseek_v4_flash", "DeepSeek V4 Flash", "deepseek-v4-flash", 128000, 16000},
		{"deepseek_v3_2", "DeepSeek V3.2", "deepseek-v3.2", 0, 0},
		{"deepseek_v3_2_exp", "DeepSeek V3.2 Exp", "deepseek-v3.2-exp", 0, 0},
		{"deepseek_r1_250528", "DeepSeek R1 (250528)", "deepseek-r1-250528", 0, 0},
		{"deepseek_v3_0324", "DeepSeek V3 (0324)", "deepseek-v3-0324", 0, 0},
	}
}

func TextCandidateByID(id string) (TextCandidate, bool) {
	for _, c := range TextCandidates() {
		if c.PublicID == id {
			return c, true
		}
	}
	return TextCandidate{}, false
}

func (c TextCandidate) Alias() TextAlias {
	return TextAlias{PublicID: c.PublicID, DisplayName: c.Name, Provider: domain.ProviderAPIMart, ProviderModelID: c.ModelCode,
		FeatureFlag: FeatureDEVModelSmoke, Readiness: apimartReadiness(),
		PricingKeys: []pricingcatalog.ProductKey{{Operation: domain.OperationTextGenerate, Modality: domain.ModalityText, TextModelID: c.PublicID}}}
}

// KnownPaidTextModels is the adapter allowlist, not a rollout decision. Candidate
// routes still require explicit worker config and server-side DEV catalog access.
func KnownPaidTextModels() []TextAlias {
	out := PaidTextModels()
	for _, c := range TextCandidates() {
		out = append(out, c.Alias())
	}
	return out
}

func DraftTextContract(c TextCandidate) modelcontract.Contract {
	u := modelcontract.Input{Support: modelcontract.Unknown}
	checks := []modelcontract.Check{}
	for _, scenario := range []string{"adapter", "negative", "boundaries", "pricing", "job-lifecycle", "catalog", "live-output"} {
		checks = append(checks, modelcontract.Check{Scenario: "reply/" + scenario, Status: "not_run"})
	}
	return modelcontract.Contract{SchemaVersion: 1, PublicID: c.PublicID, Provider: string(domain.ProviderAPIMart), ProviderModelID: c.ModelCode,
		Revision: TextCandidateCheckedAt, Endpoint: "POST /v1/chat/completions", Status: "draft", Categories: []string{"text", "study-work"},
		Sources: []modelcontract.Source{
			{ID: "chat", URL: APIMartChatDocumentation, CheckedAt: TextCandidateCheckedAt},
			{ID: "metadata", URL: "https://docs.apimart.ai/ru/api-reference/texts/models/list", CheckedAt: TextCandidateCheckedAt},
			{ID: "price", URL: "https://api.apimart.ai/api/pricing/model?model=" + c.ModelCode, CheckedAt: TextCandidateCheckedAt},
		},
		// Pricing's max_input_tokens is not proof of the combined context window.
		Operations: []modelcontract.Operation{{ID: "reply", Kind: "text", Inputs: modelcontract.Inputs{Images: u, Video: u, Audio: u, Documents: u}, Text: &modelcontract.TextOutput{MaxOutputTokens: c.ProviderMaxOutputTokens}}}, Checks: checks}
}
