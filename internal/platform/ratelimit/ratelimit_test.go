package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareIgnoresSpoofedXForwardedFor(t *testing.T) {
	limiter := New(1, 1)
	var hits int
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusNoContent)
	}))

	first := httptest.NewRequest(http.MethodPost, "/", nil)
	first.RemoteAddr = "198.51.100.10:1000"
	first.Header.Set("X-Forwarded-For", "203.0.113.1")
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, first)
	if firstRec.Code != http.StatusNoContent {
		t.Fatalf("first status = %d, want %d", firstRec.Code, http.StatusNoContent)
	}

	second := httptest.NewRequest(http.MethodPost, "/", nil)
	second.RemoteAddr = "198.51.100.10:1000"
	second.Header.Set("X-Forwarded-For", "203.0.113.2")
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, second)
	if secondRec.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d", secondRec.Code, http.StatusTooManyRequests)
	}
	if hits != 1 {
		t.Fatalf("handler hits = %d, want 1", hits)
	}
}
