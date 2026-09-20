package pricingcatalog

import "vk-ai-aggregator/internal/domain"

// Exact discounted USD micros per second, checked 2026-09-16 at
// https://api.apimart.ai/api/pricing/model?model=<native-model-id>.
// The caller must use probed inherited duration for EDIT/reference-video;
// client-supplied duration is never authority for inherited-mode billing.
func MediaVideoCandidateQuote(model, mode, resolution string, seconds int) (PricingSnapshot, error) {
	if seconds < 3 || seconds > 15 {
		return PricingSnapshot{}, ErrPriceNotFound
	}
	var rates map[string]int64
	switch model {
	case "happyhorse_1_0":
		if mode != "" && mode != "edit" {
			return PricingSnapshot{}, ErrPriceNotFound
		}
		rates = map[string]int64{"720p": 130000, "1080p": 230000}
	case "happyhorse_1_1":
		if mode != "" {
			return PricingSnapshot{}, ErrPriceNotFound
		}
		rates = map[string]int64{"720p": 130000, "1080p": 172000}
	case "skyreels_v4_fast":
		rates = map[string]int64{"480p": 64000, "720p": 88000, "1080p": 220000}
		if mode == "reference_video" || mode == "extend_video" {
			rates = map[string]int64{"480p": 120000, "720p": 160000, "1080p": 400000}
		}
	case "skyreels_v4_std":
		rates = map[string]int64{"480p": 88000, "720p": 112000, "1080p": 280000}
		if mode == "reference_video" || mode == "extend_video" {
			rates = map[string]int64{"480p": 144000, "720p": 200000, "1080p": 500000}
		}
	default:
		return PricingSnapshot{}, ErrPriceNotFound
	}
	if model == "skyreels_v4_fast" || model == "skyreels_v4_std" {
		if mode != "" && mode != "reference_video" && mode != "extend_video" {
			return PricingSnapshot{}, ErrPriceNotFound
		}
		if mode == "reference_video" && seconds > 10 {
			return PricingSnapshot{}, ErrPriceNotFound
		}
	}
	rate, ok := rates[resolution]
	if !ok {
		return PricingSnapshot{}, ErrPriceNotFound
	}
	return candidateUSDQuote(ProductKey{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, VideoRouteAlias: domain.VideoRouteAlias(model), Quality: mode, Resolution: resolution, DurationSec: seconds}, rate*int64(seconds))
}
