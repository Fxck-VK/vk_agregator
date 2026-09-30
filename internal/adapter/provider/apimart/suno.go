package apimart

import (
	"context"
	"encoding/json"
	"math"
	"net/url"
	"strconv"
	"strings"

	"vk-ai-aggregator/internal/domain"
)

const (
	ModelSunoV6     = "suno-v6"
	ModelSunoV6Wild = "suno-v6-wild"
	ModelSunoV6Mini = "suno-v6-mini"

	sunoProviderModel = "suno"
	sunoTaskPrefix    = "music:"
)

type sunoEndpointSpec struct {
	path             string
	includeModel     bool
	allowVersion     bool
	allowCustomModel bool
}

func isSunoModel(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case ModelSunoV6, ModelSunoV6Wild, ModelSunoV6Mini:
		return true
	default:
		return false
	}
}

func sunoVersionFromModelCode(model string) string {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case ModelSunoV6Wild:
		return "v6-wild"
	case ModelSunoV6Mini:
		return "v6-mini"
	case ModelSunoV6:
		return "v6"
	default:
		return ""
	}
}

func validateSunoRequest(req domain.ProviderRequest) error {
	if req.Operation != domain.OperationAudioMusic || req.Modality != domain.ModalityAudio {
		return sunoInvalid("unsupported Suno operation")
	}
	if !isSunoModel(req.ModelCode) {
		return sunoInvalid("unsupported Suno model")
	}
	music, err := sunoMusicRequest(req)
	if err != nil {
		return err
	}
	spec, ok := sunoActionSpec(music.Action)
	if !ok {
		return sunoInvalid("unsupported Suno action")
	}
	if strings.TrimSpace(music.CustomModelID) != "" && !spec.allowCustomModel {
		return sunoInvalid("custom model is not supported for Suno action")
	}
	if strings.TrimSpace(music.CustomModelID) != "" && strings.TrimSpace(music.PersonaID) != "" {
		return sunoInvalid("custom model and persona are incompatible")
	}
	if err := validateSunoTextBounds(music); err != nil {
		return err
	}
	if err := validateSunoNumericBounds(music); err != nil {
		return err
	}
	if err := validateSunoCustomModeBounds(music); err != nil {
		return err
	}
	return validateSunoActionInputs(req, music)
}

func (p *Provider) submitSuno(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	if err := validateSunoRequest(req); err != nil {
		return domain.ProviderTask{}, err
	}
	return p.submitUnversionedOnce(ctx, req, p.submitSunoTask)
}

func (p *Provider) submitSunoTask(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	music, err := sunoMusicRequest(req)
	if err != nil {
		return domain.ProviderTask{}, err
	}
	path, body, err := sunoRequestBody(req, music)
	if err != nil {
		return domain.ProviderTask{}, err
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return domain.ProviderTask{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid Suno request"}
	}
	safeReq := req
	safeReq.Params = domain.DurableProviderTaskRequestJSON()
	task, err := p.postUnversionedTask(ctx, safeReq, path, raw)
	if err != nil {
		return domain.ProviderTask{}, err
	}
	task.ExternalID = prefixSunoTaskID(task.ExternalID)
	return task, nil
}

func (p *Provider) pollSuno(ctx context.Context, ref domain.ProviderTaskRef) (domain.ProviderTaskResult, error) {
	taskID := strings.TrimSpace(strings.TrimPrefix(ref.ExternalID, sunoTaskPrefix))
	if taskID == "" {
		return domain.ProviderTaskResult{Status: domain.ProviderTaskFailed, ErrorClass: domain.ProviderErrTaskNotFound}, nil
	}
	path := "/music/tasks/" + url.PathEscape(taskID)
	if lang := strings.TrimSpace(p.cfg.TaskLanguage); lang != "" {
		path += "?language=" + url.QueryEscape(lang)
	}
	var decoded sunoTaskStatusResponse
	if err := p.getJSON(ctx, path, &decoded); err != nil {
		return domain.ProviderTaskResult{}, err
	}
	if decoded.Code != 200 {
		errValue := decoded.Error
		if errValue.empty() {
			errValue = decoded.Data.providerError()
		}
		return domain.ProviderTaskResult{}, apiEnvelopeError(decoded.Code, errValue, decoded.Message)
	}
	status := mapTaskStatus(decoded.Data.Status)
	raw := sanitizedSunoTaskMetadata(decoded.Data)
	switch status {
	case domain.ProviderTaskSucceeded:
		music, outputs, text := decoded.Data.Result.normalized()
		if len(outputs) == 0 && text == "" && !sunoMusicResultHasData(music) {
			if perr := decoded.Data.providerError(); !perr.empty() {
				class := classifyAPIMartError(0, perr.codeString(), perr.Type, perr.Message)
				return domain.ProviderTaskResult{Status: domain.ProviderTaskFailed, ErrorClass: class, ErrorMessage: providerErrorMessage(class, "apimart task failed"), Raw: raw}, nil
			}
			return domain.ProviderTaskResult{Status: domain.ProviderTaskFailed, ErrorClass: domain.ProviderErrOutputDownloadFailed, ErrorMessage: "apimart task completed without music output"}, nil
		}
		return domain.ProviderTaskResult{Status: status, OutputURLs: outputs, Text: text, Music: &music, Raw: raw}, nil
	case domain.ProviderTaskFailed:
		perr := decoded.Data.providerError()
		class := classifyAPIMartError(0, perr.codeString(), perr.Type, perr.Message)
		return domain.ProviderTaskResult{Status: domain.ProviderTaskFailed, ErrorClass: class, ErrorMessage: providerErrorMessage(class, "apimart task failed"), Raw: raw}, nil
	case domain.ProviderTaskCancelled:
		return domain.ProviderTaskResult{Status: domain.ProviderTaskCancelled, Raw: raw}, nil
	case domain.ProviderTaskPending, domain.ProviderTaskProcessing:
		return domain.ProviderTaskResult{Status: status, Raw: raw}, nil
	default:
		return domain.ProviderTaskResult{Status: domain.ProviderTaskProcessing, Raw: raw}, nil
	}
}

func sunoActionSpec(action domain.MusicAction) (sunoEndpointSpec, bool) {
	versioned := func(path string) sunoEndpointSpec {
		return sunoEndpointSpec{path: path, includeModel: true, allowVersion: true, allowCustomModel: true}
	}
	unversioned := func(path string) sunoEndpointSpec {
		return sunoEndpointSpec{path: path, includeModel: true}
	}
	switch action {
	case domain.MusicActionGenerate:
		return versioned("/music/generations"), true
	case domain.MusicActionLyrics:
		return unversioned("/music/generations/lyrics"), true
	case domain.MusicActionInspo:
		return versioned("/music/generations/inspo"), true
	case domain.MusicActionSounds:
		return sunoEndpointSpec{path: "/music/generations/sounds", includeModel: true, allowVersion: true}, true
	case domain.MusicActionUpsampleTags:
		return unversioned("/music/generations/upsampleTags"), true
	case domain.MusicActionUpload:
		return sunoEndpointSpec{path: "/music/generations/uploadTask"}, true
	case domain.MusicActionUploadCover:
		return versioned("/music/generations/uploadCover"), true
	case domain.MusicActionUploadExtend:
		return versioned("/music/generations/uploadExtend"), true
	case domain.MusicActionCreateModel:
		return unversioned("/music/generations/createModel"), true
	case domain.MusicActionExtend:
		return versioned("/music/generations/extend"), true
	case domain.MusicActionCover:
		return versioned("/music/generations/coverSong"), true
	case domain.MusicActionRemaster:
		return unversioned("/music/generations/remaster"), true
	case domain.MusicActionStems:
		return unversioned("/music/generations/stems"), true
	case domain.MusicActionStemsAll:
		return unversioned("/music/generations/stemsAll"), true
	case domain.MusicActionAddVocals:
		return versioned("/music/generations/addVocals"), true
	case domain.MusicActionAddInstrumental:
		return versioned("/music/generations/addInstrumental"), true
	case domain.MusicActionAddStem:
		return versioned("/music/generations/addStem"), true
	case domain.MusicActionVoice:
		return unversioned("/music/generations/createVoice"), true
	case domain.MusicActionPersona:
		return unversioned("/music/generations/persona"), true
	case domain.MusicActionReplaceSection:
		return versioned("/music/generations/replaceMusic"), true
	case domain.MusicActionRemoveSection:
		return unversioned("/music/generations/removeSection"), true
	case domain.MusicActionCrop:
		return unversioned("/music/generations/crop"), true
	case domain.MusicActionFadeIn:
		return unversioned("/music/generations/fadeIn"), true
	case domain.MusicActionFadeOut:
		return unversioned("/music/generations/fadeOut"), true
	case domain.MusicActionAdjustSpeed:
		return unversioned("/music/generations/adjustSpeed"), true
	case domain.MusicActionConcat:
		return unversioned("/music/generations/concat"), true
	case domain.MusicActionMashup:
		return versioned("/music/generations/mashup"), true
	case domain.MusicActionSample:
		return versioned("/music/generations/sample"), true
	case domain.MusicActionMIDI:
		return unversioned("/music/generations/midi"), true
	case domain.MusicActionAlignedLyrics:
		return unversioned("/music/generations/alignedLyrics"), true
	case domain.MusicActionBPM:
		return unversioned("/music/generations/bpm"), true
	case domain.MusicActionGenerateVideo:
		return unversioned("/music/generations/generateMp4"), true
	case domain.MusicActionExport:
		return unversioned("/music/generations/download"), true
	default:
		return sunoEndpointSpec{}, false
	}
}

func sunoMusicRequest(req domain.ProviderRequest) (domain.MusicRequest, error) {
	if req.Music != nil {
		return *req.Music, nil
	}
	if len(req.Params) == 0 {
		return domain.MusicRequest{}, sunoInvalid("music request is required")
	}
	var music domain.MusicRequest
	if err := json.Unmarshal(req.Params, &music); err != nil {
		return domain.MusicRequest{}, sunoInvalid("invalid music request json")
	}
	return music, nil
}

func sunoRequestBody(req domain.ProviderRequest, music domain.MusicRequest) (string, map[string]any, error) {
	spec, ok := sunoActionSpec(music.Action)
	if !ok {
		return "", nil, sunoInvalid("unsupported Suno action")
	}
	body := map[string]any{}
	if spec.includeModel {
		body["model"] = sunoProviderModel
	}
	if spec.allowCustomModel && strings.TrimSpace(music.CustomModelID) != "" {
		body["custom_model_id"] = strings.TrimSpace(music.CustomModelID)
	} else if spec.allowVersion {
		body["version"] = sunoVersionFromModelCode(req.ModelCode)
	}

	switch music.Action {
	case domain.MusicActionGenerate:
		putBool(body, "custom", music.Custom)
		putBool(body, "instrumental", music.Instrumental)
		putString(body, "prompt", sunoPrompt(music))
		putString(body, "title", music.Title)
		putString(body, "style", music.Style)
		putString(body, "negative_tags", music.NegativeTags)
		putBool(body, "auto_lyrics", music.AutoLyrics)
		putString(body, "persona_id", music.PersonaID)
		putString(body, "vocal_gender", normalizedVocalGender(music.VocalGender))
		putWeights(body, music, "weirdness_constraint")
		putString(body, "variety", music.Variety)
		putBool(body, "max_mode", music.MaxMode)
		putString(body, "audio_format", music.AudioFormat)
		putPositiveInt(body, "duration", music.DurationSec)
	case domain.MusicActionLyrics:
		putString(body, "prompt", music.Prompt)
		putString(body, "lyrics_model", music.LyricsModel)
	case domain.MusicActionInspo:
		body["audio_urls"] = sunoAudioURLs(req, music)
		putString(body, "prompt", music.Prompt)
		putString(body, "title", music.Title)
		putString(body, "tags", sunoTags(music))
		putString(body, "negative_tags", music.NegativeTags)
		putWeights(body, music, "weirdness")
		putString(body, "vocal_gender", normalizedVocalGender(music.VocalGender))
		putBool(body, "auto_lyrics", music.AutoLyrics)
		putString(body, "variety", music.Variety)
		putBool(body, "max_mode", music.MaxMode)
		putString(body, "audio_format", music.AudioFormat)
	case domain.MusicActionSounds:
		putString(body, "prompt", music.Prompt)
		putString(body, "type", music.SoundType)
		putPositiveInt(body, "bpm", music.BPM)
		putString(body, "key", music.Key)
		putString(body, "audio_format", music.AudioFormat)
	case domain.MusicActionUpsampleTags:
		putString(body, "tags", sunoTags(music))
	case domain.MusicActionUpload:
		putString(body, "audioFilePath", sunoAudioURL(req, music))
	case domain.MusicActionUploadCover:
		putString(body, "audio_url", sunoAudioURL(req, music))
		putBool(body, "custom", music.Custom)
		putBool(body, "instrumental", music.Instrumental)
		putString(body, "gpt_description", music.GPTDescription)
		putString(body, "prompt", sunoPrompt(music))
		putString(body, "tags", sunoTags(music))
		putString(body, "title", music.Title)
		putString(body, "negative_tags", music.NegativeTags)
		putWeights(body, music, "weirdness")
		putBool(body, "auto_lyrics", music.AutoLyrics)
		putString(body, "vocal_gender", normalizedVocalGender(music.VocalGender))
		putString(body, "persona_id", music.PersonaID)
		putPositiveInt(body, "duration_s", music.DurationSec)
		putString(body, "variety", music.Variety)
		putBool(body, "max_mode", music.MaxMode)
		putString(body, "audio_format", music.AudioFormat)
	case domain.MusicActionUploadExtend:
		putString(body, "audio_url", sunoAudioURL(req, music))
		putFloat(body, "continue_at", music.ContinueAtSec)
		putString(body, "prompt", sunoPrompt(music))
		putString(body, "tags", sunoTags(music))
		putString(body, "title", music.Title)
		putString(body, "negative_tags", music.NegativeTags)
		putWeights(body, music, "weirdness")
		putBool(body, "auto_lyrics", music.AutoLyrics)
		putString(body, "vocal_gender", normalizedVocalGender(music.VocalGender))
		putString(body, "persona_id", music.PersonaID)
		putPositiveInt(body, "duration_s", music.DurationSec)
		putString(body, "variety", music.Variety)
		putBool(body, "max_mode", music.MaxMode)
		putString(body, "audio_format", music.AudioFormat)
	case domain.MusicActionCreateModel:
		putString(body, "name", music.Name)
		body["audio_urls"] = sunoAudioURLs(req, music)
	case domain.MusicActionExtend:
		putSource(body, music)
		putFloat(body, "continue_at", music.ContinueAtSec)
		putBool(body, "custom", music.Custom)
		putString(body, "prompt", sunoPrompt(music))
		putString(body, "gpt_description", music.GPTDescription)
		putCreativeOperationFields(body, music, true, true, true)
		putPositiveInt(body, "duration_s", music.DurationSec)
	case domain.MusicActionCover:
		putSource(body, music)
		putBool(body, "custom", music.Custom)
		putString(body, "prompt", sunoPrompt(music))
		putString(body, "gpt_description", music.GPTDescription)
		putCreativeOperationFields(body, music, true, true, true)
		putPositiveInt(body, "duration_s", music.DurationSec)
	case domain.MusicActionRemaster:
		putSource(body, music)
		putString(body, "variation_category", music.VariationCategory)
		putString(body, "audio_format", music.AudioFormat)
	case domain.MusicActionStems:
		putSource(body, music)
		putString(body, "stem_type", music.StemType)
		putString(body, "audio_format", music.AudioFormat)
	case domain.MusicActionStemsAll:
		putSource(body, music)
		putString(body, "audio_format", music.AudioFormat)
	case domain.MusicActionAddVocals, domain.MusicActionAddInstrumental, domain.MusicActionAddStem:
		putSource(body, music)
		putBool(body, "custom", music.Custom)
		putString(body, "prompt", sunoPrompt(music))
		putString(body, "gpt_description", music.GPTDescription)
		putCreativeOperationFields(body, music, music.Action != domain.MusicActionAddStem, false, false)
	case domain.MusicActionVoice:
		putString(body, "audio_url", sunoAudioURL(req, music))
	case domain.MusicActionPersona:
		putSource(body, music)
		putString(body, "name", music.Name)
		putString(body, "describe", music.Description)
		putString(body, "styles", music.Styles)
		putFloat(body, "vocal_start_s", music.VocalStartSec)
		putFloat(body, "vocal_end_s", music.VocalEndSec)
	case domain.MusicActionReplaceSection:
		putSource(body, music)
		putString(body, "infill_lyrics", music.InfillLyrics)
		putStartEnd(body, music)
		putString(body, "prompt", music.Prompt)
		putString(body, "title", music.Title)
		putString(body, "tags", sunoTags(music))
		putString(body, "negative_tags", music.NegativeTags)
		putString(body, "variety", music.Variety)
		putBool(body, "max_mode", music.MaxMode)
		putString(body, "audio_format", music.AudioFormat)
	case domain.MusicActionRemoveSection, domain.MusicActionCrop:
		putSource(body, music)
		putStartEnd(body, music)
	case domain.MusicActionFadeIn, domain.MusicActionFadeOut:
		putSource(body, music)
		putPositiveInt(body, "duration_s", music.DurationSec)
		putString(body, "title", music.Title)
	case domain.MusicActionAdjustSpeed:
		putSource(body, music)
		putFloat(body, "speed", music.Speed)
		putBool(body, "keep_pitch", music.KeepPitch)
		putString(body, "title", music.Title)
	case domain.MusicActionConcat:
		putSource(body, music)
		putString(body, "audio_format", music.AudioFormat)
	case domain.MusicActionMashup:
		body["task_ids"] = stripSunoTaskPrefixes(music.SourceTaskIDs)
		if len(music.SourceAudioIndexes) > 0 {
			body["audio_indexes"] = append([]int(nil), music.SourceAudioIndexes...)
		}
		putBool(body, "instrumental", music.Instrumental)
		putBool(body, "custom", music.Custom)
		putString(body, "prompt", sunoPrompt(music))
		putString(body, "gpt_description", music.GPTDescription)
		putCreativeOperationFields(body, music, true, true, true)
	case domain.MusicActionSample:
		putSource(body, music)
		putStartEnd(body, music)
		putBool(body, "instrumental", music.Instrumental)
		putBool(body, "custom", music.Custom)
		putString(body, "prompt", sunoPrompt(music))
		putString(body, "gpt_description", music.GPTDescription)
		putCreativeOperationFields(body, music, true, true, false)
	case domain.MusicActionMIDI, domain.MusicActionAlignedLyrics, domain.MusicActionBPM, domain.MusicActionGenerateVideo:
		putSource(body, music)
	case domain.MusicActionExport:
		putSource(body, music)
		if len(music.Formats) > 0 {
			body["formats"] = append([]string(nil), music.Formats...)
		}
		putString(body, "format", music.Format)
	}
	return spec.path, body, nil
}

func putCreativeOperationFields(body map[string]any, music domain.MusicRequest, includeVocalGender, includeAutoLyrics, includePersona bool) {
	putString(body, "title", music.Title)
	putString(body, "tags", sunoTags(music))
	putString(body, "negative_tags", music.NegativeTags)
	putWeights(body, music, "weirdness")
	if includeVocalGender {
		putString(body, "vocal_gender", normalizedVocalGender(music.VocalGender))
	}
	if includeAutoLyrics {
		putBool(body, "auto_lyrics", music.AutoLyrics)
	}
	if includePersona {
		putString(body, "persona_id", music.PersonaID)
	}
	putString(body, "variety", music.Variety)
	putBool(body, "max_mode", music.MaxMode)
	putString(body, "audio_format", music.AudioFormat)
}

func putSource(body map[string]any, music domain.MusicRequest) {
	putString(body, "task_id", stripSunoTaskPrefix(music.SourceTaskID))
	if music.SourceAudioIndex > 0 {
		body["audio_index"] = music.SourceAudioIndex
	}
}

func putStartEnd(body map[string]any, music domain.MusicRequest) {
	putFloat(body, "start_s", music.StartSec)
	putFloat(body, "end_s", music.EndSec)
}

func putWeights(body map[string]any, music domain.MusicRequest, weirdnessKey string) {
	putFloat(body, "style_weight", music.StyleWeight)
	putFloat(body, weirdnessKey, music.Weirdness)
	putFloat(body, "audio_weight", music.AudioWeight)
}

func putString(body map[string]any, key, value string) {
	if value = strings.TrimSpace(value); value != "" {
		body[key] = value
	}
}

func putBool(body map[string]any, key string, value *bool) {
	if value != nil {
		body[key] = *value
	}
}

func putFloat(body map[string]any, key string, value *float64) {
	if value != nil {
		body[key] = *value
	}
}

func putPositiveInt(body map[string]any, key string, value int) {
	if value > 0 {
		body[key] = value
	}
}

func sunoPrompt(music domain.MusicRequest) string {
	if strings.TrimSpace(music.Lyrics) != "" && music.Custom != nil && *music.Custom {
		return music.Lyrics
	}
	if strings.TrimSpace(music.Prompt) != "" {
		return music.Prompt
	}
	return music.Lyrics
}

func sunoTags(music domain.MusicRequest) string {
	if strings.TrimSpace(music.Tags) != "" {
		return music.Tags
	}
	return music.Style
}

func sunoAudioURL(req domain.ProviderRequest, music domain.MusicRequest) string {
	if strings.TrimSpace(music.AudioURL) != "" {
		return strings.TrimSpace(music.AudioURL)
	}
	return firstInputURL(req.InputURLs)
}

func sunoAudioURLs(req domain.ProviderRequest, music domain.MusicRequest) []string {
	if len(music.AudioURLs) > 0 {
		return cleanInputURLs(music.AudioURLs)
	}
	return cleanInputURLs(req.InputURLs)
}

func stripSunoTaskPrefixes(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if trimmed := stripSunoTaskPrefix(id); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func stripSunoTaskPrefix(id string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(id), sunoTaskPrefix))
}

func prefixSunoTaskID(id string) string {
	id = stripSunoTaskPrefix(id)
	if id == "" {
		return ""
	}
	return sunoTaskPrefix + id
}

func normalizedVocalGender(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "m", "male":
		return "Male"
	case "f", "female":
		return "Female"
	default:
		return strings.TrimSpace(value)
	}
}

func validateSunoTextBounds(music domain.MusicRequest) error {
	if runeLen(music.Prompt) > 3000 {
		return sunoInvalid("Suno prompt exceeds 3000 characters")
	}
	if runeLen(music.GPTDescription) > 3000 {
		return sunoInvalid("Suno gpt_description exceeds 3000 characters")
	}
	if runeLen(music.Lyrics) > 5000 {
		return sunoInvalid("Suno lyrics exceed 5000 characters")
	}
	if runeLen(music.Style) > 1000 || runeLen(music.Tags) > 1000 {
		return sunoInvalid("Suno style exceeds 1000 characters")
	}
	if runeLen(music.Title) > 80 {
		return sunoInvalid("Suno title exceeds 80 characters")
	}
	return nil
}

func validateSunoNumericBounds(music domain.MusicRequest) error {
	for _, value := range []*float64{music.StyleWeight, music.Weirdness, music.AudioWeight} {
		if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 || *value > 1) {
			return sunoInvalid("Suno weight must be between 0 and 1")
		}
	}
	if music.DurationSec != 0 && (music.DurationSec < 10 || music.DurationSec > 360) {
		return sunoInvalid("Suno duration must be between 10 and 360 seconds")
	}
	if music.Speed != nil && (math.IsNaN(*music.Speed) || math.IsInf(*music.Speed, 0) || *music.Speed < 0.25 || *music.Speed > 4) {
		return sunoInvalid("Suno speed must be between 0.25 and 4")
	}
	if music.StartSec != nil && music.EndSec != nil && *music.EndSec <= *music.StartSec {
		return sunoInvalid("Suno end time must be after start time")
	}
	if music.VocalStartSec != nil && music.VocalEndSec != nil && *music.VocalEndSec <= *music.VocalStartSec {
		return sunoInvalid("Suno vocal end time must be after start time")
	}
	if strings.TrimSpace(music.Variety) != "" {
		switch strings.ToLower(strings.TrimSpace(music.Variety)) {
		case "off", "normal", "high", "extra", "max":
		default:
			return sunoInvalid("unsupported Suno variety")
		}
	}
	if strings.TrimSpace(music.AudioFormat) != "" {
		switch strings.ToLower(strings.TrimSpace(music.AudioFormat)) {
		case "mp3", "m4a", "wav":
		default:
			return sunoInvalid("unsupported Suno audio format")
		}
	}
	return nil
}

func validateSunoCustomModeBounds(music domain.MusicRequest) error {
	if boolValue(music.MaxMode) {
		if !sunoActionSupportsMax(music.Action) {
			return sunoInvalid("Suno max_mode is not supported for action")
		}
		switch music.Action {
		case domain.MusicActionGenerate,
			domain.MusicActionUploadCover,
			domain.MusicActionCover,
			domain.MusicActionMashup,
			domain.MusicActionSample,
			domain.MusicActionAddVocals,
			domain.MusicActionAddInstrumental,
			domain.MusicActionAddStem:
			if !boolValue(music.Custom) {
				return sunoInvalid("Suno max_mode requires custom=true")
			}
		case domain.MusicActionExtend:
			if boolExplicitFalse(music.Custom) {
				return sunoInvalid("Suno max_mode requires custom=true")
			}
		}
	}

	switch music.Action {
	case domain.MusicActionGenerate:
		custom := boolValue(music.Custom)
		instrumental := boolValue(music.Instrumental)
		if (!custom || !instrumental) && strings.TrimSpace(sunoPrompt(music)) == "" {
			return sunoInvalid("Suno prompt is required")
		}
	case domain.MusicActionUploadCover:
		if boolExplicitFalse(music.Custom) && strings.TrimSpace(music.GPTDescription) == "" {
			return sunoInvalid("Suno gpt_description is required")
		}
		if boolValue(music.Custom) && !boolValue(music.Instrumental) && strings.TrimSpace(sunoPrompt(music)) == "" {
			return sunoInvalid("Suno prompt is required")
		}
	case domain.MusicActionCover,
		domain.MusicActionAddVocals,
		domain.MusicActionAddInstrumental,
		domain.MusicActionAddStem:
		if boolExplicitFalse(music.Custom) && strings.TrimSpace(music.GPTDescription) == "" {
			return sunoInvalid("Suno gpt_description is required")
		}
	case domain.MusicActionMashup, domain.MusicActionSample:
		if boolExplicitFalse(music.Custom) && strings.TrimSpace(music.GPTDescription) == "" {
			return sunoInvalid("Suno gpt_description is required")
		}
		if boolValue(music.Custom) && !boolValue(music.Instrumental) && strings.TrimSpace(sunoPrompt(music)) == "" {
			return sunoInvalid("Suno prompt is required")
		}
	}
	return nil
}

func sunoActionSupportsMax(action domain.MusicAction) bool {
	switch action {
	case domain.MusicActionGenerate,
		domain.MusicActionInspo,
		domain.MusicActionUploadCover,
		domain.MusicActionUploadExtend,
		domain.MusicActionExtend,
		domain.MusicActionCover,
		domain.MusicActionAddVocals,
		domain.MusicActionAddInstrumental,
		domain.MusicActionAddStem,
		domain.MusicActionMashup,
		domain.MusicActionReplaceSection,
		domain.MusicActionSample:
		return true
	default:
		return false
	}
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func boolExplicitFalse(value *bool) bool {
	return value != nil && !*value
}

func validateSunoActionInputs(req domain.ProviderRequest, music domain.MusicRequest) error {
	switch music.Action {
	case domain.MusicActionGenerate:
		if strings.TrimSpace(sunoPrompt(music)) == "" && (music.Instrumental == nil || !*music.Instrumental) {
			return sunoInvalid("Suno prompt is required")
		}
	case domain.MusicActionLyrics, domain.MusicActionSounds:
		if strings.TrimSpace(music.Prompt) == "" {
			return sunoInvalid("Suno prompt is required")
		}
	case domain.MusicActionUpsampleTags:
		if strings.TrimSpace(sunoTags(music)) == "" {
			return sunoInvalid("Suno tags are required")
		}
	case domain.MusicActionInspo:
		urls := sunoAudioURLs(req, music)
		if len(urls) < 1 || len(urls) > 4 {
			return sunoInvalid("Suno inspo requires 1 to 4 audio URLs")
		}
		return validateSunoURLs(urls)
	case domain.MusicActionCreateModel:
		urls := sunoAudioURLs(req, music)
		if strings.TrimSpace(music.Name) == "" {
			return sunoInvalid("Suno model name is required")
		}
		if len(urls) < 6 || len(urls) > 24 {
			return sunoInvalid("Suno model creation requires 6 to 24 audio URLs")
		}
		return validateSunoURLs(urls)
	case domain.MusicActionUpload, domain.MusicActionUploadCover, domain.MusicActionVoice:
		return validateSunoURLs([]string{sunoAudioURL(req, music)})
	case domain.MusicActionUploadExtend:
		if music.ContinueAtSec == nil || *music.ContinueAtSec <= 0 {
			return sunoInvalid("Suno continue_at is required")
		}
		return validateSunoURLs([]string{sunoAudioURL(req, music)})
	case domain.MusicActionMashup:
		if len(music.SourceTaskIDs) != 2 {
			return sunoInvalid("Suno mashup requires exactly two source tasks")
		}
		if len(music.SourceAudioIndexes) != 0 && len(music.SourceAudioIndexes) != len(music.SourceTaskIDs) {
			return sunoInvalid("Suno mashup source indexes must match task ids")
		}
		for _, id := range music.SourceTaskIDs {
			if stripSunoTaskPrefix(id) == "" {
				return sunoInvalid("Suno source task is required")
			}
		}
		for _, idx := range music.SourceAudioIndexes {
			if idx <= 0 {
				return sunoInvalid("Suno source audio index must be positive")
			}
		}
	case domain.MusicActionExtend:
		if err := validateSunoSingleSource(music); err != nil {
			return err
		}
		if music.ContinueAtSec == nil || *music.ContinueAtSec <= 0 {
			return sunoInvalid("Suno continue_at is required")
		}
	case domain.MusicActionReplaceSection, domain.MusicActionRemoveSection, domain.MusicActionCrop, domain.MusicActionSample:
		if err := validateSunoSingleSource(music); err != nil {
			return err
		}
		if music.StartSec == nil || music.EndSec == nil {
			return sunoInvalid("Suno start and end are required")
		}
	case domain.MusicActionFadeIn, domain.MusicActionFadeOut:
		if err := validateSunoSingleSource(music); err != nil {
			return err
		}
		if music.DurationSec <= 0 {
			return sunoInvalid("Suno fade duration is required")
		}
	case domain.MusicActionAdjustSpeed:
		if err := validateSunoSingleSource(music); err != nil {
			return err
		}
		if music.Speed == nil {
			return sunoInvalid("Suno speed is required")
		}
	case domain.MusicActionExport:
		if err := validateSunoSingleSource(music); err != nil {
			return err
		}
		if len(music.Formats) == 0 && strings.TrimSpace(music.Format) == "" {
			return sunoInvalid("Suno export format is required")
		}
	case domain.MusicActionCover, domain.MusicActionRemaster, domain.MusicActionStems, domain.MusicActionStemsAll,
		domain.MusicActionAddVocals, domain.MusicActionAddInstrumental, domain.MusicActionAddStem,
		domain.MusicActionPersona, domain.MusicActionConcat, domain.MusicActionMIDI,
		domain.MusicActionAlignedLyrics, domain.MusicActionBPM, domain.MusicActionGenerateVideo:
		return validateSunoSingleSource(music)
	}
	return nil
}

func validateSunoSingleSource(music domain.MusicRequest) error {
	if stripSunoTaskPrefix(music.SourceTaskID) == "" {
		return sunoInvalid("Suno source task is required")
	}
	if music.SourceAudioIndex < 0 {
		return sunoInvalid("Suno source audio index must be positive")
	}
	return nil
}

func validateSunoURLs(values []string) error {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || !isHTTPURL(value) {
			return sunoInvalid("Suno audio URL must be HTTP(S)")
		}
	}
	return nil
}

func runeLen(value string) int {
	return len([]rune(value))
}

func sunoInvalid(message string) error {
	return &Error{Class: domain.ProviderErrInvalidRequest, Message: message}
}

type sunoTaskStatusResponse struct {
	Code    int           `json:"code"`
	Data    sunoTaskData  `json:"data"`
	Message string        `json:"message,omitempty"`
	Error   providerError `json:"error,omitempty"`
}

func (r *sunoTaskStatusResponse) UnmarshalJSON(data []byte) error {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return err
	}
	var envelope struct {
		Code    *int          `json:"code"`
		Data    sunoTaskData  `json:"data"`
		Message string        `json:"message,omitempty"`
		Error   providerError `json:"error,omitempty"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	if envelope.Code != nil || keys["data"] != nil {
		if envelope.Code != nil {
			r.Code = *envelope.Code
		} else {
			r.Code = 200
		}
		r.Data = envelope.Data
		r.Message = envelope.Message
		r.Error = envelope.Error
		return nil
	}
	var flat sunoTaskData
	if err := json.Unmarshal(data, &flat); err != nil {
		return err
	}
	r.Code = 200
	r.Data = flat
	return nil
}

type sunoTaskData struct {
	ID            string         `json:"id"`
	Status        string         `json:"status"`
	Cost          float64        `json:"cost,omitempty"`
	CreditsCost   float64        `json:"credits_cost,omitempty"`
	Progress      int            `json:"progress,omitempty"`
	Result        sunoTaskResult `json:"result,omitempty"`
	Created       int64          `json:"created,omitempty"`
	Completed     int64          `json:"completed,omitempty"`
	EstimatedTime int            `json:"estimated_time,omitempty"`
	ActualTime    int            `json:"actual_time,omitempty"`
	Error         providerError  `json:"error,omitempty"`
	Message       string         `json:"message,omitempty"`
	ErrorMessage  string         `json:"error_message,omitempty"`
}

func (d sunoTaskData) providerError() providerError {
	if !d.Error.empty() {
		return d.Error
	}
	if msg := strings.TrimSpace(d.ErrorMessage); msg != "" {
		return providerError{Message: msg}
	}
	if msg := strings.TrimSpace(d.Message); msg != "" {
		return providerError{Message: msg}
	}
	return providerError{}
}

type sunoTaskResult struct {
	Music         []sunoTrack     `json:"music,omitempty"`
	Lyrics        json.RawMessage `json:"lyrics,omitempty"`
	Text          string          `json:"text,omitempty"`
	Title         string          `json:"title,omitempty"`
	Tags          string          `json:"tags,omitempty"`
	UpsampledTags string          `json:"upsampled_tags,omitempty"`
	AudioID       string          `json:"audio_id,omitempty"`
	AudioURL      string          `json:"audio_url,omitempty"`
	Duration      float64         `json:"duration,omitempty"`
	Files         []sunoFile      `json:"files,omitempty"`
	WavURL        string          `json:"wavUrl,omitempty"`
	MP3URL        string          `json:"mp3Url,omitempty"`
	M4AURL        string          `json:"m4aUrl,omitempty"`
	VideoURL      string          `json:"videoUrl,omitempty"`
	Videos        []taskMedia     `json:"videos,omitempty"`
	PersonaID     string          `json:"persona_id,omitempty"`
	ModelID       string          `json:"model_id,omitempty"`
	VoiceID       string          `json:"voice_id,omitempty"`
	Name          string          `json:"name,omitempty"`
	Describe      string          `json:"describe,omitempty"`
	Styles        string          `json:"styles,omitempty"`
	State         string          `json:"state,omitempty"`
	Instruments   json.RawMessage `json:"instruments,omitempty"`
	Alignment     json.RawMessage `json:"alignment,omitempty"`
	WaveformData  json.RawMessage `json:"waveform_data,omitempty"`
	AverageBPM    any             `json:"avg_bpm,omitempty"`
	MinimumBPM    any             `json:"min_bpm,omitempty"`
	MaximumBPM    any             `json:"max_bpm,omitempty"`
}

type sunoTrack struct {
	AudioID             string   `json:"audio_id,omitempty"`
	Status              string   `json:"status,omitempty"`
	Title               string   `json:"title,omitempty"`
	Lyrics              string   `json:"lyrics,omitempty"`
	Tags                string   `json:"tags,omitempty"`
	DisplayTags         string   `json:"display_tags,omitempty"`
	NegativeTags        string   `json:"negative_tags,omitempty"`
	StyleWeight         *float64 `json:"style_weight,omitempty"`
	Weirdness           *float64 `json:"weirdness,omitempty"`
	WeirdnessConstraint *float64 `json:"weirdness_constraint,omitempty"`
	AudioWeight         *float64 `json:"audio_weight,omitempty"`
	Duration            float64  `json:"duration,omitempty"`
	AudioURL            string   `json:"audio_url,omitempty"`
	ImageURL            string   `json:"image_url,omitempty"`
	ImageLargeURL       string   `json:"image_large_url,omitempty"`
	VideoURL            string   `json:"video_url,omitempty"`
}

type sunoFile struct {
	Format string `json:"format,omitempty"`
	URL    string `json:"url,omitempty"`
}

func (r sunoTaskResult) normalized() (domain.MusicResult, []string, string) {
	var result domain.MusicResult
	var outputs []string
	for i, track := range r.Music {
		weirdness := track.Weirdness
		if weirdness == nil {
			weirdness = track.WeirdnessConstraint
		}
		result.Tracks = append(result.Tracks, domain.MusicTrackResult{
			OriginalAudioIndex: i + 1,
			AudioID:            track.AudioID,
			Status:             track.Status,
			Title:              track.Title,
			Lyrics:             track.Lyrics,
			Tags:               track.Tags,
			DisplayTags:        track.DisplayTags,
			NegativeTags:       track.NegativeTags,
			StyleWeight:        track.StyleWeight,
			Weirdness:          weirdness,
			AudioWeight:        track.AudioWeight,
			DurationSec:        track.Duration,
			AudioURL:           strings.TrimSpace(track.AudioURL),
			ImageURL:           strings.TrimSpace(track.ImageURL),
			ImageLargeURL:      strings.TrimSpace(track.ImageLargeURL),
			VideoURL:           strings.TrimSpace(track.VideoURL),
		})
		appendURL(&outputs, track.AudioURL)
		appendURL(&outputs, track.ImageURL)
		appendURL(&outputs, track.ImageLargeURL)
		appendURL(&outputs, track.VideoURL)
	}
	if len(result.Tracks) == 0 && (strings.TrimSpace(r.AudioID) != "" || strings.TrimSpace(r.AudioURL) != "" || r.Duration > 0) {
		result.Tracks = append(result.Tracks, domain.MusicTrackResult{
			OriginalAudioIndex: 1,
			AudioID:            strings.TrimSpace(r.AudioID),
			DurationSec:        r.Duration,
			AudioURL:           strings.TrimSpace(r.AudioURL),
		})
		appendURL(&outputs, r.AudioURL)
	}
	lyrics, text := r.lyricsResults()
	result.Lyrics = lyrics
	if text == "" {
		text = strings.TrimSpace(r.Text)
	}
	if text == "" {
		text = strings.TrimSpace(r.UpsampledTags)
	}
	result.UpsampledTags = strings.TrimSpace(r.UpsampledTags)
	for _, file := range r.Files {
		artifact := domain.MusicArtifactResult{Kind: "file", Format: strings.TrimSpace(file.Format), URL: strings.TrimSpace(file.URL)}
		result.Artifacts = append(result.Artifacts, artifact)
		appendURL(&outputs, file.URL)
	}
	for _, artifact := range []domain.MusicArtifactResult{
		{Kind: "audio", Format: "wav", URL: r.WavURL},
		{Kind: "audio", Format: "mp3", URL: r.MP3URL},
		{Kind: "audio", Format: "m4a", URL: r.M4AURL},
		{Kind: "video", Format: "mp4", URL: r.VideoURL},
	} {
		if strings.TrimSpace(artifact.URL) != "" {
			result.Artifacts = append(result.Artifacts, artifact)
			appendURL(&outputs, artifact.URL)
		}
	}
	for _, video := range r.Videos {
		for _, raw := range video.URL {
			result.Artifacts = append(result.Artifacts, domain.MusicArtifactResult{Kind: "video", URL: strings.TrimSpace(raw)})
			appendURL(&outputs, raw)
		}
	}
	if strings.TrimSpace(r.PersonaID) != "" {
		result.Persona = &domain.MusicPersonaResult{ID: strings.TrimSpace(r.PersonaID), Name: strings.TrimSpace(r.Name), Description: strings.TrimSpace(r.Describe), Styles: strings.TrimSpace(r.Styles)}
	}
	if strings.TrimSpace(r.ModelID) != "" {
		result.Model = &domain.MusicModelResult{ID: strings.TrimSpace(r.ModelID), Name: strings.TrimSpace(r.Name)}
	}
	if strings.TrimSpace(r.VoiceID) != "" {
		result.Voice = &domain.MusicVoiceResult{ID: strings.TrimSpace(r.VoiceID), Name: strings.TrimSpace(r.Name)}
	}
	instruments := sanitizeSunoMIDIInstruments(r.Instruments)
	if strings.TrimSpace(r.State) != "" || len(instruments) > 0 {
		result.MIDI = &domain.MusicMIDIResult{State: strings.TrimSpace(r.State), Instruments: instruments}
	}
	if bpm := r.bpmResult(); bpm != nil {
		result.BPM = bpm
	}
	if len(r.Alignment) > 0 {
		result.Alignment = append(json.RawMessage(nil), r.Alignment...)
	}
	if len(r.WaveformData) > 0 {
		result.Waveform = append(json.RawMessage(nil), r.WaveformData...)
	}
	return result, outputs, text
}

func (r sunoTaskResult) lyricsResults() ([]domain.MusicLyricsResult, string) {
	if len(r.Lyrics) == 0 || string(r.Lyrics) == "null" {
		if strings.TrimSpace(r.Text) == "" {
			return nil, ""
		}
		item := domain.MusicLyricsResult{Title: strings.TrimSpace(r.Title), Text: strings.TrimSpace(r.Text), Tags: strings.TrimSpace(r.Tags)}
		return []domain.MusicLyricsResult{item}, item.Text
	}
	var list []domain.MusicLyricsResult
	if err := json.Unmarshal(r.Lyrics, &list); err == nil {
		text := ""
		if len(list) > 0 {
			text = strings.TrimSpace(list[0].Text)
		}
		return list, text
	}
	var text string
	if err := json.Unmarshal(r.Lyrics, &text); err == nil && strings.TrimSpace(text) != "" {
		item := domain.MusicLyricsResult{Title: strings.TrimSpace(r.Title), Text: strings.TrimSpace(text), Tags: strings.TrimSpace(r.Tags)}
		return []domain.MusicLyricsResult{item}, item.Text
	}
	return nil, ""
}

func (r sunoTaskResult) bpmResult() *domain.MusicBPMResult {
	avg, avgOK := flexFloat(r.AverageBPM)
	minimum, minOK := flexFloat(r.MinimumBPM)
	maximum, maxOK := flexFloat(r.MaximumBPM)
	if !avgOK && !minOK && !maxOK {
		return nil
	}
	return &domain.MusicBPMResult{Average: avg, Minimum: minimum, Maximum: maximum}
}

func flexFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		return v, true
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return 0, false
		}
		return n, true
	}
	return 0, false
}

func sanitizeSunoMIDIInstruments(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	sanitized, ok := sanitizeSunoMIDIValue(value)
	if !ok {
		return nil
	}
	out, err := json.Marshal(sanitized)
	if err != nil {
		return nil
	}
	if string(out) == "null" || string(out) == "[]" || string(out) == "{}" {
		return nil
	}
	return out
}

func sanitizeSunoMIDIValue(value any) (any, bool) {
	switch typed := value.(type) {
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			if sanitized, ok := sanitizeSunoMIDIValue(item); ok {
				out = append(out, sanitized)
			}
		}
		return out, len(out) > 0
	case map[string]any:
		out := map[string]any{}
		for key, item := range typed {
			normalized := strings.ToLower(strings.TrimSpace(key))
			if !sunoMIDIKeyAllowed(normalized) {
				continue
			}
			if sanitized, ok := sanitizeSunoMIDIValue(item); ok {
				out[key] = sanitized
			}
		}
		return out, len(out) > 0
	case float64, bool:
		return typed, true
	default:
		return nil, false
	}
}

func sunoMIDIKeyAllowed(key string) bool {
	switch key {
	case "instrument", "instrument_index", "program", "channel", "track",
		"note", "note_number", "pitch", "velocity",
		"start", "start_time", "end", "end_time", "duration", "time",
		"tick", "ticks", "tempo", "bpm", "notes", "events":
		return true
	default:
		return false
	}
}

func appendURL(out *[]string, value string) {
	if value = strings.TrimSpace(value); value != "" {
		*out = append(*out, value)
	}
}

func sunoMusicResultHasData(result domain.MusicResult) bool {
	return len(result.Tracks) > 0 ||
		len(result.Lyrics) > 0 ||
		len(result.Artifacts) > 0 ||
		result.MIDI != nil ||
		result.Persona != nil ||
		result.Model != nil ||
		result.Voice != nil ||
		result.BPM != nil ||
		strings.TrimSpace(result.UpsampledTags) != "" ||
		len(result.Alignment) > 0 ||
		len(result.Waveform) > 0
}

func sanitizedSunoTaskMetadata(data sunoTaskData) json.RawMessage {
	metadata := map[string]any{
		"id":             data.ID,
		"status":         data.Status,
		"progress":       data.Progress,
		"cost":           data.Cost,
		"credits_cost":   data.CreditsCost,
		"created":        data.Created,
		"completed":      data.Completed,
		"estimated_time": data.EstimatedTime,
		"actual_time":    data.ActualTime,
	}
	if perr := data.providerError(); !perr.empty() {
		class := classifyAPIMartError(0, perr.codeString(), perr.Type, perr.Message)
		metadata["error"] = map[string]any{
			"code":    perr.codeString(),
			"type":    perr.Type,
			"message": providerErrorMessage(class, "apimart task failed"),
		}
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return nil
	}
	return raw
}
