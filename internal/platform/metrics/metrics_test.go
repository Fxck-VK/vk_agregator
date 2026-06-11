package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPrivateHandlerRejectsPublicHost(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://app.neiirohub.ru/metrics", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	w := httptest.NewRecorder()

	PrivateHandler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestPrivateHandlerAllowsLocalScrape(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://host.docker.internal:8080/metrics", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()

	PrivateHandler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}
