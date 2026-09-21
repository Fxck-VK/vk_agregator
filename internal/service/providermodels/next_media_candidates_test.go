package providermodels_test

import (
	"slices"
	"testing"

	"vk-ai-aggregator/internal/service/modelcontract"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestNextSixMediaCandidatesExposeDatedDraftFacts(t *testing.T) {
	tests := []struct {
		publicID string
		kind     string
		native   string
		opID     string
		endpoint string
		version  string
		sourceID string
	}{
		{"wan_3_0", "video", "wan3.0-video", "text_to_video", "POST /v1/videos/generations", "wan3.0-video", "wan_3_0_generation"},
		{"vidu_q3_pro", "video", "viduq3-pro", "text_to_video", "POST /v1/videos/generations", "viduq3-pro", "vidu_q3_pro_generation"},
		{"imagen_4_0", "image", "imagen-4.0-apimart", "generate", "POST /v1/images/generations", "imagen-4.0-apimart", "imagen_4_0_generation"},
		{"lyria_3_5", "audio", "flowmusic-lyria-3.5", "generate", "POST /v1/music/generations", "lyria-3.5", "lyria_3_5_generation"},
		{"gpt_4o_mini_tts", "audio", "gpt-4o-mini-tts", "speak", "POST /v1/audio/speech", "gpt-4o-mini-tts", "gpt_4o_mini_tts_speak"},
		{"whisper_1", "audio", "whisper-1", "transcribe", "POST /v1/audio/transcriptions", "whisper-1", "whisper_1_transcription"},
	}
	for _, tt := range tests {
		candidate, ok := providermodels.MediaCandidateByID(tt.publicID)
		if !ok {
			t.Fatalf("missing candidate %s", tt.publicID)
		}
		if candidate.Kind != tt.kind || candidate.ModelCode != tt.native || candidate.CheckedAt != "2026-09-20" {
			t.Fatalf("%s identity = kind %q native %q checked %q", tt.publicID, candidate.Kind, candidate.ModelCode, candidate.CheckedAt)
		}
		fact, ok := findOperationFact(tt.publicID, tt.opID)
		if !ok {
			t.Fatalf("missing operation fact %s/%s", tt.publicID, tt.opID)
		}
		if fact.Endpoint != tt.endpoint || fact.NativeVersion != tt.version || fact.SourceID != tt.sourceID {
			t.Fatalf("%s/%s fact = %+v", tt.publicID, tt.opID, fact)
		}
		contract := providermodels.DraftMediaContract(candidate)
		if contract.Revision != "2026-09-20" || contract.ProviderModelID != tt.native || contract.Endpoint == "" {
			t.Fatalf("%s contract identity = %+v", tt.publicID, contract)
		}
		for _, source := range contract.Sources {
			if source.CheckedAt != "2026-09-20" {
				t.Fatalf("%s source %s checked_at = %s", tt.publicID, source.ID, source.CheckedAt)
			}
		}
	}
}

func TestNextVideoCandidateDraftsKeepApplicationTextOnlyAndUnknownOutput(t *testing.T) {
	for _, tt := range []struct {
		id              string
		minDuration     int
		maxDuration     int
		resolutions     []string
		firstFactImages bool
	}{
		{"wan_3_0", 2, 30, []string{"480p", "720p", "1080p"}, true},
		{"vidu_q3_pro", 1, 16, []string{"540p", "720p", "1080p"}, true},
	} {
		candidate, _ := providermodels.MediaCandidateByID(tt.id)
		api := candidate.Capabilities.API.Video
		app := candidate.Capabilities.Application.Video
		if api == nil || app == nil {
			t.Fatalf("%s missing video capabilities", tt.id)
		}
		if *api.Duration.MinSeconds != tt.minDuration || *api.Duration.MaxSeconds != tt.maxDuration || !slices.Equal(api.Resolutions, tt.resolutions) {
			t.Fatalf("%s API video caps = %+v", tt.id, api)
		}
		if app.Images.Support != providermodels.Unsupported || app.Videos.Support != providermodels.Unsupported || app.StartFrame != "unsupported" || app.EndFrame != "unsupported" {
			t.Fatalf("%s application should remain text-only: %+v", tt.id, app)
		}
		contract := providermodels.DraftMediaContract(candidate)
		for _, op := range contract.Operations {
			if op.Video == nil {
				continue
			}
			for _, variant := range op.Video.Variants {
				if variant.FPS != 0 || variant.Audio {
					t.Fatalf("%s/%s advertises unverified fps/audio: %+v", tt.id, op.ID, variant)
				}
			}
		}
	}
}

func TestImagenLyriaAndSpeechDraftsKeepKnownBoundsOnly(t *testing.T) {
	imagen, _ := providermodels.MediaCandidateByID("imagen_4_0")
	imageOp := onlyOperation(t, providermodels.DraftMediaContract(imagen), "generate")
	if imageOp.Image == nil || imageOp.Image.MaxOutputCount != 1 {
		t.Fatalf("imagen image output = %+v", imageOp.Image)
	}
	for _, variant := range imageOp.Image.Variants {
		if variant.Resolution != "" {
			t.Fatalf("imagen should not invent pixel resolution: %+v", variant)
		}
	}

	lyria, _ := providermodels.MediaCandidateByID("lyria_3_5")
	lyriaContract := providermodels.DraftMediaContract(lyria)
	if len(lyriaContract.Operations) != 1 || lyriaContract.Operations[0].ID != "generate" || lyriaContract.Operations[0].Audio == nil {
		t.Fatalf("lyria operations = %+v", lyriaContract.Operations)
	}
	if lyriaContract.Operations[0].Audio.MaxDurationSec != 240 || len(lyriaContract.Operations[0].Inputs.Audio.Formats) != 0 {
		t.Fatalf("lyria should expose generate duration only, no audio inputs: %+v", lyriaContract.Operations[0])
	}

	tts, _ := providermodels.MediaCandidateByID("gpt_4o_mini_tts")
	ttsOp := onlyOperation(t, providermodels.DraftMediaContract(tts), "speak")
	if ttsOp.Audio == nil || !slices.Equal(ttsOp.Audio.Voices, []string{"alloy", "echo", "fable", "onyx", "nova", "shimmer"}) || !slices.Equal(ttsOp.Audio.Formats, []string{"wav", "opus", "aac", "flac", "pcm"}) {
		t.Fatalf("tts audio output = %+v", ttsOp.Audio)
	}

	whisper, _ := providermodels.MediaCandidateByID("whisper_1")
	whisperOp := onlyOperation(t, providermodels.DraftMediaContract(whisper), "transcribe")
	if whisperOp.Kind != "text" || whisperOp.Inputs.Audio.Support != modelcontract.Supported || whisperOp.Inputs.Audio.MaxBytes != 25<<20 || whisperOp.Text == nil || whisperOp.Audio != nil {
		t.Fatalf("whisper contract = %+v", whisperOp)
	}
}

func onlyOperation(t *testing.T, contract any, id string) modelcontract.Operation {
	t.Helper()
	c := contract.(modelcontract.Contract)
	for _, op := range c.Operations {
		if op.ID == id {
			return op
		}
	}
	t.Fatalf("operation %s absent in %+v", id, c.Operations)
	return modelcontract.Operation{}
}
