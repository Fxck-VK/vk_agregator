package pricingcatalog

import "vk-ai-aggregator/internal/domain"

const (
	TextMaxInputTokens  = domain.PaidTextMaxInputTokens
	TextMaxOutputTokens = domain.PaidTextMaxOutputTokens
)

// Text tariffs are fixed per bounded reply, not metered settlement. Provider
// list rates checked 2026-09-09 at https://kie.ai/pricing and
// https://apimart.ai/ru/pricing (USD per million).
// Input reserves include the full context and trusted messages. No cache
// discount is assumed; reasoning must fit inside the output token cap.
func textTariffs() []ProductPrice {
	var out []ProductPrice
	for _, rate := range []struct {
		id            string
		input, output int64
	}{
		{"gpt_5_5", 1_400_000, 8_400_000},
		{"claude_opus_4_7", 1_425_000, 7_150_000},
		{"gemini_3_1_pro", 500_000, 3_500_000},
		{"claude_opus_4_8", 2_000_000, 10_000_000},
		{"gpt_5_6_terra", 560_000, 3_360_000},
		{"gpt_6_astra", 2_800_000, 14_000_000},
		{"claude_opus_5", 2_000_000, 10_000_000},
		{"gemini_3_7_flash", 225_000, 1_125_000},
		{"claude_fable_5_1", 8_000_000, 40_000_000},
		{"claude_fable_5", 4_000_000, 20_000_000},
		{"gemini_3_6_flash", 225_000, 1_125_000},
	} {
		floor := (rate.input*TextMaxInputTokens + rate.output*TextMaxOutputTokens + 999999) / 1000000
		retail := ((floor*3 + 24999) / 25000) * 5
		conversion := mustUnitConversionForFloorUnit(FloorUnitUSDMicros)
		out = append(out, ProductPrice{Key: ProductKey{Operation: domain.OperationTextGenerate, Modality: domain.ModalityText, TextModelID: rate.id},
			Version: StaticCatalogVersion, Source: StaticSource, Floor: PriceFloor{Amount: floor, Unit: FloorUnitUSDMicros},
			Multiplier: exactRetailMultiplier(floor, conversion, retail), UnitConversion: conversion,
			Caps: SafetyCaps{InternalCreditCap: retail, FloorAmountCap: floor}, Enabled: true})
	}
	return out
}
