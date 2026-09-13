package pricingcatalog

import "vk-ai-aggregator/internal/domain"

const VideoQualityAudio = "audio"

// APIMart public pricing, checked 2026-09-12. Values are millionths of an
// APIMart credit ($0.10), not internal credits ($0.005).
func KlingV3ProviderRates(audio bool) map[string]int64 {
	if audio {
		return map[string]int64{"720p": 1008000, "1080p": 1344000, "4k": 4285600}
	}
	return map[string]int64{"720p": 672000, "1080p": 896000, "4k": 4285600}
}

func KlingMotionProviderRates() map[string]int64 {
	return map[string]int64{"std": 571200, "pro": 914400}
}

func VeoProviderFloors(alias domain.VideoRouteAlias) map[string]map[int]int64 {
	base, high := int64(1400000), int64(6400000)
	if alias == domain.VideoRouteVeo31Lite {
		base, high = 700000, 5700000
	}
	if alias == domain.VideoRouteVeo31Quality {
		base, high = 10000000, 15000000
	}
	return map[string]map[int]int64{"720p": {8: base}, "1080p": {8: base}, "4k": {8: high}}
}

func klingVeoTariffs() []ProductPrice {
	var out []ProductPrice
	add := func(alias domain.VideoRouteAlias, res string, duration int, floor int64, audio bool) {
		retail := ((floor*60 + 5*MinorUnitsPerCredit - 1) / (5 * MinorUnitsPerCredit)) * 5
		price := videoTariff(alias, res, duration, floor, FloorUnitAPIMartCredits, apimartCreditToInternal, retail)
		if audio {
			price.Key.Quality = VideoQualityAudio
		}
		out = append(out, price)
	}
	for _, audio := range []bool{false, true} {
		rates := KlingV3ProviderRates(audio)
		for _, res := range []string{"720p", "1080p", "4k"} {
			for duration := 3; duration <= 15; duration++ {
				add(domain.VideoRouteKlingV3, res, duration, rates[res]*int64(duration), audio)
			}
		}
	}
	for _, res := range []string{"std", "pro"} {
		for duration := 3; duration <= 30; duration++ {
			add(domain.VideoRouteKling26Motion, res, duration, KlingMotionProviderRates()[res]*int64(duration), false)
		}
	}
	for _, alias := range []domain.VideoRouteAlias{domain.VideoRouteVeo31Fast, domain.VideoRouteVeo31Quality, domain.VideoRouteVeo31Lite} {
		for _, res := range []string{"720p", "1080p", "4k"} {
			add(alias, res, 8, VeoProviderFloors(alias)[res][8], false)
		}
	}
	return out
}
