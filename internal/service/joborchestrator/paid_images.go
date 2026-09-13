package joborchestrator

import (
	"bytes"
	"encoding/json"
	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

func validatePaidImagePrice(in CreateJobInput) error {
	var p struct {
		ModelID     string              `json:"model_id"`
		Provider    domain.ProviderName `json:"provider"`
		ModelCode   string              `json:"model_code"`
		Prompt      string              `json:"prompt"`
		Quality     string              `json:"image_quality"`
		Resolution  string              `json:"resolution"`
		Size        string              `json:"size"`
		AspectRatio string              `json:"aspect_ratio"`
		OutputCount int                 `json:"output_count"`
		References  []uuid.UUID         `json:"reference_artifact_ids"`
	}
	snapshotRequiresParams := pricingSnapshotProvided(in.PricingSnapshot) && pricingcatalog.IsBoundedAPIMartImage(in.PricingSnapshot.Key.ImageModelID)
	if len(bytes.TrimSpace(in.Params)) == 0 {
		if snapshotRequiresParams {
			return ErrBackendPriceRequired
		}
		return nil
	}
	if err := json.Unmarshal(in.Params, &p); err != nil {
		if snapshotRequiresParams {
			return ErrBackendPriceRequired
		}
		return nil
	}
	if !snapshotRequiresParams && !pricingcatalog.IsBoundedAPIMartImage(p.ModelID) && !providermodels.IsNewAPIMartImageRoute(p.Provider, p.ModelCode) {
		return nil
	}
	if in.Operation != domain.OperationImageGenerate || in.Modality != domain.ModalityImage {
		return ErrBackendPriceRequired
	}
	ratio := p.AspectRatio
	if ratio == "" {
		ratio = p.Size
	}
	err := imagegeneration.ValidatePricedRequest(in.PricingSnapshot, imagegeneration.Request{Prompt: p.Prompt, ModelID: p.ModelID, Quality: p.Quality, AspectRatio: ratio, OutputCount: p.OutputCount, ReferenceCount: len(p.References)}, p.Provider, p.ModelCode, p.Resolution)
	if err != nil {
		return ErrBackendPriceRequired
	}
	return nil
}

func createJobReplayMatches(existing *domain.Job, in CreateJobInput) bool {
	if matches, applies := turboH3ReplayMatches(existing, in); applies {
		return matches
	}
	if paidImageReplayApplies(existing, in) {
		return paidImageReplayMatches(existing, in)
	}
	return paidTextReplayMatches(existing, in)
}

func paidImageReplayApplies(existing *domain.Job, in CreateJobInput) bool {
	existingIsPaidImage := existingPaidImageReplayCandidate(existing)
	incomingIsPaidImage := incomingPaidImageReplayCandidate(in)
	return existingIsPaidImage || incomingIsPaidImage
}

func existingPaidImageReplayCandidate(job *domain.Job) bool {
	if job == nil || job.OperationType != domain.OperationImageGenerate || job.Modality != domain.ModalityImage {
		return false
	}
	var snapshot pricingcatalog.PricingSnapshot
	if json.Unmarshal(job.PricingSnapshot, &snapshot) == nil && snapshot.Valid() && pricingcatalog.IsBoundedAPIMartImage(snapshot.Key.ImageModelID) {
		return true
	}
	p, ok := paidImageReplaySelectionFromParams(job.Params)
	return ok && (pricingcatalog.IsBoundedAPIMartImage(p.ModelID) || providermodels.IsNewAPIMartImageRoute(p.Provider, p.ModelCode))
}

func incomingPaidImageReplayCandidate(in CreateJobInput) bool {
	if in.Operation != domain.OperationImageGenerate || in.Modality != domain.ModalityImage {
		return false
	}
	if in.PricingSnapshot.Valid() && pricingcatalog.IsBoundedAPIMartImage(in.PricingSnapshot.Key.ImageModelID) {
		return true
	}
	p, ok := paidImageReplaySelectionFromParams(in.Params)
	return ok && (pricingcatalog.IsBoundedAPIMartImage(p.ModelID) || providermodels.IsNewAPIMartImageRoute(p.Provider, p.ModelCode))
}

func paidImageReplayMatches(existing *domain.Job, in CreateJobInput) bool {
	if existing == nil || existing.Status == domain.JobStatusPrepared ||
		existing.AccountID != ownerAccountID(in.UserID, in.AccountID) ||
		existing.Source != normalizedJobSource(in.Source) ||
		existing.OperationType != in.Operation ||
		existing.Modality != in.Modality ||
		!sameUUIDs(existing.InputArtifactIDs, in.InputArtifactIDs) {
		return false
	}
	old, oldOK := paidImageReplaySelectionFromParams(existing.Params)
	next, nextOK := paidImageReplaySelectionFromParams(in.Params)
	if !oldOK || !nextOK {
		return false
	}
	return old.samePublicIntent(next)
}

type paidImageReplaySelection struct {
	Provider    domain.ProviderName `json:"provider"`
	ModelID     string              `json:"model_id"`
	ModelCode   string              `json:"model_code"`
	Prompt      string              `json:"prompt"`
	Quality     string              `json:"image_quality"`
	Resolution  string              `json:"resolution"`
	Size        string              `json:"size"`
	AspectRatio string              `json:"aspect_ratio"`
	OutputCount int                 `json:"output_count"`
	References  []uuid.UUID         `json:"reference_artifact_ids"`
}

func paidImageReplaySelectionFromParams(raw json.RawMessage) (paidImageReplaySelection, bool) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return paidImageReplaySelection{}, false
	}
	var p paidImageReplaySelection
	if err := json.Unmarshal(raw, &p); err != nil {
		return paidImageReplaySelection{}, false
	}
	return p, true
}

func (p paidImageReplaySelection) samePublicIntent(other paidImageReplaySelection) bool {
	return p.ModelID == other.ModelID &&
		p.Prompt == other.Prompt &&
		p.Quality == other.Quality &&
		p.Resolution == other.Resolution &&
		p.Size == other.Size &&
		p.AspectRatio == other.AspectRatio &&
		normalizedImageOutputCount(p.OutputCount) == normalizedImageOutputCount(other.OutputCount) &&
		sameUUIDs(p.References, other.References)
}

func normalizedImageOutputCount(count int) int {
	if count == 0 {
		return 1
	}
	return count
}

func normalizedJobSource(source string) string {
	source = string(bytes.TrimSpace([]byte(source)))
	if source == "" {
		return "unknown"
	}
	return source
}

func sameUUIDs(a, b []uuid.UUID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
