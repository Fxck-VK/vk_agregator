package worker

import (
	"encoding/json"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/textgeneration"
)

func paidTextJobLimits(job *domain.Job, pp promptParams) (int, int, error) {
	model, paid := providermodels.PaidTextModel(pp.ModelID)
	if !paid && pp.Provider != domain.ProviderKIE && !(job.OperationType == domain.OperationTextGenerate && pp.Provider == domain.ProviderAPIMart) {
		return 0, 0, nil
	}
	var snapshot pricingcatalog.PricingSnapshot
	if !paid || job.OperationType != domain.OperationTextGenerate || job.Modality != domain.ModalityText || pp.Provider != model.Provider || pp.ModelCode != model.ProviderModelID || len(job.InputArtifactIDs) > 0 ||
		json.Unmarshal(job.PricingSnapshot, &snapshot) != nil || !snapshot.Valid() || snapshot.Key != textgeneration.Key(pp.ModelID) || job.CostReserved < snapshot.InternalCredits {
		return 0, 0, &deterministicRequestError{class: domain.ProviderErrInvalidRequest}
	}
	return snapshot.TextInputTokenCap, snapshot.TextOutputTokenCap, nil
}
