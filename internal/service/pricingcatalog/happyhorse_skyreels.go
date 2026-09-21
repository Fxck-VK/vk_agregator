package pricingcatalog

import (
	"strings"

	"vk-ai-aggregator/internal/domain"
)

// Exact discounted USD micros per second, checked 2026-09-16 for
// HappyHorse/SkyReels and 2026-09-20 for Wan/Vidu at
// https://api.apimart.ai/api/pricing/model?model=<native-model-id>.
// The caller must use probed inherited duration for EDIT/reference-video;
// client-supplied duration is never authority for inherited-mode billing.
func MediaVideoCandidateQuote(model, mode, resolution string, seconds int) (PricingSnapshot, error) {
	var rates map[string]int64
	minSeconds, maxSeconds := 3, 15
	resolution = strings.ToLower(strings.TrimSpace(resolution))
	switch model {
	case "wan_3_0":
		if mode != "" {
			return PricingSnapshot{}, ErrPriceNotFound
		}
		minSeconds, maxSeconds = 2, 30
		rates = map[string]int64{"480p": 32880, "720p": 65752, "1080p": 131504}
	case "vidu_q3_pro":
		if mode != "" {
			return PricingSnapshot{}, ErrPriceNotFound
		}
		minSeconds, maxSeconds = 1, 16
		rates = map[string]int64{"540p": 56000, "720p": 120000, "1080p": 128000}
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
	if seconds < minSeconds || seconds > maxSeconds {
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
