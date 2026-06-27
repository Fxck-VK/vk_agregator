package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultAPIBaseURL = "https://api.telegram.org"

type Client struct {
	token  string
	chatID string
	thread int64
	http   *http.Client
}

type Config struct {
	BotToken   string
	ChatID     string
	ThreadID   int64
	HTTPClient *http.Client
}

type Update struct {
	ID       int64
	ChatID   string
	ThreadID int64
	Text     string
}

func New(cfg Config) *Client {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 35 * time.Second}
	}
	return &Client{
		token:  strings.TrimSpace(cfg.BotToken),
		chatID: strings.TrimSpace(cfg.ChatID),
		thread: cfg.ThreadID,
		http:   httpClient,
	}
}

func (c *Client) SendMessage(ctx context.Context, text string) error {
	form := url.Values{}
	form.Set("chat_id", c.chatID)
	form.Set("text", text)
	if c.thread != 0 {
		form.Set("message_thread_id", strconv.FormatInt(c.thread, 10))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.methodURL("sendMessage"), strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("telegram: build sendMessage request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var decoded telegramEnvelope
	if err := c.do(req, "sendMessage", &decoded); err != nil {
		return err
	}
	return nil
}

func (c *Client) GetUpdates(ctx context.Context, offset int64, timeoutSec int) ([]Update, error) {
	endpoint, err := url.Parse(c.methodURL("getUpdates"))
	if err != nil {
		return nil, fmt.Errorf("telegram: build getUpdates request")
	}
	query := endpoint.Query()
	if offset != 0 {
		query.Set("offset", strconv.FormatInt(offset, 10))
	}
	if timeoutSec > 0 {
		query.Set("timeout", strconv.Itoa(timeoutSec))
	}
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("telegram: build getUpdates request")
	}

	var decoded updatesResponse
	if err := c.do(req, "getUpdates", &decoded); err != nil {
		return nil, err
	}
	updates := make([]Update, 0, len(decoded.Result))
	for _, item := range decoded.Result {
		if item.Message == nil {
			continue
		}
		updates = append(updates, Update{
			ID:       item.UpdateID,
			ChatID:   strconv.FormatInt(item.Message.Chat.ID, 10),
			ThreadID: item.Message.ThreadID,
			Text:     item.Message.Text,
		})
	}
	return updates, nil
}

func (c *Client) methodURL(method string) string {
	return defaultAPIBaseURL + "/bot" + c.token + "/" + method
}

func (c *Client) do(req *http.Request, method string, out telegramResponse) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return telegramTransportError(method, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("telegram: %s http %d", method, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("telegram: decode %s response: %w", method, err)
	}
	if !out.OK() {
		code := out.ErrorCode()
		if code == 0 {
			return fmt.Errorf("telegram: %s API error", method)
		}
		return fmt.Errorf("telegram: %s API error %d", method, code)
	}
	return nil
}

type telegramResponse interface {
	OK() bool
	ErrorCode() int
}

type telegramEnvelope struct {
	Ok          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
	Code        int    `json:"error_code,omitempty"`
}

func (r *telegramEnvelope) OK() bool {
	return r.Ok
}

func (r *telegramEnvelope) ErrorCode() int {
	return r.Code
}

type updatesResponse struct {
	Ok     bool             `json:"ok"`
	Code   int              `json:"error_code,omitempty"`
	Result []telegramUpdate `json:"result"`
}

func (r *updatesResponse) OK() bool {
	return r.Ok
}

func (r *updatesResponse) ErrorCode() int {
	return r.Code
}

type telegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *telegramMessage `json:"message,omitempty"`
}

type telegramMessage struct {
	ThreadID int64        `json:"message_thread_id,omitempty"`
	Text     string       `json:"text,omitempty"`
	Chat     telegramChat `json:"chat"`
}

type telegramChat struct {
	ID int64 `json:"id"`
}

func telegramTransportError(method string, err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("telegram: %s request failed: context canceled", method)
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("telegram: %s request failed: context deadline exceeded", method)
	default:
		return fmt.Errorf("telegram: %s request failed", method)
	}
}
