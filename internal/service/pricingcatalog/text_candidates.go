package pricingcatalog

import "vk-ai-aggregator/internal/domain"

// Default-group effective USD rates, without membership/cache discounts, checked
// 2026-09-28 at https://api.apimart.ai/api/pricing/model?model=<native-id>.
// Public source facts are retained in testdata/apimart-text-20260928.json.
// These prices enter runtime only through the explicit DEV supplement.
func TextCandidateQuote(model string) (PricingSnapshot, error) {
	for _, r := range []struct {
		id            string
		input, output int64
	}{
		{"gpt_5", 1000000, 8000000},
		{"gpt_5_1", 1000000, 8000000},
		{"gpt_5_chat_latest", 1000000, 8000000},
		{"gpt_5_mini", 200000, 1600000},
		{"claude_opus_4_6", 4000000, 8000000},
		{"claude_sonnet_4_6", 2400000, 12000000},
		{"claude_opus_4_5_20251101", 4000000, 20000000},
		{"gemini_3_5_flash", 1200000, 7200000},
		{"gemini_3_1_pro_preview", 1600000, 9600000},
		{"gemini_3_pro_preview", 1600000, 9600000},
		{"gemini_3_pro_preview_thinking", 1600000, 9600000},
		{"gemini_3_flash_preview", 400000, 2400000},
		{"gemini_2_5_pro", 1000000, 8000000},
		{"gemini_2_5_flash", 240000, 1999920},
		{"gemini_2_5_flash_lite", 80000, 320000},
		{"deepseek_v4_pro", 1028572, 3085715},
		{"deepseek_v4_flash", 342857, 1028572},
		{"deepseek_v3_2", 205600, 308400},
		{"deepseek_v3_2_exp", 205600, 308400},
		{"deepseek_r1_250528", 448000, 1792000},
		{"deepseek_v3_0324", 205600, 822400},
	} {
		if r.id != model {
			continue
		}
		floor := (r.input*TextMaxInputTokens + r.output*TextMaxOutputTokens + 999999) / 1000000
		q, err := candidateUSDQuote(ProductKey{Operation: domain.OperationTextGenerate, Modality: domain.ModalityText, TextModelID: model}, floor)
		q.Version = 20260928
		return q, err
	}
	return PricingSnapshot{}, ErrPriceNotFound
}
