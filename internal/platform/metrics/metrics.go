// Package metrics exposes Prometheus metrics for the platform (audit O1). It
// registers a process-local registry with the default Go/process collectors and
// a set of domain counters covering the request -> job -> provider -> moderation
// -> delivery pipeline, plus an HTTP handler and middleware. Workers and
// handlers increment counters directly; the API serves them at /metrics.
package metrics

import (
	"net"
	"net/http"
	"strconv"
	"strings"
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

	// PaymentWebhookSecurityDenials counts webhook requests rejected before
	// provider payload parsing.
	PaymentWebhookSecurityDenials = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "payment_webhook_security_denials_total",
		Help: "Payment provider webhook requests rejected by security checks.",
	}, []string{"provider", "reason"})

	// PaymentWebhookProcessingErrors counts async webhook inbox processing
	// failures by coarse stage without logging raw provider payloads.
	PaymentWebhookProcessingErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "payment_webhook_processing_errors_total",
		Help: "Payment provider webhook inbox processing errors by provider and stage.",
	}, []string{"provider", "stage"})

	// PaymentWebhookUnprocessedEvents tracks the current payment webhook inbox
	// backlog. It is updated by cmd/provider-webhook after processing ticks and
	// readiness probes.
	PaymentWebhookUnprocessedEvents = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "payment_webhook_unprocessed_events",
		Help: "Current count of unprocessed payment provider webhook inbox events by provider.",
	}, []string{"provider"})

	// PaymentWebhookOldestUnprocessedAgeSeconds tracks how long the oldest
	// unprocessed webhook has been waiting in the inbox.
	PaymentWebhookOldestUnprocessedAgeSeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "payment_webhook_oldest_unprocessed_age_seconds",
		Help: "Age in seconds of the oldest unprocessed payment provider webhook inbox event by provider.",
	}, []string{"provider"})

	// PaymentProviderErrors counts payment provider API operation failures.
	PaymentProviderErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "payment_provider_errors_total",
		Help: "Payment provider API operation errors by provider, operation and coarse error class.",
	}, []string{"provider", "operation", "error_class"})

	// PaymentTopups counts committed ledger top-ups from provider-confirmed payments.
	PaymentTopups = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "payment_topups_total",
		Help: "Committed payment top-up ledger entries by provider.",
	}, []string{"provider"})

	// PaymentRefunds counts protected manual refund attempts/results.
	PaymentRefunds = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "payment_refunds_total",
		Help: "Manual payment refunds by provider and result.",
	}, []string{"provider", "result"})

	// PaymentReconciliationMismatches tracks latest payment reconciliation mismatches.
	PaymentReconciliationMismatches = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "payment_reconciliation_mismatches",
		Help: "Latest count of payment reconciliation mismatches by provider.",
	}, []string{"provider"})

	// QueueDepth tracks Redis stream length by stream.
	QueueDepth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vkagg_queue_depth",
		Help: "Current Redis Stream length by stream.",
	}, []string{"stream"})

	// QueueOldestAgeSeconds tracks the oldest entry age by stream.
	QueueOldestAgeSeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vkagg_queue_oldest_age_seconds",
		Help: "Age in seconds of the oldest Redis Stream entry by stream.",
	}, []string{"stream"})

	// QueueConsumerLag tracks pending entries for a consumer group by stream.
	QueueConsumerLag = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vkagg_queue_consumer_lag",
		Help: "Pending Redis Stream entries by stream and consumer group.",
	}, []string{"stream", "group"})

	// StuckJobs tracks jobs that appear stuck in a non-terminal state.
	StuckJobs = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vkagg_stuck_jobs",
		Help: "Current count of jobs that appear stuck, labeled by status and modality.",
	}, []string{"status", "modality"})

	// WorkerTaskDuration tracks worker handler time by phase and result.
	WorkerTaskDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vkagg_worker_task_duration_seconds",
		Help:    "Worker task handler duration by phase, operation, modality and result.",
		Buckets: prometheus.DefBuckets,
	}, []string{"phase", "operation", "modality", "result"})

	// WorkerRetries counts worker retry decisions by phase.
	WorkerRetries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_worker_retries_total",
		Help: "Worker retry decisions by phase, operation and modality.",
	}, []string{"phase", "operation", "modality"})

	// JobsCreated counts newly-created jobs by source surface and operation.
	JobsCreated = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_jobs_created_total",
		Help: "Jobs created by source surface, operation and modality.",
	}, []string{"surface", "operation", "modality"})

	// JobDuration tracks time from job creation to terminal status.
	JobDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vkagg_job_duration_seconds",
		Help:    "Job duration from creation to terminal status by operation, modality and status.",
		Buckets: []float64{1, 2, 5, 10, 30, 60, 120, 300, 600, 1800},
	}, []string{"operation", "modality", "status"})

	// JobStatusCurrent tracks observed status transitions as a low-cardinality gauge.
	JobStatusCurrent = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vkagg_job_status_current",
		Help: "Observed current jobs by status, operation and modality. Values are transition-based and process-local.",
	}, []string{"status", "operation", "modality"})

	// JobRejected counts rejected jobs by coarse reason class.
	JobRejected = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_job_rejected_total",
		Help: "Rejected jobs by reason class and modality.",
	}, []string{"reason_class", "modality"})

	// ProviderRequests counts provider Submit/Poll/Cancel calls by result.
	ProviderRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_provider_requests_total",
		Help: "Provider calls by provider, model, operation and result.",
	}, []string{"provider", "model", "operation", "result"})

	// ProviderRequestDuration tracks provider call duration.
	ProviderRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vkagg_provider_request_duration_seconds",
		Help:    "Provider call duration by provider, model and operation.",
		Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60, 120, 300},
	}, []string{"provider", "model", "operation"})

	// ProviderErrors counts provider errors by normalized error class.
	ProviderErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_provider_errors_total",
		Help: "Provider errors by provider, model, operation and normalized error class.",
	}, []string{"provider", "model", "operation", "error_class"})

	// ProviderRateLimits counts provider rate-limit responses.
	ProviderRateLimits = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_provider_rate_limits_total",
		Help: "Provider rate-limit responses by provider, model and operation.",
	}, []string{"provider", "model", "operation"})

	// ProviderFallback counts routed provider fallbacks.
	ProviderFallback = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_provider_fallback_total",
		Help: "Provider fallback transitions by source provider, target provider, operation and reason.",
	}, []string{"from_provider", "to_provider", "operation", "reason"})

	// ProviderCircuitState reports 1 when a provider breaker is open.
	ProviderCircuitState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vkagg_provider_circuit_state",
		Help: "Provider circuit breaker state, 1=open and 0=closed.",
	}, []string{"provider", "operation"})

	// ProviderTokens counts provider token usage when adapters expose it.
	ProviderTokens = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_provider_tokens_total",
		Help: "Provider token usage by provider, model, operation and direction.",
	}, []string{"provider", "model", "operation", "direction"})

	// ProviderImages counts provider image outputs.
	ProviderImages = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_provider_images_total",
		Help: "Provider image outputs by provider, model and operation.",
	}, []string{"provider", "model", "operation"})

	// ProviderVideos counts provider video outputs.
	ProviderVideos = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_provider_videos_total",
		Help: "Provider video outputs by provider, model and operation.",
	}, []string{"provider", "model", "operation"})

	// ProviderEstimatedCost counts estimated provider-side credits spent.
	ProviderEstimatedCost = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_provider_estimated_cost_total",
		Help: "Estimated provider cost by provider, model, operation and currency.",
	}, []string{"provider", "model", "operation", "currency"})

	// BillingReservations counts reservation attempts.
	BillingReservations = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_billing_reservations_total",
		Help: "Billing reservation attempts by operation and result.",
	}, []string{"operation", "result"})

	// BillingCaptures counts capture attempts.
	BillingCaptures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_billing_captures_total",
		Help: "Billing capture attempts by operation and result.",
	}, []string{"operation", "result"})

	// BillingReleases counts release attempts.
	BillingReleases = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_billing_releases_total",
		Help: "Billing release attempts by operation and result.",
	}, []string{"operation", "result"})

	// LedgerEntries counts logical ledger entries by type and source.
	LedgerEntries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_ledger_entries_total",
		Help: "Logical ledger entries by entry type and source.",
	}, []string{"entry_type", "source"})

	// PaymentToLedgerDuration tracks time from intent creation to ledger top-up when known.
	PaymentToLedgerDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vkagg_payment_to_ledger_duration_seconds",
		Help:    "Duration from provider payment creation to ledger top-up by provider.",
		Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800},
	}, []string{"provider"})

	// ReferralRewards counts referral reward outcomes.
	ReferralRewards = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_referral_rewards_total",
		Help: "Referral reward outcomes.",
	}, []string{"result"})

	// FrontendEvents counts safe client telemetry events.
	FrontendEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_frontend_events_total",
		Help: "Safe frontend telemetry events by surface and event type.",
	}, []string{"surface", "event_type"})

	// FrontendJSErrors counts safe JavaScript error classes.
	FrontendJSErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_frontend_js_errors_total",
		Help: "Safe frontend JavaScript errors by surface, screen and class.",
	}, []string{"surface", "screen", "error_class"})

	// FrontendAPIFailures counts safe frontend-observed API failures.
	FrontendAPIFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_frontend_api_failures_total",
		Help: "Frontend-observed API failures by surface, route and status.",
	}, []string{"surface", "route", "status"})

	// FrontendLaunchFailures counts safe Mini App launch failures.
	FrontendLaunchFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_frontend_launch_failures_total",
		Help: "Frontend launch failures by surface and reason.",
	}, []string{"surface", "reason"})

	// FrontendPaymentFlowErrors counts frontend payment flow failures.
	FrontendPaymentFlowErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_frontend_payment_flow_errors_total",
		Help: "Frontend payment flow errors by step and error class.",
	}, []string{"step", "error_class"})

	// VKDeliveryAttempts counts VK delivery attempts by type and result.
	VKDeliveryAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_vk_delivery_attempts_total",
		Help: "VK delivery attempts by kind, result and error class.",
	}, []string{"kind", "result", "error_class"})

	// VKDeliveryDuration tracks VK delivery duration.
	VKDeliveryDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vkagg_vk_delivery_duration_seconds",
		Help:    "VK delivery duration by kind.",
		Buckets: prometheus.DefBuckets,
	}, []string{"kind"})

	// VKUploadFailures counts VK media upload failures.
	VKUploadFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_vk_upload_failures_total",
		Help: "VK media upload failures by media type and error class.",
	}, []string{"media_type", "error_class"})

	// VKMenuControlErrors counts VK control/menu errors.
	VKMenuControlErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_vk_menu_control_errors_total",
		Help: "VK menu/control errors by command type and error class.",
	}, []string{"command_type", "error_class"})

	// AuthFailures counts auth failures by surface and reason.
	AuthFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_auth_failures_total",
		Help: "Authentication failures by surface and reason.",
	}, []string{"surface", "reason"})

	// SignatureFailures counts signature failures by surface and reason.
	SignatureFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_signature_failures_total",
		Help: "Signature verification failures by surface and reason.",
	}, []string{"surface", "reason"})

	// AdminActions counts protected admin/operator action outcomes.
	AdminActions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_admin_actions_total",
		Help: "Protected admin/operator actions by action and result.",
	}, []string{"action", "result"})

	// SuspiciousEvents counts coarse suspicious events.
	SuspiciousEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_suspicious_events_total",
		Help: "Suspicious events by surface and type.",
	}, []string{"surface", "type"})

	// ConfigValidationFailures counts config validation failures by service and reason.
	ConfigValidationFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_config_validation_failures_total",
		Help: "Config validation failures by service and reason.",
	}, []string{"service", "reason"})

	// BackupLastSuccessTimestamp records the last successful backup timestamp.
	BackupLastSuccessTimestamp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vkagg_backup_last_success_timestamp",
		Help: "Unix timestamp of the last successful backup by target.",
	}, []string{"target"})

	// BackupDuration tracks backup duration.
	BackupDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vkagg_backup_duration_seconds",
		Help:    "Backup duration by target and result.",
		Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800},
	}, []string{"target", "result"})

	// BackupSizeBytes records backup artifact size by target.
	BackupSizeBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vkagg_backup_size_bytes",
		Help: "Backup artifact size in bytes by target.",
	}, []string{"target"})

	// BackupFailures counts backup failures.
	BackupFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_backup_failures_total",
		Help: "Backup failures by target and reason.",
	}, []string{"target", "reason"})

	// RestoreTestLastSuccessTimestamp records the last successful restore test timestamp.
	RestoreTestLastSuccessTimestamp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vkagg_restore_test_last_success_timestamp",
		Help: "Unix timestamp of the last successful restore test by target.",
	}, []string{"target"})
)

func init() {
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		WebhookReceived, JobsTerminal, ModerationDecisions, DLQRouted,
		DeliveriesSent, HTTPRequests, HTTPDuration, MaintenanceDeleted,
		StreamTrimmed, BillingMismatches, PaymentsCreated, PaymentsSucceeded,
		PaymentsCanceled, PaymentWebhooks, PaymentWebhookSecurityDenials,
		PaymentWebhookProcessingErrors, PaymentWebhookUnprocessedEvents,
		PaymentWebhookOldestUnprocessedAgeSeconds, PaymentProviderErrors,
		PaymentTopups, PaymentRefunds, PaymentReconciliationMismatches,
		QueueDepth, QueueOldestAgeSeconds, QueueConsumerLag, StuckJobs,
		WorkerTaskDuration, WorkerRetries, JobsCreated, JobDuration,
		JobStatusCurrent, JobRejected, ProviderRequests, ProviderRequestDuration,
		ProviderErrors, ProviderRateLimits, ProviderFallback, ProviderCircuitState,
		ProviderTokens, ProviderImages, ProviderVideos, ProviderEstimatedCost,
		BillingReservations, BillingCaptures, BillingReleases, LedgerEntries,
		PaymentToLedgerDuration, ReferralRewards, FrontendEvents, FrontendJSErrors,
		FrontendAPIFailures, FrontendLaunchFailures, FrontendPaymentFlowErrors,
		VKDeliveryAttempts, VKDeliveryDuration, VKUploadFailures,
		VKMenuControlErrors, AuthFailures, SignatureFailures, AdminActions,
		SuspiciousEvents, ConfigValidationFailures, BackupLastSuccessTimestamp,
		BackupDuration, BackupSizeBytes, BackupFailures,
		RestoreTestLastSuccessTimestamp,
	)
}

// Handler returns the Prometheus metrics HTTP handler.
func Handler() http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

// PrivateHandler exposes metrics only to local/private callers and local
// scrape hostnames. This prevents accidentally serving /metrics through a
// public Cloudflare/VK-facing hostname while still allowing Docker/SSH-local
// Prometheus scrapes.
func PrivateHandler() http.Handler {
	next := Handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if metricsRequestAllowed(r) {
			next.ServeHTTP(w, r)
			return
		}
		SuspiciousEvents.WithLabelValues("metrics", "public_probe").Inc()
		http.NotFound(w, r)
	})
}

func metricsRequestAllowed(r *http.Request) bool {
	if r == nil {
		return false
	}
	if publicProxyHeadersPresent(r) {
		return false
	}
	return privateRemoteAddr(r.RemoteAddr) && privateHost(r.Host)
}

func publicProxyHeadersPresent(r *http.Request) bool {
	for _, header := range []string{"X-Forwarded-Host", "X-Original-Host"} {
		for _, host := range splitHeaderValues(r.Header.Values(header)) {
			if host != "" && !privateHost(host) {
				return true
			}
		}
	}
	for _, header := range []string{"X-Forwarded-For", "X-Real-IP", "CF-Connecting-IP", "True-Client-IP"} {
		for _, remote := range splitHeaderValues(r.Header.Values(header)) {
			if remote != "" && !privateRemoteAddr(remote) {
				return true
			}
		}
	}
	for _, forwarded := range splitHeaderValues(r.Header.Values("Forwarded")) {
		if forwardedPublic(forwarded) {
			return true
		}
	}
	return false
}

func forwardedPublic(value string) bool {
	for _, part := range strings.Split(value, ";") {
		key, raw, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		raw = strings.Trim(strings.TrimSpace(raw), `"`)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "for":
			if raw != "" && !privateRemoteAddr(raw) {
				return true
			}
		case "host":
			if raw != "" && !privateHost(raw) {
				return true
			}
		}
	}
	return false
}

func splitHeaderValues(values []string) []string {
	var out []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			out = append(out, strings.TrimSpace(part))
		}
	}
	return out
}

func privateRemoteAddr(remote string) bool {
	host := strings.TrimSpace(remote)
	if h, _, err := net.SplitHostPort(remote); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

func privateHost(hostport string) bool {
	host := strings.ToLower(strings.TrimSpace(hostport))
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	switch host {
	case "", "localhost", "host.docker.internal":
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
	}
	// Docker Compose service names are single-label internal hostnames. Public
	// domains such as app.neiirohub.ru are deliberately not accepted here.
	return !strings.Contains(host, ".")
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
