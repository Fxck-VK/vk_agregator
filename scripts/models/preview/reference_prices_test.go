package main

import (
	"testing"

	"vk-ai-aggregator/internal/service/productcatalog"
)

func TestReferencePricesUseCanonicalCustomerQuotesAndCombinedLimits(t *testing.T) {
	catalog, err := productcatalog.WorkspacePreviewCatalog()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := referencePrices(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if got := rows["nano_banana_pro"]["2K:1:1:1:1"]; got != 55 {
		t.Fatalf("reference image quote = %d, want55", got)
	}
	if got := rows["seedream_4_5"]["4K:1:1:1:4"]; got != 180 {
		t.Fatalf("reference batch quote = %d, want180", got)
	}
	if _, exists := rows["gpt_image_2_5_flare"]; exists {
		t.Fatal("exported quotes for a model without admitted reference input")
	}
	if _, exists := rows["seedream_5_0_lite"]["2K:1:1:14:2"]; exists {
		t.Fatal("exported inadmissible combined reference/output count")
	}
	if _, exists := rows["nano_banana_pro"]["8K:1:1:1:1"]; exists {
		t.Fatal("exported unsupported resolution")
	}
	for id, variants := range rows {
		if len(variants) == 0 {
			t.Fatalf("empty reference price map for %s", id)
		}
		for key, price := range variants {
			if price <= 0 {
				t.Fatalf("nonpositive quote for %s/%s", id, key)
			}
		}
	}
}
