package productcatalog

import (
	"strconv"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/mediaprobe"
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
				op := pendingSpeechOperation(c)
				model.Operations = append(model.Operations, op)
				if op.Enabled {
					model.Verification = "dev-smoke"
					model.Description = "Распознавание собственных MP3/WAV. Доступно для ручного тестирования на DEV."
					model.Capabilities = workspaceCapabilities(c.PublicID, op)
				}
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
		if providermodels.RuntimeRegistry().MediaCandidateRunnable(c.PublicID, "generate") && c.Kind == "audio" {
			model.Verification = "dev-smoke"
			model.Description = "Доступна для ручного тестирования на DEV. Проверка модели не завершена."
			for i := range model.Operations {
				op := &model.Operations[i]
				if providermodels.RuntimeRegistry().MediaCandidateRunnable(c.PublicID, op.ID) && op.Music != nil {
					op.Enabled = true
					op.Music.UnavailableReason = ""
					if op.Music.MaxUploads > 0 {
						op.Inputs.Audio = modelcontract.Input{Support: modelcontract.Supported, Enabled: true, Required: true, Processing: "native", MaxCount: 1, AllowedCounts: []int{1}, MaxBytes: mediaprobe.MaxMusicInputBytes, MaxDurationSec: mediaprobe.MaxMusicInputDurationSec, Formats: []modelcontract.FileFormat{{Extension: ".mp3", MIME: "audio/mpeg"}, {Extension: ".wav", MIME: "audio/wav"}}}
						op.Inputs.MaxTotalBytes = mediaprobe.MaxMusicInputBytes
						maximum := 1
						model.Capabilities.Application.Audio.Audio = providermodels.InputCapability{Support: providermodels.Supported, MaxCount: &maximum, Extensions: []string{"mp3", "wav"}}
						model.Capabilities.Application.Notes = []string{"На DEV доступны собственные MP3/WAV до 25 МиБ; кавер и продолжение требуют запись короче 8 минут. Проверка результатов не завершена."}
					}
				}
			}
			out[len(out)-1] = model
		}
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
	qualities := providermodels.ImageCandidateQualities(c)
	image := WorkspaceImage{QualityLabel: "Вариант", ShowOutputCount: false, MaxOutputCount: 1, QualityOptions: qualities, DefaultQuality: qualities[0], AllowedAspectRatios: append([]string(nil), c.Capabilities.Application.Image.AspectRatios...), DefaultAspectRatio: "16:9", PriceByQuality: map[string]int64{}, PriceByVariant: map[string]int64{}}
	for _, quality := range qualities {
		quote, err := pricingcatalog.ImageCandidateQualityQuote(c.PublicID, quality)
		if err != nil {
			continue
		}
		image.PriceByQuality[quality] = quote.InternalCredits
		for _, ratio := range image.AllowedAspectRatios {
			image.PriceByVariant[quality+":"+ratio] = quote.InternalCredits
		}
	}
	return WorkspaceOperation{ID: "generate", Kind: "image", Enabled: false, Inputs: unknownWorkspaceInputs(), Image: &image}
}

func pendingVideoOperation(c providermodels.MediaCandidate) WorkspaceOperation {
	if providermodels.IsVideoExpansion(c.PublicID) {
		return pendingVideoExpansionOperation(c)
	}
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

func pendingVideoExpansionOperation(c providermodels.MediaCandidate) WorkspaceOperation {
	api := c.Capabilities.API.Video
	resolution, seconds := providermodels.VideoCandidateDefaults(c.PublicID)
	video := WorkspaceVideo{AllowedResolutions: append([]string(nil), api.Resolutions...), AllowedAspectRatios: append([]string(nil), api.AspectRatios...), AllowedDurationsSec: providermodels.VideoCandidateDurations(c), DefaultResolution: resolution, DefaultDurationSec: seconds, DefaultAspectRatio: "16:9", StartImage: "unsupported", EndImage: "unsupported", PriceByOption: map[string]int64{}, Variants: []WorkspaceVideoVariant{}}
	video.AutomaticDuration = api.Duration.Mode == "automatic"
	for _, res := range video.AllowedResolutions {
		for _, duration := range video.AllowedDurationsSec {
			if quote, err := pricingcatalog.MediaVideoCandidateQuote(c.PublicID, "", res, duration); err == nil {
				video.PriceByOption[res+":"+strconv.Itoa(duration)] = quote.InternalCredits
			}
			for _, ratio := range video.AllowedAspectRatios {
				variant := WorkspaceVideoVariant{Resolution: res, DurationSec: duration, AspectRatio: ratio}
				videoExpansionOutputMetadata(c.PublicID, &variant)
				video.Variants = append(video.Variants, variant)
			}
		}
	}
	inputs := unknownWorkspaceInputs()
	if providermodels.VideoCandidateRequiresImages(c.PublicID) {
		inputs.Images = modelcontract.Input{Support: modelcontract.Supported, Required: true, MaxCount: 7, AllowedCounts: []int{1, 2, 3, 4, 5, 6, 7}, Formats: []modelcontract.FileFormat{{Extension: ".png", MIME: "image/png"}, {Extension: ".jpg", MIME: "image/jpeg"}, {Extension: ".jpeg", MIME: "image/jpeg"}}, MaxBytes: WebReferenceMaxBytes}
	}
	return WorkspaceOperation{ID: "generate", Kind: "video", Enabled: false, Inputs: inputs, Video: &video}
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
		inputs.Audio.MaxBytes = domain.WhisperMaxInputBytes
		audio.Tasks = []string{"transcribe"}
		audio.Formats = []string{"json", "text", "srt", "vtt", "verbose_json"}
	default:
		audio.Tasks = []string{"audio"}
	}
	if c.PublicID == "whisper_1" && providermodels.RuntimeRegistry().MediaCandidateRunnable(c.PublicID, "transcribe") {
		inputs.Audio = modelcontract.Input{Support: modelcontract.Supported, Enabled: true, Required: true, Processing: "native", MaxCount: 1, AllowedCounts: []int{1}, MaxBytes: domain.WhisperMaxInputBytes, MaxDurationSec: domain.WhisperMaxInputDurationSec, Formats: []modelcontract.FileFormat{{Extension: ".mp3", MIME: "audio/mpeg"}, {Extension: ".wav", MIME: "audio/wav"}}}
		inputs.MaxTotalBytes = domain.WhisperMaxInputBytes
		audio.Formats = []string{"json"}
		audio.CreditsPerStartedMinute = pricingcatalog.WhisperDEVCreditsPerStartedMinute
		return WorkspaceOperation{ID: "transcribe", Kind: "audio", Enabled: true, Inputs: inputs, Audio: audio}
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
