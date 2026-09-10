package textgeneration

import (
	"testing"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

func TestNamedTextMustBeReadyAndPriced(t *testing.T) {
	prices, _ := pricingcatalog.NewStaticCatalog()
	allowed := Models([]string{"gpt_5_5", "claude_opus_4_7", "gemini_3_1_pro"}, prices)
	for _, tc := range []struct {
		id      string
		credits int64
	}{{"gpt_5_5", 20}, {"claude_opus_4_7", 20}, {"gemini_3_1_pro", 10}} {
		m, s, err := Resolve(tc.id, "Synthetic", allowed, prices)
		if err != nil || m.Provider != domain.ProviderKIE || s.InternalCredits != tc.credits {
			t.Fatalf("%s: %v %d", tc.id, err, s.InternalCredits)
		}
		if _, _, err := Resolve(tc.id, "Synthetic", nil, prices); err == nil {
			t.Fatal("disabled accepted")
		}
		if _, _, err := Resolve(tc.id, "Synthetic", allowed, nil); err == nil {
			t.Fatal("unpriced accepted")
		}
	}
	for _, id := range []string{"gpt-5-5", "GPT-5.5", "unknown"} {
		if _, _, err := Resolve(id, "Synthetic", allowed, prices); err == nil {
			t.Fatal("private alias accepted")
		}
	}
	m, s, err := Resolve("", "Synthetic", nil, nil)
	if err != nil || m.ModelID != "chatgpt" || s.Valid() {
		t.Fatal("default changed")
	}
}
