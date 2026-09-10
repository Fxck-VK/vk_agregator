package pricingcatalog

import "vk-ai-aggregator/internal/domain"

const (
	StaticCatalogVersion = 11

	PublicImageNanoBanana2   = "nano_banana_2"
	PublicImageNanoBananaPro = "nano_banana_pro"
	PublicImageGPTImage2     = "gpt_image_2"
	PublicImageQwenImage3    = "qwen_image_3"
	PublicImageGrokImage15   = "grok_image_1_5"
	PublicImageGrokImage20   = "grok_image_2_0"
	PublicImageMidjourneyV7  = "midjourney_v7"
	PublicImageFlux2Pro      = "flux_2_pro"
	PublicImageSeedream45    = "seedream_4_5"

	ImageQuality1K       = "1K"
	ImageQuality2K       = "2K"
	ImageQuality4K       = "4K"
	ImageQualityStandard = "standard"

	VideoResolution720p  = "720p"
	VideoResolution480p  = "480p"
	VideoResolution768p  = "768p"
	VideoResolution1080p = "1080p"
)

var (
	// Approved provider floors imply these exact internal-credit conversions:
	// PoYo credit = $0.005, APIMart credit = $0.10, Runway credit = $0.01;
	// one internal generation credit is treated as $0.005 for catalog math.
	poyoCreditToInternal    = mustUnitConversionForFloorUnit(FloorUnitPoYoCredits)
	apimartCreditToInternal = mustUnitConversionForFloorUnit(FloorUnitAPIMartCredits)
	runwayCreditToInternal  = mustUnitConversionForFloorUnit(FloorUnitRunwayCredits)
)

// DisabledProductPrice records an approved tariff that is intentionally absent
// from the active static catalog until its public dimensions are exact.
type DisabledProductPrice struct {
	Key            ProductKey
	Floor          PriceFloor
	UnitConversion UnitConversion
	Reason         string
	TargetPR       string
}

// NewStaticCatalog returns the initial code-backed generation pricing catalog.
// It is not wired into runtime consumers until PR-03.
func NewStaticCatalog() (*Catalog, error) {
	return NewCatalog(StaticProductPrices())
}

// StaticProductPrices returns enabled, exact generation tariffs.
func StaticProductPrices() []ProductPrice {
	prices := []ProductPrice{
		// APIMart FLUX.2 Pro text-to-image, 2026-09-09: https://apimart.ai/zh/model/flux-2
		// $0.024/$0.036/$0.048/$0.060 per output; x3 rounded up to 5 internal credits.
		imageTariff(PublicImageFlux2Pro, "1MP", 240000, FloorUnitAPIMartCredits, apimartCreditToInternal, 15),
		imageTariff(PublicImageFlux2Pro, "2MP", 360000, FloorUnitAPIMartCredits, apimartCreditToInternal, 25),
		imageTariff(PublicImageFlux2Pro, "3MP", 480000, FloorUnitAPIMartCredits, apimartCreditToInternal, 30),
		imageTariff(PublicImageFlux2Pro, "4MP", 600000, FloorUnitAPIMartCredits, apimartCreditToInternal, 40),
		// APIMart Imagine V7, 2026-09-09: https://apimart.ai/pricing
		// Per call, all output tiles included. Provider cost x3 rounded up to 5.
		imageTariff(PublicImageMidjourneyV7, "relax", 450400, FloorUnitAPIMartCredits, apimartCreditToInternal, 30),
		imageTariff(PublicImageMidjourneyV7, "fast", 550400, FloorUnitAPIMartCredits, apimartCreditToInternal, 35),
		imageTariff(PublicImageMidjourneyV7, "turbo", 1000000, FloorUnitAPIMartCredits, apimartCreditToInternal, 60),
		imageTariff(PublicImageNanoBanana2, ImageQuality1K, 5_000_000, FloorUnitPoYoCredits, poyoCreditToInternal, 50),
		imageTariff(PublicImageNanoBanana2, ImageQuality2K, 8_000_000, FloorUnitPoYoCredits, poyoCreditToInternal, 60),
		imageTariff(PublicImageNanoBanana2, ImageQuality4K, 12_000_000, FloorUnitPoYoCredits, poyoCreditToInternal, 70),
		imageTariff(PublicImageGPTImage2, ImageQuality1K, 60_000, FloorUnitAPIMartCredits, apimartCreditToInternal, 40),
		imageTariff(PublicImageGPTImage2, ImageQuality2K, 120_000, FloorUnitAPIMartCredits, apimartCreditToInternal, 50),
		imageTariff(PublicImageGPTImage2, ImageQuality4K, 180_000, FloorUnitAPIMartCredits, apimartCreditToInternal, 60),
		imageTariff(PublicImageNanoBananaPro, ImageQuality1K, 400_000, FloorUnitAPIMartCredits, apimartCreditToInternal, 50),
		imageTariff(PublicImageNanoBananaPro, ImageQuality2K, 500_000, FloorUnitAPIMartCredits, apimartCreditToInternal, 60),
		imageTariff(PublicImageNanoBananaPro, ImageQuality4K, 500_000, FloorUnitAPIMartCredits, apimartCreditToInternal, 70),
		fixedInternalImageTariff(PublicImageSeedream45, ImageQuality2K, 10, 30),
		fixedInternalImageTariff(PublicImageSeedream45, ImageQuality4K, 15, 40),
		qwenImage3Tariff(ImageQuality1K),
		qwenImage3Tariff(ImageQuality2K),
		grokImageTariff(PublicImageGrokImage15),
		grokImageTariff(PublicImageGrokImage20),
	}

	for _, resolution := range []string{VideoResolution720p, VideoResolution1080p} {
		for _, duration := range []int{5, 10} {
			prices = append(prices, videoTariff(
				domain.VideoRouteKlingO3Standard,
				resolution,
				duration,
				int64(duration)*10_000_000,
				FloorUnitPoYoCredits,
				poyoCreditToInternal,
				int64(duration)*40,
			))
		}
	}
	for _, duration := range []int{5, 10} {
		prices = append(prices, videoTariff(
			domain.VideoRouteSeedance20Fast,
			VideoResolution720p,
			duration,
			int64(duration)*28_000_000,
			FloorUnitPoYoCredits,
			poyoCreditToInternal,
			int64(duration)*60,
		))
	}
	for _, duration := range []int{5, 10} {
		prices = append(prices, videoTariff(
			domain.VideoRouteRunwayGen4Turbo,
			VideoResolution720p,
			duration,
			int64(duration)*5_000_000,
			FloorUnitRunwayCredits,
			runwayCreditToInternal,
			int64(duration)*60,
		))
	}
	for _, resolution := range []string{VideoResolution720p, VideoResolution1080p} {
		prices = append(prices,
			videoTariff(domain.VideoRouteRunwayGen45, resolution, 5, 75_000_000, FloorUnitPoYoCredits, poyoCreditToInternal, 450),
			videoTariff(domain.VideoRouteRunwayGen45, resolution, 10, 150_000_000, FloorUnitPoYoCredits, poyoCreditToInternal, 900),
		)
	}

	// APIMart Seedance 2.5 preauthorization rates checked 2026-09-09:
	// https://apimart.ai/zh/model/doubao-seedance-2-5
	// Text/image inputs only. Provider settles actual tokens; our user quote is
	// fixed at this estimate x3, rounded up once to five internal credits.
	for _, resolution := range []string{VideoResolution480p, VideoResolution720p, VideoResolution1080p} {
		rate := map[string]int64{VideoResolution480p: 960800, VideoResolution720p: 2160000, VideoResolution1080p: 3848800}[resolution]
		for _, duration := range []int{5, 10, 15, 30} {
			floor := rate * int64(duration)
			retail := ((floor*20*3 + 5*MinorUnitsPerCredit - 1) / (5 * MinorUnitsPerCredit)) * 5
			prices = append(prices, videoTariff(domain.VideoRouteSeedance25, resolution, duration, floor, FloorUnitAPIMartCredits, apimartCreditToInternal, retail))
		}
	}
	return append(prices, textTariffs()...)
}

// DisabledStaticProductPrices returns approved tariffs kept fail-closed because
// their active public price key is not exact enough yet.
func DisabledStaticProductPrices() []DisabledProductPrice {
	reason := "APIMart Hailuo floor is approved by resolution, but duration basis is not confirmed"
	target := "PR-03 Prompt 2/3: keep Hailuo fail-closed until per-duration floor semantics are resolved"
	disabled := []DisabledProductPrice{
		disabledVideoTariff(domain.VideoRouteHailuo23Fast, VideoResolution768p, 6, 248_000, FloorUnitAPIMartCredits, apimartCreditToInternal, reason, target),
		disabledVideoTariff(domain.VideoRouteHailuo23Fast, VideoResolution768p, 10, 248_000, FloorUnitAPIMartCredits, apimartCreditToInternal, reason, target),
		disabledVideoTariff(domain.VideoRouteHailuo23Fast, VideoResolution1080p, 6, 424_000, FloorUnitAPIMartCredits, apimartCreditToInternal, reason, target),
		disabledVideoTariff(domain.VideoRouteHailuo23Standard, VideoResolution768p, 6, 488_000, FloorUnitAPIMartCredits, apimartCreditToInternal, reason, target),
		disabledVideoTariff(domain.VideoRouteHailuo23Standard, VideoResolution768p, 10, 488_000, FloorUnitAPIMartCredits, apimartCreditToInternal, reason, target),
		disabledVideoTariff(domain.VideoRouteHailuo23Standard, VideoResolution1080p, 6, 720_000, FloorUnitAPIMartCredits, apimartCreditToInternal, reason, target),
	}
	return append([]DisabledProductPrice(nil), disabled...)
}

func imageTariff(modelID, quality string, floorAmount int64, unit FloorUnit, conversion UnitConversion, retailCredits int64) ProductPrice {
	return ProductPrice{
		Key: ProductKey{
			Operation:    domain.OperationImageGenerate,
			Modality:     domain.ModalityImage,
			ImageModelID: modelID,
			Quality:      quality,
		},
		Version:        StaticCatalogVersion,
		Source:         StaticSource,
		Floor:          PriceFloor{Amount: floorAmount, Unit: unit},
		Multiplier:     exactRetailMultiplier(floorAmount, conversion, retailCredits),
		UnitConversion: conversion,
		Caps: SafetyCaps{
			InternalCreditCap: retailCredits,
			FloorAmountCap:    floorAmount,
		},
		Enabled: true,
	}
}

func qwenImage3Tariff(quality string) ProductPrice {
	// APIMart public Standard tariff, checked 2026-09-08:
	// https://apimart.ai/ru/pricing — 0.205712 APIMart credits/image, both 1K/2K.
	// The x3 floor rounds up to the catalog's five-credit price step: 15.
	// References are free. Preserve the exact retail multiplier in the snapshot.
	return imageTariff(PublicImageQwenImage3, quality, 205_712, FloorUnitAPIMartCredits, apimartCreditToInternal, 15)
}

func grokImageTariff(modelID string) ProductPrice {
	// APIMart public pricing checked 2026-09-08: 0.15 provider credits/image.
	// The user explicitly selected this pricing source for Grok 2.0 over the
	// conflicting $0.08 documentation rate. x3 rounds up to 10 internal credits.
	// https://apimart.ai/ru/pricing
	return imageTariff(modelID, ImageQualityStandard, 150_000, FloorUnitAPIMartCredits, apimartCreditToInternal, 10)
}

func fixedInternalImageTariff(modelID, quality string, floorCredits, retailCredits int64) ProductPrice {
	floorAmount := floorCredits * MinorUnitsPerCredit
	return ProductPrice{
		Key: ProductKey{
			Operation:    domain.OperationImageGenerate,
			Modality:     domain.ModalityImage,
			ImageModelID: modelID,
			Quality:      quality,
		},
		Version:        StaticCatalogVersion,
		Source:         StaticSource,
		Floor:          PriceFloor{Amount: floorAmount, Unit: FloorUnitInternalCredits},
		Multiplier:     exactRetailMultiplier(floorAmount, IdentityUnitConversion(), retailCredits),
		UnitConversion: IdentityUnitConversion(),
		Caps: SafetyCaps{
			InternalCreditCap: retailCredits,
			FloorAmountCap:    floorAmount,
		},
		Enabled: true,
	}
}

func videoTariff(alias domain.VideoRouteAlias, resolution string, duration int, floorAmount int64, unit FloorUnit, conversion UnitConversion, retailCredits int64) ProductPrice {
	return ProductPrice{
		Key: ProductKey{
			Operation:       domain.OperationVideoGenerate,
			Modality:        domain.ModalityVideo,
			VideoRouteAlias: alias,
			Resolution:      resolution,
			DurationSec:     duration,
		},
		Version:        StaticCatalogVersion,
		Source:         StaticSource,
		Floor:          PriceFloor{Amount: floorAmount, Unit: unit},
		Multiplier:     exactRetailMultiplier(floorAmount, conversion, retailCredits),
		UnitConversion: conversion,
		Caps: SafetyCaps{
			InternalCreditCap: retailCredits,
			FloorAmountCap:    floorAmount,
		},
		Enabled: true,
	}
}

func exactRetailMultiplier(floorAmount int64, conversion UnitConversion, retailCredits int64) Multiplier {
	return Multiplier{
		Numerator:   retailCredits * conversion.FloorUnits * MinorUnitsPerCredit,
		Denominator: floorAmount * conversion.InternalCreditUnits,
	}
}

func disabledVideoTariff(alias domain.VideoRouteAlias, resolution string, duration int, floorAmount int64, unit FloorUnit, conversion UnitConversion, reason, target string) DisabledProductPrice {
	return DisabledProductPrice{
		Key: ProductKey{
			Operation:       domain.OperationVideoGenerate,
			Modality:        domain.ModalityVideo,
			VideoRouteAlias: alias,
			Resolution:      resolution,
			DurationSec:     duration,
		},
		Floor:          PriceFloor{Amount: floorAmount, Unit: unit},
		UnitConversion: conversion,
		Reason:         reason,
		TargetPR:       target,
	}
}
