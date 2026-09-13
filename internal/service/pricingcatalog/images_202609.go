package pricingcatalog

import (
	"strings"
	"vk-ai-aggregator/internal/domain"
)

// APIMart Standard after_discount prices checked 2026-09-13:
// https://apimart.ai/pricing and the exact image generation documentation.
// One APIMart credit = $0.10. Retail is cost x3 rounded UP to five credits.
func newAPIMartImageTariffs() []ProductPrice {
	var prices []ProductPrice
	for _, quality := range []string{"2K", "3K", "4K"} {
		prices = append(prices, imageTariff(PublicImageSeedream50Lite, quality, 280000, FloorUnitAPIMartCredits, apimartCreditToInternal, 20))
	}
	// Pro's first reference is free; subsequent inputs are added when quoting.
	for _, quality := range []string{"1K", "1.5K"} {
		prices = append(prices, imageTariff(PublicImageSeedream50Pro, quality, 292500, FloorUnitAPIMartCredits, apimartCreditToInternal, 20))
	}
	prices = append(prices, imageTariff(PublicImageSeedream50Pro, "2K", 585000, FloorUnitAPIMartCredits, apimartCreditToInternal, 40))
	// GPT is a fixed quote per bounded request, not actual-token settlement.
	// Standard token rates: $4/M text input, $24/M image output. The input
	// estimate reserves one token per UTF-8 byte plus 512 overhead tokens.
	// Input images and auto quality remain unavailable. Output token counts
	// come from APIMart's size_quality_tokens table, not pixel-area scaling.
	conversion := mustUnitConversionForFloorUnit(FloorUnitUSDMicros)
	for _, id := range []string{PublicImageGPTImage25Flare, PublicImageGPTImage25Sunburst} {
		for _, resolution := range []string{"1K", "2K", "4K"} {
			for _, quality := range []string{"low", "medium", "high", "xhigh", "max"} {
				key := resolution + "-" + quality
				floor, _ := gptImage25Floor(key, "1:1")
				retail := ((floor*3 + 24999) / 25000) * 5
				prices = append(prices, imageTariff(id, key, floor, FloorUnitUSDMicros, conversion, retail))
			}
		}
	}
	return prices
}

func gptImage25Floor(quality, aspectRatio string) (int64, bool) {
	resolution, detail, ok := strings.Cut(quality, "-")
	if !ok || (resolution != "1K" && resolution != "2K" && resolution != "4K") {
		return 0, false
	}
	key := aspectRatio
	if resolution != "1K" {
		key += "@" + strings.ToLower(resolution)
	}
	tokens, ok := gptImage25OutputTokens[key]
	if !ok {
		return 0, false
	}
	for index, name := range []string{"low", "medium", "high", "xhigh", "max"} {
		if detail == name {
			return int64(domain.GPTImage25MaxPromptBytes+512)*4 + tokens[index]*24, true
		}
	}
	return 0, false
}

// QuoteAPIMartImage binds the selected ratio/reference count to a per-output
// price. Callers multiply this quote by output_count exactly once afterwards.
// Custom runtime floors fail closed instead of silently overriding operators'
// tariff decisions with a different public rate table.
func QuoteAPIMartImage(s PricingSnapshot, aspectRatio string, references int) (PricingSnapshot, error) {
	if !IsBoundedAPIMartImage(s.Key.ImageModelID) {
		return s, nil
	}
	if !s.Valid() || s.ImageOutputCount != 1 || references < 0 {
		return PricingSnapshot{}, ErrInvalidSnapshot
	}
	var base ProductPrice
	for _, price := range newAPIMartImageTariffs() {
		if price.Key == s.Key {
			base = price
			break
		}
	}
	if !base.Enabled || s.Floor != base.Floor || s.UnitConversion != base.UnitConversion || s.Multiplier != base.Multiplier {
		return PricingSnapshot{}, ErrPriceNotFound
	}
	if IsGPTImage25(s.Key.ImageModelID) {
		if references != 0 {
			return PricingSnapshot{}, ErrInvalidSnapshot
		}
		floor, ok := gptImage25Floor(s.Key.Quality, aspectRatio)
		if !ok {
			return PricingSnapshot{}, ErrPriceNotFound
		}
		s.Floor.Amount = floor
		s.ImageAspectRatio = aspectRatio
	} else if s.Key.ImageModelID == PublicImageSeedream50Pro {
		if references > 10 {
			return PricingSnapshot{}, ErrInvalidSnapshot
		}
		// https://apimart.ai/api/pricing/model?model=seedream-5-0-pro:
		// input_image_price=$0.0024375 before factor 0.8, first input free.
		if references > 1 {
			s.Floor.Amount += int64(references-1) * 19500
		}
	} else if references > 14 {
		return PricingSnapshot{}, ErrInvalidSnapshot
	}
	s.ImageReferenceCount = references
	return finalizeImageQuote(s)
}

// GPT text input is shared by the batch; only image output tokens scale by n.
func ScaleGPTImage25Quote(s PricingSnapshot, count int) (PricingSnapshot, error) {
	if !s.Valid() || !IsGPTImage25(s.Key.ImageModelID) || s.ImageOutputCount != 1 || count < 1 || count > 4 {
		return PricingSnapshot{}, ErrInvalidSnapshot
	}
	outputFloor := s.Floor.Amount - s.ImageTextInputFloor
	if outputFloor > (1<<63-1-s.ImageTextInputFloor)/int64(count) {
		return PricingSnapshot{}, ErrInvalidFloor
	}
	s.Floor.Amount = s.ImageTextInputFloor + outputFloor*int64(count)
	s.ImageOutputCount = count
	return finalizeImageQuote(s)
}

func finalizeImageQuote(s PricingSnapshot) (PricingSnapshot, error) {
	credits, err := CalculateInternalCredits(s.Floor, s.UnitConversion, DefaultMultiplier(), SafetyCaps{})
	if err != nil {
		return PricingSnapshot{}, err
	}
	credits = ((credits + 4) / 5) * 5
	s.InternalCredits, s.InternalCreditCap, s.DefaultDisplayCredits = credits, credits, credits
	s.FloorAmountCap = s.Floor.Amount
	s.Multiplier = exactRetailMultiplier(s.Floor.Amount, s.UnitConversion, credits)
	if !s.Valid() {
		return PricingSnapshot{}, ErrInvalidSnapshot
	}
	return s, nil
}

func IsBoundedAPIMartImage(modelID string) bool {
	return IsGPTImage25(modelID) || modelID == PublicImageSeedream50Lite || modelID == PublicImageSeedream50Pro
}

func IsGPTImage25(modelID string) bool {
	return modelID == PublicImageGPTImage25Flare || modelID == PublicImageGPTImage25Sunburst
}
