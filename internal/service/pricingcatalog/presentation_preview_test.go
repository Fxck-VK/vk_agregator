package pricingcatalog

import (
	"math/big"
	"testing"

	"vk-ai-aggregator/internal/service/economycatalog"
)

func TestPresentationPreviewQuote(t *testing.T) {
	quote, err := PresentationPreviewQuote()
	if err != nil || quote.TextOnly <= 0 || quote.WithCover < quote.TextOnly || quote.TextOnly%5 != 0 || quote.WithCover%5 != 0 {
		t.Fatalf("invalid presentation quote: %+v (%v)", quote, err)
	}
}

func TestPresentationRetailMeetsTripleCostAtCheapestOffer(t *testing.T) {
	// Two costs of two nominal stars. At the catalogue's cheapest rate, a
	// twelve-star nominal markup must be rounded up to the next valid price.
	item := PricingSnapshot{
		Floor:          PriceFloor{Amount: 2 * MinorUnitsPerCredit, Unit: FloorUnitInternalCredits},
		UnitConversion: UnitConversion{FloorUnits: 1, InternalCreditUnits: 1},
	}
	price, err := presentationRetail(item, item)
	if err != nil {
		t.Fatal(err)
	}
	leastStarPrice := economycatalog.CheapestPaidStarKopecks()
	floor := big.NewRat(4*3*economycatalog.StarNominalKopecks, 1)
	if new(big.Rat).Mul(big.NewRat(price, 1), leastStarPrice).Cmp(floor) < 0 {
		t.Fatalf("quote %d is below three times cost at the lowest star price", price)
	}
	if price%5 != 0 || new(big.Rat).Mul(big.NewRat(price-5, 1), leastStarPrice).Cmp(floor) >= 0 {
		t.Fatalf("quote %d was not rounded once to the smallest supported price", price)
	}
	if _, err := presentationRetail(PricingSnapshot{}); err == nil {
		t.Fatal("quote accepted an unknown cost")
	}
}
