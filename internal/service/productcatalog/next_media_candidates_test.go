package productcatalog

import (
	"slices"
	"testing"
)

func TestNextPendingVisualCandidates(t *testing.T) {
	list := WorkspaceCatalog(WorkspaceConfig{IncludePendingMedia: true})
	wan := pendingByID(t, list, "wan_3_0")
	if wan.Kind != "video" || len(wan.Operations) != 1 || wan.Operations[0].Enabled {
		t.Fatalf("wan catalog = %+v", wan)
	}
	video := wan.Operations[0].Video
	if video == nil || video.StartImage != "unsupported" || video.EndImage != "unsupported" || !slices.Equal(video.AllowedDurationsSec, intRange(2, 30)) || video.PriceByOption["720p:2"] == 0 {
		t.Fatalf("wan video controls = %+v", video)
	}
	for _, variant := range video.Variants {
		if variant.FPS != nil || variant.Audio != nil {
			t.Fatalf("wan variant advertises unverified output metadata: %+v", variant)
		}
	}

	vidu := pendingByID(t, list, "vidu_q3_pro")
	if video := vidu.Operations[0].Video; video == nil || !slices.Equal(video.AllowedDurationsSec, intRange(1, 16)) || video.PriceByOption["540p:1"] == 0 {
		t.Fatalf("vidu video controls = %+v", video)
	}

	imagen := pendingByID(t, list, "imagen_4_0")
	if imagen.Kind != "image" || len(imagen.Operations) != 1 || imagen.Operations[0].Enabled {
		t.Fatalf("imagen catalog = %+v", imagen)
	}
	image := imagen.Operations[0].Image
	if image == nil || image.DefaultQuality != "standard" || image.MaxOutputCount != 1 || image.PriceByVariant["standard:16:9"] == 0 {
		t.Fatalf("imagen image controls = %+v", image)
	}
	if slices.Contains(image.QualityOptions, "1K") || slices.Contains(image.QualityOptions, "2K") {
		t.Fatalf("imagen invents pixel quality labels: %+v", image.QualityOptions)
	}
}

func TestNextPendingAudioCandidates(t *testing.T) {
	list := WorkspaceCatalog(WorkspaceConfig{IncludePendingMedia: true})
	lyria := pendingByID(t, list, "lyria_3_5")
	if len(lyria.Operations) != 1 || lyria.Operations[0].ID != "generate" || lyria.Operations[0].Music == nil {
		t.Fatalf("lyria operations = %+v", lyria.Operations)
	}
	if lyria.Operations[0].Music.EstimateCredits != 40 || lyria.Operations[0].Music.MaxEstimateCredits != 0 || lyria.Operations[0].Inputs.Audio.Enabled {
		t.Fatalf("lyria generate controls = %+v", lyria.Operations[0])
	}

	tts := pendingByID(t, list, "gpt_4o_mini_tts")
	if len(tts.Operations) != 1 || tts.Operations[0].Enabled || tts.Operations[0].Music != nil {
		t.Fatalf("tts operation = %+v", tts.Operations)
	}
	if audio := tts.Operations[0].Audio; audio == nil || !slices.Equal(audio.Tasks, []string{"speech"}) || !slices.Equal(audio.Voices, []string{"alloy", "echo", "fable", "onyx", "nova", "shimmer"}) {
		t.Fatalf("tts audio controls = %+v", audio)
	}

	whisper := pendingByID(t, list, "whisper_1")
	if len(whisper.Operations) != 1 || whisper.Operations[0].Enabled || whisper.Operations[0].Music != nil || whisper.Operations[0].Inputs.Audio.Enabled {
		t.Fatalf("whisper operation = %+v", whisper.Operations)
	}
	if audio := whisper.Operations[0].Audio; audio == nil || !slices.Equal(audio.Tasks, []string{"transcribe"}) || !slices.Equal(audio.Formats, []string{"json", "text", "srt", "vtt", "verbose_json"}) {
		t.Fatalf("whisper audio controls = %+v", audio)
	}
}

func pendingByID(t *testing.T, list WorkspaceModelList, id string) WorkspaceModel {
	t.Helper()
	for _, item := range list.Items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("pending model %s absent", id)
	return WorkspaceModel{}
}

func intRange(first, last int) []int {
	out := make([]int, 0, last-first+1)
	for n := first; n <= last; n++ {
		out = append(out, n)
	}
	return out
}
