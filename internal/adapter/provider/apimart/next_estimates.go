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
	if req.ModelCode == ModelImagen40 {
		quote, err = pricingcatalog.ImageCandidateQuote("imagen_4_0")
	} else {
		id := "wan_3_0"
		if req.ModelCode == ModelViduQ3Pro {
			id = "vidu_q3_pro"
		}
		seconds := req.DurationSec
		if seconds == 0 {
			seconds = 5
		}
		resolution := strings.ToLower(req.Resolution)
		if resolution == "" {
			resolution = "720p"
		}
		quote, err = pricingcatalog.MediaVideoCandidateQuote(id, "", resolution, seconds)
	}
	if err != nil {
		return domain.CostEstimate{}, &Error{Class: domain.ProviderErrModelUnavailable, Message: "media price unavailable"}
	}
	return domain.CostEstimate{AmountCredits: (quote.Floor.Amount + 99999) / 100000, Currency: "credits", Estimated: false}, nil
}
