package worker

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"slices"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/videoreference"
)

type ReferenceVideoSigner interface {
	URL(jobID, artifactID uuid.UUID) (string, error)
}

func (p *processor) motionReferenceURL(ctx context.Context, job *domain.Job, pp promptParams, model string, duration int) (string, error) {
	invalid := providerResultError{class: domain.ProviderErrInvalidRequest, message: "invalid motion reference"}
	if pp.VideoAudio {
		var price pricingcatalog.PricingSnapshot
		if json.Unmarshal(job.PricingSnapshot, &price) != nil || !price.Valid() || price.Key.Quality != pricingcatalog.VideoQualityAudio {
			return "", invalid
		}
	}
	if model != "kling-v2-6-motion-control" {
		return "", nil
	}
	if p.providerReferences == nil || p.artifactRepo == nil || pp.ReferenceVideoArtifactID == uuid.Nil || !slices.Contains(job.InputArtifactIDs, pp.ReferenceVideoArtifactID) {
		return "", invalid
	}
	a, err := p.artifactRepo.GetByID(ctx, pp.ReferenceVideoArtifactID)
	if err != nil {
		return "", invalid
	}
	maximum := 10
	if pp.CharacterOrientation == "video" {
		maximum = 30
	} else if pp.CharacterOrientation != "image" {
		return "", invalid
	}
	seconds, err := videoreference.Validate(a, workerJobOwnerID(job), maximum)
	var price pricingcatalog.PricingSnapshot
	if err != nil || seconds != duration || json.Unmarshal(job.PricingSnapshot, &price) != nil || !price.Valid() || price.Key.VideoRouteAlias != domain.VideoRouteKling26Motion || price.Key.DurationSec != seconds {
		return "", invalid
	}
	return p.providerReferences.URL(job.ID, a.ID)
}
