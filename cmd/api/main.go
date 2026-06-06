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

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	adminapi "vk-ai-aggregator/internal/adapter/inbound/admin"
	miniappapi "vk-ai-aggregator/internal/adapter/inbound/miniapp"
	vkinbound "vk-ai-aggregator/internal/adapter/inbound/vk"
	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	s3store "vk-ai-aggregator/internal/adapter/storage/s3"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/ratelimit"
	"vk-ai-aggregator/internal/platform/tracing"
	"vk-ai-aggregator/internal/service/antispam"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/commandrouter"
	"vk-ai-aggregator/internal/service/dialogstate"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/referralservice"
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
		ServiceName: cfg.TracingServiceName + "-api",
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

	rdb := redisqueue.NewClientWithPool(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, cfg.RedisPoolSize)
	defer rdb.Close()

	// Repositories and services.
	users := postgres.NewUserRepository(pool)
	jobs := postgres.NewJobRepository(pool)
	commands := postgres.NewCommandRepository(pool)
	inbound := postgres.NewInboundEventRepository(pool)
	idem := postgres.NewIdempotencyRepository(pool)
	deliveries := postgres.NewDeliveryRepository(pool)
	billingRepo := postgres.NewBillingRepository(pool)
	referralsRepo := postgres.NewReferralRepository(pool)
	artifacts := postgres.NewArtifactRepository(pool)
	modResults := postgres.NewModerationResultRepository(pool)

	var objectStore miniappapi.ObjectReader
	store, err := s3store.New(ctx, s3store.Config{
		Endpoint:  cfg.S3Endpoint,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
		UseSSL:    cfg.S3UseSSL,
	})
	if err != nil {
		logger.Warn("s3 connect failed; miniapp artifact downloads disabled", "error", err)
	} else {
		objectStore = store
	}

	billing := billingservice.New(billingRepo, billingservice.WithPriceOverrides(cfg.PriceOverrides))
	referrals := referralservice.New(referralsRepo, billing, referralservice.Config{
		CodeLength:                  cfg.ReferralCodeLength,
		ReferrerSignupRewardCredits: cfg.ReferralReferrerSignupRewardCredits,
		ReferredSignupRewardCredits: cfg.ReferralReferredSignupRewardCredits,
	})
	uowMgr := postgres.NewUnitOfWork(pool)
	// The orchestrator records a queued outbox event; the worker's outbox relay
	// publishes it to the queue, so the api process does not enqueue directly
	// (audit A2).
	orch := joborchestrator.New(jobs, uowMgr, billing, cfg.MaxJobCost)
	router := commandrouter.New()
	vkDialogState := dialogstate.New(redisqueue.NewDialogStateStore(rdb), dialogstate.Config{
		TTL: cfg.VKDialogModeTTL,
	})
	vkAntiSpam := antispam.New(redisqueue.NewAntiSpamStore(rdb), jobs, antispam.Config{
		Enabled:             cfg.VKAntiSpamEnabled,
		MessageLimit:        cfg.VKAntiSpamMessageLimit,
		MessageWindow:       cfg.VKAntiSpamMessageWindow,
		GPTLimit:            cfg.VKAntiSpamGPTLimit,
		GPTWindow:           cfg.VKAntiSpamGPTWindow,
		Cooldown:            cfg.VKAntiSpamCooldown,
		ViolationLimit:      cfg.VKAntiSpamViolationLimit,
		ViolationWindow:     cfg.VKAntiSpamViolationWindow,
		BlockDuration:       cfg.VKAntiSpamBlockDuration,
		NewUserAge:          cfg.VKAntiSpamNewUserAge,
		NewUserMessageLimit: cfg.VKAntiSpamNewUserMessageLimit,
		NewUserGPTLimit:     cfg.VKAntiSpamNewUserGPTLimit,
		NewUserGPTWindow:    cfg.VKAntiSpamNewUserGPTWindow,
		ActiveGPTJobLimit:   cfg.VKAntiSpamActiveGPTJobLimit,
	})

	var vkControl vkdelivery.ControlClient
	var vkProfile vkdelivery.UserProfileClient
	if cfg.VKAccessToken != "" {
		vkClient := vkdelivery.NewHTTPClient(vkdelivery.HTTPConfig{
			AccessToken: cfg.VKAccessToken,
			APIVersion:  cfg.VKAPIVersion,
			BaseURL:     cfg.VKAPIBaseURL,
		})
		vkControl = vkClient
		vkProfile = vkClient
		logger.Info("using real vk control delivery client")
	} else {
		logger.Warn("vk control responses disabled because VK_ACCESS_TOKEN is empty")
	}

	vkHandler := vkinbound.NewHandler(vkinbound.Config{
		ConfirmationToken:                   cfg.VKConfirmationToken,
		Secret:                              cfg.VKSecret,
		WelcomeAttachment:                   cfg.VKWelcomeAttachment,
		MenuButtonMode:                      cfg.VKMenuButtonMode,
		UnroutedTextMode:                    cfg.VKUnroutedTextMode,
		MenuFeatures:                        vkMenuFeatures(cfg),
		ReferralLinkBase:                    cfg.VKReferralLinkBase,
		ReferralShareBase:                   cfg.VKReferralShareBase,
		ReferralReferrerSignupRewardCredits: cfg.ReferralReferrerSignupRewardCredits,
	}, vkinbound.Deps{
		Idempotency:  idem,
		Inbound:      inbound,
		Users:        users,
		Jobs:         jobs,
		Commands:     commands,
		Billing:      billing,
		Referrals:    referrals,
		Orchestrator: orch,
		Router:       router,
		Control:      vkControl,
		Profile:      vkProfile,
		DialogState:  vkDialogState,
		AntiSpam:     vkAntiSpam,
		Logger:       logger,
	})

	admin := adminapi.NewHandler(adminapi.Config{Token: cfg.AdminToken}, adminapi.Deps{
		Jobs:       jobs,
		Users:      users,
		Deliveries: deliveries,
		Billing:    billingRepo,
	})

	// Per-user rate limiting protects billable Mini App job creation after
	// launch params have been verified by the BFF.
	miniappJobLimiter := ratelimit.New(cfg.MiniAppJobRateLimitRPS, cfg.MiniAppJobRateLimitBurst)
	miniapp := miniappapi.NewHandler(miniappapi.Config{
		AppSecret:          cfg.VKAppSecret,
		LaunchParamsMaxAge: cfg.MiniAppLaunchParamsMaxAge,
		JobRateLimiter:     miniappJobLimiter,
	}, miniappapi.Deps{
		Users:        users,
		Jobs:         jobs,
		Artifacts:    artifacts,
		Moderation:   modResults,
		Objects:      objectStore,
		Billing:      billing,
		BillingRepo:  billingRepo,
		Orchestrator: orch,
		Logger:       logger,
	})

	// Per-IP rate limiting protects the webhook intake from flooding/abuse
	// (audit S3).
	webhookLimiter := ratelimit.New(cfg.WebhookRateLimitRPS, cfg.WebhookRateLimitBurst)

	mux := http.NewServeMux()
	mux.Handle("/webhooks/vk", webhookLimiter.Middleware(metrics.Middleware("webhook", vkHandler)))
	mux.Handle("/admin/", metrics.Middleware("admin", admin.Routes()))
	mux.Handle("/miniapp/", metrics.Middleware("miniapp", miniapp.Routes()))
	mux.Handle("GET /metrics", metrics.Handler())
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

func vkMenuFeatures(cfg config.Config) vkinbound.MenuFeatureFlags {
	disabled := map[domain.CommandType]bool{}
	disableWhenFalse := func(enabled bool, commands ...domain.CommandType) {
		if enabled {
			return
		}
		for _, command := range commands {
			disabled[command] = true
		}
	}

	disableWhenFalse(cfg.VKMenuVideoEnabled, domain.CommandMenuVideo)
	disableWhenFalse(cfg.VKMenuImageEnabled, domain.CommandMenuImage)
	disableWhenFalse(cfg.VKMenuGPTEnabled, domain.CommandMenuText)
	disableWhenFalse(cfg.VKMenuStudentsEnabled, domain.CommandMenuStudents)
	disableWhenFalse(cfg.VKMenuAccountEnabled, domain.CommandAccount)
	disableWhenFalse(cfg.VKMenuTopUpEnabled, domain.CommandTopUp)
	disableWhenFalse(cfg.VKMenuVideoSora2Enabled, domain.CommandMenuVideoSora2)
	disableWhenFalse(cfg.VKMenuVideoSora2StartEnabled, domain.CommandMenuVideoSora2Start)
	disableWhenFalse(cfg.VKMenuVideoSora2ExamplesEnabled, domain.CommandMenuVideoSora2Examples)
	disableWhenFalse(cfg.VKMenuVideoKling21Enabled, domain.CommandMenuVideoKling21)
	disableWhenFalse(cfg.VKMenuVideoKling21StartEnabled, domain.CommandMenuVideoKling21Start)
	disableWhenFalse(cfg.VKMenuVideoKling21ExamplesEnabled, domain.CommandMenuVideoKling21Examples)
	disableWhenFalse(cfg.VKMenuVideoSeedance1Enabled, domain.CommandMenuVideoSeedance1)
	disableWhenFalse(cfg.VKMenuVideoSeedance1LiteEnabled, domain.CommandMenuVideoSeedance1Lite)
	disableWhenFalse(cfg.VKMenuVideoSeedance1ProEnabled, domain.CommandMenuVideoSeedance1Pro)
	disableWhenFalse(cfg.VKMenuVideoHaiuo02Enabled, domain.CommandMenuVideoHaiuo02)
	disableWhenFalse(cfg.VKMenuVideoHaiuo02StandardEnabled, domain.CommandMenuVideoHaiuo02Standard)
	disableWhenFalse(cfg.VKMenuVideoHaiuo02FastEnabled, domain.CommandMenuVideoHaiuo02Fast)
	disableWhenFalse(cfg.VKMenuImageTextEnabled, domain.CommandMenuImageText)
	disableWhenFalse(cfg.VKMenuImageReferenceEnabled, domain.CommandMenuImageReference)
	disableWhenFalse(cfg.VKMenuStudentsSolverEnabled, domain.CommandMenuStudentSolver)
	disableWhenFalse(cfg.VKMenuStudentsPresentationEnabled, domain.CommandMenuStudentPresentation)
	disableWhenFalse(cfg.VKMenuStudentsReportEnabled, domain.CommandMenuStudentReport)
	disableWhenFalse(cfg.VKMenuStudentsQAEnabled, domain.CommandMenuStudentQA)

	return vkinbound.MenuFeatureFlags{DisabledCommands: disabled}
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
