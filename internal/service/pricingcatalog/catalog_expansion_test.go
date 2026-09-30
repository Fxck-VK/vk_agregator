package pricingcatalog

import "testing"

func TestCatalogExpansionQuotes(t *testing.T) {
	for _, tc := range []struct {
		id, mode, res  string
		seconds        int
		floor, credits int64
	}{
		{"grok_imagine_1_5_video", "", "480p", 6, 61200, 40},
		{"grok_imagine_1_5_video", "", "720p", 15, 286800, 175},
		{"kling_2_6", "", "720p", 5, 184000, 115},
		{"kling_2_6", "", "1080p", 10, 625000, 375},
		{"kling_2_6", "pro-sound", "1080p", 5, 625000, 375},
		{"seedance_2_0", "", "4k", 15, 10830000, 6500},
		{"seedance_2_0_mini", "", "720p", 5, 114400, 70},
	} {
		q, err := MediaVideoCandidateQuote(tc.id, tc.mode, tc.res, tc.seconds)
		if err != nil || q.Floor.Amount != tc.floor || q.InternalCredits != tc.credits {
			t.Errorf("%s/%s/%d quote floor=%d credits=%d err=%v", tc.id, tc.res, tc.seconds, q.Floor.Amount, q.InternalCredits, err)
		}
	}
	q, err := ImageCandidateQuote("nano_banana")
	if err != nil || q.Floor.Amount != 12500 || q.InternalCredits != 10 {
		t.Errorf("Nano quote=%+v err=%v", q, err)
	}
	for _, tc := range []struct {
		id, mode, res string
		seconds       int
	}{
		{"grok_imagine_1_5_video", "", "720p", 5}, {"kling_2_6", "", "720p", 6}, {"kling_2_6", "pro-sound", "720p", 5}, {"seedance_2_0_mini", "", "1080p", 5}, {"seedance_2_0", "reference_video", "720p", 5},
	} {
		if _, err := MediaVideoCandidateQuote(tc.id, tc.mode, tc.res, tc.seconds); err == nil {
			t.Errorf("priced invalid combination %+v", tc)
		}
	}
}
