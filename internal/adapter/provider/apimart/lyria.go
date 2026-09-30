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

const ModelLyria35 = "flowmusic-lyria-3.5"
const lyriaTaskPrefix = "flowmusic:"

func validateLyriaRequest(req domain.ProviderRequest) error {
	if req.ModelCode != ModelLyria35 || req.Operation != domain.OperationAudioMusic || req.Modality != domain.ModalityAudio || req.Music == nil || domain.ValidateLyriaMusicRequest(*req.Music) != nil || len(req.InputURLs) != 0 || len(req.ReferenceArtifactIDs) != 0 || req.VideoMedia != nil {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid Lyria request"}
	}
	return nil
}

func (p *Provider) submitLyria(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	if err := validateLyriaRequest(req); err != nil {
		return domain.ProviderTask{}, err
	}
	return p.submitUnversionedOnce(ctx, req, p.submitLyriaTask)
}

func (p *Provider) submitLyriaTask(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	m := req.Music
	sound := strings.TrimSpace(m.Prompt)
	if style := strings.TrimSpace(m.Style); style != "" {
		if sound != "" {
			sound += "\n"
		}
		sound += style
	}
	body := map[string]any{"model": "flowmusic", "version": "lyria-3.5"}
	putString(body, "sound_prompt", sound)
	putString(body, "lyrics", m.Lyrics)
	putString(body, "title", m.Title)
	putPositiveInt(body, "length", m.DurationSec)
	if m.BPM > 0 {
		body["bpm"] = strconv.Itoa(m.BPM)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return domain.ProviderTask{}, err
	}
	req.Params = domain.DurableProviderTaskRequestJSON()
	task, err := p.postUnversionedTask(ctx, req, "/music/generations", raw)
	if err == nil {
		task.ExternalID = lyriaTaskPrefix + task.ExternalID
	}
	return task, err
}

func (p *Provider) pollLyria(ctx context.Context, ref domain.ProviderTaskRef) (domain.ProviderTaskResult, error) {
	id := strings.TrimPrefix(ref.ExternalID, lyriaTaskPrefix)
	if id == "" {
		return domain.ProviderTaskResult{}, &Error{Class: domain.ProviderErrTaskNotFound, Message: "music task missing"}
	}
	var decoded struct {
		Code    int           `json:"code"`
		Error   providerError `json:"error"`
		Message string        `json:"message"`
		Data    struct {
			Status string        `json:"status"`
			Error  providerError `json:"error"`
			Result struct {
				Music []struct {
					ClipID   string      `json:"clip_id"`
					Title    string      `json:"title"`
					Duration json.Number `json:"duration_seconds"`
					AudioURL string      `json:"audio_url"`
					WAVURL   string      `json:"wav_url"`
				} `json:"music"`
			} `json:"result"`
		} `json:"data"`
	}
	// Flow Music documents four languages; ru is not one of them.
	if err := p.getJSON(ctx, "/music/tasks/"+url.PathEscape(id)+"?language=en", &decoded); err != nil {
		return domain.ProviderTaskResult{}, err
	}
	if decoded.Code != 200 {
		return domain.ProviderTaskResult{}, apiEnvelopeError(decoded.Code, decoded.Error, decoded.Message)
	}
	result := domain.ProviderTaskResult{Status: mapTaskStatus(decoded.Data.Status)}
	if result.Status == domain.ProviderTaskFailed {
		e := decoded.Data.Error
		result.ErrorClass = classifyAPIMartError(0, e.codeString(), e.Type, e.Message)
		return result, nil
	}
	if result.Status != domain.ProviderTaskSucceeded {
		return result, nil
	}
	if len(decoded.Data.Result.Music) != 1 {
		return invalidLyriaOutput(), nil
	}
	m := decoded.Data.Result.Music[0]
	duration, err := m.Duration.Float64()
	if err != nil || math.IsNaN(duration) || math.IsInf(duration, 0) || duration <= 0 || !isHTTPURL(m.AudioURL) {
		return invalidLyriaOutput(), nil
	}
	result.Music = &domain.MusicResult{Tracks: []domain.MusicTrackResult{{OriginalAudioIndex: 1, AudioID: m.ClipID, Title: m.Title, DurationSec: duration, AudioURL: m.AudioURL}}}
	result.OutputURLs = []string{m.AudioURL}
	if m.WAVURL != "" {
		if !isHTTPURL(m.WAVURL) {
			return invalidLyriaOutput(), nil
		}
		result.Music.Artifacts = []domain.MusicArtifactResult{{Kind: "audio", Format: "wav", OriginalAudioIndex: 1, URL: m.WAVURL}}
		result.OutputURLs = append(result.OutputURLs, m.WAVURL)
	}
	return result, nil
}

func invalidLyriaOutput() domain.ProviderTaskResult {
	return domain.ProviderTaskResult{Status: domain.ProviderTaskFailed, ErrorClass: domain.ProviderErrOutputDownloadFailed, ErrorMessage: "invalid Lyria result"}
}
