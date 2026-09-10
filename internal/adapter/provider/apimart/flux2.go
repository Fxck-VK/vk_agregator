package apimart

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"vk-ai-aggregator/internal/domain"
)

var flux2AspectRatios = []string{"1:1", "4:3", "3:4", "16:9", "9:16", "3:2", "2:3", "21:9", "9:21"}

func validateFlux2Pro(req domain.ProviderRequest, requirePrompt bool) error {
	invalid := func(message string) error { return &Error{Class: domain.ProviderErrInvalidRequest, Message: message} }
	prompt := strings.TrimSpace(req.Prompt)
	if requirePrompt && prompt == "" {
		return invalid("prompt is required")
	}
	if len([]rune(prompt)) > 20000 {
		return invalid("prompt exceeds 20000 characters")
	}
	// Legacy K aliases map to different MP tiers at APIMart. Never infer them.
	if !slices.Contains([]string{"1MP", "2MP", "3MP", "4MP"}, req.Resolution) {
		return invalid("unsupported FLUX.2 resolution")
	}
	if req.OutputCount < 0 || req.OutputCount > 1 {
		return invalid("FLUX.2 supports one output")
	}
	if len(req.InputURLs) > 0 || len(req.ReferenceArtifactIDs) > 0 {
		return invalid("FLUX.2 reference pricing is not enabled")
	}
	if strings.TrimSpace(req.NegativePrompt) != "" {
		return invalid("FLUX.2 negative prompt is unsupported")
	}
	for _, raw := range []string{req.AspectRatio, req.Size} {
		if value := strings.TrimSpace(raw); value != "" && !slices.Contains(flux2AspectRatios, value) {
			return invalid("unsupported FLUX.2 aspect ratio")
		}
	}
	if len(req.Params) > 0 {
		var params map[string]json.RawMessage
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return invalid("invalid FLUX.2 parameters")
		}
		for _, key := range []string{"width", "height", "n", "steps", "guidance", "image_urls", "prompt_upsampling", "output_format", "seed", "safety_tolerance"} {
			if _, ok := params[key]; ok {
				return invalid("unsupported native FLUX.2 parameter")
			}
		}
		// Params also contains trusted worker metadata. Its billed resolution
		// must agree with the normalized field; native overrides are never merged.
		for _, key := range []string{"resolution", "image_quality"} {
			if raw, ok := params[key]; ok {
				var value string
				if json.Unmarshal(raw, &value) != nil || value != req.Resolution {
					return invalid("inconsistent FLUX.2 resolution")
				}
			}
		}
	}
	return nil
}

func (p *Provider) submitFlux2Pro(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	body, err := json.Marshal(struct {
		Model            string `json:"model"`
		Prompt           string `json:"prompt"`
		Resolution       string `json:"resolution"`
		Size             string `json:"size"`
		N                int    `json:"n"`
		OutputFormat     string `json:"output_format"`
		PromptUpsampling bool   `json:"prompt_upsampling"`
	}{ModelFlux2Pro, strings.TrimSpace(req.Prompt), req.Resolution, effectiveImageSize(req), 1, "png", false})
	if err != nil {
		return domain.ProviderTask{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid FLUX.2 request"}
	}
	return p.postUnversionedTask(ctx, req, "/images/generations", body)
}
