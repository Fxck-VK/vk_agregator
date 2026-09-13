package apimart

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strings"

	_ "image/jpeg"
	_ "image/png"

	"vk-ai-aggregator/internal/domain"
)

const (
	maxSeedream50LiteReferenceImages = 15
	maxSeedream50ProReferenceImages  = 10
	maxSeedream50LiteImageBytes      = 10 * 1024 * 1024
	maxSeedream50ProImageBytes       = 30 * 1024 * 1024
	maxSeedream50ReferencePixels     = 6000 * 6000

	apimartCreditsPerUSD                    = 10
	gptImage25TextInputBudgetTokens         = 4096 + 512
	gptImage25TextInputUSDPerMillionTokens  = 4
	gptImage25OutputUSDPerMillionTokens     = 24
	seedream50LiteProviderCreditsPerImage   = 0.28
	seedream50ProBaseProviderCredits1K15K   = 0.2925
	seedream50ProBaseProviderCredits2K      = 0.585
	seedream50ProExtraReferenceCreditsAfter = 0.0195
)

var (
	gptImage25AspectRatios     = []string{"1:1", "3:2", "2:3", "4:3", "3:4", "5:4", "4:5", "16:9", "9:16", "2:1", "1:2", "21:9", "9:21", "3:1", "1:3"}
	seedream50LiteAspectRatios = []string{"1:1", "4:3", "3:4", "16:9", "9:16", "3:2", "2:3", "21:9"}
	seedream50ProAspectRatios  = []string{"1:1", "4:3", "3:4", "16:9", "9:16", "3:2", "2:3", "2:1", "1:2", "21:9"}
	gptImage25Resolutions      = []string{"1K", "2K", "4K"}
	seedream50LiteResolutions  = []string{"2K", "3K", "4K"}
	seedream50ProResolutions   = []string{"1K", "1.5K", "2K"}
	gptImage25Qualities        = []string{"low", "medium", "high", "xhigh", "max"}
	gptImage25MaxOutputTokens  = map[string]map[string]int{
		"1K": {"low": 196, "medium": 439, "high": 1756, "xhigh": 3122, "max": 7024},
		"2K": {"low": 397, "medium": 892, "high": 3568, "xhigh": 6343, "max": 14272},
		"4K": {"low": 659, "medium": 1483, "high": 5930, "xhigh": 10542, "max": 23719},
	}
)

func isGPTImage25Model(model string) bool {
	model = strings.TrimSpace(model)
	return model == ModelGPTImage25Flare || model == ModelGPTImage25Sunburst
}

func isSeedream50ImageModel(model string) bool {
	model = strings.TrimSpace(model)
	return model == ModelSeedream50Lite || model == ModelSeedream50Pro
}

func validateGPTImage25(req domain.ProviderRequest, requirePrompt bool) error {
	invalid := func(message string) error { return &Error{Class: domain.ProviderErrInvalidRequest, Message: message} }
	prompt := strings.TrimSpace(req.Prompt)
	if requirePrompt && prompt == "" {
		return invalid("prompt is required")
	}
	if len([]rune(prompt)) > 20000 {
		return invalid("prompt exceeds 20000 characters")
	}
	if strings.TrimSpace(req.NegativePrompt) != "" {
		return invalid("GPT-Image-2.5 negative prompt is unsupported")
	}
	if count := normalizeOutputCount(req.OutputCount); count < 1 || count > 4 {
		return invalid("GPT-Image-2.5 supports 1 to 4 outputs")
	}
	if _, err := publicAspectRatio(req, gptImage25AspectRatios, "GPT-Image-2.5"); err != nil {
		return err
	}
	if !slices.Contains(gptImage25Resolutions, normalizeResolutionUpper(req.Resolution)) {
		return invalid("unsupported GPT-Image-2.5 resolution")
	}
	if len(cleanInputURLs(req.InputURLs)) > maxGPTImage2ReferenceImages {
		return invalid("too many GPT-Image-2.5 reference images")
	}
	for _, input := range cleanInputURLs(req.InputURLs) {
		if isDataURL(input) {
			if _, err := decodeReferenceImageDataURL(input, maxGPTImage2ImageBytes, isGPTImage25ReferenceType, "GPT-Image-2.5 reference image"); err != nil {
				return err
			}
			continue
		}
		if !isPublicHTTPSURL(input) {
			return invalid("GPT-Image-2.5 references require public HTTPS URLs")
		}
	}
	_, err := gptImage25Quality(req)
	return err
}

func validateSeedream50Image(req domain.ProviderRequest, requirePrompt bool) error {
	invalid := func(message string) error { return &Error{Class: domain.ProviderErrInvalidRequest, Message: message} }
	model := strings.TrimSpace(req.ModelCode)
	prompt := strings.TrimSpace(req.Prompt)
	if requirePrompt && prompt == "" {
		return invalid("prompt is required")
	}
	if len([]rune(prompt)) > 20000 {
		return invalid("prompt exceeds 20000 characters")
	}
	if strings.TrimSpace(req.NegativePrompt) != "" {
		return invalid("Seedream 5 negative prompt is unsupported")
	}
	allowedAspects := seedream50AllowedAspectRatios(model)
	if len(allowedAspects) == 0 {
		return &Error{Class: domain.ProviderErrUnsupportedCapab, Message: "unsupported Seedream 5 image model"}
	}
	if _, err := publicAspectRatio(req, allowedAspects, "Seedream 5"); err != nil {
		return err
	}
	resolution := normalizeSeedreamResolution(req.Resolution)
	switch model {
	case ModelSeedream50Lite:
		if !slices.Contains(seedream50LiteResolutions, resolution) {
			return invalid("unsupported Seedream 5 Lite resolution")
		}
		count := normalizeOutputCount(req.OutputCount)
		if count < 1 || count > 15 {
			return invalid("Seedream 5 Lite supports 1 to 15 outputs")
		}
		refs := cleanInputURLs(req.InputURLs)
		if len(refs) > maxSeedream50LiteReferenceImages || len(refs)+count > 15 {
			return invalid("Seedream 5 Lite references plus outputs must be at most 15")
		}
		for _, input := range refs {
			if err := validateSeedreamReference(input, maxSeedream50LiteImageBytes, 1.0/3.0, 3.0, isSeedream50LiteReferenceType, "Seedream 5 Lite reference image"); err != nil {
				return err
			}
		}
	case ModelSeedream50Pro:
		if !slices.Contains(seedream50ProResolutions, resolution) {
			return invalid("unsupported Seedream 5 Pro resolution")
		}
		if count := normalizeOutputCount(req.OutputCount); count != 1 {
			return invalid("Seedream 5 Pro supports one output")
		}
		refs := cleanInputURLs(req.InputURLs)
		if len(refs) > maxSeedream50ProReferenceImages {
			return invalid("too many Seedream 5 Pro reference images")
		}
		for _, input := range refs {
			if err := validateSeedreamReference(input, maxSeedream50ProImageBytes, 1.0/16.0, 16.0, isSeedream50ProReferenceType, "Seedream 5 Pro reference image"); err != nil {
				return err
			}
		}
	default:
		return &Error{Class: domain.ProviderErrUnsupportedCapab, Message: "unsupported Seedream 5 image model"}
	}
	return validateImageTrustedParams(req, seedreamQualityFromResolution(req.Resolution))
}

func (p *Provider) submitGPTImage25(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	imageURLs, err := p.prepareGPTImage25References(ctx, cleanInputURLs(req.InputURLs))
	if err != nil {
		return domain.ProviderTask{}, err
	}
	quality, err := gptImage25Quality(req)
	if err != nil {
		return domain.ProviderTask{}, err
	}
	body, err := json.Marshal(struct {
		Model        string   `json:"model"`
		Prompt       string   `json:"prompt"`
		Size         string   `json:"size"`
		Resolution   string   `json:"resolution"`
		Quality      string   `json:"quality"`
		N            int      `json:"n"`
		OutputFormat string   `json:"output_format"`
		Moderation   string   `json:"moderation"`
		ImageURLs    []string `json:"image_urls,omitempty"`
	}{
		Model:        strings.TrimSpace(req.ModelCode),
		Prompt:       strings.TrimSpace(req.Prompt),
		Size:         mustPublicAspectRatio(req, gptImage25AspectRatios),
		Resolution:   strings.ToLower(normalizeResolutionUpper(req.Resolution)),
		Quality:      quality,
		N:            normalizeOutputCount(req.OutputCount),
		OutputFormat: "png",
		Moderation:   "auto",
		ImageURLs:    imageURLs,
	})
	if err != nil {
		return domain.ProviderTask{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid GPT-Image-2.5 request"}
	}
	return p.postUnversionedTask(ctx, req, "/images/generations", body)
}

func (p *Provider) submitSeedream50Image(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	body, err := seedream50RequestBody(req)
	if err != nil {
		return domain.ProviderTask{}, err
	}
	return p.postUnversionedTask(ctx, req, "/images/generations", body)
}

func seedream50RequestBody(req domain.ProviderRequest) ([]byte, error) {
	type sequentialOptions struct {
		MaxImages int `json:"max_images"`
	}
	body := struct {
		Model                            string             `json:"model"`
		Prompt                           string             `json:"prompt"`
		Size                             string             `json:"size"`
		Resolution                       string             `json:"resolution"`
		N                                int                `json:"n"`
		ImageURLs                        []string           `json:"image_urls,omitempty"`
		OutputFormat                     string             `json:"output_format"`
		Watermark                        bool               `json:"watermark"`
		SequentialImageGeneration        string             `json:"sequential_image_generation,omitempty"`
		SequentialImageGenerationOptions *sequentialOptions `json:"sequential_image_generation_options,omitempty"`
		Background                       string             `json:"background,omitempty"`
	}{
		Model:        strings.TrimSpace(req.ModelCode),
		Prompt:       strings.TrimSpace(req.Prompt),
		Size:         mustPublicAspectRatio(req, seedream50AllowedAspectRatios(req.ModelCode)),
		Resolution:   normalizeSeedreamResolution(req.Resolution),
		N:            normalizeOutputCount(req.OutputCount),
		ImageURLs:    cleanInputURLs(req.InputURLs),
		OutputFormat: "png",
		Watermark:    false,
	}
	if body.Model == ModelSeedream50Lite {
		body.SequentialImageGeneration = "disabled"
		if body.N > 1 {
			body.SequentialImageGeneration = "auto"
			body.SequentialImageGenerationOptions = &sequentialOptions{MaxImages: body.N}
		}
	} else {
		body.Background = "opaque"
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid Seedream 5 request"}
	}
	return raw, nil
}

func (p *Provider) prepareGPTImage25References(ctx context.Context, values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if !isDataURL(value) {
			if !isPublicHTTPSURL(value) {
				return nil, &Error{Class: domain.ProviderErrInvalidRequest, Message: "GPT-Image-2.5 references require public HTTPS URLs"}
			}
			out = append(out, value)
			continue
		}
		image, err := decodeReferenceImageDataURL(value, maxGPTImage2ImageBytes, isGPTImage25ReferenceType, "GPT-Image-2.5 reference image")
		if err != nil {
			return nil, err
		}
		uploaded, err := p.uploadImage(ctx, image)
		if err != nil {
			return nil, err
		}
		if !isPublicHTTPSURL(uploaded) {
			return nil, &Error{Class: domain.ProviderErrInternal, Message: "apimart upload returned non-HTTPS image url"}
		}
		out = append(out, uploaded)
	}
	return out, nil
}

func gptImage25Quality(req domain.ProviderRequest) (string, error) {
	if len(req.Params) == 0 {
		return "medium", nil
	}
	params, err := parseImageParams(req.Params)
	if err != nil {
		return "", err
	}
	if err := rejectUntrustedImageParams(params, allowedGPTImage25Param); err != nil {
		return "", err
	}
	raw, ok := params["image_quality"]
	if !ok {
		return "", &Error{Class: domain.ProviderErrInvalidRequest, Message: "GPT-Image-2.5 image_quality is required when params are present"}
	}
	var composite string
	if err := json.Unmarshal(raw, &composite); err != nil {
		return "", &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid GPT-Image-2.5 image_quality"}
	}
	tier, quality, ok := strings.Cut(strings.TrimSpace(composite), "-")
	if !ok || strings.TrimSpace(tier) == "" || strings.TrimSpace(quality) == "" {
		return "", &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid GPT-Image-2.5 image_quality"}
	}
	tier = normalizeResolutionUpper(tier)
	quality = strings.ToLower(strings.TrimSpace(quality))
	if tier != normalizeResolutionUpper(req.Resolution) || !slices.Contains(gptImage25Qualities, quality) {
		return "", &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported GPT-Image-2.5 image_quality"}
	}
	return quality, validateTrustedImageDimensionParams(req, params, composite)
}

func validateImageTrustedParams(req domain.ProviderRequest, wantQuality string) error {
	if len(req.Params) == 0 {
		return nil
	}
	params, err := parseImageParams(req.Params)
	if err != nil {
		return err
	}
	if err := rejectUntrustedImageParams(params, allowedSeedream50Param); err != nil {
		return err
	}
	return validateTrustedImageDimensionParams(req, params, wantQuality)
}

func parseImageParams(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var params map[string]json.RawMessage
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid APIMart image params json"}
	}
	if params == nil {
		params = map[string]json.RawMessage{}
	}
	return params, nil
}

func rejectUntrustedImageParams(params map[string]json.RawMessage, allowed func(string) bool) error {
	for key := range params {
		if !allowed(key) {
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported native APIMart image parameter"}
		}
	}
	return nil
}

func allowedGPTImage25Param(key string) bool {
	switch key {
	case "provider", "model_code", "model_id", "model_name", "provider_model_id", "size", "resolution", "image_quality", "aspect_ratio", "output_count", "reference_artifact_ids":
		return true
	default:
		return false
	}
}

func allowedSeedream50Param(key string) bool {
	switch key {
	case "provider", "model_code", "model_id", "model_name", "provider_model_id", "size", "resolution", "image_quality", "aspect_ratio", "output_count", "reference_artifact_ids":
		return true
	default:
		return false
	}
}

func validateTrustedImageDimensionParams(req domain.ProviderRequest, params map[string]json.RawMessage, wantQuality string) error {
	if err := matchStringParam(params, "provider", string(domain.ProviderAPIMart)); err != nil {
		return err
	}
	if err := matchStringParam(params, "model_code", strings.TrimSpace(req.ModelCode)); err != nil {
		return err
	}
	if err := matchStringParam(params, "provider_model_id", strings.TrimSpace(req.ModelCode)); err != nil {
		return err
	}
	if err := matchStringParam(params, "size", strings.TrimSpace(req.Size)); err != nil {
		return err
	}
	if err := matchStringParam(params, "aspect_ratio", publicAspectRatioOrEmpty(req)); err != nil {
		return err
	}
	if err := matchStringParam(params, "resolution", strings.TrimSpace(req.Resolution)); err != nil {
		return err
	}
	if err := matchStringParam(params, "image_quality", wantQuality); err != nil {
		return err
	}
	if raw, ok := params["output_count"]; ok {
		var value int
		if err := json.Unmarshal(raw, &value); err != nil || value != normalizeOutputCount(req.OutputCount) {
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "inconsistent APIMart image output count"}
		}
	}
	return nil
}

func matchStringParam(params map[string]json.RawMessage, key, want string) error {
	raw, ok := params[key]
	if !ok {
		return nil
	}
	var got string
	if err := json.Unmarshal(raw, &got); err != nil || strings.TrimSpace(got) != strings.TrimSpace(want) {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "inconsistent APIMart image parameter"}
	}
	return nil
}

func gptImage25ProviderCostCredits(req domain.ProviderRequest) (int64, error) {
	if len(cleanInputURLs(req.InputURLs)) > 0 {
		return 0, &Error{Class: domain.ProviderErrInvalidRequest, Message: "GPT-Image-2.5 input reference estimate is unavailable"}
	}
	quality, err := gptImage25Quality(req)
	if err != nil {
		return 0, err
	}
	resolution := normalizeResolutionUpper(req.Resolution)
	qualityTokens, ok := gptImage25MaxOutputTokens[resolution]
	if !ok {
		return 0, &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported GPT-Image-2.5 resolution"}
	}
	outputTokens, ok := qualityTokens[quality]
	if !ok {
		return 0, &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported GPT-Image-2.5 quality"}
	}
	outputs := normalizeOutputCount(req.OutputCount)
	providerCredits := float64(outputTokens) * gptImage25OutputUSDPerMillionTokens / 1_000_000 * apimartCreditsPerUSD * float64(outputs)
	providerCredits += float64(gptImage25TextInputBudgetTokens) * gptImage25TextInputUSDPerMillionTokens / 1_000_000 * apimartCreditsPerUSD
	return int64(math.Ceil(providerCredits)), nil
}

func seedream50AllowedAspectRatios(model string) []string {
	switch strings.TrimSpace(model) {
	case ModelSeedream50Lite:
		return seedream50LiteAspectRatios
	case ModelSeedream50Pro:
		return seedream50ProAspectRatios
	default:
		return nil
	}
}

func seedream50LiteProviderCostCredits(req domain.ProviderRequest) int64 {
	return int64(math.Ceil(seedream50LiteProviderCreditsPerImage * float64(normalizeOutputCount(req.OutputCount))))
}

func seedream50ProProviderCostCredits(req domain.ProviderRequest) int64 {
	base := seedream50ProBaseProviderCredits1K15K
	if normalizeSeedreamResolution(req.Resolution) == "2K" {
		base = seedream50ProBaseProviderCredits2K
	}
	extraRefs := max(len(cleanInputURLs(req.InputURLs))-1, 0)
	return int64(math.Ceil(base + seedream50ProExtraReferenceCreditsAfter*float64(extraRefs)))
}

func publicAspectRatio(req domain.ProviderRequest, allowed []string, modelName string) (string, error) {
	size := strings.TrimSpace(req.Size)
	aspect := strings.TrimSpace(req.AspectRatio)
	if size == "" && aspect == "" {
		return "1:1", nil
	}
	if size != "" && !slices.Contains(allowed, size) {
		return "", &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported " + modelName + " image size"}
	}
	if aspect != "" && !slices.Contains(allowed, aspect) {
		return "", &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported " + modelName + " image aspect ratio"}
	}
	if size != "" && aspect != "" && size != aspect {
		return "", &Error{Class: domain.ProviderErrInvalidRequest, Message: "conflicting " + modelName + " image aspect ratio"}
	}
	if aspect != "" {
		return aspect, nil
	}
	return size, nil
}

func mustPublicAspectRatio(req domain.ProviderRequest, allowed []string) string {
	ratio, err := publicAspectRatio(req, allowed, "APIMart")
	if err != nil {
		return "1:1"
	}
	return ratio
}

func publicAspectRatioOrEmpty(req domain.ProviderRequest) string {
	if strings.TrimSpace(req.AspectRatio) != "" {
		return strings.TrimSpace(req.AspectRatio)
	}
	return strings.TrimSpace(req.Size)
}

func normalizeOutputCount(count int) int {
	if count == 0 {
		return 1
	}
	return count
}

func normalizeResolutionUpper(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func normalizeSeedreamResolution(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "1.5K" {
		return "1.5K"
	}
	return value
}

func seedreamQualityFromResolution(resolution string) string {
	return normalizeSeedreamResolution(resolution)
}

func validateSeedreamReference(value string, maxBytes int, minRatio, maxRatio float64, allowedType func(string) bool, label string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if isHTTPURL(value) {
		return nil
	}
	if !isDataURL(value) {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported " + label}
	}
	_, err := decodeReferenceImageDataURL(value, maxBytes, allowedType, label, imageBounds{MinRatio: minRatio, MaxRatio: maxRatio, MaxPixels: maxSeedream50ReferencePixels})
	return err
}

type imageBounds struct {
	MinRatio  float64
	MaxRatio  float64
	MaxPixels int
}

func decodeReferenceImageDataURL(value string, maxBytes int, allowedType func(string) bool, label string, bounds ...imageBounds) (uploadImage, error) {
	comma := strings.IndexByte(value, ',')
	if comma <= len("data:") {
		return uploadImage{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid " + label + " data url"}
	}
	meta := strings.ToLower(strings.TrimSpace(value[len("data:"):comma]))
	payload := strings.TrimSpace(value[comma+1:])
	parts := strings.Split(meta, ";")
	contentType := strings.TrimSpace(parts[0])
	if !allowedType(contentType) || !dataURLIsBase64(parts[1:]) {
		return uploadImage{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported " + label + " data url"}
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return uploadImage{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid " + label + " data"}
	}
	if len(data) == 0 {
		return uploadImage{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "empty " + label}
	}
	if maxBytes > 0 && len(data) > maxBytes {
		return uploadImage{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: label + " exceeds APIMart limit"}
	}
	detected := strings.ToLower(http.DetectContentType(data))
	if detected != contentType {
		return uploadImage{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: label + " content type mismatch"}
	}
	if len(bounds) > 0 {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return uploadImage{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid " + label + " dimensions"}
		}
		if err := validateImageBounds(cfg, bounds[0], label); err != nil {
			return uploadImage{}, err
		}
	}
	return uploadImage{ContentType: contentType, Data: data, Filename: "reference-image" + uploadImageExtension(contentType)}, nil
}

func validateImageBounds(cfg image.Config, bounds imageBounds, label string) error {
	if cfg.Width <= 14 || cfg.Height <= 14 {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: label + " dimensions are too small"}
	}
	pixels := cfg.Width * cfg.Height
	if bounds.MaxPixels > 0 && pixels > bounds.MaxPixels {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: label + " dimensions exceed APIMart limit"}
	}
	ratio := float64(cfg.Width) / float64(cfg.Height)
	if ratio < bounds.MinRatio || ratio > bounds.MaxRatio {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: label + " aspect ratio is unsupported"}
	}
	return nil
}

func isGPTImage25ReferenceType(contentType string) bool {
	switch contentType {
	case "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

func isSeedream50LiteReferenceType(contentType string) bool {
	switch contentType {
	case "image/jpeg", "image/png":
		return true
	default:
		return false
	}
}

func isSeedream50ProReferenceType(contentType string) bool {
	switch contentType {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

func isDataURL(value string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "data:")
}

func isPublicHTTPSURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "https" || strings.TrimSpace(parsed.Host) == "" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host != "localhost" && host != "127.0.0.1" && host != "::1"
}
