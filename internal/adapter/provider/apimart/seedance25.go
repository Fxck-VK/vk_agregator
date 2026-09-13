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

type seedance25GenerationRequest struct {
	Model         string   `json:"model"`
	Prompt        string   `json:"prompt"`
	Duration      int      `json:"duration"`
	Resolution    string   `json:"resolution"`
	Size          string   `json:"size"`
	ImageURLs     []string `json:"image_urls,omitempty"`
	GenerateAudio bool     `json:"generate_audio"`
	OutputFormat  string   `json:"output_format"`
	Watermark     bool     `json:"watermark"`
	NSFWCheck     bool     `json:"NSFWCheck"`
}

func validateSeedance25(req domain.ProviderRequest, requirePrompt bool) error {
	invalid := func(message string) error { return &Error{Class: domain.ProviderErrInvalidRequest, Message: message} }
	if req.Operation != domain.OperationVideoGenerate || req.Modality != domain.ModalityVideo {
		return invalid("unsupported Seedance operation")
	}
	if requirePrompt && strings.TrimSpace(req.Prompt) == "" {
		return invalid("prompt is required")
	}
	if !slices.Contains([]int{5, 10, 15, 30}, req.DurationSec) {
		return invalid("unsupported Seedance duration")
	}
	if !slices.Contains([]string{"480p", "720p", "1080p"}, strings.ToLower(strings.TrimSpace(req.Resolution))) {
		return invalid("unsupported Seedance resolution")
	}
	if !slices.Contains([]string{"16:9", "9:16", "4:3", "3:4", "1:1", "21:9"}, strings.TrimSpace(req.AspectRatio)) {
		return invalid("unsupported Seedance aspect ratio")
	}
	if len(req.InputURLs) > 30 {
		return invalid("too many Seedance reference images")
	}
	for _, value := range req.InputURLs {
		value = strings.TrimSpace(value)
		if value == "" {
			return invalid("empty reference image")
		}
		if strings.HasPrefix(value, "data:") {
			if _, err := validateGenerationImageInput(value, maxGeminiGenerationImageBytes); err != nil {
				return err
			}
			continue
		}
		u, err := url.Parse(value)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
			return invalid("reference image must be a public HTTPS URL")
		}
		host := strings.ToLower(u.Hostname())
		ip := net.ParseIP(host)
		if host == "localhost" || strings.HasSuffix(host, ".localhost") || (ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsMulticast())) {
			return invalid("reference image must be a public HTTPS URL")
		}
	}
	return nil
}

func seedance25CostCredits(req domain.ProviderRequest) int64 {
	// APIMart preauthorization estimate in millionths of provider credits.
	rate := map[string]int64{"480p": 960800, "720p": 2160000, "1080p": 3848800}[strings.ToLower(strings.TrimSpace(req.Resolution))]
	return (rate*int64(req.DurationSec) + 999999) / 1000000
}

func (p *Provider) submitSeedance25Once(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	if err := validateSeedance25(req, true); err != nil {
		return domain.ProviderTask{}, err
	}
	return p.submitUnversionedOnce(ctx, req, p.submitSeedance25)
}

func (p *Provider) submitSeedance25(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	images := make([]string, 0, len(req.InputURLs))
	for _, input := range req.InputURLs {
		imageURL, err := p.prepareFirstFrameImage(ctx, input)
		if err != nil {
			return domain.ProviderTask{}, err
		}
		images = append(images, imageURL)
	}
	body, err := json.Marshal(seedance25GenerationRequest{Model: ModelSeedance25, Prompt: strings.TrimSpace(req.Prompt), Duration: req.DurationSec,
		Resolution: strings.ToLower(strings.TrimSpace(req.Resolution)), Size: strings.TrimSpace(req.AspectRatio), ImageURLs: images, GenerateAudio: true, OutputFormat: "mp4", NSFWCheck: true})
	if err != nil {
		return domain.ProviderTask{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid Seedance request"}
	}
	return p.postUnversionedTask(ctx, req, "/videos/generations", body)
}
