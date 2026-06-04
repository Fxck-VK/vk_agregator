package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/moderationservice"
)

// ModerationConfig configures the OpenAI moderation adapter.
type ModerationConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	// HTTPClient overrides the HTTP client (mainly for tests). Optional.
	HTTPClient *http.Client
}

// Moderator implements moderationservice.Moderator using OpenAI Moderations.
type Moderator struct {
	cfg  ModerationConfig
	http *http.Client
}

// NewModerator builds an OpenAI moderation adapter.
func NewModerator(cfg ModerationConfig) *Moderator {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.Model == "" {
		cfg.Model = "omni-moderation-latest"
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Moderator{cfg: cfg, http: httpClient}
}

var _ moderationservice.Moderator = (*Moderator)(nil)

// Name identifies the moderation provider.
func (m *Moderator) Name() string { return "openai:" + m.cfg.Model }

// Check evaluates prompt/text/image inputs and returns a normalized verdict.
func (m *Moderator) Check(ctx context.Context, in moderationservice.Input) (moderationservice.Outcome, error) {
	input := in.Prompt
	if in.Text != "" {
		input = strings.TrimSpace(in.Prompt + "\n" + in.Text)
	}
	out, err := m.moderate(ctx, input)
	if err != nil {
		return moderationservice.Outcome{}, err
	}
	return out, nil
}

// Scan implements artifactservice.Scanner for text and image artifacts. OpenAI
// moderation does not support video; video scanning is a separate provider
// integration and is left fail-open here.
func (m *Moderator) Scan(ctx context.Context, mediaType domain.MediaType, mimeType string, data []byte) error {
	var input any
	switch mediaType {
	case domain.MediaTypeText:
		input = string(data)
	case domain.MediaTypeImage:
		input = []map[string]any{
			{"type": "text", "text": "moderate generated image artifact"},
			{"type": "image_url", "image_url": map[string]string{"url": "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)}},
		}
	default:
		return nil
	}
	out, err := m.moderate(ctx, input)
	if err != nil {
		return err
	}
	if !out.Decision.Allowed() {
		return fmt.Errorf("openai moderation rejected artifact: %s", strings.Join(out.Categories, ","))
	}
	return nil
}

type moderationRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"`
}

type moderationResponse struct {
	Results []struct {
		Flagged    bool            `json:"flagged"`
		Categories map[string]bool `json:"categories"`
	} `json:"results"`
	Error *apiError `json:"error"`
}

func (m *Moderator) moderate(ctx context.Context, input any) (moderationservice.Outcome, error) {
	body, err := json.Marshal(moderationRequest{Model: m.cfg.Model, Input: input})
	if err != nil {
		return moderationservice.Outcome{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: err.Error()}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.cfg.BaseURL+"/moderations", bytes.NewReader(body))
	if err != nil {
		return moderationservice.Outcome{}, &Error{Class: domain.ProviderErrInternal, Message: err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+m.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.http.Do(req)
	if err != nil {
		return moderationservice.Outcome{}, &Error{Class: domain.ProviderErrTimeout, Message: err.Error()}
	}
	defer resp.Body.Close()

	var decoded moderationResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return moderationservice.Outcome{}, &Error{Class: domain.ProviderErrInternal, Message: "decode moderation response: " + err.Error()}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := fmt.Sprintf("openai moderation http %d", resp.StatusCode)
		if decoded.Error != nil && decoded.Error.Message != "" {
			msg = decoded.Error.Message
		}
		return moderationservice.Outcome{}, &Error{Class: classifyStatus(resp.StatusCode), Message: msg}
	}
	if len(decoded.Results) == 0 {
		return moderationservice.Outcome{}, &Error{Class: domain.ProviderErrInternal, Message: "empty moderation response"}
	}
	categories := flaggedCategories(decoded.Results[0].Categories)
	if decoded.Results[0].Flagged {
		return moderationservice.Outcome{Decision: domain.ModerationBlock, Categories: categories}, nil
	}
	return moderationservice.Outcome{Decision: domain.ModerationAllow, Categories: categories}, nil
}

func flaggedCategories(in map[string]bool) []string {
	out := make([]string, 0, len(in))
	for k, flagged := range in {
		if flagged {
			out = append(out, k)
		}
	}
	return out
}
