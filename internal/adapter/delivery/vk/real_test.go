package vkdelivery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPClientSendText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("peer_id") != "42" || r.FormValue("random_id") == "" {
			t.Errorf("missing peer/random id: %v", r.Form)
		}
		if r.FormValue("message") != "hello" {
			t.Errorf("message = %q", r.FormValue("message"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":12345}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(HTTPConfig{AccessToken: "tok", BaseURL: srv.URL, HTTPClient: srv.Client()})
	res, err := c.SendText(context.Background(), 42, DeterministicRandomID("k"), "hello")
	if err != nil {
		t.Fatalf("send text: %v", err)
	}
	if res.MessageID != 12345 || res.PeerID != 42 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestHTTPClientSendMessageWithKeyboard(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("message") != "menu" {
			t.Errorf("message = %q", r.FormValue("message"))
		}
		var keyboard vkKeyboard
		if err := json.Unmarshal([]byte(r.FormValue("keyboard")), &keyboard); err != nil {
			t.Fatalf("keyboard json: %v", err)
		}
		if len(keyboard.Buttons) != 1 || len(keyboard.Buttons[0]) != 1 {
			t.Fatalf("unexpected keyboard: %+v", keyboard)
		}
		button := keyboard.Buttons[0][0]
		if button.Action.Type != "text" || button.Action.Label != "🎬 Создать видео" || button.Color != "primary" {
			t.Fatalf("unexpected button: %+v", button)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":12346}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(HTTPConfig{AccessToken: "tok", BaseURL: srv.URL, HTTPClient: srv.Client()})
	res, err := c.SendMessage(context.Background(), 42, 11, Message{
		Text: "menu",
		Keyboard: &Keyboard{Buttons: [][]KeyboardButton{{
			{Label: "🎬 Создать видео", Payload: `{"command":"menu.video"}`, Color: "primary"},
		}}},
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if res.MessageID != 12346 {
		t.Fatalf("message_id = %d", res.MessageID)
	}
}

func TestHTTPClientSendMessageWithCallbackKeyboard(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		var keyboard vkKeyboard
		if err := json.Unmarshal([]byte(r.FormValue("keyboard")), &keyboard); err != nil {
			t.Fatalf("keyboard json: %v", err)
		}
		button := keyboard.Buttons[0][0]
		if button.Action.Type != "callback" || button.Action.Label != "Open" || button.Action.Payload != `{"command":"menu.video"}` {
			t.Fatalf("unexpected callback button: %+v", button)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":12347}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(HTTPConfig{AccessToken: "tok", BaseURL: srv.URL, HTTPClient: srv.Client()})
	if _, err := c.SendMessage(context.Background(), 42, 12, Message{
		Text: "menu",
		Keyboard: &Keyboard{Inline: true, Buttons: [][]KeyboardButton{{
			{Label: "Open", Payload: `{"command":"menu.video"}`, Color: "primary", ActionType: "callback"},
		}}},
	}); err != nil {
		t.Fatalf("send callback keyboard: %v", err)
	}
}

func TestHTTPClientEditMessageWithKeyboard(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages.edit" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = r.ParseForm()
		if r.FormValue("peer_id") != "42" || r.FormValue("message_id") != "777" {
			t.Errorf("missing peer/message id: %v", r.Form)
		}
		if r.FormValue("message") != "updated menu" {
			t.Errorf("message = %q", r.FormValue("message"))
		}
		var keyboard vkKeyboard
		if err := json.Unmarshal([]byte(r.FormValue("keyboard")), &keyboard); err != nil {
			t.Fatalf("keyboard json: %v", err)
		}
		if len(keyboard.Buttons) != 1 || keyboard.Buttons[0][0].Action.Label != "Back" {
			t.Fatalf("unexpected keyboard: %+v", keyboard)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":1}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(HTTPConfig{AccessToken: "tok", BaseURL: srv.URL, HTTPClient: srv.Client()})
	res, err := c.EditMessage(context.Background(), 42, 777, Message{
		Text: "updated menu",
		Keyboard: &Keyboard{Buttons: [][]KeyboardButton{{
			{Label: "Back", Payload: `{"command":"show_menu"}`, Color: "secondary"},
		}}},
	})
	if err != nil {
		t.Fatalf("edit message: %v", err)
	}
	if res.MessageID != 777 || res.PeerID != 42 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestHTTPClientAnswerMessageEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages.sendMessageEventAnswer" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = r.ParseForm()
		if r.FormValue("event_id") != "evt-button" || r.FormValue("user_id") != "7" || r.FormValue("peer_id") != "42" {
			t.Fatalf("unexpected form: %v", r.Form)
		}
		if r.FormValue("event_data") != "" {
			t.Fatalf("event_data = %q, want blank answer", r.FormValue("event_data"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":1}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(HTTPConfig{AccessToken: "tok", BaseURL: srv.URL, HTTPClient: srv.Client()})
	if err := c.AnswerMessageEvent(context.Background(), "evt-button", 7, 42); err != nil {
		t.Fatalf("answer message event: %v", err)
	}
}

func TestHTTPClientGetUserProfile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users.get" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = r.ParseForm()
		if r.FormValue("user_ids") != "777" {
			t.Fatalf("user_ids = %q", r.FormValue("user_ids"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":[{"id":777,"first_name":"Сергей","last_name":"Макаров"}]}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(HTTPConfig{AccessToken: "tok", BaseURL: srv.URL, HTTPClient: srv.Client()})
	profile, err := c.GetUserProfile(context.Background(), 777)
	if err != nil {
		t.Fatalf("get user profile: %v", err)
	}
	if profile.UserID != 777 || profile.FirstName != "Сергей" || profile.LastName != "Макаров" {
		t.Fatalf("unexpected profile: %+v", profile)
	}
}

func TestHTTPClientVKError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":{"error_code":5,"error_msg":"User authorization failed"}}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(HTTPConfig{AccessToken: "bad", BaseURL: srv.URL, HTTPClient: srv.Client()})
	if _, err := c.SendText(context.Background(), 1, 1, "x"); err == nil {
		t.Fatal("expected error for vk error envelope")
	} else if !IsAPIErrorCode(err, 5) {
		t.Fatalf("expected API error code 5, got %v", err)
	}
}

func TestHTTPClientUploadPhoto(t *testing.T) {
	var uploadHit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/photos.getMessagesUploadServer":
			_ = r.ParseForm()
			if r.FormValue("peer_id") != "42" {
				t.Errorf("peer_id = %q", r.FormValue("peer_id"))
			}
			_, _ = w.Write([]byte(`{"response":{"upload_url":"` + "http://" + r.Host + `/upload_photo"}}`))
		case "/upload_photo":
			uploadHit = true
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatalf("parse multipart: %v", err)
			}
			if r.MultipartForm.File["photo"] == nil {
				t.Fatalf("missing photo upload field")
			}
			_, _ = w.Write([]byte(`{"server":7,"photo":"[]","hash":"h"}`))
		case "/photos.saveMessagesPhoto":
			_ = r.ParseForm()
			if r.FormValue("server") != "7" || r.FormValue("hash") != "h" {
				t.Errorf("unexpected save form: %v", r.Form)
			}
			_, _ = w.Write([]byte(`{"response":[{"id":456,"owner_id":123,"access_key":"ak"}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := NewHTTPClient(HTTPConfig{AccessToken: "tok", BaseURL: srv.URL, HTTPClient: srv.Client()})
	attachment, err := c.UploadPhoto(context.Background(), 42, "out.png", []byte("png"), "image/png")
	if err != nil {
		t.Fatalf("upload photo: %v", err)
	}
	if !uploadHit || attachment != "photo123_456_ak" {
		t.Fatalf("attachment = %q uploadHit=%v", attachment, uploadHit)
	}
}

func TestHTTPClientUploadVideo(t *testing.T) {
	var uploadHit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/video.save":
			_ = r.ParseForm()
			if !strings.Contains(r.FormValue("name"), ".mp4") {
				t.Errorf("name = %q", r.FormValue("name"))
			}
			_, _ = w.Write([]byte(`{"response":{"upload_url":"` + "http://" + r.Host + `/upload_video","owner_id":-10,"video_id":99,"access_key":"vk"}}`))
		case "/upload_video":
			uploadHit = true
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatalf("parse multipart: %v", err)
			}
			if r.MultipartForm.File["video_file"] == nil {
				t.Fatalf("missing video upload field")
			}
			_, _ = w.Write([]byte(`{"size":9}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := NewHTTPClient(HTTPConfig{AccessToken: "tok", BaseURL: srv.URL, HTTPClient: srv.Client()})
	attachment, err := c.UploadVideo(context.Background(), 42, "out.mp4", []byte("mp4"), "video/mp4")
	if err != nil {
		t.Fatalf("upload video: %v", err)
	}
	if !uploadHit || attachment != "video-10_99_vk" {
		t.Fatalf("attachment = %q uploadHit=%v", attachment, uploadHit)
	}
}
