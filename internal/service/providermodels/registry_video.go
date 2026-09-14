package providermodels

import (
	"fmt"
	"maps"
	"strings"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

const (
	FeatureVideoRouter             = "FEATURE_VIDEO_ROUTER_ENABLED"
	FeatureVideoHailuo23Fast       = "FEATURE_VIDEO_ROUTE_HAILUO_2_3_FAST_ENABLED"
	FeatureVideoHailuo23Standard   = "FEATURE_VIDEO_ROUTE_HAILUO_2_3_STANDARD_ENABLED"
	FeatureVideoKlingO3Standard    = "FEATURE_VIDEO_ROUTE_KLING_O3_STANDARD_ENABLED"
	FeatureVideoRunwayGen4Turbo    = "FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_TURBO_ENABLED"
	FeatureVideoSeedance20Fast     = "FEATURE_VIDEO_ROUTE_SEEDANCE_2_0_FAST_ENABLED"
	FeatureVideoSeedance25         = "FEATURE_APIMART_SEEDANCE_2_5_ENABLED"
	FeatureVideoRunwayGen45        = "FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_5_ENABLED"
	FeatureVideoMockTextToVideo    = "FEATURE_VIDEO_ROUTE_MOCK_TEXT_TO_VIDEO_ENABLED"
	FeatureVideoResellerExperiment = "FEATURE_VIDEO_ROUTE_RESELLER_EXPERIMENTS_ENABLED"
)

// VideoLimits describes public video request/media bounds for one route.
type VideoLimits struct {
	SupportsAudio               bool
	RequiresReferenceVideo      bool
	AutomaticDuration           bool
	AllowedReferenceImageCounts []int
	AllowedDurationsSec         []int
	AllowedResolutions          []string
	AllowedAspectRatios         []string
	ResolutionDurationsSec      map[string][]int
	SupportsReferenceImage      bool
	RequiresStartImage          bool
	MaxReferenceImages          int
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
	Limits              VideoLimits
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

func videoRoutes() []VideoRoute {
	return append(klingVeoRoutes(), []VideoRoute{
		turboH3VideoRoute(false),
		turboH3VideoRoute(true),
		omniVideoRoute(false),
		omniVideoRoute(true),
		videoRoute(seedance25Spec(), FeatureVideoSeedance25, apimartReadiness(), videoPricingKeys(domain.VideoRouteSeedance25, []string{pricingcatalog.VideoResolution480p, pricingcatalog.VideoResolution720p, pricingcatalog.VideoResolution1080p}, []int{5, 10, 15, 30}), nil, false),
		videoRoute(hailuo23FastSpec(), FeatureVideoHailuo23Fast, apimartReadiness(), nil, disabledVideoPricingKeys(domain.VideoRouteHailuo23Fast, []string{pricingcatalog.VideoResolution768p, pricingcatalog.VideoResolution1080p}, map[string][]int{
			pricingcatalog.VideoResolution768p:  {6, 10},
			pricingcatalog.VideoResolution1080p: {6},
		}), false),
		videoRoute(hailuo23StandardSpec(), FeatureVideoHailuo23Standard, apimartReadiness(), nil, disabledVideoPricingKeys(domain.VideoRouteHailuo23Standard, []string{pricingcatalog.VideoResolution768p, pricingcatalog.VideoResolution1080p}, map[string][]int{
			pricingcatalog.VideoResolution768p:  {6, 10},
			pricingcatalog.VideoResolution1080p: {6},
		}), false),
		videoRoute(klingO3StandardSpec(), FeatureVideoKlingO3Standard, poyoReadiness(), videoPricingKeys(domain.VideoRouteKlingO3Standard, []string{pricingcatalog.VideoResolution720p, pricingcatalog.VideoResolution1080p}, []int{5, 10}), nil, false),
		videoRoute(runwayGen4TurboSpec(), FeatureVideoRunwayGen4Turbo, runwayReadiness(), videoPricingKeys(domain.VideoRouteRunwayGen4Turbo, []string{pricingcatalog.VideoResolution720p}, []int{5, 10}), nil, false),
		videoRoute(seedance20FastSpec(), FeatureVideoSeedance20Fast, poyoReadiness(), videoPricingKeys(domain.VideoRouteSeedance20Fast, []string{pricingcatalog.VideoResolution720p}, []int{5, 10}), nil, false),
		videoRoute(runwayGen45Spec(), FeatureVideoRunwayGen45, poyoReadiness(), videoPricingKeys(domain.VideoRouteRunwayGen45, []string{pricingcatalog.VideoResolution720p, pricingcatalog.VideoResolution1080p}, []int{5, 10}), nil, false),
		videoRoute(mockTextToVideoSpec(), FeatureVideoMockTextToVideo, mockReadiness(), nil, nil, true),
	}...)
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
		Limits: VideoLimits{
			AllowedDurationsSec:         append([]int(nil), spec.AllowedDurationsSec...),
			SupportsAudio:               spec.SupportsAudio,
			RequiresReferenceVideo:      spec.RequiresReferenceVideo,
			AutomaticDuration:           spec.AutomaticDuration,
			AllowedReferenceImageCounts: append([]int(nil), spec.AllowedReferenceImageCounts...),
			AllowedResolutions:          append([]string(nil), spec.AllowedResolutions...),
			AllowedAspectRatios:         append([]string(nil), spec.AllowedAspectRatios...),
			ResolutionDurationsSec:      copyResolutionDurations(spec.ResolutionDurationsSec),
			SupportsReferenceImage:      spec.SupportsReferenceImage,
			RequiresStartImage:          spec.RequiresStartImage,
			MaxReferenceImages:          spec.MaxReferenceImages,
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
		AllowedDurationsSec:          []int{5, 10},
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

func seedance25Spec() domain.VideoRouteSpec {
	return domain.VideoRouteSpec{
		Alias:                                   domain.VideoRouteSeedance25,
		Provider:                                domain.ProviderAPIMart,
		ProviderModelID:                         "seedance-2.5",
		ModelClass:                              "seedance_2_5",
		InputModes:                              []domain.VideoInputMode{domain.VideoInputText, domain.VideoInputImage, domain.VideoInputReference},
		AllowedDurationsSec:                     []int{5, 10, 15, 30},
		AllowedResolutions:                      []string{"480p", "720p", "1080p"},
		AllowedAspectRatios:                     []string{"16:9", "9:16", "4:3", "3:4", "1:1", "21:9"},
		SupportsReferenceImage:                  true,
		MaxReferenceImages:                      4,
		ProviderCostMicrosPerSecondByResolution: map[string]int64{"480p": 960800, "720p": 2160000, "1080p": 3848800},
		MaxProviderCostCredits:                  116, MaxInternalCostCredits: 6960,
		// Legacy safety metadata converts APIMart credits ($0.10) to internal
		// credits ($0.005) and applies x3. Exact retail lives in pricingcatalog.
		PriceMultiplier: 60,
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
		Alias:                        domain.VideoRouteRunwayGen45,
		Provider:                     domain.ProviderPoYo,
		ProviderModelID:              "runway-gen-4.5",
		ModelClass:                   "runway_gen4_5",
		InputModes:                   []domain.VideoInputMode{domain.VideoInputText, domain.VideoInputImage},
		AllowedDurationsSec:          []int{5, 10},
		AllowedResolutions:           []string{"720p", "1080p"},
		AllowedAspectRatios:          []string{"16:9", "9:16", "4:3", "3:4", "1:1", "21:9"},
		SupportsReferenceImage:       true,
		MaxReferenceImages:           1,
		ProviderCostCreditsPerSecond: 15,
		MaxProviderCostCredits:       150,
		MaxInternalCostCredits:       450,
		PriceMultiplier:              3,
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

func copyVideoRoute(route VideoRoute) VideoRoute {
	route.Readiness = copyReadiness(route.Readiness)
	route.Spec = copyVideoRouteSpec(route.Spec)
	route.Limits = copyVideoLimits(route.Limits)
	route.PricingKeys = append([]pricingcatalog.ProductKey(nil), route.PricingKeys...)
	route.DisabledPricingKeys = append([]pricingcatalog.ProductKey(nil), route.DisabledPricingKeys...)
	return route
}

func copyVideoLimits(limits VideoLimits) VideoLimits {
	limits.AllowedReferenceImageCounts = append([]int(nil), limits.AllowedReferenceImageCounts...)
	limits.AllowedDurationsSec = append([]int(nil), limits.AllowedDurationsSec...)
	limits.AllowedResolutions = append([]string(nil), limits.AllowedResolutions...)
	limits.AllowedAspectRatios = append([]string(nil), limits.AllowedAspectRatios...)
	limits.ResolutionDurationsSec = copyResolutionDurations(limits.ResolutionDurationsSec)
	return limits
}

func copyVideoRouteSpec(spec domain.VideoRouteSpec) domain.VideoRouteSpec {
	if spec.ProviderCostMicrosByResolutionDuration != nil {
		prices := make(map[string]map[int]int64, len(spec.ProviderCostMicrosByResolutionDuration))
		for resolution, durations := range spec.ProviderCostMicrosByResolutionDuration {
			prices[resolution] = maps.Clone(durations)
		}
		spec.ProviderCostMicrosByResolutionDuration = prices
	}
	spec.AllowedReferenceImageCounts = append([]int(nil), spec.AllowedReferenceImageCounts...)
	spec.ProviderCostMicrosPerSecondByResolution = maps.Clone(spec.ProviderCostMicrosPerSecondByResolution)
	spec.ProviderCostMicrosPerSecondWithAudioByResolution = maps.Clone(spec.ProviderCostMicrosPerSecondWithAudioByResolution)
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
