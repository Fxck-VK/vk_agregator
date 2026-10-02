package providermodels

import (
	"fmt"
	"sync/atomic"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

const FeatureDEVModelSmoke = "FEATURE_DEV_MODEL_SMOKE_ENABLED"

var devSmokeRegistry atomic.Pointer[Registry]

// ConfigureDEVSmoke is called once by API/worker bootstrap, after config validation.
// It authorizes manual development execution, never verified model admission.
func ConfigureDEVSmoke(environment string, enabled bool) error {
	if enabled && environment != "development" {
		devSmokeRegistry.Store(nil)
		return fmt.Errorf("model smoke is only allowed in development")
	}
	if enabled {
		r := buildDEVSmokeRegistry()
		if err := r.Validate(); err != nil {
			return err
		}
		devSmokeRegistry.Store(&r)
	} else {
		devSmokeRegistry.Store(nil)
	}
	return nil
}

// RuntimeRegistry adds only priced, wired candidates for the explicitly
// enabled DEV contour. StaticRegistry and its frozen admission baseline stay strict.
func RuntimeRegistry() Registry {
	if configured := devSmokeRegistry.Load(); configured != nil {
		r := *configured
		r.TextAliases = configured.TextAliasModels()
		r.ImageModels = append([]ImageModel(nil), configured.ImageModels...)
		for i := range r.ImageModels {
			r.ImageModels[i].Limits = copyImageLimits(r.ImageModels[i].Limits)
			r.ImageModels[i].Readiness = copyReadiness(r.ImageModels[i].Readiness)
			r.ImageModels[i].PricingKeys = append([]pricingcatalog.ProductKey(nil), r.ImageModels[i].PricingKeys...)
		}
		r.VideoRouteModels = configured.VideoRoutes()
		return r
	}
	return StaticRegistry()
}

func buildDEVSmokeRegistry() Registry {
	r := StaticRegistry()
	r.devSmoke = true
	r.smokeFingerprints = map[string]string{}
	for _, c := range TextCandidates() {
		q, err := pricingcatalog.TextCandidateQuote(c.PublicID)
		if err != nil {
			continue
		}
		r.TextAliases = append(r.TextAliases, c.Alias())
		r.smokePrices = append(r.smokePrices, smokePrice(q))
	}
	for _, c := range MediaCandidates() {
		switch c.Kind {
		case "image":
			qualities := []string{}
			for _, quality := range ImageCandidateQualities(c) {
				q, err := pricingcatalog.ImageCandidateQualityQuote(c.PublicID, quality)
				if err != nil {
					continue
				}
				qualities = append(qualities, quality)
				r.smokePrices = append(r.smokePrices, smokePrice(q))
			}
			if len(qualities) == 0 {
				continue
			}
			m := imageModelWithQualities(c.PublicID, c.Name, c.Provider, c.ModelCode, FeatureDEVModelSmoke, apimartReadiness(), qualities, 0)
			m.Limits.MaxOutputCount = 1
			m.Limits.SupportsReferenceImage = false
			m.Limits.AllowedAspectRatios = append([]string(nil), c.Capabilities.Application.Image.AspectRatios...)
			r.ImageModels = append(r.ImageModels, m)
		case "video":
			v := c.Capabilities.API.Video
			durations := append([]int(nil), v.Duration.AllowedSeconds...)
			if len(durations) == 0 && v.Duration.MinSeconds != nil && v.Duration.MaxSeconds != nil {
				durations = integerRange(*v.Duration.MinSeconds, *v.Duration.MaxSeconds)
			}
			spec := domain.VideoRouteSpec{Alias: domain.VideoRouteAlias(c.PublicID), Provider: c.Provider, ProviderModelID: c.ModelCode, ModelClass: c.PublicID, InputModes: []domain.VideoInputMode{domain.VideoInputText}, AllowedDurationsSec: durations, AllowedResolutions: append([]string(nil), v.Resolutions...), AllowedAspectRatios: append([]string(nil), v.AspectRatios...), PriceMultiplier: 3, ProviderCostMicrosByResolutionDuration: map[string]map[int]int64{}}
			if IsVideoExpansion(c.PublicID) {
				resolution, seconds := VideoCandidateDefaults(c.PublicID)
				spec.AllowedResolutions = candidateDefaultFirst(spec.AllowedResolutions, resolution)
				durations = candidateDefaultFirst(VideoCandidateDurations(c), seconds)
				spec.AllowedDurationsSec = durations
				spec.AutomaticDuration = v.Duration.Mode == videoDurationAutomatic
				if VideoCandidateRequiresImages(c.PublicID) {
					spec.InputModes = []domain.VideoInputMode{domain.VideoInputReference}
					spec.SupportsReferenceImage = true
					spec.MaxReferenceImages = 7
					spec.AllowedReferenceImageCounts = intSequence(1, 7)
				}
			}
			var keys []pricingcatalog.ProductKey
			for _, res := range spec.AllowedResolutions {
				spec.ProviderCostMicrosByResolutionDuration[res] = map[int]int64{}
				for _, seconds := range durations {
					q, err := pricingcatalog.MediaVideoCandidateQuote(c.PublicID, "", res, seconds)
					if err != nil {
						continue
					}
					// One APIMart provider credit = $0.10. Route safety uses provider-credit micros.
					spec.ProviderCostMicrosByResolutionDuration[res][seconds] = q.Floor.Amount * 10
					spec.MaxProviderCostCredits = max(spec.MaxProviderCostCredits, (q.Floor.Amount+99999)/100000)
					spec.MaxInternalCostCredits = max(spec.MaxInternalCostCredits, q.InternalCredits)
					keys = append(keys, q.Key)
					r.smokePrices = append(r.smokePrices, smokePrice(q))
				}
			}
			// An API contract alone is not a billable route. In particular, never
			// publish a new candidate with a zero or unrelated model's price.
			if len(keys) == 0 {
				continue
			}
			r.VideoRouteModels = append(r.VideoRouteModels, videoRoute(spec, FeatureDEVModelSmoke, apimartReadiness(), keys, nil, false))
		}
	}
	for _, b := range r.Bindings() {
		if _, ok := TextCandidateByID(b.PublicID); ok {
			r.smokeFingerprints[b.PublicID] = b.Fingerprint
		}
		if _, ok := MediaCandidateByID(b.PublicID); ok {
			r.smokeFingerprints[b.PublicID] = b.Fingerprint
		}
	}
	return r
}

func smokePrice(q pricingcatalog.PricingSnapshot) pricingcatalog.ProductPrice {
	return pricingcatalog.ProductPrice{Key: q.Key, Version: q.Version, Source: q.Source, Floor: q.Floor, Multiplier: q.Multiplier, UnitConversion: q.UnitConversion, Caps: pricingcatalog.SafetyCaps{InternalCreditCap: q.InternalCreditCap, FloorAmountCap: q.FloorAmountCap}, Enabled: true, DefaultDisplayCredits: q.DefaultDisplayCredits}
}

func (r Registry) DEVSmokePrices() []pricingcatalog.ProductPrice {
	return append([]pricingcatalog.ProductPrice(nil), r.smokePrices...)
}

func (r Registry) IsDEVSmokeModel(id string) bool {
	if !r.devSmoke {
		return false
	}
	if _, ok := TextCandidateByID(id); ok {
		return r.smokeFingerprints[id] != ""
	}
	c, ok := MediaCandidateByID(id)
	if !ok {
		return false
	}
	if c.Kind == "audio" {
		_, err := pricingcatalog.MusicCandidateQuote(id, "generate", false)
		return err == nil
	}
	return r.smokeFingerprints[id] != ""
}

// MediaCandidateRunnable distinguishes manual DEV smoke from verified admission.
// Advanced input/transform operations and unmetered speech remain closed.
func (r Registry) MediaCandidateRunnable(id, operation string) bool {
	if r.MediaCandidateAdmitted(id, operation) {
		return true
	}
	if !r.IsDEVSmokeModel(id) {
		return false
	}
	c, ok := MediaCandidateByID(id)
	if !ok {
		return false
	}
	if c.Kind == "video" {
		if VideoCandidateRequiresImages(id) {
			return operation == "reference_image_to_video"
		}
		return operation == "text_to_video"
	}
	return operation == "generate"
}

func candidateDefaultFirst[T comparable](values []T, preferred T) []T {
	result := make([]T, 0, len(values))
	for _, value := range values {
		if value == preferred {
			result = append(result, value)
		}
	}
	for _, value := range values {
		if value != preferred {
			result = append(result, value)
		}
	}
	return result
}
