package productcatalog

import (
	"strconv"
	"vk-ai-aggregator/internal/service/modelcontract"
	"vk-ai-aggregator/internal/service/musicgeneration"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

// WorkspaceMusic describes a product operation without native API IDs or URLs.
type WorkspaceMusic struct {
	musicgeneration.Operation
	EstimateCredits    int64  `json:"estimate_credits"`
	MaxEstimateCredits int64  `json:"max_estimate_credits,omitempty"`
	UnavailableReason  string `json:"unavailable_reason,omitempty"`
}

func pendingMediaWorkspaceModels() []WorkspaceModel {
	out := []WorkspaceModel{}
	for _, c := range providermodels.MediaCandidates() {
		model := WorkspaceModel{ID: c.PublicID, Name: c.Name, Description: "Ожидает проверки перед запуском.", Kind: c.Kind, Categories: []string{"video-audio"}, Verification: "pending-verification", Capabilities: &c.Capabilities, Operations: []WorkspaceOperation{}}
		if c.Kind == "audio" {
			model.Description = "Создание музыки, работа с треками и музыкальные инструменты."
			for _, op := range musicgeneration.Operations() {
				s, err := pricingcatalog.MusicCandidateQuote(c.PublicID, string(op.ID), false)
				if err != nil {
					continue
				}
				controls := WorkspaceMusic{Operation: op, EstimateCredits: s.InternalCredits, UnavailableReason: "Операция ожидает проверки перед запуском."}
				if op.SupportsMax {
					maxQuote, err := pricingcatalog.MusicCandidateQuote(c.PublicID, string(op.ID), true)
					if err == nil {
						controls.MaxEstimateCredits = maxQuote.InternalCredits
					}
				}
				formats := []string{}
				if op.SupportsAudioFormat {
					formats = []string{"mp3", "m4a", "wav"}
				}
				inputs := unknownWorkspaceInputs()
				if op.MaxUploads > 0 {
					inputs.Audio.Support = modelcontract.Supported
					inputs.Audio.MaxCount = op.MaxUploads
				}
				model.Operations = append(model.Operations, WorkspaceOperation{ID: string(op.ID), Kind: "audio", Enabled: false, Inputs: inputs, Audio: &WorkspaceAudio{Tasks: []string{"music", "transform"}, Languages: []string{}, Voices: []string{}, Formats: formats}, Music: &controls})
			}
		} else {
			api := c.Capabilities.API.Video
			video := WorkspaceVideo{AllowedResolutions: api.Resolutions, AllowedAspectRatios: api.AspectRatios, AllowedDurationsSec: []int{}, DefaultResolution: "720p", DefaultAspectRatio: "16:9", DefaultDurationSec: 5, StartImage: api.StartFrame, EndImage: api.EndFrame, PriceByOption: map[string]int64{}, Variants: []WorkspaceVideoVariant{}}
			for seconds := 3; seconds <= 15; seconds++ {
				video.AllowedDurationsSec = append(video.AllowedDurationsSec, seconds)
				for _, resolution := range api.Resolutions {
					quote, err := pricingcatalog.MediaVideoCandidateQuote(c.PublicID, "", resolution, seconds)
					if err != nil {
						continue
					}
					video.PriceByOption[resolution+":"+strconv.Itoa(seconds)] = quote.InternalCredits
					for _, ratio := range api.AspectRatios {
						video.Variants = append(video.Variants, WorkspaceVideoVariant{Resolution: resolution, DurationSec: seconds, AspectRatio: ratio})
					}
				}
			}
			model.Operations = append(model.Operations, WorkspaceOperation{ID: "generate", Kind: "video", Enabled: false, Inputs: unknownWorkspaceInputs(), Video: &video})
		}
		out = append(out, model)
	}
	return out
}
