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
	// VideoAccessToken is a user token with video upload rights. VK video.save
	// is unavailable with group auth, so bot message sending must keep using the
	// community AccessToken while raw video uploads use this token.
	VideoAccessToken string
	// VideoUploadGroupID, when set, saves uploaded videos into that community
	// using VideoAccessToken. The token owner must have sufficient rights in the
	// group. Leave 0 to save as the token owner.
	VideoUploadGroupID int64
	// VideoDeliveryMode selects generated video delivery: "doc" uploads mp4 as
	// a message document, "video" uses video.save and sends a native video
	// attachment with an inline VK player.
	VideoDeliveryMode string
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
	cfg.VideoDeliveryMode = strings.ToLower(strings.TrimSpace(cfg.VideoDeliveryMode))
	if cfg.VideoDeliveryMode == "" {
		cfg.VideoDeliveryMode = "doc"
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &HTTPClient{cfg: cfg, http: httpClient}
}

var (
	_ Client            = (*HTTPClient)(nil)
	_ ControlClient     = (*HTTPClient)(nil)
	_ UserProfileClient = (*HTTPClient)(nil)
	_ MediaUploader     = (*HTTPClient)(nil)
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

// EditMessage edits an existing VK message with optional attachment and
// keyboard. VK returns a boolean-like response for messages.edit, so the
// normalized result keeps the caller-provided message id.
func (c *HTTPClient) EditMessage(ctx context.Context, peerID, messageID int64, msg Message) (SendResult, error) {
	form := url.Values{}
	form.Set("peer_id", strconv.FormatInt(peerID, 10))
	form.Set("message_id", strconv.FormatInt(messageID, 10))
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
	if err := c.api(ctx, "messages.edit", form, &decoded); err != nil {
		return SendResult{}, err
	}
	return SendResult{MessageID: messageID, PeerID: peerID}, nil
}

// AnswerMessageEvent acknowledges a VK callback button click. An empty
// event_data is enough to stop the loading animation without showing a snackbar.
func (c *HTTPClient) AnswerMessageEvent(ctx context.Context, eventID string, userID, peerID int64) error {
	form := url.Values{}
	form.Set("event_id", eventID)
	form.Set("user_id", strconv.FormatInt(userID, 10))
	form.Set("peer_id", strconv.FormatInt(peerID, 10))
	form.Set("event_data", "")

	var decoded vkMessageResponse
	if err := c.api(ctx, "messages.sendMessageEventAnswer", form, &decoded); err != nil {
		return err
	}
	return nil
}

// GetUserProfile fetches a VK user's display name for one-time personalization.
func (c *HTTPClient) GetUserProfile(ctx context.Context, userID int64) (UserProfile, error) {
	form := url.Values{}
	form.Set("user_ids", strconv.FormatInt(userID, 10))

	var decoded struct {
		Response []struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		} `json:"response"`
		Error *struct {
			Code int    `json:"error_code"`
			Msg  string `json:"error_msg"`
		} `json:"error"`
	}
	if err := c.api(ctx, "users.get", form, &decoded); err != nil {
		return UserProfile{}, err
	}
	if len(decoded.Response) == 0 {
		return UserProfile{}, fmt.Errorf("vkdelivery: users.get returned no users")
	}
	u := decoded.Response[0]
	return UserProfile{UserID: u.ID, FirstName: u.FirstName, LastName: u.LastName}, nil
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
	Link    string `json:"link,omitempty"`
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
			actionType := button.ActionType
			if actionType == "" {
				actionType = "text"
			}
			vkRow = append(vkRow, vkKeyboardButton{
				Action: vkKeyboardAction{
					Type:    actionType,
					Label:   button.Label,
					Payload: button.Payload,
					Link:    button.Link,
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

// UploadVideo uploads a stored video artifact as a message document and returns
// a VK doc attachment. VK video.save is unavailable with group auth, while the
// message document upload flow works with the community token already used for
// messages.send.
func (c *HTTPClient) UploadVideo(ctx context.Context, peerID int64, filename string, data []byte, mimeType string) (string, error) {
	if strings.EqualFold(c.cfg.VideoDeliveryMode, "video") {
		return c.UploadVideoFile(ctx, peerID, filename, data, mimeType)
	}
	if filename == "" {
		filename = "video.mp4"
	}
	form := url.Values{}
	form.Set("peer_id", strconv.FormatInt(peerID, 10))
	form.Set("type", "doc")
	var server struct {
		Response struct {
			UploadURL string `json:"upload_url"`
		} `json:"response"`
	}
	if err := c.api(ctx, "docs.getMessagesUploadServer", form, &server); err != nil {
		return "", err
	}
	if server.Response.UploadURL == "" {
		return "", fmt.Errorf("vkdelivery: empty document upload_url")
	}

	var uploaded struct {
		File string `json:"file"`
	}
	if err := c.uploadMultipart(ctx, server.Response.UploadURL, "file", filename, mimeType, data, &uploaded); err != nil {
		return "", err
	}
	if uploaded.File == "" {
		return "", fmt.Errorf("vkdelivery: empty uploaded document file")
	}

	save := url.Values{}
	save.Set("file", uploaded.File)
	save.Set("title", filename)
	var saved struct {
		Response struct {
			Type string `json:"type"`
			Doc  struct {
				ID        int64  `json:"id"`
				OwnerID   int64  `json:"owner_id"`
				AccessKey string `json:"access_key"`
			} `json:"doc"`
		} `json:"response"`
	}
	if err := c.api(ctx, "docs.save", save, &saved); err != nil {
		return "", err
	}
	if saved.Response.Doc.ID == 0 {
		return "", fmt.Errorf("vkdelivery: empty saved document response")
	}
	return attachment("doc", saved.Response.Doc.OwnerID, saved.Response.Doc.ID, saved.Response.Doc.AccessKey), nil
}

// UploadVideoFile uploads a stored video artifact through video.save and returns
// a VK video attachment. It is retained for environments that can provide a user
// token with VK video rights, but regular bot delivery should prefer
// UploadVideo's message-document flow.
func (c *HTTPClient) UploadVideoFile(ctx context.Context, peerID int64, filename string, data []byte, mimeType string) (string, error) {
	token := strings.TrimSpace(c.cfg.VideoAccessToken)
	if token == "" {
		return "", fmt.Errorf("vkdelivery: VK_VIDEO_ACCESS_TOKEN is required for video.save uploads")
	}
	form := url.Values{}
	form.Set("name", filename)
	form.Set("description", "VK AI Aggregator generated video")
	form.Set("is_private", "1")
	form.Set("peer_id", strconv.FormatInt(peerID, 10))
	if c.cfg.VideoUploadGroupID > 0 {
		form.Set("group_id", strconv.FormatInt(c.cfg.VideoUploadGroupID, 10))
	}
	var saved struct {
		Response struct {
			UploadURL string `json:"upload_url"`
			OwnerID   int64  `json:"owner_id"`
			VideoID   int64  `json:"video_id"`
			AccessKey string `json:"access_key"`
		} `json:"response"`
	}
	if err := c.apiWithToken(ctx, token, "video.save", form, &saved); err != nil {
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
	return c.apiWithToken(ctx, c.cfg.AccessToken, method, form, out)
}

func (c *HTTPClient) apiWithToken(ctx context.Context, token, method string, form url.Values, out any) error {
	if form == nil {
		form = url.Values{}
	}
	form.Set("access_token", token)
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

	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("vkdelivery: read %s response: %w", method, err)
	}
	if err := vkEnvelopeErrorBytes(data); err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("vkdelivery: decode %s response: %w", method, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("vkdelivery: %s http %d", method, resp.StatusCode)
	}
	return nil
}

func vkEnvelopeErrorBytes(raw []byte) error {
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
