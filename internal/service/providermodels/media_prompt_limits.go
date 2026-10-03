package providermodels

import (
	"strings"
	"unicode/utf8"

	"vk-ai-aggregator/internal/domain"
)

// MediaPromptLimits describes the request bounds of the attached provider,
// checked against PoYo's Seedream 4.5, Runway Gen-4.5 and Seedance Fast schemas
// on 2026-10-03. Zero means no additional model-specific product bound.
func MediaPromptLimits(id string) (minimum, maximum int) {
	switch id {
	case PublicImageSeedream45:
		return 1, 3000
	case string(domain.VideoRouteRunwayGen45):
		return 1, 1800
	case string(domain.VideoRouteSeedance20Fast):
		return 3, 2000
	default:
		return 0, 0
	}
}

func MediaPromptValid(id, prompt string) bool {
	minimum, maximum := MediaPromptLimits(id)
	if minimum == 0 && maximum == 0 {
		return true
	}
	count := utf8.RuneCountInString(strings.TrimSpace(prompt))
	return utf8.ValidString(prompt) && count >= minimum && (maximum == 0 || count <= maximum)
}
