package joborchestrator

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

type turboH3Selection struct {
	Alias       domain.VideoRouteAlias `json:"video_route_alias"`
	Prompt      string                 `json:"prompt"`
	Resolution  string                 `json:"resolution"`
	AspectRatio string                 `json:"aspect_ratio"`
	Duration    int                    `json:"duration_sec"`
	Audio       bool                   `json:"video_audio"`
	References  []uuid.UUID            `json:"reference_artifact_ids"`
}

func turboH3Alias(alias domain.VideoRouteAlias) bool {
	return alias == domain.VideoRouteKling30Turbo || alias == domain.VideoRouteMiniMaxH3
}

func (s turboH3Selection) normalize() turboH3Selection {
	s.Resolution = strings.ToLower(strings.TrimSpace(s.Resolution))
	if s.Duration == 0 {
		s.Duration = 5
	}
	if s.Resolution == "" {
		s.Resolution = "720p"
		if s.Alias == domain.VideoRouteMiniMaxH3 {
			s.Resolution = "2k"
		}
	}
	if s.AspectRatio == "" {
		s.AspectRatio = "16:9"
	}
	return s
}

func validateTurboH3Price(in CreateJobInput) error {
	var selection turboH3Selection
	err := json.Unmarshal(in.Params, &selection)
	if !turboH3Alias(selection.Alias) && !turboH3Alias(in.PricingSnapshot.Key.VideoRouteAlias) {
		return nil
	}
	if err != nil || !turboH3Alias(selection.Alias) || selection.Audio || len(selection.References) > 1 || !in.PricingSnapshot.Valid() || in.Operation != domain.OperationVideoGenerate || in.Modality != domain.ModalityVideo {
		return ErrBackendPriceRequired
	}
	selection = selection.normalize()
	want := pricingcatalog.ProductKey{Operation: in.Operation, Modality: in.Modality, VideoRouteAlias: selection.Alias, Resolution: selection.Resolution, DurationSec: selection.Duration}
	if in.PricingSnapshot.Key.Normalize() != want {
		return ErrBackendPriceRequired
	}
	return nil
}

func turboH3ReplayMatches(existing *domain.Job, in CreateJobInput) (bool, bool) {
	var old, next turboH3Selection
	oldErr, nextErr := json.Unmarshal(existing.Params, &old), json.Unmarshal(in.Params, &next)
	var oldPrice pricingcatalog.PricingSnapshot
	_ = json.Unmarshal(existing.PricingSnapshot, &oldPrice)
	if !turboH3Alias(old.Alias) && !turboH3Alias(next.Alias) && !turboH3Alias(oldPrice.Key.VideoRouteAlias) && !turboH3Alias(in.PricingSnapshot.Key.VideoRouteAlias) {
		return false, false
	}
	old, next = old.normalize(), next.normalize()
	matches := oldErr == nil && nextErr == nil &&
		existing.Status != domain.JobStatusPrepared &&
		existing.AccountID == ownerAccountID(in.UserID, in.AccountID) &&
		existing.Source == normalizedJobSource(in.Source) &&
		existing.OperationType == in.Operation && existing.Modality == in.Modality &&
		sameUUIDs(existing.InputArtifactIDs, in.InputArtifactIDs) &&
		old.Alias == next.Alias && old.Prompt == next.Prompt &&
		old.Resolution == next.Resolution && old.AspectRatio == next.AspectRatio &&
		old.Duration == next.Duration && old.Audio == next.Audio &&
		sameUUIDs(old.References, next.References)
	return matches, true
}
