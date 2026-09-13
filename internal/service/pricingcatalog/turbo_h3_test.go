package pricingcatalog

import (
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestTurboH3TariffsQuoteAllDurationsAtX3(t *testing.T) {
	catalog, err := NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		alias             domain.VideoRouteAlias
		res               string
		minimum           int
		rate, fiveSeconds int64
	}{
		{"video_kling_3_0_turbo", "720p", 3, 1144000, 345},
		{"video_kling_3_0_turbo", "1080p", 3, 1432000, 430},
		{"video_minimax_h3", "768p", 4, 571200, 175},
		{"video_minimax_h3", "2k", 4, 914400, 275},
	} {
		for duration := tc.minimum; duration <= 15; duration++ {
			key := ProductKey{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, VideoRouteAlias: tc.alias, Resolution: tc.res, DurationSec: duration}
			price, err := catalog.Lookup(key)
			if err != nil {
				t.Fatalf("missing %s/%s/%ds: %v", tc.alias, tc.res, duration, err)
			}
			assertKlingVeoPrice(t, price, key, tc.rate*int64(duration))
			if duration == 5 {
				credits, err := price.InternalCredits()
				if err != nil || credits != tc.fiveSeconds {
					t.Fatalf("5-second price=%d want%d err%v", credits, tc.fiveSeconds, err)
				}
			}
		}
		for _, duration := range []int{tc.minimum - 1, 16} {
			if _, err := catalog.Lookup(ProductKey{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, VideoRouteAlias: tc.alias, Resolution: tc.res, DurationSec: duration}); err == nil {
				t.Fatal("unsupported duration priced")
			}
		}
		if _, err := catalog.Lookup(ProductKey{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, VideoRouteAlias: tc.alias, Resolution: tc.res, DurationSec: 5, Quality: VideoQualityAudio}); err == nil {
			t.Fatal("undocumented audio option priced")
		}
	}
}
