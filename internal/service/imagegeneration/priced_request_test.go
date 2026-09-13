package imagegeneration_test

import (
	"strings"
	"testing"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

func TestPricedImageRequestRejectsChangedExecutionDimensions(t *testing.T) {
	prices, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id, code, quality, resolution string
		refs                          int
	}{
		{"gpt_image_2_5_flare", "gpt-image-2.5-flare", "2K-high", "2K", 0},
		{"seedream_5_0_lite", "seedream-5-0-lite", "3K", "3K", 2},
		{"seedream_5_0_pro", "seedream-5-0-pro", "1.5K", "1.5K", 10},
	} {
		s, err := prices.Snapshot(pricingcatalog.ProductKey{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ImageModelID: tc.id, Quality: tc.quality})
		if err != nil {
			t.Fatal(err)
		}
		s, err = pricingcatalog.QuoteAPIMartImage(s, "16:9", tc.refs)
		if err != nil {
			t.Fatal(err)
		}
		base := imagegeneration.Request{Prompt: "A mountain", ModelID: tc.id, Quality: tc.quality, AspectRatio: "16:9", ReferenceCount: tc.refs, OutputCount: 1}
		check := func(req imagegeneration.Request, code, res string) error {
			return imagegeneration.ValidatePricedRequest(s, req, domain.ProviderAPIMart, code, res)
		}
		if err := check(base, tc.code, tc.resolution); err != nil {
			t.Fatalf("valid priced request rejected: %v", err)
		}
		for _, mutate := range []func(*imagegeneration.Request){
			func(r *imagegeneration.Request) { r.Quality = "4K-max" },
			func(r *imagegeneration.Request) { r.ModelID = "gpt_image_2" },
			func(r *imagegeneration.Request) { r.OutputCount = 2 },
			func(r *imagegeneration.Request) { r.ReferenceCount++ },
		} {
			r := base
			mutate(&r)
			if check(r, tc.code, tc.resolution) == nil {
				t.Fatal("changed pricing dimensions accepted")
			}
		}
		if check(base, "other", "4K") == nil || check(base, tc.code, "4K") == nil {
			t.Fatal("different provider route accepted")
		}
		if tc.refs == 0 {
			r := base
			r.AspectRatio = "1:1"
			if check(r, tc.code, tc.resolution) == nil {
				t.Fatal("unpriced aspect ratio accepted")
			}
			r = base
			r.Prompt = strings.Repeat("я", 2049)
			if check(r, tc.code, tc.resolution) == nil {
				t.Fatal("UTF-8 byte cap bypassed")
			}
		}
	}
}
