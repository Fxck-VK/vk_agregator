package domain

import (
	"errors"
	"math"
	"slices"
	"strings"
)

// ValidateSunoToolOptions checks documented options before preparation and again
// at the provider boundary. Native source identifiers are resolved separately.
func ValidateSunoToolOptions(m MusicRequest) error {
	invalid := errors.New("invalid music tool options")
	switch m.Action {
	case MusicActionLyrics:
		if strings.TrimSpace(m.Prompt) == "" || !slices.Contains([]string{"", "classic", "remi"}, m.LyricsModel) {
			return invalid
		}
	case MusicActionSounds:
		if strings.TrimSpace(m.Prompt) == "" || !slices.Contains([]string{"", "one-shot", "loop"}, m.SoundType) || m.BPM < 0 || m.BPM > 300 {
			return invalid
		}
		if m.Key != "" {
			key := strings.TrimSuffix(m.Key, "m")
			if !slices.Contains([]string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}, key) {
				return invalid
			}
		}
	case MusicActionUpsampleTags:
		if strings.TrimSpace(m.Tags) == "" && strings.TrimSpace(m.Style) == "" {
			return invalid
		}
	case MusicActionStems:
		if m.StemType != "" && !slices.Contains(strings.Fields(`lead_vocal drum_kit bass backing_vocals piano electric_guitar percussion string_section synth acoustic_guitar sound_effects synth_pad synth_bass guitar brass_section organ electronic_drum_kit lead_electric_guitar synth_keys rhythm_electric_guitar electric_piano upright_bass keyboards distorted_electric_guitar kick snare risers synth_strings synth_lead woodwinds flute harp tambourine trumpet arpeggiator accordion fiddle pedal_steel_guitar synth_voice violin digital_piano synth_brass mandolin choir banjo bells clarinet tenor_saxophone trombone shaker french_horn glockenspiel electric_bass cello timpani harmonica marimba vibraphone lap_steel_guitar saxophone orchestra horns cymbals hand_clap oboe celesta congas drone alto_saxophone double_bass ukulele harpsichord baritone_saxophone xylophone tuba bass_guitar whistle lead_guitar rhodes 808 bongos bassoon cowbell viola sitar steel_drums piccolo theremin bagpipes hi_hat music_box melodica tabla koto djembe taiko didgeridoo`), m.StemType) {
			return invalid
		}
	case MusicActionUploadExtend:
		if m.ContinueAtSec == nil || math.IsNaN(*m.ContinueAtSec) || math.IsInf(*m.ContinueAtSec, 0) || *m.ContinueAtSec < 1 {
			return invalid
		}
	}
	return nil
}
