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
	VideoRouteSeedance25       VideoRouteAlias = "video_seedance_2_5"
	VideoRouteOmni11Flash      VideoRouteAlias = "video_gemini_omni_1_1_flash"
	VideoRouteOmni11FlashExt   VideoRouteAlias = "video_gemini_omni_1_1_flash_ext"
	VideoRouteKlingV3          VideoRouteAlias = "video_kling_v3"
	VideoRouteKling26Motion    VideoRouteAlias = "video_kling_2_6_motion_control"
	VideoRouteVeo31Fast        VideoRouteAlias = "video_veo_3_1_fast"
	VideoRouteVeo31Quality     VideoRouteAlias = "video_veo_3_1_quality"
	VideoRouteVeo31Lite        VideoRouteAlias = "video_veo_3_1_lite"
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
	SupportsAudio                                    bool             `json:"supports_audio,omitempty"`
	RequiresReferenceVideo                           bool             `json:"requires_reference_video,omitempty"`
	ProviderCostMicrosPerSecondWithAudioByResolution map[string]int64 `json:"provider_cost_micros_per_second_with_audio_by_resolution,omitempty"`
	Alias                                            VideoRouteAlias  `json:"alias"`
	Provider                                         ProviderName     `json:"provider"`
	ProviderModelID                                  string           `json:"provider_model_id"`
	ModelClass                                       string           `json:"model_class"`
	InputModes                                       []VideoInputMode `json:"input_modes,omitempty"`
	RequiresStartImage                               bool             `json:"requires_start_image,omitempty"`
	AutomaticDuration                                bool             `json:"automatic_duration,omitempty"`
	AllowedReferenceImageCounts                      []int            `json:"allowed_reference_image_counts,omitempty"`

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
	// Optional exact rate by resolution, in millionths of a provider credit.
	ProviderCostMicrosPerSecondByResolution map[string]int64         `json:"provider_cost_micros_per_second_by_resolution,omitempty"`
	ProviderCostMicrosByResolutionDuration  map[string]map[int]int64 `json:"provider_cost_micros_by_resolution_duration,omitempty"`
	MaxProviderCostCredits                  int64                    `json:"max_provider_cost_credits,omitempty"`
	MaxInternalCostCredits                  int64                    `json:"max_internal_cost_credits,omitempty"`

	// PriceMultiplier is retained only for legacy route compatibility and
	// safety-cap math until the old route pricing path is removed.
	PriceMultiplier float64 `json:"price_multiplier"`
}

// VideoRouteSnapshot is stored on each resolved job so later route/config
// changes cannot alter the reserved amount or provider request shape.
type VideoRouteSnapshot struct {
	VideoAudio               bool            `json:"video_audio,omitempty"`
	ReferenceVideoArtifactID string          `json:"reference_video_artifact_id,omitempty"`
	CharacterOrientation     string          `json:"character_orientation,omitempty"`
	KeepOriginalSound        bool            `json:"keep_original_sound"`
	Alias                    VideoRouteAlias `json:"alias"`
	Provider                 ProviderName    `json:"provider"`
	ProviderModelID          string          `json:"provider_model_id"`
	ModelClass               string          `json:"model_class"`
	DurationSec              int             `json:"duration_sec"`
	Resolution               string          `json:"resolution,omitempty"`
	AspectRatio              string          `json:"aspect_ratio,omitempty"`
	ProviderCostCredits      int64           `json:"provider_cost_credits"`
	InternalCostCredits      int64           `json:"internal_cost_credits"`
	PriceMultiplier          float64         `json:"price_multiplier"`
	MaxProviderCostCredits   int64           `json:"max_provider_cost_credits,omitempty"`
	MaxInternalCostCredits   int64           `json:"max_internal_cost_credits,omitempty"`
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
	for _, resolution := range r.AllowedResolutions {
		if len(r.ProviderCostMicrosPerSecondByResolution) > 0 && r.ProviderCostMicrosPerSecondByResolution[resolution] <= 0 {
			return fmt.Errorf("video route %s: missing provider rate for %s", r.Alias, resolution)
		}
		if len(r.ProviderCostMicrosByResolutionDuration) > 0 {
			for _, duration := range r.AllowedDurationsSec {
				if r.ProviderCostMicrosByResolutionDuration[resolution][duration] <= 0 {
					return fmt.Errorf("video route %s: missing provider price for %s/%d", r.Alias, resolution, duration)
				}
			}
		}
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
