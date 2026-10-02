package apimart

import (
	"strings"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

func estimateNextVisual(req domain.ProviderRequest) (domain.CostEstimate, error) {
	if err := validateNextVisualRequest(req); err != nil {
		return domain.CostEstimate{}, err
	}
	var quote pricingcatalog.PricingSnapshot
	var err error
	if imageExpansionID(req.ModelCode) != "" {
		quote, err = pricingcatalog.ImageCandidateQualityQuote(imageExpansionID(req.ModelCode), imageExpansionResolution(req))
	} else if spec, ok := videoExpansionSpec(req.ModelCode); ok {
		// Advanced native input modes have separate rates. Only the wired,
		// priced reference-only Vidu routes may include media in a quote.
		if req.ModelCode != ModelViduQ3 && req.ModelCode != ModelViduQ3Mix && (req.VideoMedia != nil || len(req.InputURLs) > 0) {
			return domain.CostEstimate{}, &Error{Class: domain.ProviderErrModelUnavailable, Message: "media input price unavailable"}
		}
		seconds, resolution := videoExpansionOptions(req, spec)
		mode := ""
		if req.VideoAudio {
			mode = "audio"
		}
		quote, err = pricingcatalog.MediaVideoCandidateQuote(spec.id, mode, resolution, seconds)
	} else if req.ModelCode == ModelImagen40 || req.ModelCode == ModelNanoBanana {
		id := "imagen_4_0"
		if req.ModelCode == ModelNanoBanana {
			id = "nano_banana"
		}
		quote, err = pricingcatalog.ImageCandidateQuote(id)
	} else {
		id := "wan_3_0"
		if req.ModelCode == ModelViduQ3Pro {
			id = "vidu_q3_pro"
		}
		if expansionID := catalogExpansionID(req.ModelCode); expansionID != "" {
			id = expansionID
		}
		seconds := req.DurationSec
		if seconds == 0 {
			seconds = 5
			if req.ModelCode == ModelGrokImagineVideo {
				seconds = 6
			}
		}
		resolution := strings.ToLower(req.Resolution)
		if resolution == "" {
			resolution = "720p"
		}
		mode := ""
		if req.ModelCode == ModelKling26 && req.VideoAudio {
			mode = "pro-sound"
		}
		quote, err = pricingcatalog.MediaVideoCandidateQuote(id, mode, resolution, seconds)
	}
	if err != nil {
		return domain.CostEstimate{}, &Error{Class: domain.ProviderErrModelUnavailable, Message: "media price unavailable"}
	}
	return domain.CostEstimate{AmountCredits: (quote.Floor.Amount + 99999) / 100000, Currency: "credits", Estimated: false}, nil
}
