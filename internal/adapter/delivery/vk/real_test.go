package vkdelivery

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func TestHTTPClientVKError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":{"error_code":5,"error_msg":"User authorization failed"}}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(HTTPConfig{AccessToken: "bad", BaseURL: srv.URL, HTTPClient: srv.Client()})
	if _, err := c.SendText(context.Background(), 1, 1, "x"); err == nil {
		t.Fatal("expected error for vk error envelope")
	}
}
