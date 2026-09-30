package productcatalog

import (
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

func pendingTextWorkspaceModels() []WorkspaceModel {
	var out []WorkspaceModel
	for _, c := range providermodels.TextCandidates() {
		q, err := pricingcatalog.TextCandidateQuote(c.PublicID)
		if err != nil {
			continue
		}
		out = append(out, WorkspaceModel{ID: c.PublicID, Name: c.Name, Kind: "text", Categories: []string{"text", "study-work"},
			Description: "Ожидает проверки перед запуском.", Verification: "pending-verification", Capabilities: providermodels.Capabilities(c.PublicID),
			Operations: []WorkspaceOperation{{ID: "reply", Kind: "text", Enabled: false, Inputs: unknownWorkspaceInputs(),
				Text: &WorkspaceText{EstimateCredits: q.InternalCredits, MaxPromptBytes: q.TextInputTokenCap - 512, MaxOutputTokens: q.TextOutputTokenCap}}}})
	}
	return out
}
