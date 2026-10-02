package pricingcatalog

import "testing"

func TestVideoExpansionRetailQuotes(t *testing.T) {
	for _, tc := range []struct {
		model, resolution  string
		seconds            int
		usdMicros, credits int64
	}{
		{"flux_3_video", "720p", 5, 680000, 410},
		{"pixverse_v6", "540p", 5, 120000, 75},
		{"vidu_q3", "720p", 5, 400000, 240},
		{"vidu_q3_mix", "720p", 5, 500000, 300},
		{"vidu_q3_turbo", "720p", 5, 240000, 145},
		{"kling_video_o1", "720p", 5, 336000, 205},
		{"minimax_h3_max", "768p", 5, 285600, 175},
		{"wan_3_0_prime", "1080p", 5, 1028575, 620},
		{"wan_2_7", "1080p", 5, 548000, 330},
		{"gemini_omni_flash_preview", "720p", 10, 880000, 530},
	} {
		q, err := MediaVideoCandidateQuote(tc.model, "", tc.resolution, tc.seconds)
		if err != nil || q.Floor.Amount != tc.usdMicros || q.InternalCredits != tc.credits || !q.Valid() {
			t.Fatalf("%s: floor=%d credits=%d err=%v", tc.model, q.Floor.Amount, q.InternalCredits, err)
		}
		if _, err := MediaVideoCandidateQuote(tc.model, "audio", tc.resolution, tc.seconds); err == nil {
			t.Fatal("unpriced audio switch accepted")
		}
		if _, err := MediaVideoCandidateQuote(tc.model, "", "4k", tc.seconds); err == nil {
			t.Fatal("unpriced resolution accepted")
		}
	}
	for _, tc := range []struct {
		model   string
		seconds int
	}{{"kling_video_o1", 6}, {"flux_3_video", 4}, {"flux_3_video", 21}, {"vidu_q3", 2}, {"vidu_q3_mix", 17}, {"wan_2_7", 16}, {"wan_3_0_prime", 31}, {"gemini_omni_flash_preview", 5}} {
		if _, err := MediaVideoCandidateQuote(tc.model, "", "720p", tc.seconds); err == nil {
			t.Fatalf("%s accepted %ds", tc.model, tc.seconds)
		}
	}
}
