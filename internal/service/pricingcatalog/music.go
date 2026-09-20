package pricingcatalog

import "vk-ai-aggregator/internal/domain"

// APIMart public action tariff, checked 2026-09-16:
// https://api.apimart.ai/api/pricing/model?model=suno
// Values below are USD micros AFTER the published 20% discount. Quotes use
// the existing $0.005 internal-credit conversion, x3 rounded up to five.
// Candidate prices are deliberately not inserted into StaticProductPrices:
// pricing evidence alone does not admit a model or an operation.
func MusicCandidateQuote(model, action string, max bool) (PricingSnapshot, error) {
	if model != "suno_v6" && model != "suno_v6_wild" && model != "suno_v6_mini" {
		return PricingSnapshot{}, ErrPriceNotFound
	}
	var floor int64
	maxAllowed := false
	switch action {
	case "generate", "extend", "cover", "upload_cover", "upload_extend", "add_vocals", "add_instrumental", "add_stem", "mashup", "replace_section", "sample":
		floor, maxAllowed = 50000, true
	case "inspo":
		floor, maxAllowed = 68000, true
	case "sounds":
		floor = 9600
	case "lyrics", "remove_section", "crop", "fade_in", "fade_out":
		floor = 8000
	case "upsample_tags", "upload", "persona", "concat", "generate_video":
		floor = 4000
	case "create_model":
		floor = 960000
	case "voice":
		floor = 16000
	case "remaster", "midi":
		floor = 50000
	case "stems":
		floor = 100000
	case "stems_all":
		floor = 240000
	case "adjust_speed":
		floor = 24000
	case "aligned_lyrics", "bpm":
		floor = 800
	case "export":
		floor = 1600
	default:
		return PricingSnapshot{}, ErrPriceNotFound
	}
	if max {
		if !maxAllowed {
			return PricingSnapshot{}, ErrPriceNotFound
		}
		floor *= 2
	}
	return candidateUSDQuote(ProductKey{Operation: domain.OperationAudioMusic, Modality: domain.ModalityAudio, AudioModelID: model, AudioAction: action, AudioMax: max}, floor)
}

func candidateUSDQuote(key ProductKey, amount int64) (PricingSnapshot, error) {
	floor := PriceFloor{Amount: amount, Unit: FloorUnitUSDMicros}
	conversion := mustUnitConversionForFloorUnit(FloorUnitUSDMicros)
	credits, err := CalculateInternalCredits(floor, conversion, DefaultMultiplier(), SafetyCaps{})
	if err != nil {
		return PricingSnapshot{}, err
	}
	credits = ((credits + 4) / 5) * 5
	return (ProductPrice{Key: key, Version: StaticCatalogVersion, Source: StaticSource, Enabled: true, Floor: floor, UnitConversion: conversion, Multiplier: exactRetailMultiplier(amount, conversion, credits), Caps: SafetyCaps{InternalCreditCap: credits, FloorAmountCap: amount}}).Snapshot()
}
