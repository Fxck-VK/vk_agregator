package pricingcatalog

import (
	"testing"
	"vk-ai-aggregator/internal/domain"
)

func TestMusicCandidateQuotes(t *testing.T) {
	for _, tc := range []struct {
		action        string
		max           bool
		floor, retail int64
	}{
		{"generate", false, 50000, 30}, {"generate", true, 100000, 60},
		{"inspo", false, 68000, 45}, {"sounds", false, 9600, 10},
		{"lyrics", false, 8000, 5}, {"create_model", false, 960000, 580},
		{"stems_all", false, 240000, 145}, {"export", false, 1600, 5},
	} {
		for _, model := range []string{"suno_v6", "suno_v6_wild", "suno_v6_mini"} {
			s, err := MusicCandidateQuote(model, tc.action, tc.max)
			if err != nil || !s.Valid() || s.Floor.Amount != tc.floor || s.InternalCredits != tc.retail {
				t.Errorf("%s/%s max=%v: got floor=%d price=%d err=%v", model, tc.action, tc.max, s.Floor.Amount, s.InternalCredits, err)
			}
		}
	}
	for _, action := range []string{"unknown", "vox", "lyrics", "sounds", "stems_all"} {
		if _, err := MusicCandidateQuote("suno_v6", action, true); err == nil {
			t.Errorf("accepted unsupported Max action %q", action)
		}
	}
	if _, err := MusicCandidateQuote("unknown", "generate", false); err == nil {
		t.Fatal("unknown model accepted")
	}
	key := ProductKey{Operation: domain.OperationAudioMusic, Modality: domain.ModalityAudio, AudioModelID: "suno_v6", AudioAction: "generate"}
	if !key.Valid() {
		t.Fatal("valid music key rejected")
	}
	key.ImageModelID = "other"
	if key.Valid() {
		t.Fatal("mixed music/image price accepted")
	}
	key = ProductKey{Operation: domain.OperationTextGenerate, Modality: domain.ModalityText, TextModelID: "chatgpt", AudioAction: "generate"}
	if key.Valid() {
		t.Fatal("music dimensions accepted for text")
	}
}

func TestCandidatePricesDoNotActivateProducts(t *testing.T) {
	c, err := NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	s, err := MusicCandidateQuote("suno_v6", "generate", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Snapshot(s.Key); err == nil {
		t.Fatal("unadmitted music price became active")
	}
}

func TestVideoCandidateQuotes(t *testing.T) {
	for _, tc := range []struct {
		model, quality, resolution string
		duration                   int
		floor, credits             int64
	}{
		{"happyhorse_1_0", "", "720p", 5, 650000, 390},
		{"happyhorse_1_1", "", "1080p", 5, 860000, 520},
		{"skyreels_v4_fast", "", "480p", 3, 192000, 120},
		{"skyreels_v4_std", "reference_video", "1080p", 10, 5000000, 3000},
	} {
		s, err := MediaVideoCandidateQuote(tc.model, tc.quality, tc.resolution, tc.duration)
		if err != nil || !s.Valid() || s.Floor.Amount != tc.floor || s.InternalCredits != tc.credits {
			t.Errorf("%+v: got floor=%d retail=%d err=%v", tc, s.Floor.Amount, s.InternalCredits, err)
		}
	}
	for _, n := range []int{0, 2, 16} {
		if _, err := MediaVideoCandidateQuote("skyreels_v4_std", "", "720p", n); err == nil {
			t.Errorf("invalid duration %d accepted", n)
		}
	}
	if _, err := MediaVideoCandidateQuote("happyhorse_1_1", "edit", "720p", 5); err == nil {
		t.Fatal("HappyHorse 1.1 EDIT accepted")
	}
	if _, err := MediaVideoCandidateQuote("skyreels_v4_std", "reference_video", "720p", 11); err == nil {
		t.Fatal("reference-video duration above ten accepted")
	}
}
