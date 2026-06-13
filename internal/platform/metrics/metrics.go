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
	"unicode/utf8"

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

	// MediaQueueBacklog tracks media-relevant queue pressure by curated queue
	// class. It must never include raw stream names derived from user input.
	MediaQueueBacklog = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vkagg_media_queue_backlog",
		Help: "Current media-relevant queue backlog by bounded queue class.",
	}, []string{"queue_class"})

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

	// MediaProbeResults counts worker-owned media probe outcomes. Labels are
	// bounded; never add ids, raw errors, paths, URLs or prompts here.
	MediaProbeResults = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_media_probe_results_total",
		Help: "Media probe outcomes by operation, modality, result and coarse error class.",
	}, []string{"result", "operation", "modality", "error_class"})

	// MediaProbeByProvider counts media probe outcomes by provider/model class
	// without exposing raw model ids, job ids, artifact ids, URLs or prompts.
	MediaProbeByProvider = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_media_probe_total",
		Help: "Media probe outcomes by bounded provider class, curated model class, result and coarse error class.",
	}, []string{"result", "error_class", "provider_class", "model_class"})

	// MediaTranscodeResults counts worker-owned media transcode outcomes.
	MediaTranscodeResults = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_media_transcode_results_total",
		Help: "Media transcode outcomes by operation, modality, variant type, result and coarse error class.",
	}, []string{"result", "operation", "modality", "variant_type", "error_class"})

	// MediaTranscodeByPolicy counts transcode outcomes by policy. This is the
	// production-scale view for detecting unexpected ffmpeg usage.
	MediaTranscodeByPolicy = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_media_transcode_total",
		Help: "Media transcode outcomes by video transcode policy, result and coarse error class.",
	}, []string{"policy", "result", "error_class"})

	// MediaTranscodeDuration tracks media transcode duration.
	MediaTranscodeDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vkagg_media_transcode_duration_seconds",
		Help:    "Media transcode duration by operation, modality, variant type and result.",
		Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60, 120, 300},
	}, []string{"result", "operation", "modality", "variant_type"})

	// MediaTranscodeCPUSeconds is a wall-duration proxy for ffmpeg CPU pressure.
	// Exact CPU accounting is platform-specific, so this keeps labels bounded
	// and operationally useful without parsing raw ffmpeg output.
	MediaTranscodeCPUSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vkagg_media_transcode_cpu_seconds",
		Help:    "Media transcode CPU pressure proxy by policy, result and coarse error class.",
		Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60, 120, 300},
	}, []string{"policy", "result", "error_class"})

	// MediaBytes tracks bounded media byte distributions without exposing
	// object keys or URLs.
	MediaBytes = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vkagg_media_bytes",
		Help:    "Media object sizes by operation, modality and variant type.",
		Buckets: []float64{1024, 16 * 1024, 64 * 1024, 256 * 1024, 1 << 20, 5 << 20, 20 << 20, 100 << 20, 256 << 20},
	}, []string{"operation", "modality", "variant_type"})

	// UploadValidation counts API-side upload validation decisions with
	// bounded classes only. Do not add filenames, users, object keys or raw
	// validation errors as labels.
	UploadValidation = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_upload_validation_total",
		Help: "Upload validation decisions by result, reason, surface and bounded MIME class.",
	}, []string{"result", "reason", "surface", "mime_class"})

	// UploadBytes tracks uploaded byte pressure by surface and MIME class.
	UploadBytes = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vkagg_upload_bytes",
		Help:    "Upload byte size distribution by surface and bounded MIME class.",
		Buckets: []float64{1024, 16 * 1024, 64 * 1024, 256 * 1024, 1 << 20, 5 << 20, 20 << 20, 50 << 20},
	}, []string{"surface", "mime_class"})

	// UploadPixels tracks decoded image pixel pressure when cheap metadata is
	// available. It intentionally omits filenames and object identities.
	UploadPixels = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vkagg_upload_pixels",
		Help:    "Upload decoded image pixels by surface and bounded MIME class.",
		Buckets: []float64{1e4, 1e5, 5e5, 1e6, 2e6, 5e6, 1e7, 2e7, 5e7},
	}, []string{"surface", "mime_class"})

	// MediaVariantBacklog tracks in-process variant work without job/artifact ids.
	MediaVariantBacklog = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vkagg_media_variant_backlog",
		Help: "Current in-process media variant backlog by operation, modality and variant type.",
	}, []string{"operation", "modality", "variant_type"})

	// MediaPolicyDecisions counts media policy decisions, including accepted,
	// rejected, degraded, skipped, fallback and kill-switch outcomes.
	MediaPolicyDecisions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_media_policy_decisions_total",
		Help: "Media policy decisions by bounded surface, operation, modality, decision and reason.",
	}, []string{"surface", "operation", "modality", "decision", "reason"})

	// MediaFastPath counts simplified fast-path decisions for scale alerts.
	MediaFastPath = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_media_fast_path_total",
		Help: "Simplified media fast-path decisions by bounded result.",
	}, []string{"result"})

	// MediaVideoFastPath counts worker video postprocessing decisions. Labels are
	// bounded; model_class must come from product policy, not raw model ids.
	MediaVideoFastPath = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_media_video_fast_path_total",
		Help: "Video fast-path decisions by result, operation, modality, provider and curated model class.",
	}, []string{"result", "operation", "modality", "provider", "model_class"})

	// ProviderQualityState exposes the current bounded quality state for a
	// provider/model_class/modality tuple. model_class must be curated product
	// policy, not raw provider-native model ids.
	ProviderQualityState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vkagg_provider_quality_state",
		Help: "Provider quality state by provider, curated model class, modality and state. Exactly one state should be 1 per observed tuple.",
	}, []string{"provider", "model_class", "modality", "state"})

	// ProviderQualitySamples counts quality samples used by Prometheus recording
	// rules. Labels are bounded and intentionally exclude job/user/artifact ids.
	ProviderQualitySamples = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_provider_quality_samples_total",
		Help: "Provider quality samples by provider, curated model class, modality and result.",
	}, []string{"provider", "model_class", "modality", "result"})

	// ProviderOutputInvalid counts provider successes that later produced
	// invalid or unusable output in product-owned media processing.
	ProviderOutputInvalid = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_provider_output_invalid_total",
		Help: "Provider output invalid events by provider, curated model class, modality and bounded reason.",
	}, []string{"provider", "model_class", "modality", "reason"})

	// ProductMediaWaste counts internal credit units at risk after provider
	// success but before product delivery/capture. It is not money and must not
	// include high-cardinality labels.
	ProductMediaWaste = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_product_media_waste_total",
		Help: "Estimated internal credits wasted or at risk by provider, curated model class, modality and bounded reason.",
	}, []string{"provider", "model_class", "modality", "reason"})

	// MediaProviderWaste is the media-specific provider waste view requested by
	// production readiness alerts. provider_class/model_class are curated.
	MediaProviderWaste = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_media_provider_waste_total",
		Help: "Media provider waste or money-at-risk units by bounded provider class, curated model class and reason.",
	}, []string{"provider_class", "model_class", "reason"})

	// MediaDeliveryCaptureGap counts cases where media reached delivery/capture
	// boundary but could not safely complete the product flow.
	MediaDeliveryCaptureGap = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_media_delivery_capture_gap_total",
		Help: "Media delivery/capture gap events by operation, modality and bounded reason.",
	}, []string{"operation", "modality", "reason"})

	// MediaCleanupDeleted counts media cleanup deletion outcomes for inactive
	// artifacts and variants only.
	MediaCleanupDeleted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_media_cleanup_deleted_total",
		Help: "Media cleanup deletion outcomes by variant type and coarse error class.",
	}, []string{"result", "variant_type", "error_class"})

	// JobsCreated counts newly-created jobs by source surface and operation.
	JobsCreated = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_jobs_created_total",
		Help: "Jobs created by source surface, operation and modality.",
	}, []string{"surface", "operation", "modality"})

	// ProductEvents counts privacy-safe product funnel events. Labels are
	// intentionally bounded and must never contain user/job/payment ids, raw
	// URLs, prompts, launch params, provider payloads or raw errors.
	ProductEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_product_events_total",
		Help: "Privacy-safe product funnel events by surface, journey, step, operation, modality and result.",
	}, []string{"surface", "journey", "step", "operation", "modality", "result"})

	// ProductActiveUserEvents counts job-creation events that satisfy the MVP
	// active-user definition. It is not a unique-user counter; exact DAU/D1/D7
	// retention belongs in a scheduled aggregate/event warehouse to avoid
	// high-cardinality Prometheus labels.
	ProductActiveUserEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_product_active_user_events_total",
		Help: "Job creation events for the MVP active-user definition, by bounded product dimensions.",
	}, []string{"surface", "operation", "modality", "result"})

	// ProductActiveUsers tracks exact unique users with at least one job inside
	// a coarse window. It is updated by a scheduled aggregate query and never
	// labels by user_id.
	ProductActiveUsers = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vkagg_product_active_users",
		Help: "Unique users with at least one job in the given window, by bounded product dimensions.",
	}, []string{"window", "surface", "operation", "modality"})

	// ProductPromptLength tracks prompt character-count buckets without
	// exporting prompt text.
	ProductPromptLength = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vkagg_product_prompt_length_chars",
		Help:    "Prompt length in characters by surface, operation and modality. Prompt text is never exported.",
		Buckets: []float64{1, 25, 50, 100, 250, 500, 1000, 2000, 4000, 8000, 16000},
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

	// ProductFrontendAPIDuration tracks safe client-observed API latency. Route
	// labels must come from an allowlist/template mapper, not raw browser URLs.
	ProductFrontendAPIDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vkagg_product_frontend_api_duration_seconds",
		Help:    "Client-observed API latency by safe route template and status.",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30},
	}, []string{"surface", "route", "status"})

	// ProductFrontendUIDuration tracks coarse UI milestones such as first
	// render. It must not include screen content or user identifiers.
	ProductFrontendUIDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vkagg_product_frontend_ui_duration_seconds",
		Help:    "Client-observed UI milestone duration by safe step and result.",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30},
	}, []string{"surface", "step", "result"})

	// ProductCreditsFlow counts aggregate credit units through ledger-backed
	// product flows. Amounts are credits, not money, and labels are bounded.
	ProductCreditsFlow = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vkagg_product_credits_flow_total",
		Help: "Aggregate credit units flowing through ledger-backed product paths.",
	}, []string{"source", "flow", "result"})

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
		QueueDepth, QueueOldestAgeSeconds, QueueConsumerLag, MediaQueueBacklog,
		StuckJobs, WorkerTaskDuration, WorkerRetries, MediaProbeResults,
		MediaProbeByProvider, MediaTranscodeResults, MediaTranscodeByPolicy,
		MediaTranscodeDuration, MediaTranscodeCPUSeconds, MediaBytes,
		UploadValidation, UploadBytes, UploadPixels, MediaVariantBacklog,
		MediaPolicyDecisions, MediaFastPath, MediaVideoFastPath, ProviderQualityState,
		ProviderQualitySamples, ProviderOutputInvalid, ProductMediaWaste,
		MediaProviderWaste, MediaDeliveryCaptureGap, MediaCleanupDeleted, JobsCreated, JobDuration,
		ProductEvents, ProductActiveUserEvents, ProductActiveUsers, ProductPromptLength,
		JobStatusCurrent, JobRejected, ProviderRequests, ProviderRequestDuration,
		ProviderErrors, ProviderRateLimits, ProviderFallback, ProviderCircuitState,
		ProviderTokens, ProviderImages, ProviderVideos, ProviderEstimatedCost,
		BillingReservations, BillingCaptures, BillingReleases, LedgerEntries,
		PaymentToLedgerDuration, ReferralRewards, FrontendEvents, FrontendJSErrors,
		FrontendAPIFailures, FrontendLaunchFailures, FrontendPaymentFlowErrors,
		ProductFrontendAPIDuration, ProductFrontendUIDuration, ProductCreditsFlow,
		VKDeliveryAttempts, VKDeliveryDuration, VKUploadFailures,
		VKMenuControlErrors, AuthFailures, SignatureFailures, AdminActions,
		SuspiciousEvents, ConfigValidationFailures, BackupLastSuccessTimestamp,
		BackupDuration, BackupSizeBytes, BackupFailures,
		RestoreTestLastSuccessTimestamp,
	)
}

// InitPaymentProviderMetrics pre-creates bounded payment metric series for a
// configured provider so monitoring can distinguish "zero errors" from a
// missing or unscheduled scrape.
func InitPaymentProviderMetrics(provider string) {
	provider = ProductLabel(provider, "unknown")
	PaymentWebhookUnprocessedEvents.WithLabelValues(provider).Set(0)
	PaymentWebhookOldestUnprocessedAgeSeconds.WithLabelValues(provider).Set(0)

	for _, stage := range []string{"processing", "invalid", "unsupported", "provider_unverified", "provider_mismatch"} {
		PaymentWebhookProcessingErrors.WithLabelValues(provider, stage).Add(0)
	}
	for _, operation := range []string{"get_payment", "cancel_payment", "create_refund"} {
		for _, class := range []string{"provider_error", "timeout", "canceled", "not_found", "provider_mismatch"} {
			PaymentProviderErrors.WithLabelValues(provider, operation, class).Add(0)
		}
	}
	for _, result := range []string{"provider_error", "provider_mismatch", "rollback_failed", "rollback_succeeded"} {
		PaymentRefunds.WithLabelValues(provider, result).Add(0)
	}
}

// ProductLabel sanitizes bounded product-observability labels. It is for
// trusted product dimensions only; never pass prompt text, full URLs, raw
// errors, ids, launch params or provider/payment payloads.
func ProductLabel(value string, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		if fallback == "" {
			return "unknown"
		}
		return fallback
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-' || r == '.' || r == ':' || r == '/':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
		if b.Len() >= 96 {
			break
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		if fallback == "" {
			return "unknown"
		}
		return fallback
	}
	return out
}

// ObserveProductEvent records one bounded product funnel event.
func ObserveProductEvent(surface, journey, step, operation, modality, result string) {
	ProductEvents.WithLabelValues(
		ProductLabel(surface, "unknown"),
		ProductLabel(journey, "unknown"),
		ProductLabel(step, "unknown"),
		ProductLabel(operation, "unknown"),
		ProductLabel(modality, "unknown"),
		ProductLabel(result, "unknown"),
	).Inc()
}

// ObserveProductActiveUserEvent records a job-creation event matching the MVP
// active-user definition. It intentionally does not try to count unique users.
func ObserveProductActiveUserEvent(surface, operation, modality, result string) {
	ProductActiveUserEvents.WithLabelValues(
		ProductLabel(surface, "unknown"),
		ProductLabel(operation, "unknown"),
		ProductLabel(modality, "unknown"),
		ProductLabel(result, "unknown"),
	).Inc()
}

// ObserveProductPromptLength records only the prompt length bucket, never the
// prompt content.
func ObserveProductPromptLength(surface, operation, modality, prompt string) {
	ProductPromptLength.WithLabelValues(
		ProductLabel(surface, "unknown"),
		ProductLabel(operation, "unknown"),
		ProductLabel(modality, "unknown"),
	).Observe(float64(utf8.RuneCountInString(prompt)))
}

// ObserveProductFrontendAPIDuration records safe client-observed API latency.
func ObserveProductFrontendAPIDuration(surface, route, status string, durationMS int64) {
	if durationMS <= 0 {
		return
	}
	const maxClientDurationMS = int64(10 * 60 * 1000)
	if durationMS > maxClientDurationMS {
		durationMS = maxClientDurationMS
	}
	ProductFrontendAPIDuration.WithLabelValues(
		ProductLabel(surface, "unknown"),
		ProductLabel(route, "unknown"),
		ProductLabel(status, "unknown"),
	).Observe(float64(durationMS) / 1000)
}

// ObserveProductFrontendUIDuration records safe client-observed UI milestone
// latency.
func ObserveProductFrontendUIDuration(surface, step, result string, durationMS int64) {
	if durationMS <= 0 {
		return
	}
	const maxClientDurationMS = int64(10 * 60 * 1000)
	if durationMS > maxClientDurationMS {
		durationMS = maxClientDurationMS
	}
	ProductFrontendUIDuration.WithLabelValues(
		ProductLabel(surface, "unknown"),
		ProductLabel(step, "unknown"),
		ProductLabel(result, "unknown"),
	).Observe(float64(durationMS) / 1000)
}

// AddProductCreditsFlow records aggregate credit units for ledger-backed flows.
func AddProductCreditsFlow(source, flow, result string, credits int64) {
	if credits <= 0 {
		return
	}
	ProductCreditsFlow.WithLabelValues(
		ProductLabel(source, "unknown"),
		ProductLabel(flow, "unknown"),
		ProductLabel(result, "unknown"),
	).Add(float64(credits))
}

// SetMediaQueueBacklog sets media-relevant backlog for a bounded queue class.
func SetMediaQueueBacklog(queueClass string, backlog int64) {
	if backlog < 0 {
		backlog = 0
	}
	MediaQueueBacklog.WithLabelValues(
		ProductLabel(queueClass, "unknown"),
	).Set(float64(backlog))
}

// ObserveMediaProbe records one worker-owned media probe outcome.
func ObserveMediaProbe(result, operation, modality, errorClass string) {
	MediaProbeResults.WithLabelValues(
		ProductLabel(result, "unknown"),
		ProductLabel(operation, "unknown"),
		ProductLabel(modality, "unknown"),
		ProductLabel(errorClass, "none"),
	).Inc()
}

// ObserveMediaProbeByProvider records provider/model-class probe outcomes.
func ObserveMediaProbeByProvider(result, errorClass, providerClass, modelClass string) {
	MediaProbeByProvider.WithLabelValues(
		ProductLabel(result, "unknown"),
		ProductLabel(errorClass, "none"),
		ProductLabel(providerClass, "unknown"),
		ProductLabel(modelClass, "unknown"),
	).Inc()
}

// ObserveMediaTranscode records one worker-owned media transcode outcome.
func ObserveMediaTranscode(result, operation, modality, variantType, errorClass string) {
	MediaTranscodeResults.WithLabelValues(
		ProductLabel(result, "unknown"),
		ProductLabel(operation, "unknown"),
		ProductLabel(modality, "unknown"),
		ProductLabel(variantType, "unknown"),
		ProductLabel(errorClass, "none"),
	).Inc()
}

// ObserveMediaTranscodeByPolicy records transcode outcomes by policy.
func ObserveMediaTranscodeByPolicy(policy, result, errorClass string) {
	MediaTranscodeByPolicy.WithLabelValues(
		ProductLabel(policy, "unknown"),
		ProductLabel(result, "unknown"),
		ProductLabel(errorClass, "none"),
	).Inc()
}

// ObserveMediaTranscodeDuration records positive transcode durations only.
func ObserveMediaTranscodeDuration(result, operation, modality, variantType string, duration time.Duration) {
	if duration <= 0 {
		return
	}
	MediaTranscodeDuration.WithLabelValues(
		ProductLabel(result, "unknown"),
		ProductLabel(operation, "unknown"),
		ProductLabel(modality, "unknown"),
		ProductLabel(variantType, "unknown"),
	).Observe(duration.Seconds())
}

// ObserveMediaTranscodeCPUSeconds records a bounded wall-duration proxy for CPU
// pressure from ffmpeg work.
func ObserveMediaTranscodeCPUSeconds(policy, result, errorClass string, duration time.Duration) {
	if duration <= 0 {
		return
	}
	MediaTranscodeCPUSeconds.WithLabelValues(
		ProductLabel(policy, "unknown"),
		ProductLabel(result, "unknown"),
		ProductLabel(errorClass, "none"),
	).Observe(duration.Seconds())
}

// ObserveMediaBytes records positive media object sizes only.
func ObserveMediaBytes(operation, modality, variantType string, sizeBytes int64) {
	if sizeBytes <= 0 {
		return
	}
	MediaBytes.WithLabelValues(
		ProductLabel(operation, "unknown"),
		ProductLabel(modality, "unknown"),
		ProductLabel(variantType, "unknown"),
	).Observe(float64(sizeBytes))
}

// ObserveMediaUploadValidation records a bounded upload validation decision.
func ObserveMediaUploadValidation(surface, result, reason, mimeClass string) {
	UploadValidation.WithLabelValues(
		ProductLabel(result, "unknown"),
		ProductLabel(reason, "unknown"),
		ProductLabel(surface, "unknown"),
		ProductLabel(mimeClass, "unknown"),
	).Inc()
}

// ObserveMediaUploadBytes records positive upload byte sizes.
func ObserveMediaUploadBytes(surface, mimeClass string, sizeBytes int64) {
	if sizeBytes <= 0 {
		return
	}
	UploadBytes.WithLabelValues(
		ProductLabel(surface, "unknown"),
		ProductLabel(mimeClass, "unknown"),
	).Observe(float64(sizeBytes))
}

// ObserveMediaUploadPixels records positive decoded image pixel counts.
func ObserveMediaUploadPixels(surface, mimeClass string, pixels int64) {
	if pixels <= 0 {
		return
	}
	UploadPixels.WithLabelValues(
		ProductLabel(surface, "unknown"),
		ProductLabel(mimeClass, "unknown"),
	).Observe(float64(pixels))
}

// AddMediaVariantBacklog updates the in-process media variant backlog.
func AddMediaVariantBacklog(operation, modality, variantType string, delta float64) {
	if delta == 0 {
		return
	}
	MediaVariantBacklog.WithLabelValues(
		ProductLabel(operation, "unknown"),
		ProductLabel(modality, "unknown"),
		ProductLabel(variantType, "unknown"),
	).Add(delta)
}

// ObserveMediaPolicyDecision records product-level media policy decisions.
func ObserveMediaPolicyDecision(surface, operation, modality, decision, reason string) {
	MediaPolicyDecisions.WithLabelValues(
		ProductLabel(surface, "unknown"),
		ProductLabel(operation, "unknown"),
		ProductLabel(modality, "unknown"),
		ProductLabel(decision, "unknown"),
		ProductLabel(reason, "unknown"),
	).Inc()
}

// ObserveMediaFastPath records simplified fast-path decisions.
func ObserveMediaFastPath(result string) {
	MediaFastPath.WithLabelValues(
		ProductLabel(result, "unknown"),
	).Inc()
}

// ObserveMediaVideoFastPath records one bounded video postprocessing decision.
func ObserveMediaVideoFastPath(result, operation, modality, provider, modelClass string) {
	MediaVideoFastPath.WithLabelValues(
		ProductLabel(result, "unknown"),
		ProductLabel(operation, "unknown"),
		ProductLabel(modality, "unknown"),
		ProductLabel(provider, "unknown"),
		ProductLabel(modelClass, "unknown"),
	).Inc()
}

// ObserveProviderQualityState sets a one-hot quality state gauge. State values
// are bounded to healthy/degraded/disabled.
func ObserveProviderQualityState(provider, modelClass, modality, state string) {
	state = ProductLabel(state, "healthy")
	switch state {
	case "healthy", "degraded", "disabled":
	default:
		state = "healthy"
	}
	provider = ProductLabel(provider, "unknown")
	modelClass = ProductLabel(modelClass, "unknown")
	modality = ProductLabel(modality, "unknown")
	for _, candidate := range []string{"healthy", "degraded", "disabled"} {
		value := 0.0
		if candidate == state {
			value = 1
		}
		ProviderQualityState.WithLabelValues(provider, modelClass, modality, candidate).Set(value)
	}
}

// ObserveProviderQualitySample records a bounded quality sample.
func ObserveProviderQualitySample(provider, modelClass, modality, result string) {
	ProviderQualitySamples.WithLabelValues(
		ProductLabel(provider, "unknown"),
		ProductLabel(modelClass, "unknown"),
		ProductLabel(modality, "unknown"),
		ProductLabel(result, "unknown"),
	).Inc()
}

// ObserveProviderOutputInvalid records unusable provider media output.
func ObserveProviderOutputInvalid(provider, modelClass, modality, reason string) {
	ProviderOutputInvalid.WithLabelValues(
		ProductLabel(provider, "unknown"),
		ProductLabel(modelClass, "unknown"),
		ProductLabel(modality, "unknown"),
		ProductLabel(reason, "unknown"),
	).Inc()
}

// AddProductMediaWaste records bounded internal credit waste/risk units.
func AddProductMediaWaste(provider, modelClass, modality, reason string, credits int64) {
	if credits <= 0 {
		credits = 1
	}
	ProductMediaWaste.WithLabelValues(
		ProductLabel(provider, "unknown"),
		ProductLabel(modelClass, "unknown"),
		ProductLabel(modality, "unknown"),
		ProductLabel(reason, "unknown"),
	).Add(float64(credits))
}

// AddMediaProviderWaste records media-specific provider waste/risk units.
func AddMediaProviderWaste(providerClass, modelClass, reason string, credits int64) {
	if credits <= 0 {
		credits = 1
	}
	MediaProviderWaste.WithLabelValues(
		ProductLabel(providerClass, "unknown"),
		ProductLabel(modelClass, "unknown"),
		ProductLabel(reason, "unknown"),
	).Add(float64(credits))
}

// ObserveMediaDeliveryCaptureGap records delivery/capture boundary failures.
func ObserveMediaDeliveryCaptureGap(operation, modality, reason string) {
	MediaDeliveryCaptureGap.WithLabelValues(
		ProductLabel(operation, "unknown"),
		ProductLabel(modality, "unknown"),
		ProductLabel(reason, "unknown"),
	).Inc()
}

// ObserveMediaCleanupDeleted records one media cleanup outcome.
func ObserveMediaCleanupDeleted(result, variantType, errorClass string) {
	MediaCleanupDeleted.WithLabelValues(
		ProductLabel(result, "unknown"),
		ProductLabel(variantType, "unknown"),
		ProductLabel(errorClass, "none"),
	).Inc()
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
