package worker

import (
	"testing"
	"vk-ai-aggregator/internal/domain"
)

func TestUnknownProviderCostIsNotAFavourableFreeQuote(t *testing.T) {
	candidates := []providerCandidate{{name: "unknown", costUnknown: true}, {name: "priced", cost: 10}}
	sortCandidates(candidates)
	if candidates[0].name != "priced" {
		t.Fatal("unknown provider price sorted as free")
	}
	contracts := newMediaContractRegistry(nil, false, false)
	if _, err := contracts.validateProviderSubmit(domain.ProviderMock, domain.ProviderRequest{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo}, domain.CostEstimate{Unknown: true}); err == nil {
		t.Fatal("unknown amount bypassed video spend guard")
	}
}
