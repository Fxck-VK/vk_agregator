package productcatalog

import (
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/modelcontract"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/textgeneration"
)

// Web transport bounds; provider/model limits may be narrower.
const WebReferenceMaxBytes = 20 << 20
const WebReferenceMaxDimension = 4096

type WorkspaceConfig struct {
	IncludePendingMedia   bool
	ImageReferenceUploads bool
	TextModels            []textgeneration.PublicModel
	ImageModels           []imagegeneration.PublicModel
	VideoRoutes           []VideoRoute
	Pricing               imagegeneration.SnapshotCatalog
}

type WorkspaceModelList struct {
	SchemaVersion  int              `json:"schema_version"`
	DefaultModelID string           `json:"default_model_id"`
	Items          []WorkspaceModel `json:"items"`
}

// WorkspaceModel contains product facts only. Provider routing and admission
// evidence never cross this public boundary.
type WorkspaceModel struct {
	ID           string                            `json:"id"`
	Name         string                            `json:"name"`
	Description  string                            `json:"description"`
	Kind         string                            `json:"kind"`
	Categories   []string                          `json:"categories"`
	Verification string                            `json:"verification"`
	Version      string                            `json:"version,omitempty"`
	Operations   []WorkspaceOperation              `json:"operations"`
	Capabilities *providermodels.ModelCapabilities `json:"capabilities,omitempty"`
}

type WorkspaceOperation struct {
	ID      string               `json:"id"`
	Kind    string               `json:"kind"`
	Enabled bool                 `json:"enabled"`
	Inputs  modelcontract.Inputs `json:"inputs"`
	Text    *WorkspaceText       `json:"text,omitempty"`
	Image   *WorkspaceImage      `json:"image,omitempty"`
	Video   *WorkspaceVideo      `json:"video,omitempty"`
	Audio   *WorkspaceAudio      `json:"audio,omitempty"`
	Music   *WorkspaceMusic      `json:"music,omitempty"`
}

type WorkspaceText struct {
	EstimateCredits int64 `json:"estimate_credits"`
	MaxPromptBytes  int   `json:"max_prompt_bytes,omitempty"`
	MaxOutputTokens int   `json:"max_output_tokens,omitempty"`
	ContextTokens   int   `json:"context_tokens,omitempty"`
}

// Output duration and language coverage stay absent when the API does not
// establish an upper bound for this particular operation.
type WorkspaceAudio struct {
	Tasks          []string `json:"tasks"`
	Languages      []string `json:"languages"`
	Voices         []string `json:"voices"`
	Formats        []string `json:"formats"`
	MaxDurationSec int      `json:"max_duration_sec,omitempty"`
}

type WorkspaceImage struct {
	QualityLabel           string           `json:"quality_label"`
	ShowOutputCount        bool             `json:"show_output_count"`
	MaxPromptBytes         int              `json:"max_prompt_bytes,omitempty"`
	QualityOptions         []string         `json:"quality_options"`
	DefaultQuality         string           `json:"default_quality"`
	AllowedAspectRatios    []string         `json:"allowed_aspect_ratios"`
	DefaultAspectRatio     string           `json:"default_aspect_ratio"`
	MaxOutputCount         int              `json:"max_output_count"`
	SupportsReferenceImage bool             `json:"supports_reference_image"`
	MaxReferenceImages     int              `json:"max_reference_images"`
	PriceByQuality         map[string]int64 `json:"price_by_quality"`
	PriceByVariant         map[string]int64 `json:"price_by_variant"`
}

type WorkspaceVideo struct {
	AllowedResolutions  []string                `json:"allowed_resolutions"`
	AllowedDurationsSec []int                   `json:"allowed_durations_sec"`
	AllowedAspectRatios []string                `json:"allowed_aspect_ratios"`
	DefaultResolution   string                  `json:"default_resolution"`
	DefaultDurationSec  int                     `json:"default_duration_sec"`
	DefaultAspectRatio  string                  `json:"default_aspect_ratio"`
	PriceByOption       map[string]int64        `json:"price_by_option"`
	Variants            []WorkspaceVideoVariant `json:"variants"`
	StartImage          string                  `json:"start_image"`
	EndImage            string                  `json:"end_image"`
}

// Nil means unverified output FPS/audio, not an enabled request control.
type WorkspaceVideoVariant struct {
	DurationSec int    `json:"duration_sec"`
	Resolution  string `json:"resolution"`
	AspectRatio string `json:"aspect_ratio"`
	FPS         *int   `json:"fps"`
	Audio       *bool  `json:"audio"`
}
