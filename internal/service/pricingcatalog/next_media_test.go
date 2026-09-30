package pricingcatalog

import (
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestNextVideoCandidateQuotes(t *testing.T) {
	for _, tc := range []struct {
		model, resolution string
		duration          int
		floor, credits    int64
	}{
		{"wan_3_0", "480p", 2, 65760, 40},
		{"wan_3_0", "1080p", 30, 3945120, 2370},
		{"vidu_q3_pro", "540p", 1, 56000, 35},
		{"vidu_q3_pro", "720p", 16, 1920000, 1155},
	} {
		s, err := MediaVideoCandidateQuote(tc.model, "", tc.resolution, tc.duration)
		if err != nil || !s.Valid() || s.Floor.Amount != tc.floor || s.InternalCredits != tc.credits {
			t.Fatalf("%+v: got floor=%d credits=%d err=%v", tc, s.Floor.Amount, s.InternalCredits, err)
		}
	}
	for _, tc := range []struct {
		model, resolution string
		duration          int
	}{
		{"wan_3_0", "480p", 1},
		{"wan_3_0", "720p", 31},
		{"vidu_q3_pro", "540p", 0},
		{"vidu_q3_pro", "1080p", 17},
		{"vidu_q3_pro", "4k", 8},
	} {
		if _, err := MediaVideoCandidateQuote(tc.model, "", tc.resolution, tc.duration); err == nil {
			t.Fatalf("accepted invalid video quote %+v", tc)
		}
	}
	if _, err := MediaVideoCandidateQuote("wan_3_0", "reference_video", "720p", 5); err == nil {
		t.Fatal("accepted unpriced Wan reference mode")
	}
}

func TestNextImageAndMusicCandidateQuotes(t *testing.T) {
	image, err := ImageCandidateQuote("imagen_4_0")
	if err != nil || !image.Valid() || image.Floor.Amount != 40000 || image.InternalCredits != 25 {
		t.Fatalf("imagen quote floor=%d credits=%d err=%v", image.Floor.Amount, image.InternalCredits, err)
	}
	if image.Key.Operation != domain.OperationImageGenerate || image.Key.Modality != domain.ModalityImage {
		t.Fatalf("imagen key = %+v", image.Key)
	}
	if _, err := ImageCandidateQuote("unknown"); err == nil {
		t.Fatal("unknown image candidate accepted")
	}

	lyria, err := MusicCandidateQuote("lyria_3_5", "generate", false)
	if err != nil || !lyria.Valid() || lyria.Floor.Amount != 60000 || lyria.InternalCredits != 40 {
		t.Fatalf("lyria quote floor=%d credits=%d err=%v", lyria.Floor.Amount, lyria.InternalCredits, err)
	}
	for _, tc := range []struct {
		action string
		max    bool
	}{
		{"generate", true},
		{"extend", false},
		{"lyrics", false},
	} {
		if _, err := MusicCandidateQuote("lyria_3_5", tc.action, tc.max); err == nil {
			t.Fatalf("accepted unsupported lyria quote %+v", tc)
		}
	}
}

func TestNextCandidatePricesDoNotActivateProducts(t *testing.T) {
	c, err := NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []ProductKey{
		{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, VideoRouteAlias: "wan_3_0", Resolution: "720p", DurationSec: 5},
		{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ImageModelID: "imagen_4_0"},
		{Operation: domain.OperationAudioMusic, Modality: domain.ModalityAudio, AudioModelID: "lyria_3_5", AudioAction: "generate"},
	} {
		if _, err := c.Snapshot(key); err == nil {
			t.Fatalf("pending price became active: %+v", key)
		}
	}
}
