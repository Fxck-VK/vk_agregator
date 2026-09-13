package pricingcatalog

import (
	"testing"
	"vk-ai-aggregator/internal/domain"
)

func TestTextTariffsAreBoundedAndSeparate(t *testing.T) {
	c, err := NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"gpt_5_5", "claude_opus_4_7", "gemini_3_1_pro"} {
		k := ProductKey{Operation: domain.OperationTextGenerate, Modality: domain.ModalityText, TextModelID: id}
		s, err := c.Snapshot(k)
		if err != nil || !s.Valid() || s.InternalCredits <= 0 || s.InternalCredits%5 != 0 {
			t.Fatalf("%s: %+v %v", id, s, err)
		}
		k.ImageModelID = "injected"
		if k.Valid() {
			t.Fatal("mixed dimensions accepted")
		}
	}
}
