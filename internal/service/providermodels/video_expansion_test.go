package providermodels

import (
	"net/url"
	"strings"
	"testing"
)

func TestVideoExpansionHasExactDocumentedCandidates(t *testing.T) {
	want := map[string]string{"flux_3_video": "flux-3-video", "pixverse_v6": "pixverse-v6", "vidu_q3": "viduq3", "vidu_q3_mix": "viduq3-mix", "vidu_q3_turbo": "viduq3-turbo", "kling_video_o1": "kling-video-o1", "minimax_h3_max": "MiniMax-H3-Max", "wan_3_0_prime": "wan3.0-video-prime", "wan_2_7": "wan2.7", "gemini_omni_flash_preview": "gemini-omni-flash-preview"}
	for id, native := range want {
		c, ok := MediaCandidateByID(id)
		if !ok {
			t.Errorf("missing %s", id)
			continue
		}
		if c.ModelCode != native || c.CheckedAt != "2026-09-30" || !strings.HasPrefix(c.Documentation, "https://docs.apimart.ai/ru/api-reference/videos/") {
			t.Fatalf("wrong dated source for %s", id)
		}
		draft := DraftMediaContract(c)
		if draft.ValidateReady() == nil || len(draft.Operations) == 0 || len(draft.Sources) == 0 {
			t.Fatalf("incorrect draft %s", id)
		}
		for _, source := range draft.Sources {
			if source.ID == id+"_pricing" && source.URL == "https://api.apimart.ai/api/pricing/model?model="+url.QueryEscape(native) && source.CheckedAt == "2026-10-02" {
				continue
			}
			if source.URL != c.Documentation {
				t.Fatalf("unapproved source for %s: %s", id, source.URL)
			}
		}
		for _, op := range draft.Operations {
			if op.Video == nil || len(op.Video.Variants) == 0 {
				t.Fatalf("empty video operation %s/%s", id, op.ID)
			}
		}
		if StaticRegistry().MediaCandidateRunnable(id, "text_to_video") {
			t.Fatal("unverified model became production runnable")
		}
	}
}

func TestVideoExpansionCapabilitiesDoNotInventControls(t *testing.T) {
	standard, _ := MediaCandidateByID("vidu_q3")
	mix, _ := MediaCandidateByID("vidu_q3_mix")
	omni, _ := MediaCandidateByID("gemini_omni_flash_preview")
	if standard.Capabilities.API.Video == nil || mix.Capabilities.API.Video == nil || omni.Capabilities.API.Video == nil {
		t.Fatal("missing candidates")
	}
	if *standard.Capabilities.API.Video.Duration.MinSeconds != 3 || *mix.Capabilities.API.Video.Duration.MinSeconds != 1 {
		t.Fatal("Vidu duration mismatch")
	}
	if strings.Join(mix.Capabilities.API.Video.Resolutions, ",") != "720p,1080p" {
		t.Fatal("Mix advertises 540p")
	}
	if omni.Capabilities.API.Video.Duration.Mode != "automatic" || omni.Capabilities.API.Video.Audio.Mode != "generated" || omni.Capabilities.API.Video.Audio.Selectable {
		t.Fatal("Omni advertised unsupported controls")
	}
	for _, id := range []string{"vidu_q3", "vidu_q3_mix"} {
		for _, fact := range MediaCandidateOperationFacts(id) {
			if fact.ID == "text_to_video" {
				t.Fatal("reference-only model exposed text-to-video")
			}
		}
	}
}
