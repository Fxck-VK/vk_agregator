package musicgeneration

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

func TestToolOptionsRejectedBeforePreparation(t *testing.T) {
	for _, music := range []domain.MusicRequest{
		{Action: domain.MusicActionLyrics},
		{Action: domain.MusicActionLyrics, Prompt: "topic", LyricsModel: "unknown"},
		{Action: domain.MusicActionSounds},
		{Action: domain.MusicActionSounds, Prompt: "rain", SoundType: "unknown"},
		{Action: domain.MusicActionSounds, Prompt: "rain", BPM: 301},
		{Action: domain.MusicActionSounds, Prompt: "rain", BPM: -1},
		{Action: domain.MusicActionSounds, Prompt: "rain", Key: "Db"},
		{Action: domain.MusicActionSounds, Prompt: "rain", Key: "B#"},
		{Action: domain.MusicActionUploadExtend},
		{Action: domain.MusicActionStems, StemType: "unknown"},
		{Action: domain.MusicActionCover, GPTDescription: strings.Repeat("я", 3001)},
	} {
		r := Request{ModelID: "suno_v6", Music: music}
		op, _ := OperationByID(music.Action)
		for range op.MinSources {
			r.Sources = append(r.Sources, Source{JobID: uuid.New(), AudioIndex: 1})
		}
		for range op.MinUploads {
			r.AudioArtifactIDs = append(r.AudioArtifactIDs, uuid.New())
		}
		if r.Validate() == nil {
			t.Errorf("invalid tool options accepted for %s", music.Action)
		}
	}
	for _, music := range []domain.MusicRequest{
		{Action: domain.MusicActionLyrics, Prompt: "topic", LyricsModel: "remi"},
		{Action: domain.MusicActionSounds, Prompt: "rain", SoundType: "loop", BPM: 300, Key: "C#m"},
	} {
		if (Request{ModelID: "suno_v6", Music: music}).Validate() != nil {
			t.Fatalf("valid %s options rejected", music.Action)
		}
	}
}

func TestUploadContinuationUsesInspectedMetadata(t *testing.T) {
	at := 119.99
	m := domain.MusicRequest{Action: domain.MusicActionUploadExtend, ContinueAtSec: &at}
	for _, tc := range []struct {
		duration int64
		mime     string
		valid    bool
	}{
		{120000, "audio/mpeg", true}, {120000, "audio/wav", true},
		{119990, "audio/mpeg", false}, {480000, "audio/wav", false}, {120000, "audio/aac", false},
	} {
		if got := ValidateUpload(m, &domain.Artifact{DurationMS: tc.duration, MimeType: tc.mime}) == nil; got != tc.valid {
			t.Fatalf("upload duration=%d mime=%s accepted=%v", tc.duration, tc.mime, got)
		}
	}
}
