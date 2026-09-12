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
	ModelKlingV3        = "kling-v3"
	ModelKling26Motion  = "kling-v2-6-motion-control"
	ModelVeo31Fast      = "veo3.1-fast"
	ModelVeo31Quality   = "veo3.1-quality"
	ModelVeo31Lite      = "veo3.1-lite"
	klingVeoMicrosScale = int64(1000000)
)

func isKlingVeoVideo(model string) bool {
	switch strings.TrimSpace(model) {
	case ModelKlingV3, ModelKling26Motion, ModelVeo31Fast, ModelVeo31Quality, ModelVeo31Lite:
		return true
	default:
		return false
	}
}

type klingV3VideoRequest struct {
	Model          string   `json:"model"`
	Prompt         string   `json:"prompt"`
	Mode           string   `json:"mode"`
	Duration       int      `json:"duration"`
	AspectRatio    string   `json:"aspect_ratio"`
	ImageURLs      []string `json:"image_urls,omitempty"`
	NegativePrompt string   `json:"negative_prompt,omitempty"`
	Audio          bool     `json:"audio"`
	Watermark      bool     `json:"watermark"`
	NSFWCheck      bool     `json:"nsfw_check"`
}

type kling26MotionVideoRequest struct {
	Model                string                `json:"model"`
	Prompt               string                `json:"prompt,omitempty"`
	ImageURL             string                `json:"image_url"`
	VideoURL             string                `json:"video_url"`
	Mode                 string                `json:"mode"`
	CharacterOrientation string                `json:"character_orientation"`
	KeepOriginalSound    string                `json:"keep_original_sound"`
	NSFWCheck            bool                  `json:"nsfw_check"`
	WatermarkInfo        klingVeoWatermarkInfo `json:"watermark_info"`
}

type klingVeoWatermarkInfo struct {
	Enabled bool `json:"enabled"`
}

type veo31VideoRequest struct {
	Model            string   `json:"model"`
	Prompt           string   `json:"prompt"`
	Duration         int      `json:"duration"`
	AspectRatio      string   `json:"aspect_ratio"`
	Resolution       string   `json:"resolution"`
	ImageURLs        []string `json:"image_urls,omitempty"`
	GenerationType   string   `json:"generation_type,omitempty"`
	OfficialFallback *bool    `json:"official_fallback,omitempty"`
	NSFWCheck        bool     `json:"nsfw_check"`
	EnableGIF        bool     `json:"enable_gif"`
}

func validateKlingVeoVideo(req domain.ProviderRequest, requirePrompt bool) error {
	invalid := func(message string) error { return &Error{Class: domain.ProviderErrInvalidRequest, Message: message} }
	if req.Operation != domain.OperationVideoGenerate || req.Modality != domain.ModalityVideo || !isKlingVeoVideo(req.ModelCode) {
		return invalid("unsupported Kling/Veo video operation")
	}
	if req.OutputCount < 0 || req.OutputCount > 1 {
		return invalid("Kling/Veo video supports one output")
	}
	switch strings.TrimSpace(req.ModelCode) {
	case ModelKlingV3:
		if requirePrompt && strings.TrimSpace(req.Prompt) == "" {
			return invalid("Kling V3 prompt is required")
		}
		if !slices.Contains([]string{"720p", "1080p", "4k"}, strings.ToLower(strings.TrimSpace(req.Resolution))) {
			return invalid("unsupported Kling V3 resolution")
		}
		if req.DurationSec < 3 || req.DurationSec > 15 {
			return invalid("unsupported Kling V3 duration")
		}
		if !slices.Contains([]string{"16:9", "9:16", "1:1"}, strings.TrimSpace(req.AspectRatio)) {
			return invalid("unsupported Kling V3 aspect ratio")
		}
		if len(req.InputURLs) > 2 {
			return invalid("too many Kling V3 reference images")
		}
		for _, input := range req.InputURLs {
			if err := validateKlingVeoImageReference(input); err != nil {
				return err
			}
		}
	case ModelKling26Motion:
		if strings.TrimSpace(req.NegativePrompt) != "" {
			return invalid("Kling motion negative prompt is unsupported")
		}
		if !slices.Contains([]string{"std", "pro"}, strings.ToLower(strings.TrimSpace(req.Resolution))) {
			return invalid("unsupported Kling motion mode")
		}
		if len(req.InputURLs) != 1 {
			return invalid("Kling motion requires one reference image")
		}
		if err := validateKlingVeoImageReference(req.InputURLs[0]); err != nil {
			return err
		}
		if err := validateKlingVeoPublicHTTPSURL(req.ReferenceVideoURL, "Kling motion reference video must be a public HTTPS URL"); err != nil {
			return err
		}
		switch strings.ToLower(strings.TrimSpace(req.CharacterOrientation)) {
		case "image":
			if req.DurationSec < 3 || req.DurationSec > 10 {
				return invalid("unsupported Kling motion image duration")
			}
		case "video":
			if req.DurationSec < 3 || req.DurationSec > 30 {
				return invalid("unsupported Kling motion video duration")
			}
		default:
			return invalid("unsupported Kling motion character orientation")
		}
	case ModelVeo31Fast, ModelVeo31Quality, ModelVeo31Lite:
		if requirePrompt && strings.TrimSpace(req.Prompt) == "" {
			return invalid("Veo prompt is required")
		}
		if strings.TrimSpace(req.NegativePrompt) != "" {
			return invalid("Veo negative prompt is unsupported")
		}
		if req.DurationSec != 8 {
			return invalid("unsupported Veo duration")
		}
		if !slices.Contains([]string{"16:9", "9:16"}, strings.TrimSpace(req.AspectRatio)) {
			return invalid("unsupported Veo aspect ratio")
		}
		if !slices.Contains([]string{"720p", "1080p", "4k"}, strings.ToLower(strings.TrimSpace(req.Resolution))) {
			return invalid("unsupported Veo resolution")
		}
		maxImages := 3
		if strings.TrimSpace(req.ModelCode) == ModelVeo31Quality {
			maxImages = 2
		}
		if strings.TrimSpace(req.ModelCode) == ModelVeo31Lite {
			maxImages = 0
		}
		if len(req.InputURLs) > maxImages {
			return invalid("unsupported Veo reference image count")
		}
		for _, input := range req.InputURLs {
			if err := validateKlingVeoImageReference(input); err != nil {
				return err
			}
		}
	}
	return nil
}

func klingVeoVideoCostCredits(req domain.ProviderRequest) int64 {
	model := strings.TrimSpace(req.ModelCode)
	resolution := strings.ToLower(strings.TrimSpace(req.Resolution))
	switch model {
	case ModelKlingV3:
		rate := map[string]int64{"720p": 672000, "1080p": 896000, "4k": 4285600}[resolution]
		if req.VideoAudio {
			rate = map[string]int64{"720p": 1008000, "1080p": 1344000, "4k": 4285600}[resolution]
		}
		return klingVeoCeilMicros(rate * int64(req.DurationSec))
	case ModelKling26Motion:
		rate := map[string]int64{"std": 571200, "pro": 914400}[resolution]
		return klingVeoCeilMicros(rate * int64(req.DurationSec))
	case ModelVeo31Fast:
		if resolution == "4k" {
			return 7
		}
		return 2
	case ModelVeo31Quality:
		if resolution == "4k" {
			return 15
		}
		return 10
	case ModelVeo31Lite:
		if resolution == "4k" {
			return 6
		}
		return 1
	default:
		return 0
	}
}

func (p *Provider) submitKlingVeoVideo(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	if err := validateKlingVeoVideo(req, true); err != nil {
		return domain.ProviderTask{}, err
	}
	body, err := p.klingVeoVideoBody(ctx, req)
	if err != nil {
		return domain.ProviderTask{}, err
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return domain.ProviderTask{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid Kling/Veo video request"}
	}
	return p.postUnversionedTask(ctx, req, "/videos/generations", raw)
}

func (p *Provider) klingVeoVideoBody(ctx context.Context, req domain.ProviderRequest) (any, error) {
	switch strings.TrimSpace(req.ModelCode) {
	case ModelKlingV3:
		images, err := p.prepareKlingVeoImages(ctx, req.InputURLs)
		if err != nil {
			return nil, err
		}
		return klingV3VideoRequest{
			Model:          ModelKlingV3,
			Prompt:         strings.TrimSpace(req.Prompt),
			Mode:           klingV3Mode(req.Resolution),
			Duration:       req.DurationSec,
			AspectRatio:    strings.TrimSpace(req.AspectRatio),
			ImageURLs:      images,
			NegativePrompt: strings.TrimSpace(req.NegativePrompt),
			Audio:          req.VideoAudio,
			Watermark:      false,
			NSFWCheck:      true,
		}, nil
	case ModelKling26Motion:
		imageURL, err := p.prepareFirstFrameImage(ctx, firstInputURL(req.InputURLs))
		if err != nil {
			return nil, err
		}
		keepOriginalSound := "no"
		if req.KeepOriginalSound {
			keepOriginalSound = "yes"
		}
		return kling26MotionVideoRequest{
			Model:                ModelKling26Motion,
			Prompt:               strings.TrimSpace(req.Prompt),
			ImageURL:             imageURL,
			VideoURL:             strings.TrimSpace(req.ReferenceVideoURL),
			Mode:                 strings.ToLower(strings.TrimSpace(req.Resolution)),
			CharacterOrientation: strings.ToLower(strings.TrimSpace(req.CharacterOrientation)),
			KeepOriginalSound:    keepOriginalSound,
			NSFWCheck:            true,
			WatermarkInfo:        klingVeoWatermarkInfo{Enabled: false},
		}, nil
	case ModelVeo31Fast, ModelVeo31Quality, ModelVeo31Lite:
		images, err := p.prepareKlingVeoImages(ctx, req.InputURLs)
		if err != nil {
			return nil, err
		}
		body := veo31VideoRequest{
			Model:       strings.TrimSpace(req.ModelCode),
			Prompt:      strings.TrimSpace(req.Prompt),
			Duration:    8,
			AspectRatio: strings.TrimSpace(req.AspectRatio),
			Resolution:  strings.ToLower(strings.TrimSpace(req.Resolution)),
			ImageURLs:   images,
			NSFWCheck:   true,
			EnableGIF:   false,
		}
		if body.Model != ModelVeo31Lite {
			officialFallback := false
			body.OfficialFallback = &officialFallback
			body.GenerationType = veo31GenerationType(body.Model, len(images))
		}
		return body, nil
	default:
		return nil, &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported Kling/Veo video model"}
	}
}

func (p *Provider) prepareKlingVeoImages(ctx context.Context, inputs []string) ([]string, error) {
	images := make([]string, 0, len(inputs))
	for _, input := range inputs {
		imageURL, err := p.prepareFirstFrameImage(ctx, input)
		if err != nil {
			return nil, err
		}
		images = append(images, imageURL)
	}
	return images, nil
}

func klingV3Mode(resolution string) string {
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "720p":
		return "std"
	case "1080p":
		return "pro"
	case "4k":
		return "4k"
	default:
		return ""
	}
}

func veo31GenerationType(model string, imageCount int) string {
	if imageCount == 0 {
		return ""
	}
	if model == ModelVeo31Fast && imageCount == 3 {
		return "reference"
	}
	return "frame"
}

func validateKlingVeoImageReference(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "empty reference image"}
	}
	if strings.HasPrefix(strings.ToLower(value), "data:") {
		_, err := validateGenerationImageInput(value, maxGeminiGenerationImageBytes)
		return err
	}
	return validateKlingVeoPublicHTTPSURL(value, "reference image must be a public HTTPS URL")
}

func validateKlingVeoPublicHTTPSURL(value, message string) error {
	value = strings.TrimSpace(value)
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: message}
	}
	host := strings.ToLower(u.Hostname())
	ip := net.ParseIP(host)
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || (ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsMulticast())) {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: message}
	}
	return nil
}

func klingVeoCeilMicros(value int64) int64 {
	if value <= 0 {
		return 0
	}
	return (value + klingVeoMicrosScale - 1) / klingVeoMicrosScale
}
