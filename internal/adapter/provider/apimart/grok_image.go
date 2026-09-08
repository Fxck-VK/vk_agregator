package apimart

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"vk-ai-aggregator/internal/domain"
)

func isGrokImageModel(model string) bool {
	return strings.TrimSpace(model) == ModelGrokImage15 || strings.TrimSpace(model) == ModelGrokImage20
}

func isGrokImageSize(model, size string) bool {
	switch size {
	case "1:1", "16:9", "9:16", "3:2", "2:3":
		return true
	case "3:4", "4:3", "1024x1024", "1024x1792", "1792x1024", "720x1280", "1280x720":
		return strings.TrimSpace(model) == ModelGrokImage20
	default:
		return false
	}
}

func validateGrokImageRequest(req domain.ProviderRequest, requirePrompt bool) error {
	invalid := func() error {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported Grok image request options"}
	}
	model := strings.TrimSpace(req.ModelCode)
	if requirePrompt && (strings.TrimSpace(req.Prompt) == "" || len([]rune(req.Prompt)) > 20000) {
		return invalid()
	}
	if req.OutputCount < 0 || req.OutputCount > 1 || strings.TrimSpace(req.NegativePrompt) != "" {
		return invalid()
	}
	for _, size := range []string{req.Size, req.AspectRatio} {
		if size = strings.ToLower(strings.TrimSpace(size)); size != "" && !isGrokImageSize(model, size) {
			return invalid()
		}
	}
	resolution := strings.TrimSpace(req.Resolution)
	if resolution != "" && (model != ModelGrokImage20 || resolution != "quality") {
		return invalid()
	}
	maxRefs := 1
	if model == ModelGrokImage20 {
		maxRefs = 0
	}
	if len(req.InputURLs) > maxRefs {
		return invalid()
	}
	for _, ref := range req.InputURLs {
		// Keep the existing application-side 10 MiB reference safety cap.
		if _, err := validateGenerationImageInput(ref, maxGeminiGenerationImageBytes); err != nil {
			return err
		}
	}
	if model == ModelGrokImage20 {
		if requirePrompt && req.IdempotencyKey == "" {
			return invalid()
		}
		if len(req.IdempotencyKey) > 191 {
			return invalid()
		}
		for _, ch := range req.IdempotencyKey {
			if ch < 33 || ch > 126 {
				return invalid()
			}
		}
	}
	if len(req.Params) == 0 {
		return nil
	}
	var options struct {
		N              *int            `json:"n"`
		Quality        json.RawMessage `json:"quality"`
		Style          json.RawMessage `json:"style"`
		Stream         bool            `json:"stream"`
		ResponseFormat string          `json:"response_format"`
		ImageURLs      json.RawMessage `json:"image_urls"`
		ImageWithRoles json.RawMessage `json:"image_with_roles"`
		PromptExtend   bool            `json:"prompt_extend"`
	}
	if err := json.Unmarshal(req.Params, &options); err != nil {
		return invalid()
	}
	if (options.N != nil && *options.N != 1) || len(options.Quality) > 0 || len(options.Style) > 0 || options.Stream || options.PromptExtend ||
		(options.ResponseFormat != "" && options.ResponseFormat != "url") || len(options.ImageURLs) > 0 || len(options.ImageWithRoles) > 0 {
		return invalid()
	}
	return nil
}

func (p *Provider) submitGrokImage(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	// A dedicated allowlist keeps pixel-quality and editing options from other
	// image providers out of Grok's distinct request contracts.
	body := struct {
		Model          string   `json:"model"`
		Prompt         string   `json:"prompt"`
		Size           string   `json:"size"`
		N              int      `json:"n"`
		Resolution     string   `json:"resolution,omitempty"`
		ResponseFormat string   `json:"response_format,omitempty"`
		ImageURLs      []string `json:"image_urls,omitempty"`
	}{Model: strings.TrimSpace(req.ModelCode), Prompt: strings.TrimSpace(req.Prompt), Size: effectiveImageSize(req), N: 1, ImageURLs: cleanInputURLs(req.InputURLs)}
	if body.Model == ModelGrokImage20 {
		body.Resolution, body.ResponseFormat = "quality", "url"
	}
	var decoded struct {
		Code    int             `json:"code"`
		Data    json.RawMessage `json:"data"`
		Error   providerError   `json:"error"`
		Message string          `json:"message"`
	}
	post := p.postJSON
	if body.Model == ModelGrokImage20 {
		post = p.postGrok20JSON
	}
	if err := post(ctx, "/images/generations", body, &decoded, req.IdempotencyKey); err != nil {
		return domain.ProviderTask{}, err
	}
	if decoded.Code != 200 && !(body.Model == ModelGrokImage20 && decoded.Code == 202) {
		if body.Model == ModelGrokImage20 && (decoded.Code < 400 || decoded.Code == 408 || decoded.Code == 409 || decoded.Code >= 500) {
			return domain.ProviderTask{}, grokSubmitIndeterminate()
		}
		return domain.ProviderTask{}, apiEnvelopeError(decoded.Code, decoded.Error, decoded.Message)
	}
	var id, status string
	if body.Model == ModelGrokImage20 {
		var data struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal(decoded.Data, &data); err == nil {
			id, status = data.ID, data.Status
		}
	} else {
		var data []submitData
		if err := json.Unmarshal(decoded.Data, &data); err == nil && len(data) == 1 {
			id, status = data[0].TaskID, data[0].Status
		}
	}
	if strings.TrimSpace(id) == "" {
		if body.Model == ModelGrokImage20 {
			return domain.ProviderTask{}, grokSubmitIndeterminate()
		}
		return domain.ProviderTask{}, &Error{Class: domain.ProviderErrInternal, Message: "empty Grok submit task id"}
	}
	now := p.now()
	state := mapTaskStatus(status)
	if state == "" {
		state = domain.ProviderTaskPending
	}
	task := domain.ProviderTask{JobID: req.JobID, Provider: domain.ProviderAPIMart, ModelCode: body.Model,
		ExternalID: strings.TrimSpace(id), AttemptNo: 1, Status: state, Request: req.Params,
		SubmittedAt: &now, CreatedAt: now, UpdatedAt: now, IdempotencyKey: req.IdempotencyKey}
	if state.IsTerminal() {
		task.CompletedAt = &now
	}
	return task, nil
}

func grokSubmitIndeterminate() error {
	return &Error{Class: domain.ProviderErrSubmitIndeterminate, Message: "apimart submit outcome unresolved; automatic resubmission stopped"}
}

// Replay uncertain submissions with identical bytes, key and response version.
// If replay cannot recover the task, fail closed instead of letting the worker
// increment its attempt and create a new paid intent.
func (p *Provider) postGrok20JSON(ctx context.Context, path string, in, out any, key string) error {
	body, err := json.Marshal(in)
	if err != nil {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid Grok request"}
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for attempt := 0; attempt < 3; attempt++ {
		req, err := p.request(ctx, http.MethodPost, path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Idempotency-Key", key)
		req.Header.Set("X-APIMart-Response-Version", "2026-07-27")
		delay := time.Second
		resp, err := p.http.Do(req)
		if err == nil {
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				err = json.NewDecoder(resp.Body).Decode(out)
				_ = resp.Body.Close()
				if err != nil {
					return grokSubmitIndeterminate()
				}
				return nil
			}
			if value := resp.Header.Get("Retry-After"); value != "" {
				if seconds, parseErr := strconv.ParseUint(value, 10, 31); parseErr == nil {
					delay = time.Duration(seconds) * time.Second
				} else if when, parseErr := http.ParseTime(value); parseErr == nil && when.After(p.now()) {
					delay = when.Sub(p.now())
				}
			}
			if resp.StatusCode == http.StatusConflict {
				var conflict struct {
					Error providerError `json:"error"`
				}
				_ = json.NewDecoder(resp.Body).Decode(&conflict)
				_ = resp.Body.Close()
				if conflict.Error.codeString() == "idempotency_key_reused" {
					return &Error{Class: domain.ProviderErrInvalidRequest, Message: "apimart idempotency key conflict"}
				}
				if conflict.Error.codeString() != "idempotency_in_progress" {
					return grokSubmitIndeterminate()
				}
			} else if resp.StatusCode < 500 && resp.StatusCode != http.StatusRequestTimeout && resp.StatusCode != http.StatusTooManyRequests {
				err = decodeHTTPError(resp)
				_ = resp.Body.Close()
				return err
			} else {
				_ = resp.Body.Close()
			}
		}
		if attempt == 2 {
			break
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return grokSubmitIndeterminate()
		case <-timer.C:
		}
	}
	return grokSubmitIndeterminate()
}
