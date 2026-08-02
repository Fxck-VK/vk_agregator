package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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
	ObserveVideoRouteSubmit(" PoYo ", "Video Kling O3@example.com", "Rate Limited@example.com")
	AddVideoRouteActualCost(" PoYo ", "Video Kling O3@example.com", "Credits ", 10)
	ObserveVideoRouteEstimateActualDelta(" PoYo ", "Video Kling O3@example.com", "Success ", -2)
	ObserveVideoRouteSubmitToComplete(" PoYo ", "Video Kling O3@example.com", "Succeeded ", time.Second)
	ObserveVideoRouteProviderTaskFailure(" PoYo ", "Video Kling O3@example.com", "Provider Timeout@example.com")
	ObserveVideoRouteMediaFailure(" PoYo ", "Video Kling O3@example.com", "Download ", "Private URL Expired@example.com")
	ObserveVideoRouteBilling(" PoYo ", "Video Kling O3@example.com", "Capture ", "Success ")
	SetVideoRouteProviderBalance(" PoYo ", 100)
	routeCounter := counterValue(t, VideoRouteSubmit, "poyo", "video_kling_o3_example.com", "rate_limited_example.com")
	if routeCounter <= 0 {
		t.Fatalf("video route submit counter = %v, want > 0", routeCounter)
	}
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

func TestOutboxRelayMetricLabelsAreFiniteAndBounded(t *testing.T) {
	for _, test := range []struct {
		name string
		got  string
		want string
	}{
		{name: "known queued class", got: outboxEventClass("event.job.queued"), want: "queued"},
		{name: "known conversation title class", got: outboxEventClass("event.conversation_title.queued"), want: "conversation_title"},
		{name: "unknown event does not escape", got: outboxEventClass("event.job.secret." + strings.Repeat("x", 200)), want: "unknown"},
		{name: "known outcome", got: outboxOutcome("retry"), want: "retry"},
		{name: "raw outcome does not escape", got: outboxOutcome("job-" + strings.Repeat("1", 100)), want: "other"},
		{name: "known failure", got: outboxFailureClass("publish_error"), want: "publish_error"},
		{name: "raw error does not escape", got: outboxFailureClass("redis password=secret"), want: "other"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("bounded label = %q, want %q", test.got, test.want)
			}
		})
	}
}

func TestOutboxRelayMetricHelpersRecordBoundedOutcomes(t *testing.T) {
	ObserveOutboxRelayClaim("event.job.queued", "claimed")
	ObserveOutboxRelayClaimDuration("claimed", time.Second)
	ObserveOutboxRelayPublish("event.job.queued", "success", "none")
	ObserveOutboxRelayResolution("event.job.queued", "retry", "publish_error")
	ObserveOutboxRelayLeaseRecovery("event.job.queued", "recovered")

	if got := counterValue(t, OutboxRelayClaims, "queued", "claimed"); got <= 0 {
		t.Fatalf("claim counter = %v, want > 0", got)
	}
	if got := counterValue(t, OutboxRelayPublishes, "queued", "success", "none"); got <= 0 {
		t.Fatalf("publish counter = %v, want > 0", got)
	}
	if got := counterValue(t, OutboxRelayResolutions, "queued", "retry", "publish_error"); got <= 0 {
		t.Fatalf("resolution counter = %v, want > 0", got)
	}
	if got := counterValue(t, OutboxRelayLeaseRecoveries, "queued", "recovered"); got <= 0 {
		t.Fatalf("lease recovery counter = %v, want > 0", got)
	}
}

func TestScalableFinalizationMetricHelpersRecordBoundedSamples(t *testing.T) {
	publishBefore := histogramCount(t, OutboxRelayPublishDuration, "unknown", "success")
	reconcileBefore := histogramCount(t, ResultReadyReconciliationDuration, "error")
	blockedBefore := counterValue(t, ResultReadyReconciliationItems, "blocked")
	readinessBefore := counterValue(t, FinalizationReadinessFailures, "unknown", "other")
	captureBefore := histogramCount(t, ResultFinalizationCaptureDuration, "unknown")

	ObserveOutboxRelayClaimToAcknowledgedPublicationDuration("private."+strings.Repeat("x", 100), time.Second)
	ObserveResultReadyReconciliationDuration("error", time.Second)
	AddResultReadyReconciliationItems("blocked", 2)
	ObserveFinalizationReadinessFailure("private-mode", "raw secret")
	ObserveResultFinalizationCaptureDuration("private-mode", time.Second)

	if got := histogramCount(t, OutboxRelayPublishDuration, "unknown", "success"); got != publishBefore+1 {
		t.Fatalf("publish duration samples = %d, want %d", got, publishBefore+1)
	}
	if got := histogramCount(t, ResultReadyReconciliationDuration, "error"); got != reconcileBefore+1 {
		t.Fatalf("reconciliation duration samples = %d, want %d", got, reconcileBefore+1)
	}
	if got := counterValue(t, ResultReadyReconciliationItems, "blocked"); got != blockedBefore+2 {
		t.Fatalf("blocked reconciliation items = %v, want %v", got, blockedBefore+2)
	}
	if got := counterValue(t, FinalizationReadinessFailures, "unknown", "other"); got != readinessBefore+1 {
		t.Fatalf("bounded readiness failures = %v, want %v", got, readinessBefore+1)
	}
	if got := histogramCount(t, ResultFinalizationCaptureDuration, "unknown"); got != captureBefore+1 {
		t.Fatalf("capture latency samples = %d, want %d", got, captureBefore+1)
	}
}

func histogramCount(t *testing.T, histogram *prometheus.HistogramVec, labels ...string) uint64 {
	t.Helper()
	observer, err := histogram.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues() error = %v", err)
	}
	metricWriter, ok := observer.(prometheus.Metric)
	if !ok {
		t.Fatal("histogram observer does not implement prometheus.Metric")
	}
	var metric dto.Metric
	if err := metricWriter.Write(&metric); err != nil {
		t.Fatalf("histogram.Write() error = %v", err)
	}
	return metric.GetHistogram().GetSampleCount()
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

func counterValue(t *testing.T, counter *prometheus.CounterVec, labels ...string) float64 {
	t.Helper()
	metricCounter, err := counter.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues() error = %v", err)
	}
	var metric dto.Metric
	if err := metricCounter.Write(&metric); err != nil {
		t.Fatalf("counter.Write() error = %v", err)
	}
	if metric.Counter == nil {
		t.Fatal("metric counter is nil")
	}
	return metric.Counter.GetValue()
}
