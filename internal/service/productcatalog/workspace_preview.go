package productcatalog

import (
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/textgeneration"
	"vk-ai-aggregator/internal/service/videorouter"
)

// WorkspacePreviewCatalog is an offline fixture, not production readiness or
// provider verification. It uses the actual registry, resolver and static prices
// without reading environment variables, secrets or calling a provider.
func WorkspacePreviewCatalog() (WorkspaceModelList, error) {
	r := providermodels.StaticRegistry()
	if err := r.Validate(); err != nil {
		return WorkspaceModelList{}, err
	}
	prices, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		return WorkspaceModelList{}, err
	}
	providers := map[domain.ProviderName]videorouter.ProviderConfig{}
	enabledRoutes := map[domain.VideoRouteAlias]bool{}
	for _, route := range r.VideoRoutes() {
		if route.LoadTestOnly {
			continue
		}
		enabledRoutes[route.Alias] = true
		providers[route.Provider] = videorouter.ProviderConfig{Enabled: true, APIKeyConfigured: true, BaseURLConfigured: true}
	}
	video, err := videorouter.NewCatalog(videorouter.Config{RouterEnabled: true, Providers: providers, EnabledRoutes: enabledRoutes})
	if err != nil {
		return WorkspaceModelList{}, err
	}
	cfg := Config{ImageProviderReady: map[domain.ProviderName]bool{}, EnabledImageModels: map[string]bool{}, VideoRoutes: video.PublicRoutes(), PricingCatalog: prices}
	for _, model := range r.PublicImageModels() {
		cfg.ImageProviderReady[model.Provider], cfg.EnabledImageModels[model.PublicID] = true, true
	}
	catalog := New(cfg)
	images := []imagegeneration.PublicModel{}
	for _, m := range catalog.ImageModels() {
		images = append(images, imagegeneration.PublicModel{ID: m.ID, Name: m.Name, Enabled: true, Ready: true, QualityOptions: m.QualityOptions, DefaultQuality: m.DefaultQuality, SupportsReferenceImage: m.SupportsReferenceImage, MaxReferenceImages: m.MaxReferenceImages, MaxOutputCount: m.MaxOutputCount, AllowedAspectRatios: m.AllowedAspectRatios})
	}
	textIDs := []string{}
	for _, m := range r.TextAliasModels() {
		if m.PublicID != providermodels.PublicTextChatGPT {
			textIDs = append(textIDs, m.PublicID)
		}
	}
	return WorkspaceCatalog(WorkspaceConfig{ImageReferenceUploads: true, ImageModels: images, TextModels: textgeneration.Models(textIDs, prices), VideoRoutes: catalog.VideoRoutes(), Pricing: prices, IncludePendingMedia: true}), nil
}
