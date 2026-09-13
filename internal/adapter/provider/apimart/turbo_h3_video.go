package apimart

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"net/http"
	"slices"
	"strings"

	_ "image/jpeg"
	_ "image/png"

	"vk-ai-aggregator/internal/domain"
)

const (
	ModelKling30Turbo = "kling-3.0-turbo"
	ModelMiniMaxH3    = "MiniMax-H3"

	turboH3MicrosScale = int64(1000000)
)

var (
	kling30TurboAspectRatios = []string{"16:9", "9:16", "1:1"}
	minimaxH3AspectRatios    = []string{"21:9", "16:9", "4:3", "1:1", "3:4", "9:16"}
)

func isTurboH3Video(model string) bool {
	switch strings.TrimSpace(model) {
	case ModelKling30Turbo, ModelMiniMaxH3:
		return true
	default:
		return false
	}
}

type turboH3VideoRequest struct {
	Model           string `json:"model"`
	Prompt          string `json:"prompt,omitempty"`
	Duration        int    `json:"duration"`
	Resolution      string `json:"resolution"`
	AspectRatio     string `json:"aspect_ratio,omitempty"`
	FirstFrameImage string `json:"first_frame_image,omitempty"`
}

func validateTurboH3Video(req domain.ProviderRequest, requirePrompt bool) error {
	invalid := func(message string) error { return &Error{Class: domain.ProviderErrInvalidRequest, Message: message} }
	model := strings.TrimSpace(req.ModelCode)
	if req.Operation != domain.OperationVideoGenerate || req.Modality != domain.ModalityVideo || !isTurboH3Video(model) {
		return invalid("unsupported APIMart Turbo/H3 video operation")
	}
	if req.OutputCount < 0 || req.OutputCount > 1 {
		return invalid("APIMart Turbo/H3 video supports one output")
	}
	if strings.TrimSpace(req.NegativePrompt) != "" {
		return invalid("APIMart Turbo/H3 negative prompt is unsupported")
	}
	if strings.TrimSpace(req.Size) != "" {
		return invalid("APIMart Turbo/H3 video size is unsupported")
	}
	if req.VideoAudio || strings.TrimSpace(req.ReferenceVideoURL) != "" || strings.TrimSpace(req.CharacterOrientation) != "" {
		return invalid("unsupported APIMart Turbo/H3 video media control")
	}
	if req.Draft {
		return invalid("APIMart Turbo/H3 draft mode is unsupported")
	}
	if len(cleanInputURLs(req.InputURLs)) > 1 {
		return invalid("APIMart Turbo/H3 supports one first frame image")
	}
	if err := validateTurboH3VideoParams(req); err != nil {
		return err
	}
	prompt := strings.TrimSpace(req.Prompt)
	hasFirstFrame := len(cleanInputURLs(req.InputURLs)) == 1
	switch model {
	case ModelKling30Turbo:
		if requirePrompt && prompt == "" && !hasFirstFrame {
			return invalid("Kling 3.0 Turbo prompt or first frame is required")
		}
		if len([]rune(prompt)) > 3072 {
			return invalid("Kling 3.0 Turbo prompt exceeds 3072 characters")
		}
		if req.DurationSec < 3 || req.DurationSec > 15 {
			return invalid("unsupported Kling 3.0 Turbo duration")
		}
		if !slices.Contains([]string{"720p", "1080p"}, strings.ToLower(strings.TrimSpace(req.Resolution))) {
			return invalid("unsupported Kling 3.0 Turbo resolution")
		}
		if !hasFirstFrame && !slices.Contains(kling30TurboAspectRatios, strings.TrimSpace(req.AspectRatio)) {
			return invalid("unsupported Kling 3.0 Turbo aspect ratio")
		}
		if hasFirstFrame && strings.TrimSpace(req.AspectRatio) != "" && !slices.Contains(kling30TurboAspectRatios, strings.TrimSpace(req.AspectRatio)) {
			return invalid("unsupported Kling 3.0 Turbo aspect ratio")
		}
	case ModelMiniMaxH3:
		if requirePrompt && prompt == "" {
			return invalid("MiniMax-H3 prompt is required")
		}
		if len([]rune(prompt)) > 7000 {
			return invalid("MiniMax-H3 prompt exceeds 7000 characters")
		}
		if req.DurationSec < 4 || req.DurationSec > 15 {
			return invalid("unsupported MiniMax-H3 duration")
		}
		if !slices.Contains([]string{"768P", "2K"}, normalizeTurboH3Resolution(model, req.Resolution)) {
			return invalid("unsupported MiniMax-H3 resolution")
		}
		if !hasFirstFrame && !slices.Contains(minimaxH3AspectRatios, strings.TrimSpace(req.AspectRatio)) {
			return invalid("unsupported MiniMax-H3 aspect ratio")
		}
		if hasFirstFrame && strings.TrimSpace(req.AspectRatio) != "" && !slices.Contains(minimaxH3AspectRatios, strings.TrimSpace(req.AspectRatio)) {
			return invalid("unsupported MiniMax-H3 aspect ratio")
		}
	}
	for _, input := range cleanInputURLs(req.InputURLs) {
		if err := validateTurboH3FirstFrame(model, input); err != nil {
			return err
		}
	}
	return nil
}

func validateTurboH3VideoParams(req domain.ProviderRequest) error {
	invalid := func(message string) error { return &Error{Class: domain.ProviderErrInvalidRequest, Message: message} }
	if len(req.Params) == 0 {
		return nil
	}
	var params map[string]json.RawMessage
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return invalid("invalid APIMart Turbo/H3 video params json")
	}
	for key := range params {
		if !allowedTurboH3VideoParam(key) {
			return invalid("unsupported native APIMart video parameter")
		}
	}
	if err := matchTurboH3IntParam(params, "duration_sec", req.DurationSec); err != nil {
		return err
	}
	if err := matchTurboH3ResolutionParam(params, strings.TrimSpace(req.ModelCode), "resolution", req.Resolution); err != nil {
		return err
	}
	if err := matchTurboH3StringParam(params, "aspect_ratio", strings.TrimSpace(req.AspectRatio)); err != nil {
		return err
	}
	if raw, ok := params["draft"]; ok {
		var draft bool
		if err := json.Unmarshal(raw, &draft); err != nil || draft {
			return invalid("APIMart Turbo/H3 draft mode is unsupported")
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
		return invalid("resolved route snapshot does not match APIMart Turbo/H3 request")
	}
	if snapshot.DurationSec != 0 && snapshot.DurationSec != req.DurationSec {
		return invalid("resolved route duration does not match APIMart Turbo/H3 request")
	}
	if strings.TrimSpace(snapshot.Resolution) != "" && normalizeTurboH3Resolution(req.ModelCode, snapshot.Resolution) != normalizeTurboH3Resolution(req.ModelCode, req.Resolution) {
		return invalid("resolved route resolution does not match APIMart Turbo/H3 request")
	}
	if strings.TrimSpace(snapshot.AspectRatio) != "" && strings.TrimSpace(snapshot.AspectRatio) != strings.TrimSpace(req.AspectRatio) {
		return invalid("resolved route aspect ratio does not match APIMart Turbo/H3 request")
	}
	if snapshot.VideoAudio || strings.TrimSpace(snapshot.ReferenceVideoArtifactID) != "" || strings.TrimSpace(snapshot.CharacterOrientation) != "" {
		return invalid("resolved route uses unsupported APIMart Turbo/H3 media control")
	}
	return nil
}

func allowedTurboH3VideoParam(key string) bool {
	switch key {
	case "duration_sec", "resolution", "aspect_ratio", "draft", "resolved_video_route":
		return true
	default:
		return false
	}
}

func matchTurboH3IntParam(params map[string]json.RawMessage, key string, want int) error {
	raw, ok := params[key]
	if !ok {
		return nil
	}
	var got int
	if err := json.Unmarshal(raw, &got); err != nil || got != want {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "inconsistent APIMart Turbo/H3 video parameter"}
	}
	return nil
}

func matchTurboH3StringParam(params map[string]json.RawMessage, key, want string) error {
	raw, ok := params[key]
	if !ok {
		return nil
	}
	var got string
	if err := json.Unmarshal(raw, &got); err != nil || strings.TrimSpace(got) != strings.TrimSpace(want) {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "inconsistent APIMart Turbo/H3 video parameter"}
	}
	return nil
}

func matchTurboH3ResolutionParam(params map[string]json.RawMessage, model, key, want string) error {
	raw, ok := params[key]
	if !ok {
		return nil
	}
	var got string
	if err := json.Unmarshal(raw, &got); err != nil || normalizeTurboH3Resolution(model, got) != normalizeTurboH3Resolution(model, want) {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "inconsistent APIMart Turbo/H3 video parameter"}
	}
	return nil
}

func turboH3VideoCostCredits(req domain.ProviderRequest) int64 {
	var rate int64
	switch strings.TrimSpace(req.ModelCode) {
	case ModelKling30Turbo:
		switch strings.ToLower(strings.TrimSpace(req.Resolution)) {
		case "720p":
			rate = 1144000
		case "1080p":
			rate = 1432000
		}
	case ModelMiniMaxH3:
		switch normalizeTurboH3Resolution(req.ModelCode, req.Resolution) {
		case "768P":
			rate = 571200
		case "2K":
			rate = 914400
		}
	}
	return turboH3CeilMicros(rate * int64(req.DurationSec))
}

func turboH3CeilMicros(value int64) int64 {
	if value <= 0 {
		return 0
	}
	return (value + turboH3MicrosScale - 1) / turboH3MicrosScale
}

func (p *Provider) submitTurboH3Video(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	body, err := p.turboH3VideoBody(ctx, req)
	if err != nil {
		return domain.ProviderTask{}, err
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return domain.ProviderTask{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid APIMart Turbo/H3 video request"}
	}
	return p.postUnversionedTask(ctx, req, "/videos/generations", raw)
}

func (p *Provider) turboH3VideoBody(ctx context.Context, req domain.ProviderRequest) (turboH3VideoRequest, error) {
	firstFrame, err := p.prepareTurboH3FirstFrame(ctx, strings.TrimSpace(req.ModelCode), firstInputURL(req.InputURLs))
	if err != nil {
		return turboH3VideoRequest{}, err
	}
	body := turboH3VideoRequest{
		Model:           strings.TrimSpace(req.ModelCode),
		Prompt:          strings.TrimSpace(req.Prompt),
		Duration:        req.DurationSec,
		Resolution:      normalizeTurboH3Resolution(req.ModelCode, req.Resolution),
		FirstFrameImage: firstFrame,
	}
	if firstFrame == "" {
		body.AspectRatio = strings.TrimSpace(req.AspectRatio)
	}
	return body, nil
}

func normalizeTurboH3Resolution(model, value string) string {
	value = strings.TrimSpace(value)
	switch strings.TrimSpace(model) {
	case ModelKling30Turbo:
		return strings.ToLower(value)
	case ModelMiniMaxH3:
		return strings.ToUpper(value)
	default:
		return value
	}
}

func validateTurboH3FirstFrame(model, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.HasPrefix(strings.ToLower(value), "data:") {
		_, err := decodeTurboH3FirstFrameDataURL(model, value)
		return err
	}
	return validateKlingVeoPublicHTTPSURL(value, "APIMart Turbo/H3 first frame must be a public HTTPS image URL")
}

func (p *Provider) prepareTurboH3FirstFrame(ctx context.Context, model, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(strings.ToLower(value), "data:") {
		if err := validateKlingVeoPublicHTTPSURL(value, "APIMart Turbo/H3 first frame must be a public HTTPS image URL"); err != nil {
			return "", err
		}
		return value, nil
	}
	image, err := decodeTurboH3FirstFrameDataURL(model, value)
	if err != nil {
		return "", err
	}
	uploaded, err := p.uploadImage(ctx, image)
	if err != nil {
		return "", err
	}
	if err := validateKlingVeoPublicHTTPSURL(uploaded, "APIMart upload returned invalid first frame URL"); err != nil {
		return "", err
	}
	return uploaded, nil
}

func decodeTurboH3FirstFrameDataURL(model, value string) (uploadImage, error) {
	label := "APIMart Turbo/H3 first frame image"
	comma := strings.IndexByte(value, ',')
	if comma <= len("data:") {
		return uploadImage{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid " + label + " data url"}
	}
	meta := strings.ToLower(strings.TrimSpace(value[len("data:"):comma]))
	payload := strings.TrimSpace(value[comma+1:])
	parts := strings.Split(meta, ";")
	contentType := strings.TrimSpace(parts[0])
	if !isAllowedTurboH3FirstFrameType(model, contentType) || !dataURLIsBase64(parts[1:]) {
		return uploadImage{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported " + label + " data url"}
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return uploadImage{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid " + label + " data"}
	}
	if len(data) == 0 {
		return uploadImage{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "empty " + label}
	}
	if len(data) > maxUploadImageBytes {
		return uploadImage{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: label + " exceeds APIMart upload limit"}
	}
	if detected := strings.ToLower(http.DetectContentType(data)); detected != contentType {
		return uploadImage{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: label + " content type mismatch"}
	}
	if contentType != "image/webp" {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return uploadImage{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid " + label + " dimensions"}
		}
		if err := validateTurboH3FirstFrameDimensions(model, cfg); err != nil {
			return uploadImage{}, err
		}
	}
	return uploadImage{ContentType: contentType, Data: data, Filename: "first-frame" + uploadImageExtension(contentType)}, nil
}

func isAllowedTurboH3FirstFrameType(model, contentType string) bool {
	switch strings.TrimSpace(model) {
	case ModelKling30Turbo:
		return contentType == "image/jpeg" || contentType == "image/png"
	case ModelMiniMaxH3:
		return contentType == "image/jpeg" || contentType == "image/png" || contentType == "image/webp"
	default:
		return false
	}
}

func validateTurboH3FirstFrameDimensions(model string, cfg image.Config) error {
	minSide, maxSide := 300, 0
	if strings.TrimSpace(model) == ModelMiniMaxH3 {
		minSide, maxSide = 256, 5760
	}
	if cfg.Width < minSide || cfg.Height < minSide {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "APIMart Turbo/H3 first frame dimensions are too small"}
	}
	if maxSide > 0 && (cfg.Width > maxSide || cfg.Height > maxSide) {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "APIMart Turbo/H3 first frame dimensions exceed APIMart limit"}
	}
	ratio := float64(cfg.Width) / float64(cfg.Height)
	if ratio < 0.4 || ratio > 2.5 {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "APIMart Turbo/H3 first frame aspect ratio is unsupported"}
	}
	return nil
}
