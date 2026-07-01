// Package runway implements the official Runway API provider adapter.
package runway

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
	DefaultBaseURL    = "https://api.dev.runwayml.com/v1"
	DefaultAPIVersion = "2024-11-06"

	ModelGen4Turbo = "gen4_turbo"

	gen4TurboCreditsPerSecond int64 = 5

	maxReferenceImages = 1
)

// Config holds official Runway API connection settings.
type Config struct {
	APISecret  string
	BaseURL    string
	APIVersion string
	HTTPClient *http.Client
}

// Provider is the official Runway domain.Provider adapter.
type Provider struct {
	cfg        Config
	http       *http.Client
	mu         sync.Mutex
	idempotent map[string]domain.ProviderTask
	now        func() time.Time
}

// New builds a Runway provider adapter.
func New(cfg Config) *Provider {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if strings.TrimSpace(cfg.APIVersion) == "" {
		cfg.APIVersion = DefaultAPIVersion
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

// Name returns the official Runway provider identifier.
func (p *Provider) Name() domain.ProviderName { return domain.ProviderRunway }

// Capabilities reports supported official Runway routes.
func (p *Provider) Capabilities(context.Context) ([]domain.Capability, error) {
	return []domain.Capability{
		{
			Operation:       domain.OperationVideoGenerate,
			Modality:        domain.ModalityVideo,
			ModelCode:       ModelGen4Turbo,
			SupportsPolling: true,
			MaxDurationSec:  10,
		},
	}, nil
}

// Estimate reports provider-side credits for worker routing, media safety caps
// and telemetry. User billing must use pricingcatalog snapshots, never adapter
// estimates.
func (p *Provider) Estimate(_ context.Context, req domain.ProviderRequest) (domain.CostEstimate, error) {
	snapshot, hasSnapshot, err := resolvedRouteSnapshot(req.Params)
	if err != nil {
		return domain.CostEstimate{}, err
	}
	if hasSnapshot {
		if snapshot.Provider != domain.ProviderRunway || strings.TrimSpace(snapshot.ProviderModelID) != strings.TrimSpace(req.ModelCode) {
			return domain.CostEstimate{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "resolved route snapshot does not match Runway request"}
		}
		if snapshot.ProviderCostCredits <= 0 {
			return domain.CostEstimate{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "resolved route snapshot provider cost is unavailable"}
		}
		return domain.CostEstimate{AmountCredits: snapshot.ProviderCostCredits, Currency: "credits", Estimated: false}, nil
	}
	if err := validateVideoShape(req); err != nil {
		return domain.CostEstimate{}, err
	}
	return domain.CostEstimate{
		AmountCredits: int64(effectiveDuration(req.DurationSec)) * gen4TurboCreditsPerSecond,
		Currency:      "credits",
		Estimated:     false,
	}, nil
}

// Submit creates an async official Runway image-to-video task.
func (p *Provider) Submit(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	if req.IdempotencyKey != "" {
		if task, ok := p.idempotentTask(req.IdempotencyKey); ok {
			return task, nil
		}
	}
	body, err := buildImageToVideoRequest(req)
	if err != nil {
		return domain.ProviderTask{}, err
	}
	var decoded taskResponse
	if err := p.postJSON(ctx, "/image_to_video", body, &decoded); err != nil {
		return domain.ProviderTask{}, err
	}
	if strings.TrimSpace(decoded.ID) == "" {
		return domain.ProviderTask{}, &Error{Class: domain.ProviderErrInternal, Message: "empty submit task id"}
	}

	now := p.now()
	status := mapTaskStatus(decoded.Status)
	if status == "" {
		status = domain.ProviderTaskPending
	}
	task := domain.ProviderTask{
		JobID:          req.JobID,
		Provider:       domain.ProviderRunway,
		ModelCode:      req.ModelCode,
		ExternalID:     strings.TrimSpace(decoded.ID),
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

// Poll fetches Runway task status. Output URLs are returned only to the worker,
// which stores them as our artifacts before the job can succeed.
func (p *Provider) Poll(ctx context.Context, ref domain.ProviderTaskRef) (domain.ProviderTaskResult, error) {
	taskID := strings.TrimSpace(ref.ExternalID)
	if taskID == "" {
		return domain.ProviderTaskResult{
			Status:     domain.ProviderTaskFailed,
			ErrorClass: domain.ProviderErrTaskNotFound,
		}, nil
	}
	var decoded taskResponse
	if err := p.getJSON(ctx, "/tasks/"+url.PathEscape(taskID), &decoded); err != nil {
		return domain.ProviderTaskResult{}, err
	}
	status := mapTaskStatus(decoded.Status)
	switch status {
	case domain.ProviderTaskSucceeded:
		outputs := cleanStrings(decoded.Output)
		if len(outputs) == 0 {
			return domain.ProviderTaskResult{
				Status:       domain.ProviderTaskFailed,
				ErrorClass:   domain.ProviderErrOutputDownloadFailed,
				ErrorMessage: "runway task completed without video output",
				Raw:          sanitizedTaskMetadata(decoded),
			}, nil
		}
		return domain.ProviderTaskResult{Status: status, OutputURLs: outputs, Raw: sanitizedTaskMetadata(decoded)}, nil
	case domain.ProviderTaskFailed:
		class := classifyRunwayError(0, decoded.FailureCode, decoded.Failure)
		return domain.ProviderTaskResult{
			Status:       domain.ProviderTaskFailed,
			ErrorClass:   class,
			ErrorMessage: providerErrorMessage(class, "runway task failed"),
			Raw:          sanitizedTaskMetadata(decoded),
		}, nil
	case domain.ProviderTaskCancelled:
		return domain.ProviderTaskResult{Status: domain.ProviderTaskCancelled, Raw: sanitizedTaskMetadata(decoded)}, nil
	case domain.ProviderTaskPending, domain.ProviderTaskProcessing:
		return domain.ProviderTaskResult{Status: status, Raw: sanitizedTaskMetadata(decoded)}, nil
	default:
		return domain.ProviderTaskResult{Status: domain.ProviderTaskProcessing, Raw: sanitizedTaskMetadata(decoded)}, nil
	}
}

// Cancel requests Runway task cancellation/deletion. Official docs say 404 can
// be ignored for idempotency when a task is already deleted or aborted.
func (p *Provider) Cancel(ctx context.Context, ref domain.ProviderTaskRef) error {
	taskID := strings.TrimSpace(ref.ExternalID)
	if taskID == "" {
		return nil
	}
	req, err := p.request(ctx, http.MethodDelete, "/tasks/"+url.PathEscape(taskID), nil)
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
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeHTTPError(resp)
	}
	return nil
}

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

type imageToVideoRequest struct {
	Model       string `json:"model"`
	PromptImage string `json:"promptImage"`
	PromptText  string `json:"promptText,omitempty"`
	Ratio       string `json:"ratio"`
	Duration    int    `json:"duration,omitempty"`
}

type taskResponse struct {
	ID          string   `json:"id"`
	Status      string   `json:"status"`
	CreatedAt   string   `json:"createdAt,omitempty"`
	Progress    float64  `json:"progress,omitempty"`
	Output      []string `json:"output,omitempty"`
	Failure     string   `json:"failure,omitempty"`
	FailureCode string   `json:"failureCode,omitempty"`
}

type requestParams struct {
	ResolvedVideoRoute domain.VideoRouteSnapshot `json:"resolved_video_route,omitempty"`
	Ratio              string                    `json:"ratio,omitempty"`
}

func buildImageToVideoRequest(req domain.ProviderRequest) (imageToVideoRequest, error) {
	if err := validateVideoShape(req); err != nil {
		return imageToVideoRequest{}, err
	}
	return imageToVideoRequest{
		Model:       ModelGen4Turbo,
		PromptImage: cleanStrings(req.InputURLs)[0],
		PromptText:  strings.TrimSpace(req.Prompt),
		Ratio:       runwayRatio(req.AspectRatio),
		Duration:    effectiveDuration(req.DurationSec),
	}, nil
}

func validateVideoShape(req domain.ProviderRequest) error {
	if req.Operation != domain.OperationVideoGenerate || req.Modality != domain.ModalityVideo {
		return &Error{Class: domain.ProviderErrUnsupportedCapab, Message: string(req.Operation) + "/" + string(req.Modality)}
	}
	if strings.TrimSpace(req.ModelCode) != ModelGen4Turbo {
		return &Error{Class: domain.ProviderErrUnsupportedCapab, Message: "unsupported Runway video model"}
	}
	if strings.TrimSpace(req.Prompt) != "" && len([]rune(strings.TrimSpace(req.Prompt))) > 1000 {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "prompt exceeds 1000 characters"}
	}
	if _, hasSnapshot, err := resolvedRouteSnapshot(req.Params); err != nil {
		return err
	} else if !hasSnapshot && providerNativeRatioRequested(req.Params) {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "provider-native ratio is not accepted"}
	}
	duration := effectiveDuration(req.DurationSec)
	if duration < 2 || duration > 10 {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "duration must be 2-10 seconds"}
	}
	if !allowedResolution(req.Resolution) {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported Runway video resolution"}
	}
	if !allowedAspectRatio(req.AspectRatio) {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported Runway video aspect ratio"}
	}
	inputs := cleanStrings(req.InputURLs)
	if len(inputs) == 0 {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "prompt image is required for Runway Gen-4 Turbo"}
	}
	if len(inputs) > maxReferenceImages {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "Runway Gen-4 Turbo accepts one prompt image"}
	}
	return nil
}

func providerNativeRatioRequested(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var params requestParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return false
	}
	return strings.TrimSpace(params.Ratio) != ""
}

func resolvedRouteSnapshot(raw json.RawMessage) (domain.VideoRouteSnapshot, bool, error) {
	if len(raw) == 0 {
		return domain.VideoRouteSnapshot{}, false, nil
	}
	var params requestParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return domain.VideoRouteSnapshot{}, false, &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid Runway params json"}
	}
	if !params.ResolvedVideoRoute.Valid() {
		return domain.VideoRouteSnapshot{}, false, nil
	}
	return params.ResolvedVideoRoute, true, nil
}

func effectiveDuration(value int) int {
	if value > 0 {
		return value
	}
	return 5
}

func allowedResolution(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "" || value == "720p"
}

func allowedAspectRatio(value string) bool {
	switch strings.TrimSpace(value) {
	case "", "16:9", "9:16", "4:3", "3:4", "1:1", "21:9":
		return true
	default:
		return false
	}
}

func runwayRatio(aspect string) string {
	switch strings.TrimSpace(aspect) {
	case "9:16":
		return "720:1280"
	case "4:3":
		return "1104:832"
	case "3:4":
		return "832:1104"
	case "1:1":
		return "960:960"
	case "21:9":
		return "1584:672"
	default:
		return "1280:720"
	}
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func (p *Provider) postJSON(ctx context.Context, path string, in, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: err.Error()}
	}
	req, err := p.request(ctx, http.MethodPost, path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
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
	req.Header.Set("Authorization", "Bearer "+p.cfg.APISecret)
	req.Header.Set("X-Runway-Version", p.cfg.APIVersion)
	return req, nil
}

func decodeHTTPError(resp *http.Response) error {
	msg := fmt.Sprintf("runway http %d", resp.StatusCode)
	var decoded struct {
		Error       string `json:"error,omitempty"`
		Message     string `json:"message,omitempty"`
		Failure     string `json:"failure,omitempty"`
		FailureCode string `json:"failureCode,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err == nil {
		message := firstNonEmpty(decoded.Error, decoded.Message, decoded.Failure)
		class := classifyRunwayError(resp.StatusCode, decoded.FailureCode, message)
		return &Error{Class: class, Message: providerErrorMessage(class, msg)}
	}
	return &Error{Class: classifyRunwayError(resp.StatusCode, "", ""), Message: msg}
}

func classifyRunwayError(status int, code, msg string) domain.ProviderErrorClass {
	lower := strings.ToLower(strings.Join([]string{code, msg}, " "))
	switch {
	case strings.Contains(lower, "safety") ||
		strings.Contains(lower, "moderation") ||
		strings.Contains(lower, "policy") ||
		strings.Contains(lower, "content rejected") ||
		strings.Contains(lower, "nsfw") ||
		strings.Contains(lower, "copyright") ||
		strings.Contains(lower, "filtered out") ||
		strings.Contains(lower, "blocked by") ||
		strings.Contains(lower, "prohibited") ||
		strings.Contains(lower, "violat"):
		return domain.ProviderErrContentRejected
	case isModelUnavailableError(lower):
		return domain.ProviderErrModelUnavailable
	case strings.Contains(lower, "balance") || strings.Contains(lower, "credit") || strings.Contains(lower, "quota") || strings.Contains(lower, "insufficient"):
		return domain.ProviderErrInsufficientBalance
	case strings.Contains(lower, "rate") || strings.Contains(lower, "throttle") || strings.Contains(lower, "too many"):
		return domain.ProviderErrRateLimited
	case strings.Contains(lower, "auth") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "forbidden") || strings.Contains(lower, "key"):
		return domain.ProviderErrAuthFailed
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "timed out"):
		return domain.ProviderErrTimeout
	case strings.Contains(lower, "invalid") || strings.Contains(lower, "validation") || strings.Contains(lower, "bad request"):
		return domain.ProviderErrInvalidRequest
	case strings.Contains(lower, "overload") || strings.Contains(lower, "unavailable") || strings.Contains(lower, "internal"):
		return domain.ProviderErrOverloaded
	}
	switch status {
	case http.StatusTooManyRequests:
		return domain.ProviderErrRateLimited
	case http.StatusUnauthorized, http.StatusForbidden:
		return domain.ProviderErrAuthFailed
	case http.StatusPaymentRequired:
		return domain.ProviderErrInsufficientBalance
	case http.StatusNotFound:
		return domain.ProviderErrTaskNotFound
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

func isModelUnavailableError(lower string) bool {
	normalized := strings.NewReplacer("_", " ", "-", " ").Replace(lower)
	for _, phrase := range []string{
		"model not found",
		"unknown model",
		"model unknown",
		"unsupported model",
		"model unsupported",
		"model unavailable",
		"model not available",
		"model does not exist",
		"model doesn't exist",
		"model doesnt exist",
		"model not exist",
		"invalid model",
		"model invalid",
	} {
		if strings.Contains(normalized, phrase) {
			return true
		}
	}
	if !strings.Contains(normalized, "model") {
		return false
	}
	for _, phrase := range []string{
		"not found",
		"not available",
		"unavailable",
		"does not exist",
		"doesn't exist",
		"doesnt exist",
		"not exist",
	} {
		if strings.Contains(normalized, phrase) {
			return true
		}
	}
	return false
}

func mapTaskStatus(status string) domain.ProviderTaskStatus {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "PENDING", "THROTTLED":
		return domain.ProviderTaskPending
	case "RUNNING":
		return domain.ProviderTaskProcessing
	case "SUCCEEDED":
		return domain.ProviderTaskSucceeded
	case "FAILED":
		return domain.ProviderTaskFailed
	case "CANCELLED", "CANCELED":
		return domain.ProviderTaskCancelled
	default:
		return domain.ProviderTaskProcessing
	}
}

func sanitizedTaskMetadata(data taskResponse) json.RawMessage {
	metadata := map[string]any{
		"id":        data.ID,
		"status":    data.Status,
		"createdAt": data.CreatedAt,
	}
	if data.Progress > 0 {
		metadata["progress"] = data.Progress
	}
	if strings.TrimSpace(data.FailureCode) != "" {
		class := classifyRunwayError(0, data.FailureCode, data.Failure)
		metadata["error"] = map[string]any{
			"code":    data.FailureCode,
			"message": providerErrorMessage(class, "runway task failed"),
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
		return "runway authentication failed"
	case domain.ProviderErrInsufficientBalance:
		return "runway balance is insufficient"
	case domain.ProviderErrRateLimited:
		return "runway rate limit exceeded"
	case domain.ProviderErrInvalidRequest:
		return "runway request validation failed"
	case domain.ProviderErrModelUnavailable:
		return "runway model is unavailable"
	case domain.ProviderErrContentRejected:
		return "runway content moderation rejected the request"
	case domain.ProviderErrOverloaded:
		return "runway provider is unavailable"
	case domain.ProviderErrTimeout:
		return "runway provider timed out"
	case domain.ProviderErrTaskNotFound:
		return "runway task was not found"
	case domain.ProviderErrOutputDownloadFailed:
		return "runway completed without a downloadable output"
	}
	if strings.TrimSpace(fallback) == "" {
		return "runway provider error"
	}
	return sanitizeMessage(fallback)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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

// Error is a Runway failure carrying a normalized class.
type Error struct {
	Class   domain.ProviderErrorClass
	Message string
}

func (e *Error) Error() string {
	if e.Message == "" {
		return "runway provider: " + string(e.Class)
	}
	return "runway provider: " + string(e.Class) + ": " + e.Message
}

// ProviderErrorClass exposes the normalized class for worker classification.
func (e *Error) ProviderErrorClass() domain.ProviderErrorClass { return e.Class }
