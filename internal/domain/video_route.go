package domain

import (
	"fmt"
	"strings"
)

// VideoRouteAlias is the public, stable product route accepted from trusted
// server-side catalogs. It is intentionally separate from provider model ids.
type VideoRouteAlias string

const (
	VideoRouteHailuo23Fast     VideoRouteAlias = "video_hailuo_2_3_fast"
	VideoRouteHailuo23Standard VideoRouteAlias = "video_hailuo_2_3_standard"
	VideoRouteKlingO3Standard  VideoRouteAlias = "video_kling_o3_standard"
	VideoRouteRunwayGen4Turbo  VideoRouteAlias = "video_runway_gen4_turbo"
	VideoRouteSeedance20Fast   VideoRouteAlias = "video_seedance_2_0_fast"
	VideoRouteRunwayGen45      VideoRouteAlias = "video_runway_gen4_5"
	VideoRouteMockTextToVideo  VideoRouteAlias = "video_mock_text_to_video"
)

// VideoInputMode describes the input shape a route supports.
type VideoInputMode string

const (
	VideoInputText        VideoInputMode = "text"
	VideoInputImage       VideoInputMode = "image"
	VideoInputVideo       VideoInputMode = "video"
	VideoInputReference   VideoInputMode = "reference"
	VideoInputAudioPrompt VideoInputMode = "audio_prompt"
)

// VideoRouteSpec is the hidden provider route catalog entry. Frontends see
// aliases only; provider ids stay server-side and are used later by workers.
type VideoRouteSpec struct {
	Alias              VideoRouteAlias  `json:"alias"`
	Provider           ProviderName     `json:"provider"`
	ProviderModelID    string           `json:"provider_model_id"`
	ModelClass         string           `json:"model_class"`
	InputModes         []VideoInputMode `json:"input_modes,omitempty"`
	RequiresStartImage bool             `json:"requires_start_image,omitempty"`

	AllowedDurationsSec    []int            `json:"allowed_durations_sec,omitempty"`
	AllowedResolutions     []string         `json:"allowed_resolutions,omitempty"`
	AllowedAspectRatios    []string         `json:"allowed_aspect_ratios,omitempty"`
	ResolutionDurationsSec map[string][]int `json:"resolution_durations_sec,omitempty"`

	SupportsReferenceImage bool `json:"supports_reference_image,omitempty"`
	SupportsReferenceVideo bool `json:"supports_reference_video,omitempty"`
	SupportsReferenceAudio bool `json:"supports_reference_audio,omitempty"`
	MaxReferenceImages     int  `json:"max_reference_images,omitempty"`

	// Provider cost fields are backend-only route safety metadata used to
	// validate provider spend and build immutable worker snapshots. Public user
	// prices come from pricingcatalog, not from these route fields.
	ProviderCostCreditsFixed     int64 `json:"provider_cost_credits_fixed,omitempty"`
	ProviderCostCreditsPerSecond int64 `json:"provider_cost_credits_per_second,omitempty"`
	MaxProviderCostCredits       int64 `json:"max_provider_cost_credits,omitempty"`
	MaxInternalCostCredits       int64 `json:"max_internal_cost_credits,omitempty"`

	// PriceMultiplier is retained only for legacy route compatibility and
	// safety-cap math until the old route pricing path is removed.
	PriceMultiplier float64 `json:"price_multiplier"`
}

// VideoRouteSnapshot is stored on each resolved job so later route/config
// changes cannot alter the reserved amount or provider request shape.
type VideoRouteSnapshot struct {
	Alias                  VideoRouteAlias `json:"alias"`
	Provider               ProviderName    `json:"provider"`
	ProviderModelID        string          `json:"provider_model_id"`
	ModelClass             string          `json:"model_class"`
	DurationSec            int             `json:"duration_sec"`
	Resolution             string          `json:"resolution,omitempty"`
	AspectRatio            string          `json:"aspect_ratio,omitempty"`
	ProviderCostCredits    int64           `json:"provider_cost_credits"`
	InternalCostCredits    int64           `json:"internal_cost_credits"`
	PriceMultiplier        float64         `json:"price_multiplier"`
	MaxProviderCostCredits int64           `json:"max_provider_cost_credits,omitempty"`
	MaxInternalCostCredits int64           `json:"max_internal_cost_credits,omitempty"`
}

// Valid reports whether the snapshot has the minimum data needed by workers.
func (s VideoRouteSnapshot) Valid() bool {
	return strings.TrimSpace(string(s.Alias)) != "" &&
		strings.TrimSpace(string(s.Provider)) != "" &&
		strings.TrimSpace(s.ProviderModelID) != "" &&
		s.InternalCostCredits > 0
}

// Validate reports malformed route catalog entries before they can be used.
func (r VideoRouteSpec) Validate() error {
	if strings.TrimSpace(string(r.Alias)) == "" {
		return fmt.Errorf("video route: alias is required")
	}
	if strings.TrimSpace(string(r.Provider)) == "" {
		return fmt.Errorf("video route %s: provider is required", r.Alias)
	}
	if strings.TrimSpace(r.ProviderModelID) == "" {
		return fmt.Errorf("video route %s: provider_model_id is required", r.Alias)
	}
	if strings.TrimSpace(r.ModelClass) == "" {
		return fmt.Errorf("video route %s: model_class is required", r.Alias)
	}
	if len(r.AllowedDurationsSec) == 0 {
		return fmt.Errorf("video route %s: allowed_durations_sec is required", r.Alias)
	}
	for _, duration := range r.AllowedDurationsSec {
		if duration <= 0 {
			return fmt.Errorf("video route %s: duration must be positive", r.Alias)
		}
	}
	if r.ProviderCostCreditsFixed < 0 {
		return fmt.Errorf("video route %s: provider_cost_credits_fixed must be non-negative", r.Alias)
	}
	if r.ProviderCostCreditsPerSecond < 0 {
		return fmt.Errorf("video route %s: provider_cost_credits_per_second must be non-negative", r.Alias)
	}
	if r.MaxProviderCostCredits < 0 {
		return fmt.Errorf("video route %s: max_provider_cost_credits must be non-negative", r.Alias)
	}
	if r.MaxInternalCostCredits < 0 {
		return fmt.Errorf("video route %s: max_internal_cost_credits must be non-negative", r.Alias)
	}
	if r.MaxReferenceImages < 0 {
		return fmt.Errorf("video route %s: max_reference_images must be non-negative", r.Alias)
	}
	if r.PriceMultiplier <= 0 {
		return fmt.Errorf("video route %s: price_multiplier must be positive", r.Alias)
	}
	return nil
}
