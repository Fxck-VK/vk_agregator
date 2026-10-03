package imagegeneration_test

import (
	"errors"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestSingleResultImageResolverRejectsStaleCatalogCounts(t *testing.T) {
	for _, id := range []string{providermodels.PublicImageNanoBanana2, providermodels.PublicImageNanoBananaPro, providermodels.PublicImageGPTImage2} {
		r := imagegeneration.NewResolver([]imagegeneration.PublicModel{{ID: id, Name: id, Enabled: true, Ready: true, QualityOptions: []string{"1K"}, DefaultQuality: "1K", MaxOutputCount: 4, MaxReferenceImages: 16, SupportsReferenceImage: true}}, staticPricingCatalog(t))
		if _, err := r.Resolve(imagegeneration.Request{ModelID: id, OutputCount: 2}); !errors.Is(err, imagegeneration.ErrOutputCountLimit) {
			t.Errorf("%s stale count: got %v, want output limit", id, err)
		}
		if id == providermodels.PublicImageGPTImage2 {
			if _, err := r.Resolve(imagegeneration.Request{ModelID: id, ReferenceCount: 16}); !errors.Is(err, imagegeneration.ErrReferenceLimit) {
				t.Errorf("stale GPT references: got %v, want reference limit", err)
			}
		}
	}
}

func TestSeedreamPromptRejectedBeforePricing(t *testing.T) {
	id := providermodels.PublicImageSeedream45
	r := imagegeneration.NewResolver([]imagegeneration.PublicModel{{ID: id, Name: id, Enabled: true, Ready: true, QualityOptions: []string{"2K"}, DefaultQuality: "2K", MaxOutputCount: 4}}, staticPricingCatalog(t))
	for _, size := range []int{3000, 3001} {
		_, err := r.Resolve(imagegeneration.Request{ModelID: id, Prompt: strings.Repeat("я", size)})
		if (err != nil) != (size > 3000) || (size > 3000 && !errors.Is(err, imagegeneration.ErrPromptTooLong)) {
			t.Errorf("%d Unicode characters: err=%v", size, err)
		}
	}
}
