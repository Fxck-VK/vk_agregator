package pricingcatalog

import (
	"testing"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/economycatalog"
)

func TestCompleteEconomyPricesEveryExistingCostBasedProduct(t *testing.T) {
	c, err := NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id, quality string
		want        int64
	}{
		{PublicImageNanoBanana2, "1K", 15}, {PublicImageNanoBanana2, "4K", 40},
		{PublicImageSeedream45, "4K", 45}, {PublicImageGPTImage2, "1K", 51},
	} {
		s, err := c.Snapshot(ProductKey{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ImageModelID: tc.id, Quality: tc.quality})
		if err != nil || s.InternalCredits != tc.want || s.RetailPolicyVersion != economycatalog.EconomyVersion {
			t.Fatalf("%s/%s quote%+v err%v", tc.id, tc.quality, s, err)
		}
	}
	q, err := TextCandidateQuote("gpt_6_sol")
	if err != nil || q.RetailPolicyVersion != economycatalog.EconomyVersion {
		t.Fatalf("unmanaged text economics: %+v %v", q, err)
	}
}

func TestCompleteEconomyProtectsReferenceAgainstDiscountedIncome(t *testing.T) {
	c, _ := NewStaticCatalog()
	for _, tc := range []struct {
		quality string
		want    int64
	}{
		{"1K-medium", 52}, {"1K-max", 52}, {"2K-max", 220}, {"4K-xhigh", 165}, {"4K-max", 355},
	} {
		s, err := c.Snapshot(ProductKey{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ImageModelID: PublicImageGPTImage25Flare, Quality: tc.quality})
		if err != nil || s.InternalCredits != tc.want {
			t.Fatalf("unsafe price %s: %+v %v", tc.quality, s, err)
		}
	}
}

func TestExactRetailMultiplierCannotOverflowLargeRuntimeCosts(t *testing.T) {
	floor := int64(500000000000000000)
	conversion := UnitConversion{InternalCreditUnits: 200, FloorUnits: 1}
	want := int64(300000000000000)
	m := exactRetailMultiplier(floor, conversion, want)
	got, err := CalculateInternalCredits(PriceFloor{Amount: floor, Unit: FloorUnitUSDMicros}, conversion, m, SafetyCaps{})
	if err != nil || got != want {
		t.Fatalf("wrapped exact price: %d want%d err%v multiplier%+v", got, want, err, m)
	}
}

func TestDirectWhisperQuoteFreezesOperatorEconomics(t *testing.T) {
	q, err := WhisperDEVQuote(120)
	if err != nil || q.InternalCredits != 50 || q.EconomicBasis != "operator_fixed" || q.RetailPolicyVersion != economycatalog.EconomyVersion {
		t.Fatalf("operator quote missing economics: %+v %v", q, err)
	}
}
