// Command api runs the HTTP server: the VK callback webhook, the read-only
// admin API and a health endpoint. It performs intake only and never calls AI
// providers (that happens in the worker).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/platform/logging"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/ratelimit"
	"vk-ai-aggregator/internal/platform/readiness"
	"vk-ai-aggregator/internal/platform/tracing"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/videorouter"
)

type postgresReadyPool interface {
	Ping(context.Context) error
	readiness.SchemaQuerier
}

func main() {
	logger := slog.New(logging.NewJSONHandler(os.Stdout, nil))
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
	defer func() {
		_ = rdb.Close()
	}()

	mediaQueueGuard := redisqueue.NewBackpressureGuard(rdb, cfg.WorkerGroup, cfg.MediaQueueDegradeThreshold)
	videoRouteResolver, err := videoRouteResolverFromConfig(cfg)
	if err != nil {
		logger.Error("video route catalog wiring failed", "error", err)
		os.Exit(1)
	}
	core, err := apiapp.NewSharedCore(pool, cfg, apiapp.WithOrchestratorOptions(
		joborchestrator.WithMaxActiveVideoJobsPerUser(cfg.MediaMaxActiveVideoJobsPerUser),
		joborchestrator.WithCapacityGuard(mediaCapacityGuard(mediaQueueGuard)),
		joborchestrator.WithVideoRouteResolver(videoRouteResolver),
	))
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

	admin := adminapi.NewHandler(adminapi.Config{
		Token:   cfg.AdminToken,
		Runtime: adminapi.NewRuntimeSnapshot(cfg),
	}, adminapi.Deps{
		Jobs:        core.Jobs,
		Users:       core.Users,
		Deliveries:  core.Deliveries,
		Audits:      core.Audits,
		Referrals:   core.Referrals,
		Billing:     core.BillingRepo,
		Payment:     core.Payment,
		Maintenance: core.Maintenance,
	})
	billing := billingapi.NewHandler(billingapi.Config{
		Token:                     cfg.AdminToken,
		AllowLoadTestMockPayments: cfg.IsLoadTest() && strings.EqualFold(strings.TrimSpace(cfg.PaymentProvider), string(domain.PaymentProviderMock)),
	}, billingapi.Deps{
		Users:      core.Users,
		Billing:    core.BillingRepo,
		Payment:    core.Payment,
		PaymentOps: core.PaymentOps,
		Audits:     core.Audits,
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
	apiReadyHandler := readinessHandler(pool, rdb, cfg.MigrationsDir)
	mux.HandleFunc("GET /readyz", apiReadyHandler)
	mux.HandleFunc("GET /healthz", apiReadyHandler)

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

func videoRouteResolverFromConfig(cfg config.Config) (joborchestrator.VideoRouteResolver, error) {
	catalog, err := videorouter.NewCatalog(videorouter.Config{
		RouterEnabled: cfg.FeatureVideoRouterEnabled,
		Providers: map[domain.ProviderName]videorouter.ProviderConfig{
			domain.ProviderAPIMart: {
				Enabled:           cfg.APIMartProviderEnabled,
				RequireAPIKey:     true,
				APIKeyConfigured:  strings.TrimSpace(cfg.APIMartAPIKey) != "",
				RequireBaseURL:    true,
				BaseURLConfigured: strings.TrimSpace(cfg.APIMartBaseURL) != "",
			},
			domain.ProviderPoYo: {
				Enabled:           cfg.PoYoProviderEnabled,
				RequireAPIKey:     true,
				APIKeyConfigured:  strings.TrimSpace(cfg.PoYoAPIKey) != "",
				RequireBaseURL:    true,
				BaseURLConfigured: strings.TrimSpace(cfg.PoYoBaseURL) != "",
			},
			domain.ProviderRunway: {
				Enabled:           cfg.RunwayProviderEnabled,
				RequireAPIKey:     true,
				APIKeyConfigured:  strings.TrimSpace(cfg.RunwayMLAPISecret) != "",
				RequireBaseURL:    true,
				BaseURLConfigured: strings.TrimSpace(cfg.RunwayMLBaseURL) != "",
			},
		},
		EnabledRoutes: map[domain.VideoRouteAlias]bool{
			domain.VideoRouteHailuo23Fast:     cfg.FeatureVideoRouteHailuo23FastEnabled,
			domain.VideoRouteHailuo23Standard: cfg.FeatureVideoRouteHailuo23StandardEnabled,
			domain.VideoRouteKlingO3Standard:  cfg.FeatureVideoRouteKlingO3StandardEnabled,
			domain.VideoRouteRunwayGen4Turbo:  cfg.FeatureVideoRouteRunwayGen4TurboEnabled,
			domain.VideoRouteSeedance20Fast:   cfg.FeatureVideoRouteSeedance20FastEnabled,
			domain.VideoRouteRunwayGen45:      cfg.FeatureVideoRouteRunwayGen45Enabled,
		},
	})
	if err != nil {
		return nil, err
	}
	return joborchestrator.VideoRouteResolverFunc(func(ctx context.Context, in joborchestrator.VideoRouteCheckInput) (joborchestrator.VideoRouteResolution, error) {
		resolution, err := catalog.Resolve(ctx, videorouter.Request{
			Source:           in.Source,
			Operation:        in.Operation,
			Modality:         in.Modality,
			Params:           in.Params,
			InputArtifactIDs: in.InputArtifactIDs,
		})
		if err != nil {
			return joborchestrator.VideoRouteResolution{}, err
		}
		return joborchestrator.VideoRouteResolution{
			Resolved:            resolution.Resolved,
			Params:              resolution.Params,
			Snapshot:            resolution.Snapshot,
			InternalCostCredits: resolution.InternalCostCredits,
		}, nil
	}), nil
}

func mediaCapacityGuard(queueGuard *redisqueue.BackpressureGuard) joborchestrator.CapacityGuard {
	return joborchestrator.CapacityGuardFunc(func(ctx context.Context, in joborchestrator.CapacityCheckInput) error {
		if !isMediaCapacityOperation(in.Operation, in.Modality) {
			return nil
		}
		if err := queueGuard.Check(ctx, mediaBackpressureStreams(in.Operation)...); err != nil {
			var bp redisqueue.BackpressureError
			if errors.As(err, &bp) {
				return fmt.Errorf("%w: queue %s degraded", domain.ErrCapacityDegraded, bp.Pressure.Stream)
			}
			return fmt.Errorf("%w: queue pressure unavailable", domain.ErrCapacityDegraded)
		}
		return nil
	})
}

func isMediaCapacityOperation(op domain.OperationType, modality domain.Modality) bool {
	switch modality {
	case domain.ModalityImage, domain.ModalityVideo:
		return true
	}
	switch op {
	case domain.OperationImageGenerate, domain.OperationImageEdit, domain.OperationImageUpscale,
		domain.OperationVideoGenerate, domain.OperationVideoImageToVideo, domain.OperationVideoExtend:
		return true
	default:
		return false
	}
}

func mediaBackpressureStreams(op domain.OperationType) []string {
	streams := []string{redisqueue.StreamForOperation(op), redisqueue.StreamProviderPoll, redisqueue.StreamDelivery}
	seen := make(map[string]struct{}, len(streams))
	out := make([]string, 0, len(streams))
	for _, stream := range streams {
		if _, ok := seen[stream]; ok {
			continue
		}
		seen[stream] = struct{}{}
		out = append(out, stream)
	}
	return out
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

// readinessHandler fails closed until API dependencies and the latest schema
// migration are available.
func readinessHandler(pool postgresReadyPool, rdb *redis.Client, migrationsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		checks := map[string]string{
			"postgres":   "ok",
			"redis":      "ok",
			"migrations": "ok",
		}
		status := http.StatusOK
		latestMigration := ""
		if err := pool.Ping(ctx); err != nil {
			checks["postgres"] = "down"
			status = http.StatusServiceUnavailable
		}
		if err := rdb.Ping(ctx).Err(); err != nil {
			checks["redis"] = "down"
			status = http.StatusServiceUnavailable
		}
		if version, err := readiness.CheckLatestMigrationApplied(ctx, pool, migrationsDir); err != nil {
			latestMigration = version
			checks["migrations"] = "pending"
			status = http.StatusServiceUnavailable
		} else {
			latestMigration = version
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":           map[int]string{http.StatusOK: "ok", http.StatusServiceUnavailable: "degraded"}[status],
			"checks":           checks,
			"latest_migration": latestMigration,
		})
	}
}
