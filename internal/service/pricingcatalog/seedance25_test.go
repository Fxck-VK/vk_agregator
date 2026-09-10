package pricingcatalog

import (
	"fmt"
	"testing"
	"vk-ai-aggregator/internal/domain"
)

func TestSeedance25ApprovedX3Tariffs(t *testing.T) {
	c, err := NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		resolution string
		rate       int64
		prices     []int64
	}{
		{"480p", 960800, []int64{290, 580, 865, 1730}},
		{"720p", 2160000, []int64{650, 1300, 1945, 3890}},
		{"1080p", 3848800, []int64{1155, 2310, 3465, 6930}},
	} {
		for i, duration := range []int{5, 10, 15, 30} {
			t.Run(fmt.Sprintf("%s/%d", tc.resolution, duration), func(t *testing.T) {
				key := ProductKey{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, VideoRouteAlias: "video_seedance_2_5", Resolution: tc.resolution, DurationSec: duration}
				cost, err := c.CostEstimateCredits(key)
				if err != nil {
					t.Fatal(err)
				}
				if cost != tc.prices[i] {
					t.Fatalf("credits=%d, want %d", cost, tc.prices[i])
				}
				price, err := c.Lookup(key)
				if err != nil {
					t.Fatal(err)
				}
				if price.Floor.Unit != FloorUnitAPIMartCredits || price.Floor.Amount != tc.rate*int64(duration) {
					t.Fatal("provider preauthorization estimate lost precision")
				}
			})
		}
	}
}
