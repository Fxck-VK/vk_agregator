package pricingcatalog

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestEconomicPolicyRepricesOnlyNewVerifiedNanoQuotes(t *testing.T) {
	for _, quality := range []string{ImageQuality1K, ImageQuality2K, ImageQuality4K} {
		base := imageTariff(PublicImageNanoBananaPro, quality, 400000, FloorUnitAPIMartCredits, apimartCreditToInternal, 50)
		old, err := base.Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(old)
		catalog, err := NewCatalog([]ProductPrice{base})
		if err != nil {
			t.Fatal(err)
		}
		quote, err := catalog.Snapshot(base.Key)
		if err != nil {
			t.Fatal(err)
		}
		if quote.InternalCredits != 55 || quote.InternalCreditCap != 55 || quote.RetailPolicyVersion != 2026102802 || quote.Floor != old.Floor || quote.FloorAmountCap != old.FloorAmountCap || quote.UnitConversion != old.UnitConversion {
			t.Fatalf("bad new retail quote: %+v old=%+v", quote, old)
		}
		var persisted PricingSnapshot
		if err := json.Unmarshal(raw, &persisted); err != nil || !persisted.Valid() || persisted.InternalCredits != 50 || persisted.InternalCreditCap != 50 || persisted.RetailPolicyVersion != 0 {
			t.Fatalf("historical price changed: %+v %v", persisted, err)
		}
	}
}

func TestEconomicPolicySurvivesRuntimeDBReloadWithoutNewAdmission(t *testing.T) {
	base := imageTariff(PublicImageNanoBananaPro, ImageQuality2K, 500000, FloorUnitAPIMartCredits, apimartCreditToInternal, 60)
	repo := &mutableRuntimeRepo{}
	repo.setSet(RuntimePriceSet{Version: RuntimeCatalogVersion{PriceVersion: 19}, Prices: []ProductPrice{base}})
	cache := NewRuntimeCatalogCache(repo, nil, RuntimeCatalogConfig{DBEnabled: true})
	if err := cache.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	catalog, _, err := cache.Current()
	if err != nil {
		t.Fatal(err)
	}
	for _, floor := range []int64{500000, 550000} {
		base.Floor.Amount, base.Caps.FloorAmountCap = floor, floor
		base.Multiplier = exactRetailMultiplier(floor, base.UnitConversion, 70)
		base.Caps.InternalCreditCap = 70
		repo.setSet(RuntimePriceSet{Version: RuntimeCatalogVersion{PriceVersion: 20}, Prices: []ProductPrice{base}})
		if err := cache.Reload(context.Background()); err != nil {
			t.Fatal(err)
		}
		quote, err := catalog.Snapshot(base.Key)
		if err != nil {
			t.Fatal(err)
		}
		if quote.InternalCredits != 55 || quote.RetailPolicyVersion != 2026102802 || quote.Source != RuntimeDBSource || quote.Version != 20 || quote.Floor.Amount != floor || quote.FloorAmountCap != floor {
			t.Fatalf("DB changed retail policy or provider metadata: %+v", quote)
		}
		prices := catalog.Prices()
		if got, _ := prices[0].InternalCredits(); got != 55 {
			t.Fatalf("operator inventory diverges: %d", got)
		}
	}
	if _, err := catalog.Snapshot(ProductKey{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ImageModelID: PublicImageNanoBananaPro, Quality: "8K"}); !errors.Is(err, ErrPriceNotFound) {
		t.Fatalf("unverified variant admitted: %v", err)
	}
	if len(catalog.Prices()) != 1 {
		t.Fatal("retail policy admitted additional products")
	}
}

func TestEconomicPolicyLeavesOtherMinimaAndUnitsUnchanged(t *testing.T) {
	catalog, err := NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, price := range StaticProductPrices() {
		if price.Key.ImageModelID == PublicImageNanoBananaPro {
			continue
		}
		want, err := price.Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		got, err := catalog.Snapshot(price.Key)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("unverified competitor minimum repriced %v", price.Key)
		}
	}
	if _, err := catalog.CostEstimateCredits(ProductKey{Operation: domain.OperationTextGenerate, Modality: domain.ModalityText, TextModelID: "missing-study24-model"}); !errors.Is(err, ErrPriceNotFound) {
		t.Fatal("unknown text product admitted")
	}
}

func TestStaticProductInventoryUsesCurrentEconomicRetailPolicy(t *testing.T) {
	matched := 0
	for _, price := range StaticProductPrices() {
		if price.Key.ImageModelID != PublicImageNanoBananaPro {
			continue
		}
		matched++
		got, err := price.InternalCredits()
		if err != nil || got != 55 || price.RetailPolicyVersion != 2026102802 {
			t.Fatalf("static inventory has stale retail: %+v %v", price, err)
		}
	}
	if matched != 3 {
		t.Fatalf("unexpected verified variant count%d", matched)
	}
}
