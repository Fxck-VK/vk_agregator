package apimart

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"vk-ai-aggregator/internal/domain"
)

const (
	ModelHappyHorse10   = "happyhorse-1.0"
	ModelHappyHorse11   = "happyhorse-1.1"
	ModelSkyReelsV4Fast = "skyreels-v4-fast"
	ModelSkyReelsV4Std  = "skyreels-v4-std"
	maxHappyHorsePrompt = 2500
	// Local conservative UTF-8 byte budget. APIMart documents 1280 tokens but
	// does not publish its tokenizer; words are not a valid token counter.
	maxSkyReelsPromptBytes = 1280
)

var (
	happyHorseResolutions = []string{"720P", "1080P"}
	happyHorseSizes       = []string{"16:9", "9:16", "1:1", "4:3", "3:4"}
	skyReelsResolutions   = []string{"480p", "720p", "1080p"}
	skyReelsAspectRatios  = []string{"16:9", "4:3", "1:1", "9:16", "3:4"}
)

func isHappyHorseSkyReelsModel(model string) bool {
	switch strings.TrimSpace(model) {
	case ModelHappyHorse10, ModelHappyHorse11, ModelSkyReelsV4Fast, ModelSkyReelsV4Std:
		return true
	default:
		return false
	}
}

type happyHorseVideoRequest struct {
	Model           string   `json:"model"`
	Prompt          string   `json:"prompt,omitempty"`
	Resolution      string   `json:"resolution"`
	Size            string   `json:"size,omitempty"`
	Duration        *int     `json:"duration,omitempty"`
	Seed            *int     `json:"seed,omitempty"`
	FirstFrameImage string   `json:"first_frame_image,omitempty"`
	ImageURLs       []string `json:"image_urls,omitempty"`
	VideoURL        string   `json:"video_url,omitempty"`
	AudioSetting    string   `json:"audio_setting,omitempty"`
}

type skyReelsVideoRequest struct {
	Model           string                 `json:"model"`
	Prompt          string                 `json:"prompt"`
	Duration        *int                   `json:"duration,omitempty"`
	Resolution      string                 `json:"resolution"`
	AspectRatio     string                 `json:"aspect_ratio,omitempty"`
	PromptOptimizer *bool                  `json:"prompt_optimizer,omitempty"`
	FirstFrameImage string                 `json:"first_frame_image,omitempty"`
	EndFrameImage   string                 `json:"end_frame_image,omitempty"`
	MidFrameImages  []skyReelsKeyFrameBody `json:"mid_frame_images,omitempty"`
	ReferenceImages []skyReelsRefImageBody `json:"ref_images,omitempty"`
	ReferenceVideos []skyReelsRefVideoBody `json:"ref_videos,omitempty"`
}

type skyReelsKeyFrameBody struct {
	Tag       string `json:"tag"`
	ImageURL  string `json:"image_url"`
	TimeStamp *int   `json:"time_stamp,omitempty"`
}

type skyReelsRefImageBody struct {
	Tag       string   `json:"tag"`
	Type      string   `json:"type"`
	ImageURLs []string `json:"image_urls"`
	AudioURL  string   `json:"audio_url,omitempty"`
}

type skyReelsRefVideoBody struct {
	Tag      string `json:"tag"`
	Type     string `json:"type"`
	VideoURL string `json:"video_url"`
}

func validateHappyHorseSkyReelsRequest(req domain.ProviderRequest) error {
	return validateHappyHorseSkyReelsRequestWithMedia(req, req.VideoMedia)
}

func validateHappyHorseSkyReelsRequestWithMedia(req domain.ProviderRequest, media *domain.VideoMediaRequest) error {
	invalid := func(message string) error { return &Error{Class: domain.ProviderErrInvalidRequest, Message: message} }
	model := strings.TrimSpace(req.ModelCode)
	if req.Operation != domain.OperationVideoGenerate || req.Modality != domain.ModalityVideo || !isHappyHorseSkyReelsModel(model) {
		return invalid("unsupported APIMart native video operation")
	}
	if req.OutputCount < 0 || req.OutputCount > 1 {
		return invalid("APIMart native video supports one output")
	}
	if strings.TrimSpace(req.NegativePrompt) != "" {
		return invalid("APIMart native video negative prompt is unsupported")
	}
	if req.VideoAudio || req.KeepOriginalSound || strings.TrimSpace(req.ReferenceVideoURL) != "" || strings.TrimSpace(req.CharacterOrientation) != "" {
		return invalid("APIMart native video requires typed VideoMedia controls")
	}
	if req.Draft {
		return invalid("APIMart native video draft mode is unsupported")
	}
	if len(cleanInputURLs(req.InputURLs)) > 0 {
		return invalid("APIMart native video input URLs must be carried by VideoMedia")
	}
	if err := validateHappyHorseSkyReelsParams(req); err != nil {
		return err
	}
	if isHappyHorseModel(model) {
		return validateHappyHorseRequest(req, media)
	}
	return validateSkyReelsRequest(req, media)
}

func validateHappyHorseSkyReelsParams(req domain.ProviderRequest) error {
	if len(req.Params) == 0 {
		return nil
	}
	var params map[string]json.RawMessage
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid APIMart native video params json"}
	}
	for key := range params {
		switch key {
		case "resolved_video_route", "duration_sec", "resolution", "aspect_ratio", "seed", "prompt_optimizer":
		default:
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported native APIMart video parameter"}
		}
	}
	if err := matchHappyHorseSkyReelsIntParam(params, "duration_sec", req.DurationSec); err != nil {
		return err
	}
	if err := matchHappyHorseSkyReelsStringParam(params, "resolution", req.Resolution); err != nil {
		return err
	}
	if err := matchHappyHorseSkyReelsStringParam(params, "aspect_ratio", req.AspectRatio); err != nil {
		return err
	}
	snapshot, hasSnapshot, err := resolvedRouteSnapshot(req.Params)
	if err != nil {
		return err
	}
	if hasSnapshot {
		if snapshot.Provider != domain.ProviderAPIMart || strings.TrimSpace(snapshot.ProviderModelID) != strings.TrimSpace(req.ModelCode) {
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "resolved route snapshot does not match APIMart native video request"}
		}
		if snapshot.DurationSec != 0 && snapshot.DurationSec != req.DurationSec {
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "resolved route duration does not match APIMart native video request"}
		}
		if strings.TrimSpace(snapshot.Resolution) != "" && !sameNativeVideoResolution(req.ModelCode, snapshot.Resolution, req.Resolution) {
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "resolved route resolution does not match APIMart native video request"}
		}
		if strings.TrimSpace(snapshot.AspectRatio) != "" && strings.TrimSpace(snapshot.AspectRatio) != strings.TrimSpace(req.AspectRatio) {
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "resolved route aspect ratio does not match APIMart native video request"}
		}
		if snapshot.VideoAudio || strings.TrimSpace(snapshot.ReferenceVideoArtifactID) != "" || strings.TrimSpace(snapshot.CharacterOrientation) != "" {
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "resolved route uses unsupported APIMart native video media control"}
		}
	}
	return nil
}

func matchHappyHorseSkyReelsIntParam(params map[string]json.RawMessage, key string, want int) error {
	raw, ok := params[key]
	if !ok {
		return nil
	}
	var got int
	if err := json.Unmarshal(raw, &got); err != nil || got != want {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "inconsistent APIMart native video parameter"}
	}
	return nil
}

func matchHappyHorseSkyReelsStringParam(params map[string]json.RawMessage, key, want string) error {
	raw, ok := params[key]
	if !ok {
		return nil
	}
	var got string
	if err := json.Unmarshal(raw, &got); err != nil || strings.TrimSpace(got) != strings.TrimSpace(want) {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "inconsistent APIMart native video parameter"}
	}
	return nil
}

func sameNativeVideoResolution(model, a, b string) bool {
	if isHappyHorseModel(model) {
		return normalizeHappyHorseResolution(a) == normalizeHappyHorseResolution(b)
	}
	return strings.ToLower(strings.TrimSpace(a)) == strings.ToLower(strings.TrimSpace(b))
}

func isHappyHorseModel(model string) bool {
	switch strings.TrimSpace(model) {
	case ModelHappyHorse10, ModelHappyHorse11:
		return true
	default:
		return false
	}
}

func isSkyReelsModel(model string) bool {
	switch strings.TrimSpace(model) {
	case ModelSkyReelsV4Fast, ModelSkyReelsV4Std:
		return true
	default:
		return false
	}
}

func validateHappyHorseRequest(req domain.ProviderRequest, media *domain.VideoMediaRequest) error {
	invalid := func(message string) error { return &Error{Class: domain.ProviderErrInvalidRequest, Message: message} }
	model := strings.TrimSpace(req.ModelCode)
	if len([]rune(strings.TrimSpace(req.Prompt))) > maxHappyHorsePrompt {
		return invalid("HappyHorse prompt exceeds 2500 characters")
	}
	if !slices.Contains(happyHorseResolutions, normalizeHappyHorseResolution(req.Resolution)) {
		return invalid("unsupported HappyHorse resolution")
	}
	if strings.TrimSpace(req.AspectRatio) != "" && !slices.Contains(happyHorseSizes, strings.TrimSpace(req.AspectRatio)) {
		return invalid("unsupported HappyHorse size")
	}
	state := classifyHappyHorseMedia(media)
	if model == ModelHappyHorse11 && state.hasVideo {
		return invalid("HappyHorse 1.1 video edit is unsupported")
	}
	if state.hasUnsupportedSkyMedia {
		return invalid("HappyHorse supports only first frame, reference images, source video and audio_setting")
	}
	if state.hasFirst && (state.refCount > 0 || state.hasVideo) {
		return invalid("HappyHorse first frame cannot be mixed with references or source video")
	}
	if state.hasVideo && state.videoCount != 1 {
		return invalid("HappyHorse edit requires one source video")
	}
	if !state.hasVideo && state.refCount > 9 {
		return invalid("HappyHorse reference image limit is 9")
	}
	if state.hasVideo && state.refCount > 5 {
		return invalid("HappyHorse edit reference image limit is 5")
	}
	if state.refCount > 0 && state.hasGrid {
		return invalid("HappyHorse grid references are unsupported")
	}
	if !state.hasVideo && state.hasAudio {
		return invalid("HappyHorse audio_setting is only valid for edit")
	}
	if state.hasAudio && !validHappyHorseAudioSetting(media.Audio.Setting) {
		return invalid("unsupported HappyHorse audio_setting")
	}
	if state.hasVideo && media != nil && media.Mode != "" && media.Mode != domain.VideoMediaModeEdit {
		return invalid("HappyHorse edit media mode mismatch")
	}
	if !state.hasVideo && (req.DurationSec < 3 || req.DurationSec > 15) {
		return invalid("unsupported HappyHorse duration")
	}
	if state.hasVideo && req.DurationSec != 0 && (req.DurationSec < 3 || req.DurationSec > 15) {
		return invalid("unsupported HappyHorse edit duration")
	}
	if strings.TrimSpace(req.Prompt) == "" && !state.hasFirst {
		return invalid("HappyHorse prompt is required")
	}
	return validateHappyHorseURLs(media)
}

type happyHorseMediaState struct {
	hasFirst               bool
	hasVideo               bool
	hasAudio               bool
	hasGrid                bool
	hasUnsupportedSkyMedia bool
	videoCount             int
	refCount               int
}

func classifyHappyHorseMedia(media *domain.VideoMediaRequest) happyHorseMediaState {
	var state happyHorseMediaState
	if media == nil {
		return state
	}
	state.hasFirst = media.StartFrame != nil && strings.TrimSpace(media.StartFrame.URL) != ""
	state.hasAudio = media.Audio != nil && media.Audio.Setting != ""
	if media.EndFrame != nil && strings.TrimSpace(media.EndFrame.URL) != "" || len(media.KeyFrames) > 0 {
		state.hasUnsupportedSkyMedia = true
	}
	for _, video := range media.ReferenceVideos {
		if strings.TrimSpace(video.URL) == "" {
			continue
		}
		state.videoCount++
		state.hasVideo = true
	}
	for _, group := range media.ReferenceImageGroups {
		if group.Type == domain.VideoReferenceImageTypeGrid {
			state.hasGrid = true
		}
		state.refCount += len(cleanInputURLs(group.URLs))
	}
	return state
}

func validHappyHorseAudioSetting(setting domain.VideoAudioSetting) bool {
	switch setting {
	case domain.VideoAudioSettingAuto, domain.VideoAudioSettingOrigin:
		return true
	default:
		return false
	}
}

func validateHappyHorseURLs(media *domain.VideoMediaRequest) error {
	if media == nil {
		return nil
	}
	if media.StartFrame != nil && strings.TrimSpace(media.StartFrame.URL) != "" {
		if err := validateNativeVideoImage(media.StartFrame.URL, "HappyHorse first frame must be a public HTTPS image URL"); err != nil {
			return err
		}
	}
	for _, group := range media.ReferenceImageGroups {
		if group.Type != "" && group.Type != domain.VideoReferenceImageTypeImage && group.Type != domain.VideoReferenceImageTypeGrid {
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported HappyHorse reference image type"}
		}
		for _, value := range cleanInputURLs(group.URLs) {
			if err := validateNativeVideoImage(value, "HappyHorse reference image must be a public HTTPS image URL"); err != nil {
				return err
			}
		}
		if strings.TrimSpace(group.AudioURL) != "" {
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "HappyHorse reference audio is unsupported"}
		}
	}
	for _, video := range media.ReferenceVideos {
		if video.Type != "" && video.Type != domain.VideoReferenceVideoTypeEdit {
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported HappyHorse reference video type"}
		}
		if err := validateKlingVeoPublicHTTPSURL(video.URL, "HappyHorse source video must be a public HTTPS URL"); err != nil {
			return err
		}
	}
	if media.Audio != nil && strings.TrimSpace(media.Audio.URL) != "" {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "HappyHorse audio URL is unsupported"}
	}
	return nil
}

func validateSkyReelsRequest(req domain.ProviderRequest, media *domain.VideoMediaRequest) error {
	invalid := func(message string) error { return &Error{Class: domain.ProviderErrInvalidRequest, Message: message} }
	if !isSkyReelsModel(req.ModelCode) {
		return invalid("unsupported SkyReels model")
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return invalid("SkyReels prompt is required")
	}
	if len(prompt) > maxSkyReelsPromptBytes {
		return invalid("SkyReels prompt exceeds application byte limit")
	}
	if req.DurationSec < 3 || req.DurationSec > 15 {
		return invalid("unsupported SkyReels duration")
	}
	if !slices.Contains(skyReelsResolutions, strings.ToLower(strings.TrimSpace(req.Resolution))) {
		return invalid("unsupported SkyReels resolution")
	}
	if strings.TrimSpace(req.AspectRatio) != "" && !slices.Contains(skyReelsAspectRatios, strings.TrimSpace(req.AspectRatio)) {
		return invalid("unsupported SkyReels aspect ratio")
	}
	if media == nil {
		return nil
	}
	if media.Audio != nil && (media.Audio.Setting != "" || strings.TrimSpace(media.Audio.URL) != "") {
		return invalid("SkyReels global audio controls are unsupported")
	}
	i2v := hasSkyReelsI2V(media)
	omni := hasSkyReelsOmni(media)
	if i2v && omni {
		return invalid("SkyReels I2V and Omni inputs are mutually exclusive")
	}
	if media.Mode != "" {
		if i2v && media.Mode != domain.VideoMediaModeImage {
			return invalid("SkyReels I2V media mode mismatch")
		}
		if omni && media.Mode != domain.VideoMediaModeOmni {
			return invalid("SkyReels Omni media mode mismatch")
		}
	}
	if err := validateSkyReelsI2V(media, req.DurationSec, prompt); err != nil {
		return err
	}
	if err := validateSkyReelsOmni(media, prompt); err != nil {
		return err
	}
	return nil
}

func hasSkyReelsI2V(media *domain.VideoMediaRequest) bool {
	if media == nil {
		return false
	}
	return media.StartFrame != nil && strings.TrimSpace(media.StartFrame.URL) != "" ||
		media.EndFrame != nil && strings.TrimSpace(media.EndFrame.URL) != "" ||
		len(media.KeyFrames) > 0
}

func hasSkyReelsOmni(media *domain.VideoMediaRequest) bool {
	if media == nil {
		return false
	}
	return len(media.ReferenceImageGroups) > 0 || len(media.ReferenceVideos) > 0
}

func validateSkyReelsI2V(media *domain.VideoMediaRequest, duration int, prompt string) error {
	if media == nil {
		return nil
	}
	if media.StartFrame != nil && strings.TrimSpace(media.StartFrame.URL) != "" {
		if err := validateNativeVideoImage(media.StartFrame.URL, "SkyReels first frame must be a public HTTPS image URL"); err != nil {
			return err
		}
	}
	if media.EndFrame != nil && strings.TrimSpace(media.EndFrame.URL) != "" {
		if err := validateNativeVideoImage(media.EndFrame.URL, "SkyReels end frame must be a public HTTPS image URL"); err != nil {
			return err
		}
	}
	if len(media.KeyFrames) > 6 {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "SkyReels key frame limit is 6"}
	}
	for _, frame := range media.KeyFrames {
		if err := validateSkyReelsTag(frame.Tag, prompt); err != nil {
			return err
		}
		if err := validateNativeVideoImage(frame.URL, "SkyReels key frame must be a public HTTPS image URL"); err != nil {
			return err
		}
		if frame.TimeStampSec != nil && *frame.TimeStampSec != -1 && (*frame.TimeStampSec <= 0 || *frame.TimeStampSec >= duration) {
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "SkyReels key frame timestamp is outside duration"}
		}
	}
	return nil
}

func validateSkyReelsOmni(media *domain.VideoMediaRequest, prompt string) error {
	if media == nil {
		return nil
	}
	if len(media.ReferenceVideos) > 1 {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "SkyReels supports one reference video"}
	}
	imageType := domain.VideoReferenceImageType("")
	for _, group := range media.ReferenceImageGroups {
		if err := validateSkyReelsTag(group.Tag, prompt); err != nil {
			return err
		}
		if group.Type != domain.VideoReferenceImageTypeImage && group.Type != domain.VideoReferenceImageTypeGrid {
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported SkyReels reference image type"}
		}
		if imageType == "" {
			imageType = group.Type
		}
		if group.Type != imageType {
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "SkyReels reference image types cannot be mixed"}
		}
		if group.Type == domain.VideoReferenceImageTypeImage {
			if len(media.ReferenceImageGroups) > 3 || len(cleanInputURLs(group.URLs)) < 1 || len(cleanInputURLs(group.URLs)) > 5 {
				return &Error{Class: domain.ProviderErrInvalidRequest, Message: "SkyReels image reference groups must be 1-3 subjects with 1-5 photos each"}
			}
			if strings.TrimSpace(group.AudioURL) != "" {
				if err := validateKlingVeoPublicHTTPSURL(group.AudioURL, "SkyReels reference audio must be a public HTTPS URL"); err != nil {
					return err
				}
				if group.AudioDurationSec > 15 {
					return &Error{Class: domain.ProviderErrInvalidRequest, Message: "SkyReels reference audio exceeds 15 seconds"}
				}
			}
		}
		if group.Type == domain.VideoReferenceImageTypeGrid {
			if len(media.ReferenceImageGroups) != 1 || len(cleanInputURLs(group.URLs)) != 1 {
				return &Error{Class: domain.ProviderErrInvalidRequest, Message: "SkyReels grid reference requires one group with one image"}
			}
			if strings.TrimSpace(group.AudioURL) != "" {
				return &Error{Class: domain.ProviderErrInvalidRequest, Message: "SkyReels grid audio is unsupported"}
			}
		}
		for _, value := range cleanInputURLs(group.URLs) {
			if err := validateNativeVideoImage(value, "SkyReels reference image must be a public HTTPS image URL"); err != nil {
				return err
			}
		}
	}
	for _, video := range media.ReferenceVideos {
		if err := validateSkyReelsTag(video.Tag, prompt); err != nil {
			return err
		}
		switch video.Type {
		case domain.VideoReferenceVideoTypeReference:
			if video.DurationSec <= 0 || video.DurationSec > 10 {
				return &Error{Class: domain.ProviderErrInvalidRequest, Message: "SkyReels reference video output duration must be 1-10 seconds"}
			}
			if imageType == domain.VideoReferenceImageTypeGrid {
				return &Error{Class: domain.ProviderErrInvalidRequest, Message: "SkyReels reference video cannot be combined with grid images"}
			}
		case domain.VideoReferenceVideoTypeExtend:
			if len(media.ReferenceImageGroups) > 0 {
				return &Error{Class: domain.ProviderErrInvalidRequest, Message: "SkyReels extend video cannot be combined with images"}
			}
		default:
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported SkyReels reference video type"}
		}
		if err := validateKlingVeoPublicHTTPSURL(video.URL, "SkyReels reference video must be a public HTTPS URL"); err != nil {
			return err
		}
	}
	return nil
}

func validateSkyReelsTag(tag, prompt string) error {
	tag = strings.TrimSpace(tag)
	if !strings.HasPrefix(tag, "@") || !strings.Contains(prompt, tag) {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "SkyReels tag must start with @ and appear in prompt"}
	}
	return nil
}

func validateNativeVideoImage(value, message string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: message}
	}
	if strings.HasPrefix(strings.ToLower(value), "data:") {
		_, err := validateGenerationImageInput(value, maxGeminiGenerationImageBytes)
		return err
	}
	return validateKlingVeoPublicHTTPSURL(value, message)
}

func (p *Provider) submitHappyHorseSkyReels(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	return p.submitHappyHorseSkyReelsWithMedia(ctx, req, req.VideoMedia)
}

func (p *Provider) submitHappyHorseSkyReelsWithMedia(ctx context.Context, req domain.ProviderRequest, media *domain.VideoMediaRequest) (domain.ProviderTask, error) {
	if err := validateHappyHorseSkyReelsRequestWithMedia(req, media); err != nil {
		return domain.ProviderTask{}, err
	}
	return p.submitUnversionedOnce(ctx, req, func(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
		return p.submitHappyHorseSkyReelsOnce(ctx, req, media)
	})
}

func (p *Provider) submitHappyHorseSkyReelsOnce(ctx context.Context, req domain.ProviderRequest, media *domain.VideoMediaRequest) (domain.ProviderTask, error) {
	body, err := p.happyHorseSkyReelsBody(ctx, req, media)
	if err != nil {
		return domain.ProviderTask{}, err
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return domain.ProviderTask{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid APIMart native video request"}
	}
	return p.postUnversionedTask(ctx, req, "/videos/generations", raw)
}

func (p *Provider) happyHorseSkyReelsBody(ctx context.Context, req domain.ProviderRequest, media *domain.VideoMediaRequest) (any, error) {
	if err := validateHappyHorseSkyReelsRequestWithMedia(req, media); err != nil {
		return nil, err
	}
	if isHappyHorseModel(req.ModelCode) {
		return p.happyHorseBody(ctx, req, media)
	}
	return p.skyReelsBody(ctx, req, media)
}

func (p *Provider) happyHorseBody(ctx context.Context, req domain.ProviderRequest, media *domain.VideoMediaRequest) (happyHorseVideoRequest, error) {
	state := classifyHappyHorseMedia(media)
	body := happyHorseVideoRequest{
		Model:      strings.TrimSpace(req.ModelCode),
		Prompt:     strings.TrimSpace(req.Prompt),
		Resolution: normalizeHappyHorseResolution(req.Resolution),
	}
	if media != nil {
		body.Seed = media.Seed
	}
	if !state.hasVideo {
		duration := req.DurationSec
		body.Duration = &duration
	}
	if !state.hasVideo && !state.hasFirst {
		body.Size = strings.TrimSpace(req.AspectRatio)
	}
	if media == nil {
		return body, nil
	}
	if state.hasFirst {
		firstFrame, err := p.prepareNativeVideoImage(ctx, media.StartFrame.URL)
		if err != nil {
			return happyHorseVideoRequest{}, err
		}
		body.FirstFrameImage = firstFrame
	}
	if state.hasVideo {
		body.VideoURL = strings.TrimSpace(media.ReferenceVideos[0].URL)
		if media.Audio != nil && media.Audio.Setting != "" {
			body.AudioSetting = string(media.Audio.Setting)
		}
	}
	images, err := p.prepareHappyHorseReferenceImages(ctx, media)
	if err != nil {
		return happyHorseVideoRequest{}, err
	}
	body.ImageURLs = images
	return body, nil
}

func (p *Provider) prepareHappyHorseReferenceImages(ctx context.Context, media *domain.VideoMediaRequest) ([]string, error) {
	if media == nil {
		return nil, nil
	}
	var images []string
	for _, group := range media.ReferenceImageGroups {
		for _, value := range cleanInputURLs(group.URLs) {
			imageURL, err := p.prepareNativeVideoImage(ctx, value)
			if err != nil {
				return nil, err
			}
			images = append(images, imageURL)
		}
	}
	return images, nil
}

func (p *Provider) skyReelsBody(ctx context.Context, req domain.ProviderRequest, media *domain.VideoMediaRequest) (skyReelsVideoRequest, error) {
	promptOptimizer := true
	if media != nil && media.PromptOptimizer != nil {
		promptOptimizer = *media.PromptOptimizer
	}
	body := skyReelsVideoRequest{
		Model:           strings.TrimSpace(req.ModelCode),
		Prompt:          strings.TrimSpace(req.Prompt),
		Resolution:      strings.ToLower(strings.TrimSpace(req.Resolution)),
		PromptOptimizer: &promptOptimizer,
	}
	if media == nil || !hasReferenceVideoType(media, domain.VideoReferenceVideoTypeReference) {
		duration := req.DurationSec
		body.Duration = &duration
	}
	if media == nil || (!hasSkyReelsI2V(media) && !hasReferenceVideo(media)) {
		body.AspectRatio = strings.TrimSpace(req.AspectRatio)
	}
	if media == nil {
		return body, nil
	}
	if media.StartFrame != nil && strings.TrimSpace(media.StartFrame.URL) != "" {
		firstFrame, err := p.prepareNativeVideoImage(ctx, media.StartFrame.URL)
		if err != nil {
			return skyReelsVideoRequest{}, err
		}
		body.FirstFrameImage = firstFrame
	}
	if media.EndFrame != nil && strings.TrimSpace(media.EndFrame.URL) != "" {
		endFrame, err := p.prepareNativeVideoImage(ctx, media.EndFrame.URL)
		if err != nil {
			return skyReelsVideoRequest{}, err
		}
		body.EndFrameImage = endFrame
	}
	keyFrames, err := p.prepareSkyReelsKeyFrames(ctx, media.KeyFrames)
	if err != nil {
		return skyReelsVideoRequest{}, err
	}
	body.MidFrameImages = keyFrames
	refImages, err := p.prepareSkyReelsRefImages(ctx, media.ReferenceImageGroups)
	if err != nil {
		return skyReelsVideoRequest{}, err
	}
	body.ReferenceImages = refImages
	body.ReferenceVideos = prepareSkyReelsRefVideos(media.ReferenceVideos)
	return body, nil
}

func (p *Provider) prepareSkyReelsKeyFrames(ctx context.Context, frames []domain.VideoKeyFrame) ([]skyReelsKeyFrameBody, error) {
	out := make([]skyReelsKeyFrameBody, 0, len(frames))
	for _, frame := range frames {
		imageURL, err := p.prepareNativeVideoImage(ctx, frame.URL)
		if err != nil {
			return nil, err
		}
		out = append(out, skyReelsKeyFrameBody{Tag: strings.TrimSpace(frame.Tag), ImageURL: imageURL, TimeStamp: frame.TimeStampSec})
	}
	return out, nil
}

func (p *Provider) prepareSkyReelsRefImages(ctx context.Context, groups []domain.VideoReferenceImageGroup) ([]skyReelsRefImageBody, error) {
	out := make([]skyReelsRefImageBody, 0, len(groups))
	for _, group := range groups {
		images := make([]string, 0, len(group.URLs))
		for _, value := range cleanInputURLs(group.URLs) {
			imageURL, err := p.prepareNativeVideoImage(ctx, value)
			if err != nil {
				return nil, err
			}
			images = append(images, imageURL)
		}
		out = append(out, skyReelsRefImageBody{
			Tag:       strings.TrimSpace(group.Tag),
			Type:      string(group.Type),
			ImageURLs: images,
			AudioURL:  strings.TrimSpace(group.AudioURL),
		})
	}
	return out, nil
}

func prepareSkyReelsRefVideos(videos []domain.VideoReferenceVideo) []skyReelsRefVideoBody {
	out := make([]skyReelsRefVideoBody, 0, len(videos))
	for _, video := range videos {
		out = append(out, skyReelsRefVideoBody{
			Tag:      strings.TrimSpace(video.Tag),
			Type:     string(video.Type),
			VideoURL: strings.TrimSpace(video.URL),
		})
	}
	return out
}

func hasReferenceVideo(media *domain.VideoMediaRequest) bool {
	return media != nil && len(media.ReferenceVideos) > 0
}

func hasReferenceVideoType(media *domain.VideoMediaRequest, typ domain.VideoReferenceVideoType) bool {
	if media == nil {
		return false
	}
	for _, video := range media.ReferenceVideos {
		if video.Type == typ {
			return true
		}
	}
	return false
}

func (p *Provider) prepareNativeVideoImage(ctx context.Context, value string) (string, error) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(value), "data:") {
		imageURL, err := p.prepareFirstFrameImage(ctx, value)
		if err != nil {
			return "", err
		}
		if err := validateKlingVeoPublicHTTPSURL(imageURL, "APIMart upload returned invalid image URL"); err != nil {
			return "", err
		}
		return imageURL, nil
	}
	if err := validateKlingVeoPublicHTTPSURL(value, "reference image must be a public HTTPS URL"); err != nil {
		return "", err
	}
	return value, nil
}

func normalizeHappyHorseResolution(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "720P":
		return "720P"
	case "1080P":
		return "1080P"
	default:
		return strings.TrimSpace(value)
	}
}
