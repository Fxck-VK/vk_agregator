package pricingcatalog

import "vk-ai-aggregator/internal/domain"

// Current discounted prices, checked 2026-09-13 at
// https://apimart.ai/api/pricing/model?model=kling-3.0-turbo and
// https://apimart.ai/api/pricing/model?model=MiniMax-H3.
// Millionths of an APIMart credit ($0.10) per output second. T2V and one
// first-frame image have the same price; multimodal inputs are not exposed.
func Kling30TurboProviderRates() map[string]int64 {
	return map[string]int64{"720p": 1144000, "1080p": 1432000}
}

func MiniMaxH3ProviderRates() map[string]int64 {
	return map[string]int64{"768p": 571200, "2k": 914400}
}

func turboH3Tariffs() []ProductPrice {
	var prices []ProductPrice
	for _, entry := range []struct {
		alias       domain.VideoRouteAlias
		resolutions []string
		minimum     int
		rates       map[string]int64
	}{
		{domain.VideoRouteKling30Turbo, []string{"720p", "1080p"}, 3, Kling30TurboProviderRates()},
		{domain.VideoRouteMiniMaxH3, []string{"2k", "768p"}, 4, MiniMaxH3ProviderRates()},
	} {
		for _, resolution := range entry.resolutions {
			for duration := entry.minimum; duration <= 15; duration++ {
				floor := entry.rates[resolution] * int64(duration)
				retail := ((floor*60 + 5*MinorUnitsPerCredit - 1) / (5 * MinorUnitsPerCredit)) * 5
				prices = append(prices, videoTariff(entry.alias, resolution, duration, floor, FloorUnitAPIMartCredits, apimartCreditToInternal, retail))
			}
		}
	}
	return prices
}
