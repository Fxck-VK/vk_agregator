package deepinfra

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

const (
	defaultVideoModel      = "PrunaAI/p-video"
	defaultVideoDuration   = 5
	defaultVideoResolution = "720p"
	defaultVideoAspect     = "16:9"
)

type nativeVideoRequest struct {
	Prompt      string `json:"prompt"`
	Duration    int    `json:"duration,omitempty"`
	Resolution  string `json:"resolution,omitempty"`
	AspectRatio string `json:"aspect_ratio,omitempty"`
	Draft       bool   `json:"draft,omitempty"`
}

type nativeVideoResponse struct {
	VideoURL        string `json:"video_url"`
	RequestID       string `json:"request_id,omitempty"`
	InferenceStatus any    `json:"inference_status,omitempty"`
}

func (p *Provider) submitVideo(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	vreq := req.VideoRequest()
	model := p.cfg.VideoModel
	if vreq.ModelCode != "" {
		model = vreq.ModelCode
	}
	duration := vreq.DurationSec
	if duration <= 0 {
		duration = p.cfg.VideoDurationSec
	}
	if duration < 1 || duration > 10 {
		return domain.ProviderTask{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "video duration must be between 1 and 10 seconds"}
	}
	resolution := strings.TrimSpace(vreq.Resolution)
	if resolution == "" {
		resolution = p.cfg.VideoResolution
	}
	aspectRatio := strings.TrimSpace(vreq.AspectRatio)
	if aspectRatio == "" {
		aspectRatio = p.cfg.VideoAspectRatio
	}
	draft := vreq.Draft || p.cfg.VideoDraft

	result, err := p.generateVideo(ctx, domain.VideoGenerationRequest{
		Prompt:         vreq.Prompt,
		ModelCode:      model,
		DurationSec:    duration,
		Resolution:     resolution,
		AspectRatio:    aspectRatio,
		Draft:          draft,
		IdempotencyKey: vreq.IdempotencyKey,
	})
	if err != nil {
		return domain.ProviderTask{}, err
	}

	now := p.now()
	externalID := "deepinfra-video-" + uuid.NewString()
	res := domain.ProviderTaskResult{
		Status:     domain.ProviderTaskSucceeded,
		OutputURLs: []string{result.OutputURL},
	}
	p.store(externalID, taskState{
		status:     res.Status,
		outputURLs: res.OutputURLs,
	})
	task := domain.ProviderTask{
		JobID:          req.JobID,
		Provider:       domain.ProviderDeepInfra,
		ModelCode:      model,
		ExternalID:     externalID,
		AttemptNo:      1,
		Status:         domain.ProviderTaskSucceeded,
		Request:        req.Params,
		Result:         providerTaskResultRaw(res),
		SubmittedAt:    &now,
		CompletedAt:    &now,
		CreatedAt:      now,
		UpdatedAt:      now,
		IdempotencyKey: req.IdempotencyKey,
	}
	return task, nil
}

func (p *Provider) generateVideo(ctx context.Context, req domain.VideoGenerationRequest) (domain.VideoGenerationResult, error) {
	model := strings.TrimSpace(req.ModelCode)
	if model == "" {
		model = p.cfg.VideoModel
	}
	body := nativeVideoRequest{
		Prompt:      req.Prompt,
		Duration:    req.DurationSec,
		Resolution:  req.Resolution,
		AspectRatio: req.AspectRatio,
		Draft:       req.Draft,
	}
	var decoded nativeVideoResponse
	if err := p.postNativeJSON(ctx, model, body, &decoded, req.IdempotencyKey); err != nil {
		return domain.VideoGenerationResult{}, err
	}
	outputURL := strings.TrimSpace(decoded.VideoURL)
	if outputURL == "" {
		return domain.VideoGenerationResult{}, &Error{Class: domain.ProviderErrInternal, Message: "empty video response"}
	}
	outputURL = p.resolveVideoURL(outputURL)
	metadata := map[string]any{}
	if decoded.RequestID != "" {
		metadata["request_id"] = decoded.RequestID
	}
	if decoded.InferenceStatus != nil {
		metadata["inference_status"] = decoded.InferenceStatus
	}
	result := domain.VideoGenerationResult{
		Provider:  domain.ProviderDeepInfra,
		ModelCode: model,
		OutputURL: outputURL,
	}
	if len(metadata) > 0 {
		if raw, err := json.Marshal(metadata); err == nil {
			result.Metadata = raw
		}
	}
	return result, nil
}

func (p *Provider) resolveVideoURL(raw string) string {
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	base := strings.TrimRight(p.cfg.BaseURL, "/")
	if strings.HasSuffix(base, "/v1/openai") {
		base = strings.TrimSuffix(base, "/openai")
	}
	if !strings.HasSuffix(base, "/v1") {
		base += "/v1"
	}
	hostBase := strings.TrimSuffix(base, "/v1")
	if strings.HasPrefix(raw, "/") {
		return hostBase + raw
	}
	return hostBase + "/" + strings.TrimPrefix(raw, "/")
}
