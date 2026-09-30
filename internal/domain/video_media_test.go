package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVideoMediaRequestKeepsEphemeralURLsOutOfJSON(t *testing.T) {
	zero := 0
	media := VideoMediaRequest{
		Mode: VideoMediaModeOmni,
		StartFrame: &VideoFrame{
			URL: "https://127.0.0.1/start.png",
		},
		EndFrame: &VideoFrame{
			URL: "https://private.example/end.png",
		},
		KeyFrames: []VideoKeyFrame{
			{Tag: "@beat", URL: "https://private.example/key.png", TimeStampSec: &zero},
		},
		ReferenceImageGroups: []VideoReferenceImageGroup{
			{
				Tag:              "@actor",
				Type:             VideoReferenceImageTypeImage,
				URLs:             []string{"https://private.example/a.png", "https://private.example/b.png"},
				AudioURL:         "https://private.example/voice.mp3",
				AudioDurationSec: 12,
			},
		},
		ReferenceVideos: []VideoReferenceVideo{
			{
				Tag:         "@move",
				Type:        VideoReferenceVideoTypeReference,
				URL:         "https://private.example/motion.mp4",
				DurationSec: 8,
			},
		},
		Audio: &VideoMediaAudio{
			Setting: VideoAudioSettingOrigin,
			URL:     "https://private.example/source-audio.mp3",
		},
		Seed: &zero,
	}

	raw, err := json.Marshal(media)
	if err != nil {
		t.Fatalf("marshal video media: %v", err)
	}
	reqRaw, err := json.Marshal(ProviderRequest{VideoMedia: &media})
	if err != nil {
		t.Fatalf("marshal provider request with video media: %v", err)
	}
	serialized := string(raw) + "\n" + string(reqRaw)
	for _, forbidden := range []string{
		"127.0.0.1",
		"private.example",
		"image_url",
		"image_urls",
		"video_url",
		"audio_url",
		`"url"`,
	} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("video media JSON leaked ephemeral URL material %q: %s", forbidden, serialized)
		}
	}
	if !strings.Contains(serialized, `"seed":0`) {
		t.Fatalf("explicit seed zero must stay distinguishable from omitted seed: %s", serialized)
	}
	if !strings.Contains(serialized, `"tag":"@actor"`) || !strings.Contains(serialized, `"duration_sec":8`) {
		t.Fatalf("bounded non-URL metadata was not preserved: %s", serialized)
	}
}
