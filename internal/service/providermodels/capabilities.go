package providermodels

// Support distinguishes an unavailable feature from an unverified provider claim.
type Support string

const (
	Supported   Support = "supported"
	Unsupported Support = "unsupported"
	Unknown     Support = "unknown"
)

// InputCapability lists file extensions accepted by this particular integration.
// A nil maximum or empty extensions on an unknown API input means unverified,
// not unlimited. Extensions describe content formats; uploads still verify MIME.
type InputCapability struct {
	Support    Support  `json:"support"`
	Extensions []string `json:"extensions"`
	MaxCount   *int     `json:"max_count"`
}

type TextCapabilities struct {
	Images InputCapability `json:"images"`
	Videos InputCapability `json:"videos"`
	Files  InputCapability `json:"files"`
}

type ImageCapabilities struct {
	Images            InputCapability `json:"images"`
	AspectRatios      []string        `json:"aspect_ratios"`
	Resolutions       []string        `json:"resolutions"`
	QualityModes      []string        `json:"quality_modes"`
	SpeedModes        []string        `json:"speed_modes"`
	MaxOutputCount    *int            `json:"max_output_count"`
	MaxCombinedImages *int            `json:"max_combined_images"`
}

type DurationCapability struct {
	Mode           string           `json:"mode"` // selected, automatic, reference_video, unknown
	MinSeconds     *int             `json:"min_seconds"`
	MaxSeconds     *int             `json:"max_seconds"`
	AllowedSeconds []int            `json:"allowed_seconds"`
	ByResolution   map[string][]int `json:"by_resolution,omitempty"`
	ByOrientation  map[string]int   `json:"max_by_orientation,omitempty"`
}

type VideoAudioCapability struct {
	Mode       string `json:"mode"` // optional, generated, silent, preserve_source, unknown
	Selectable bool   `json:"selectable"`
}

type VideoCapabilities struct {
	Images             InputCapability      `json:"images"`
	Videos             InputCapability      `json:"videos"`
	AllowedImageCounts []int                `json:"allowed_image_counts"`
	Duration           DurationCapability   `json:"duration"`
	Resolutions        []string             `json:"resolutions"`
	QualityModes       []string             `json:"quality_modes"`
	AspectRatios       []string             `json:"aspect_ratios"`
	Audio              VideoAudioCapability `json:"audio"`
	StartFrame         string               `json:"start_frame"` // required, optional, unsupported, unknown
	EndFrame           string               `json:"end_frame"`
}

type AudioCapabilities struct {
	Audio  InputCapability `json:"audio"`
	Videos InputCapability `json:"videos"`
	Output string          `json:"output"`
}

// CapabilityProfile is a discriminated union: exactly one purpose is populated.
// API records describe verified API contracts; unknown fields must stay unknown.
// Application records describe the supported request path, not rollout readiness.
type CapabilityProfile struct {
	Text  *TextCapabilities  `json:"text,omitempty"`
	Image *ImageCapabilities `json:"image,omitempty"`
	Video *VideoCapabilities `json:"video,omitempty"`
	Audio *AudioCapabilities `json:"audio,omitempty"`
	Notes []string           `json:"notes,omitempty"`
}

type ModelCapabilities struct {
	SchemaVersion int               `json:"schema_version"`
	API           CapabilityProfile `json:"api"`
	Application   CapabilityProfile `json:"application"`
}

func integer(n int) *int { return &n }

func inputCapability(support Support, maximum *int, extensions ...string) InputCapability {
	return InputCapability{Support: support, MaxCount: maximum, Extensions: append([]string{}, extensions...)}
}

func noInput() InputCapability      { return inputCapability(Unsupported, integer(0)) }
func unknownInput() InputCapability { return inputCapability(Unknown, nil) }

// Capabilities returns a fresh, safe metadata value with no provider identifiers,
// secrets, URLs, or prices. It does not enable models or broaden accepted input.
func Capabilities(publicID string) *ModelCapabilities {
	r := StaticRegistry()
	for _, m := range r.TextAliases {
		if m.PublicID == publicID {
			return textCapabilities(m)
		}
	}
	for _, m := range r.ImageModels {
		if m.PublicID == publicID {
			return imageCapabilities(m)
		}
	}
	for _, m := range r.VideoRouteModels {
		if string(m.Alias) == publicID && !m.LoadTestOnly {
			return videoCapabilities(m)
		}
	}
	return nil
}
