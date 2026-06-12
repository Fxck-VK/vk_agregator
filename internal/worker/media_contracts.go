package worker

import (
	"fmt"
	"strings"

	"vk-ai-aggregator/internal/domain"
)

type mediaContractRegistry struct {
	contracts             []domain.ProviderMediaContract
	requireVideoProbe     bool
	videoTranscodeEnabled bool
}

func newMediaContractRegistry(contracts []domain.ProviderMediaContract, requireVideoProbe, videoTranscodeEnabled bool) mediaContractRegistry {
	return mediaContractRegistry{
		contracts:             append([]domain.ProviderMediaContract(nil), contracts...),
		requireVideoProbe:     requireVideoProbe,
		videoTranscodeEnabled: videoTranscodeEnabled,
	}
}

func (r mediaContractRegistry) validateProviderSubmit(provider domain.ProviderName, req domain.ProviderRequest, estimate domain.CostEstimate) (*domain.ProviderMediaContract, error) {
	if req.Modality != domain.ModalityVideo || req.Operation != domain.OperationVideoGenerate {
		return nil, nil
	}
	contract, ok := r.find(provider, req.ModelCode, req.Modality)
	if !ok {
		if provider == domain.ProviderMock && len(r.contracts) == 0 {
			return nil, nil
		}
		return nil, providerContractError{message: fmt.Sprintf("worker: no media contract for provider %s selected video model", provider)}
	}
	if err := contract.Validate(); err != nil {
		return contract, providerContractError{message: "worker: invalid media contract"}
	}
	if contract.MaxProviderAttempts > 0 {
		attempt := req.AttemptNo
		if attempt <= 0 {
			attempt = 1
		}
		if attempt > contract.MaxProviderAttempts {
			return contract, providerContractError{message: "worker: media contract provider attempt budget exceeded"}
		}
	}
	if !allowedDuration(req.DurationSec, contract.AllowedDurationsSec) {
		return contract, providerContractError{message: "worker: media contract rejected unsupported video duration"}
	}
	if !allowedToken(req.AspectRatio, contract.AllowedAspectRatios) {
		return contract, providerContractError{message: "worker: media contract rejected unsupported video aspect ratio"}
	}
	if !allowedToken(req.Resolution, contract.AllowedResolutions) {
		return contract, providerContractError{message: "worker: media contract rejected unsupported video resolution"}
	}
	if contract.RequiresProbe && !r.requireVideoProbe {
		return contract, providerContractError{message: "worker: media contract requires video probe policy"}
	}
	if contract.RequiresTranscode && !r.videoTranscodeEnabled {
		return contract, providerContractError{message: "worker: media contract requires enabled video transcode policy"}
	}
	if !contract.DeliveryReadyOutput && !r.videoTranscodeEnabled {
		return contract, providerContractError{message: "worker: media contract output is not delivery-ready"}
	}
	if contract.MaxProviderCostCredits > 0 && estimate.AmountCredits > contract.MaxProviderCostCredits {
		return contract, providerContractError{message: "worker: media contract provider cost budget exceeded"}
	}
	return contract, nil
}

func (r mediaContractRegistry) modelClass(provider domain.ProviderName, model string, modality domain.Modality) string {
	if contract, ok := r.find(provider, model, modality); ok && strings.TrimSpace(contract.ModelClass) != "" {
		return contract.ModelClass
	}
	return ""
}

func (r mediaContractRegistry) find(provider domain.ProviderName, model string, modality domain.Modality) (*domain.ProviderMediaContract, bool) {
	model = strings.TrimSpace(model)
	for i := len(r.contracts) - 1; i >= 0; i-- {
		contract := r.contracts[i]
		if contract.Provider != provider || contract.Modality != modality {
			continue
		}
		if model != "" && strings.TrimSpace(contract.Model) != model {
			continue
		}
		if model == "" && strings.TrimSpace(contract.Model) == "" {
			continue
		}
		return &contract, true
	}
	return nil, false
}

func allowedDuration(value int, allowed []int) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func allowedToken(value string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	value = strings.ToLower(strings.TrimSpace(value))
	for _, candidate := range allowed {
		if value == strings.ToLower(strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}

type providerContractError struct {
	message string
}

func (e providerContractError) Error() string {
	if e.message != "" {
		return e.message
	}
	return "worker: provider media contract rejected request"
}

func (e providerContractError) ProviderErrorClass() domain.ProviderErrorClass {
	return domain.ProviderErrUnsupportedCapab
}
