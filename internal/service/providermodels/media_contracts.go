package providermodels

import "vk-ai-aggregator/internal/domain"

const defaultExpectedMaxVideoBytes int64 = 256 << 20

// MediaContractRuntime carries worker runtime policy that must not be frozen in
// the static model registry.
type MediaContractRuntime struct {
	ExpectedMaxVideoBytes int64
	RequireVideoProbe     bool
	VideoTranscodeAllowed bool
}

// ProviderMediaContracts builds worker media contracts from registry video
// routes plus runtime media policy. It performs no provider calls.
func (r Registry) ProviderMediaContracts(runtime MediaContractRuntime) []domain.ProviderMediaContract {
	maxBytes := runtime.ExpectedMaxVideoBytes
	if maxBytes <= 0 {
		maxBytes = defaultExpectedMaxVideoBytes
	}
	routes := r.VideoRoutes()
	contracts := make([]domain.ProviderMediaContract, 0, len(routes))
	for _, route := range routes {
		durations := append([]int(nil), route.Spec.AllowedDurationsSec...)
		if route.Spec.AutomaticDuration {
			// The request duration selects a fixed tariff; Omni chooses the output length.
			durations = []int{3, 4, 5, 6, 7, 8, 9, 10}
		}
		contracts = append(contracts, domain.ProviderMediaContract{
			Provider:                     route.Provider,
			Model:                        route.ProviderModelID,
			ModelClass:                   route.ModelClass,
			Modality:                     route.MediaContract.Modality,
			AllowedDurationsSec:          durations,
			AllowedAspectRatios:          append([]string(nil), route.Spec.AllowedAspectRatios...),
			AllowedResolutions:           append([]string(nil), route.Spec.AllowedResolutions...),
			ExpectedContainer:            route.MediaContract.ExpectedContainer,
			ExpectedCodec:                route.MediaContract.ExpectedCodec,
			ExpectedMaxBytes:             maxBytes,
			DeliveryReadyOutput:          route.MediaContract.DeliveryReadyOutput,
			RequiresProbe:                runtime.RequireVideoProbe,
			TranscodeAllowed:             runtime.VideoTranscodeAllowed,
			SupportsProviderIdempotency:  false,
			ProviderIdempotencyGuarantee: domain.ProviderIdempotencyNone,
			MaxProviderAttempts:          1,
			MaxFallbackAttempts:          0,
			MaxProviderCostCredits:       route.Spec.MaxProviderCostCredits,
		})
		if route.Spec.RequiresReferenceVideo {
			contracts[len(contracts)-1].AllowedOutputResolutions = []string{"4k"}
			contracts[len(contracts)-1].AllowedAspectRatios = nil
		}
	}
	return contracts
}
