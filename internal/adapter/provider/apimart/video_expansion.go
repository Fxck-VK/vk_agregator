package apimart

import (
	"bytes"
	"image"
	"slices"
	"strings"

	"vk-ai-aggregator/internal/domain"
)

func validateVideoExpansionRequest(req domain.ProviderRequest) error {
	if req.ModelCode == ModelViduQ3 || req.ModelCode == ModelViduQ3Mix {
		req.InputURLs = slices.Clone(req.InputURLs)
		for i, value := range req.InputURLs {
			if !strings.HasPrefix(value, "data:") {
				continue
			}
			decoded, err := decodeImageDataURL(value)
			if err != nil {
				return err
			}
			if decoded.ContentType != "image/png" && decoded.ContentType != "image/jpeg" {
				return nextVisualInvalid("Vidu application accepts JPEG/PNG references")
			}
			cfg, _, err := image.DecodeConfig(bytes.NewReader(decoded.Data))
			if err != nil || cfg.Width < 128 || cfg.Height < 128 || cfg.Width > 4*cfg.Height || cfg.Height > 4*cfg.Width {
				return nextVisualInvalid("Vidu reference requires >=128px edges and aspect 1:4..4:1")
			}
			// Placeholder is validation-only. The original sanitized bytes are
			// uploaded through the existing APIMart image transport at submit.
			req.InputURLs[i] = "https://reference.invalid/validated-image.png"
		}
	}
	_, err := buildVideoExpansionBody(req)
	return err
}

const (
	ModelFlux3Video        = "flux-3-video"
	ModelPixVerseV6        = "pixverse-v6"
	ModelViduQ3            = "viduq3"
	ModelViduQ3Mix         = "viduq3-mix"
	ModelViduQ3Turbo       = "viduq3-turbo"
	ModelKlingVideoO1      = "kling-video-o1"
	ModelMiniMaxH3Max      = "MiniMax-H3-Max"
	ModelWan30Prime        = "wan3.0-video-prime"
	ModelWan27             = "wan2.7"
	ModelGeminiOmniPreview = "gemini-omni-flash-preview"
)

type expansionVideoSpec struct {
	id                    string
	min, max, promptLimit int
	resolution            string
	resolutions, ratios   []string
	audio                 bool
}

// Wire contracts read 2026-09-30 from the nine user-selected generation pages.
// Unknown native prompt limits use an explicitly local 20000-character bound.
func videoExpansionSpec(model string) (expansionVideoSpec, bool) {
	s := expansionVideoSpec{min: 1, max: 16, promptLimit: 20000, resolution: "720p", resolutions: []string{"540p", "720p", "1080p"}, ratios: []string{"16:9", "9:16", "4:3", "3:4", "1:1"}}
	switch model {
	case ModelFlux3Video:
		s.id = "flux_3_video"
		s.min = 5
		s.max = 20
		s.resolution = "720p"
		s.resolutions = []string{"720p", "1080p"}
		s.ratios = append(s.ratios, "21:9", "2:1")
		s.audio = true
	case ModelPixVerseV6:
		s.id = "pixverse_v6"
		s.max = 15
		s.promptLimit = 5000
		s.resolution = "540p"
		s.resolutions = []string{"360p", "540p", "720p", "1080p"}
		s.ratios = append(s.ratios, "2:3", "3:2", "21:9")
		s.audio = true
	case ModelViduQ3:
		s.id = "vidu_q3"
		s.min = 3
		s.promptLimit = 5000
	case ModelViduQ3Mix:
		s.id = "vidu_q3_mix"
		s.promptLimit = 5000
		s.resolutions = []string{"720p", "1080p"}
	case ModelViduQ3Turbo:
		s.id = "vidu_q3_turbo"
		s.promptLimit = 2000
		s.audio = true
	case ModelKlingVideoO1:
		s.id = "kling_video_o1"
		s.min = 5
		s.max = 10
		s.promptLimit = 2500
		s.resolutions = []string{"720p", "1080p"}
		s.ratios = []string{"16:9", "9:16", "1:1"}
	case ModelMiniMaxH3Max:
		s.id = "minimax_h3_max"
		s.min = 5
		s.max = 15
		s.promptLimit = 7000
		s.resolution = "768p"
		s.resolutions = []string{"480p", "768p", "1080p"}
		s.ratios = append(s.ratios, "21:9")
	case ModelWan30Prime:
		s.id = "wan_3_0_prime"
		s.min = 2
		s.max = 30
		s.resolution = "1080p"
		s.resolutions = []string{"480p", "720p", "1080p"}
		s.audio = true
	case ModelWan27:
		s.id = "wan_2_7"
		s.min = 2
		s.max = 15
		s.promptLimit = 5000
		s.resolution = "1080p"
		s.resolutions = []string{"720p", "1080p"}
	case ModelGeminiOmniPreview:
		s.id = "gemini_omni_flash_preview"
		s.min = 3
		s.max = 10
		s.resolutions = []string{"720p"}
		s.ratios = []string{"16:9", "9:16"}
	default:
		return expansionVideoSpec{}, false
	}
	return s, true
}

func videoExpansionID(model string) string { s, _ := videoExpansionSpec(model); return s.id }

func videoExpansionOptions(req domain.ProviderRequest, s expansionVideoSpec) (int, string) {
	duration := nextVisualDuration(req.DurationSec, 5)
	if req.ModelCode == ModelGeminiOmniPreview && req.DurationSec == 0 {
		duration = 10
	}
	resolution := strings.ToLower(strings.TrimSpace(req.Resolution))
	if resolution == "" {
		resolution = s.resolution
	}
	if req.ModelCode == ModelFlux3Video {
		if resolution == "hd" {
			resolution = "720p"
		}
		if resolution == "fhd" {
			resolution = "1080p"
		}
	}
	return duration, resolution
}

func buildVideoExpansionBody(req domain.ProviderRequest) ([]byte, error) {
	s, ok := videoExpansionSpec(req.ModelCode)
	if !ok || req.Operation != domain.OperationVideoGenerate || req.Modality != domain.ModalityVideo || (req.Provider != "" && req.Provider != domain.ProviderAPIMart) {
		return nil, nextVisualInvalid("unsupported APIMart video route")
	}
	if req.Music != nil || req.Speech != nil || req.OutputCount < 0 || req.OutputCount > 1 || req.Draft || req.ReferenceVideoURL != "" || req.CharacterOrientation != "" || req.KeepOriginalSound {
		return nil, nextVisualInvalid("unsupported video operation or control")
	}
	if req.VideoAudio && !s.audio {
		return nil, nextVisualInvalid("audio switch is not supported by this video API")
	}
	duration, resolution := videoExpansionOptions(req, s)
	if duration < s.min || duration > s.max || (req.ModelCode == ModelKlingVideoO1 && duration != 5 && duration != 10) || (req.ModelCode == ModelGeminiOmniPreview && duration != 10) {
		return nil, nextVisualInvalid("unsupported video duration")
	}
	if !slices.Contains(s.resolutions, resolution) {
		return nil, nextVisualInvalid("unsupported video resolution")
	}
	ratio, err := nextVisualRatio(req, s.ratios, "16:9", "APIMart video")
	if err != nil {
		return nil, err
	}
	prompt := strings.TrimSpace(req.Prompt)
	if len([]rune(prompt)) > s.promptLimit {
		return nil, nextVisualInvalid("video prompt exceeds route limit")
	}
	frames, references, seed, err := videoExpansionImages(req)
	if err != nil {
		return nil, err
	}
	if prompt == "" {
		optional := req.ModelCode == ModelViduQ3Turbo || req.ModelCode == ModelWan30Prime || req.ModelCode == ModelWan27 || req.ModelCode == ModelGeminiOmniPreview
		if !optional || len(frames)+len(references) == 0 {
			return nil, nextVisualInvalid("video prompt is required")
		}
	}
	body := map[string]any{"model": req.ModelCode, "resolution": resolution}
	if prompt != "" {
		body["prompt"] = prompt
	}
	if req.ModelCode != ModelGeminiOmniPreview {
		body["duration"] = duration
	}
	if s.audio {
		body["audio"] = req.VideoAudio
	}
	if seed != nil {
		body["seed"] = *seed
	}
	if len(frames) > 0 {
		body["image_urls"] = frames
	}
	if len(references) > 0 {
		body["image_urls"] = references
	}
	aspectForSnapshot := ratio
	switch req.ModelCode {
	case ModelFlux3Video:
		body["resolution"] = map[string]string{"720p": "hd", "1080p": "fhd"}[resolution]
		body["aspect_ratio"] = ratio
	case ModelPixVerseV6:
		if len(frames) == 2 {
			if duration != 5 && duration != 8 {
				return nil, nextVisualInvalid("PixVerse transition requires 5 or 8 seconds")
			}
			delete(body, "image_urls")
			body["first_frame_image"] = frames[0]
			body["last_frame_image"] = frames[1]
		}
		if len(references) > 0 {
			delete(body, "image_urls")
			body["img_references"] = references
		}
		if len(frames) == 0 {
			body["size"] = ratio
		}
	case ModelViduQ3, ModelViduQ3Mix:
		if len(references) == 0 {
			return nil, nextVisualInvalid("Vidu Q3 Standard/Mix require 1..7 reference images")
		}
		body["aspect_ratio"] = ratio
	case ModelViduQ3Turbo:
		if len(frames) > 0 {
			if strings.TrimSpace(req.AspectRatio) != "" || strings.TrimSpace(req.Size) != "" {
				return nil, nextVisualInvalid("Vidu frame mode forbids aspect ratio")
			}
			aspectForSnapshot = ""
		} else {
			body["aspect_ratio"] = ratio
		}
	case ModelKlingVideoO1:
		delete(body, "resolution")
		body["mode"] = map[string]string{"720p": "std", "1080p": "pro"}[resolution]
		body["aspect_ratio"] = ratio
	case ModelMiniMaxH3Max:
		body["resolution"] = strings.ToUpper(resolution)
		if req.VideoMedia != nil {
			if req.VideoMedia.StartFrame != nil {
				body["first_frame_image"] = req.VideoMedia.StartFrame.URL
			}
			if req.VideoMedia.EndFrame != nil {
				body["last_frame_image"] = req.VideoMedia.EndFrame.URL
			}
		}
		if len(frames) > 0 {
			delete(body, "image_urls")
		} else {
			body["aspect_ratio"] = ratio
		}
	case ModelWan30Prime, ModelWan27:
		body["resolution"] = strings.ToUpper(resolution)
		if len(frames) == 0 {
			body["size"] = ratio
		}
		if req.ModelCode == ModelWan30Prime && len(references) > 0 {
			body["generation_type"] = "reference"
		}
	case ModelGeminiOmniPreview:
		body["aspect_ratio"] = ratio
	}
	if req.NegativePrompt != "" {
		limit := 0
		if req.ModelCode == ModelPixVerseV6 {
			limit = 2048
		}
		if req.ModelCode == ModelWan27 {
			limit = 500
		}
		if limit == 0 || len([]rune(req.NegativePrompt)) > limit {
			return nil, nextVisualInvalid("unsupported negative prompt")
		}
		body["negative_prompt"] = req.NegativePrompt
	}
	if err := validateNextVisualVideoParams(req, duration, resolution, aspectForSnapshot); err != nil {
		return nil, err
	}
	return nextVisualMarshal(body, "invalid APIMart video request")
}

// Worker-owned media only. No provider task IDs or arbitrary native JSON are
// accepted, so ownership-sensitive continuation/edit operations remain closed.
func videoExpansionImages(req domain.ProviderRequest) (frames, references []string, seed *int, err error) {
	invalid := func() ([]string, []string, *int, error) {
		return nil, nil, nil, nextVisualInvalid("unsupported video media combination")
	}
	media := req.VideoMedia
	if len(req.ReferenceArtifactIDs) > 0 && len(req.ReferenceArtifactIDs) != len(req.InputURLs) {
		return invalid()
	}
	if len(req.InputURLs) > 0 {
		if req.ModelCode != ModelViduQ3 && req.ModelCode != ModelViduQ3Mix {
			return invalid()
		}
		references = append(references, req.InputURLs...)
	}
	if media != nil {
		if len(media.ReferenceVideos) > 0 || media.Audio != nil || media.PromptOptimizer != nil {
			return invalid()
		}
		if media.StartFrame != nil {
			frames = append(frames, media.StartFrame.URL)
		}
		if len(media.KeyFrames) > 0 {
			if req.ModelCode != ModelFlux3Video || media.StartFrame == nil || media.EndFrame == nil || req.DurationSec == 0 {
				return invalid()
			}
			for _, frame := range media.KeyFrames {
				if frame.TimeStampSec != nil || frame.Tag != "" {
					return invalid()
				}
				frames = append(frames, frame.URL)
			}
		}
		if media.EndFrame != nil {
			if media.StartFrame == nil && req.ModelCode != ModelMiniMaxH3Max {
				return invalid()
			}
			frames = append(frames, media.EndFrame.URL)
		}
		if len(req.InputURLs) > 0 && (len(frames) > 0 || len(media.ReferenceImageGroups) > 0) {
			return invalid()
		}
		for _, group := range media.ReferenceImageGroups {
			if group.Type != "" && group.Type != domain.VideoReferenceImageTypeImage || group.Tag != "" || group.AudioURL != "" || group.AudioDurationSec != 0 || len(group.URLs) == 0 {
				return invalid()
			}
			references = append(references, group.URLs...)
		}
		switch media.Mode {
		case "":
		case domain.VideoMediaModeText:
			if len(frames)+len(references) > 0 {
				return invalid()
			}
		case domain.VideoMediaModeImage:
			if len(frames) == 0 || len(references) > 0 {
				return invalid()
			}
		case domain.VideoMediaModeReferenceImage:
			if len(references) == 0 || len(frames) > 0 {
				return invalid()
			}
		default:
			return invalid()
		}
		seed = media.Seed
	}
	if len(frames) > 0 && len(references) > 0 {
		return invalid()
	}
	maxFrames, maxReferences := 2, 0
	switch req.ModelCode {
	case ModelFlux3Video:
		maxFrames = 10
	case ModelPixVerseV6:
		maxReferences = 7
	case ModelViduQ3, ModelViduQ3Mix:
		maxFrames = 0
		maxReferences = 7
	case ModelMiniMaxH3Max:
		maxReferences = 9
	case ModelWan30Prime:
		maxReferences = 10
	case ModelGeminiOmniPreview:
		maxFrames = 0
		maxReferences = 16
	}
	if len(frames) > maxFrames || len(references) > maxReferences {
		return invalid()
	}
	for _, urls := range [][]string{frames, references} {
		for _, u := range urls {
			if strings.TrimSpace(u) != u || validateKlingVeoPublicHTTPSURL(u, "video input must be a public HTTPS URL") != nil {
				return invalid()
			}
		}
	}
	if seed != nil {
		switch req.ModelCode {
		case ModelViduQ3Turbo:
			if int64(*seed) < -1 || int64(*seed) > 4294967295 {
				return invalid()
			}
		case ModelPixVerseV6, ModelWan30Prime:
			if *seed < 0 || int64(*seed) > 2147483647 {
				return invalid()
			}
		case ModelWan27:
			if *seed < 0 {
				return invalid()
			}
		case ModelViduQ3, ModelViduQ3Mix: // No narrower seed range is documented.
		default:
			return invalid()
		}
	}
	return frames, references, seed, nil
}
