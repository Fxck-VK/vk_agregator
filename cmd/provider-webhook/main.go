// Command provider-webhook runs the payment provider webhook intake and async
// inbox processor. It does not mount VK or Mini App auth.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"

	paymentadapter "vk-ai-aggregator/internal/adapter/payment"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/ratelimit"
	"vk-ai-aggregator/internal/platform/tracing"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/paymentservice"
)

const maxWebhookBodyBytes = 1 << 20

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := tracing.Init(ctx, tracing.Config{
		ServiceName: cfg.TracingServiceName + "-provider-webhook",
		Exporter:    cfg.TracingExporter,
	}, logger)
	if err != nil {
		logger.Error("tracing init failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracing(shutdownCtx)
	}()

	pool, err := postgres.NewPoolConfigured(ctx, cfg.DatabaseURL, cfg.DBMaxConns, cfg.DBMinConns)
	if err != nil {
		logger.Error("postgres connect failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	provider, err := paymentadapter.NewProvider(cfg)
	if err != nil {
		logger.Error("payment provider wiring failed", "error", err)
		os.Exit(1)
	}
	if provider.Code() != domain.PaymentProviderYooKassa {
		logger.Warn("provider-webhook started with non-yookassa payment provider", "provider", provider.Code())
	}

	payments := postgres.NewPaymentRepository(pool)
	billingRepo := postgres.NewBillingRepository(pool)
	billing := billingservice.New(billingRepo)
	txRunner := paymentservice.TxRunnerFunc(func(ctx context.Context, fn func(context.Context, domain.PaymentRepository, domain.BillingRepository) error) error {
		return postgres.RunInTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
			return fn(ctx, postgres.NewPaymentRepository(tx), postgres.NewBillingRepositoryTx(tx))
		})
	})
	processor := paymentservice.NewWebhookProcessor(payments, provider, billing, txRunner)

	mux := http.NewServeMux()
	limiter := ratelimit.New(cfg.WebhookRateLimitRPS, cfg.WebhookRateLimitBurst)
	httpsRequired := cfg.PaymentWebhookHTTPSRequired()
	mux.Handle("POST /billing/webhooks/yookassa", limiter.Middleware(metrics.Middleware("billing_webhook", webhookHandler(processor, logger, provider.Code(), httpsRequired))))
	mux.Handle("GET /metrics", metrics.Handler())
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	readyHandler := readinessHandler(pool, processor, provider.Code(), logger)
	mux.Handle("GET /readyz", metrics.Middleware("payment_readyz", readyHandler))
	mux.Handle("GET /healthz", readyHandler)

	srv := &http.Server{
		Addr:              cfg.PaymentWebhookAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go runProcessorLoop(ctx, processor, cfg, logger)

	go func() {
		logger.Info("provider webhook server listening", "addr", cfg.PaymentWebhookAddr, "provider", provider.Code(), "https_required", httpsRequired)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("provider webhook server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	logger.Info("provider webhook server stopped")
}

func webhookHandler(processor *paymentservice.WebhookProcessor, logger *slog.Logger, provider domain.PaymentProviderCode, requireHTTPS bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requireHTTPS && !isSecureWebhookRequest(r) {
			metrics.PaymentWebhookSecurityDenials.WithLabelValues(string(provider), "insecure_transport").Inc()
			http.Error(w, "https required", http.StatusUpgradeRequired)
			return
		}
		defer r.Body.Close()
		raw, err := readWebhookBody(w, r)
		if err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		event, created, err := processor.IngestWebhook(r.Context(), raw, r.Header)
		if err != nil {
			if errors.Is(err, paymentservice.ErrWebhookInvalid) {
				http.Error(w, "invalid webhook", http.StatusBadRequest)
				return
			}
			logger.Error("payment webhook ingest failed", "error", err)
			http.Error(w, "webhook ingest failed", http.StatusInternalServerError)
			return
		}
		logger.Info(
			"payment webhook ingested",
			"provider", event.Provider,
			"event_type", event.EventType,
			"created", created,
			"provider_payment_id", event.ProviderPaymentID,
			"provider_refund_id", event.ProviderRefundID,
		)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func readWebhookBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBodyBytes)
	return io.ReadAll(r.Body)
}

func isSecureWebhookRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	if firstHeaderValue(r.Header.Get("X-Forwarded-Proto")) == "https" {
		return true
	}
	for _, value := range r.Header.Values("Forwarded") {
		if forwardedProto(value) == "https" {
			return true
		}
	}
	return strings.Contains(strings.ToLower(r.Header.Get("CF-Visitor")), `"scheme":"https"`)
}

func firstHeaderValue(value string) string {
	if i := strings.IndexByte(value, ','); i >= 0 {
		value = value[:i]
	}
	return strings.ToLower(strings.TrimSpace(value))
}

func forwardedProto(value string) string {
	for _, part := range strings.Split(value, ";") {
		key, raw, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "proto") {
			continue
		}
		return strings.ToLower(strings.Trim(strings.TrimSpace(raw), `"`))
	}
	return ""
}

func runProcessorLoop(ctx context.Context, processor *paymentservice.WebhookProcessor, cfg config.Config, logger *slog.Logger) {
	interval := cfg.PaymentWebhookPollInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	batchLimit := cfg.PaymentWebhookBatchLimit
	if batchLimit <= 0 {
		batchLimit = 20
	}
	reconcileInterval := cfg.PaymentReconciliationInterval
	if reconcileInterval <= 0 {
		reconcileInterval = time.Minute
	}
	reconcileLimit := cfg.PaymentReconciliationLimit
	if reconcileLimit <= 0 {
		reconcileLimit = 100
	}
	reconcileStaleAfter := cfg.PaymentReconciliationStaleAfter
	if reconcileStaleAfter <= 0 {
		reconcileStaleAfter = 30 * time.Second
	}
	process := func() {
		processed, err := processor.ProcessBatch(ctx, batchLimit)
		if _, statsErr := processor.InboxStats(ctx); statsErr != nil {
			logger.Error("payment webhook inbox stats failed", "error", statsErr)
		}
		if err != nil {
			logger.Error("payment webhook batch processing failed", "error", err, "processed", processed)
			return
		}
		if processed > 0 {
			logger.Info("payment webhook batch processed", "processed", processed)
		}
	}
	reconcile := func() {
		result, err := processor.ReconcilePendingOlderThan(ctx, reconcileLimit, reconcileStaleAfter)
		if err != nil {
			logger.Error("payment reconciliation failed", "error", err, "checked", result.Checked, "processed", result.Processed, "mismatches", result.Mismatches)
			return
		}
		if result.Checked > 0 || result.Mismatches > 0 {
			logger.Info("payment reconciliation completed", "checked", result.Checked, "processed", result.Processed, "mismatches", result.Mismatches)
		}
	}
	process()
	reconcile()
	ticker := time.NewTicker(interval)
	reconcileTicker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()
	defer reconcileTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			process()
		case <-reconcileTicker.C:
			reconcile()
		}
	}
}

type postgresPinger interface {
	Ping(context.Context) error
}

type readinessResponse struct {
	Status         string                  `json:"status"`
	Checks         map[string]string       `json:"checks"`
	PaymentWebhook paymentWebhookReadiness `json:"payment_webhook"`
}

type paymentWebhookReadiness struct {
	Provider                    string  `json:"provider"`
	UnprocessedEvents           int64   `json:"unprocessed_events"`
	OldestUnprocessedAgeSeconds float64 `json:"oldest_unprocessed_age_seconds"`
	OldestUnprocessedReceivedAt string  `json:"oldest_unprocessed_received_at,omitempty"`
}

func readinessHandler(db postgresPinger, processor *paymentservice.WebhookProcessor, provider domain.PaymentProviderCode, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := readinessResponse{
			Status: "ok",
			Checks: map[string]string{
				"postgres":      "ok",
				"webhook_inbox": "ok",
			},
			PaymentWebhook: paymentWebhookReadiness{
				Provider: string(provider),
			},
		}
		status := http.StatusOK
		if err := db.Ping(r.Context()); err != nil {
			resp.Status = "degraded"
			resp.Checks["postgres"] = "unavailable"
			status = http.StatusServiceUnavailable
			logger.Error("provider webhook readiness postgres failed", "error", err)
		}
		if processor == nil {
			resp.Status = "degraded"
			resp.Checks["webhook_inbox"] = "unavailable"
			status = http.StatusServiceUnavailable
		} else {
			stats, err := processor.InboxStats(r.Context())
			if err != nil {
				resp.Status = "degraded"
				resp.Checks["webhook_inbox"] = "unavailable"
				status = http.StatusServiceUnavailable
				logger.Error("provider webhook readiness inbox stats failed", "error", err)
			} else {
				resp.PaymentWebhook.Provider = string(stats.Provider)
				resp.PaymentWebhook.UnprocessedEvents = stats.UnprocessedEvents
				if stats.OldestUnprocessedReceivedAt != nil {
					resp.PaymentWebhook.OldestUnprocessedReceivedAt = stats.OldestUnprocessedReceivedAt.UTC().Format(time.RFC3339)
					age := time.Since(*stats.OldestUnprocessedReceivedAt)
					if age > 0 {
						resp.PaymentWebhook.OldestUnprocessedAgeSeconds = age.Seconds()
					}
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(resp)
	})
}
