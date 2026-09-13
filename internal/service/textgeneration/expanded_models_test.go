package textgeneration

import (
	"testing"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

func TestExpandedModelsResolveExactProviderAndPrice(t *testing.T) {
	prices, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id, code string
		provider domain.ProviderName
		credits  int64
	}{
		{"claude_opus_4_8", "claude-opus-4-8", domain.ProviderKIE, 25},
		{"gpt_5_6_terra", "gpt-5-6-terra", domain.ProviderKIE, 10},
		{"gpt_6_astra", "gpt-6-astra", domain.ProviderKIE, 35},
		{"claude_opus_5", "claude-opus-5", domain.ProviderKIE, 25},
		{"gemini_3_7_flash", "gemini-3-7-flash-openai", domain.ProviderKIE, 5},
		{"claude_fable_5_1", "claude-fable-5.1", domain.ProviderAPIMart, 90},
		{"claude_fable_5", "claude-fable-5", domain.ProviderKIE, 45},
		{"gemini_3_6_flash", "gemini-3-6-flash-openai", domain.ProviderKIE, 5},
	} {
		t.Run(tc.id, func(t *testing.T) {
			allowed := Models([]string{tc.id}, prices)
			m, snapshot, err := Resolve(tc.id, "Synthetic", allowed, prices)
			if err != nil || m.Provider != tc.provider || m.ModelCode != tc.code || snapshot.InternalCredits != tc.credits {
				t.Fatalf("route/price: %s %s %d %v", m.Provider, m.ModelCode, snapshot.InternalCredits, err)
			}
			if _, _, err := Resolve(tc.id, "Synthetic", nil, prices); err == nil {
				t.Fatal("disabled model accepted")
			}
			if _, _, err := Resolve(tc.code, "Synthetic", allowed, prices); err == nil {
				t.Fatal("provider ID accepted")
			}
		})
	}
}
