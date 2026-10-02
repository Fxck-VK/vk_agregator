package pricingcatalog

import "vk-ai-aggregator/internal/domain"

// Checked 2026-09-30 in the user's APIMart pricing page and public pricing API:
// https://api.apimart.ai/api/pricing/model?model=<native-model-id>
// Gold paid_price/resolution_paid_prices, rounded UP to USD micros.
// Seedream Flash's pricing endpoint has no resolution breakdown and conflicts
// with its generation docs. Reserve the higher documented $0.045/$0.09 until
// authorized live billing evidence resolves the discrepancy (no invented discount).
// https://docs.apimart.ai/ru/api-reference/images/seedream-5-0-flash/generation
func imageExpansionFloor(model, quality string) int64 {
	prices := map[string]map[string]int64{
		"seedream_5_0_flash": {"1K": 45000, "1.5K": 45000, "2K": 90000},
		"z_image_turbo":      {"1K": 10000, "2K": 10000},
		"flux_2_max":         {"1MP": 56000, "2MP": 80000, "3MP": 104000, "4MP": 128000},
		"flux_2_flex":        {"1MP": 40000, "2MP": 80000, "3MP": 120000, "4MP": 160000},
		"qwen_image_3_pro":   {"1K": 28572, "2K": 57144},
	}
	return prices[model][quality]
}

func IsImageExpansion(model string) bool {
	switch model {
	case "seedream_5_0_flash", "z_image_turbo", "flux_2_max", "flux_2_flex", "qwen_image_3_pro":
		return true
	default:
		return false
	}
}

// ImageCandidateQualityQuote pins the selected output tier to a single image.
// References, rewriting, layers and arbitrary pixels have no enabled price.
func ImageCandidateQualityQuote(model, quality string) (PricingSnapshot, error) {
	if !IsImageExpansion(model) {
		q, err := ImageCandidateQuote(model)
		if err != nil {
			return PricingSnapshot{}, err
		}
		want := "standard"
		if model == "nano_banana" {
			want = "1K"
		}
		if quality != want {
			return PricingSnapshot{}, ErrPriceNotFound
		}
		q.Key.Quality = quality
		return q, nil
	}
	floor := imageExpansionFloor(model, quality)
	if floor == 0 {
		return PricingSnapshot{}, ErrPriceNotFound
	}
	return candidateUSDQuote(ProductKey{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ImageModelID: model, Quality: quality}, floor)
}

func quoteImageExpansion(s PricingSnapshot, references int) (PricingSnapshot, error) {
	base, err := ImageCandidateQualityQuote(s.Key.ImageModelID, s.Key.Quality)
	if err != nil || !s.Valid() || references != 0 || s.ImageOutputCount != 1 || s.Floor != base.Floor || s.UnitConversion != base.UnitConversion || s.Multiplier != base.Multiplier {
		return PricingSnapshot{}, ErrPriceNotFound
	}
	return s, nil
}
