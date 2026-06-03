// This file implements the production VK delivery Client backed by the real VK
// API (messages.send). It is wired only when VK_DELIVERY_MODE=real and
// VK_ACCESS_TOKEN is set; the default runtime uses the mock client, so no real
// token is required for local development or CI (audit V1).
//
// Media handling: SendPhoto/SendVideo forward a VK attachment string (e.g.
// "photo-123_456") through messages.send. Uploading raw bytes to VK upload
// servers (photos.getMessagesUploadServer -> upload -> photos.saveMessagesPhoto)
// is a separate, documented step the delivery layer performs before calling
// these methods.
package vkdelivery

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"encoding/json"
)

// HTTPConfig configures the real VK client.
type HTTPConfig struct {
	// AccessToken is the community/user token authorizing messages.send.
	AccessToken string
	// APIVersion is the VK API version (e.g. "5.199").
	APIVersion string
	// BaseURL is the API method root (default https://api.vk.com/method).
	BaseURL string
	// HTTPClient overrides the HTTP client (mainly for tests).
	HTTPClient *http.Client
}

// HTTPClient is the production Client that calls the real VK API.
type HTTPClient struct {
	cfg  HTTPConfig
	http *http.Client
}

// NewHTTPClient builds a real VK delivery client, applying defaults.
func NewHTTPClient(cfg HTTPConfig) *HTTPClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.vk.com/method"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.APIVersion == "" {
		cfg.APIVersion = "5.199"
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &HTTPClient{cfg: cfg, http: httpClient}
}

var _ Client = (*HTTPClient)(nil)

// SendText sends a plain text message via messages.send.
func (c *HTTPClient) SendText(ctx context.Context, peerID, randomID int64, text string) (SendResult, error) {
	return c.send(ctx, peerID, randomID, text, "")
}

// SendPhoto sends a photo referenced by a VK attachment string with a caption.
func (c *HTTPClient) SendPhoto(ctx context.Context, peerID, randomID int64, attachment, caption string) (SendResult, error) {
	return c.send(ctx, peerID, randomID, caption, attachment)
}

// SendVideo sends a video referenced by a VK attachment string with a caption.
func (c *HTTPClient) SendVideo(ctx context.Context, peerID, randomID int64, attachment, caption string) (SendResult, error) {
	return c.send(ctx, peerID, randomID, caption, attachment)
}

// vkResponse is the VK messages.send envelope. On success response holds the
// message id; on failure error is populated.
type vkResponse struct {
	Response int64 `json:"response"`
	Error    *struct {
		Code int    `json:"error_code"`
		Msg  string `json:"error_msg"`
	} `json:"error"`
}

func (c *HTTPClient) send(ctx context.Context, peerID, randomID int64, message, attachment string) (SendResult, error) {
	form := url.Values{}
	form.Set("access_token", c.cfg.AccessToken)
	form.Set("v", c.cfg.APIVersion)
	form.Set("peer_id", strconv.FormatInt(peerID, 10))
	form.Set("random_id", strconv.FormatInt(randomID, 10))
	if message != "" {
		form.Set("message", message)
	}
	if attachment != "" {
		form.Set("attachment", attachment)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/messages.send", strings.NewReader(form.Encode()))
	if err != nil {
		return SendResult{}, fmt.Errorf("vkdelivery: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return SendResult{}, fmt.Errorf("vkdelivery: send: %w", err)
	}
	defer resp.Body.Close()

	var decoded vkResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return SendResult{}, fmt.Errorf("vkdelivery: decode response: %w", err)
	}
	if decoded.Error != nil {
		return SendResult{}, fmt.Errorf("vkdelivery: vk error %d: %s", decoded.Error.Code, decoded.Error.Msg)
	}
	return SendResult{MessageID: decoded.Response, PeerID: peerID}, nil
}
