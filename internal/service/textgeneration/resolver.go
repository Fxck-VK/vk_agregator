// Package textgeneration resolves public text choices and immutable bounded
// reply prices. It never calls providers or accepts client-owned billing data.
package textgeneration

import (
	"errors"
	"strings"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/modelcatalog"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

var ErrUnavailable = errors.New("text model unavailable")

type PublicModel struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	EstimateCredits int64  `json:"estimate_credits"`
	MaxPromptBytes  int    `json:"max_prompt_bytes,omitempty"`
	MaxOutputTokens int    `json:"max_output_tokens,omitempty"`
}
type SnapshotCatalog interface {
	Snapshot(pricingcatalog.ProductKey) (pricingcatalog.PricingSnapshot, error)
}

func Models(enabledIDs []string, prices SnapshotCatalog) []PublicModel {
	models := []PublicModel{{ID: providermodels.PublicTextChatGPT, Name: "NeiroHub Chat"}}
	for _, id := range enabledIDs {
		m, ok := providermodels.PaidTextModel(id)
		if !ok || prices == nil {
			continue
		}
		snapshot, err := prices.Snapshot(Key(id))
		if err != nil || !snapshot.Valid() {
			continue
		}
		models = append(models, PublicModel{ID: id, Name: m.DisplayName, EstimateCredits: snapshot.InternalCredits, MaxPromptBytes: pricingcatalog.TextMaxInputTokens - 512, MaxOutputTokens: pricingcatalog.TextMaxOutputTokens})
	}
	return models
}

func Key(id string) pricingcatalog.ProductKey {
	return pricingcatalog.ProductKey{Operation: domain.OperationTextGenerate, Modality: domain.ModalityText, TextModelID: id}
}

func Resolve(id, prompt string, allowed []PublicModel, prices SnapshotCatalog) (modelcatalog.Model, pricingcatalog.PricingSnapshot, error) {
	id = strings.TrimSpace(id)
	if id == "" || id == providermodels.PublicTextChatGPT {
		m, _ := modelcatalog.ResolvePublicModel(domain.OperationTextGenerate, "")
		return m, pricingcatalog.PricingSnapshot{}, nil
	}
	m, ok := providermodels.PaidTextModel(id)
	if !ok || prices == nil {
		return modelcatalog.Model{}, pricingcatalog.PricingSnapshot{}, ErrUnavailable
	}
	enabled := false
	for _, entry := range allowed {
		if entry.ID == id {
			enabled = true
		}
	}
	if !enabled || len(prompt) > pricingcatalog.TextMaxInputTokens-512 || strings.TrimSpace(prompt) == "" {
		return modelcatalog.Model{}, pricingcatalog.PricingSnapshot{}, ErrUnavailable
	}
	snapshot, err := prices.Snapshot(Key(id))
	if err != nil || !snapshot.Valid() {
		return modelcatalog.Model{}, pricingcatalog.PricingSnapshot{}, ErrUnavailable
	}
	return modelcatalog.Model{ModelID: m.PublicID, ModelName: m.DisplayName, Provider: m.Provider, ModelCode: m.ProviderModelID, ExposeID: true}, snapshot, nil
}
