package pricingcatalog

import "vk-ai-aggregator/internal/domain"

// Default-group effective USD rates, without membership/cache discounts, checked
// 2026-09-28/29 at https://api.apimart.ai/api/pricing/model?model=<native-id>.
// Public source facts are retained in the dated testdata/apimart-text-*.json.
// Tiered rates apply below their first threshold (the reply cap is 8192 input tokens).
// These prices enter runtime only through the explicit DEV supplement.
func TextCandidateQuote(model string) (PricingSnapshot, error) {
	for _, r := range []struct {
		id            string
		version       int
		input, output int64
	}{
		{"gpt_5", 20260928, 1000000, 8000000},
		{"gpt_5_1", 20260928, 1000000, 8000000},
		{"gpt_5_chat_latest", 20260928, 1000000, 8000000},
		{"gpt_5_mini", 20260928, 200000, 1600000},
		{"claude_opus_4_6", 20260928, 4000000, 8000000},
		{"claude_sonnet_4_6", 20260928, 2400000, 12000000},
		{"claude_opus_4_5_20251101", 20260928, 4000000, 20000000},
		{"gemini_3_5_flash", 20260928, 1200000, 7200000},
		{"gemini_3_1_pro_preview", 20260928, 1600000, 9600000},
		{"gemini_3_pro_preview", 20260928, 1600000, 9600000},
		{"gemini_3_pro_preview_thinking", 20260928, 1600000, 9600000},
		{"gemini_3_flash_preview", 20260928, 400000, 2400000},
		{"gemini_2_5_pro", 20260928, 1000000, 8000000},
		{"gemini_2_5_flash", 20260928, 240000, 1999920},
		{"gemini_2_5_flash_lite", 20260928, 80000, 320000},
		{"deepseek_v4_pro", 20260928, 1028572, 3085715},
		{"deepseek_v4_flash", 20260928, 342857, 1028572},
		{"deepseek_v3_2", 20260928, 205600, 308400},
		{"deepseek_v3_2_exp", 20260928, 205600, 308400},
		{"deepseek_r1_250528", 20260928, 448000, 1792000},
		{"deepseek_v3_0324", 20260928, 205600, 822400},
		{"gpt_6_sol", 20260929, 1600000, 8000000},
		{"gpt_6_luna", 20260929, 80000, 400000},
		{"gpt_5_4", 20260929, 2000000, 12000000},
		{"gpt_5_3_codex", 20260929, 1400000, 11200000},
		{"gpt_5_2", 20260929, 1400000, 11200000},
		{"claude_opus_5_5", 20260929, 3200000, 16000000},
		{"claude_haiku_4_5_20251001", 20260929, 800000, 4000000},
		{"kimi_k3", 20260929, 2400000, 12000000},
		{"qwen_3_8_max", 20260929, 1371429, 4114286},
		{"qwen_3_7_flash", 20260929, 22857, 91429},
		{"grok_4_5", 20260929, 1600000, 3200000},
		{"grok_4_6", 20260929, 1600000, 4800000},
		{"grok_4_7", 20260929, 1600000, 4800000},
	} {
		if r.id != model {
			continue
		}
		floor := (r.input*TextMaxInputTokens + r.output*TextMaxOutputTokens + 999999) / 1000000
		q, err := candidateUSDQuote(ProductKey{Operation: domain.OperationTextGenerate, Modality: domain.ModalityText, TextModelID: model}, floor)
		q.Version = r.version
		return q, err
	}
	return PricingSnapshot{}, ErrPriceNotFound
}
