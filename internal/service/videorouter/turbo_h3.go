package videorouter

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"vk-ai-aggregator/internal/domain"
)

// The initial public contract only accepts text and an owned first-frame
// artifact. Native reference roles, media URLs and callbacks are not public inputs.
func validateTurboH3PublicInput(alias domain.VideoRouteAlias, raw json.RawMessage, imageCount int) error {
	if alias != domain.VideoRouteKling30Turbo && alias != domain.VideoRouteMiniMaxH3 {
		return nil
	}
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil || values == nil {
		return ErrInvalidRouteRequest
	}
	for _, key := range []string{"model", "first_frame_image", "last_frame_image", "image_urls", "image_with_roles", "video_urls", "audio_urls", "callback_url", "webhook", "duration", "size", "ratio", "watermark", "aigc_watermark", "nsfw_check", "audio", "multi_prompt", "multi_shot", "negative_prompt"} {
		if _, ok := values[key]; ok {
			return ErrInvalidRouteRequest
		}
	}
	var prompt string
	if rawPrompt, present := values["prompt"]; present {
		if json.Unmarshal(rawPrompt, &prompt) != nil {
			return ErrInvalidRouteRequest
		}
	}
	prompt = strings.TrimSpace(prompt)
	maximum := 3072
	if alias == domain.VideoRouteMiniMaxH3 {
		maximum = 7000
	}
	if !utf8.ValidString(prompt) || utf8.RuneCountInString(prompt) > maximum || (prompt == "" && (alias == domain.VideoRouteMiniMaxH3 || imageCount == 0)) {
		return ErrInvalidRouteRequest
	}
	return nil
}
