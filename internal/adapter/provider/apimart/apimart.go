// Package apimart implements the APIMart provider adapter.
package apimart

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"vk-ai-aggregator/internal/domain"
)

const (
	DefaultBaseURL = "https://api.apimart.ai/v1"

	ModelHailuo23Standard = "MiniMax-Hailuo-2.3"
	ModelHailuo23Fast     = "MiniMax-Hailuo-2.3-Fast"

	defaultInternalVideoPriceCredits = 2
	defaultTaskLanguage              = "en"
)

// Config holds APIMart connection settings.
type Config struct {
	APIKey                    string
	BaseURL                   string
	InternalVideoPriceCredits int64
	TaskLanguage              string
	HTTPClient                *http.Client
}

// Provider is the APIMart domain.Provider adapter.
type Provider struct {
	cfg        Config
	http       *http.Client
	mu         sync.Mutex
	idempotent map[string]domain.ProviderTask
	now        func() time.Time
}

// New builds an APIMart provider adapter.
func New(cfg Config) *Provider {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if cfg.InternalVideoPriceCredits <= 0 {
		cfg.InternalVideoPriceCredits = defaultInternalVideoPriceCredits
	}
	if strings.TrimSpace(cfg.TaskLanguage) == "" {
		cfg.TaskLanguage = defaultTaskLanguage
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	return &Provider{
		cfg:        cfg,
		http:       httpClient,
		idempotent: map[string]domain.ProviderTask{},
		now:        time.Now,
	}
}

var _ domain.Provider = (*Provider)(nil)

// Name returns the APIMart provider identifier.
func (p *Provider) Name() domain.ProviderName { return domain.ProviderAPIMart }

// Capabilities reports supported APIMart Hailuo routes.
func (p *Provider) Capabilities(context.Context) ([]domain.Capability, error) {
	return []domain.Capability{
		{
			Operation:       domain.OperationVideoGenerate,
			Modality:        domain.ModalityVideo,
			ModelCode:       ModelHailuo23Standard,
			SupportsPolling: true,
			MaxDurationSec:  10,
		},
		{
			Operation:       domain.OperationVideoGenerate,
			Modality:        domain.ModalityVideo,
			ModelCode:       ModelHailuo23Fast,
			SupportsPolling: true,
			MaxDurationSec:  10,
		},
	}, nil
}

// Estimate returns the route-level internal credits used by the worker router.
func (p *Provider) Estimate(_ context.Context, req domain.ProviderRequest) (domain.CostEstimate, error) {
	if err := validateVideoShape(req, false); err != nil {
		return domain.CostEstimate{}, err
	}
	return domain.CostEstimate{
		AmountCredits: p.cfg.InternalVideoPriceCredits,
		Currency:      "credits",
		Estimated:     false,
	}, nil
}

// Submit creates an async APIMart video task.
func (p *Provider) Submit(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	if req.IdempotencyKey != "" {
		if task, ok := p.idempotentTask(req.IdempotencyKey); ok {
			return task, nil
		}
	}
	if err := validateVideoShape(req, true); err != nil {
		return domain.ProviderTask{}, err
	}

	body := videoGenerationRequest{
		Model:            req.ModelCode,
		Prompt:           req.Prompt,
		Duration:         req.DurationSec,
		Resolution:       req.Resolution,
		PromptOptimizer:  true,
		FastPretreatment: false,
		Watermark:        false,
		FirstFrameImage:  firstInputURL(req.InputURLs),
	}
	var decoded submitResponse
	if err := p.postJSON(ctx, "/videos/generations", body, &decoded, req.IdempotencyKey); err != nil {
		return domain.ProviderTask{}, err
	}
	if decoded.Code != 200 {
		return domain.ProviderTask{}, apiEnvelopeError(decoded.Code, decoded.Error, decoded.Message)
	}
	if len(decoded.Data) == 0 || strings.TrimSpace(decoded.Data[0].TaskID) == "" {
		return domain.ProviderTask{}, &Error{Class: domain.ProviderErrInternal, Message: "empty submit task id"}
	}

	now := p.now()
	status := mapTaskStatus(decoded.Data[0].Status)
	if status == "" {
		status = domain.ProviderTaskPending
	}
	task := domain.ProviderTask{
		JobID:          req.JobID,
		Provider:       domain.ProviderAPIMart,
		ModelCode:      req.ModelCode,
		ExternalID:     strings.TrimSpace(decoded.Data[0].TaskID),
		AttemptNo:      1,
		Status:         status,
		Request:        req.Params,
		SubmittedAt:    &now,
		CreatedAt:      now,
		UpdatedAt:      now,
		IdempotencyKey: req.IdempotencyKey,
	}
	if status.IsTerminal() {
		task.CompletedAt = &now
	}
	if req.IdempotencyKey != "" {
		p.storeIdempotentTask(req.IdempotencyKey, task)
	}
	return task, nil
}

// Poll fetches an APIMart task status and returns normalized output URLs only
// when the task has completed. The worker stores those URLs as our artifacts
// before the job can become successful.
func (p *Provider) Poll(ctx context.Context, ref domain.ProviderTaskRef) (domain.ProviderTaskResult, error) {
	taskID := strings.TrimSpace(ref.ExternalID)
	if taskID == "" {
		return domain.ProviderTaskResult{
			Status:     domain.ProviderTaskFailed,
			ErrorClass: domain.ProviderErrTaskNotFound,
		}, nil
	}
	path := "/tasks/" + url.PathEscape(taskID)
	if lang := strings.TrimSpace(p.cfg.TaskLanguage); lang != "" {
		path += "?language=" + url.QueryEscape(lang)
	}
	var decoded taskStatusResponse
	if err := p.getJSON(ctx, path, &decoded); err != nil {
		return domain.ProviderTaskResult{}, err
	}
	if decoded.Code != 200 {
		errValue := decoded.Error
		if errValue.empty() {
			errValue = decoded.Data.Error
		}
		return domain.ProviderTaskResult{}, apiEnvelopeError(decoded.Code, errValue, decoded.Message)
	}
	status := mapTaskStatus(decoded.Data.Status)
	switch status {
	case domain.ProviderTaskSucceeded:
		outputs := decoded.Data.Result.VideoURLs()
		if len(outputs) == 0 {
			return domain.ProviderTaskResult{
				Status:       domain.ProviderTaskFailed,
				ErrorClass:   domain.ProviderErrOutputDownloadFailed,
				ErrorMessage: "apimart task completed without video output",
			}, nil
		}
		raw := sanitizedTaskMetadata(decoded.Data)
		return domain.ProviderTaskResult{Status: status, OutputURLs: outputs, Raw: raw}, nil
	case domain.ProviderTaskFailed:
		class := classifyAPIMartError(0, decoded.Data.Error.codeString(), decoded.Data.Error.Type, decoded.Data.Error.Message)
		return domain.ProviderTaskResult{
			Status:       domain.ProviderTaskFailed,
			ErrorClass:   class,
			ErrorMessage: providerErrorMessage(class, "apimart task failed"),
			Raw:          sanitizedTaskMetadata(decoded.Data),
		}, nil
	case domain.ProviderTaskCancelled:
		return domain.ProviderTaskResult{Status: domain.ProviderTaskCancelled, Raw: sanitizedTaskMetadata(decoded.Data)}, nil
	case domain.ProviderTaskPending, domain.ProviderTaskProcessing:
		return domain.ProviderTaskResult{Status: status, Raw: sanitizedTaskMetadata(decoded.Data)}, nil
	default:
		return domain.ProviderTaskResult{Status: domain.ProviderTaskProcessing, Raw: sanitizedTaskMetadata(decoded.Data)}, nil
	}
}

// Cancel is a no-op until APIMart documents a stable task cancellation endpoint.
func (p *Provider) Cancel(context.Context, domain.ProviderTaskRef) error { return nil }

func (p *Provider) idempotentTask(key string) (domain.ProviderTask, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	task, ok := p.idempotent[key]
	return task, ok
}

func (p *Provider) storeIdempotentTask(key string, task domain.ProviderTask) {
	p.mu.Lock()
	p.idempotent[key] = task
	p.mu.Unlock()
}

type videoGenerationRequest struct {
	Model            string `json:"model"`
	Prompt           string `json:"prompt"`
	Duration         int    `json:"duration,omitempty"`
	Resolution       string `json:"resolution,omitempty"`
	FirstFrameImage  string `json:"first_frame_image,omitempty"`
	PromptOptimizer  bool   `json:"prompt_optimizer"`
	FastPretreatment bool   `json:"fast_pretreatment"`
	Watermark        bool   `json:"watermark"`
}

type submitResponse struct {
	Code    int           `json:"code"`
	Data    []submitData  `json:"data"`
	Message string        `json:"message,omitempty"`
	Error   providerError `json:"error,omitempty"`
}

type submitData struct {
	Status string `json:"status"`
	TaskID string `json:"task_id"`
}

type taskStatusResponse struct {
	Code    int           `json:"code"`
	Data    taskData      `json:"data"`
	Message string        `json:"message,omitempty"`
	Error   providerError `json:"error,omitempty"`
}

type taskData struct {
	ID            string        `json:"id"`
	Status        string        `json:"status"`
	Cost          float64       `json:"cost,omitempty"`
	CreditsCost   float64       `json:"credits_cost,omitempty"`
	Progress      int           `json:"progress,omitempty"`
	Result        taskResult    `json:"result,omitempty"`
	Created       int64         `json:"created,omitempty"`
	Completed     int64         `json:"completed,omitempty"`
	EstimatedTime int           `json:"estimated_time,omitempty"`
	ActualTime    int           `json:"actual_time,omitempty"`
	Error         providerError `json:"error,omitempty"`
}

type taskResult struct {
	Videos []taskMedia `json:"videos,omitempty"`
}

func (r taskResult) VideoURLs() []string {
	var urls []string
	for _, video := range r.Videos {
		for _, raw := range video.URL {
			if trimmed := strings.TrimSpace(raw); trimmed != "" {
				urls = append(urls, trimmed)
			}
		}
	}
	return urls
}

type taskMedia struct {
	URL       urlList `json:"url"`
	ExpiresAt int64   `json:"expires_at,omitempty"`
}

type urlList []string

func (u *urlList) UnmarshalJSON(data []byte) error {
	var values []string
	if err := json.Unmarshal(data, &values); err == nil {
		*u = values
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		if strings.TrimSpace(value) == "" {
			*u = nil
		} else {
			*u = []string{value}
		}
		return nil
	}
	return fmt.Errorf("apimart provider: invalid url field")
}

type providerError struct {
	Code    json.RawMessage `json:"code,omitempty"`
	Message string          `json:"message,omitempty"`
	Type    string          `json:"type,omitempty"`
}

func (e providerError) codeString() string {
	if len(e.Code) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(e.Code, &s); err == nil {
		return s
	}
	var n int64
	if err := json.Unmarshal(e.Code, &n); err == nil {
		return fmt.Sprintf("%d", n)
	}
	return ""
}

func (e providerError) empty() bool {
	return len(e.Code) == 0 && strings.TrimSpace(e.Message) == "" && strings.TrimSpace(e.Type) == ""
}

func validateVideoShape(req domain.ProviderRequest, requirePrompt bool) error {
	if req.Operation != domain.OperationVideoGenerate || req.Modality != domain.ModalityVideo {
		return &Error{Class: domain.ProviderErrUnsupportedCapab, Message: string(req.Operation) + "/" + string(req.Modality)}
	}
	model := strings.TrimSpace(req.ModelCode)
	if !isSupportedModel(model) {
		return &Error{Class: domain.ProviderErrUnsupportedCapab, Message: "unsupported APIMart video model"}
	}
	if requirePrompt {
		prompt := strings.TrimSpace(req.Prompt)
		if prompt == "" {
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "prompt is required"}
		}
		if len([]rune(prompt)) > 2000 {
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "prompt exceeds 2000 characters"}
		}
	}
	if req.DurationSec != 6 && req.DurationSec != 10 {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "duration must be 6 or 10 seconds"}
	}
	resolution := strings.ToLower(strings.TrimSpace(req.Resolution))
	if resolution != "768p" && resolution != "1080p" {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "resolution must be 768p or 1080p"}
	}
	if resolution == "1080p" && req.DurationSec != 6 {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "1080p supports only 6 seconds"}
	}
	if model == ModelHailuo23Fast && len(req.InputURLs) == 0 {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "first frame image is required for Hailuo fast"}
	}
	if len(req.InputURLs) > 1 {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "only one first frame image is supported"}
	}
	return nil
}

func isSupportedModel(model string) bool {
	switch strings.TrimSpace(model) {
	case ModelHailuo23Standard, ModelHailuo23Fast:
		return true
	default:
		return false
	}
}

func firstInputURL(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func (p *Provider) postJSON(ctx context.Context, path string, in, out any, idempotencyKey string) error {
	body, err := json.Marshal(in)
	if err != nil {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: err.Error()}
	}
	req, err := p.request(ctx, http.MethodPost, path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return &Error{Class: domain.ProviderErrTimeout, Message: sanitizeMessage(err.Error())}
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeHTTPError(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return &Error{Class: domain.ProviderErrInternal, Message: "decode response: " + sanitizeMessage(err.Error())}
	}
	return nil
}

func (p *Provider) getJSON(ctx context.Context, path string, out any) error {
	req, err := p.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return &Error{Class: domain.ProviderErrTimeout, Message: sanitizeMessage(err.Error())}
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeHTTPError(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return &Error{Class: domain.ProviderErrInternal, Message: "decode response: " + sanitizeMessage(err.Error())}
	}
	return nil
}

func (p *Provider) request(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, p.cfg.BaseURL+path, body)
	if err != nil {
		return nil, &Error{Class: domain.ProviderErrInternal, Message: sanitizeMessage(err.Error())}
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	return req, nil
}

func decodeHTTPError(resp *http.Response) error {
	msg := fmt.Sprintf("apimart http %d", resp.StatusCode)
	var decoded struct {
		Code    json.RawMessage `json:"code,omitempty"`
		Message string          `json:"message,omitempty"`
		Type    string          `json:"type,omitempty"`
		Error   providerError   `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err == nil {
		if decoded.Error.Message != "" || decoded.Error.Type != "" || len(decoded.Error.Code) > 0 {
			class := classifyAPIMartError(resp.StatusCode, decoded.Error.codeString(), decoded.Error.Type, decoded.Error.Message)
			return &Error{
				Class:   class,
				Message: providerErrorMessage(class, msg),
			}
		}
		code := rawCodeString(decoded.Code)
		if decoded.Message != "" || decoded.Type != "" || code != "" {
			class := classifyAPIMartError(resp.StatusCode, code, decoded.Type, decoded.Message)
			return &Error{
				Class:   class,
				Message: providerErrorMessage(class, msg),
			}
		}
	}
	return &Error{Class: classifyAPIMartError(resp.StatusCode, "", "", ""), Message: msg}
}

func rawCodeString(raw json.RawMessage) string {
	return providerError{Code: raw}.codeString()
}

func apiEnvelopeError(code int, errValue providerError, message string) error {
	msg := message
	if msg == "" {
		msg = errValue.Message
	}
	if msg == "" {
		msg = fmt.Sprintf("apimart api code %d", code)
	}
	class := classifyAPIMartError(code, errValue.codeString(), errValue.Type, msg)
	return &Error{
		Class:   class,
		Message: providerErrorMessage(class, msg),
	}
}

func classifyAPIMartError(status int, code, typ, msg string) domain.ProviderErrorClass {
	lower := strings.ToLower(strings.Join([]string{code, typ, msg}, " "))
	switch {
	case strings.Contains(lower, "balance") || strings.Contains(lower, "insufficient") || strings.Contains(lower, "quota"):
		return domain.ProviderErrInsufficientBalance
	case strings.Contains(lower, "rate") || strings.Contains(lower, "too many"):
		return domain.ProviderErrRateLimited
	case strings.Contains(lower, "auth") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "forbidden") || strings.Contains(lower, "token") || strings.Contains(lower, "permission"):
		return domain.ProviderErrAuthFailed
	case strings.Contains(lower, "moderation") || strings.Contains(lower, "policy") || strings.Contains(lower, "safety") || strings.Contains(lower, "nsfw") || strings.Contains(lower, "sensitive") || strings.Contains(lower, "content rejected"):
		return domain.ProviderErrContentRejected
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "timed out"):
		return domain.ProviderErrTimeout
	case strings.Contains(lower, "unavailable") || strings.Contains(lower, "overload") || strings.Contains(lower, "busy") || strings.Contains(lower, "capacity"):
		return domain.ProviderErrOverloaded
	case strings.Contains(lower, "validation") || strings.Contains(lower, "invalid") || strings.Contains(lower, "bad request"):
		return domain.ProviderErrInvalidRequest
	}
	switch status {
	case http.StatusTooManyRequests:
		return domain.ProviderErrRateLimited
	case http.StatusUnauthorized, http.StatusForbidden:
		return domain.ProviderErrAuthFailed
	case http.StatusPaymentRequired:
		return domain.ProviderErrInsufficientBalance
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return domain.ProviderErrInvalidRequest
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return domain.ProviderErrTimeout
	}
	if status >= 500 {
		return domain.ProviderErrOverloaded
	}
	return domain.ProviderErrInternal
}

func mapTaskStatus(status string) domain.ProviderTaskStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "submitted", "pending", "queued":
		return domain.ProviderTaskPending
	case "processing", "running", "in_progress":
		return domain.ProviderTaskProcessing
	case "completed", "complete", "succeeded", "success":
		return domain.ProviderTaskSucceeded
	case "failed", "error":
		return domain.ProviderTaskFailed
	case "cancelled", "canceled":
		return domain.ProviderTaskCancelled
	default:
		return domain.ProviderTaskProcessing
	}
}

func sanitizedTaskMetadata(data taskData) json.RawMessage {
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
	if data.Error.Message != "" || data.Error.Type != "" || len(data.Error.Code) > 0 {
		class := classifyAPIMartError(0, data.Error.codeString(), data.Error.Type, data.Error.Message)
		metadata["error"] = map[string]any{
			"code":    data.Error.codeString(),
			"type":    data.Error.Type,
			"message": providerErrorMessage(class, "apimart task failed"),
		}
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return nil
	}
	return raw
}

func providerErrorMessage(class domain.ProviderErrorClass, fallback string) string {
	switch class {
	case domain.ProviderErrAuthFailed:
		return "apimart authentication failed"
	case domain.ProviderErrInsufficientBalance:
		return "apimart balance is insufficient"
	case domain.ProviderErrRateLimited:
		return "apimart rate limit exceeded"
	case domain.ProviderErrInvalidRequest:
		return "apimart request validation failed"
	case domain.ProviderErrContentRejected:
		return "apimart content moderation rejected the request"
	case domain.ProviderErrOverloaded:
		return "apimart provider is unavailable"
	case domain.ProviderErrTimeout:
		return "apimart provider timed out"
	case domain.ProviderErrTaskNotFound:
		return "apimart task was not found"
	case domain.ProviderErrOutputDownloadFailed:
		return "apimart completed without a downloadable output"
	}
	if strings.TrimSpace(fallback) == "" {
		return "apimart provider error"
	}
	return sanitizeMessage(fallback)
}

func sanitizeMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	parts := strings.Fields(message)
	for i, part := range parts {
		lower := strings.ToLower(part)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "data:") {
			parts[i] = "[redacted-url]"
		}
	}
	return strings.Join(parts, " ")
}

// Error is an APIMart failure carrying a normalized class.
type Error struct {
	Class   domain.ProviderErrorClass
	Message string
}

func (e *Error) Error() string {
	if e.Message == "" {
		return "apimart provider: " + string(e.Class)
	}
	return "apimart provider: " + string(e.Class) + ": " + e.Message
}

// ProviderErrorClass exposes the normalized class for worker classification.
func (e *Error) ProviderErrorClass() domain.ProviderErrorClass { return e.Class }
