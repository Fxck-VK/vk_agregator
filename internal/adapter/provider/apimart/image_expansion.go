package apimart

import (
	"slices"
	"strings"

	"vk-ai-aggregator/internal/domain"
)

const (
	ModelSeedream50Flash = "seedream-5-0-flash"
	ModelZImageTurbo     = "z-image-turbo"
	ModelFlux2Max        = "flux-2-max"
	ModelFlux2Flex       = "flux-2-flex"
	ModelQwenImage3Pro   = "qwen-image-3.0-pro"
)

func imageExpansionID(model string) string {
	switch model {
	case ModelSeedream50Flash:
		return "seedream_5_0_flash"
	case ModelZImageTurbo:
		return "z_image_turbo"
	case ModelFlux2Max:
		return "flux_2_max"
	case ModelFlux2Flex:
		return "flux_2_flex"
	case ModelQwenImage3Pro:
		return "qwen_image_3_pro"
	default:
		return ""
	}
}

func imageExpansionResolution(req domain.ProviderRequest) string {
	if req.Resolution != "" {
		return req.Resolution
	}
	if req.ModelCode == ModelFlux2Max || req.ModelCode == ModelFlux2Flex {
		return "1MP"
	}
	return "1K"
}

// Dated wire facts: providermodels/image_expansion.go. The application exposes
// one priced text-to-image output. Never merge native client params into this body.
func buildImageExpansionBody(req domain.ProviderRequest) ([]byte, error) {
	if req.Provider != "" && req.Provider != domain.ProviderAPIMart {
		return nil, nextVisualInvalid("APIMart provider mismatch")
	}
	if imageExpansionID(req.ModelCode) == "" || req.Operation != domain.OperationImageGenerate || req.Modality != domain.ModalityImage {
		return nil, nextVisualInvalid("unsupported image operation")
	}
	if len(req.InputURLs) > 0 || len(req.ReferenceArtifactIDs) > 0 || req.VideoMedia != nil || req.Music != nil || req.Speech != nil || req.VideoAudio || req.Draft || req.KeepOriginalSound || req.DurationSec != 0 || req.CharacterOrientation != "" || req.ReferenceVideoURL != "" || req.NegativePrompt != "" {
		return nil, nextVisualInvalid("unsupported image input or control")
	}
	if req.OutputCount < 0 || req.OutputCount > 1 {
		return nil, nextVisualInvalid("one image per request required")
	}
	maxPrompt := 4000 // Conservative application character bound, not a native token limit.
	if req.ModelCode == ModelZImageTurbo {
		maxPrompt = 800
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" || len([]rune(prompt)) > maxPrompt {
		return nil, nextVisualInvalid("prompt exceeds image route limit")
	}
	resolutions := []string{"1K", "2K"}
	ratios := []string{"1:1", "4:3", "3:4", "16:9", "9:16", "3:2", "2:3"}
	switch req.ModelCode {
	case ModelSeedream50Flash:
		resolutions = []string{"1K", "1.5K", "2K"}
		ratios = append(ratios, "2:1", "1:2", "21:9")
	case ModelFlux2Max, ModelFlux2Flex:
		resolutions = []string{"1MP", "2MP", "3MP", "4MP"}
		ratios = append(ratios, "21:9", "9:21")
	}
	resolution := imageExpansionResolution(req)
	if !slices.Contains(resolutions, resolution) {
		return nil, nextVisualInvalid("unsupported image resolution")
	}
	ratio, err := nextVisualRatio(req, ratios, "16:9", "APIMart image")
	if err != nil {
		return nil, err
	}
	params, err := nextVisualParseParams(req.Params)
	if err != nil {
		return nil, err
	}
	for key := range params {
		if !allowedNextVisualImageParam(key) && key != "resolution" && key != "image_quality" {
			return nil, nextVisualInvalid("unsupported native image parameter")
		}
	}
	for key, value := range map[string]string{"provider": string(domain.ProviderAPIMart), "model_code": req.ModelCode, "model_id": req.ModelCode, "model_name": req.ModelCode, "provider_model_id": req.ModelCode, "size": ratio, "aspect_ratio": ratio, "resolution": resolution, "image_quality": resolution} {
		if err := nextVisualMatchStringParam(params, key, value); err != nil {
			return nil, err
		}
	}
	if err := nextVisualMatchIntParam(params, "output_count", 1); err != nil {
		return nil, err
	}
	body := map[string]any{"model": req.ModelCode, "prompt": prompt, "resolution": resolution, "size": ratio, "nsfw_check": true}
	if req.ModelCode != ModelZImageTurbo {
		body["n"] = 1
	}
	switch req.ModelCode {
	case ModelSeedream50Flash:
		body["output_format"] = "png"
	case ModelFlux2Max, ModelFlux2Flex:
		body["output_format"] = "png"
		body["prompt_upsampling"] = false
	case ModelZImageTurbo, ModelQwenImage3Pro:
		body["prompt_extend"] = false
	}
	return nextVisualMarshal(body, "invalid image request")
}
