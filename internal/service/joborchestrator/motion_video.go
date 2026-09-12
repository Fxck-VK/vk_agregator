package joborchestrator

import (
	"encoding/json"
	"slices"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

func motionInputPolicy(in CreateJobInput) (uuid.UUID, int, int, error) {
	var params struct {
		VideoRouteAlias          domain.VideoRouteAlias `json:"video_route_alias"`
		ReferenceVideoArtifactID uuid.UUID              `json:"reference_video_artifact_id"`
		CharacterOrientation     string                 `json:"character_orientation"`
		DurationSec              int                    `json:"duration_sec"`
		VideoAudio               bool                   `json:"video_audio"`
	}
	if len(in.Params) > 0 && json.Unmarshal(in.Params, &params) != nil {
		return uuid.Nil, 0, 0, ErrInvalidInputArtifact
	}
	if params.VideoAudio && (params.VideoRouteAlias != domain.VideoRouteKlingV3 || in.PricingSnapshot.Key.Quality != pricingcatalog.VideoQualityAudio) {
		return uuid.Nil, 0, 0, ErrInvalidInputArtifact
	}
	if params.VideoRouteAlias != domain.VideoRouteKling26Motion {
		if params.ReferenceVideoArtifactID != uuid.Nil {
			return uuid.Nil, 0, 0, ErrInvalidInputArtifact
		}
		return uuid.Nil, 0, 0, nil
	}
	maximum := 10
	if params.CharacterOrientation == "video" {
		maximum = 30
	} else if params.CharacterOrientation != "image" {
		return uuid.Nil, 0, 0, ErrInvalidInputArtifact
	}
	if in.Operation != domain.OperationVideoGenerate || in.Modality != domain.ModalityVideo || params.ReferenceVideoArtifactID == uuid.Nil || !slices.Contains(in.InputArtifactIDs, params.ReferenceVideoArtifactID) || params.DurationSec < 3 || params.DurationSec > maximum || in.PricingSnapshot.Key.DurationSec != params.DurationSec || in.PricingSnapshot.Key.VideoRouteAlias != params.VideoRouteAlias {
		return uuid.Nil, 0, 0, ErrInvalidInputArtifact
	}
	return params.ReferenceVideoArtifactID, params.DurationSec, maximum, nil
}
