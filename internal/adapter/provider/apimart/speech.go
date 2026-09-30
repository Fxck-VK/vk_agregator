package apimart

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"vk-ai-aggregator/internal/domain"
)

const ModelGPT4oMiniTTS = "gpt-4o-mini-tts"
const ModelWhisper1 = "whisper-1"

func isSpeechModel(model string) bool { return model == ModelGPT4oMiniTTS || model == ModelWhisper1 }

func validateSpeechRequest(req domain.ProviderRequest) error {
	validRoute := req.ModelCode == ModelGPT4oMiniTTS && req.Operation == domain.OperationAudioTTS && req.Modality == domain.ModalityAudio || req.ModelCode == ModelWhisper1 && req.Operation == domain.OperationAudioSTT && req.Modality == domain.ModalityText
	if !validRoute || req.Speech == nil || domain.ValidateSpeechRequest(*req.Speech, req.Operation, true) != nil || req.Music != nil || req.VideoMedia != nil || len(req.InputURLs) > 0 || len(req.ReferenceArtifactIDs) > 0 || req.Prompt != "" || req.ReferenceVideoURL != "" || strings.TrimSpace(req.IdempotencyKey) == "" {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid speech request"}
	}
	return nil
}

func (p *Provider) submitSpeech(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	if err := validateSpeechRequest(req); err != nil {
		return domain.ProviderTask{}, err
	}
	// Native synchronous endpoints cannot recover an output by task ID. Keep a
	// replay tombstone, not private text/audio bytes, in the process cache. The
	// durable worker intent prevents another HTTP call after a process restart.
	p.mu.Lock()
	if _, exists := p.unversionedSubmits[req.IdempotencyKey]; exists {
		p.mu.Unlock()
		return domain.ProviderTask{}, unversionedSubmitIndeterminate()
	}
	state := &unversionedSubmission{done: make(chan struct{}), err: unversionedSubmitIndeterminate()}
	p.unversionedSubmits[req.IdempotencyKey] = state
	close(state.done)
	p.mu.Unlock()
	return p.submitSpeechHTTP(ctx, req)
}

func (p *Provider) submitSpeechHTTP(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	s := req.Speech
	var body bytes.Buffer
	endpoint, contentType := "/audio/speech", "application/json"
	if req.ModelCode == ModelGPT4oMiniTTS {
		if err := json.NewEncoder(&body).Encode(map[string]any{"model": req.ModelCode, "input": s.Text, "voice": s.Voice, "response_format": s.Format, "speed": s.Speed}); err != nil {
			return domain.ProviderTask{}, err
		}
	} else {
		endpoint = "/audio/transcriptions"
		w := multipart.NewWriter(&body)
		part, err := w.CreateFormFile("file", "input."+s.FileExtension)
		if err != nil {
			return domain.ProviderTask{}, err
		}
		if _, err = part.Write(s.FileBytes); err != nil {
			return domain.ProviderTask{}, err
		}
		for k, v := range map[string]string{"model": req.ModelCode, "response_format": s.Format, "temperature": strconv.FormatFloat(s.Temperature, 'f', -1, 64)} {
			if err := w.WriteField(k, v); err != nil {
				return domain.ProviderTask{}, err
			}
		}
		if s.Language != "" {
			if err := w.WriteField("language", s.Language); err != nil {
				return domain.ProviderTask{}, err
			}
		}
		if err := w.Close(); err != nil {
			return domain.ProviderTask{}, err
		}
		contentType = w.FormDataContentType()
	}
	httpReq, err := p.request(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return domain.ProviderTask{}, err
	}
	httpReq.Header.Set("Content-Type", contentType)
	httpReq.Header.Set("Idempotency-Key", req.IdempotencyKey)
	client := *p.http
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(httpReq)
	if err != nil {
		return domain.ProviderTask{}, unversionedSubmitIndeterminate()
	}
	defer resp.Body.Close()
	if uncertainUnversionedStatus(resp.StatusCode) || resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return domain.ProviderTask{}, unversionedSubmitIndeterminate()
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domain.ProviderTask{}, decodeHTTPError(resp)
	}
	limit := int64(4 << 20)
	if req.ModelCode == ModelGPT4oMiniTTS {
		limit = 64 << 20
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil || int64(len(raw)) > limit || len(raw) == 0 {
		return domain.ProviderTask{}, unversionedSubmitIndeterminate()
	}
	result := domain.ProviderTaskResult{Status: domain.ProviderTaskSucceeded}
	if req.ModelCode == ModelGPT4oMiniTTS {
		mimeType := map[string]string{"wav": "audio/wav", "opus": "audio/ogg", "aac": "audio/aac", "flac": "audio/flac", "pcm": "audio/pcm"}[s.Format]
		if !strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "audio/") && !strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "application/octet-stream") {
			return domain.ProviderTask{}, unversionedSubmitIndeterminate()
		}
		result.InlineAudio = &domain.InlineAudio{Bytes: raw, MIME: mimeType, Extension: s.Format}
	} else if s.Format == "json" || s.Format == "verbose_json" {
		var decoded struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(raw, &decoded) != nil {
			return domain.ProviderTask{}, unversionedSubmitIndeterminate()
		}
		result.Text = decoded.Text
	} else {
		result.Text = string(raw)
	}
	if req.ModelCode == ModelWhisper1 && strings.TrimSpace(result.Text) == "" {
		return domain.ProviderTask{}, &Error{Class: domain.ProviderErrOutputDownloadFailed, Message: "empty transcription"}
	}
	now := p.now()
	return domain.ProviderTask{JobID: req.JobID, Provider: domain.ProviderAPIMart, ModelCode: req.ModelCode, ExternalID: "speech:" + req.JobID.String(), Status: domain.ProviderTaskSucceeded, AttemptNo: 1, IdempotencyKey: req.IdempotencyKey, Request: domain.DurableProviderTaskRequestJSON(), SubmittedAt: &now, CreatedAt: now, UpdatedAt: now, ImmediateResult: &result}, nil
}
