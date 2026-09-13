package worker

import (
	"encoding/json"
	"strings"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

func validatePaidImageJob(job *domain.Job, p promptParams) error {
	var snapshot pricingcatalog.PricingSnapshot
	snapshotErr := json.Unmarshal(job.PricingSnapshot, &snapshot)
	if !pricingcatalog.IsBoundedAPIMartImage(snapshot.Key.ImageModelID) && !pricingcatalog.IsBoundedAPIMartImage(p.ModelID) && !providermodels.IsNewAPIMartImageRoute(p.Provider, p.ModelCode) {
		return nil
	}
	if job.OperationType != domain.OperationImageGenerate || job.Modality != domain.ModalityImage || snapshotErr != nil || json.Unmarshal(job.Params, &p) != nil || job.CostReserved < snapshot.InternalCredits {
		return invalidImagePrice()
	}
	ratio := p.AspectRatio
	if ratio == "" {
		ratio = p.Size
	}
	if imagegeneration.ValidatePricedRequest(snapshot, imagegeneration.Request{Prompt: p.Prompt, ModelID: p.ModelID, Quality: p.ImageQuality, AspectRatio: ratio, OutputCount: p.OutputCount, ReferenceCount: len(p.ReferenceArtifactIDs)}, p.Provider, p.ModelCode, p.Resolution) != nil {
		return invalidImagePrice()
	}
	return nil
}

func paidImageOutputCountMatches(job *domain.Job, urls []string) bool {
	var p promptParams
	_ = json.Unmarshal(job.Params, &p)
	if !providermodels.IsNewAPIMartImageRoute(p.Provider, p.ModelCode) {
		return true
	}
	count := p.OutputCount
	if count == 0 {
		count = 1
	}
	// Durable task results omit private URLs. Once all artifacts were saved,
	// recovery continues from those owned artifacts without refetching URLs.
	if len(urls) == 0 && len(job.OutputArtifactIDs) == count {
		return true
	}
	if len(urls) != count {
		return false
	}
	seen := make(map[string]bool, len(urls))
	for _, raw := range urls {
		value := strings.TrimSpace(raw)
		if value == "" || seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}

func invalidImagePrice() error {
	return &deterministicRequestError{class: domain.ProviderErrInvalidRequest}
}

// Keep channel metadata and any provider-native overrides out of image calls.
func paidImageProviderParams(p promptParams, size string) json.RawMessage {
	params := struct {
		ImageQuality string `json:"image_quality"`
		Resolution   string `json:"resolution"`
		Size         string `json:"size"`
		AspectRatio  string `json:"aspect_ratio,omitempty"`
		OutputCount  int    `json:"output_count"`
	}{p.ImageQuality, p.Resolution, size, p.AspectRatio, p.OutputCount}
	if params.OutputCount == 0 {
		params.OutputCount = 1
	}
	raw, _ := json.Marshal(params)
	return raw
}
