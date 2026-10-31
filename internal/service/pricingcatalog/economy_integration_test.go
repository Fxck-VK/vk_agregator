package pricingcatalog_test

import (
	"errors"
	"testing"

	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

func TestEconomicRetailPolicyUsesExistingCountAndAdmissionBoundary(t *testing.T) {
	catalog, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	model := imagegeneration.PublicModel{ID: pricingcatalog.PublicImageNanoBananaPro, Name: "Nano Banana Pro", Enabled: true, Ready: true,
		QualityOptions: []string{"1K", "2K", "4K"}, DefaultQuality: "1K", MaxOutputCount: 4}
	resolver := imagegeneration.NewResolver([]imagegeneration.PublicModel{model}, catalog)
	for _, quality := range model.QualityOptions {
		for _, count := range []int{1} {
			got, err := resolver.Resolve(imagegeneration.Request{Prompt: "offline fixture", ModelID: model.ID, Quality: quality, OutputCount: count})
			if err != nil {
				t.Fatal(err)
			}
			if got.PricingSnapshot.InternalCredits != int64(count)*55 || got.PricingSnapshot.InternalCreditCap != int64(count)*55 || got.PricingSnapshot.RetailPolicyVersion != 2026102802 || !got.PricingSnapshot.Valid() {
				t.Fatalf("bad count quote: %+v", got.PricingSnapshot)
			}
		}
	}
	// Study24's four-image total is research evidence, not permission to
	// increase our existing provider contract's one-image limit.
	if _, err := resolver.Resolve(imagegeneration.Request{ModelID: model.ID, Quality: "1K", OutputCount: 4}); !errors.Is(err, imagegeneration.ErrOutputCountLimit) {
		t.Fatalf("research count bypassed provider limit: %v", err)
	}
	model.Ready = false
	closed := imagegeneration.NewResolver([]imagegeneration.PublicModel{model}, catalog)
	if _, err := closed.Resolve(imagegeneration.Request{ModelID: model.ID, Quality: "1K"}); !errors.Is(err, imagegeneration.ErrPublicModelUnavailable) {
		t.Fatalf("pricing fact bypassed admission: %v", err)
	}
}
