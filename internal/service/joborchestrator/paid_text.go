package joborchestrator

import (
	"encoding/json"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/textgeneration"
)

func validatePaidTextPrice(in CreateJobInput) error {
	var params struct {
		ModelID   string              `json:"model_id"`
		Provider  domain.ProviderName `json:"provider"`
		ModelCode string              `json:"model_code"`
	}
	_ = json.Unmarshal(in.Params, &params)
	model, paid := providermodels.PaidTextModel(params.ModelID)
	if !paid && params.Provider != domain.ProviderKIE && !(in.Operation == domain.OperationTextGenerate && params.Provider == domain.ProviderAPIMart) {
		return nil
	}
	if !paid || in.Operation != domain.OperationTextGenerate || in.Modality != domain.ModalityText || params.Provider != model.Provider || params.ModelCode != model.ProviderModelID || !in.PricingSnapshot.Valid() || in.PricingSnapshot.Key != textgeneration.Key(params.ModelID) || len(in.InputArtifactIDs) > 0 {
		return ErrBackendPriceRequired
	}
	return nil
}

// Replays may use a newer catalog snapshot, but must identify the same user
// intent. The original price remains attached to the original Job.
func paidTextReplayMatches(existing *domain.Job, in CreateJobInput) bool {
	type selection struct {
		ModelID   string              `json:"model_id"`
		Prompt    string              `json:"prompt"`
		Provider  domain.ProviderName `json:"provider"`
		ModelCode string              `json:"model_code"`
	}
	var old, next selection
	_ = json.Unmarshal(existing.Params, &old)
	_ = json.Unmarshal(in.Params, &next)
	_, oldPaid := providermodels.PaidTextModel(old.ModelID)
	_, nextPaid := providermodels.PaidTextModel(next.ModelID)
	if !oldPaid && !nextPaid && old.Provider != domain.ProviderKIE && next.Provider != domain.ProviderKIE {
		return true
	}
	return old == next && existing.AccountID == ownerAccountID(in.UserID, in.AccountID) && existing.OperationType == in.Operation && existing.Modality == in.Modality && existing.Source == in.Source
}
