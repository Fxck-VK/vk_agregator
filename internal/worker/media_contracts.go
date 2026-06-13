package worker

import (
	"fmt"
	"strconv"
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

func (r mediaContractRegistry) videoContract(provider domain.ProviderName, model string) (*domain.ProviderMediaContract, bool) {
	return r.find(provider, model, domain.ModalityVideo)
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

func deliveryReadyVideoOutput(contract *domain.ProviderMediaContract, metadata domain.ArtifactMediaMetadata, sizeBytes int64) bool {
	if contract == nil || !contract.DeliveryReadyOutput {
		return false
	}
	metadata = metadata.Normalize()
	if metadata.ProbeStatus != domain.MediaProbePassed {
		return false
	}
	if !matchesMediaToken(metadata.Container, contract.ExpectedContainer) {
		return false
	}
	if !matchesMediaToken(metadata.Codec, contract.ExpectedCodec) {
		return false
	}
	if contract.ExpectedMaxBytes > 0 && sizeBytes > contract.ExpectedMaxBytes {
		return false
	}
	if !allowedDuration(durationSeconds(metadata.DurationMS), contract.AllowedDurationsSec) {
		return false
	}
	if !allowedOutputAspect(metadata.Width, metadata.Height, contract.AllowedAspectRatios) {
		return false
	}
	if !allowedOutputResolution(metadata.Width, metadata.Height, contract.AllowedResolutions) {
		return false
	}
	return true
}

func matchesMediaToken(value, expected string) bool {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if expected == "" {
		return true
	}
	return strings.ToLower(strings.TrimSpace(value)) == expected
}

func durationSeconds(durationMS int64) int {
	if durationMS <= 0 {
		return 0
	}
	return int((durationMS + 999) / 1000)
}

func allowedOutputAspect(width, height int, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	if width <= 0 || height <= 0 {
		return false
	}
	actual := reducedAspect(width, height)
	return allowedToken(actual, allowed)
}

func reducedAspect(width, height int) string {
	divisor := gcd(width, height)
	if divisor <= 0 {
		return ""
	}
	return strconv.Itoa(width/divisor) + ":" + strconv.Itoa(height/divisor)
}

func gcd(a, b int) int {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func allowedOutputResolution(width, height int, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	if width <= 0 || height <= 0 {
		return false
	}
	for _, token := range allowed {
		if outputResolutionMatches(width, height, token) {
			return true
		}
	}
	return false
}

func outputResolutionMatches(width, height int, token string) bool {
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" {
		return false
	}
	if strings.HasSuffix(token, "p") {
		maxHeight, err := strconv.Atoi(strings.TrimSuffix(token, "p"))
		return err == nil && maxHeight > 0 && height <= maxHeight
	}
	if strings.Contains(token, "x") {
		parts := strings.SplitN(token, "x", 2)
		maxWidth, werr := strconv.Atoi(strings.TrimSpace(parts[0]))
		maxHeight, herr := strconv.Atoi(strings.TrimSpace(parts[1]))
		return werr == nil && herr == nil && maxWidth > 0 && maxHeight > 0 && width <= maxWidth && height <= maxHeight
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
