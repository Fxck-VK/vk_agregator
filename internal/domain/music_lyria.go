package domain

import (
	"errors"
	"reflect"
	"strings"
	"unicode/utf8"
)

// ValidateLyriaMusicRequest restricts the first product operation to the native
// Lyria generation fields. Text bounds are local policy, not provider claims.
func ValidateLyriaMusicRequest(m MusicRequest) error {
	allowed := MusicRequest{Action: m.Action, Prompt: m.Prompt, Lyrics: m.Lyrics, Title: m.Title, Style: m.Style, DurationSec: m.DurationSec, BPM: m.BPM}
	if !reflect.DeepEqual(m, allowed) || m.Action != MusicActionGenerate ||
		(strings.TrimSpace(m.Prompt) == "" && strings.TrimSpace(m.Style) == "" && strings.TrimSpace(m.Lyrics) == "") ||
		utf8.RuneCountInString(m.Prompt) > 5000 || utf8.RuneCountInString(m.Lyrics) > 5000 || utf8.RuneCountInString(m.Style) > 1000 || utf8.RuneCountInString(m.Title) > 80 ||
		m.DurationSec < 0 || m.DurationSec > 240 || m.BPM < 0 {
		return errors.New("invalid Lyria music request")
	}
	return nil
}
