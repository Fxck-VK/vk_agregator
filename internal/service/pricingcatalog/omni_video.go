package pricingcatalog

import "vk-ai-aggregator/internal/domain"

// OmniVideoProviderFloors returns exact preauthorization floors in millionths
// of APIMart credits. Standard uses a 10s budget because duration is automatic.
// Source: https://apimart.ai/ru/pricing, checked 2026-09-12, text/image inputs.
func OmniVideoProviderFloors(ext bool) map[string]map[int]int64 {
	out := map[string]map[int]int64{}
	for _, res := range []string{"360p", "720p", "1080p", "4k"} {
		out[res] = map[int]int64{}
		if !ext {
			out[res][10] = map[string]int64{"360p": 2960000, "720p": 8800000, "1080p": 13200000, "4k": 26400000}[res]
			continue
		}
		base, step := int64(2500000), int64(250000)
		if res == "360p" {
			base, step = 1500000, 125000
		}
		if res == "4k" {
			base = 7500000
		}
		for _, duration := range []int{4, 6, 8, 10} {
			out[res][duration] = base + int64(duration-4)*step
		}
	}
	return out
}

func omniVideoTariffs() []ProductPrice {
	var prices []ProductPrice
	for _, ext := range []bool{false, true} {
		alias := domain.VideoRouteOmni11Flash
		if ext {
			alias = domain.VideoRouteOmni11FlashExt
		}
		floors := OmniVideoProviderFloors(ext)
		for _, res := range []string{"720p", "1080p", "360p", "4k"} {
			for _, duration := range []int{4, 6, 8, 10} {
				floor := floors[res][duration]
				if floor == 0 {
					continue
				}
				retail := ((floor*60 + 5*MinorUnitsPerCredit - 1) / (5 * MinorUnitsPerCredit)) * 5
				prices = append(prices, videoTariff(alias, res, duration, floor, FloorUnitAPIMartCredits, apimartCreditToInternal, retail))
			}
		}
	}
	return prices
}
