package apimart

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"slices"
	"strings"

	"vk-ai-aggregator/internal/domain"
)

// Imagine is billed per invocation. Resolution carries the server-resolved
// public quality dimension (speed); it is never sent as pixel resolution.
func validateMidjourneyV7(req domain.ProviderRequest, requirePrompt bool) error {
	invalid := func(message string) error { return &Error{Class: domain.ProviderErrInvalidRequest, Message: message} }
	prompt := strings.TrimSpace(req.Prompt)
	if requirePrompt && prompt == "" {
		return invalid("prompt is required")
	}
	// Native flags and permutation syntax can change the billed operation/count.
	if strings.Contains(prompt, "--") || strings.ContainsAny(prompt, "{}") {
		return invalid("Midjourney native flags and permutations are unsupported; use the generation controls")
	}
	if len([]rune(prompt)) > 20000 {
		return invalid("prompt exceeds 20000 characters")
	}
	if !slices.Contains([]string{"relax", "fast", "turbo"}, req.Resolution) {
		return invalid("unsupported Midjourney speed")
	}
	if req.OutputCount < 0 || req.OutputCount > 1 {
		return invalid("Midjourney supports one Imagine invocation")
	}
	if strings.TrimSpace(req.NegativePrompt) != "" {
		return invalid("Midjourney negative prompt is unsupported")
	}
	for _, raw := range []string{req.AspectRatio, req.Size} {
		if value := strings.TrimSpace(raw); value != "" && !slices.Contains([]string{"1:1", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9"}, value) {
			return invalid("unsupported Midjourney aspect ratio")
		}
	}
	if len(req.InputURLs) > 4 {
		return invalid("too many Midjourney reference images")
	}
	for _, input := range req.InputURLs {
		if _, err := validateGenerationImageInput(input, maxGeminiGenerationImageBytes); err != nil {
			return err
		}
		if !strings.HasPrefix(strings.TrimSpace(input), "data:") {
			u, err := url.Parse(strings.TrimSpace(input))
			if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
				return invalid("reference image must be a public HTTPS URL")
			}
			host := strings.ToLower(u.Hostname())
			ip := net.ParseIP(host)
			if host == "localhost" || strings.HasSuffix(host, ".localhost") || (ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsMulticast())) {
				return invalid("reference image must be a public HTTPS URL")
			}
		}
	}
	if len(req.Params) > 0 {
		var params map[string]json.RawMessage
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return invalid("invalid Midjourney parameters")
		}
		// Provider-native fields cannot override server-selected billing dimensions.
		for _, key := range []string{"version", "speed", "quality", "n", "repeat", "extra", "niji", "draft", "hd", "image_urls", "cref", "sref", "dref"} {
			if _, ok := params[key]; ok {
				return invalid("unsupported native Midjourney parameter")
			}
		}
	}
	return nil
}

func (p *Provider) submitMidjourneyV7(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	// APIMart accepts owned reference data URLs directly; no separate upload.
	body, err := json.Marshal(struct {
		Prompt    string   `json:"prompt"`
		Version   string   `json:"version"`
		Speed     string   `json:"speed"`
		Size      string   `json:"size"`
		Quality   string   `json:"quality"`
		ImageURLs []string `json:"image_urls,omitempty"`
	}{strings.TrimSpace(req.Prompt), "7", req.Resolution, effectiveImageSize(req), "1", req.InputURLs})
	if err != nil {
		return domain.ProviderTask{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid Midjourney request"}
	}
	return p.postUnversionedTask(ctx, req, "/midjourney/generations", body)
}
