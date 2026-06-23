package videorouter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

const (
	ParamVideoRouteAlias = "video_route_alias"
)

var (
	ErrUnknownRoute              = errors.New("video route unknown")
	ErrRouteDisabled             = errors.New("video route disabled")
	ErrProviderDisabled          = errors.New("video route provider disabled")
	ErrProviderUnconfigured      = errors.New("video route provider unconfigured")
	ErrUnsupportedDuration       = errors.New("video route unsupported duration")
	ErrUnsupportedResolution     = errors.New("video route unsupported resolution")
	ErrUnsupportedAspectRatio    = errors.New("video route unsupported aspect ratio")
	ErrMissingStartImage         = errors.New("video route requires start image")
	ErrTooManyReferenceImages    = errors.New("video route has too many reference images")
	ErrProviderModelIDNotAllowed = errors.New("provider model id is not accepted from client params")
	ErrRouteCostUnavailable      = errors.New("video route cost unavailable")
	ErrInvalidRouteRequest       = errors.New("video route request invalid")
)

type ProviderConfig struct {
	Enabled           bool
	RequireAPIKey     bool
	APIKeyConfigured  bool
	RequireBaseURL    bool
	BaseURLConfigured bool
}

type Config struct {
	RouterEnabled bool
	Providers     map[domain.ProviderName]ProviderConfig
	EnabledRoutes map[domain.VideoRouteAlias]bool
}

type Request struct {
	Source           string
	Operation        domain.OperationType
	Modality         domain.Modality
	Params           json.RawMessage
	InputArtifactIDs []uuid.UUID
}

type Resolution struct {
	Resolved            bool
	Params              json.RawMessage
	Snapshot            domain.VideoRouteSnapshot
	InternalCostCredits int64
}

type Route struct {
	Spec                   domain.VideoRouteSpec
	Enabled                bool
	ProviderEnabled        bool
	ProviderConfigured     bool
	ProviderBaseConfigured bool
}

type PublicRoute struct {
	Alias                  domain.VideoRouteAlias `json:"alias"`
	EstimateCredits        int64                  `json:"estimate_credits,omitempty"`
	AllowedDurationsSec    []int                  `json:"allowed_durations_sec,omitempty"`
	AllowedResolutions     []string               `json:"allowed_resolutions,omitempty"`
	AllowedAspectRatios    []string               `json:"allowed_aspect_ratios,omitempty"`
	DefaultDurationSec     int                    `json:"default_duration_sec,omitempty"`
	DefaultResolution      string                 `json:"default_resolution,omitempty"`
	DefaultAspectRatio     string                 `json:"default_aspect_ratio,omitempty"`
	RequiresStartImage     bool                   `json:"requires_start_image"`
	SupportsReferenceImage bool                   `json:"supports_reference_image"`
	MaxReferenceImages     int                    `json:"max_reference_images,omitempty"`
}

type Catalog struct {
	routerEnabled bool
	routes        map[domain.VideoRouteAlias]Route
	modelIDs      map[string]domain.VideoRouteAlias
}

func NewCatalog(cfg Config) (*Catalog, error) {
	routes := make(map[domain.VideoRouteAlias]Route)
	modelIDs := make(map[string]domain.VideoRouteAlias)
	for _, spec := range DefaultRouteSpecs() {
		if err := spec.Validate(); err != nil {
			return nil, err
		}
		if _, exists := routes[spec.Alias]; exists {
			return nil, fmt.Errorf("video route catalog: duplicate route alias %q", spec.Alias)
		}
		providerCfg := cfg.Providers[spec.Provider]
		route := Route{
			Spec:                   spec,
			Enabled:                cfg.EnabledRoutes[spec.Alias],
			ProviderEnabled:        providerCfg.Enabled,
			ProviderConfigured:     !providerCfg.RequireAPIKey || providerCfg.APIKeyConfigured,
			ProviderBaseConfigured: !providerCfg.RequireBaseURL || providerCfg.BaseURLConfigured,
		}
		routes[spec.Alias] = route
		modelKey := normalize(spec.ProviderModelID)
		if existing, exists := modelIDs[modelKey]; exists {
			return nil, fmt.Errorf("video route catalog: duplicate provider model id %q for %s and %s", spec.ProviderModelID, existing, spec.Alias)
		}
		modelIDs[modelKey] = spec.Alias
	}
	addProviderModelAlias(modelIDs, domain.VideoRouteKlingO3Standard, "kling-o3")
	addProviderModelAlias(modelIDs, domain.VideoRouteKlingO3Standard, "kling-o3-standard")
	addProviderModelAlias(modelIDs, domain.VideoRouteKlingO3Standard, "Kling O3 Standard")
	addProviderModelAlias(modelIDs, domain.VideoRouteSeedance20Fast, "seedance-2.0-fast")
	addProviderModelAlias(modelIDs, domain.VideoRouteRunwayGen4Turbo, "runway-gen-4-turbo")
	return &Catalog{
		routerEnabled: cfg.RouterEnabled,
		routes:        routes,
		modelIDs:      modelIDs,
	}, nil
}

func DefaultRouteSpecs() []domain.VideoRouteSpec {
	return []domain.VideoRouteSpec{
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
	}
}

func (c *Catalog) PublicRoutes() []PublicRoute {
	if c == nil || !c.routerEnabled {
		return nil
	}
	out := make([]PublicRoute, 0, len(c.routes))
	for _, spec := range DefaultRouteSpecs() {
		route, ok := c.routes[spec.Alias]
		if !ok || !route.publiclyAvailable() {
			continue
		}
		out = append(out, PublicRoute{
			Alias:                  route.Spec.Alias,
			EstimateCredits:        defaultEstimateCredits(route.Spec),
			AllowedDurationsSec:    append([]int(nil), route.Spec.AllowedDurationsSec...),
			AllowedResolutions:     append([]string(nil), route.Spec.AllowedResolutions...),
			AllowedAspectRatios:    append([]string(nil), route.Spec.AllowedAspectRatios...),
			DefaultDurationSec:     defaultDuration(route.Spec),
			DefaultResolution:      defaultResolution(route.Spec),
			DefaultAspectRatio:     defaultAspectRatio(route.Spec),
			RequiresStartImage:     route.Spec.RequiresStartImage,
			SupportsReferenceImage: route.Spec.SupportsReferenceImage,
			MaxReferenceImages:     route.Spec.MaxReferenceImages,
		})
	}
	return out
}

func (r Route) publiclyAvailable() bool {
	return r.Enabled &&
		r.ProviderEnabled &&
		r.ProviderConfigured &&
		r.ProviderBaseConfigured &&
		(r.Spec.ProviderCostCreditsFixed > 0 || r.Spec.ProviderCostCreditsPerSecond > 0)
}

func defaultEstimateCredits(spec domain.VideoRouteSpec) int64 {
	duration := defaultDuration(spec)
	if duration <= 0 {
		return 0
	}
	providerCost := spec.ProviderCostCreditsFixed + spec.ProviderCostCreditsPerSecond*int64(duration)
	if providerCost <= 0 {
		return 0
	}
	internalCost := int64(math.Ceil(float64(providerCost) * spec.PriceMultiplier))
	if internalCost <= 0 {
		return 0
	}
	if spec.MaxInternalCostCredits > 0 && internalCost > spec.MaxInternalCostCredits {
		return spec.MaxInternalCostCredits
	}
	return internalCost
}

func (c *Catalog) Validate(ctx context.Context, req Request) error {
	_, err := c.Resolve(ctx, req)
	return err
}

func (c *Catalog) Resolve(ctx context.Context, req Request) (Resolution, error) {
	_ = ctx
	if c == nil || !isVideoRequest(req.Operation, req.Modality) {
		return Resolution{Params: req.Params}, nil
	}
	params, err := parseParams(req.Params)
	if err != nil {
		return Resolution{}, err
	}
	routeAlias := domain.VideoRouteAlias(strings.TrimSpace(params.routeAlias()))
	if hasReservedBillingSelection(req.Params) {
		return Resolution{}, ErrProviderModelIDNotAllowed
	}
	if routeAlias == "" {
		if c.hasProviderNativeSelection(params, false) {
			return Resolution{}, ErrProviderModelIDNotAllowed
		}
		return Resolution{Params: req.Params}, nil
	}
	if c.hasProviderNativeSelection(params, true) {
		return Resolution{}, ErrProviderModelIDNotAllowed
	}
	if !c.routerEnabled {
		return Resolution{}, fmt.Errorf("%w: %s", ErrRouteDisabled, routeAlias)
	}
	route, ok := c.routes[routeAlias]
	if !ok {
		return Resolution{}, fmt.Errorf("%w: %s", ErrUnknownRoute, routeAlias)
	}
	if !route.Enabled {
		return Resolution{}, fmt.Errorf("%w: %s", ErrRouteDisabled, routeAlias)
	}
	if !route.ProviderEnabled {
		return Resolution{}, fmt.Errorf("%w: %s", ErrProviderDisabled, route.Spec.Provider)
	}
	if !route.ProviderConfigured || !route.ProviderBaseConfigured {
		return Resolution{}, fmt.Errorf("%w: %s", ErrProviderUnconfigured, route.Spec.Provider)
	}
	durationSec := params.DurationSec
	if durationSec == 0 {
		durationSec = defaultDuration(route.Spec)
	}
	if durationSec == 0 || !supportsDuration(route.Spec, durationSec) {
		return Resolution{}, fmt.Errorf("%w: %s duration %d", ErrUnsupportedDuration, routeAlias, durationSec)
	}
	resolutionRaw := strings.TrimSpace(params.Resolution)
	if resolutionRaw == "" {
		resolutionRaw = defaultResolution(route.Spec)
	}
	if resolutionRaw != "" {
		resolution := normalize(resolutionRaw)
		if !containsNormalized(route.Spec.AllowedResolutions, resolution) {
			return Resolution{}, fmt.Errorf("%w: %s resolution %s", ErrUnsupportedResolution, routeAlias, resolutionRaw)
		}
		if len(route.Spec.ResolutionDurationsSec) > 0 && !supportsResolutionDuration(route.Spec, resolution, durationSec) {
			return Resolution{}, fmt.Errorf("%w: %s resolution %s duration %d", ErrUnsupportedDuration, routeAlias, resolutionRaw, durationSec)
		}
	}
	aspectRatio := strings.TrimSpace(params.AspectRatio)
	if aspectRatio == "" {
		aspectRatio = defaultAspectRatio(route.Spec)
	}
	if aspectRatio != "" && !containsNormalized(route.Spec.AllowedAspectRatios, normalize(aspectRatio)) {
		return Resolution{}, fmt.Errorf("%w: %s aspect %s", ErrUnsupportedAspectRatio, routeAlias, aspectRatio)
	}
	imageReferenceCount := imageReferenceCount(req, params)
	if imageReferenceCount > 0 && !route.Spec.SupportsReferenceImage {
		return Resolution{}, fmt.Errorf("%w: %s image reference", ErrInvalidRouteRequest, routeAlias)
	}
	if route.Spec.MaxReferenceImages > 0 && imageReferenceCount > route.Spec.MaxReferenceImages {
		return Resolution{}, fmt.Errorf("%w: %s images %d exceeds cap %d", ErrTooManyReferenceImages, routeAlias, imageReferenceCount, route.Spec.MaxReferenceImages)
	}
	if route.Spec.RequiresStartImage && imageReferenceCount == 0 {
		return Resolution{}, fmt.Errorf("%w: %s", ErrMissingStartImage, routeAlias)
	}
	if hasVideoReference(params) && !route.Spec.SupportsReferenceVideo {
		return Resolution{}, fmt.Errorf("%w: %s video reference", ErrInvalidRouteRequest, routeAlias)
	}
	if hasAudioReference(params) && !route.Spec.SupportsReferenceAudio {
		return Resolution{}, fmt.Errorf("%w: %s audio reference", ErrInvalidRouteRequest, routeAlias)
	}
	providerCost := route.Spec.ProviderCostCreditsFixed + route.Spec.ProviderCostCreditsPerSecond*int64(durationSec)
	if providerCost <= 0 {
		return Resolution{}, fmt.Errorf("%w: %s", ErrRouteCostUnavailable, routeAlias)
	}
	if route.Spec.MaxProviderCostCredits > 0 && providerCost > route.Spec.MaxProviderCostCredits {
		return Resolution{}, fmt.Errorf("%w: provider cost %d exceeds route cap %d", domain.ErrCostCapExceeded, providerCost, route.Spec.MaxProviderCostCredits)
	}
	internalCost := int64(math.Ceil(float64(providerCost) * route.Spec.PriceMultiplier))
	if internalCost <= 0 {
		return Resolution{}, fmt.Errorf("%w: %s", ErrRouteCostUnavailable, routeAlias)
	}
	if route.Spec.MaxInternalCostCredits > 0 && internalCost > route.Spec.MaxInternalCostCredits {
		return Resolution{}, fmt.Errorf("%w: internal cost %d exceeds route cap %d", domain.ErrCostCapExceeded, internalCost, route.Spec.MaxInternalCostCredits)
	}
	snapshot := domain.VideoRouteSnapshot{
		Alias:                  route.Spec.Alias,
		Provider:               route.Spec.Provider,
		ProviderModelID:        route.Spec.ProviderModelID,
		ModelClass:             route.Spec.ModelClass,
		DurationSec:            durationSec,
		Resolution:             resolutionRaw,
		AspectRatio:            aspectRatio,
		ProviderCostCredits:    providerCost,
		InternalCostCredits:    internalCost,
		PriceMultiplier:        route.Spec.PriceMultiplier,
		MaxProviderCostCredits: route.Spec.MaxProviderCostCredits,
		MaxInternalCostCredits: route.Spec.MaxInternalCostCredits,
	}
	resolvedParams, err := withResolvedSnapshot(req.Params, snapshot)
	if err != nil {
		return Resolution{}, err
	}
	return Resolution{
		Resolved:            true,
		Params:              resolvedParams,
		Snapshot:            snapshot,
		InternalCostCredits: internalCost,
	}, nil
}

type requestParams struct {
	VideoRouteAlias      string      `json:"video_route_alias"`
	RouteAlias           string      `json:"route_alias"`
	ModelID              string      `json:"model_id"`
	Provider             string      `json:"provider"`
	ModelCode            string      `json:"model_code"`
	DurationSec          int         `json:"duration_sec"`
	Resolution           string      `json:"resolution"`
	AspectRatio          string      `json:"aspect_ratio"`
	ReferenceArtifactIDs []uuid.UUID `json:"reference_artifact_ids"`
	FirstFrameImage      string      `json:"first_frame_image"`
	InputURLs            []string    `json:"input_urls"`
	ReferenceVideoURL    string      `json:"reference_video_url"`
	ReferenceVideoURLs   []string    `json:"reference_video_urls"`
	ReferenceAudioURL    string      `json:"reference_audio_url"`
	ReferenceAudioURLs   []string    `json:"reference_audio_urls"`
}

func (p requestParams) routeAlias() string {
	if strings.TrimSpace(p.VideoRouteAlias) != "" {
		return p.VideoRouteAlias
	}
	return p.RouteAlias
}

func (c *Catalog) hasProviderNativeSelection(params requestParams, strict bool) bool {
	if strict && (strings.TrimSpace(params.Provider) != "" || strings.TrimSpace(params.ModelCode) != "") {
		return true
	}
	switch normalize(params.Provider) {
	case string(domain.ProviderAPIMart), string(domain.ProviderPoYo), string(domain.ProviderRunway):
		return true
	}
	if strings.TrimSpace(params.FirstFrameImage) != "" || len(params.InputURLs) > 0 {
		return true
	}
	if _, ok := c.modelIDs[normalize(params.ModelCode)]; ok {
		return true
	}
	if _, ok := c.modelIDs[normalize(params.ModelID)]; ok {
		return true
	}
	return false
}

func hasReservedBillingSelection(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return false
	}
	for _, key := range []string{
		"resolved_video_route",
		"provider_cost_credits",
		"internal_cost_credits",
		"price",
		"cost",
		"cost_estimate",
		"max_provider_cost_credits",
		"max_internal_cost_credits",
	} {
		if _, ok := values[key]; ok {
			return true
		}
	}
	return false
}

func parseParams(raw json.RawMessage) (requestParams, error) {
	if len(raw) == 0 {
		return requestParams{}, nil
	}
	var params requestParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return requestParams{}, fmt.Errorf("%w: params json: %v", ErrInvalidRouteRequest, err)
	}
	return params, nil
}

func isVideoRequest(op domain.OperationType, modality domain.Modality) bool {
	if modality == domain.ModalityVideo {
		return true
	}
	switch op {
	case domain.OperationVideoGenerate, domain.OperationVideoImageToVideo, domain.OperationVideoExtend:
		return true
	default:
		return false
	}
}

func hasVideoReference(params requestParams) bool {
	return strings.TrimSpace(params.ReferenceVideoURL) != "" || len(params.ReferenceVideoURLs) > 0
}

func hasAudioReference(params requestParams) bool {
	return strings.TrimSpace(params.ReferenceAudioURL) != "" || len(params.ReferenceAudioURLs) > 0
}

func imageReferenceCount(req Request, params requestParams) int {
	seen := make(map[uuid.UUID]struct{}, len(req.InputArtifactIDs)+len(params.ReferenceArtifactIDs))
	for _, id := range req.InputArtifactIDs {
		if id == uuid.Nil {
			continue
		}
		seen[id] = struct{}{}
	}
	for _, id := range params.ReferenceArtifactIDs {
		if id == uuid.Nil {
			continue
		}
		seen[id] = struct{}{}
	}
	return len(seen)
}

func supportsDuration(spec domain.VideoRouteSpec, duration int) bool {
	for _, allowed := range spec.AllowedDurationsSec {
		if duration == allowed {
			return true
		}
	}
	return false
}

func defaultDuration(spec domain.VideoRouteSpec) int {
	if len(spec.AllowedDurationsSec) == 0 {
		return 0
	}
	return spec.AllowedDurationsSec[0]
}

func defaultResolution(spec domain.VideoRouteSpec) string {
	if len(spec.AllowedResolutions) == 0 {
		return ""
	}
	return spec.AllowedResolutions[0]
}

func defaultAspectRatio(spec domain.VideoRouteSpec) string {
	if len(spec.AllowedAspectRatios) == 0 {
		return ""
	}
	return spec.AllowedAspectRatios[0]
}

func supportsResolutionDuration(spec domain.VideoRouteSpec, resolution string, duration int) bool {
	for candidate, durations := range spec.ResolutionDurationsSec {
		if normalize(candidate) != resolution {
			continue
		}
		for _, allowed := range durations {
			if duration == allowed {
				return true
			}
		}
		return false
	}
	return true
}

func containsNormalized(values []string, target string) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		if normalize(value) == target {
			return true
		}
	}
	return false
}

func addProviderModelAlias(modelIDs map[string]domain.VideoRouteAlias, alias domain.VideoRouteAlias, modelID string) {
	modelIDs[normalize(modelID)] = alias
}

func withResolvedSnapshot(raw json.RawMessage, snapshot domain.VideoRouteSnapshot) (json.RawMessage, error) {
	values := map[string]json.RawMessage{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil, fmt.Errorf("%w: params json: %v", ErrInvalidRouteRequest, err)
		}
	}
	for _, key := range []string{
		"provider",
		"model_code",
		"first_frame_image",
		"input_urls",
		"resolved_video_route",
		"provider_cost_credits",
		"internal_cost_credits",
		"price",
		"cost",
		"cost_estimate",
		"max_provider_cost_credits",
		"max_internal_cost_credits",
	} {
		delete(values, key)
	}
	setJSON(values, ParamVideoRouteAlias, snapshot.Alias)
	setJSON(values, "provider", snapshot.Provider)
	setJSON(values, "model_code", snapshot.ProviderModelID)
	setJSON(values, "duration_sec", snapshot.DurationSec)
	if strings.TrimSpace(snapshot.Resolution) != "" {
		setJSON(values, "resolution", snapshot.Resolution)
	}
	if strings.TrimSpace(snapshot.AspectRatio) != "" {
		setJSON(values, "aspect_ratio", snapshot.AspectRatio)
	}
	setJSON(values, "resolved_video_route", snapshot)
	out, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("%w: resolved params json: %v", ErrInvalidRouteRequest, err)
	}
	return out, nil
}

func setJSON(values map[string]json.RawMessage, key string, value any) {
	raw, err := json.Marshal(value)
	if err != nil {
		return
	}
	values[key] = raw
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
