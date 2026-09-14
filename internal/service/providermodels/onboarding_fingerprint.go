package providermodels

import (
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

// admissionLimitsV1 preserves the original fingerprint encoding. Runtime limits
// are purpose-specific; removing unrelated zero fields from Go structs must not
// invalidate unchanged admissions. Keep field order and nil slices stable.
type admissionLimitsV1 struct {
	SupportsAudio               bool
	RequiresReferenceVideo      bool
	AutomaticDuration           bool
	AllowedReferenceImageCounts []int
	AllowedQualities            []string
	AllowedDurationsSec         []int
	AllowedResolutions          []string
	AllowedAspectRatios         []string
	ResolutionDurationsSec      map[string][]int
	SupportsReferenceImage      bool
	RequiresStartImage          bool
	MaxReferenceImages          int
	MaxOutputCount              int
}

func imageAdmissionValue(m ImageModel) any {
	return struct {
		PublicID        string
		DisplayName     string
		Provider        domain.ProviderName
		ProviderModelID string
		FeatureFlag     string
		Readiness       ProviderReadiness
		Limits          admissionLimitsV1
		PricingKeys     []pricingcatalog.ProductKey
		LoadTestOnly    bool
	}{m.PublicID, m.DisplayName, m.Provider, m.ProviderModelID, m.FeatureFlag, m.Readiness,
		admissionLimitsV1{
			AllowedQualities: m.Limits.AllowedQualities, AllowedAspectRatios: m.Limits.AllowedAspectRatios,
			SupportsReferenceImage: m.Limits.SupportsReferenceImage,
			MaxReferenceImages:     m.Limits.MaxReferenceImages, MaxOutputCount: m.Limits.MaxOutputCount,
		}, m.PricingKeys, m.LoadTestOnly}
}

func videoAdmissionValue(m VideoRoute, aliases []ProviderModelAlias) any {
	type routeV1 struct {
		Alias               domain.VideoRouteAlias
		Provider            domain.ProviderName
		ProviderModelID     string
		ModelClass          string
		FeatureFlag         string
		RouterFeatureFlag   string
		Readiness           ProviderReadiness
		Spec                domain.VideoRouteSpec
		Limits              admissionLimitsV1
		MediaContract       MediaContractClass
		PricingKeys         []pricingcatalog.ProductKey
		DisabledPricingKeys []pricingcatalog.ProductKey
		LoadTestOnly        bool
	}
	return struct {
		Route   routeV1
		Aliases []ProviderModelAlias
	}{routeV1{m.Alias, m.Provider, m.ProviderModelID, m.ModelClass, m.FeatureFlag, m.RouterFeatureFlag,
		m.Readiness, m.Spec, admissionLimitsV1{
			SupportsAudio: m.Limits.SupportsAudio, RequiresReferenceVideo: m.Limits.RequiresReferenceVideo,
			AutomaticDuration: m.Limits.AutomaticDuration, AllowedReferenceImageCounts: m.Limits.AllowedReferenceImageCounts,
			AllowedDurationsSec: m.Limits.AllowedDurationsSec, AllowedResolutions: m.Limits.AllowedResolutions,
			AllowedAspectRatios: m.Limits.AllowedAspectRatios, ResolutionDurationsSec: m.Limits.ResolutionDurationsSec,
			SupportsReferenceImage: m.Limits.SupportsReferenceImage, RequiresStartImage: m.Limits.RequiresStartImage,
			MaxReferenceImages: m.Limits.MaxReferenceImages,
		}, m.MediaContract, m.PricingKeys, m.DisabledPricingKeys, m.LoadTestOnly}, aliases}
}
