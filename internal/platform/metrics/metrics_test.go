package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestPrivateHandlerRejectsPublicForwardedHost(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/metrics", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-Host", "vk.neiirohub.ru")
	w := httptest.NewRecorder()

	PrivateHandler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestPrivateHandlerRejectsPublicForwardedClient(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/metrics", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("CF-Connecting-IP", "198.51.100.10")
	w := httptest.NewRecorder()

	PrivateHandler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestPrivateHandlerRejectsPublicForwardedHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/metrics", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Forwarded", `for=198.51.100.10;host=vk.neiirohub.ru;proto=https`)
	w := httptest.NewRecorder()

	PrivateHandler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestPrivateHandlerRejectsPublicOriginalHost(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/metrics", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Original-Host", "app.neiirohub.ru")
	w := httptest.NewRecorder()

	PrivateHandler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestProductLabelSanitizesAndBoundsValue(t *testing.T) {
	raw := " Payment Flow / Created + secret@example.com " + strings.Repeat("x", 200)
	got := ProductLabel(raw, "fallback")

	if strings.Contains(got, "@") || strings.Contains(got, "+") || strings.Contains(got, " ") {
		t.Fatalf("ProductLabel() kept unsafe characters: %q", got)
	}
	if len(got) > 96 {
		t.Fatalf("ProductLabel() length = %d, want <= 96", len(got))
	}
	if got == "" || got == "fallback" {
		t.Fatalf("ProductLabel() = %q, want sanitized non-fallback label", got)
	}
}
