package apimart

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"vk-ai-aggregator/internal/domain"
)

const (
	ModelWan30Video = "wan3.0-video"
	ModelViduQ3Pro  = "viduq3-pro"
	ModelImagen40   = "imagen-4.0-apimart"
)

var (
	nextVisualVideoAspectRatios = []string{"16:9", "4:3", "1:1", "3:4", "9:16"}
	imagen40AspectRatios        = []string{"1:1", "4:3", "3:4", "16:9", "9:16"}
)

type nextVisualVideoRequest struct {
	Model       string   `json:"model"`
	Prompt      string   `json:"prompt,omitempty"`
	Resolution  string   `json:"resolution"`
	Size        string   `json:"size,omitempty"`
	AspectRatio string   `json:"aspect_ratio,omitempty"`
	Duration    int      `json:"duration"`
	Audio       bool     `json:"audio"`
	Seed        *int     `json:"seed,omitempty"`
	ImageURLs   []string `json:"image_urls,omitempty"`
}

type nextVisualImageRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Size   string `json:"size"`
	N      int    `json:"n"`
}

func isNextVisualModel(model string) bool {
	switch strings.TrimSpace(model) {
	case ModelWan30Video, ModelViduQ3Pro, ModelImagen40:
		return true
	default:
		return false
	}
}

func validateNextVisualRequest(req domain.ProviderRequest) error {
	switch strings.TrimSpace(req.ModelCode) {
	case ModelWan30Video:
		return validateNextVisualWan(req)
	case ModelViduQ3Pro:
		return validateNextVisualVidu(req)
	case ModelImagen40:
		return validateNextVisualImagen(req)
	default:
		return nextVisualInvalid("unsupported APIMart next visual model")
	}
}

func buildNextVisualBody(req domain.ProviderRequest) ([]byte, error) {
	if err := validateNextVisualRequest(req); err != nil {
		return nil, err
	}
	switch strings.TrimSpace(req.ModelCode) {
	case ModelWan30Video:
		frames, err := nextVisualFrameURLs(req)
		if err != nil {
			return nil, err
		}
		seed, err := nextVisualWanSeed(req)
		if err != nil {
			return nil, err
		}
		body := nextVisualVideoRequest{
			Model:      ModelWan30Video,
			Prompt:     strings.TrimSpace(req.Prompt),
			Resolution: nextVisualWanResolution(req.Resolution),
			Size:       nextVisualRatioValue(req, nextVisualVideoAspectRatios, "16:9"),
			Duration:   nextVisualDuration(req.DurationSec, 5),
			Audio:      req.VideoAudio,
			Seed:       seed,
			ImageURLs:  frames,
		}
		return nextVisualMarshal(body, "invalid Wan 3.0 video request")
	case ModelViduQ3Pro:
		frames, err := nextVisualFrameURLs(req)
		if err != nil {
			return nil, err
		}
		body := nextVisualVideoRequest{
			Model:      ModelViduQ3Pro,
			Prompt:     strings.TrimSpace(req.Prompt),
			Resolution: nextVisualViduResolution(req.Resolution),
			Duration:   nextVisualDuration(req.DurationSec, 5),
			Audio:      req.VideoAudio,
			ImageURLs:  frames,
		}
		if len(frames) == 0 {
			body.AspectRatio = nextVisualRatioValue(req, nextVisualVideoAspectRatios, "16:9")
		}
		return nextVisualMarshal(body, "invalid Vidu Q3 Pro request")
	case ModelImagen40:
		body := nextVisualImageRequest{
			Model:  ModelImagen40,
			Prompt: strings.TrimSpace(req.Prompt),
			Size:   nextVisualRatioValue(req, imagen40AspectRatios, "16:9"),
			N:      normalizeOutputCount(req.OutputCount),
		}
		return nextVisualMarshal(body, "invalid Imagen 4.0 request")
	default:
		return nil, nextVisualInvalid("unsupported APIMart next visual model")
	}
}

func (p *Provider) submitNextVisual(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	body, err := buildNextVisualBody(req)
	if err != nil {
		return domain.ProviderTask{}, err
	}
	path := "/videos/generations"
	if strings.TrimSpace(req.ModelCode) == ModelImagen40 {
		path = "/images/generations"
	}
	return p.postUnversionedTask(ctx, req, path, body)
}

func validateNextVisualWan(req domain.ProviderRequest) error {
	if err := validateNextVisualVideoCommon(req, ModelWan30Video); err != nil {
		return err
	}
	prompt := strings.TrimSpace(req.Prompt)
	if len([]rune(prompt)) > 20000 {
		return nextVisualInvalid("Wan 3.0 prompt exceeds 20000 characters")
	}
	if _, err := nextVisualFrameURLs(req); err != nil {
		return err
	}
	if prompt == "" && !nextVisualHasFrames(req) {
		return nextVisualInvalid("Wan 3.0 prompt or frame is required")
	}
	duration := nextVisualDuration(req.DurationSec, 5)
	if duration < 2 || duration > 30 {
		return nextVisualInvalid("unsupported Wan 3.0 duration")
	}
	if !slices.Contains([]string{"480P", "720P", "1080P"}, nextVisualWanResolution(req.Resolution)) {
		return nextVisualInvalid("unsupported Wan 3.0 resolution")
	}
	if _, err := nextVisualRatio(req, nextVisualVideoAspectRatios, "16:9", "Wan 3.0"); err != nil {
		return err
	}
	if _, err := nextVisualWanSeed(req); err != nil {
		return err
	}
	return validateNextVisualVideoParams(req, duration, nextVisualWanResolution(req.Resolution), nextVisualRatioValue(req, nextVisualVideoAspectRatios, "16:9"))
}

func validateNextVisualVidu(req domain.ProviderRequest) error {
	if err := validateNextVisualVideoCommon(req, ModelViduQ3Pro); err != nil {
		return err
	}
	if req.VideoMedia != nil && req.VideoMedia.Seed != nil {
		return nextVisualInvalid("Vidu Q3 Pro seed is unsupported")
	}
	prompt := strings.TrimSpace(req.Prompt)
	if len([]rune(prompt)) > 2000 {
		return nextVisualInvalid("Vidu Q3 Pro prompt exceeds 2000 characters")
	}
	frames, err := nextVisualFrameURLs(req)
	if err != nil {
		return err
	}
	if prompt == "" && len(frames) == 0 {
		return nextVisualInvalid("Vidu Q3 Pro prompt or frame is required")
	}
	duration := nextVisualDuration(req.DurationSec, 5)
	if duration < 1 || duration > 16 {
		return nextVisualInvalid("unsupported Vidu Q3 Pro duration")
	}
	if !slices.Contains([]string{"540p", "720p", "1080p"}, nextVisualViduResolution(req.Resolution)) {
		return nextVisualInvalid("unsupported Vidu Q3 Pro resolution")
	}
	aspect := ""
	if len(frames) > 0 {
		if strings.TrimSpace(req.AspectRatio) != "" || strings.TrimSpace(req.Size) != "" {
			return nextVisualInvalid("Vidu Q3 Pro frame mode does not support explicit aspect ratio")
		}
	} else {
		if _, err := nextVisualRatio(req, nextVisualVideoAspectRatios, "16:9", "Vidu Q3 Pro"); err != nil {
			return err
		}
		aspect = nextVisualRatioValue(req, nextVisualVideoAspectRatios, "16:9")
	}
	return validateNextVisualVideoParams(req, duration, nextVisualViduResolution(req.Resolution), aspect)
}

func validateNextVisualImagen(req domain.ProviderRequest) error {
	if req.Operation != domain.OperationImageGenerate || req.Modality != domain.ModalityImage {
		return nextVisualInvalid("unsupported Imagen 4.0 operation")
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return nextVisualInvalid("prompt is required")
	}
	if req.Music != nil || req.VideoMedia != nil {
		return nextVisualInvalid("Imagen 4.0 does not support native media controls")
	}
	if req.OutputCount < 0 || normalizeOutputCount(req.OutputCount) != 1 {
		return nextVisualInvalid("Imagen 4.0 supports one output")
	}
	if len(cleanInputURLs(req.InputURLs)) > 0 || len(req.ReferenceArtifactIDs) > 0 {
		return nextVisualInvalid("Imagen 4.0 supports text-to-image only")
	}
	if strings.TrimSpace(req.NegativePrompt) != "" {
		return nextVisualInvalid("Imagen 4.0 negative prompt is unsupported")
	}
	if strings.TrimSpace(req.Resolution) != "" {
		return nextVisualInvalid("Imagen 4.0 resolution is unsupported")
	}
	if req.DurationSec != 0 || req.VideoAudio || req.Draft || strings.TrimSpace(req.ReferenceVideoURL) != "" ||
		strings.TrimSpace(req.CharacterOrientation) != "" || req.KeepOriginalSound {
		return nextVisualInvalid("Imagen 4.0 received unsupported video controls")
	}
	if _, err := nextVisualRatio(req, imagen40AspectRatios, "16:9", "Imagen 4.0"); err != nil {
		return err
	}
	return validateNextVisualImageParams(req)
}

func validateNextVisualVideoCommon(req domain.ProviderRequest, model string) error {
	if req.Operation != domain.OperationVideoGenerate || req.Modality != domain.ModalityVideo {
		return nextVisualInvalid("unsupported APIMart next visual video operation")
	}
	if strings.TrimSpace(req.ModelCode) != model {
		return nextVisualInvalid("APIMart next visual model mismatch")
	}
	if req.Music != nil {
		return nextVisualInvalid("APIMart next visual video does not support music controls")
	}
	if req.OutputCount < 0 || req.OutputCount > 1 {
		return nextVisualInvalid("APIMart next visual video supports one output")
	}
	if len(cleanInputURLs(req.InputURLs)) > 0 || len(req.ReferenceArtifactIDs) > 0 {
		return nextVisualInvalid("APIMart next visual video uses VideoMedia frames, not generic inputs")
	}
	if strings.TrimSpace(req.NegativePrompt) != "" || req.Draft || strings.TrimSpace(req.ReferenceVideoURL) != "" ||
		strings.TrimSpace(req.CharacterOrientation) != "" || req.KeepOriginalSound {
		return nextVisualInvalid("unsupported APIMart next visual video control")
	}
	return nil
}

func nextVisualFrameURLs(req domain.ProviderRequest) ([]string, error) {
	media := req.VideoMedia
	if media == nil {
		return nil, nil
	}
	if media.Mode == domain.VideoMediaModeText && (media.StartFrame != nil || media.EndFrame != nil) {
		return nil, nextVisualInvalid("text video cannot contain frames")
	}
	switch media.Mode {
	case "", domain.VideoMediaModeText, domain.VideoMediaModeImage:
	default:
		return nil, nextVisualInvalid("unsupported APIMart next visual media mode")
	}
	if len(media.KeyFrames) > 0 || len(media.ReferenceImageGroups) > 0 || len(media.ReferenceVideos) > 0 ||
		media.Audio != nil || media.PromptOptimizer != nil {
		return nil, nextVisualInvalid("unsupported APIMart next visual media input")
	}
	if media.Mode == domain.VideoMediaModeImage && media.StartFrame == nil {
		return nil, nextVisualInvalid("APIMart next visual image mode requires a start frame")
	}
	if media.EndFrame != nil && media.StartFrame == nil {
		return nil, nextVisualInvalid("APIMart next visual last frame requires a start frame")
	}
	frames := make([]string, 0, 2)
	for _, frame := range []*domain.VideoFrame{media.StartFrame, media.EndFrame} {
		if frame == nil {
			continue
		}
		url := strings.TrimSpace(frame.URL)
		if err := validateKlingVeoPublicHTTPSURL(url, "APIMart next visual frame must be a public HTTPS image URL"); err != nil {
			return nil, err
		}
		frames = append(frames, url)
	}
	return frames, nil
}

func nextVisualHasFrames(req domain.ProviderRequest) bool {
	frames, err := nextVisualFrameURLs(req)
	return err == nil && len(frames) > 0
}

func nextVisualWanSeed(req domain.ProviderRequest) (*int, error) {
	if req.VideoMedia == nil || req.VideoMedia.Seed == nil {
		return nil, nil
	}
	seed := *req.VideoMedia.Seed
	if seed < 0 || seed > 2147483647 {
		return nil, nextVisualInvalid("unsupported Wan 3.0 seed")
	}
	return &seed, nil
}

func validateNextVisualVideoParams(req domain.ProviderRequest, duration int, resolution, aspect string) error {
	params, err := nextVisualParseParams(req.Params)
	if err != nil || len(params) == 0 {
		return err
	}
	for key := range params {
		if !allowedNextVisualVideoParam(key) {
			return nextVisualInvalid("unsupported native APIMart next visual video parameter")
		}
	}
	if err := nextVisualMatchStringParam(params, "provider", string(domain.ProviderAPIMart)); err != nil {
		return err
	}
	for _, key := range []string{"model_code", "model_id", "model_name", "provider_model_id"} {
		if err := nextVisualMatchStringParam(params, key, strings.TrimSpace(req.ModelCode)); err != nil {
			return err
		}
	}
	if err := nextVisualMatchIntParam(params, "duration_sec", duration); err != nil {
		return err
	}
	if err := nextVisualMatchStringParam(params, "resolution", resolution); err != nil {
		return err
	}
	if err := nextVisualMatchStringParam(params, "aspect_ratio", aspect); err != nil {
		return err
	}
	if raw, ok := params["video_audio"]; ok {
		var got bool
		if err := json.Unmarshal(raw, &got); err != nil || got != req.VideoAudio {
			return nextVisualInvalid("inconsistent APIMart next visual video audio parameter")
		}
	}
	snapshot, hasSnapshot, err := resolvedRouteSnapshot(req.Params)
	if err != nil {
		return err
	}
	if !hasSnapshot {
		return nil
	}
	if snapshot.Provider != domain.ProviderAPIMart || strings.TrimSpace(snapshot.ProviderModelID) != strings.TrimSpace(req.ModelCode) {
		return nextVisualInvalid("resolved route snapshot does not match APIMart next visual request")
	}
	if snapshot.DurationSec != 0 && snapshot.DurationSec != duration {
		return nextVisualInvalid("resolved route duration does not match APIMart next visual request")
	}
	if strings.TrimSpace(snapshot.Resolution) != "" && strings.TrimSpace(snapshot.Resolution) != resolution {
		return nextVisualInvalid("resolved route resolution does not match APIMart next visual request")
	}
	if strings.TrimSpace(snapshot.AspectRatio) != "" && strings.TrimSpace(snapshot.AspectRatio) != aspect {
		return nextVisualInvalid("resolved route aspect ratio does not match APIMart next visual request")
	}
	if snapshot.VideoAudio != req.VideoAudio || strings.TrimSpace(snapshot.ReferenceVideoArtifactID) != "" ||
		strings.TrimSpace(snapshot.CharacterOrientation) != "" || snapshot.KeepOriginalSound {
		return nextVisualInvalid("resolved route uses unsupported APIMart next visual media control")
	}
	return nil
}

func allowedNextVisualVideoParam(key string) bool {
	switch key {
	case "provider", "model_code", "model_id", "model_name", "provider_model_id",
		"duration_sec", "resolution", "aspect_ratio", "video_audio", "resolved_video_route":
		return true
	default:
		return false
	}
}

func validateNextVisualImageParams(req domain.ProviderRequest) error {
	params, err := nextVisualParseParams(req.Params)
	if err != nil || len(params) == 0 {
		return err
	}
	for key := range params {
		if !allowedNextVisualImageParam(key) {
			return nextVisualInvalid("unsupported native APIMart next visual image parameter")
		}
	}
	if err := nextVisualMatchStringParam(params, "provider", string(domain.ProviderAPIMart)); err != nil {
		return err
	}
	for _, key := range []string{"model_code", "model_id", "model_name", "provider_model_id"} {
		if err := nextVisualMatchStringParam(params, key, strings.TrimSpace(req.ModelCode)); err != nil {
			return err
		}
	}
	ratio := nextVisualRatioValue(req, imagen40AspectRatios, "16:9")
	if err := nextVisualMatchStringParam(params, "size", ratio); err != nil {
		return err
	}
	if err := nextVisualMatchStringParam(params, "aspect_ratio", ratio); err != nil {
		return err
	}
	return nextVisualMatchIntParam(params, "output_count", 1)
}

func allowedNextVisualImageParam(key string) bool {
	switch key {
	case "provider", "model_code", "model_id", "model_name", "provider_model_id", "size", "aspect_ratio", "output_count":
		return true
	default:
		return false
	}
}

func nextVisualParseParams(raw json.RawMessage) (map[string]json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var params map[string]json.RawMessage
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, nextVisualInvalid("invalid APIMart next visual params json")
	}
	if params == nil {
		params = map[string]json.RawMessage{}
	}
	return params, nil
}

func nextVisualMatchStringParam(params map[string]json.RawMessage, key, want string) error {
	raw, ok := params[key]
	if !ok {
		return nil
	}
	var got string
	if err := json.Unmarshal(raw, &got); err != nil || strings.TrimSpace(got) != strings.TrimSpace(want) {
		return nextVisualInvalid("inconsistent APIMart next visual parameter")
	}
	return nil
}

func nextVisualMatchIntParam(params map[string]json.RawMessage, key string, want int) error {
	raw, ok := params[key]
	if !ok {
		return nil
	}
	var got int
	if err := json.Unmarshal(raw, &got); err != nil || got != want {
		return nextVisualInvalid("inconsistent APIMart next visual parameter")
	}
	return nil
}

func nextVisualDuration(value, defaultValue int) int {
	if value == 0 {
		return defaultValue
	}
	return value
}

func nextVisualWanResolution(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return "720P"
	}
	return value
}

func nextVisualViduResolution(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "720p"
	}
	return value
}

func nextVisualRatio(req domain.ProviderRequest, allowed []string, defaultValue, label string) (string, error) {
	size := strings.TrimSpace(req.Size)
	aspect := strings.TrimSpace(req.AspectRatio)
	if size == "" && aspect == "" {
		return defaultValue, nil
	}
	if size != "" && !slices.Contains(allowed, size) {
		return "", nextVisualInvalid("unsupported " + label + " aspect ratio")
	}
	if aspect != "" && !slices.Contains(allowed, aspect) {
		return "", nextVisualInvalid("unsupported " + label + " aspect ratio")
	}
	if size != "" && aspect != "" && size != aspect {
		return "", nextVisualInvalid("conflicting " + label + " aspect ratio")
	}
	if aspect != "" {
		return aspect, nil
	}
	return size, nil
}

func nextVisualRatioValue(req domain.ProviderRequest, allowed []string, defaultValue string) string {
	ratio, err := nextVisualRatio(req, allowed, defaultValue, "APIMart next visual")
	if err != nil {
		return defaultValue
	}
	return ratio
}

func nextVisualMarshal(value any, message string) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, nextVisualInvalid(message)
	}
	return raw, nil
}

func nextVisualInvalid(message string) error {
	return &Error{Class: domain.ProviderErrInvalidRequest, Message: message}
}
