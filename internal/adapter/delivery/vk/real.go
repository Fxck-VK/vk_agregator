// This file implements the production VK delivery Client backed by the real VK
// API. It sends via messages.send and uploads raw media artifacts through the VK
// upload-server flows before delivery. It is wired only when
// VK_DELIVERY_MODE=real and VK_ACCESS_TOKEN is set; the default runtime uses the
// mock client, so no real token is required for local development or CI.
package vkdelivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
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

var (
	_ Client        = (*HTTPClient)(nil)
	_ ControlClient = (*HTTPClient)(nil)
	_ MediaUploader = (*HTTPClient)(nil)
)

// SendText sends a plain text message via messages.send.
func (c *HTTPClient) SendText(ctx context.Context, peerID, randomID int64, text string) (SendResult, error) {
	return c.SendMessage(ctx, peerID, randomID, Message{Text: text})
}

// SendPhoto sends a photo referenced by a VK attachment string with a caption.
func (c *HTTPClient) SendPhoto(ctx context.Context, peerID, randomID int64, attachment, caption string) (SendResult, error) {
	return c.SendMessage(ctx, peerID, randomID, Message{Text: caption, Attachment: attachment})
}

// SendVideo sends a video referenced by a VK attachment string with a caption.
func (c *HTTPClient) SendVideo(ctx context.Context, peerID, randomID int64, attachment, caption string) (SendResult, error) {
	return c.SendMessage(ctx, peerID, randomID, Message{Text: caption, Attachment: attachment})
}

// SendMessage sends a VK message with optional attachment and keyboard.
func (c *HTTPClient) SendMessage(ctx context.Context, peerID, randomID int64, msg Message) (SendResult, error) {
	return c.send(ctx, peerID, randomID, msg)
}

// vkMessageResponse is the VK messages.send envelope. On success response holds
// the message id; on failure error is populated.
type vkMessageResponse struct {
	Response int64 `json:"response"`
	Error    *struct {
		Code int    `json:"error_code"`
		Msg  string `json:"error_msg"`
	} `json:"error"`
}

// APIError is a normalized VK API error envelope.
type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("vkdelivery: vk error %d: %s", e.Code, e.Message)
}

// IsAPIErrorCode reports whether err wraps a VK API error with one of codes.
func IsAPIErrorCode(err error, codes ...int) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	for _, code := range codes {
		if apiErr.Code == code {
			return true
		}
	}
	return false
}

func (c *HTTPClient) send(ctx context.Context, peerID, randomID int64, msg Message) (SendResult, error) {
	form := url.Values{}
	form.Set("peer_id", strconv.FormatInt(peerID, 10))
	form.Set("random_id", strconv.FormatInt(randomID, 10))
	if msg.Text != "" {
		form.Set("message", msg.Text)
	}
	if msg.Attachment != "" {
		form.Set("attachment", msg.Attachment)
	}
	if msg.Keyboard != nil {
		keyboard, err := encodeKeyboard(msg.Keyboard)
		if err != nil {
			return SendResult{}, err
		}
		form.Set("keyboard", keyboard)
	}

	var decoded vkMessageResponse
	if err := c.api(ctx, "messages.send", form, &decoded); err != nil {
		return SendResult{}, err
	}
	return SendResult{MessageID: decoded.Response, PeerID: peerID}, nil
}

type vkKeyboard struct {
	OneTime bool                 `json:"one_time"`
	Inline  bool                 `json:"inline"`
	Buttons [][]vkKeyboardButton `json:"buttons"`
}

type vkKeyboardButton struct {
	Action vkKeyboardAction `json:"action"`
	Color  string           `json:"color,omitempty"`
}

type vkKeyboardAction struct {
	Type    string `json:"type"`
	Label   string `json:"label"`
	Payload string `json:"payload,omitempty"`
}

func encodeKeyboard(k *Keyboard) (string, error) {
	vk := vkKeyboard{
		OneTime: k.OneTime,
		Inline:  k.Inline,
		Buttons: make([][]vkKeyboardButton, 0, len(k.Buttons)),
	}
	for _, row := range k.Buttons {
		vkRow := make([]vkKeyboardButton, 0, len(row))
		for _, button := range row {
			vkRow = append(vkRow, vkKeyboardButton{
				Action: vkKeyboardAction{
					Type:    "text",
					Label:   button.Label,
					Payload: button.Payload,
				},
				Color: button.Color,
			})
		}
		vk.Buttons = append(vk.Buttons, vkRow)
	}
	data, err := json.Marshal(vk)
	if err != nil {
		return "", fmt.Errorf("vkdelivery: encode keyboard: %w", err)
	}
	return string(data), nil
}

// UploadPhoto uploads a stored image artifact and returns a VK photo attachment.
func (c *HTTPClient) UploadPhoto(ctx context.Context, peerID int64, filename string, data []byte, mimeType string) (string, error) {
	form := url.Values{}
	form.Set("peer_id", strconv.FormatInt(peerID, 10))
	var server struct {
		Response struct {
			UploadURL string `json:"upload_url"`
		} `json:"response"`
	}
	if err := c.api(ctx, "photos.getMessagesUploadServer", form, &server); err != nil {
		return "", err
	}
	if server.Response.UploadURL == "" {
		return "", fmt.Errorf("vkdelivery: empty photo upload_url")
	}

	var uploaded struct {
		Server int64  `json:"server"`
		Photo  string `json:"photo"`
		Hash   string `json:"hash"`
	}
	if err := c.uploadMultipart(ctx, server.Response.UploadURL, "photo", filename, mimeType, data, &uploaded); err != nil {
		return "", err
	}

	save := url.Values{}
	save.Set("server", strconv.FormatInt(uploaded.Server, 10))
	save.Set("photo", uploaded.Photo)
	save.Set("hash", uploaded.Hash)
	var saved struct {
		Response []struct {
			ID        int64  `json:"id"`
			OwnerID   int64  `json:"owner_id"`
			AccessKey string `json:"access_key"`
		} `json:"response"`
	}
	if err := c.api(ctx, "photos.saveMessagesPhoto", save, &saved); err != nil {
		return "", err
	}
	if len(saved.Response) == 0 {
		return "", fmt.Errorf("vkdelivery: empty saved photo response")
	}
	photo := saved.Response[0]
	return attachment("photo", photo.OwnerID, photo.ID, photo.AccessKey), nil
}

// UploadVideo uploads a stored video artifact and returns a VK video attachment.
func (c *HTTPClient) UploadVideo(ctx context.Context, peerID int64, filename string, data []byte, mimeType string) (string, error) {
	form := url.Values{}
	form.Set("name", filename)
	form.Set("description", "VK AI Aggregator generated video")
	form.Set("is_private", "1")
	form.Set("peer_id", strconv.FormatInt(peerID, 10))
	var saved struct {
		Response struct {
			UploadURL string `json:"upload_url"`
			OwnerID   int64  `json:"owner_id"`
			VideoID   int64  `json:"video_id"`
			AccessKey string `json:"access_key"`
		} `json:"response"`
	}
	if err := c.api(ctx, "video.save", form, &saved); err != nil {
		return "", err
	}
	if saved.Response.UploadURL == "" {
		return "", fmt.Errorf("vkdelivery: empty video upload_url")
	}
	var uploadResult map[string]any
	if err := c.uploadMultipart(ctx, saved.Response.UploadURL, "video_file", filename, mimeType, data, &uploadResult); err != nil {
		return "", err
	}
	return attachment("video", saved.Response.OwnerID, saved.Response.VideoID, saved.Response.AccessKey), nil
}

type vkErrorEnvelope struct {
	Error *struct {
		Code int    `json:"error_code"`
		Msg  string `json:"error_msg"`
	} `json:"error"`
}

func (c *HTTPClient) api(ctx context.Context, method string, form url.Values, out any) error {
	if form == nil {
		form = url.Values{}
	}
	form.Set("access_token", c.cfg.AccessToken)
	form.Set("v", c.cfg.APIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/"+method, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("vkdelivery: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("vkdelivery: %s: %w", method, err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("vkdelivery: decode %s response: %w", method, err)
	}
	if err := vkEnvelopeError(out); err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("vkdelivery: %s http %d", method, resp.StatusCode)
	}
	return nil
}

func vkEnvelopeError(out any) error {
	raw, err := json.Marshal(out)
	if err != nil {
		return nil
	}
	var env vkErrorEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil
	}
	if env.Error != nil {
		return &APIError{Code: env.Error.Code, Message: env.Error.Msg}
	}
	return nil
}

func (c *HTTPClient) uploadMultipart(ctx context.Context, uploadURL, field, filename, mimeType string, data []byte, out any) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeQuotes(field), escapeQuotes(filename)))
	if mimeType != "" {
		header.Set("Content-Type", mimeType)
	}
	part, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("vkdelivery: create multipart part: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return fmt.Errorf("vkdelivery: write multipart part: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("vkdelivery: close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &body)
	if err != nil {
		return fmt.Errorf("vkdelivery: build upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("vkdelivery: upload media: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("vkdelivery: upload http %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("vkdelivery: decode upload response: %w", err)
	}
	return nil
}

func escapeQuotes(v string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, "\\\"").Replace(v)
}

func attachment(kind string, ownerID, id int64, accessKey string) string {
	ref := fmt.Sprintf("%s%d_%d", kind, ownerID, id)
	if accessKey != "" {
		ref += "_" + accessKey
	}
	return ref
}
