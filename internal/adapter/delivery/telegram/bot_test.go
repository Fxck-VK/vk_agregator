package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestClientSendMessagePostsFormWithThreadID(t *testing.T) {
	var seenPath string
	var seenForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if contentType := r.Header.Get("Content-Type"); !strings.Contains(contentType, "application/x-www-form-urlencoded") {
			t.Fatalf("Content-Type = %q, want form", contentType)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		seenForm = r.Form
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":77}}`))
	}))
	defer server.Close()

	client := New(Config{
		BotToken:   "TOKEN",
		ChatID:     "-1004435823124",
		ThreadID:   317,
		HTTPClient: rewriteClient(t, server.URL),
	})

	if err := client.SendMessage(context.Background(), "Provider balance bot smoke"); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if seenPath != "/botTOKEN/sendMessage" {
		t.Fatalf("path = %q, want /botTOKEN/sendMessage", seenPath)
	}
	if seenForm.Get("chat_id") != "-1004435823124" {
		t.Fatalf("chat_id = %q", seenForm.Get("chat_id"))
	}
	if seenForm.Get("message_thread_id") != "317" {
		t.Fatalf("message_thread_id = %q", seenForm.Get("message_thread_id"))
	}
	if seenForm.Get("text") != "Provider balance bot smoke" {
		t.Fatalf("text = %q", seenForm.Get("text"))
	}
}

func TestClientSendMessageOmitsZeroThreadID(t *testing.T) {
	var seenForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		seenForm = r.Form
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":77}}`))
	}))
	defer server.Close()

	client := New(Config{
		BotToken:   "TOKEN",
		ChatID:     "-1004435823124",
		HTTPClient: rewriteClient(t, server.URL),
	})

	if err := client.SendMessage(context.Background(), "hello"); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if _, ok := seenForm["message_thread_id"]; ok {
		t.Fatalf("message_thread_id should be omitted when zero: %v", seenForm)
	}
}

func TestClientGetUpdatesParsesMessages(t *testing.T) {
	var seenPath string
	var seenQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"ok": true,
			"result": [
				{
					"update_id": 123,
					"message": {
						"message_thread_id": 317,
						"text": "/balances",
						"chat": {"id": -1004435823124}
					}
				}
			]
		}`))
	}))
	defer server.Close()

	client := New(Config{
		BotToken:   "TOKEN",
		ChatID:     "-1004435823124",
		ThreadID:   317,
		HTTPClient: rewriteClient(t, server.URL),
	})

	updates, err := client.GetUpdates(context.Background(), 124, 30)
	if err != nil {
		t.Fatalf("GetUpdates returned error: %v", err)
	}
	if seenPath != "/botTOKEN/getUpdates" {
		t.Fatalf("path = %q, want /botTOKEN/getUpdates", seenPath)
	}
	if seenQuery.Get("offset") != "124" || seenQuery.Get("timeout") != "30" {
		t.Fatalf("query = %v", seenQuery)
	}
	if len(updates) != 1 {
		t.Fatalf("updates len = %d, want 1", len(updates))
	}
	got := updates[0]
	if got.ID != 123 || got.ChatID != "-1004435823124" || got.ThreadID != 317 || got.Text != "/balances" {
		t.Fatalf("unexpected update: %+v", got)
	}
}

func TestClientErrorsDoNotLeakBotToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad token TOKEN", http.StatusUnauthorized)
	}))
	defer server.Close()

	const token = "TOKEN"
	client := New(Config{
		BotToken:   token,
		ChatID:     "-1004435823124",
		HTTPClient: rewriteClient(t, server.URL),
	})

	err := client.SendMessage(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("error leaked bot token: %v", err)
	}
}

func rewriteClient(t *testing.T, rawBase string) *http.Client {
	t.Helper()
	base, err := url.Parse(rawBase)
	if err != nil {
		t.Fatalf("parse test server url: %v", err)
	}
	return &http.Client{Transport: rewriteTransport{base: base}}
}

type rewriteTransport struct {
	base *url.URL
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = t.base.Scheme
	clone.URL.Host = t.base.Host
	clone.Host = t.base.Host
	return http.DefaultTransport.RoundTrip(clone)
}
