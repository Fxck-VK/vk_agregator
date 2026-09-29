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
	CheckedAt                                       string
	ProviderMaxInputTokens, ProviderMaxOutputTokens int
}

func TextCandidates() []TextCandidate {
	return []TextCandidate{
		{"gpt_5", "GPT-5", "gpt-5", TextCandidateCheckedAt, 128000, 16000},
		{"gpt_5_1", "GPT-5.1", "gpt-5.1", TextCandidateCheckedAt, 128000, 16000},
		{"gpt_5_chat_latest", "GPT-5 Chat Latest", "gpt-5-chat-latest", TextCandidateCheckedAt, 0, 0},
		{"gpt_5_mini", "GPT-5 Mini", "gpt-5-mini", TextCandidateCheckedAt, 128000, 16000},
		{"claude_opus_4_6", "Claude Opus 4.6", "claude-opus-4-6", TextCandidateCheckedAt, 1000000, 1000000},
		{"claude_sonnet_4_6", "Claude Sonnet 4.6", "claude-sonnet-4-6", TextCandidateCheckedAt, 128000, 16000},
		{"claude_opus_4_5_20251101", "Claude Opus 4.5 (20251101)", "claude-opus-4-5-20251101", TextCandidateCheckedAt, 0, 0},
		{"gemini_3_5_flash", "Gemini 3.5 Flash", "gemini-3.5-flash", TextCandidateCheckedAt, 0, 0},
		{"gemini_3_1_pro_preview", "Gemini 3.1 Pro Preview", "gemini-3.1-pro-preview", TextCandidateCheckedAt, 0, 0},
		{"gemini_3_pro_preview", "Gemini 3 Pro Preview", "gemini-3-pro-preview", TextCandidateCheckedAt, 0, 0},
		{"gemini_3_pro_preview_thinking", "Gemini 3 Pro Preview Thinking", "gemini-3-pro-preview-thinking", TextCandidateCheckedAt, 0, 0},
		{"gemini_3_flash_preview", "Gemini 3 Flash Preview", "gemini-3-flash-preview", TextCandidateCheckedAt, 0, 0},
		{"gemini_2_5_pro", "Gemini 2.5 Pro", "gemini-2.5-pro", TextCandidateCheckedAt, 0, 0},
		{"gemini_2_5_flash", "Gemini 2.5 Flash", "gemini-2.5-flash", TextCandidateCheckedAt, 0, 0},
		{"gemini_2_5_flash_lite", "Gemini 2.5 Flash Lite", "gemini-2.5-flash-lite", TextCandidateCheckedAt, 0, 0},
		{"deepseek_v4_pro", "DeepSeek V4 Pro", "deepseek-v4-pro", TextCandidateCheckedAt, 1000000, 393216},
		{"deepseek_v4_flash", "DeepSeek V4 Flash", "deepseek-v4-flash", TextCandidateCheckedAt, 128000, 16000},
		{"deepseek_v3_2", "DeepSeek V3.2", "deepseek-v3.2", TextCandidateCheckedAt, 0, 0},
		{"deepseek_v3_2_exp", "DeepSeek V3.2 Exp", "deepseek-v3.2-exp", TextCandidateCheckedAt, 0, 0},
		{"deepseek_r1_250528", "DeepSeek R1 (250528)", "deepseek-r1-250528", TextCandidateCheckedAt, 0, 0},
		{"deepseek_v3_0324", "DeepSeek V3 (0324)", "deepseek-v3-0324", TextCandidateCheckedAt, 0, 0},
		{"gpt_6_sol", "GPT-6 Sol", "gpt-6-sol", "2026-09-29", 922000, 12800},
		{"gpt_6_luna", "GPT-6 Luna", "gpt-6-luna", "2026-09-29", 922000, 12800},
		{"gpt_5_4", "GPT-5.4", "gpt-5.4", "2026-09-29", 1000000, 12800},
		{"gpt_5_3_codex", "GPT-5.3 Codex", "gpt-5.3-codex", "2026-09-29", 0, 0},
		{"gpt_5_2", "GPT-5.2", "gpt-5.2", "2026-09-29", 128000, 16000},
		{"claude_opus_5_5", "Claude Opus 5.5", "claude-opus-5-5", "2026-09-29", 128000, 16000},
		{"claude_haiku_4_5_20251001", "Claude Haiku 4.5 (20251001)", "claude-haiku-4-5-20251001", "2026-09-29", 128000, 16000},
		{"kimi_k3", "Kimi K3", "kimi-k3", "2026-09-29", 1000000, 104857},
		{"qwen_3_8_max", "Qwen 3.8 Max", "qwen3.8-max", "2026-09-29", 983616, 131072},
		{"qwen_3_7_flash", "Qwen 3.7 Flash", "qwen3.7-flash", "2026-09-29", 983616, 131072},
		{"grok_4_5", "Grok 4.5", "grok-4.5", "2026-09-29", 0, 0},
		{"grok_4_6", "Grok 4.6", "grok-4.6", "2026-09-29", 500000, 12800},
		{"grok_4_7", "Grok 4.7", "grok-4.7", "2026-09-29", 500000, 12800},
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

// Qwen's model-specific guide bounds thinking and answer tokens together through
// Responses max_output_tokens. Tools and explicit cache creation are not enabled.
func (c TextCandidate) EndpointPath() string {
	if c.ModelCode == "qwen3.8-max" {
		return "/v1/responses"
	}
	return "/v1/chat/completions"
}

func DraftTextContract(c TextCandidate) modelcontract.Contract {
	u := modelcontract.Input{Support: modelcontract.Unknown}
	checks := []modelcontract.Check{}
	for _, scenario := range []string{"adapter", "negative", "boundaries", "pricing", "job-lifecycle", "catalog", "live-output"} {
		checks = append(checks, modelcontract.Check{Scenario: "reply/" + scenario, Status: "not_run"})
	}
	sources := []modelcontract.Source{
		{ID: "chat", URL: APIMartChatDocumentation, CheckedAt: c.CheckedAt},
		{ID: "metadata", URL: "https://docs.apimart.ai/ru/api-reference/texts/models/list", CheckedAt: c.CheckedAt},
		{ID: "price", URL: "https://api.apimart.ai/api/pricing/model?model=" + c.ModelCode, CheckedAt: c.CheckedAt},
	}
	if c.ModelCode == "qwen3.8-max" {
		sources = append(sources,
			modelcontract.Source{ID: "model-guide", URL: "https://docs.apimart.ai/ru/api-reference/texts/qwen3.8-max/guide", CheckedAt: c.CheckedAt},
			modelcontract.Source{ID: "responses", URL: "https://docs.apimart.ai/ru/api-reference/texts/openai/responses", CheckedAt: c.CheckedAt})
	}
	return modelcontract.Contract{SchemaVersion: 1, PublicID: c.PublicID, Provider: string(domain.ProviderAPIMart), ProviderModelID: c.ModelCode,
		Revision: c.CheckedAt, Endpoint: "POST " + c.EndpointPath(), Status: "draft", Categories: []string{"text", "study-work"}, Sources: sources,
		// Pricing's max_input_tokens is not proof of the combined context window.
		Operations: []modelcontract.Operation{{ID: "reply", Kind: "text", Inputs: modelcontract.Inputs{Images: u, Video: u, Audio: u, Documents: u}, Text: &modelcontract.TextOutput{MaxOutputTokens: c.ProviderMaxOutputTokens}}}, Checks: checks}
}
