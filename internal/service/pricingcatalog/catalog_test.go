package pricingcatalog

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestProductKeyUsesOnlyPublicDimensions(t *testing.T) {
	imageKey := ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: " nano_banana_2 ",
		Quality:      " 2K ",
	}
	normalizedImage := imageKey.Normalize()
	if !normalizedImage.Valid() || normalizedImage.ImageModelID != "nano_banana_2" || normalizedImage.Quality != "2K" {
		t.Fatalf("image product key did not normalize as public valid key: %+v", normalizedImage)
	}

	videoKey := ProductKey{
		Operation:       domain.OperationVideoGenerate,
		Modality:        domain.ModalityVideo,
		VideoRouteAlias: domain.VideoRouteKlingO3Standard,
		Resolution:      "720p",
		DurationSec:     5,
	}
	if !videoKey.Valid() {
		t.Fatalf("video product key should be valid: %+v", videoKey)
	}

	raw, err := json.Marshal(videoKey)
	if err != nil {
		t.Fatalf("marshal product key: %v", err)
	}
	assertNoPrivatePricingJSON(t, raw)
}

func TestFloorUnitsAreExplicitAndProviderSpecific(t *testing.T) {
	valid := []FloorUnit{
		FloorUnitUSDMicros,
		FloorUnitPoYoCredits,
		FloorUnitAPIMartCredits,
		FloorUnitRunwayCredits,
		FloorUnitInternalCredits,
	}
	for _, unit := range valid {
		if !unit.Valid() {
			t.Fatalf("unit %q should be valid", unit)
		}
		if !(PriceFloor{Amount: 1, Unit: unit}).Valid() {
			t.Fatalf("price floor with unit %q should be valid", unit)
		}
	}
	for _, unit := range []FloorUnit{"provider_credit_micros", "credits", "usd"} {
		if unit.Valid() {
			t.Fatalf("ambiguous unit %q must not be accepted", unit)
		}
	}
}

func TestDefaultMultiplierIsExactX3(t *testing.T) {
	m := DefaultMultiplier()
	if !m.Valid() || m.Numerator != 3 || m.Denominator != 1 {
		t.Fatalf("default multiplier = %+v, want exact 3/1", m)
	}
}

func TestProductPriceShapeAndSnapshotValidation(t *testing.T) {
	price := ProductPrice{
		Key: ProductKey{
			Operation:    domain.OperationImageGenerate,
			Modality:     domain.ModalityImage,
			ImageModelID: "nano_banana_2",
			Quality:      "1K",
		},
		Version:               1,
		Floor:                 PriceFloor{Amount: 5_000_000, Unit: FloorUnitPoYoCredits},
		Multiplier:            DefaultMultiplier(),
		UnitConversion:        IdentityUnitConversion(),
		Caps:                  SafetyCaps{InternalCreditCap: 100, FloorAmountCap: 10_000_000},
		Enabled:               true,
		DefaultDisplayCredits: 15,
	}
	price = price.Normalize()
	if !price.Valid() || price.Source != StaticSource {
		t.Fatalf("product price shape should be valid after normalize: %+v", price)
	}

	internalCredits, err := price.InternalCredits()
	if err != nil {
		t.Fatalf("internal credits: %v", err)
	}
	if internalCredits != 15 {
		t.Fatalf("internal credits = %d, want 15", internalCredits)
	}

	snapshot := PricingSnapshot{
		Version:               price.Version,
		Source:                price.Source,
		Key:                   price.Key,
		Floor:                 price.Floor,
		Multiplier:            price.Multiplier,
		UnitConversion:        price.UnitConversion,
		InternalCredits:       15,
		InternalCreditCap:     price.Caps.InternalCreditCap,
		FloorAmountCap:        price.Caps.FloorAmountCap,
		DefaultDisplayCredits: price.DefaultDisplayCredits,
	}
	if !snapshot.Valid() {
		t.Fatalf("snapshot should be valid: %+v", snapshot)
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	assertNoPrivatePricingJSON(t, raw)
}

func TestCatalogLookupCostEstimateDisplayHintAndSnapshot(t *testing.T) {
	key := ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: "nano_banana_2",
		Quality:      "1K",
	}
	catalog, err := NewCatalog([]ProductPrice{
		{
			Key:                   key,
			Version:               7,
			Source:                "static",
			Floor:                 PriceFloor{Amount: 5_000_000, Unit: FloorUnitPoYoCredits},
			Multiplier:            DefaultMultiplier(),
			UnitConversion:        IdentityUnitConversion(),
			Caps:                  SafetyCaps{InternalCreditCap: 20, FloorAmountCap: 6_000_000},
			Enabled:               true,
			DefaultDisplayCredits: 11,
		},
	})
	if err != nil {
		t.Fatalf("new catalog: %v", err)
	}

	price, err := catalog.Lookup(ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: " nano_banana_2 ",
		Quality:      "1K",
	})
	if err != nil {
		t.Fatalf("lookup normalized key: %v", err)
	}
	if price.Key.ImageModelID != "nano_banana_2" || price.Version != 7 {
		t.Fatalf("unexpected lookup price: %+v", price)
	}

	exact, err := catalog.CostEstimateCredits(key)
	if err != nil {
		t.Fatalf("cost estimate: %v", err)
	}
	if exact != 15 {
		t.Fatalf("exact cost estimate = %d, want 15", exact)
	}
	display, err := catalog.DisplayEstimateCredits(key)
	if err != nil {
		t.Fatalf("display estimate: %v", err)
	}
	if display != 11 {
		t.Fatalf("display estimate = %d, want backend display hint 11", display)
	}

	firstSnapshot, err := catalog.Snapshot(key)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	secondSnapshot, err := catalog.Snapshot(key)
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}
	if !reflect.DeepEqual(firstSnapshot, secondSnapshot) {
		t.Fatalf("snapshot should be stable:\nfirst=%+v\nsecond=%+v", firstSnapshot, secondSnapshot)
	}
	if firstSnapshot.InternalCredits != exact || firstSnapshot.DefaultDisplayCredits != display {
		t.Fatalf("snapshot credits mismatch: %+v exact=%d display=%d", firstSnapshot, exact, display)
	}
}

func TestCatalogDisplayHintDefaultsToExactEstimate(t *testing.T) {
	key := ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: "nano_banana_2",
		Quality:      "1K",
	}
	catalog, err := NewCatalog([]ProductPrice{
		{
			Key:            key,
			Version:        1,
			Floor:          PriceFloor{Amount: 5_000_000, Unit: FloorUnitPoYoCredits},
			Multiplier:     DefaultMultiplier(),
			UnitConversion: IdentityUnitConversion(),
			Enabled:        true,
		},
	})
	if err != nil {
		t.Fatalf("new catalog: %v", err)
	}

	exact, err := catalog.CostEstimateCredits(key)
	if err != nil {
		t.Fatalf("cost estimate: %v", err)
	}
	display, err := catalog.DisplayEstimateCredits(key)
	if err != nil {
		t.Fatalf("display estimate: %v", err)
	}
	if display != exact {
		t.Fatalf("display estimate = %d, want exact estimate %d", display, exact)
	}
}

func TestCatalogLookupFailsClosed(t *testing.T) {
	key := ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: "nano_banana_2",
		Quality:      "1K",
	}
	catalog, err := NewCatalog([]ProductPrice{
		{
			Key:            key,
			Version:        1,
			Floor:          PriceFloor{Amount: 5_000_000, Unit: FloorUnitPoYoCredits},
			Multiplier:     DefaultMultiplier(),
			UnitConversion: IdentityUnitConversion(),
			Enabled:        true,
		},
	})
	if err != nil {
		t.Fatalf("new catalog: %v", err)
	}

	_, err = catalog.Lookup(ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: "gpt_image_2",
		Quality:      "1K",
	})
	if !errors.Is(err, ErrPriceNotFound) {
		t.Fatalf("missing price error = %v, want ErrPriceNotFound", err)
	}

	_, err = catalog.Lookup(ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: "provider-native-model",
		DurationSec:  1,
	})
	if !errors.Is(err, ErrInvalidProductKey) {
		t.Fatalf("invalid public key error = %v, want ErrInvalidProductKey", err)
	}

	_, err = NewCatalog([]ProductPrice{
		{
			Key:            key,
			Version:        1,
			Floor:          PriceFloor{Amount: 5_000_000, Unit: FloorUnitPoYoCredits},
			Multiplier:     DefaultMultiplier(),
			UnitConversion: IdentityUnitConversion(),
			Enabled:        true,
		},
		{
			Key:            key,
			Version:        2,
			Floor:          PriceFloor{Amount: 6_000_000, Unit: FloorUnitPoYoCredits},
			Multiplier:     DefaultMultiplier(),
			UnitConversion: IdentityUnitConversion(),
			Enabled:        true,
		},
	})
	if !errors.Is(err, ErrDuplicatePrice) {
		t.Fatalf("duplicate price error = %v, want ErrDuplicatePrice", err)
	}
}

func TestCalculateInternalCreditsCeilingAndValidation(t *testing.T) {
	cases := []struct {
		name       string
		floor      PriceFloor
		conversion UnitConversion
		multiplier Multiplier
		caps       SafetyCaps
		want       int64
	}{
		{
			name:       "whole provider credits",
			floor:      PriceFloor{Amount: 5_000_000, Unit: FloorUnitPoYoCredits},
			conversion: IdentityUnitConversion(),
			multiplier: DefaultMultiplier(),
			want:       15,
		},
		{
			name:       "fractional apimart provider credits with explicit conversion",
			floor:      PriceFloor{Amount: 248_000, Unit: FloorUnitAPIMartCredits},
			conversion: UnitConversion{InternalCreditUnits: 20, FloorUnits: 1},
			multiplier: DefaultMultiplier(),
			want:       15,
		},
		{
			name:       "fractional result ceil",
			floor:      PriceFloor{Amount: 2_500_001, Unit: FloorUnitRunwayCredits},
			conversion: IdentityUnitConversion(),
			multiplier: DefaultMultiplier(),
			want:       8,
		},
		{
			name:       "valid cap",
			floor:      PriceFloor{Amount: 5_000_000, Unit: FloorUnitPoYoCredits},
			conversion: IdentityUnitConversion(),
			multiplier: DefaultMultiplier(),
			caps:       SafetyCaps{InternalCreditCap: 15, FloorAmountCap: 5_000_000},
			want:       15,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CalculateInternalCredits(tc.floor, tc.conversion, tc.multiplier, tc.caps)
			if err != nil {
				t.Fatalf("calculate internal credits: %v", err)
			}
			if got != tc.want {
				t.Fatalf("internal credits = %d, want %d", got, tc.want)
			}
		})
	}

	invalidCases := []struct {
		name       string
		floor      PriceFloor
		conversion UnitConversion
		multiplier Multiplier
		caps       SafetyCaps
		wantErr    error
	}{
		{name: "zero floor", floor: PriceFloor{Unit: FloorUnitPoYoCredits}, conversion: IdentityUnitConversion(), multiplier: DefaultMultiplier(), wantErr: ErrInvalidFloor},
		{name: "unknown unit", floor: PriceFloor{Amount: 1, Unit: "provider_credit_micros"}, conversion: IdentityUnitConversion(), multiplier: DefaultMultiplier(), wantErr: ErrInvalidFloor},
		{name: "zero conversion", floor: PriceFloor{Amount: 1, Unit: FloorUnitPoYoCredits}, multiplier: DefaultMultiplier(), wantErr: ErrCreditCalculation},
		{name: "zero multiplier", floor: PriceFloor{Amount: 1, Unit: FloorUnitPoYoCredits}, conversion: IdentityUnitConversion(), multiplier: Multiplier{Numerator: 0, Denominator: 1}, wantErr: ErrInvalidMultiplier},
		{name: "negative cap", floor: PriceFloor{Amount: 1, Unit: FloorUnitPoYoCredits}, conversion: IdentityUnitConversion(), multiplier: DefaultMultiplier(), caps: SafetyCaps{InternalCreditCap: -1}, wantErr: ErrInvalidCap},
		{name: "floor cap exceeded", floor: PriceFloor{Amount: 5_000_000, Unit: FloorUnitPoYoCredits}, conversion: IdentityUnitConversion(), multiplier: DefaultMultiplier(), caps: SafetyCaps{FloorAmountCap: 4_999_999}, wantErr: ErrInvalidCap},
		{name: "internal cap exceeded", floor: PriceFloor{Amount: 5_000_000, Unit: FloorUnitPoYoCredits}, conversion: IdentityUnitConversion(), multiplier: DefaultMultiplier(), caps: SafetyCaps{InternalCreditCap: 14}, wantErr: ErrInvalidCap},
	}
	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CalculateInternalCredits(tc.floor, tc.conversion, tc.multiplier, tc.caps)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("calculate error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestInvalidShapeFailsClosed(t *testing.T) {
	validKey := ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: "nano_banana_2",
		Quality:      "1K",
	}
	cases := []struct {
		name  string
		price ProductPrice
	}{
		{name: "disabled", price: ProductPrice{Key: validKey, Version: 1, Floor: PriceFloor{Amount: 1, Unit: FloorUnitUSDMicros}, UnitConversion: IdentityUnitConversion(), Multiplier: DefaultMultiplier()}},
		{name: "missing version", price: ProductPrice{Key: validKey, Floor: PriceFloor{Amount: 1, Unit: FloorUnitUSDMicros}, UnitConversion: IdentityUnitConversion(), Multiplier: DefaultMultiplier(), Enabled: true}},
		{name: "missing key", price: ProductPrice{Version: 1, Floor: PriceFloor{Amount: 1, Unit: FloorUnitUSDMicros}, UnitConversion: IdentityUnitConversion(), Multiplier: DefaultMultiplier(), Enabled: true}},
		{name: "zero floor", price: ProductPrice{Key: validKey, Version: 1, Floor: PriceFloor{Unit: FloorUnitUSDMicros}, UnitConversion: IdentityUnitConversion(), Multiplier: DefaultMultiplier(), Enabled: true}},
		{name: "bad multiplier", price: ProductPrice{Key: validKey, Version: 1, Floor: PriceFloor{Amount: 1, Unit: FloorUnitUSDMicros}, UnitConversion: IdentityUnitConversion(), Multiplier: Multiplier{Numerator: 3}, Enabled: true}},
		{name: "missing conversion", price: ProductPrice{Key: validKey, Version: 1, Floor: PriceFloor{Amount: 1, Unit: FloorUnitUSDMicros}, Multiplier: DefaultMultiplier(), Enabled: true}},
		{name: "negative cap", price: ProductPrice{Key: validKey, Version: 1, Floor: PriceFloor{Amount: 1, Unit: FloorUnitUSDMicros}, UnitConversion: IdentityUnitConversion(), Multiplier: DefaultMultiplier(), Caps: SafetyCaps{InternalCreditCap: -1}, Enabled: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.price.Valid() {
				t.Fatalf("invalid product price shape passed validation: %+v", tc.price)
			}
		})
	}
}

func assertNoPrivatePricingJSON(t *testing.T, raw []byte) {
	t.Helper()
	serialized := strings.ToLower(string(raw))
	for _, forbidden := range []string{
		"provider",
		"provider_model_id",
		"provider_native_model_id",
		"model_code",
		"prompt",
		"payload",
		"private_url",
	} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("pricing public shape leaked private field %q: %s", forbidden, raw)
		}
	}
}
