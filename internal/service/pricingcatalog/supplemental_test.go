package pricingcatalog

import "testing"

func TestSupplementalPricesSurviveRefreshWithoutOverridingPrimary(t *testing.T) {
	c, err := NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	q, err := ImageCandidateQuote("nano_banana")
	if err != nil {
		t.Fatal(err)
	}
	price := ProductPrice{Key: q.Key, Version: q.Version, Source: q.Source, Floor: q.Floor, Multiplier: q.Multiplier, UnitConversion: q.UnitConversion, Enabled: true}
	if err := c.AddSupplemental([]ProductPrice{price}); err != nil {
		t.Fatal(err)
	}
	next, err := NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.ReplaceWith(next); err != nil {
		t.Fatal(err)
	}
	if got, err := c.CostEstimateCredits(q.Key); err != nil || got != q.InternalCredits {
		t.Fatalf("supplement lost: %d %v", got, err)
	}
	primary := price
	primary.Version++
	next, err = NewCatalog([]ProductPrice{primary})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.ReplaceWith(next); err != nil {
		t.Fatal(err)
	}
	if got, err := c.Lookup(q.Key); err != nil || got.Version != primary.Version {
		t.Fatalf("primary overridden: %+v %v", got, err)
	}
	if len(c.Prices()) != 1 {
		t.Fatal("duplicate supplemental price")
	}
	// Even an explicitly disabled primary entry must never fall back to a supplement.
	primary.Enabled = false
	c.prices[q.Key.Normalize()] = primary
	if _, err := c.Lookup(q.Key); err == nil {
		t.Fatal("disabled primary bypassed")
	}
}
