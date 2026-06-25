package pricingcatalog

import "vk-ai-aggregator/internal/domain"

const (
	StaticCatalogVersion = 1

	PublicImageNanoBanana2   = "nano_banana_2"
	PublicImageNanoBananaPro = "nano_banana_pro"
	PublicImageGPTImage2     = "gpt_image_2"

	ImageQuality1K = "1K"
	ImageQuality2K = "2K"
	ImageQuality4K = "4K"

	VideoResolution720p  = "720p"
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
		imageTariff(PublicImageNanoBanana2, ImageQuality1K, 5_000_000, FloorUnitPoYoCredits, poyoCreditToInternal, 15),
		imageTariff(PublicImageNanoBanana2, ImageQuality2K, 8_000_000, FloorUnitPoYoCredits, poyoCreditToInternal, 24),
		imageTariff(PublicImageNanoBanana2, ImageQuality4K, 12_000_000, FloorUnitPoYoCredits, poyoCreditToInternal, 36),
		imageTariff(PublicImageGPTImage2, ImageQuality1K, 60_000, FloorUnitAPIMartCredits, apimartCreditToInternal, 4),
		imageTariff(PublicImageGPTImage2, ImageQuality2K, 120_000, FloorUnitAPIMartCredits, apimartCreditToInternal, 8),
		imageTariff(PublicImageGPTImage2, ImageQuality4K, 180_000, FloorUnitAPIMartCredits, apimartCreditToInternal, 11),
		imageTariff(PublicImageNanoBananaPro, ImageQuality1K, 400_000, FloorUnitAPIMartCredits, apimartCreditToInternal, 24),
		imageTariff(PublicImageNanoBananaPro, ImageQuality2K, 500_000, FloorUnitAPIMartCredits, apimartCreditToInternal, 30),
		imageTariff(PublicImageNanoBananaPro, ImageQuality4K, 500_000, FloorUnitAPIMartCredits, apimartCreditToInternal, 30),
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
				int64(duration)*30,
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
			int64(duration)*84,
		))
	}
	for duration := 2; duration <= 10; duration++ {
		prices = append(prices, videoTariff(
			domain.VideoRouteRunwayGen4Turbo,
			VideoResolution720p,
			duration,
			int64(duration)*5_000_000,
			FloorUnitRunwayCredits,
			runwayCreditToInternal,
			int64(duration)*30,
		))
	}
	for _, resolution := range []string{VideoResolution720p, VideoResolution1080p} {
		prices = append(prices,
			videoTariff(domain.VideoRouteRunwayGen45, resolution, 5, 75_000_000, FloorUnitPoYoCredits, poyoCreditToInternal, 225),
			videoTariff(domain.VideoRouteRunwayGen45, resolution, 10, 150_000_000, FloorUnitPoYoCredits, poyoCreditToInternal, 450),
		)
	}

	return append([]ProductPrice(nil), prices...)
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

func imageTariff(modelID, quality string, floorAmount int64, unit FloorUnit, conversion UnitConversion, internalCap int64) ProductPrice {
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
		Multiplier:     DefaultMultiplier(),
		UnitConversion: conversion,
		Caps: SafetyCaps{
			InternalCreditCap: internalCap,
			FloorAmountCap:    floorAmount,
		},
		Enabled: true,
	}
}

func videoTariff(alias domain.VideoRouteAlias, resolution string, duration int, floorAmount int64, unit FloorUnit, conversion UnitConversion, internalCap int64) ProductPrice {
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
		Multiplier:     DefaultMultiplier(),
		UnitConversion: conversion,
		Caps: SafetyCaps{
			InternalCreditCap: internalCap,
			FloorAmountCap:    floorAmount,
		},
		Enabled: true,
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
