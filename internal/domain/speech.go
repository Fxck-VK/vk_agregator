package domain

import (
	"errors"
	"math"
	"slices"
	"strings"
	"unicode/utf8"
)

// SpeechRequest holds normalized speech controls. File bytes are supplied only
// by the worker from an owned, inspected private input artifact.
type SpeechRequest struct {
	Text          string  `json:"text,omitempty"`
	Voice         string  `json:"voice,omitempty"`
	Format        string  `json:"format,omitempty"`
	Speed         float64 `json:"speed,omitempty"`
	Language      string  `json:"language,omitempty"`
	Temperature   float64 `json:"temperature,omitempty"`
	FileExtension string  `json:"-"`
	FileBytes     []byte  `json:"-"`
}

type InlineAudio struct {
	Bytes     []byte `json:"-"`
	MIME      string `json:"-"`
	Extension string `json:"-"`
}

func ValidateSpeechRequest(s SpeechRequest, op OperationType, requireFile bool) error {
	bad := errors.New("invalid speech request")
	if op == OperationAudioTTS {
		if strings.TrimSpace(s.Text) == "" || !utf8.ValidString(s.Text) || utf8.RuneCountInString(s.Text) > 4096 ||
			!slices.Contains([]string{"alloy", "echo", "fable", "onyx", "nova", "shimmer"}, s.Voice) ||
			!slices.Contains([]string{"wav", "opus", "aac", "flac", "pcm"}, s.Format) ||
			math.IsNaN(s.Speed) || math.IsInf(s.Speed, 0) || s.Speed < 0.25 || s.Speed > 4 || s.Language != "" || s.Temperature != 0 || s.FileExtension != "" || len(s.FileBytes) > 0 {
			return bad
		}
		return nil
	}
	if op != OperationAudioSTT || s.Text != "" || s.Voice != "" || s.Speed != 0 ||
		!slices.Contains([]string{"json", "text", "srt", "vtt", "verbose_json"}, s.Format) || math.IsNaN(s.Temperature) || math.IsInf(s.Temperature, 0) || s.Temperature < 0 || s.Temperature > 1 {
		return bad
	}
	if s.Language != "" && (len(s.Language) != 2 || s.Language[0] < 'a' || s.Language[0] > 'z' || s.Language[1] < 'a' || s.Language[1] > 'z') {
		return bad
	}
	if len(s.FileBytes) > 25<<20 || (requireFile && len(s.FileBytes) == 0) {
		return bad
	}
	if (requireFile || len(s.FileBytes) > 0 || s.FileExtension != "") && !slices.Contains([]string{"mp3", "mp4", "mpeg", "mpga", "m4a", "wav", "webm"}, s.FileExtension) {
		return bad
	}
	return nil
}
