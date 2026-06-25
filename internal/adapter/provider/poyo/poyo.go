// Package poyo implements the PoYo video provider adapter.
package poyo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"vk-ai-aggregator/internal/domain"
)

const (
	DefaultBaseURL = "https://api.poyo.ai"

	ModelKlingO3Standard = "kling-o3/standard"
	ModelSeedance20Fast  = "seedance-2-fast"
	ModelRunwayGen45     = "runway-gen-4.5"
	ModelNanoBanana2New  = "nano-banana-2"

	klingO3CreditsPerSecond    int64 = 10
	seedance20CreditsPerSecond int64 = 28

	maxKlingReferenceImages       = 4
	maxSeedanceReferenceImages    = 4
	maxRunwayReferenceImages      = 1
	maxNanoBanana2ReferenceImages = 14
)

// Config holds PoYo connection settings.
type Config struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

// Provider is the PoYo domain.Provider adapter.
type Provider struct {
	cfg        Config
	http       *http.Client
	mu         sync.Mutex
	idempotent map[string]domain.ProviderTask
	now        func() time.Time
}

// New builds a PoYo provider adapter.
func New(cfg Config) *Provider {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
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

// Name returns the PoYo provider identifier.
func (p *Provider) Name() domain.ProviderName { return domain.ProviderPoYo }

// Capabilities reports supported PoYo video routes.
func (p *Provider) Capabilities(context.Context) ([]domain.Capability, error) {
	return []domain.Capability{
		{
			Operation:       domain.OperationImageGenerate,
			Modality:        domain.ModalityImage,
			ModelCode:       ModelNanoBanana2New,
			SupportsPolling: true,
		},
		{
			Operation:       domain.OperationVideoGenerate,
			Modality:        domain.ModalityVideo,
			ModelCode:       ModelKlingO3Standard,
			SupportsPolling: true,
			MaxDurationSec:  10,
		},
		{
			Operation:       domain.OperationVideoGenerate,
			Modality:        domain.ModalityVideo,
			ModelCode:       ModelSeedance20Fast,
			SupportsPolling: true,
			MaxDurationSec:  10,
		},
		{
			Operation:       domain.OperationVideoGenerate,
			Modality:        domain.ModalityVideo,
			ModelCode:       ModelRunwayGen45,
			SupportsPolling: true,
			MaxDurationSec:  10,
		},
	}, nil
}

// Estimate reports provider-side credits for worker routing, media safety caps
// and telemetry. User billing must use pricingcatalog snapshots, never adapter
// estimates. Runway Gen-4.5 stays fail-closed until provider cost is configured.
func (p *Provider) Estimate(_ context.Context, req domain.ProviderRequest) (domain.CostEstimate, error) {
	snapshot, hasSnapshot, err := resolvedRouteSnapshot(req.Params)
	if err != nil {
		return domain.CostEstimate{}, err
	}
	if hasSnapshot {
		if snapshot.Provider != domain.ProviderPoYo || strings.TrimSpace(snapshot.ProviderModelID) != strings.TrimSpace(req.ModelCode) {
			return domain.CostEstimate{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "resolved route snapshot does not match PoYo request"}
		}
		if snapshot.ProviderCostCredits <= 0 {
			return domain.CostEstimate{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "resolved route snapshot provider cost is unavailable"}
		}
		return domain.CostEstimate{AmountCredits: snapshot.ProviderCostCredits, Currency: "credits", Estimated: false}, nil
	}
	if req.Operation == domain.OperationImageGenerate || req.Modality == domain.ModalityImage {
		if err := validateImageShape(req, false); err != nil {
			return domain.CostEstimate{}, err
		}
		providerCredits := nanoBanana2ProviderCredits(req.Resolution)
		return domain.CostEstimate{
			AmountCredits: providerCredits,
			Currency:      "credits",
			Estimated:     false,
		}, nil
	}
	if err := validateVideoShape(req, false); err != nil {
		return domain.CostEstimate{}, err
	}
	rate, ok := exactCreditsPerSecond(req.ModelCode)
	if !ok {
		return domain.CostEstimate{}, &Error{Class: domain.ProviderErrUnsupportedCapab, Message: "PoYo route cost is unavailable"}
	}
	duration := effectiveDuration(req.DurationSec)
	return domain.CostEstimate{
		AmountCredits: int64(duration) * rate,
		Currency:      "credits",
		Estimated:     false,
	}, nil
}

// Submit creates an async PoYo video generation task.
func (p *Provider) Submit(ctx context.Context, req domain.ProviderRequest) (domain.ProviderTask, error) {
	if req.IdempotencyKey != "" {
		if task, ok := p.idempotentTask(req.IdempotencyKey); ok {
			return task, nil
		}
	}
	body, err := buildSubmitRequest(req)
	if err != nil {
		return domain.ProviderTask{}, err
	}
	var decoded submitResponse
	if err := p.postJSON(ctx, "/api/generate/submit", body, &decoded, req.IdempotencyKey); err != nil {
		return domain.ProviderTask{}, err
	}
	if err := decoded.err(); err != nil {
		return domain.ProviderTask{}, err
	}
	taskID := decoded.taskID()
	if strings.TrimSpace(taskID) == "" {
		return domain.ProviderTask{}, &Error{Class: domain.ProviderErrInternal, Message: "empty submit task id"}
	}

	now := p.now()
	status := mapTaskStatus(decoded.status())
	if status == "" {
		status = domain.ProviderTaskPending
	}
	task := domain.ProviderTask{
		JobID:          req.JobID,
		Provider:       domain.ProviderPoYo,
		ModelCode:      req.ModelCode,
		ExternalID:     strings.TrimSpace(taskID),
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

// Poll fetches PoYo task status. Output URLs are returned only to the worker,
// which stores them as our artifacts before the job can succeed.
func (p *Provider) Poll(ctx context.Context, ref domain.ProviderTaskRef) (domain.ProviderTaskResult, error) {
	taskID := strings.TrimSpace(ref.ExternalID)
	if taskID == "" {
		return domain.ProviderTaskResult{
			Status:     domain.ProviderTaskFailed,
			ErrorClass: domain.ProviderErrTaskNotFound,
		}, nil
	}
	var decoded statusResponse
	if err := p.getJSON(ctx, "/api/generate/status/"+url.PathEscape(taskID), &decoded); err != nil {
		return domain.ProviderTaskResult{}, err
	}
	if err := decoded.err(); err != nil {
		return domain.ProviderTaskResult{}, err
	}
	status := mapTaskStatus(decoded.status())
	switch status {
	case domain.ProviderTaskSucceeded:
		outputs := decoded.outputMediaURLs()
		if len(outputs) == 0 {
			return domain.ProviderTaskResult{
				Status:       domain.ProviderTaskFailed,
				ErrorClass:   domain.ProviderErrOutputDownloadFailed,
				ErrorMessage: "poyo task completed without media output",
				Raw:          sanitizedTaskMetadata(decoded),
			}, nil
		}
		return domain.ProviderTaskResult{Status: status, OutputURLs: outputs, Raw: sanitizedTaskMetadata(decoded)}, nil
	case domain.ProviderTaskFailed:
		perr := decoded.providerError()
		class := classifyPoYoError(0, perr.codeString(), perr.Type, perr.Message)
		return domain.ProviderTaskResult{
			Status:       domain.ProviderTaskFailed,
			ErrorClass:   class,
			ErrorMessage: providerErrorMessage(class, "poyo task failed"),
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

// Cancel is a no-op until PoYo documents a stable cancellation endpoint.
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

type submitRequest struct {
	Model       string         `json:"model"`
	Input       map[string]any `json:"input"`
	CallbackURL string         `json:"callback_url,omitempty"`
}

type submitResponse struct {
	Code    int           `json:"code,omitempty"`
	Data    submitData    `json:"data,omitempty"`
	Task    submitData    `json:"task,omitempty"`
	Success *bool         `json:"success,omitempty"`
	ID      string        `json:"id,omitempty"`
	UUID    string        `json:"uuid,omitempty"`
	JobID   string        `json:"job_id,omitempty"`
	TaskID  string        `json:"task_id,omitempty"`
	Status  string        `json:"status,omitempty"`
	Message string        `json:"message,omitempty"`
	Error   providerError `json:"error,omitempty"`
}

type submitData struct {
	ID          string `json:"id,omitempty"`
	UUID        string `json:"uuid,omitempty"`
	JobID       string `json:"job_id,omitempty"`
	TaskID      string `json:"task_id,omitempty"`
	Status      string `json:"status,omitempty"`
	CreatedTime string `json:"created_time,omitempty"`
}

func (r submitResponse) err() error {
	if r.Success != nil && !*r.Success {
		return apiEnvelopeError(http.StatusOK, r.Error, r.Message)
	}
	if r.Code != 0 && r.Code != http.StatusOK {
		return apiEnvelopeError(r.Code, r.Error, r.Message)
	}
	if !r.Error.empty() {
		return apiEnvelopeError(http.StatusOK, r.Error, r.Message)
	}
	return nil
}

func (r submitResponse) taskID() string {
	for _, raw := range []string{
		r.Data.TaskID,
		r.Data.ID,
		r.Data.UUID,
		r.Data.JobID,
		r.Task.TaskID,
		r.Task.ID,
		r.Task.UUID,
		r.Task.JobID,
		r.TaskID,
		r.ID,
		r.UUID,
		r.JobID,
	} {
		if trimmed := strings.TrimSpace(raw); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (r submitResponse) status() string {
	if trimmed := strings.TrimSpace(r.Data.Status); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(r.Task.Status); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(r.Status)
}

type statusResponse struct {
	Code      int            `json:"code,omitempty"`
	Data      statusData     `json:"data,omitempty"`
	Success   *bool          `json:"success,omitempty"`
	TaskID    string         `json:"task_id,omitempty"`
	Status    string         `json:"status,omitempty"`
	Progress  progressValue  `json:"progress,omitempty"`
	Result    statusResult   `json:"result,omitempty"`
	Error     providerError  `json:"error,omitempty"`
	Message   string         `json:"message,omitempty"`
	CreatedAt string         `json:"created_at,omitempty"`
	UpdatedAt string         `json:"updated_at,omitempty"`
	Raw       map[string]any `json:"-"`
}

type statusData struct {
	ID            string        `json:"id,omitempty"`
	UUID          string        `json:"uuid,omitempty"`
	JobID         string        `json:"job_id,omitempty"`
	TaskID        string        `json:"task_id,omitempty"`
	Status        string        `json:"status,omitempty"`
	CreditsAmount float64       `json:"credits_amount,omitempty"`
	Files         []statusFile  `json:"files,omitempty"`
	Result        statusResult  `json:"result,omitempty"`
	Output        statusResult  `json:"output,omitempty"`
	CreatedTime   string        `json:"created_time,omitempty"`
	Progress      progressValue `json:"progress,omitempty"`
	ErrorMessage  *string       `json:"error_message,omitempty"`
}

type statusFile struct {
	FileURL     string `json:"file_url,omitempty"`
	FileType    string `json:"file_type,omitempty"`
	Label       string `json:"label,omitempty"`
	Format      string `json:"format,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	FileName    string `json:"file_name,omitempty"`
	FileSize    int64  `json:"file_size,omitempty"`
}

func (r statusResponse) err() error {
	if r.Success != nil && !*r.Success {
		return apiEnvelopeError(http.StatusOK, r.Error, r.Message)
	}
	if r.Code != 0 && r.Code != http.StatusOK {
		return apiEnvelopeError(r.Code, r.Error, r.Message)
	}
	if !r.Error.empty() {
		return apiEnvelopeError(http.StatusOK, r.Error, r.Message)
	}
	return nil
}

func (r statusResponse) taskID() string {
	for _, raw := range []string{r.Data.TaskID, r.Data.ID, r.Data.UUID, r.Data.JobID, r.TaskID} {
		if trimmed := strings.TrimSpace(raw); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (r statusResponse) status() string {
	if trimmed := strings.TrimSpace(r.Data.Status); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(r.Status)
}

func (r statusResponse) progress() int {
	if r.Data.Progress != 0 {
		return int(r.Data.Progress)
	}
	return int(r.Progress)
}

func (r statusResponse) createdAt() string {
	if trimmed := strings.TrimSpace(r.Data.CreatedTime); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(r.CreatedAt)
}

func (r statusResponse) updatedAt() string {
	return strings.TrimSpace(r.UpdatedAt)
}

func (r statusResponse) outputMediaURLs() []string {
	out := r.Result.OutputVideoURLs()
	out = append(out, r.Data.Result.OutputVideoURLs()...)
	out = append(out, r.Data.Output.OutputVideoURLs()...)
	for _, file := range r.Data.Files {
		if strings.TrimSpace(file.FileURL) == "" {
			continue
		}
		fileType := strings.ToLower(strings.TrimSpace(file.FileType))
		format := strings.ToLower(strings.TrimSpace(file.Format))
		contentType := strings.ToLower(strings.TrimSpace(file.ContentType))
		if fileType == "image" ||
			fileType == "video" ||
			strings.Contains(contentType, "image/") ||
			strings.Contains(contentType, "video/") ||
			format == "jpg" ||
			format == "jpeg" ||
			format == "png" ||
			format == "webp" ||
			format == "mp4" ||
			format == "mov" {
			out = append(out, strings.TrimSpace(file.FileURL))
		}
	}
	return out
}

func (r statusResponse) providerError() providerError {
	if r.Data.ErrorMessage != nil && strings.TrimSpace(*r.Data.ErrorMessage) != "" {
		return providerError{Message: strings.TrimSpace(*r.Data.ErrorMessage)}
	}
	if !r.Error.empty() {
		return r.Error
	}
	if msg := strings.TrimSpace(r.Message); msg != "" {
		return providerError{Message: msg}
	}
	return r.Error
}

type statusResult struct {
	VideoURL   string  `json:"video_url,omitempty"`
	VideoURLs  urlList `json:"video_urls,omitempty"`
	ImageURL   string  `json:"image_url,omitempty"`
	ImageURLs  urlList `json:"image_urls,omitempty"`
	OutputURL  string  `json:"output_url,omitempty"`
	OutputURLs urlList `json:"output_urls,omitempty"`
	URL        string  `json:"url,omitempty"`
	URLs       urlList `json:"urls,omitempty"`
	Duration   int     `json:"duration,omitempty"`
	Resolution string  `json:"resolution,omitempty"`
}

func (r statusResult) OutputVideoURLs() []string {
	var out []string
	for _, value := range []string{r.VideoURL, r.ImageURL, r.OutputURL, r.URL} {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	for _, values := range []urlList{r.VideoURLs, r.ImageURLs, r.OutputURLs, r.URLs} {
		for _, value := range values {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				out = append(out, trimmed)
			}
		}
	}
	return out
}

type urlList []string

type progressValue int

func (p *progressValue) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		return nil
	}
	var numeric float64
	if err := json.Unmarshal(data, &numeric); err == nil {
		*p = progressValue(numeric)
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		text = strings.TrimSpace(strings.TrimSuffix(text, "%"))
		if text == "" {
			return nil
		}
		if parsed, parseErr := strconv.ParseFloat(text, 64); parseErr == nil {
			*p = progressValue(parsed)
		}
	}
	return nil
}

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
	return fmt.Errorf("poyo provider: invalid url field")
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

type requestParams struct {
	ResolvedVideoRoute   domain.VideoRouteSnapshot `json:"resolved_video_route,omitempty"`
	Audio                *bool                     `json:"audio,omitempty"`
	WithAudio            *bool                     `json:"with_audio,omitempty"`
	GenerateAudio        *bool                     `json:"generate_audio,omitempty"`
	AudioPrompt          string                    `json:"audio_prompt,omitempty"`
	ReferenceAudioURL    string                    `json:"reference_audio_url,omitempty"`
	ReferenceAudioURLs   []string                  `json:"reference_audio_urls,omitempty"`
	ReferenceVideoURL    string                    `json:"reference_video_url,omitempty"`
	ReferenceVideoURLs   []string                  `json:"reference_video_urls,omitempty"`
	ReferenceArtifactIDs []string                  `json:"reference_artifact_ids,omitempty"`
}

func buildSubmitRequest(req domain.ProviderRequest) (submitRequest, error) {
	if req.Operation == domain.OperationImageGenerate || req.Modality == domain.ModalityImage {
		return buildImageSubmitRequest(req)
	}
	return buildVideoSubmitRequest(req)
}

func buildImageSubmitRequest(req domain.ProviderRequest) (submitRequest, error) {
	if err := validateImageShape(req, true); err != nil {
		return submitRequest{}, err
	}
	inputURLs := cleanInputURLs(req.InputURLs)
	input := map[string]any{
		"prompt":     strings.TrimSpace(req.Prompt),
		"size":       effectiveImageSize(req),
		"resolution": effectiveImageResolution(req.Resolution),
	}
	if len(inputURLs) > 0 {
		input["image_urls"] = inputURLs
	}
	return submitRequest{
		Model: strings.TrimSpace(req.ModelCode),
		Input: input,
	}, nil
}

func buildVideoSubmitRequest(req domain.ProviderRequest) (submitRequest, error) {
	if err := validateVideoShape(req, true); err != nil {
		return submitRequest{}, err
	}
	inputURLs := cleanInputURLs(req.InputURLs)
	input := map[string]any{
		"prompt":       strings.TrimSpace(req.Prompt),
		"duration":     effectiveDuration(req.DurationSec),
		"aspect_ratio": effectiveAspectRatio(req.AspectRatio),
	}
	switch strings.TrimSpace(req.ModelCode) {
	case ModelKlingO3Standard:
		input["sound"] = false
		if len(inputURLs) > 0 {
			input["image_urls"] = inputURLs
		}
	case ModelSeedance20Fast:
		input["resolution"] = effectiveResolution(req.ModelCode, req.Resolution)
		input["generate_audio"] = false
		if len(inputURLs) > 0 {
			input["image_urls"] = inputURLs
		}
	case ModelRunwayGen45:
		// PoYo Runway Gen-4.5 allows only one optional reference image until
		// smoke tests prove a broader input contract.
		input["resolution"] = effectiveResolution(req.ModelCode, req.Resolution)
		if len(inputURLs) > 0 {
			input["image_url"] = inputURLs[0]
		}
	}
	return submitRequest{
		Model: strings.TrimSpace(req.ModelCode),
		Input: input,
	}, nil
}

func validateImageShape(req domain.ProviderRequest, requirePrompt bool) error {
	if req.Operation != domain.OperationImageGenerate || req.Modality != domain.ModalityImage {
		return &Error{Class: domain.ProviderErrUnsupportedCapab, Message: string(req.Operation) + "/" + string(req.Modality)}
	}
	if strings.TrimSpace(req.ModelCode) != ModelNanoBanana2New {
		return &Error{Class: domain.ProviderErrUnsupportedCapab, Message: "unsupported PoYo image model"}
	}
	if requirePrompt {
		prompt := strings.TrimSpace(req.Prompt)
		if prompt == "" {
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "prompt is required"}
		}
		if len([]rune(prompt)) > 20000 {
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "prompt exceeds 20000 characters"}
		}
	}
	if value := strings.TrimSpace(req.AspectRatio); value != "" && !allowedImageSize(value) {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported PoYo image size"}
	}
	if value := strings.TrimSpace(req.Size); value != "" && !allowedImageSize(value) {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported PoYo image size"}
	}
	if value := strings.TrimSpace(req.Resolution); value != "" && !allowedImageResolution(value) {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported PoYo image resolution"}
	}
	if len(cleanInputURLs(req.InputURLs)) > maxNanoBanana2ReferenceImages {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "too many Nano Banana 2 reference images"}
	}
	return nil
}

func effectiveImageSize(req domain.ProviderRequest) string {
	for _, value := range []string{req.AspectRatio, req.Size} {
		trimmed := strings.TrimSpace(value)
		if allowedImageSize(trimmed) {
			return trimmed
		}
	}
	return "1:1"
}

func effectiveImageResolution(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "1K", "2K", "4K":
		return strings.ToUpper(strings.TrimSpace(value))
	default:
		return "1K"
	}
}

func nanoBanana2ProviderCredits(resolution string) int64 {
	switch strings.ToUpper(strings.TrimSpace(resolution)) {
	case "2K":
		return 8
	case "4K":
		return 12
	default:
		return 5
	}
}

func allowedImageResolution(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "1K", "2K", "4K":
		return true
	default:
		return false
	}
}

func allowedImageSize(value string) bool {
	switch strings.TrimSpace(value) {
	case "1:1", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9",
		"1:4", "4:1", "1:8", "8:1":
		return true
	default:
		return false
	}
}

func validateVideoShape(req domain.ProviderRequest, requirePrompt bool) error {
	if req.Operation != domain.OperationVideoGenerate || req.Modality != domain.ModalityVideo {
		return &Error{Class: domain.ProviderErrUnsupportedCapab, Message: string(req.Operation) + "/" + string(req.Modality)}
	}
	model := strings.TrimSpace(req.ModelCode)
	if !isSupportedVideoModel(model) {
		return &Error{Class: domain.ProviderErrUnsupportedCapab, Message: "unsupported PoYo video model"}
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
	params, err := parseParams(req.Params)
	if err != nil {
		return err
	}
	if audioRequested(params) {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "audio is disabled for PoYo route"}
	}
	if videoReferenceRequested(params) {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "video references are not enabled for PoYo route"}
	}
	duration := effectiveDuration(req.DurationSec)
	if duration != 5 && duration != 10 {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "duration must be 5 or 10 seconds"}
	}
	resolution := effectiveResolution(model, req.Resolution)
	if !allowedResolution(model, resolution) {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported PoYo video resolution"}
	}
	aspectRatio := effectiveAspectRatio(req.AspectRatio)
	if aspectRatio != "16:9" && aspectRatio != "9:16" && aspectRatio != "1:1" {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "unsupported PoYo video aspect ratio"}
	}
	inputCount := len(cleanInputURLs(req.InputURLs))
	switch model {
	case ModelKlingO3Standard:
		if inputCount > maxKlingReferenceImages {
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "too many Kling reference images"}
		}
	case ModelSeedance20Fast:
		if inputCount > maxSeedanceReferenceImages {
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "too many Seedance reference images"}
		}
	case ModelRunwayGen45:
		if inputCount > maxRunwayReferenceImages {
			return &Error{Class: domain.ProviderErrInvalidRequest, Message: "Runway Gen-4.5 accepts one reference image"}
		}
	}
	return nil
}

func parseParams(raw json.RawMessage) (requestParams, error) {
	if len(raw) == 0 {
		return requestParams{}, nil
	}
	var params requestParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return requestParams{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid PoYo params json"}
	}
	return params, nil
}

func resolvedRouteSnapshot(raw json.RawMessage) (domain.VideoRouteSnapshot, bool, error) {
	params, err := parseParams(raw)
	if err != nil {
		return domain.VideoRouteSnapshot{}, false, err
	}
	if !params.ResolvedVideoRoute.Valid() {
		return domain.VideoRouteSnapshot{}, false, nil
	}
	return params.ResolvedVideoRoute, true, nil
}

func audioRequested(params requestParams) bool {
	for _, value := range []*bool{params.Audio, params.WithAudio, params.GenerateAudio} {
		if value != nil && *value {
			return true
		}
	}
	return strings.TrimSpace(params.AudioPrompt) != "" ||
		strings.TrimSpace(params.ReferenceAudioURL) != "" ||
		len(params.ReferenceAudioURLs) > 0
}

func videoReferenceRequested(params requestParams) bool {
	return strings.TrimSpace(params.ReferenceVideoURL) != "" || len(params.ReferenceVideoURLs) > 0
}

func isSupportedModel(model string) bool {
	return isSupportedVideoModel(model) || strings.TrimSpace(model) == ModelNanoBanana2New
}

func isSupportedVideoModel(model string) bool {
	switch strings.TrimSpace(model) {
	case ModelKlingO3Standard, ModelSeedance20Fast, ModelRunwayGen45:
		return true
	default:
		return false
	}
}

func exactCreditsPerSecond(model string) (int64, bool) {
	switch strings.TrimSpace(model) {
	case ModelKlingO3Standard:
		return klingO3CreditsPerSecond, true
	case ModelSeedance20Fast:
		return seedance20CreditsPerSecond, true
	default:
		return 0, false
	}
}

func effectiveDuration(value int) int {
	if value > 0 {
		return value
	}
	return 5
}

func effectiveResolution(model, value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value != "" {
		return value
	}
	if strings.TrimSpace(model) == ModelSeedance20Fast {
		return "720p"
	}
	return "720p"
}

func allowedResolution(model, resolution string) bool {
	switch strings.TrimSpace(model) {
	case ModelSeedance20Fast:
		return resolution == "720p"
	default:
		return resolution == "720p" || resolution == "1080p"
	}
}

func effectiveAspectRatio(value string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return "16:9"
}

func cleanInputURLs(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
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
	msg := fmt.Sprintf("poyo http %d", resp.StatusCode)
	var decoded struct {
		Code    json.RawMessage `json:"code,omitempty"`
		Message string          `json:"message,omitempty"`
		Type    string          `json:"type,omitempty"`
		Error   providerError   `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err == nil {
		if !decoded.Error.empty() {
			class := classifyPoYoError(resp.StatusCode, decoded.Error.codeString(), decoded.Error.Type, decoded.Error.Message)
			return &Error{Class: class, Message: providerErrorMessage(class, msg)}
		}
		code := providerError{Code: decoded.Code}.codeString()
		if decoded.Message != "" || decoded.Type != "" || code != "" {
			class := classifyPoYoError(resp.StatusCode, code, decoded.Type, decoded.Message)
			return &Error{Class: class, Message: providerErrorMessage(class, msg)}
		}
	}
	return &Error{Class: classifyPoYoError(resp.StatusCode, "", "", ""), Message: msg}
}

func apiEnvelopeError(status int, errValue providerError, message string) error {
	msg := message
	if msg == "" {
		msg = errValue.Message
	}
	if msg == "" {
		msg = "poyo api request failed"
	}
	class := classifyPoYoError(status, errValue.codeString(), errValue.Type, msg)
	return &Error{Class: class, Message: providerErrorMessage(class, msg)}
}

func classifyPoYoError(status int, code, typ, msg string) domain.ProviderErrorClass {
	lower := strings.ToLower(strings.Join([]string{code, typ, msg}, " "))
	switch {
	case strings.Contains(lower, "balance") || strings.Contains(lower, "insufficient") || strings.Contains(lower, "quota") || strings.Contains(lower, "credit"):
		return domain.ProviderErrInsufficientBalance
	case strings.Contains(lower, "auth") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "forbidden") || strings.Contains(lower, "token") || strings.Contains(lower, "permission"):
		return domain.ProviderErrAuthFailed
	case strings.Contains(lower, "moderation") ||
		strings.Contains(lower, "policy") ||
		strings.Contains(lower, "safety") ||
		strings.Contains(lower, "nsfw") ||
		strings.Contains(lower, "sensitive") ||
		strings.Contains(lower, "content rejected") ||
		strings.Contains(lower, "does not comply") ||
		strings.Contains(lower, "platform regulation") ||
		strings.Contains(lower, "prohibited") ||
		strings.Contains(lower, "violat") ||
		strings.Contains(lower, "filtered out") ||
		strings.Contains(lower, "blocked by"):
		return domain.ProviderErrContentRejected
	case strings.Contains(lower, "rate") || strings.Contains(lower, "too many"):
		return domain.ProviderErrRateLimited
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
	case "not_started", "submitted", "pending", "queued":
		return domain.ProviderTaskPending
	case "processing", "running", "in_progress", "in-progress":
		return domain.ProviderTaskProcessing
	case "finished", "completed", "complete", "succeeded", "success", "done":
		return domain.ProviderTaskSucceeded
	case "failed", "error":
		return domain.ProviderTaskFailed
	case "cancelled", "canceled":
		return domain.ProviderTaskCancelled
	default:
		return domain.ProviderTaskProcessing
	}
}

func sanitizedTaskMetadata(data statusResponse) json.RawMessage {
	metadata := map[string]any{
		"task_id":    data.taskID(),
		"status":     data.status(),
		"progress":   data.progress(),
		"created_at": data.createdAt(),
		"updated_at": data.updatedAt(),
	}
	if data.Result.Duration > 0 {
		metadata["duration"] = data.Result.Duration
	}
	if strings.TrimSpace(data.Result.Resolution) != "" {
		metadata["resolution"] = data.Result.Resolution
	}
	if perr := data.providerError(); !perr.empty() {
		class := classifyPoYoError(0, perr.codeString(), perr.Type, perr.Message)
		metadata["error"] = map[string]any{
			"code":    perr.codeString(),
			"type":    perr.Type,
			"message": providerErrorMessage(class, "poyo task failed"),
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
		return "poyo authentication failed"
	case domain.ProviderErrInsufficientBalance:
		return "poyo balance is insufficient"
	case domain.ProviderErrRateLimited:
		return "poyo rate limit exceeded"
	case domain.ProviderErrInvalidRequest:
		return "poyo request validation failed"
	case domain.ProviderErrContentRejected:
		return "poyo content moderation rejected the request"
	case domain.ProviderErrOverloaded:
		return "poyo provider is unavailable"
	case domain.ProviderErrTimeout:
		return "poyo provider timed out"
	case domain.ProviderErrTaskNotFound:
		return "poyo task was not found"
	case domain.ProviderErrOutputDownloadFailed:
		return "poyo completed without a downloadable output"
	}
	if strings.TrimSpace(fallback) == "" {
		return "poyo provider error"
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

// Error is a PoYo failure carrying a normalized class.
type Error struct {
	Class   domain.ProviderErrorClass
	Message string
}

func (e *Error) Error() string {
	if e.Message == "" {
		return "poyo provider: " + string(e.Class)
	}
	return "poyo provider: " + string(e.Class) + ": " + e.Message
}

// ProviderErrorClass exposes the normalized class for worker classification.
func (e *Error) ProviderErrorClass() domain.ProviderErrorClass { return e.Class }
