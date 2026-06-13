package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
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

func TestMediaMetricsHelpersUseSanitizedLabels(t *testing.T) {
	ObserveMediaProbe(" Failed ", "Video Generate", "Video", "Probe Failed@example.com")
	counter := mediaProbeCounterValue(t, "failed", "video_generate", "video", "probe_failed_example.com")
	if counter <= 0 {
		t.Fatalf("media probe counter = %v, want > 0", counter)
	}

	ObserveMediaTranscode(" Success ", "Video Generate", "Video", "VK Video", "None")
	ObserveMediaTranscodeByPolicy(" Fallback ", "Success", "None")
	ObserveMediaTranscodeDuration(" Success ", "Video Generate", "Video", "VK Video", time.Second)
	ObserveMediaTranscodeCPUSeconds(" Fallback ", "Success", "None", time.Second)
	ObserveMediaProbeByProvider(" Success ", "None", " DeepInfra ", "Video Class@example.com")
	ObserveMediaBytes("Cleanup", "Video", "VK Video", 4096)
	ObserveMediaUploadValidation(" Mini App ", "Rejected", "Bad File@example.com", "Image/JPEG")
	ObserveMediaUploadBytes(" Mini App ", "Image/JPEG", 4096)
	ObserveMediaUploadPixels(" Mini App ", "Image/JPEG", 1024)
	AddMediaVariantBacklog("Video Generate", "Video", "VK Video", 1)
	AddMediaVariantBacklog("Video Generate", "Video", "VK Video", -1)
	SetMediaQueueBacklog("Video", 2)
	ObserveMediaPolicyDecision(" Worker ", "Video Generate", "Video", "Fallback", "Needs Transcode")
	ObserveMediaFastPath(" Used ")
	ObserveMediaCleanupDeleted("Success", "VK Video", "None")
	ObserveProviderQualityState(" DeepInfra ", "Video Class@example.com", "Video", "Disabled")
	ObserveProviderQualitySample(" DeepInfra ", "Video Class@example.com", "Video", "Failure")
	ObserveProviderOutputInvalid(" DeepInfra ", "Video Class@example.com", "Video", "Probe Failed@example.com")
	AddProductMediaWaste(" DeepInfra ", "Video Class@example.com", "Video", "No Capture@example.com", 10)
	AddMediaProviderWaste(" DeepInfra ", "Video Class@example.com", "No Capture@example.com", 10)
	ObserveMediaDeliveryCaptureGap("Video Generate", "Video", "Capture Failed@example.com")
}

func TestInitPaymentProviderMetricsCreatesZeroProviderErrorSeries(t *testing.T) {
	InitPaymentProviderMetrics("YooKassa")

	counter, err := PaymentProviderErrors.GetMetricWithLabelValues("yookassa", "get_payment", "provider_error")
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues() error = %v", err)
	}
	var metric dto.Metric
	if err := counter.Write(&metric); err != nil {
		t.Fatalf("counter.Write() error = %v", err)
	}
	if metric.Counter == nil {
		t.Fatal("metric counter is nil")
	}
	if got := metric.Counter.GetValue(); got != 0 {
		t.Fatalf("provider error counter = %v, want 0", got)
	}
}

func mediaProbeCounterValue(t *testing.T, labels ...string) float64 {
	t.Helper()
	counter, err := MediaProbeResults.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues() error = %v", err)
	}
	var metric dto.Metric
	if err := counter.Write(&metric); err != nil {
		t.Fatalf("counter.Write() error = %v", err)
	}
	if metric.Counter == nil {
		t.Fatal("metric counter is nil")
	}
	return metric.Counter.GetValue()
}
