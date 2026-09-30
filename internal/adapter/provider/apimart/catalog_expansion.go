package apimart

import (
	"slices"
	"strings"

	"vk-ai-aggregator/internal/domain"
)

const (
	ModelNanoBanana       = "gemini-2.5-flash-image-preview"
	ModelGrokImagineVideo = "grok-imagine-1.5-video-ext"
	ModelKling26          = "kling-v2-6"
	ModelSeedance20       = "seedance-2.0"
	ModelSeedance20Mini   = "seedance-2.0-mini"
)

// Wire contracts checked against the APIMart generation pages on 2026-09-28.
// Admission, input ownership and billing remain the responsibility of the worker.
type nanoBananaRequest struct {
	Model            string `json:"model"`
	Prompt           string `json:"prompt"`
	Size             string `json:"size"`
	Resolution       string `json:"resolution"`
	N                int    `json:"n"`
	OfficialFallback bool   `json:"official_fallback"`
	NSFWCheck        bool   `json:"nsfw_check"`
}

type catalogVideoRequest struct {
	Model           string             `json:"model"`
	Prompt          string             `json:"prompt,omitempty"`
	Duration        int                `json:"duration"`
	Resolution      string             `json:"resolution,omitempty"`
	Size            string             `json:"size,omitempty"`
	AspectRatio     string             `json:"aspect_ratio,omitempty"`
	Mode            string             `json:"mode,omitempty"`
	Audio           *bool              `json:"audio,omitempty"`
	GenerateAudio   *bool              `json:"generate_audio,omitempty"`
	ImageURLs       []string           `json:"image_urls,omitempty"`
	ImagesWithRoles []catalogImageRole `json:"image_with_roles,omitempty"`
	NSFWCheck       bool               `json:"nsfw_check"`
}

type catalogImageRole struct {
	URL  string `json:"url"`
	Role string `json:"role"`
}

func catalogExpansionID(model string) string {
	switch strings.TrimSpace(model) {
	case ModelNanoBanana:
		return "nano_banana"
	case ModelGrokImagineVideo:
		return "grok_imagine_1_5_video"
	case ModelKling26:
		return "kling_2_6"
	case ModelSeedance20:
		return "seedance_2_0"
	case ModelSeedance20Mini:
		return "seedance_2_0_mini"
	default:
		return ""
	}
}

func catalogExpansionRatios(model string) []string {
	switch model {
	case ModelNanoBanana:
		return []string{"1:1", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9"}
	case ModelGrokImagineVideo:
		return []string{"16:9", "9:16", "1:1", "3:2", "2:3"}
	case ModelKling26:
		return []string{"16:9", "9:16", "1:1"}
	default:
		return []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}
	}
}

func catalogExpansionDuration(req domain.ProviderRequest) int {
	if req.ModelCode == ModelGrokImagineVideo {
		return nextVisualDuration(req.DurationSec, 6)
	}
	return nextVisualDuration(req.DurationSec, 5)
}

func buildCatalogExpansionBody(req domain.ProviderRequest) ([]byte, error) {
	if strings.TrimSpace(string(req.Provider)) != "" && req.Provider != domain.ProviderAPIMart {
		return nil, nextVisualInvalid("APIMart provider mismatch")
	}
	if req.ModelCode == ModelNanoBanana {
		return buildNanoBananaBody(req)
	}
	if err := validateNextVisualVideoCommon(req, req.ModelCode); err != nil {
		return nil, err
	}
	if req.Speech != nil {
		return nil, nextVisualInvalid("speech controls are unsupported")
	}
	if req.VideoMedia != nil && req.VideoMedia.Seed != nil {
		return nil, nextVisualInvalid("seed is not enabled for this route")
	}
	frames, err := nextVisualFrameURLs(req)
	if err != nil {
		return nil, err
	}
	if req.ModelCode == ModelGrokImagineVideo && len(frames) > 1 {
		return nil, nextVisualInvalid("Grok last frame semantics are not documented")
	}
	prompt := strings.TrimSpace(req.Prompt)
	maxPrompt := 4000 // Application bound for Mini and Grok; not a provider claim.
	if req.ModelCode == ModelKling26 {
		maxPrompt = 2500
	}
	if len([]rune(prompt)) > maxPrompt {
		return nil, nextVisualInvalid("prompt exceeds route limit")
	}
	if prompt == "" && (len(frames) == 0 || req.ModelCode == ModelKling26 || req.ModelCode == ModelGrokImagineVideo) {
		return nil, nextVisualInvalid("prompt is required")
	}
	duration := catalogExpansionDuration(req)
	resolution := nextVisualViduResolution(req.Resolution)
	allowedResolutions := []string{"480p", "720p"}
	minSeconds, maxSeconds := 4, 15
	switch req.ModelCode {
	case ModelGrokImagineVideo:
		minSeconds = 6
		if req.VideoAudio {
			return nil, nextVisualInvalid("Grok audio control is undocumented")
		}
	case ModelKling26:
		allowedResolutions = []string{"720p", "1080p"}
		minSeconds, maxSeconds = 5, 10
		if duration != 5 && duration != 10 {
			return nil, nextVisualInvalid("Kling 2.6 duration must be 5 or 10 seconds")
		}
		if resolution == "720p" && (req.VideoAudio || len(frames) > 1) {
			return nil, nextVisualInvalid("Kling 2.6 audio and last frame require pro mode")
		}
		if req.VideoAudio && len(frames) > 1 {
			return nil, nextVisualInvalid("Kling 2.6 audio excludes last frame")
		}
	case ModelSeedance20:
		allowedResolutions = []string{"480p", "720p", "1080p", "4k"}
	case ModelSeedance20Mini:
	default:
		return nil, nextVisualInvalid("unsupported APIMart catalog model")
	}
	if duration < minSeconds || duration > maxSeconds || !slices.Contains(allowedResolutions, resolution) {
		return nil, nextVisualInvalid("unsupported duration or resolution")
	}
	ratio, err := nextVisualRatio(req, catalogExpansionRatios(req.ModelCode), "16:9", "APIMart catalog")
	if err != nil {
		return nil, err
	}
	if err := validateNextVisualVideoParams(req, duration, resolution, ratio); err != nil {
		return nil, err
	}
	body := catalogVideoRequest{Model: req.ModelCode, Prompt: prompt, Duration: duration, Resolution: resolution, Size: ratio, ImageURLs: frames, NSFWCheck: true}
	switch req.ModelCode {
	case ModelKling26:
		body.Resolution = ""
		body.Size = ""
		body.AspectRatio = ratio
		body.Mode = "std"
		body.Audio = &req.VideoAudio
		if resolution == "1080p" {
			body.Mode = "pro"
		}
	case ModelSeedance20, ModelSeedance20Mini:
		body.GenerateAudio = &req.VideoAudio
		body.ImageURLs = nil
		for i, url := range frames {
			role := "first_frame"
			if i == 1 {
				role = "last_frame"
			}
			body.ImagesWithRoles = append(body.ImagesWithRoles, catalogImageRole{URL: url, Role: role})
		}
	}
	return nextVisualMarshal(body, "invalid APIMart catalog video body")
}

func buildNanoBananaBody(req domain.ProviderRequest) ([]byte, error) {
	if req.Operation != domain.OperationImageGenerate || req.Modality != domain.ModalityImage {
		return nil, nextVisualInvalid("unsupported Nano Banana operation")
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" || len([]rune(prompt)) > 1000 {
		return nil, nextVisualInvalid("Nano Banana prompt must contain 1..1000 characters")
	}
	if req.OutputCount < 0 || req.OutputCount > 1 {
		return nil, nextVisualInvalid("Nano Banana produces one image")
	}
	if req.Resolution != "" && req.Resolution != "1K" {
		return nil, nextVisualInvalid("Nano Banana supports 1K only")
	}
	// Owned reference upload/validation is deliberately closed until its separate
	// input-format live checks pass; the API's 14-image support is recorded in metadata.
	if len(req.InputURLs) > 0 || len(req.ReferenceArtifactIDs) > 0 || req.VideoMedia != nil || req.Music != nil || req.Speech != nil || req.VideoAudio || req.Draft || req.KeepOriginalSound || req.DurationSec != 0 || req.CharacterOrientation != "" || req.ReferenceVideoURL != "" || req.NegativePrompt != "" {
		return nil, nextVisualInvalid("unsupported Nano Banana input or control")
	}
	ratio, err := nextVisualRatio(req, catalogExpansionRatios(ModelNanoBanana), "16:9", "Nano Banana")
	if err != nil {
		return nil, err
	}
	if err := validateNextVisualImageParams(req); err != nil {
		return nil, err
	}
	return nextVisualMarshal(nanoBananaRequest{Model: ModelNanoBanana, Prompt: prompt, Size: ratio, Resolution: "1K", N: 1, OfficialFallback: false, NSFWCheck: true}, "invalid Nano Banana body")
}
