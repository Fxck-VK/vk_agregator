package pricingcatalog

import (
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestKlingVeoTariffsHaveExactKeyMatrixAndX3RoundedRetail(t *testing.T) {
	got := make(map[ProductKey]ProductPrice)
	for _, price := range klingVeoTariffs() {
		key := price.Key.Normalize()
		if _, exists := got[key]; exists {
			t.Fatalf("duplicate Kling/Veo tariff key: %+v", key)
		}
		got[key] = price
	}

	want := make(map[ProductKey]int64)
	add := func(alias domain.VideoRouteAlias, resolution string, duration int, floor int64, quality string) {
		want[ProductKey{
			Operation:       domain.OperationVideoGenerate,
			Modality:        domain.ModalityVideo,
			VideoRouteAlias: alias,
			Quality:         quality,
			Resolution:      resolution,
			DurationSec:     duration,
		}] = floor
	}
	for _, audio := range []bool{false, true} {
		quality := ""
		if audio {
			quality = VideoQualityAudio
		}
		for resolution, rate := range map[string]int64{
			"720p":  672_000,
			"1080p": 896_000,
			"4k":    4_285_600,
		} {
			if audio {
				switch resolution {
				case "720p":
					rate = 1_008_000
				case "1080p":
					rate = 1_344_000
				}
			}
			for duration := 3; duration <= 15; duration++ {
				add(domain.VideoRouteKlingV3, resolution, duration, rate*int64(duration), quality)
			}
		}
	}
	for _, tc := range []struct {
		resolution string
		rate       int64
	}{
		{resolution: "std", rate: 571_200},
		{resolution: "pro", rate: 914_400},
	} {
		for duration := 3; duration <= 30; duration++ {
			add(domain.VideoRouteKling26Motion, tc.resolution, duration, tc.rate*int64(duration), "")
		}
	}
	for _, tc := range []struct {
		alias domain.VideoRouteAlias
		base  int64
		high  int64
	}{
		{alias: domain.VideoRouteVeo31Lite, base: 700_000, high: 5_700_000},
		{alias: domain.VideoRouteVeo31Fast, base: 1_400_000, high: 6_400_000},
		{alias: domain.VideoRouteVeo31Quality, base: 10_000_000, high: 15_000_000},
	} {
		add(tc.alias, "720p", 8, tc.base, "")
		add(tc.alias, "1080p", 8, tc.base, "")
		add(tc.alias, "4k", 8, tc.high, "")
	}
	if len(want) != 143 {
		t.Fatalf("test expected-key fixture has %d keys, want 143", len(want))
	}
	if len(got) != len(want) {
		t.Fatalf("Kling/Veo tariff count = %d, want %d", len(got), len(want))
	}

	staticCatalog, err := NewStaticCatalog()
	if err != nil {
		t.Fatalf("new static catalog: %v", err)
	}
	for key, wantFloor := range want {
		price, ok := got[key]
		if !ok {
			t.Fatalf("missing Kling/Veo tariff key: %+v", key)
		}
		assertKlingVeoPrice(t, price, key, wantFloor)

		catalogPrice, err := staticCatalog.Lookup(key)
		if err != nil {
			t.Fatalf("static catalog missing key %+v: %v", key, err)
		}
		assertKlingVeoPrice(t, catalogPrice, key, wantFloor)
	}
	for key := range got {
		if _, ok := want[key]; !ok {
			t.Fatalf("unexpected Kling/Veo tariff key: %+v", key)
		}
	}
}

func assertKlingVeoPrice(t *testing.T, price ProductPrice, key ProductKey, wantFloor int64) {
	t.Helper()
	if price.Key.Normalize() != key {
		t.Fatalf("price key = %+v, want %+v", price.Key.Normalize(), key)
	}
	if price.Floor.Unit != FloorUnitAPIMartCredits || price.Floor.Amount != wantFloor {
		t.Fatalf("%+v floor = %d %s, want %d %s", key, price.Floor.Amount, price.Floor.Unit, wantFloor, FloorUnitAPIMartCredits)
	}
	if price.Version != StaticCatalogVersion || price.Source != StaticSource || !price.Enabled {
		t.Fatalf("%+v invalid catalog metadata: %+v", key, price)
	}
	credits, err := price.InternalCredits()
	if err != nil {
		t.Fatalf("%+v internal credits: %v", key, err)
	}
	if want := providerCreditMicrosX3RoundedToFive(wantFloor); credits != want {
		t.Fatalf("%+v internal credits = %d, want %d", key, credits, want)
	}
	if credits%5 != 0 {
		t.Fatalf("%+v internal credits = %d, want 5-credit step", key, credits)
	}
}

func providerCreditMicrosX3RoundedToFive(floorMicros int64) int64 {
	const providerCreditToInternalX3 = 60
	const fiveInternalCreditsMicros = 5 * MinorUnitsPerCredit
	return ((floorMicros*providerCreditToInternalX3 + fiveInternalCreditsMicros - 1) / fiveInternalCreditsMicros) * 5
}
