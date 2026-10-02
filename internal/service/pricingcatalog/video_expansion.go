package pricingcatalog

import "vk-ai-aggregator/internal/domain"

// Gold USD/second, read 2026-10-02 from the official pricing page and
// https://api.apimart.ai/api/pricing/model?model=<exact-provider-id>.
// Round fractional USD micros UP. No membership upgrade discount is assumed.
// Only text generation and Vidu Standard/Mix image references are priced here;
// input-video, continuation, drafts and charged MiniMax references stay closed.
func videoExpansionQuote(model, mode, resolution string, seconds int) (PricingSnapshot, bool, error) {
	minSeconds, maxSeconds := 1, 16
	var rates map[string]int64
	switch model {
	case "flux_3_video":
		minSeconds, maxSeconds = 5, 20
		rates = map[string]int64{"720p": 136000, "1080p": 232000}
	case "pixverse_v6":
		maxSeconds = 15
		rates = map[string]int64{"360p": 16000, "540p": 24000, "720p": 32000, "1080p": 64000}
	case "vidu_q3":
		minSeconds = 3
		rates = map[string]int64{"540p": 40000, "720p": 80000, "1080p": 100000}
	case "vidu_q3_mix":
		rates = map[string]int64{"720p": 100000, "1080p": 120000}
	case "vidu_q3_turbo":
		rates = map[string]int64{"540p": 32000, "720p": 48000, "1080p": 56000}
	case "kling_video_o1":
		minSeconds, maxSeconds = 5, 10
		rates = map[string]int64{"720p": 67200, "1080p": 89600}
		if seconds != 5 && seconds != 10 {
			return PricingSnapshot{}, true, ErrPriceNotFound
		}
	case "minimax_h3_max":
		minSeconds, maxSeconds = 5, 15
		rates = map[string]int64{"480p": 37680, "768p": 57120, "1080p": 128000}
	case "wan_3_0_prime":
		minSeconds, maxSeconds = 2, 30
		rates = map[string]int64{"480p": 51429, "720p": 102857, "1080p": 205715}
	case "wan_2_7":
		minSeconds, maxSeconds = 2, 15
		// The base paid_price is 720p; the explicit 1080P tier overrides it.
		rates = map[string]int64{"720p": 66400, "1080p": 109600}
	case "gemini_omni_flash_preview":
		// Fixed retail quote for the bounded 3..10s automatic output. The
		// provider receives no duration parameter. UI must show automatic mode.
		minSeconds, maxSeconds = 10, 10
		rates = map[string]int64{"720p": 88000}
	default:
		return PricingSnapshot{}, false, nil
	}
	if mode != "" || seconds < minSeconds || seconds > maxSeconds || rates[resolution] <= 0 {
		return PricingSnapshot{}, true, ErrPriceNotFound
	}
	q, err := candidateUSDQuote(ProductKey{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, VideoRouteAlias: domain.VideoRouteAlias(model), Resolution: resolution, DurationSec: seconds}, rates[resolution]*int64(seconds))
	return q, true, err
}
