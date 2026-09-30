package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMusicActionValidityMatchesSunoContract(t *testing.T) {
	actions := []MusicAction{
		MusicActionGenerate,
		MusicActionLyrics,
		MusicActionInspo,
		MusicActionSounds,
		MusicActionUpsampleTags,
		MusicActionUpload,
		MusicActionUploadCover,
		MusicActionUploadExtend,
		MusicActionCreateModel,
		MusicActionExtend,
		MusicActionCover,
		MusicActionRemaster,
		MusicActionStems,
		MusicActionStemsAll,
		MusicActionAddVocals,
		MusicActionAddInstrumental,
		MusicActionAddStem,
		MusicActionVoice,
		MusicActionPersona,
		MusicActionReplaceSection,
		MusicActionRemoveSection,
		MusicActionCrop,
		MusicActionFadeIn,
		MusicActionFadeOut,
		MusicActionAdjustSpeed,
		MusicActionConcat,
		MusicActionMashup,
		MusicActionSample,
		MusicActionMIDI,
		MusicActionAlignedLyrics,
		MusicActionBPM,
		MusicActionGenerateVideo,
		MusicActionExport,
	}

	seen := map[MusicAction]bool{}
	for _, action := range actions {
		if !action.Valid() {
			t.Fatalf("%q should be valid", action)
		}
		if seen[action] {
			t.Fatalf("duplicate action %q", action)
		}
		seen[action] = true
	}
	for _, action := range []MusicAction{"vox", "upload_cover_song", ""} {
		if action.Valid() {
			t.Fatalf("%q should be invalid", action)
		}
	}
}

func TestMusicRequestJSONKeepsZeroWeightsAndOmitsEphemeralURLs(t *testing.T) {
	zero := 0.0
	req := MusicRequest{
		Action:      MusicActionGenerate,
		Custom:      boolPtr(true),
		Lyrics:      "la la",
		Title:       "Zero Weights",
		StyleWeight: &zero,
		Weirdness:   &zero,
		AudioWeight: &zero,
		AudioURL:    "https://private.example/input.mp3?token=secret",
		AudioURLs:   []string{"https://private.example/ref-1.mp3?token=secret"},
	}

	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal music request: %v", err)
	}
	serialized := string(raw)
	for _, want := range []string{`"style_weight":0`, `"weirdness":0`, `"audio_weight":0`} {
		if !strings.Contains(serialized, want) {
			t.Fatalf("serialized request lost explicit zero %s: %s", want, serialized)
		}
	}
	for _, forbidden := range []string{"private.example", "token=secret", "audio_url", "audio_urls"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("serialized request contains ephemeral URL material %q: %s", forbidden, serialized)
		}
	}
}

func boolPtr(value bool) *bool {
	return &value
}
