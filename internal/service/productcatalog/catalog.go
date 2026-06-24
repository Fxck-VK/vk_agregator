// Package productcatalog builds public, product-level model choices for
// inbound surfaces. It is safe for Mini App and VK bot UI code: provider
// names, model codes, keys and private URLs never appear in exported items.
package productcatalog

import (
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/modelcatalog"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/videorouter"
)

const (
	TypeImage = "image"
	TypeVideo = "video"
)

// Config contains only server-owned readiness decisions and already sanitized
// video routes. The catalog does not fetch provider state and does not expose
// provider identifiers in public items.
type Config struct {
	ImageProviderReady map[domain.ProviderName]bool
	EnabledImageModels map[string]bool
	VideoRoutes        []videorouter.PublicRoute
	PricingCatalog     *pricingcatalog.Catalog
}

type Catalog struct {
	images []ImageModel
	videos []VideoRoute
	items  []Item
}

type ImageModel struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// EstimateCredits is a backend-computed display hint for catalog UI only.
	// Clients must call the estimate endpoint before paid submission.
	EstimateCredits        int64    `json:"estimate_credits,omitempty"`
	Enabled                bool     `json:"enabled"`
	QualityOptions         []string `json:"quality_options,omitempty"`
	DefaultQuality         string   `json:"default_quality,omitempty"`
	SupportsReferenceImage bool     `json:"supports_reference_image"`
	MaxReferenceImages     int      `json:"max_reference_images,omitempty"`
}

type VideoRoute struct {
	Type        string `json:"type"`
	Alias       string `json:"alias"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// EstimateCredits is a backend-computed display hint for catalog UI only.
	// Clients must call the estimate endpoint before paid submission.
	EstimateCredits        int64    `json:"estimate_credits,omitempty"`
	Enabled                bool     `json:"enabled"`
	AllowedDurationsSec    []int    `json:"allowed_durations_sec,omitempty"`
	AllowedResolutions     []string `json:"allowed_resolutions,omitempty"`
	AllowedAspectRatios    []string `json:"allowed_aspect_ratios,omitempty"`
	DefaultDurationSec     int      `json:"default_duration_sec,omitempty"`
	DefaultResolution      string   `json:"default_resolution,omitempty"`
	DefaultAspectRatio     string   `json:"default_aspect_ratio,omitempty"`
	RequiresStartImage     bool     `json:"requires_start_image"`
	SupportsReferenceImage bool     `json:"supports_reference_image"`
	MaxReferenceImages     int      `json:"max_reference_images,omitempty"`
}

type Item struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Alias       string `json:"alias,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// EstimateCredits is the only generation price hint exposed in the public
	// catalog. Provider cost, floors, multipliers and provider-native ids stay
	// out of this DTO.
	EstimateCredits        int64    `json:"estimate_credits,omitempty"`
	Enabled                bool     `json:"enabled"`
	QualityOptions         []string `json:"quality_options,omitempty"`
	DefaultQuality         string   `json:"default_quality,omitempty"`
	AllowedDurationsSec    []int    `json:"allowed_durations_sec,omitempty"`
	AllowedResolutions     []string `json:"allowed_resolutions,omitempty"`
	AllowedAspectRatios    []string `json:"allowed_aspect_ratios,omitempty"`
	DefaultDurationSec     int      `json:"default_duration_sec,omitempty"`
	DefaultResolution      string   `json:"default_resolution,omitempty"`
	DefaultAspectRatio     string   `json:"default_aspect_ratio,omitempty"`
	RequiresStartImage     bool     `json:"requires_start_image"`
	SupportsReferenceImage bool     `json:"supports_reference_image"`
	MaxReferenceImages     int      `json:"max_reference_images,omitempty"`
}

func New(cfg Config) *Catalog {
	images := imageModels(cfg)
	videos := videoRoutes(cfg.VideoRoutes, cfg.PricingCatalog)
	items := make([]Item, 0, len(images)+len(videos))
	for _, model := range images {
		items = append(items, itemFromImage(model))
	}
	for _, route := range videos {
		items = append(items, itemFromVideo(route))
	}
	return &Catalog{
		images: images,
		videos: videos,
		items:  items,
	}
}

func (c *Catalog) ImageModels() []ImageModel {
	if c == nil {
		return nil
	}
	out := make([]ImageModel, 0, len(c.images))
	for _, model := range c.images {
		out = append(out, copyImageModel(model))
	}
	return out
}

func (c *Catalog) VideoRoutes() []VideoRoute {
	if c == nil {
		return nil
	}
	out := make([]VideoRoute, 0, len(c.videos))
	for _, route := range c.videos {
		out = append(out, copyVideoRoute(route))
	}
	return out
}

func (c *Catalog) Items() []Item {
	if c == nil {
		return nil
	}
	out := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		out = append(out, copyItem(item))
	}
	return out
}

func imageModels(cfg Config) []ImageModel {
	if cfg.PricingCatalog == nil {
		return nil
	}
	models := modelcatalog.ListMiniAppModels(domain.OperationImageGenerate)
	out := make([]ImageModel, 0, len(models))
	for _, model := range models {
		if !cfg.EnabledImageModels[model.ModelID] {
			continue
		}
		if !cfg.ImageProviderReady[model.Provider] {
			continue
		}
		modelID := modelcatalog.MiniAppResponseModelID(model)
		if modelID == "" {
			continue
		}
		qualityOptions := pricedImageQualityOptions(cfg.PricingCatalog, modelID, imageQualityOptions(model.ModelID))
		defaultQuality := pricedImageDefaultQuality(cfg.PricingCatalog, modelID, imageDefaultQuality(model.ModelID), qualityOptions)
		estimateCredits, ok := displayImageEstimateCredits(cfg.PricingCatalog, modelID, defaultQuality)
		if !ok {
			continue
		}
		out = append(out, ImageModel{
			Type:                   TypeImage,
			ID:                     modelID,
			Name:                   model.ModelName,
			Description:            imageDescription(model.ModelID),
			EstimateCredits:        estimateCredits,
			Enabled:                true,
			QualityOptions:         qualityOptions,
			DefaultQuality:         defaultQuality,
			SupportsReferenceImage: model.SupportsReferenceImage,
			MaxReferenceImages:     model.MaxReferenceImages,
		})
	}
	return out
}

func videoRoutes(routes []videorouter.PublicRoute, pricingCatalog *pricingcatalog.Catalog) []VideoRoute {
	if pricingCatalog == nil {
		return nil
	}
	out := make([]VideoRoute, 0, len(routes))
	for _, route := range routes {
		defaultResolution := route.DefaultResolution
		pricedDurations := pricedVideoDurations(pricingCatalog, route, defaultResolution)
		defaultDuration := pricedVideoDefaultDuration(route.DefaultDurationSec, pricedDurations)
		estimateCredits, ok := displayVideoEstimateCredits(pricingCatalog, route.Alias, defaultResolution, defaultDuration)
		if !ok {
			continue
		}
		out = append(out, VideoRoute{
			Type:                   TypeVideo,
			Alias:                  string(route.Alias),
			Name:                   videoName(route.Alias),
			Description:            videoDescription(route.Alias),
			EstimateCredits:        estimateCredits,
			Enabled:                true,
			AllowedDurationsSec:    pricedDurations,
			AllowedResolutions:     append([]string(nil), route.AllowedResolutions...),
			AllowedAspectRatios:    append([]string(nil), route.AllowedAspectRatios...),
			DefaultDurationSec:     defaultDuration,
			DefaultResolution:      defaultResolution,
			DefaultAspectRatio:     route.DefaultAspectRatio,
			RequiresStartImage:     route.RequiresStartImage,
			SupportsReferenceImage: route.SupportsReferenceImage,
			MaxReferenceImages:     route.MaxReferenceImages,
		})
	}
	return out
}

func pricedImageQualityOptions(catalog *pricingcatalog.Catalog, modelID string, options []string) []string {
	out := make([]string, 0, len(options))
	for _, option := range options {
		if _, ok := displayImageEstimateCredits(catalog, modelID, option); ok {
			out = append(out, option)
		}
	}
	return out
}

func pricedImageDefaultQuality(catalog *pricingcatalog.Catalog, modelID, configured string, options []string) string {
	if configured != "" {
		if _, ok := displayImageEstimateCredits(catalog, modelID, configured); ok {
			return configured
		}
	}
	if len(options) > 0 {
		return options[0]
	}
	return ""
}

func displayImageEstimateCredits(catalog *pricingcatalog.Catalog, modelID, quality string) (int64, bool) {
	if catalog == nil {
		return 0, false
	}
	credits, err := catalog.DisplayEstimateCredits(pricingcatalog.ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: modelID,
		Quality:      quality,
	})
	return credits, err == nil && credits > 0
}

func pricedVideoDurations(catalog *pricingcatalog.Catalog, route videorouter.PublicRoute, defaultResolution string) []int {
	out := make([]int, 0, len(route.AllowedDurationsSec))
	for _, duration := range route.AllowedDurationsSec {
		if _, ok := displayVideoEstimateCredits(catalog, route.Alias, defaultResolution, duration); ok {
			out = append(out, duration)
		}
	}
	return out
}

func pricedVideoDefaultDuration(configured int, durations []int) int {
	for _, duration := range durations {
		if duration == configured {
			return configured
		}
	}
	if len(durations) > 0 {
		return durations[0]
	}
	return 0
}

func displayVideoEstimateCredits(catalog *pricingcatalog.Catalog, alias domain.VideoRouteAlias, resolution string, duration int) (int64, bool) {
	if catalog == nil || duration <= 0 {
		return 0, false
	}
	credits, err := catalog.DisplayEstimateCredits(pricingcatalog.ProductKey{
		Operation:       domain.OperationVideoGenerate,
		Modality:        domain.ModalityVideo,
		VideoRouteAlias: alias,
		Resolution:      resolution,
		DurationSec:     duration,
	})
	return credits, err == nil && credits > 0
}

func imageDescription(modelID string) string {
	if modelID == modelcatalog.MiniAppImageMock {
		return "Synthetic image route for load tests without paid provider calls."
	}
	switch modelID {
	case modelcatalog.MiniAppImageNanoBanana2:
		return "Быстрая генерация и редактирование изображений с референсами."
	case modelcatalog.MiniAppImageNanoBananaPro:
		return "Премиальная генерация изображений с сильной детализацией и референсами."
	case modelcatalog.MiniAppImageGPTImage2:
		return "Качественная генерация и редактирование изображений с надежной композицией."
	case modelcatalog.MiniAppImageSeedream45:
		return "Быстрая эстетичная генерация изображений для концептов и визуалов."
	case modelcatalog.MiniAppImageSDXLTurbo:
		return "Быстрая недорогая генерация простых визуальных идей."
	default:
		return "Генерация изображений."
	}
}

func imageQualityOptions(modelID string) []string {
	switch modelID {
	case modelcatalog.MiniAppImageNanoBanana2,
		modelcatalog.MiniAppImageNanoBananaPro,
		modelcatalog.MiniAppImageGPTImage2:
		return []string{modelcatalog.ImageQuality1K, modelcatalog.ImageQuality2K, modelcatalog.ImageQuality4K}
	default:
		return nil
	}
}

func imageDefaultQuality(modelID string) string {
	options := imageQualityOptions(modelID)
	if len(options) == 0 {
		return ""
	}
	return options[0]
}

func videoName(alias domain.VideoRouteAlias) string {
	switch alias {
	case domain.VideoRouteHailuo23Fast:
		return "Hailuo 2.3 Fast"
	case domain.VideoRouteHailuo23Standard:
		return "Hailuo 2.3 Standard"
	case domain.VideoRouteKlingO3Standard:
		return "Kling O3 Standard"
	case domain.VideoRouteRunwayGen4Turbo:
		return "Runway Gen-4 Turbo"
	case domain.VideoRouteSeedance20Fast:
		return "Seedance 2.0 Fast"
	case domain.VideoRouteRunwayGen45:
		return "Runway Gen-4.5"
	case domain.VideoRouteMockTextToVideo:
		return "Mock Video Loadtest"
	default:
		return "Видео"
	}
}

func videoDescription(alias domain.VideoRouteAlias) string {
	switch alias {
	case domain.VideoRouteHailuo23Fast:
		return "Быстрое image-to-video. Требуется стартовое изображение."
	case domain.VideoRouteHailuo23Standard:
		return "Универсальная генерация видео по тексту или изображению."
	case domain.VideoRouteKlingO3Standard:
		return "Стабильная mid-модель для видео без аудио."
	case domain.VideoRouteRunwayGen4Turbo:
		return "Официальный творческий маршрут для image-to-video."
	case domain.VideoRouteSeedance20Fast:
		return "Быстрый reference-driven маршрут для связного видео."
	case domain.VideoRouteRunwayGen45:
		return "Премиальный маршрут для кинематографичных видео."
	default:
		return "Генерация видео."
	}
}

func itemFromImage(model ImageModel) Item {
	return Item{
		Type:                   TypeImage,
		ID:                     model.ID,
		Name:                   model.Name,
		Description:            model.Description,
		EstimateCredits:        model.EstimateCredits,
		Enabled:                model.Enabled,
		QualityOptions:         append([]string(nil), model.QualityOptions...),
		DefaultQuality:         model.DefaultQuality,
		SupportsReferenceImage: model.SupportsReferenceImage,
		MaxReferenceImages:     model.MaxReferenceImages,
	}
}

func itemFromVideo(route VideoRoute) Item {
	return Item{
		Type:                   TypeVideo,
		ID:                     route.Alias,
		Alias:                  route.Alias,
		Name:                   route.Name,
		Description:            route.Description,
		EstimateCredits:        route.EstimateCredits,
		Enabled:                route.Enabled,
		AllowedDurationsSec:    append([]int(nil), route.AllowedDurationsSec...),
		AllowedResolutions:     append([]string(nil), route.AllowedResolutions...),
		AllowedAspectRatios:    append([]string(nil), route.AllowedAspectRatios...),
		DefaultDurationSec:     route.DefaultDurationSec,
		DefaultResolution:      route.DefaultResolution,
		DefaultAspectRatio:     route.DefaultAspectRatio,
		RequiresStartImage:     route.RequiresStartImage,
		SupportsReferenceImage: route.SupportsReferenceImage,
		MaxReferenceImages:     route.MaxReferenceImages,
	}
}

func copyImageModel(model ImageModel) ImageModel {
	model.QualityOptions = append([]string(nil), model.QualityOptions...)
	return model
}

func copyVideoRoute(route VideoRoute) VideoRoute {
	route.AllowedDurationsSec = append([]int(nil), route.AllowedDurationsSec...)
	route.AllowedResolutions = append([]string(nil), route.AllowedResolutions...)
	route.AllowedAspectRatios = append([]string(nil), route.AllowedAspectRatios...)
	return route
}

func copyItem(item Item) Item {
	item.QualityOptions = append([]string(nil), item.QualityOptions...)
	item.AllowedDurationsSec = append([]int(nil), item.AllowedDurationsSec...)
	item.AllowedResolutions = append([]string(nil), item.AllowedResolutions...)
	item.AllowedAspectRatios = append([]string(nil), item.AllowedAspectRatios...)
	return item
}
