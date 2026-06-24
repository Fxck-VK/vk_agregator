package pricingcatalog

import (
	"errors"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestNewStaticCatalogContainsApprovedImageTariffs(t *testing.T) {
	catalog, err := NewStaticCatalog()
	if err != nil {
		t.Fatalf("new static catalog: %v", err)
	}

	cases := []struct {
		modelID string
		quality string
		want    int64
		unit    FloorUnit
	}{
		{modelID: PublicImageNanoBanana2, quality: ImageQuality1K, want: 15, unit: FloorUnitPoYoCredits},
		{modelID: PublicImageNanoBanana2, quality: ImageQuality2K, want: 24, unit: FloorUnitPoYoCredits},
		{modelID: PublicImageNanoBanana2, quality: ImageQuality4K, want: 36, unit: FloorUnitPoYoCredits},
		{modelID: PublicImageGPTImage2, quality: ImageQuality1K, want: 4, unit: FloorUnitAPIMartCredits},
		{modelID: PublicImageGPTImage2, quality: ImageQuality2K, want: 8, unit: FloorUnitAPIMartCredits},
		{modelID: PublicImageGPTImage2, quality: ImageQuality4K, want: 11, unit: FloorUnitAPIMartCredits},
		{modelID: PublicImageNanoBananaPro, quality: ImageQuality1K, want: 24, unit: FloorUnitAPIMartCredits},
		{modelID: PublicImageNanoBananaPro, quality: ImageQuality4K, want: 30, unit: FloorUnitAPIMartCredits},
	}
	for _, tc := range cases {
		t.Run(tc.modelID+"/"+tc.quality, func(t *testing.T) {
			key := ProductKey{
				Operation:    domain.OperationImageGenerate,
				Modality:     domain.ModalityImage,
				ImageModelID: tc.modelID,
				Quality:      tc.quality,
			}
			got, err := catalog.CostEstimateCredits(key)
			if err != nil {
				t.Fatalf("cost estimate: %v", err)
			}
			if got != tc.want {
				t.Fatalf("cost estimate = %d, want %d", got, tc.want)
			}
			price, err := catalog.Lookup(key)
			if err != nil {
				t.Fatalf("lookup: %v", err)
			}
			if price.Floor.Unit != tc.unit {
				t.Fatalf("floor unit = %s, want %s", price.Floor.Unit, tc.unit)
			}
		})
	}
}

func TestNewStaticCatalogContainsApprovedVideoTariffs(t *testing.T) {
	catalog, err := NewStaticCatalog()
	if err != nil {
		t.Fatalf("new static catalog: %v", err)
	}

	cases := []struct {
		alias      domain.VideoRouteAlias
		resolution string
		duration   int
		want       int64
		unit       FloorUnit
	}{
		{alias: domain.VideoRouteKlingO3Standard, resolution: VideoResolution720p, duration: 5, want: 150, unit: FloorUnitPoYoCredits},
		{alias: domain.VideoRouteKlingO3Standard, resolution: VideoResolution1080p, duration: 10, want: 300, unit: FloorUnitPoYoCredits},
		{alias: domain.VideoRouteSeedance20Fast, resolution: VideoResolution720p, duration: 5, want: 420, unit: FloorUnitPoYoCredits},
		{alias: domain.VideoRouteSeedance20Fast, resolution: VideoResolution720p, duration: 10, want: 840, unit: FloorUnitPoYoCredits},
		{alias: domain.VideoRouteRunwayGen4Turbo, resolution: VideoResolution720p, duration: 7, want: 210, unit: FloorUnitRunwayCredits},
		{alias: domain.VideoRouteRunwayGen45, resolution: VideoResolution720p, duration: 5, want: 225, unit: FloorUnitPoYoCredits},
		{alias: domain.VideoRouteRunwayGen45, resolution: VideoResolution1080p, duration: 10, want: 450, unit: FloorUnitPoYoCredits},
	}
	for _, tc := range cases {
		t.Run(string(tc.alias)+"/"+tc.resolution, func(t *testing.T) {
			key := ProductKey{
				Operation:       domain.OperationVideoGenerate,
				Modality:        domain.ModalityVideo,
				VideoRouteAlias: tc.alias,
				Resolution:      tc.resolution,
				DurationSec:     tc.duration,
			}
			got, err := catalog.CostEstimateCredits(key)
			if err != nil {
				t.Fatalf("cost estimate: %v", err)
			}
			if got != tc.want {
				t.Fatalf("cost estimate = %d, want %d", got, tc.want)
			}
			price, err := catalog.Lookup(key)
			if err != nil {
				t.Fatalf("lookup: %v", err)
			}
			if price.Floor.Unit != tc.unit {
				t.Fatalf("floor unit = %s, want %s", price.Floor.Unit, tc.unit)
			}
		})
	}
}

func TestStaticCatalogUsesOnlyBoundedAliasesAndExplicitUnits(t *testing.T) {
	allowedImages := map[string]bool{
		PublicImageNanoBanana2:   true,
		PublicImageNanoBananaPro: true,
		PublicImageGPTImage2:     true,
	}
	allowedVideos := map[domain.VideoRouteAlias]bool{
		domain.VideoRouteKlingO3Standard: true,
		domain.VideoRouteRunwayGen4Turbo: true,
		domain.VideoRouteSeedance20Fast:  true,
		domain.VideoRouteRunwayGen45:     true,
	}

	for _, price := range StaticProductPrices() {
		if !price.Valid() {
			t.Fatalf("static price should be valid: %+v", price)
		}
		if price.Floor.Unit == FloorUnit("provider_credit_micros") {
			t.Fatalf("static price used generic provider credit unit: %+v", price)
		}
		if price.Multiplier != DefaultMultiplier() {
			t.Fatalf("static price multiplier = %+v, want x3", price.Multiplier)
		}
		switch price.Key.Modality {
		case domain.ModalityImage:
			if !allowedImages[price.Key.ImageModelID] {
				t.Fatalf("unexpected image alias in static catalog: %+v", price.Key)
			}
		case domain.ModalityVideo:
			if !allowedVideos[price.Key.VideoRouteAlias] {
				t.Fatalf("unexpected video alias in static catalog: %+v", price.Key)
			}
		default:
			t.Fatalf("unexpected modality in static catalog: %+v", price.Key)
		}
	}
}

func TestAmbiguousHailuoTariffsRemainDisabledAndFailClosed(t *testing.T) {
	disabled := DisabledStaticProductPrices()
	if len(disabled) == 0 {
		t.Fatal("expected disabled Hailuo tariffs to be recorded")
	}
	for _, price := range disabled {
		if price.Key.VideoRouteAlias != domain.VideoRouteHailuo23Fast &&
			price.Key.VideoRouteAlias != domain.VideoRouteHailuo23Standard {
			t.Fatalf("unexpected disabled tariff: %+v", price)
		}
		if !price.Floor.Valid() || price.Floor.Unit != FloorUnitAPIMartCredits {
			t.Fatalf("disabled Hailuo tariff must keep explicit APIMart floor: %+v", price)
		}
		if !price.UnitConversion.Valid() {
			t.Fatalf("disabled Hailuo tariff must keep explicit conversion: %+v", price)
		}
		if strings.TrimSpace(price.Reason) == "" || strings.TrimSpace(price.TargetPR) == "" {
			t.Fatalf("disabled Hailuo tariff must record reason and target PR: %+v", price)
		}
	}

	catalog, err := NewStaticCatalog()
	if err != nil {
		t.Fatalf("new static catalog: %v", err)
	}
	_, err = catalog.Lookup(disabled[0].Key)
	if !errors.Is(err, ErrPriceNotFound) {
		t.Fatalf("disabled Hailuo lookup error = %v, want ErrPriceNotFound", err)
	}
}

func TestStaticCatalogMissingRouteAndModelFailClosed(t *testing.T) {
	catalog, err := NewStaticCatalog()
	if err != nil {
		t.Fatalf("new static catalog: %v", err)
	}

	cases := []struct {
		name string
		key  ProductKey
	}{
		{
			name: "missing image model",
			key: ProductKey{
				Operation:    domain.OperationImageGenerate,
				Modality:     domain.ModalityImage,
				ImageModelID: "seedream_4_5",
				Quality:      ImageQuality1K,
			},
		},
		{
			name: "missing video route",
			key: ProductKey{
				Operation:       domain.OperationVideoGenerate,
				Modality:        domain.ModalityVideo,
				VideoRouteAlias: domain.VideoRouteMockTextToVideo,
				Resolution:      VideoResolution720p,
				DurationSec:     5,
			},
		},
		{
			name: "missing duration variant",
			key: ProductKey{
				Operation:       domain.OperationVideoGenerate,
				Modality:        domain.ModalityVideo,
				VideoRouteAlias: domain.VideoRouteRunwayGen45,
				Resolution:      VideoResolution720p,
				DurationSec:     7,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := catalog.Lookup(tc.key)
			if !errors.Is(err, ErrPriceNotFound) {
				t.Fatalf("lookup error = %v, want ErrPriceNotFound", err)
			}
		})
	}
}
