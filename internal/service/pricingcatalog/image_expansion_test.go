package pricingcatalog

import "testing"

func TestImageExpansionResolutionPrices(t *testing.T) {
	for _, tc := range []struct {
		id, quality    string
		floor, credits int64
	}{
		{"seedream_5_0_flash", "1K", 45000, 30}, {"seedream_5_0_flash", "1.5K", 45000, 30}, {"seedream_5_0_flash", "2K", 90000, 55},
		{"z_image_turbo", "1K", 10000, 10}, {"z_image_turbo", "2K", 10000, 10},
		{"flux_2_max", "1MP", 56000, 35}, {"flux_2_max", "2MP", 80000, 50}, {"flux_2_max", "3MP", 104000, 65}, {"flux_2_max", "4MP", 128000, 80},
		{"flux_2_flex", "1MP", 40000, 25}, {"flux_2_flex", "2MP", 80000, 50}, {"flux_2_flex", "3MP", 120000, 75}, {"flux_2_flex", "4MP", 160000, 100},
		{"qwen_image_3_pro", "1K", 28572, 20}, {"qwen_image_3_pro", "2K", 57144, 35},
	} {
		q, err := ImageCandidateQualityQuote(tc.id, tc.quality)
		if err != nil {
			t.Fatal(err)
		}
		if q.Floor.Amount != tc.floor || q.InternalCredits != tc.credits || q.ImageOutputCount != 1 {
			t.Errorf("%s %s floor=%d credits=%d", tc.id, tc.quality, q.Floor.Amount, q.InternalCredits)
		}
		if _, err := QuoteAPIMartImage(q, "16:9", 0); err != nil {
			t.Fatal(err)
		}
		if _, err := QuoteAPIMartImage(q, "16:9", 1); err == nil {
			t.Fatal("unpriced reference accepted")
		}
		q.Floor.Amount++
		if _, err := QuoteAPIMartImage(q, "16:9", 0); err == nil {
			t.Fatal("mutated floor accepted")
		}
		if _, err := ImageCandidateQualityQuote(tc.id, "4K"); err == nil {
			t.Fatal("unsupported quality priced")
		}
	}
}
