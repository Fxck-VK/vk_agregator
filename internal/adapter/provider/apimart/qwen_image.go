package apimart

import (
	"encoding/json"
	"strings"

	"vk-ai-aggregator/internal/domain"
)

// Qwen's first public route is one Standard image with no prompt rewriting.
// Keep unsupported tuning fail-closed even when supplied in a stored Job.
func validateQwenImageOptions(req domain.ProviderRequest) error {
	if req.OutputCount < 0 || req.OutputCount > 1 {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "Qwen Image 3 requires one image"}
	}
	for _, value := range []string{req.Resolution, req.Size} {
		if strings.EqualFold(strings.TrimSpace(value), "4K") {
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "Qwen Image 3 supports only 1K and 2K"}
		}
	}
	if len(req.Params) == 0 {
		return nil
	}
	var options struct {
		N                *int   `json:"n"`
		PromptExtend     bool   `json:"prompt_extend"`
		PromptExtendMode string `json:"prompt_extend_mode"`
	}
	if err := json.Unmarshal(req.Params, &options); err != nil {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid Qwen Image params"}
	}
	if (options.N != nil && *options.N != 1) || options.PromptExtend || options.PromptExtendMode != "" {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "Qwen Image 3 requires one image without prompt rewriting"}
	}
	return nil
}

func isQwenImageSize(value string) bool {
	switch strings.ReplaceAll(value, "x", ":") {
	case "1:1", "4:3", "3:4", "16:9", "9:16", "3:2", "2:3":
		return true
	}
	return qwenImagePixelArea(value) > 0
}

func qwenImagePixelArea(value string) int {
	w, h, ok := strings.Cut(strings.ToLower(strings.TrimSpace(value)), "x")
	if !ok {
		return 0
	}
	width, wErr := parsePositiveImageDimension(w)
	height, hErr := parsePositiveImageDimension(h)
	if wErr != nil || hErr != nil || width < 512 || width > 2048 || height < 512 || height > 2048 || width > height*8 || height > width*8 {
		return 0
	}
	return width * height
}
