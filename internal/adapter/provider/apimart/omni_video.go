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

const (
	ModelOmni11Flash    = "gemini-omni-1.1-flash"
	ModelOmni11FlashExt = "gemini-omni-1.1-flash-ext"
)

func isOmniVideo(model string) bool { return model == ModelOmni11Flash || model == ModelOmni11FlashExt }

// These are deliberately separate from Seedance's request fields. Standard
// Omni has no duration parameter; EXT requires discrete durations and counts.
type omniVideoRequest struct {
	Model          string   `json:"model"`
	Prompt         string   `json:"prompt,omitempty"`
	Resolution     string   `json:"resolution"`
	AspectRatio    string   `json:"aspect_ratio"`
	Duration       int      `json:"duration,omitempty"`
	ImageURLs      []string `json:"image_urls,omitempty"`
	GenerationType string   `json:"generation_type,omitempty"`
	NSFWCheck      bool     `json:"nsfw_check,omitempty"`
}

func validateOmniVideo(req domain.ProviderRequest, requirePrompt bool) error {
	invalid := func(message string) error { return &Error{Class: domain.ProviderErrInvalidRequest, Message: message} }
	if req.Operation != domain.OperationVideoGenerate || req.Modality != domain.ModalityVideo || !isOmniVideo(req.ModelCode) {
		return invalid("unsupported Omni video operation")
	}
	ext := req.ModelCode == ModelOmni11FlashExt
	if requirePrompt && strings.TrimSpace(req.Prompt) == "" && (ext || len(req.InputURLs) == 0) {
		return invalid("Omni video prompt or image is required")
	}
	// Standard duration is only our preauthorization window, never sent upstream.
	if (!ext && req.DurationSec != 10) || (ext && !slices.Contains([]int{4, 6, 8, 10}, req.DurationSec)) {
		return invalid("unsupported Omni video duration")
	}
	if !slices.Contains([]string{"360p", "720p", "1080p", "4k"}, strings.ToLower(strings.TrimSpace(req.Resolution))) {
		return invalid("unsupported Omni video resolution")
	}
	if !slices.Contains([]string{"16:9", "9:16"}, strings.TrimSpace(req.AspectRatio)) {
		return invalid("unsupported Omni video aspect ratio")
	}
	if (ext && !slices.Contains([]int{0, 1, 3}, len(req.InputURLs))) || (!ext && len(req.InputURLs) > 10) {
		return invalid("unsupported Omni reference image count")
	}
	for _, value := range req.InputURLs {
		if strings.HasPrefix(value, "data:") {
			if _, err := validateGenerationImageInput(value, maxGeminiGenerationImageBytes); err != nil {
				return err
			}
			continue
		}
		u, err := url.Parse(value)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
			return invalid("Omni reference must be a public HTTPS image")
		}
		host := strings.ToLower(u.Hostname())
		ip := net.ParseIP(host)
		if host == "localhost" || strings.HasSuffix(host, ".localhost") || (ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsMulticast())) {
			return invalid("Omni reference must be a public HTTPS image")
		}
	}
	return nil
}

func omniVideoCostCredits(req domain.ProviderRequest) int64 {
	resolution := strings.ToLower(strings.TrimSpace(req.Resolution))
	var micros int64
	if req.ModelCode == ModelOmni11Flash {
		micros = map[string]int64{"360p": 296000, "720p": 880000, "1080p": 1320000, "4k": 2640000}[resolution] * 10
	} else {
		// EXT is billed per call, not per second. Values are provider credits.
		base, step := int64(2500000), int64(250000)
		if resolution == "360p" {
			base, step = 1500000, 125000
		}
		if resolution == "4k" {
			base = 7500000
		}
		micros = base + int64(req.DurationSec-4)*step
	}
	return (micros + 999999) / 1000000
}

func (p *Provider) submitOmniVideo(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	images := make([]string, 0, len(req.InputURLs))
	for _, value := range req.InputURLs {
		imageURL, err := p.prepareFirstFrameImage(ctx, value)
		if err != nil {
			return domain.ProviderTask{}, err
		}
		images = append(images, imageURL)
	}
	body := omniVideoRequest{Model: req.ModelCode, Prompt: strings.TrimSpace(req.Prompt), Resolution: strings.ToLower(strings.TrimSpace(req.Resolution)), AspectRatio: strings.TrimSpace(req.AspectRatio), ImageURLs: images}
	if req.ModelCode == ModelOmni11FlashExt {
		body.Duration = req.DurationSec
		body.NSFWCheck = true
		if len(images) == 1 {
			body.GenerationType = "frame"
		} else if len(images) == 3 {
			body.GenerationType = "reference"
		}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return domain.ProviderTask{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid Omni video request"}
	}
	return p.postUnversionedTask(ctx, req, "/videos/generations", raw)
}
