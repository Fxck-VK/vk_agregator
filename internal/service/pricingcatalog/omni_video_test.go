package pricingcatalog

import (
	"testing"
	"vk-ai-aggregator/internal/domain"
)

func TestOmniVideoX3Prices(t *testing.T) {
	catalog, err := NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		alias, resolution string
		duration          int
		floor, retail     int64
	}{
		{"video_gemini_omni_1_1_flash", "360p", 10, 2960000, 180},
		{"video_gemini_omni_1_1_flash", "720p", 10, 8800000, 530},
		{"video_gemini_omni_1_1_flash", "1080p", 10, 13200000, 795},
		{"video_gemini_omni_1_1_flash", "4k", 10, 26400000, 1585},
		{"video_gemini_omni_1_1_flash_ext", "360p", 4, 1500000, 90},
		{"video_gemini_omni_1_1_flash_ext", "720p", 6, 3000000, 180},
		{"video_gemini_omni_1_1_flash_ext", "1080p", 8, 3500000, 210},
		{"video_gemini_omni_1_1_flash_ext", "4k", 10, 9000000, 540},
	} {
		key := ProductKey{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, VideoRouteAlias: domain.VideoRouteAlias(tc.alias), Resolution: tc.resolution, DurationSec: tc.duration}
		price, err := catalog.Snapshot(key)
		if err != nil {
			t.Errorf("%s/%s/%d: %v", tc.alias, tc.resolution, tc.duration, err)
			continue
		}
		if price.InternalCredits != tc.retail || price.Floor.Amount != tc.floor {
			t.Errorf("wrong x3 tariff: %+v", price)
		}
	}
}
