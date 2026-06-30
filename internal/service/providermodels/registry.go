// Package providermodels defines the central provider/model registry that will
// become the source of truth for public model ids, provider model ids, feature
// gates, readiness requirements, route limits and media policy metadata.
package providermodels

import (
	"fmt"
	"strings"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

const (
	PublicTextChatGPT        = "chatgpt"
	ProviderModelDeepSeekV4  = "deepseek-ai/DeepSeek-V4-Flash"
	PublicImageNanoBanana2   = "nano_banana_2"
	PublicImageNanoBananaPro = "nano_banana_pro"
	PublicImageGPTImage2     = "gpt_image_2"
	LoadTestImageMock        = "mock_image"

	ProviderModelPoYoNanoBanana2 = "nano-banana-2"
	ProviderModelGemini3ProImage = "gemini-3-pro-image-preview"
	ProviderModelGPTImage2       = "gpt-image-2"
	ProviderModelMockImage       = "mock-image"

	FeatureImageNanoBanana2   = "FEATURE_IMAGE_MODEL_NANO_BANANA_2_ENABLED"
	FeatureImageNanoBananaPro = "FEATURE_IMAGE_MODEL_NANO_BANANA_PRO_ENABLED"
	FeatureImageGPTImage2     = "FEATURE_IMAGE_MODEL_GPT_IMAGE_2_ENABLED"
	FeatureImageMock          = "FEATURE_IMAGE_MODEL_MOCK_ENABLED"

	FeatureVideoRouter             = "FEATURE_VIDEO_ROUTER_ENABLED"
	FeatureVideoHailuo23Fast       = "FEATURE_VIDEO_ROUTE_HAILUO_2_3_FAST_ENABLED"
	FeatureVideoHailuo23Standard   = "FEATURE_VIDEO_ROUTE_HAILUO_2_3_STANDARD_ENABLED"
	FeatureVideoKlingO3Standard    = "FEATURE_VIDEO_ROUTE_KLING_O3_STANDARD_ENABLED"
	FeatureVideoRunwayGen4Turbo    = "FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_TURBO_ENABLED"
	FeatureVideoSeedance20Fast     = "FEATURE_VIDEO_ROUTE_SEEDANCE_2_0_FAST_ENABLED"
	FeatureVideoRunwayGen45        = "FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_5_ENABLED"
	FeatureVideoMockTextToVideo    = "FEATURE_VIDEO_ROUTE_MOCK_TEXT_TO_VIDEO_ENABLED"
	FeatureVideoResellerExperiment = "FEATURE_VIDEO_ROUTE_RESELLER_EXPERIMENTS_ENABLED"

	ProviderFlagAPIMart  = "APIMART_PROVIDER_ENABLED"
	ProviderFlagPoYo     = "POYO_PROVIDER_ENABLED"
	ProviderFlagRunway   = "RUNWAY_PROVIDER_ENABLED"
	ProviderFlagLoadtest = "APP_ENV"

	ConfigKeyAPIMartAPIKey  = "APIMART_API_KEY"
	ConfigKeyAPIMartBaseURL = "APIMART_BASE_URL"
	ConfigKeyPoYoAPIKey     = "POYO_API_KEY"
	ConfigKeyPoYoBaseURL    = "POYO_BASE_URL"
	ConfigKeyRunwaySecret   = "RUNWAYML_API_SECRET"
	ConfigKeyRunwayBaseURL  = "RUNWAYML_BASE_URL"
	ConfigKeyDeepInfraKey   = "DEEPINFRA_API_KEY"
	ConfigKeyDeepInfraURL   = "DEEPINFRA_BASE_URL"
)

// ProviderReadiness is static config metadata only. It stores env/config names,
// never values, and performs no provider calls.
type ProviderReadiness struct {
	ProviderEnabledFlag string
	RequiredConfigKeys  []string
	LoadTestOnly        bool
}

// Limits describes public request/media bounds for one model or route.
type Limits struct {
	AllowedQualities       []string
	AllowedDurationsSec    []int
	AllowedResolutions     []string
	AllowedAspectRatios    []string
	ResolutionDurationsSec map[string][]int
	SupportsReferenceImage bool
	RequiresStartImage     bool
	MaxReferenceImages     int
}

// TextAlias is the public text model alias mapped to the hidden provider model.
type TextAlias struct {
	PublicID        string
	DisplayName     string
	Provider        domain.ProviderName
	ProviderModelID string
	FeatureFlag     string
	Readiness       ProviderReadiness
	PricingKeys     []pricingcatalog.ProductKey
}

// ImageModel maps one priced public image model id to its provider metadata.
type ImageModel struct {
	PublicID        string
	DisplayName     string
	Provider        domain.ProviderName
	ProviderModelID string
	FeatureFlag     string
	Readiness       ProviderReadiness
	Limits          Limits
	PricingKeys     []pricingcatalog.ProductKey
	LoadTestOnly    bool
}

// MediaContractClass is the static portion of provider media contract policy.
// Runtime fields such as max bytes, probe and transcode toggles remain worker
// config until phase_5 wires worker contracts through the registry.
type MediaContractClass struct {
	ModelClass          string
	Modality            domain.Modality
	ExpectedContainer   string
	ExpectedCodec       string
	DeliveryReadyOutput bool
}

// VideoRoute maps one public route alias to provider and media metadata.
type VideoRoute struct {
	Alias               domain.VideoRouteAlias
	Provider            domain.ProviderName
	ProviderModelID     string
	ModelClass          string
	FeatureFlag         string
	RouterFeatureFlag   string
	Readiness           ProviderReadiness
	Spec                domain.VideoRouteSpec
	Limits              Limits
	MediaContract       MediaContractClass
	PricingKeys         []pricingcatalog.ProductKey
	DisabledPricingKeys []pricingcatalog.ProductKey
	LoadTestOnly        bool
}

// ProviderModelAlias maps legacy server-side model selections to public video
// route aliases without exposing provider-native ids to clients.
type ProviderModelAlias struct {
	Alias           domain.VideoRouteAlias
	ProviderModelID string
}

// Registry is the static provider/model registry.
type Registry struct {
	TextAliases         []TextAlias
	ImageModels         []ImageModel
	LoadTestImageModels []ImageModel
	VideoRouteModels    []VideoRoute
}

// StaticRegistry returns the current static provider/model registry.
func StaticRegistry() Registry {
	return Registry{
		TextAliases:         textAliases(),
		ImageModels:         imageModels(),
		LoadTestImageModels: loadTestImageModels(),
		VideoRouteModels:    videoRoutes(),
	}
}

func textAliases() []TextAlias {
	return []TextAlias{
		{
			PublicID:        PublicTextChatGPT,
			DisplayName:     "NeiroHub Chat",
			Provider:        domain.ProviderDeepInfra,
			ProviderModelID: ProviderModelDeepSeekV4,
			Readiness: ProviderReadiness{
				RequiredConfigKeys: []string{ConfigKeyDeepInfraKey, ConfigKeyDeepInfraURL},
			},
		},
	}
}

func imageModels() []ImageModel {
	return []ImageModel{
		imageModel(PublicImageNanoBanana2, "Nano Banana 2", domain.ProviderPoYo, ProviderModelPoYoNanoBanana2, FeatureImageNanoBanana2, poyoReadiness(), 4),
		imageModel(PublicImageNanoBananaPro, "Nano Banana Pro", domain.ProviderAPIMart, ProviderModelGemini3ProImage, FeatureImageNanoBananaPro, apimartReadiness(), 14),
		imageModel(PublicImageGPTImage2, "GPT Image 2", domain.ProviderAPIMart, ProviderModelGPTImage2, FeatureImageGPTImage2, apimartReadiness(), 16),
	}
}

func loadTestImageModels() []ImageModel {
	return []ImageModel{
		{
			PublicID:        LoadTestImageMock,
			DisplayName:     "Mock Image Loadtest",
			Provider:        domain.ProviderMock,
			ProviderModelID: ProviderModelMockImage,
			FeatureFlag:     FeatureImageMock,
			Readiness:       mockReadiness(),
			LoadTestOnly:    true,
		},
	}
}

func imageModel(publicID, displayName string, provider domain.ProviderName, providerModelID, featureFlag string, readiness ProviderReadiness, maxRefs int) ImageModel {
	qualities := []string{pricingcatalog.ImageQuality1K, pricingcatalog.ImageQuality2K, pricingcatalog.ImageQuality4K}
	keys := make([]pricingcatalog.ProductKey, 0, len(qualities))
	for _, quality := range qualities {
		keys = append(keys, pricingcatalog.ProductKey{
			Operation:    domain.OperationImageGenerate,
			Modality:     domain.ModalityImage,
			ImageModelID: publicID,
			Quality:      quality,
		})
	}
	return ImageModel{
		PublicID:        publicID,
		DisplayName:     displayName,
		Provider:        provider,
		ProviderModelID: providerModelID,
		FeatureFlag:     featureFlag,
		Readiness:       readiness,
		Limits: Limits{
			AllowedQualities:       qualities,
			SupportsReferenceImage: true,
			MaxReferenceImages:     maxRefs,
		},
		PricingKeys: keys,
	}
}

func videoRoutes() []VideoRoute {
	return []VideoRoute{
		videoRoute(hailuo23FastSpec(), FeatureVideoHailuo23Fast, apimartReadiness(), nil, disabledVideoPricingKeys(domain.VideoRouteHailuo23Fast, []string{pricingcatalog.VideoResolution768p, pricingcatalog.VideoResolution1080p}, map[string][]int{
			pricingcatalog.VideoResolution768p:  {6, 10},
			pricingcatalog.VideoResolution1080p: {6},
		}), false),
		videoRoute(hailuo23StandardSpec(), FeatureVideoHailuo23Standard, apimartReadiness(), nil, disabledVideoPricingKeys(domain.VideoRouteHailuo23Standard, []string{pricingcatalog.VideoResolution768p, pricingcatalog.VideoResolution1080p}, map[string][]int{
			pricingcatalog.VideoResolution768p:  {6, 10},
			pricingcatalog.VideoResolution1080p: {6},
		}), false),
		videoRoute(klingO3StandardSpec(), FeatureVideoKlingO3Standard, poyoReadiness(), videoPricingKeys(domain.VideoRouteKlingO3Standard, []string{pricingcatalog.VideoResolution720p, pricingcatalog.VideoResolution1080p}, []int{5, 10}), nil, false),
		videoRoute(runwayGen4TurboSpec(), FeatureVideoRunwayGen4Turbo, runwayReadiness(), videoPricingKeys(domain.VideoRouteRunwayGen4Turbo, []string{pricingcatalog.VideoResolution720p}, []int{2, 3, 4, 5, 6, 7, 8, 9, 10}), nil, false),
		videoRoute(seedance20FastSpec(), FeatureVideoSeedance20Fast, poyoReadiness(), videoPricingKeys(domain.VideoRouteSeedance20Fast, []string{pricingcatalog.VideoResolution720p}, []int{5, 10}), nil, false),
		videoRoute(runwayGen45Spec(), FeatureVideoRunwayGen45, poyoReadiness(), videoPricingKeys(domain.VideoRouteRunwayGen45, []string{pricingcatalog.VideoResolution720p, pricingcatalog.VideoResolution1080p}, []int{5, 10}), nil, false),
		videoRoute(mockTextToVideoSpec(), FeatureVideoMockTextToVideo, mockReadiness(), nil, nil, true),
	}
}

func videoRoute(spec domain.VideoRouteSpec, featureFlag string, readiness ProviderReadiness, pricingKeys, disabledPricingKeys []pricingcatalog.ProductKey, loadTestOnly bool) VideoRoute {
	return VideoRoute{
		Alias:             spec.Alias,
		Provider:          spec.Provider,
		ProviderModelID:   spec.ProviderModelID,
		ModelClass:        spec.ModelClass,
		FeatureFlag:       featureFlag,
		RouterFeatureFlag: FeatureVideoRouter,
		Readiness:         readiness,
		Spec:              spec,
		Limits: Limits{
			AllowedDurationsSec:    append([]int(nil), spec.AllowedDurationsSec...),
			AllowedResolutions:     append([]string(nil), spec.AllowedResolutions...),
			AllowedAspectRatios:    append([]string(nil), spec.AllowedAspectRatios...),
			ResolutionDurationsSec: copyResolutionDurations(spec.ResolutionDurationsSec),
			SupportsReferenceImage: spec.SupportsReferenceImage,
			RequiresStartImage:     spec.RequiresStartImage,
			MaxReferenceImages:     spec.MaxReferenceImages,
		},
		MediaContract: MediaContractClass{
			ModelClass:          spec.ModelClass,
			Modality:            domain.ModalityVideo,
			ExpectedContainer:   "mp4",
			ExpectedCodec:       "h264",
			DeliveryReadyOutput: true,
		},
		PricingKeys:         append([]pricingcatalog.ProductKey(nil), pricingKeys...),
		DisabledPricingKeys: append([]pricingcatalog.ProductKey(nil), disabledPricingKeys...),
		LoadTestOnly:        loadTestOnly,
	}
}

func hailuo23FastSpec() domain.VideoRouteSpec {
	return domain.VideoRouteSpec{
		Alias:               domain.VideoRouteHailuo23Fast,
		Provider:            domain.ProviderAPIMart,
		ProviderModelID:     "MiniMax-Hailuo-2.3-Fast",
		ModelClass:          "hailuo_2_3_fast",
		InputModes:          []domain.VideoInputMode{domain.VideoInputImage},
		RequiresStartImage:  true,
		AllowedDurationsSec: []int{6, 10},
		AllowedResolutions:  []string{"768p", "1080p"},
		ResolutionDurationsSec: map[string][]int{
			"768p":  {6, 10},
			"1080p": {6},
		},
		SupportsReferenceImage:   true,
		MaxReferenceImages:       1,
		ProviderCostCreditsFixed: 1,
		MaxProviderCostCredits:   1,
		MaxInternalCostCredits:   2,
		PriceMultiplier:          2,
	}
}

func hailuo23StandardSpec() domain.VideoRouteSpec {
	return domain.VideoRouteSpec{
		Alias:               domain.VideoRouteHailuo23Standard,
		Provider:            domain.ProviderAPIMart,
		ProviderModelID:     "MiniMax-Hailuo-2.3",
		ModelClass:          "hailuo_2_3_standard",
		InputModes:          []domain.VideoInputMode{domain.VideoInputText, domain.VideoInputImage},
		AllowedDurationsSec: []int{6, 10},
		AllowedResolutions:  []string{"768p", "1080p"},
		ResolutionDurationsSec: map[string][]int{
			"768p":  {6, 10},
			"1080p": {6},
		},
		SupportsReferenceImage:   true,
		MaxReferenceImages:       1,
		ProviderCostCreditsFixed: 1,
		MaxProviderCostCredits:   1,
		MaxInternalCostCredits:   2,
		PriceMultiplier:          2,
	}
}

func klingO3StandardSpec() domain.VideoRouteSpec {
	return domain.VideoRouteSpec{
		Alias:                        domain.VideoRouteKlingO3Standard,
		Provider:                     domain.ProviderPoYo,
		ProviderModelID:              "kling-o3/standard",
		ModelClass:                   "kling_o3_standard",
		InputModes:                   []domain.VideoInputMode{domain.VideoInputText, domain.VideoInputImage},
		AllowedDurationsSec:          []int{5, 10},
		AllowedResolutions:           []string{"720p", "1080p"},
		AllowedAspectRatios:          []string{"16:9", "9:16", "1:1"},
		SupportsReferenceImage:       true,
		MaxReferenceImages:           1,
		ProviderCostCreditsPerSecond: 10,
		MaxProviderCostCredits:       100,
		MaxInternalCostCredits:       200,
		PriceMultiplier:              2,
	}
}

func runwayGen4TurboSpec() domain.VideoRouteSpec {
	return domain.VideoRouteSpec{
		Alias:                        domain.VideoRouteRunwayGen4Turbo,
		Provider:                     domain.ProviderRunway,
		ProviderModelID:              "gen4_turbo",
		ModelClass:                   "runway_gen4_turbo",
		InputModes:                   []domain.VideoInputMode{domain.VideoInputImage},
		RequiresStartImage:           true,
		AllowedDurationsSec:          []int{2, 3, 4, 5, 6, 7, 8, 9, 10},
		AllowedResolutions:           []string{"720p"},
		AllowedAspectRatios:          []string{"16:9", "9:16", "4:3", "3:4", "1:1", "21:9"},
		SupportsReferenceImage:       true,
		MaxReferenceImages:           1,
		ProviderCostCreditsPerSecond: 5,
		MaxProviderCostCredits:       50,
		MaxInternalCostCredits:       100,
		PriceMultiplier:              2,
	}
}

func seedance20FastSpec() domain.VideoRouteSpec {
	return domain.VideoRouteSpec{
		Alias:                        domain.VideoRouteSeedance20Fast,
		Provider:                     domain.ProviderPoYo,
		ProviderModelID:              "seedance-2-fast",
		ModelClass:                   "seedance_2_0_fast",
		InputModes:                   []domain.VideoInputMode{domain.VideoInputText, domain.VideoInputImage, domain.VideoInputReference},
		AllowedDurationsSec:          []int{5, 10},
		AllowedResolutions:           []string{"720p"},
		AllowedAspectRatios:          []string{"16:9", "9:16", "1:1"},
		SupportsReferenceImage:       true,
		MaxReferenceImages:           4,
		ProviderCostCreditsPerSecond: 28,
		MaxProviderCostCredits:       280,
		MaxInternalCostCredits:       560,
		PriceMultiplier:              2,
	}
}

func runwayGen45Spec() domain.VideoRouteSpec {
	return domain.VideoRouteSpec{
		Alias:                  domain.VideoRouteRunwayGen45,
		Provider:               domain.ProviderPoYo,
		ProviderModelID:        "runway-gen-4.5",
		ModelClass:             "runway_gen4_5",
		InputModes:             []domain.VideoInputMode{domain.VideoInputText, domain.VideoInputImage},
		AllowedDurationsSec:    []int{5, 10},
		AllowedResolutions:     []string{"720p", "1080p"},
		AllowedAspectRatios:    []string{"16:9", "9:16", "1:1"},
		SupportsReferenceImage: true,
		MaxReferenceImages:     1,
		MaxProviderCostCredits: 0,
		PriceMultiplier:        2,
	}
}

func mockTextToVideoSpec() domain.VideoRouteSpec {
	return domain.VideoRouteSpec{
		Alias:                    domain.VideoRouteMockTextToVideo,
		Provider:                 domain.ProviderMock,
		ProviderModelID:          "mock-video",
		ModelClass:               "mock_video",
		InputModes:               []domain.VideoInputMode{domain.VideoInputText},
		AllowedDurationsSec:      []int{3, 5, 10},
		AllowedResolutions:       []string{"720p", "1080p"},
		AllowedAspectRatios:      []string{"16:9", "9:16", "1:1"},
		ProviderCostCreditsFixed: 50,
		MaxProviderCostCredits:   50,
		MaxInternalCostCredits:   50,
		PriceMultiplier:          1,
	}
}

func apimartReadiness() ProviderReadiness {
	return ProviderReadiness{
		ProviderEnabledFlag: ProviderFlagAPIMart,
		RequiredConfigKeys:  []string{ConfigKeyAPIMartAPIKey, ConfigKeyAPIMartBaseURL},
	}
}

func poyoReadiness() ProviderReadiness {
	return ProviderReadiness{
		ProviderEnabledFlag: ProviderFlagPoYo,
		RequiredConfigKeys:  []string{ConfigKeyPoYoAPIKey, ConfigKeyPoYoBaseURL},
	}
}

func runwayReadiness() ProviderReadiness {
	return ProviderReadiness{
		ProviderEnabledFlag: ProviderFlagRunway,
		RequiredConfigKeys:  []string{ConfigKeyRunwaySecret, ConfigKeyRunwayBaseURL},
	}
}

func mockReadiness() ProviderReadiness {
	return ProviderReadiness{
		ProviderEnabledFlag: ProviderFlagLoadtest,
		RequiredConfigKeys:  []string{"PROVIDER", "PROVIDER_CHAIN", "IMAGE_PROVIDER", "VIDEO_PROVIDER"},
		LoadTestOnly:        true,
	}
}

func videoPricingKeys(alias domain.VideoRouteAlias, resolutions []string, durations []int) []pricingcatalog.ProductKey {
	keys := make([]pricingcatalog.ProductKey, 0, len(resolutions)*len(durations))
	for _, resolution := range resolutions {
		for _, duration := range durations {
			keys = append(keys, pricingcatalog.ProductKey{
				Operation:       domain.OperationVideoGenerate,
				Modality:        domain.ModalityVideo,
				VideoRouteAlias: alias,
				Resolution:      resolution,
				DurationSec:     duration,
			})
		}
	}
	return keys
}

func disabledVideoPricingKeys(alias domain.VideoRouteAlias, resolutions []string, durationsByResolution map[string][]int) []pricingcatalog.ProductKey {
	keys := make([]pricingcatalog.ProductKey, 0)
	for _, resolution := range resolutions {
		for _, duration := range durationsByResolution[resolution] {
			keys = append(keys, pricingcatalog.ProductKey{
				Operation:       domain.OperationVideoGenerate,
				Modality:        domain.ModalityVideo,
				VideoRouteAlias: alias,
				Resolution:      resolution,
				DurationSec:     duration,
			})
		}
	}
	return keys
}

// PublicImageModels returns priced public image models. Loadtest-only models are
// exposed separately through LoadTestImageModels to avoid accidental public sale.
func (r Registry) PublicImageModels() []ImageModel {
	return copyImageModels(r.ImageModels)
}

func (r Registry) PublicImageModel(publicID string) (ImageModel, bool) {
	for _, model := range r.ImageModels {
		if model.PublicID == publicID {
			return copyImageModel(model), true
		}
	}
	return ImageModel{}, false
}

func (r Registry) LoadTestImageModel(publicID string) (ImageModel, bool) {
	for _, model := range r.LoadTestImageModels {
		if model.PublicID == publicID {
			return copyImageModel(model), true
		}
	}
	return ImageModel{}, false
}

func (r Registry) TextAlias(publicID string) (TextAlias, bool) {
	for _, alias := range r.TextAliases {
		if alias.PublicID == publicID {
			return copyTextAlias(alias), true
		}
	}
	return TextAlias{}, false
}

func (r Registry) TextAliasModels() []TextAlias {
	out := make([]TextAlias, 0, len(r.TextAliases))
	for _, alias := range r.TextAliases {
		out = append(out, copyTextAlias(alias))
	}
	return out
}

func (r Registry) VideoRoutes() []VideoRoute {
	out := make([]VideoRoute, 0, len(r.VideoRouteModels))
	for _, route := range r.VideoRouteModels {
		out = append(out, copyVideoRoute(route))
	}
	return out
}

func (r Registry) VideoRoute(alias domain.VideoRouteAlias) (VideoRoute, bool) {
	for _, route := range r.VideoRouteModels {
		if route.Alias == alias {
			return copyVideoRoute(route), true
		}
	}
	return VideoRoute{}, false
}

func (r Registry) VideoRouteSpecs() []domain.VideoRouteSpec {
	out := make([]domain.VideoRouteSpec, 0, len(r.VideoRouteModels))
	for _, route := range r.VideoRouteModels {
		out = append(out, copyVideoRouteSpec(route.Spec))
	}
	return out
}

func (r Registry) ProviderModelAliases() []ProviderModelAlias {
	return []ProviderModelAlias{
		{Alias: domain.VideoRouteKlingO3Standard, ProviderModelID: "kling-o3"},
		{Alias: domain.VideoRouteKlingO3Standard, ProviderModelID: "kling-o3-standard"},
		{Alias: domain.VideoRouteKlingO3Standard, ProviderModelID: "Kling O3 Standard"},
		{Alias: domain.VideoRouteSeedance20Fast, ProviderModelID: "seedance-2.0-fast"},
		{Alias: domain.VideoRouteRunwayGen4Turbo, ProviderModelID: "runway-gen-4-turbo"},
	}
}

func (r Registry) ProviderReadiness() []ProviderReadiness {
	seen := map[string]struct{}{}
	var out []ProviderReadiness
	add := func(readiness ProviderReadiness) {
		key := readiness.ProviderEnabledFlag + "\x00" + strings.Join(readiness.RequiredConfigKeys, "\x00")
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, copyReadiness(readiness))
	}
	for _, alias := range r.TextAliases {
		add(alias.Readiness)
	}
	for _, model := range r.ImageModels {
		add(model.Readiness)
	}
	for _, model := range r.LoadTestImageModels {
		add(model.Readiness)
	}
	for _, route := range r.VideoRouteModels {
		add(route.Readiness)
	}
	return out
}

func (r Registry) Validate() error {
	seenImages := map[string]struct{}{}
	for _, model := range r.ImageModels {
		if err := validateImageModel(model, false); err != nil {
			return err
		}
		if _, exists := seenImages[model.PublicID]; exists {
			return fmt.Errorf("providermodels: duplicate image model %s", model.PublicID)
		}
		seenImages[model.PublicID] = struct{}{}
	}
	for _, model := range r.LoadTestImageModels {
		if err := validateImageModel(model, true); err != nil {
			return err
		}
		if _, exists := seenImages[model.PublicID]; exists {
			return fmt.Errorf("providermodels: duplicate image model %s", model.PublicID)
		}
		seenImages[model.PublicID] = struct{}{}
	}
	seenText := map[string]struct{}{}
	for _, alias := range r.TextAliases {
		if strings.TrimSpace(alias.PublicID) == "" || alias.Provider == "" || strings.TrimSpace(alias.ProviderModelID) == "" {
			return fmt.Errorf("providermodels: text alias %s missing provider metadata", alias.PublicID)
		}
		if _, exists := seenText[alias.PublicID]; exists {
			return fmt.Errorf("providermodels: duplicate text alias %s", alias.PublicID)
		}
		seenText[alias.PublicID] = struct{}{}
	}
	seenRoutes := map[domain.VideoRouteAlias]struct{}{}
	for _, route := range r.VideoRouteModels {
		if err := validateVideoRoute(route); err != nil {
			return err
		}
		if _, exists := seenRoutes[route.Alias]; exists {
			return fmt.Errorf("providermodels: duplicate video route %s", route.Alias)
		}
		seenRoutes[route.Alias] = struct{}{}
	}
	return nil
}

func validateImageModel(model ImageModel, loadTestOnly bool) error {
	if strings.TrimSpace(model.PublicID) == "" {
		return fmt.Errorf("providermodels: image public id is required")
	}
	if model.Provider == "" || strings.TrimSpace(model.ProviderModelID) == "" {
		return fmt.Errorf("providermodels: image %s missing provider metadata", model.PublicID)
	}
	if strings.TrimSpace(model.FeatureFlag) == "" {
		return fmt.Errorf("providermodels: image %s missing feature flag", model.PublicID)
	}
	if !loadTestOnly && len(model.PricingKeys) == 0 {
		return fmt.Errorf("providermodels: image %s missing pricing keys", model.PublicID)
	}
	if !loadTestOnly && len(model.Limits.AllowedQualities) == 0 {
		return fmt.Errorf("providermodels: image %s missing quality limits", model.PublicID)
	}
	if !loadTestOnly && model.Limits.SupportsReferenceImage && model.Limits.MaxReferenceImages <= 0 {
		return fmt.Errorf("providermodels: image %s missing reference limits", model.PublicID)
	}
	return nil
}

func validateVideoRoute(route VideoRoute) error {
	if route.Alias == "" {
		return fmt.Errorf("providermodels: video route alias is required")
	}
	if route.Provider == "" || strings.TrimSpace(route.ProviderModelID) == "" || strings.TrimSpace(route.ModelClass) == "" {
		return fmt.Errorf("providermodels: route %s missing provider metadata", route.Alias)
	}
	if strings.TrimSpace(route.FeatureFlag) == "" || strings.TrimSpace(route.RouterFeatureFlag) == "" {
		return fmt.Errorf("providermodels: route %s missing feature flag", route.Alias)
	}
	if err := route.Spec.Validate(); err != nil {
		return err
	}
	if route.MediaContract.Modality != domain.ModalityVideo || strings.TrimSpace(route.MediaContract.ModelClass) == "" {
		return fmt.Errorf("providermodels: route %s missing media contract class", route.Alias)
	}
	if !route.LoadTestOnly && len(route.PricingKeys) == 0 && len(route.DisabledPricingKeys) == 0 {
		return fmt.Errorf("providermodels: route %s missing pricing keys", route.Alias)
	}
	return nil
}

func (r Registry) ValidatePricingCoverage(catalog *pricingcatalog.Catalog, disabled []pricingcatalog.DisabledProductPrice) error {
	if catalog == nil {
		return fmt.Errorf("providermodels: pricing catalog is required")
	}
	disabledKeys := map[pricingcatalog.ProductKey]struct{}{}
	for _, price := range disabled {
		disabledKeys[price.Key.Normalize()] = struct{}{}
	}
	for _, model := range r.ImageModels {
		for _, key := range model.PricingKeys {
			if _, err := catalog.Lookup(key); err != nil {
				return fmt.Errorf("providermodels: image %s pricing key missing: %w", model.PublicID, err)
			}
		}
	}
	for _, route := range r.VideoRouteModels {
		for _, key := range route.PricingKeys {
			if _, err := catalog.Lookup(key); err != nil {
				return fmt.Errorf("providermodels: route %s pricing key missing: %w", route.Alias, err)
			}
		}
		for _, key := range route.DisabledPricingKeys {
			if _, ok := disabledKeys[key.Normalize()]; !ok {
				return fmt.Errorf("providermodels: route %s disabled pricing key missing: %+v", route.Alias, key)
			}
		}
	}
	return nil
}

func copyTextAlias(alias TextAlias) TextAlias {
	alias.Readiness = copyReadiness(alias.Readiness)
	alias.PricingKeys = append([]pricingcatalog.ProductKey(nil), alias.PricingKeys...)
	return alias
}

func copyImageModels(in []ImageModel) []ImageModel {
	out := make([]ImageModel, 0, len(in))
	for _, model := range in {
		out = append(out, copyImageModel(model))
	}
	return out
}

func copyImageModel(model ImageModel) ImageModel {
	model.Readiness = copyReadiness(model.Readiness)
	model.Limits = copyLimits(model.Limits)
	model.PricingKeys = append([]pricingcatalog.ProductKey(nil), model.PricingKeys...)
	return model
}

func copyVideoRoute(route VideoRoute) VideoRoute {
	route.Readiness = copyReadiness(route.Readiness)
	route.Spec = copyVideoRouteSpec(route.Spec)
	route.Limits = copyLimits(route.Limits)
	route.PricingKeys = append([]pricingcatalog.ProductKey(nil), route.PricingKeys...)
	route.DisabledPricingKeys = append([]pricingcatalog.ProductKey(nil), route.DisabledPricingKeys...)
	return route
}

func copyReadiness(readiness ProviderReadiness) ProviderReadiness {
	readiness.RequiredConfigKeys = append([]string(nil), readiness.RequiredConfigKeys...)
	return readiness
}

func copyLimits(limits Limits) Limits {
	limits.AllowedQualities = append([]string(nil), limits.AllowedQualities...)
	limits.AllowedDurationsSec = append([]int(nil), limits.AllowedDurationsSec...)
	limits.AllowedResolutions = append([]string(nil), limits.AllowedResolutions...)
	limits.AllowedAspectRatios = append([]string(nil), limits.AllowedAspectRatios...)
	limits.ResolutionDurationsSec = copyResolutionDurations(limits.ResolutionDurationsSec)
	return limits
}

func copyVideoRouteSpec(spec domain.VideoRouteSpec) domain.VideoRouteSpec {
	spec.InputModes = append([]domain.VideoInputMode(nil), spec.InputModes...)
	spec.AllowedDurationsSec = append([]int(nil), spec.AllowedDurationsSec...)
	spec.AllowedResolutions = append([]string(nil), spec.AllowedResolutions...)
	spec.AllowedAspectRatios = append([]string(nil), spec.AllowedAspectRatios...)
	spec.ResolutionDurationsSec = copyResolutionDurations(spec.ResolutionDurationsSec)
	return spec
}

func copyResolutionDurations(in map[string][]int) map[string][]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]int, len(in))
	for key, values := range in {
		out[key] = append([]int(nil), values...)
	}
	return out
}
