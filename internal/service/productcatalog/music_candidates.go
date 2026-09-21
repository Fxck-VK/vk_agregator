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
		switch c.Kind {
		case "audio":
			model.Description = "Создание музыки, работа с треками и музыкальные инструменты."
			ops := musicgeneration.OperationsForModel(c.PublicID)
			if len(ops) == 0 {
				model.Description = "Ожидает проверки тарификации перед запуском."
				model.Operations = append(model.Operations, pendingSpeechOperation(c))
				break
			}
			for _, op := range ops {
				if operation := pendingMusicOperation(c, op); operation.Music != nil {
					model.Operations = append(model.Operations, operation)
				}
			}
		case "image":
			model.Categories = []string{"images"}
			model.Description = "Ожидает проверки перед запуском. Референсы и редактирование выключены."
			model.Operations = append(model.Operations, pendingImageOperation(c))
		case "video":
			model.Operations = append(model.Operations, pendingVideoOperation(c))
		}
		out = append(out, model)
	}
	return out
}

func pendingMusicOperation(c providermodels.MediaCandidate, op musicgeneration.Operation) WorkspaceOperation {
	s, err := pricingcatalog.MusicCandidateQuote(c.PublicID, string(op.ID), false)
	if err != nil {
		return WorkspaceOperation{}
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
	return WorkspaceOperation{ID: string(op.ID), Kind: "audio", Enabled: false, Inputs: inputs, Audio: &WorkspaceAudio{Tasks: []string{"music", "transform"}, Languages: []string{}, Voices: []string{}, Formats: formats}, Music: &controls}
}

func pendingImageOperation(c providermodels.MediaCandidate) WorkspaceOperation {
	image := WorkspaceImage{QualityLabel: "Вариант", ShowOutputCount: false, MaxOutputCount: 1, QualityOptions: []string{"standard"}, DefaultQuality: "standard", AllowedAspectRatios: append([]string(nil), c.Capabilities.API.Image.AspectRatios...), DefaultAspectRatio: "16:9", PriceByQuality: map[string]int64{}, PriceByVariant: map[string]int64{}}
	quote, err := pricingcatalog.ImageCandidateQuote(c.PublicID)
	if err == nil {
		image.PriceByQuality["standard"] = quote.InternalCredits
		for _, ratio := range image.AllowedAspectRatios {
			image.PriceByVariant["standard:"+ratio] = quote.InternalCredits
		}
	}
	return WorkspaceOperation{ID: "generate", Kind: "image", Enabled: false, Inputs: unknownWorkspaceInputs(), Image: &image}
}

func pendingVideoOperation(c providermodels.MediaCandidate) WorkspaceOperation {
	api := c.Capabilities.API.Video
	video := WorkspaceVideo{AllowedResolutions: append([]string(nil), api.Resolutions...), AllowedAspectRatios: append([]string(nil), api.AspectRatios...), AllowedDurationsSec: []int{}, DefaultResolution: pendingDefaultResolution(api.Resolutions), DefaultAspectRatio: pendingDefaultAspect(api.AspectRatios), DefaultDurationSec: 5, StartImage: "unsupported", EndImage: "unsupported", PriceByOption: map[string]int64{}, Variants: []WorkspaceVideoVariant{}}
	for _, seconds := range pendingVideoDurations(api.Duration) {
		hasPrice := false
		for _, resolution := range api.Resolutions {
			quote, err := pricingcatalog.MediaVideoCandidateQuote(c.PublicID, "", resolution, seconds)
			if err != nil {
				continue
			}
			hasPrice = true
			video.PriceByOption[resolution+":"+strconv.Itoa(seconds)] = quote.InternalCredits
			for _, ratio := range api.AspectRatios {
				video.Variants = append(video.Variants, WorkspaceVideoVariant{Resolution: resolution, DurationSec: seconds, AspectRatio: ratio})
			}
		}
		if hasPrice {
			video.AllowedDurationsSec = append(video.AllowedDurationsSec, seconds)
		}
	}
	if !containsInt(video.AllowedDurationsSec, video.DefaultDurationSec) && len(video.AllowedDurationsSec) > 0 {
		video.DefaultDurationSec = video.AllowedDurationsSec[0]
	}
	return WorkspaceOperation{ID: "generate", Kind: "video", Enabled: false, Inputs: unknownWorkspaceInputs(), Video: &video}
}

func pendingSpeechOperation(c providermodels.MediaCandidate) WorkspaceOperation {
	inputs := unknownWorkspaceInputs()
	audio := &WorkspaceAudio{Tasks: []string{}, Languages: []string{}, Voices: []string{}, Formats: []string{}}
	switch c.PublicID {
	case "gpt_4o_mini_tts":
		audio.Tasks = []string{"speech"}
		audio.Voices = []string{"alloy", "echo", "fable", "onyx", "nova", "shimmer"}
		audio.Formats = []string{"wav", "opus", "aac", "flac", "pcm"}
	case "whisper_1":
		inputs.Audio.Support = modelcontract.Supported
		inputs.Audio.MaxCount = 1
		inputs.Audio.MaxBytes = 25 << 20
		audio.Tasks = []string{"transcribe"}
		audio.Formats = []string{"json", "text", "srt", "vtt", "verbose_json"}
	default:
		audio.Tasks = []string{"audio"}
	}
	return WorkspaceOperation{ID: pendingSpeechOperationID(c.PublicID), Kind: "audio", Enabled: false, Inputs: inputs, Audio: audio}
}

func pendingSpeechOperationID(id string) string {
	switch id {
	case "gpt_4o_mini_tts":
		return "speak"
	case "whisper_1":
		return "transcribe"
	default:
		return "audio"
	}
}

func pendingVideoDurations(duration providermodels.DurationCapability) []int {
	if len(duration.AllowedSeconds) > 0 {
		return append([]int(nil), duration.AllowedSeconds...)
	}
	if duration.MinSeconds == nil || duration.MaxSeconds == nil || *duration.MinSeconds > *duration.MaxSeconds {
		return nil
	}
	out := make([]int, 0, *duration.MaxSeconds-*duration.MinSeconds+1)
	for seconds := *duration.MinSeconds; seconds <= *duration.MaxSeconds; seconds++ {
		out = append(out, seconds)
	}
	return out
}

func pendingDefaultResolution(values []string) string {
	for _, value := range values {
		if value == "720p" {
			return value
		}
	}
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func pendingDefaultAspect(values []string) string {
	for _, value := range values {
		if value == "16:9" {
			return value
		}
	}
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func containsInt(values []int, needle int) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
