package providermodels

import (
	"fmt"
	"strings"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

// AudioLimits describes public audio request/media bounds for one model.
// It is intentionally empty in StaticRegistry until audio products are priced
// and wired through the same fail-closed provider/job path as other media.
type AudioLimits struct {
	SupportsAudioInput   bool
	SupportsVideoInput   bool
	AllowedInputFormats  []string
	AllowedOutputFormats []string
	AllowedQualities     []string
	MaxInputAudioFiles   int
	MaxInputVideoFiles   int
}

// AudioModel is the typed metadata skeleton for future public audio models.
type AudioModel struct {
	PublicID        string
	DisplayName     string
	Provider        domain.ProviderName
	ProviderModelID string
	FeatureFlag     string
	Readiness       ProviderReadiness
	Limits          AudioLimits
	PricingKeys     []pricingcatalog.ProductKey
	LoadTestOnly    bool
}

func audioModels() []AudioModel {
	return []AudioModel{}
}

func (r Registry) PublicAudioModels() []AudioModel {
	return copyAudioModels(r.AudioModels)
}

func (r Registry) AudioModel(publicID string) (AudioModel, bool) {
	for _, model := range r.AudioModels {
		if model.PublicID == publicID {
			return copyAudioModel(model), true
		}
	}
	return AudioModel{}, false
}

func validateAudioModel(model AudioModel, loadTestOnly bool) error {
	if strings.TrimSpace(model.PublicID) == "" {
		return fmt.Errorf("providermodels: audio public id is required")
	}
	if model.Provider == "" || strings.TrimSpace(model.ProviderModelID) == "" {
		return fmt.Errorf("providermodels: audio %s missing provider metadata", model.PublicID)
	}
	if strings.TrimSpace(model.FeatureFlag) == "" {
		return fmt.Errorf("providermodels: audio %s missing feature flag", model.PublicID)
	}
	if !loadTestOnly && len(model.PricingKeys) == 0 {
		return fmt.Errorf("providermodels: audio %s missing pricing keys", model.PublicID)
	}
	return nil
}

func copyAudioModels(in []AudioModel) []AudioModel {
	out := make([]AudioModel, 0, len(in))
	for _, model := range in {
		out = append(out, copyAudioModel(model))
	}
	return out
}

func copyAudioModel(model AudioModel) AudioModel {
	model.Readiness = copyReadiness(model.Readiness)
	model.Limits = copyAudioLimits(model.Limits)
	model.PricingKeys = append([]pricingcatalog.ProductKey(nil), model.PricingKeys...)
	return model
}

func copyAudioLimits(limits AudioLimits) AudioLimits {
	limits.AllowedInputFormats = append([]string(nil), limits.AllowedInputFormats...)
	limits.AllowedOutputFormats = append([]string(nil), limits.AllowedOutputFormats...)
	limits.AllowedQualities = append([]string(nil), limits.AllowedQualities...)
	return limits
}
