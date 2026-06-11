// Command api runs the HTTP server: the VK callback webhook, the read-only
// admin API and a health endpoint. It performs intake only and never calls AI
// providers (that happens in the worker).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	adminapi "vk-ai-aggregator/internal/adapter/inbound/admin"
	billingapi "vk-ai-aggregator/internal/adapter/inbound/billing"
	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	apiapp "vk-ai-aggregator/internal/app/api"
	miniappapp "vk-ai-aggregator/internal/app/miniapp"
	"vk-ai-aggregator/internal/app/vkbot"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/ratelimit"
	"vk-ai-aggregator/internal/platform/tracing"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	// Fail closed: refuse to start in production without the secrets that
	// protect the webhook intake and admin API (audit S1).
	if err := cfg.Validate(); err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	shutdownTracing, err := tracing.Init(ctx, tracing.Config{
		ServiceName:         cfg.TracingServiceName + "-api",
		Exporter:            cfg.TracingExporter,
		OTLPEndpoint:        cfg.TracingOTLPEndpoint,
		SampleRatio:         cfg.TracingSampleRatio,
		CriticalSampleRatio: cfg.TracingCriticalSampleRatio,
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

	rdb := redisqueue.NewClientWithPool(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, cfg.RedisPoolSize)
	defer rdb.Close()

	core, err := apiapp.NewSharedCore(pool, cfg)
	if err != nil {
		logger.Error("api core wiring failed", "error", err)
		os.Exit(1)
	}
	vkHandler := vkbot.NewHandler(cfg, vkbot.Deps{
		Redis:        rdb,
		Idempotency:  core.Idempotency,
		Inbound:      core.Inbound,
		Users:        core.Users,
		Jobs:         core.Jobs,
		Commands:     core.Commands,
		Billing:      core.Billing,
		Payment:      core.Payment,
		Referrals:    core.Referrals,
		Orchestrator: core.Orchestrator,
		Router:       core.Router,
		Logger:       logger,
	})

	admin := adminapi.NewHandler(adminapi.Config{Token: cfg.AdminToken}, adminapi.Deps{
		Jobs:       core.Jobs,
		Users:      core.Users,
		Deliveries: core.Deliveries,
		Referrals:  core.Referrals,
		Billing:    core.BillingRepo,
	})
	billing := billingapi.NewHandler(billingapi.Config{Token: cfg.AdminToken}, billingapi.Deps{
		Users:      core.Users,
		Payment:    core.Payment,
		PaymentOps: core.PaymentOps,
	})

	miniapp := miniappapp.NewHandler(ctx, cfg, miniappapp.Deps{
		Users:         core.Users,
		Jobs:          core.Jobs,
		Conversations: core.Conversations,
		Artifacts:     core.Artifacts,
		Moderation:    core.Moderation,
		Billing:       core.Billing,
		BillingRepo:   core.BillingRepo,
		Payment:       core.Payment,
		Referrals:     core.Referrals,
		Orchestrator:  core.Orchestrator,
		Logger:        logger,
	})

	// Per-IP rate limiting protects the webhook intake from flooding/abuse
	// (audit S3).
	webhookLimiter := ratelimit.New(cfg.WebhookRateLimitRPS, cfg.WebhookRateLimitBurst)

	mux := http.NewServeMux()
	mux.Handle("/webhooks/vk", webhookLimiter.Middleware(metrics.Middleware("webhook", vkHandler)))
	mux.Handle("/admin/", metrics.Middleware("admin", admin.Routes()))
	mux.Handle("/billing/", metrics.Middleware("billing", billing.Routes()))
	mux.Handle("/miniapp/", metrics.Middleware("miniapp", miniapp.Routes()))
	mux.Handle("GET /metrics", metrics.PrivateHandler())
	mux.HandleFunc("GET /health", healthHandler(pool, rdb))
	mux.HandleFunc("GET /healthz", healthHandler(pool, rdb))

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("api listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	logger.Info("api stopped")
}

// healthHandler reports 200 only when PostgreSQL and Redis are both reachable.
func healthHandler(pool interface {
	Ping(context.Context) error
}, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		checks := map[string]string{"postgres": "ok", "redis": "ok"}
		status := http.StatusOK
		if err := pool.Ping(ctx); err != nil {
			checks["postgres"] = "down"
			status = http.StatusServiceUnavailable
		}
		if err := rdb.Ping(ctx).Err(); err != nil {
			checks["redis"] = "down"
			status = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": map[int]string{http.StatusOK: "ok", http.StatusServiceUnavailable: "degraded"}[status],
			"checks": checks,
		})
	}
}
