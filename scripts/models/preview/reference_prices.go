package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
)

// Only public customer totals are exported. Provider facts and arithmetic stay
// in the same resolver used by real jobs; the local simulator performs a lookup.
func referencePrices(catalog productcatalog.WorkspaceModelList) (map[string]map[string]int64, error) {
	prices, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		return nil, err
	}
	result := map[string]map[string]int64{}
	for _, model := range catalog.Items {
		for _, op := range model.Operations {
			image := op.Image
			if !op.Enabled || image == nil || !image.SupportsReferenceImage || image.MaxReferenceImages < 1 {
				continue
			}
			resolver := imagegeneration.NewResolver([]imagegeneration.PublicModel{{ID: model.ID, Name: model.Name, Enabled: true, Ready: true,
				QualityOptions: image.QualityOptions, DefaultQuality: image.DefaultQuality, SupportsReferenceImage: true,
				MaxReferenceImages: image.MaxReferenceImages, MaxOutputCount: image.MaxOutputCount, AllowedAspectRatios: image.AllowedAspectRatios}}, prices)
			rows := map[string]int64{}
			for variant := range image.PriceByVariant {
				quality, ratio, _ := strings.Cut(variant, ":")
				for refs := 1; refs <= image.MaxReferenceImages; refs++ {
					for count := 1; count <= image.MaxOutputCount; count++ {
						request := imagegeneration.Request{Prompt: "Local preview image price", ModelID: model.ID, Quality: quality, AspectRatio: ratio, ReferenceCount: refs, OutputCount: count}
						if _, err := resolver.ResolvePublic(request); errors.Is(err, imagegeneration.ErrReferenceLimit) || errors.Is(err, imagegeneration.ErrOutputCountLimit) {
							continue // A combined reference/output limit can exclude this pair.
						} else if err != nil {
							return nil, fmt.Errorf("preview reference admission %s/%s: %w", model.ID, variant, err)
						}
						quote, err := resolver.Resolve(request)
						if err != nil {
							return nil, fmt.Errorf("preview reference price %s/%s/%d/%d: %w", model.ID, variant, refs, count, err)
						}
						rows[variant+":"+strconv.Itoa(refs)+":"+strconv.Itoa(count)] = quote.PricingSnapshot.InternalCredits
					}
				}
			}
			result[model.ID] = rows
		}
	}
	return result, nil
}
