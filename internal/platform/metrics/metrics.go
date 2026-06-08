// Package metrics exposes Prometheus metrics for the platform (audit O1). It
// registers a process-local registry with the default Go/process collectors and
// a set of domain counters covering the request -> job -> provider -> moderation
// -> delivery pipeline, plus an HTTP handler and middleware. Workers and
// handlers increment counters directly; the API serves them at /metrics.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	registry = prometheus.NewRegistry()

	// WebhookReceived counts inbound webhook requests by type.
	WebhookReceived = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_webhook_received_total",
		Help: "Inbound VK webhook requests by event type.",
	}, []string{"type"})

	// JobsTerminal counts jobs that reached a terminal status.
	JobsTerminal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_jobs_terminal_total",
		Help: "Jobs that reached a terminal status, labeled by status.",
	}, []string{"status"})

	// ModerationDecisions counts moderation verdicts by decision.
	ModerationDecisions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_moderation_decisions_total",
		Help: "Output moderation verdicts, labeled by decision.",
	}, []string{"decision"})

	// DLQRouted counts tasks dead-lettered after exhausting their retry budget.
	DLQRouted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_dlq_routed_total",
		Help: "Tasks routed to the dead-letter queue, labeled by phase.",
	}, []string{"phase"})

	// DeliveriesSent counts successful VK deliveries.
	DeliveriesSent = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "vkagg_deliveries_sent_total",
		Help: "Successful VK deliveries.",
	})

	// HTTPRequests counts HTTP requests by path and status.
	HTTPRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_http_requests_total",
		Help: "HTTP requests by route and status code.",
	}, []string{"route", "code"})

	// HTTPDuration tracks HTTP request latency.
	HTTPDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vkagg_http_request_duration_seconds",
		Help:    "HTTP request latency by route.",
		Buckets: prometheus.DefBuckets,
	}, []string{"route"})

	// MaintenanceDeleted counts rows removed by retention cleanup jobs.
	MaintenanceDeleted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_maintenance_deleted_total",
		Help: "Rows deleted by maintenance cleanup jobs, labeled by resource.",
	}, []string{"resource"})

	// StreamTrimmed counts Redis Stream entries trimmed by maintenance.
	StreamTrimmed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_stream_trimmed_total",
		Help: "Redis Stream entries trimmed by maintenance, labeled by stream.",
	}, []string{"stream"})

	// BillingMismatches tracks current balance-vs-ledger mismatches.
	BillingMismatches = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vkagg_billing_mismatches",
		Help: "Number of credit accounts whose cached balance differs from the committed ledger projection.",
	})

	// PaymentsCreated counts newly created payment intents.
	PaymentsCreated = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "payments_created_total",
		Help: "Payment intents created locally, labeled by provider and source.",
	}, []string{"provider", "source"})

	// PaymentsSucceeded counts intents newly moved to succeeded.
	PaymentsSucceeded = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "payments_succeeded_total",
		Help: "Payment intents moved to succeeded, labeled by provider and source.",
	}, []string{"provider", "source"})

	// PaymentsCanceled counts intents newly moved to canceled.
	PaymentsCanceled = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "payments_canceled_total",
		Help: "Payment intents moved to canceled, labeled by provider.",
	}, []string{"provider"})

	// PaymentWebhooks counts provider webhook ingestion results.
	PaymentWebhooks = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "payment_webhooks_total",
		Help: "Payment provider webhooks by provider, event type and ingestion result.",
	}, []string{"provider", "event_type", "result"})

	// PaymentTopups counts committed ledger top-ups from provider-confirmed payments.
	PaymentTopups = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "payment_topups_total",
		Help: "Committed payment top-up ledger entries by provider.",
	}, []string{"provider"})

	// PaymentReconciliationMismatches tracks latest payment reconciliation mismatches.
	PaymentReconciliationMismatches = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "payment_reconciliation_mismatches",
		Help: "Latest count of payment reconciliation mismatches by provider.",
	}, []string{"provider"})
)

func init() {
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		WebhookReceived, JobsTerminal, ModerationDecisions, DLQRouted,
		DeliveriesSent, HTTPRequests, HTTPDuration, MaintenanceDeleted,
		StreamTrimmed, BillingMismatches, PaymentsCreated, PaymentsSucceeded,
		PaymentsCanceled, PaymentWebhooks, PaymentTopups,
		PaymentReconciliationMismatches,
	)
}

// Handler returns the Prometheus metrics HTTP handler.
func Handler() http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

// statusRecorder captures the response status code for metrics.
type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

// Middleware records request count and latency for the given route label.
func Middleware(route string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(rec, r)
		HTTPDuration.WithLabelValues(route).Observe(time.Since(start).Seconds())
		HTTPRequests.WithLabelValues(route, strconv.Itoa(rec.code)).Inc()
	})
}
