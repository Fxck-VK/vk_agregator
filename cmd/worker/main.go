// Command worker runs the worker pools: generation (text/image/video), provider
// polling and delivery. Workers consume Redis Streams via consumer groups,
// recover un-acked work on startup and are the only place AI providers are
// called.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	"vk-ai-aggregator/internal/adapter/provider/apimart"
	"vk-ai-aggregator/internal/adapter/provider/deepinfra"
	"vk-ai-aggregator/internal/adapter/provider/mock"
	"vk-ai-aggregator/internal/adapter/provider/openai"
	"vk-ai-aggregator/internal/adapter/provider/poyo"
	"vk-ai-aggregator/internal/adapter/provider/runway"
	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/adapter/storage/s3"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/platform/logging"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/readiness"
	"vk-ai-aggregator/internal/platform/tracing"
	"vk-ai-aggregator/internal/service/artifactservice"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/dialogcontext"
	"vk-ai-aggregator/internal/service/maintenance"
	"vk-ai-aggregator/internal/service/mediaprobe"
	"vk-ai-aggregator/internal/service/mediatranscode"
	"vk-ai-aggregator/internal/service/moderationservice"
	"vk-ai-aggregator/internal/service/outboxrelay"
	"vk-ai-aggregator/internal/worker"
)

type workerReadyPool interface {
	Ping(context.Context) error
	readiness.SchemaQuerier
}

type workerReadyObjectStore interface {
	BucketReady(context.Context, string) error
}

func main() {
	logger := slog.New(logging.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	if err := cfg.Validate(); err != nil {
		logger.Error("invalid configuration", logging.ErrorAttr(err))
		os.Exit(1)
	}

	rootCtx := context.Background()
	shutdownTracing, err := tracing.Init(rootCtx, tracing.Config{
		ServiceName:         cfg.TracingServiceName + "-worker",
		Exporter:            cfg.TracingExporter,
		OTLPEndpoint:        cfg.TracingOTLPEndpoint,
		SampleRatio:         cfg.TracingSampleRatio,
		CriticalSampleRatio: cfg.TracingCriticalSampleRatio,
	}, logger)
	if err != nil {
		logger.Error("tracing init failed", logging.ErrorAttr(err))
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracing(shutdownCtx)
	}()

	readCtx, stopReads := context.WithCancel(rootCtx)
	handlerCtx, stopHandlers := context.WithCancel(rootCtx)
	defer stopHandlers()

	pool, err := postgres.NewPoolConfigured(readCtx, cfg.DatabaseURL, cfg.DBMaxConns, cfg.DBMinConns)
	if err != nil {
		logger.Error("postgres connect failed", logging.ErrorAttr(err))
		os.Exit(1)
	}
	defer pool.Close()

	rdb := redisqueue.NewClientWithPool(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, cfg.RedisPoolSize)
	defer func() {
		_ = rdb.Close()
	}()

	store, err := s3.New(readCtx, s3.Config{
		Endpoint:        cfg.S3Endpoint,
		AccessKey:       cfg.S3AccessKey,
		SecretKey:       cfg.S3SecretKey,
		UseSSL:          cfg.S3UseSSL,
		Region:          cfg.S3Region,
		AddressingStyle: cfg.S3AddressingStyle,
	})
	if err != nil {
		logger.Error("s3 connect failed", logging.ErrorAttr(err))
		os.Exit(1)
	}
	if err := store.EnsureBucket(readCtx, cfg.S3Bucket); err != nil {
		logger.Error("s3 ensure bucket failed", logging.ErrorAttr(err))
		os.Exit(1)
	}
	// Configure object retention so generated artifacts are purged on a schedule
	// (audit ST1).
	if cfg.ArtifactRetentionDays > 0 {
		if err := store.SetRetention(readCtx, cfg.S3Bucket, cfg.ArtifactRetentionDays); err != nil {
			logger.Warn("s3 set retention failed", logging.ErrorAttr(err))
		}
	}

	// Repositories and services.
	jobs := postgres.NewJobRepository(pool)
	tasks := postgres.NewProviderTaskRepository(pool)
	artRepo := postgres.NewArtifactRepository(pool)
	deliveries := postgres.NewDeliveryRepository(pool)
	billingRepo := postgres.NewBillingRepository(pool)
	modResults := postgres.NewModerationResultRepository(pool)
	maintenanceRepo := postgres.NewMaintenanceRepository(pool)
	conversations := postgres.NewConversationRepository(pool)

	billing := billingservice.New(billingRepo, billingservice.WithPriceOverrides(cfg.PriceOverrides))
	publisher := redisqueue.NewPublisher(rdb, cfg.StreamMaxLen)

	// Provider routing: the first provider is primary; later providers are
	// fallback candidates used by the worker registry when retryable submit
	// failures trip circuit breakers.
	var providerList []domain.Provider
	var artOpts []artifactservice.Option
	hasMockProvider := false
	providerNames := append([]string(nil), cfg.ProviderChain...)
	if cfg.ImageProvider != "" && !containsProvider(providerNames, cfg.ImageProvider) {
		providerNames = append(providerNames, cfg.ImageProvider)
	}
	if cfg.VideoProvider != "" && !containsProvider(providerNames, cfg.VideoProvider) {
		providerNames = append(providerNames, cfg.VideoProvider)
	}
	if (cfg.ImageProvider != "" || cfg.VideoProvider != "") && !cfg.IsProduction() && !containsProvider(providerNames, string(domain.ProviderMock)) {
		providerNames = append(providerNames, string(domain.ProviderMock))
	}
	for _, name := range providerNames {
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "apimart":
			if !cfg.APIMartProviderEnabled {
				logger.Warn("apimart provider skipped; provider switch disabled")
				continue
			}
			providerList = append(providerList, apimart.New(apimart.Config{
				APIKey:  cfg.APIMartAPIKey,
				BaseURL: cfg.APIMartBaseURL,
			}))
			logger.Info("registered apimart provider")
		case "poyo":
			if !cfg.PoYoProviderEnabled {
				logger.Warn("poyo provider skipped; provider switch disabled")
				continue
			}
			if strings.TrimSpace(cfg.PoYoBaseURL) == "" {
				logger.Warn("poyo provider skipped; base url is empty")
				continue
			}
			providerList = append(providerList, poyo.New(poyo.Config{
				APIKey:  cfg.PoYoAPIKey,
				BaseURL: cfg.PoYoBaseURL,
			}))
			logger.Info("registered poyo provider")
		case "runway":
			if !cfg.RunwayProviderEnabled {
				logger.Warn("runway provider skipped; provider switch disabled")
				continue
			}
			if strings.TrimSpace(cfg.RunwayMLAPISecret) == "" {
				logger.Warn("runway provider skipped; api secret is empty")
				continue
			}
			if strings.TrimSpace(cfg.RunwayMLBaseURL) == "" {
				logger.Warn("runway provider skipped; base url is empty")
				continue
			}
			providerList = append(providerList, runway.New(runway.Config{
				APISecret: cfg.RunwayMLAPISecret,
				BaseURL:   cfg.RunwayMLBaseURL,
			}))
			logger.Info("registered runway provider")
		case "deepinfra":
			providerList = append(providerList, deepinfra.New(deepinfra.Config{
				APIKey:                  cfg.DeepInfraAPIKey,
				BaseURL:                 cfg.DeepInfraBaseURL,
				TextModel:               cfg.DeepInfraTextModel,
				TextProviderCostCredits: cfg.DeepInfraTextProviderCostCredits,
			}))
			logger.Info("registered deepinfra provider")
		case "mock":
			providerList = append(providerList, mock.New())
			hasMockProvider = true
			logger.Info("registered mock provider")
		case "":
			continue
		default:
			logger.Warn("unknown provider skipped", "provider", name)
		}
	}
	if len(providerList) == 0 {
		providerList = append(providerList, mock.New())
		hasMockProvider = true
		logger.Warn("provider chain empty; using mock provider")
	}
	if hasMockProvider {
		// The mock provider emits synthetic mock:// output URLs, so use a matching
		// downloader to resolve them into real bytes for storage while delegating
		// real provider URLs to the SSRF-hardened platform downloader.
		artOpts = append(artOpts, artifactservice.WithDownloader(mock.NewDownloader(artifactservice.NewHTTPDownloader())))
	}

	var openAIModerator *openai.Moderator
	if strings.EqualFold(cfg.ModerationProvider, "openai") || strings.EqualFold(cfg.ArtifactScanner, "openai") {
		openAIModerator = openai.NewModerator(openai.ModerationConfig{
			APIKey:  cfg.OpenAIAPIKey,
			BaseURL: cfg.OpenAIBaseURL,
			Model:   cfg.OpenAIModerationModel,
		})
	}
	if strings.EqualFold(cfg.ArtifactScanner, "openai") {
		artOpts = append(artOpts, artifactservice.WithScanner(openAIModerator))
		logger.Info("using openai artifact scanner")
	}
	artSvc := artifactservice.New(artRepo, store, cfg.S3Bucket, artOpts...)
	var videoProber worker.VideoProber
	var videoTranscoder worker.VideoTranscoder
	probePolicy := cfg.EffectiveMediaVideoProbePolicy()
	transcodePolicy := cfg.EffectiveMediaVideoTranscodePolicy()
	rawProviderVideoPolicy := cfg.EffectiveMediaDeliverRawProviderVideo()
	logger.Info("media video policy loaded",
		"probe_policy", probePolicy,
		"transcode_policy", transcodePolicy,
		"raw_provider_video_policy", rawProviderVideoPolicy)
	if cfg.MediaPipelineEnabled && probePolicy == config.MediaVideoProbePolicyProbeRequired {
		videoProber = mediaprobe.NewFFProbe(mediaprobe.Config{
			FFProbePath:            cfg.FFProbePath,
			MaxVideoSizeBytes:      cfg.MediaMaxVideoSizeBytes,
			MaxVideoDurationSec:    cfg.MediaMaxVideoDurationSec,
			MaxVideoWidth:          cfg.MediaMaxVideoWidth,
			MaxVideoHeight:         cfg.MediaMaxVideoHeight,
			MaxVideoBitrate:        cfg.MediaMaxVideoBitrate,
			AllowedVideoContainers: cfg.MediaAllowedVideoContainers,
			AllowedVideoCodecs:     cfg.MediaAllowedVideoCodecs,
			Timeout:                cfg.MediaProbeTimeout,
		})
		logger.Info("using media video probe", "policy", probePolicy)
	}
	if cfg.MediaPipelineEnabled && cfg.MediaVideoTranscodeEnabled() {
		videoTranscoder = mediatranscode.NewFFmpeg(mediatranscode.Config{
			FFmpegPath:        cfg.FFmpegPath,
			MaxVideoSizeBytes: cfg.MediaMaxVideoSizeBytes,
			MaxVideoWidth:     cfg.MediaMaxVideoWidth,
			MaxVideoHeight:    cfg.MediaMaxVideoHeight,
			MaxVideoBitrate:   cfg.MediaMaxVideoBitrate,
			TranscodeTimeout:  cfg.MediaTranscodeTimeout,
		})
		logger.Info("using media video transcode", "policy", transcodePolicy)
	} else if cfg.MediaPipelineEnabled {
		logger.Info("media video transcode disabled", "policy", transcodePolicy)
	}
	if cfg.MediaVideoProbeRequired() && videoProber == nil {
		logger.Warn("media video probe unavailable; video jobs that require probing will fail closed", "policy", probePolicy)
	}
	providers := worker.NewRegistry(providerList[0], providerList[1:]...)
	if cfg.ImageProvider != "" {
		providers.PreferProvider(domain.ModalityImage, domain.ProviderName(strings.ToLower(strings.TrimSpace(cfg.ImageProvider))))
	}
	if cfg.VideoProvider != "" {
		providers.PreferProvider(domain.ModalityVideo, domain.ProviderName(strings.ToLower(strings.TrimSpace(cfg.VideoProvider))))
	}

	// Delivery client selection: mock by default, real VK API when configured
	// (audit V1).
	var vkClient vkdelivery.Client
	switch cfg.VKDeliveryMode {
	case "real":
		vkClient = vkdelivery.NewHTTPClient(vkdelivery.HTTPConfig{
			AccessToken:        cfg.VKAccessToken,
			VideoAccessToken:   cfg.VKVideoAccessToken,
			VideoUploadGroupID: cfg.VKVideoUploadGroupID,
			VideoDeliveryMode:  cfg.VKVideoDeliveryMode,
			APIVersion:         cfg.VKAPIVersion,
			BaseURL:            cfg.VKAPIBaseURL,
		})
		logger.Info("using real vk delivery client")
	default:
		vkClient = vkdelivery.NewMockClient()
	}
	// Output moderation gates delivery (invariant #15). Keyword moderation is
	// the local default; OpenAI moderation is enabled with MODERATION_PROVIDER.
	var moderator worker.Moderator = moderationservice.NewKeywordModerator(cfg.ModerationExtraTerms...)
	if strings.EqualFold(cfg.ModerationProvider, "openai") {
		moderator = openAIModerator
		logger.Info("using openai moderation provider")
	}

	deps := worker.Deps{
		Jobs:                                  jobs,
		Tasks:                                 tasks,
		Artifacts:                             artSvc,
		ArtifactRepo:                          artRepo,
		Objects:                               store,
		Providers:                             providers,
		Streams:                               publisher,
		ImageModel:                            cfg.ImageModel,
		ImageSize:                             cfg.ImageSize,
		VideoModel:                            cfg.VideoModel,
		VideoDurationSec:                      cfg.VideoDurationSec,
		VideoResolution:                       cfg.VideoResolution,
		VideoAspectRatio:                      cfg.VideoAspectRatio,
		VideoDraft:                            cfg.VideoDraft,
		VideoProber:                           videoProber,
		VideoTranscoder:                       videoTranscoder,
		RequireVideoProbe:                     cfg.MediaVideoProbeRequired(),
		VideoTranscodeEnabled:                 cfg.MediaVideoTranscodeEnabled(),
		VideoTranscodePolicy:                  cfg.EffectiveMediaVideoTranscodePolicy(),
		RawVideoDeliveryPolicy:                cfg.EffectiveMediaDeliverRawProviderVideo(),
		ProviderMediaContracts:                effectiveProviderMediaContracts(cfg),
		MediaMaxConcurrentProbes:              cfg.MediaMaxConcurrentProbes,
		MediaMaxConcurrentTranscodes:          cfg.MediaMaxConcurrentTranscodes,
		MediaMaxPendingVariants:               cfg.MediaMaxPendingVariants,
		MediaProviderMaxAttempts:              cfg.MediaProviderMaxAttemptsPerJob,
		MediaProviderFallbackBudget:           cfg.MediaProviderFallbackBudget,
		MediaProviderQualityGuardEnabled:      cfg.MediaProviderQualityGuardEnabled,
		MediaProviderQualityDegradedFailures:  cfg.MediaProviderQualityDegradedFailures,
		MediaProviderQualityDisabledFailures:  cfg.MediaProviderQualityDisabledFailures,
		MediaProviderQualityRecoverySuccesses: cfg.MediaProviderQualityRecoverySuccesses,
		ProviderCallTimeout:                   cfg.WorkerProviderCallTimeout,
		ProviderPollBackoff:                   worker.ExponentialBackoff(cfg.WorkerProviderPollBaseDelay, cfg.WorkerProviderPollMaxDelay),
		TextContext: dialogcontext.New(conversations, dialogcontext.Config{
			Enabled:                cfg.TextContextEnabled,
			MaxInputTokens:         cfg.TextContextMaxInputTokens,
			MaxOutputTokens:        cfg.TextContextMaxOutputTokens,
			SummaryMaxTokens:       cfg.TextContextSummaryMaxTokens,
			RecentMessagesLimit:    cfg.TextContextRecentMessagesLimit,
			SummarizeAfterMessages: cfg.TextContextSummarizeAfterMessages,
			SummarizeAfterTokens:   cfg.TextContextSummarizeAfterTokens,
		}),
		Moderator:   moderator,
		ModResults:  modResults,
		Releaser:    billing,
		MaxAttempts: cfg.MaxAttempts,
		Backoff:     worker.ExponentialBackoff(cfg.RetryBaseDelay, cfg.RetryMaxDelay),
	}
	gen := worker.NewGenerationWorker(deps)
	poll := worker.NewPollWorker(deps)
	delivery := worker.NewDeliveryWorker(worker.DeliveryDeps{
		Jobs:                   jobs,
		Deliveries:             deliveries,
		Artifacts:              artRepo,
		Objects:                store,
		VK:                     vkClient,
		Billing:                billing,
		Streams:                publisher,
		MaxAttempts:            cfg.MaxAttempts,
		Backoff:                worker.ExponentialBackoff(cfg.RetryBaseDelay, cfg.RetryMaxDelay),
		Signer:                 store,
		SignedURLs:             cfg.SignedDelivery,
		RawVideoDeliveryPolicy: cfg.EffectiveMediaDeliverRawProviderVideo(),
		URLTTL:                 cfg.ArtifactURLTTL,
	})

	// The outbox relay publishes queued jobs from the transactional outbox to the
	// worker queue, so a crash between commit and enqueue cannot lose work
	// (audit A2).
	relay := outboxrelay.New(postgres.NewUnitOfWork(pool), publisher, outboxrelay.WithLogger(logger))

	runJobWorkers := shouldRunJobWorkers(cfg.WorkerMode)
	runMaintenance := shouldRunMaintenance(cfg.WorkerMode)
	var wg sync.WaitGroup
	if runJobWorkers {
		consumer := redisqueue.NewConsumer(rdb, cfg.WorkerGroup, cfg.WorkerConsumer)
		if err := consumer.EnsureGroups(readCtx, redisqueue.AllStreams...); err != nil {
			logger.Error("ensure consumer groups failed", logging.ErrorAttr(err))
			os.Exit(1)
		}

		genStreams := []string{redisqueue.StreamText, redisqueue.StreamImage, redisqueue.StreamVideo}
		engines := []*worker.Engine{
			worker.NewEngine(consumer, genStreams, gen.Process, worker.WithLogger(logger)),
			worker.NewEngine(consumer, []string{redisqueue.StreamProviderPoll}, poll.Process, worker.WithLogger(logger)),
			worker.NewEngine(consumer, []string{redisqueue.StreamDelivery}, delivery.Process, worker.WithLogger(logger)),
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			runQueueMetrics(readCtx, rdb, cfg.WorkerGroup, 15*time.Second, logger)
		}()
		for _, e := range engines {
			wg.Add(1)
			go func(eng *worker.Engine) {
				defer wg.Done()
				_ = eng.RunWithHandlerContext(readCtx, handlerCtx)
			}(e)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			relay.Run(readCtx, time.Second)
		}()
		logger.Info("job worker loops started", "mode", cfg.WorkerMode, "group", cfg.WorkerGroup, "consumer", cfg.WorkerConsumer)
	} else {
		logger.Info("job worker loops disabled", "mode", cfg.WorkerMode)
	}

	if runMaintenance {
		maintenanceSvc := maintenance.New(
			maintenanceRepo,
			redisqueue.NewTrimmer(rdb, cfg.StreamMaxLen, redisqueue.AllStreamsWithDLQ...),
			maintenance.Config{
				Interval:                      cfg.MaintenanceInterval,
				OutboxRetention:               cfg.OutboxRetention,
				BillingReconciliationInterval: cfg.BillingReconciliationInterval,
				BillingReconciliationLimit:    cfg.BillingReconciliationLimit,
				JobEventsRetention:            time.Duration(cfg.JobEventsRetentionDays) * 24 * time.Hour,
				ProviderPayloadRetention:      time.Duration(cfg.ProviderPayloadRetentionDays) * 24 * time.Hour,
				InboundPayloadRetention:       time.Duration(cfg.VKInboundPayloadRetentionDays) * 24 * time.Hour,
				InboundPayloadRetentionLimit:  cfg.VKInboundRetentionBatchSize,
				CommandRawTextRetention:       time.Duration(cfg.CommandRawTextRetentionDays) * 24 * time.Hour,
				CommandRetentionLimit:         cfg.CommandRetentionBatchSize,
				JobLogRetentionLimit:          cfg.JobLogRetentionBatchSize,
				JobErrorAggregateLookback:     time.Duration(cfg.JobErrorAggregateLookbackDays) * 24 * time.Hour,
				AnalyticsAggregateLookback:    time.Duration(cfg.AnalyticsAggregateLookbackDays) * 24 * time.Hour,
				ConversationMessageRetention:  time.Duration(cfg.ConversationMessageRetentionDays) * 24 * time.Hour,
				ConversationSummaryRetention:  time.Duration(cfg.ConversationSummaryRetentionDays) * 24 * time.Hour,
				ConversationRetentionLimit:    cfg.ConversationRetentionBatchSize,
				MediaFreeRetention:            time.Duration(cfg.ArtifactFreeRetentionDays) * 24 * time.Hour,
				MediaPaidRetention:            time.Duration(cfg.ArtifactPaidRetentionDays) * 24 * time.Hour,
				MediaOrphanRetention:          time.Duration(cfg.ArtifactOrphanRetentionDays) * 24 * time.Hour,
				MediaTempUploadRetention:      time.Duration(cfg.ArtifactTemporaryRetentionDays) * 24 * time.Hour,
				MediaInputRetention:           time.Duration(cfg.MediaInputRetentionDays) * 24 * time.Hour,
				MediaFailedRetention:          time.Duration(cfg.MediaFailedRetentionDays) * 24 * time.Hour,
				MediaOriginalRetention:        time.Duration(cfg.MediaOriginalRetentionDays) * 24 * time.Hour,
				MediaVariantRetention:         time.Duration(cfg.MediaVariantRetentionDays) * 24 * time.Hour,
			},
			maintenance.WithLogger(logger),
			maintenance.WithMediaObjectStore(store),
		)
		wg.Add(1)
		go func() {
			defer wg.Done()
			maintenanceSvc.Run(readCtx)
		}()
		logger.Info("maintenance loop started", "mode", cfg.WorkerMode)
	} else {
		logger.Info("maintenance loop disabled", "mode", cfg.WorkerMode)
	}

	var metricsSrv *http.Server
	if cfg.WorkerMetricsAddr != "" {
		mux := http.NewServeMux()
		mux.Handle("GET /metrics", metrics.PrivateHandler())
		workerReadyHandler := workerReadinessHandler(pool, rdb, store, cfg.S3Bucket, cfg.MigrationsDir)
		mux.HandleFunc("GET /readyz", workerReadyHandler)
		mux.HandleFunc("GET /healthz", workerReadyHandler)
		metricsSrv = &http.Server{
			Addr:              cfg.WorkerMetricsAddr,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Info("worker metrics listening", "addr", cfg.WorkerMetricsAddr)
			if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Error("worker metrics server error", logging.ErrorAttr(err))
				os.Exit(1)
			}
		}()
	}
	logger.Info("worker runtime started", "mode", cfg.WorkerMode, "group", cfg.WorkerGroup, "consumer", cfg.WorkerConsumer)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	logger.Info("shutting down worker runtime", "mode", cfg.WorkerMode, "grace", cfg.WorkerShutdownGrace)
	stopReads()
	if metricsSrv != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = metricsSrv.Shutdown(shutdownCtx)
		cancel()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	grace := cfg.WorkerShutdownGrace
	if grace <= 0 {
		grace = 30 * time.Second
	}
	select {
	case <-done:
	case <-time.After(grace):
		logger.Warn("worker drain timeout; cancelling in-flight handlers")
		stopHandlers()
		<-done
	}
	logger.Info("workers stopped")
}

func workerReadinessHandler(pool workerReadyPool, rdb *redis.Client, store workerReadyObjectStore, bucket, migrationsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		checks := map[string]string{
			"postgres":   "ok",
			"redis":      "ok",
			"s3_bucket":  "ok",
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
		if err := store.BucketReady(ctx, bucket); err != nil {
			checks["s3_bucket"] = "down"
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

func containsProvider(names []string, want string) bool {
	for _, name := range names {
		if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(want)) {
			return true
		}
	}
	return false
}

func runQueueMetrics(ctx context.Context, rdb redis.Cmdable, group string, interval time.Duration, logger *slog.Logger) {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	collect := func() {
		if err := redisqueue.CollectMetrics(ctx, rdb, group, redisqueue.AllStreamsWithDLQ...); err != nil && logger != nil {
			logger.Warn("queue metrics collection failed", logging.ErrorAttr(err))
		}
	}
	collect()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			collect()
		}
	}
}

func effectiveProviderMediaContracts(cfg config.Config) []domain.ProviderMediaContract {
	defaults := defaultProviderMediaContracts(cfg)
	if len(cfg.MediaProviderContracts) == 0 {
		return defaults
	}
	out := make([]domain.ProviderMediaContract, 0, len(defaults)+len(cfg.MediaProviderContracts))
	out = append(out, defaults...)
	out = append(out, cfg.MediaProviderContracts...)
	return out
}

func defaultProviderMediaContracts(cfg config.Config) []domain.ProviderMediaContract {
	maxBytes := cfg.MediaMaxVideoSizeBytes
	if maxBytes <= 0 {
		maxBytes = 256 << 20
	}
	probeRequired := cfg.MediaVideoProbeRequired()
	transcodeAllowed := cfg.MediaVideoTranscodeEnabled()
	contracts := []domain.ProviderMediaContract{
		{
			Provider:               domain.ProviderMock,
			Model:                  "mock-video",
			ModelClass:             "mock_video",
			Modality:               domain.ModalityVideo,
			AllowedDurationsSec:    []int{3, 5, 10},
			AllowedAspectRatios:    []string{"16:9", "9:16", "1:1"},
			AllowedResolutions:     []string{"720p", "1080p"},
			ExpectedContainer:      "mp4",
			ExpectedCodec:          "h264",
			ExpectedMaxBytes:       maxBytes,
			DeliveryReadyOutput:    true,
			RequiresProbe:          probeRequired,
			TranscodeAllowed:       transcodeAllowed,
			MaxProviderAttempts:    1,
			MaxFallbackAttempts:    0,
			MaxProviderCostCredits: 50,
		},
	}
	for _, model := range []struct {
		id    string
		class string
	}{
		{id: apimart.ModelHailuo23Fast, class: "hailuo_2_3_fast"},
		{id: apimart.ModelHailuo23Standard, class: "hailuo_2_3_standard"},
	} {
		contracts = append(contracts, domain.ProviderMediaContract{
			Provider:               domain.ProviderAPIMart,
			Model:                  model.id,
			ModelClass:             model.class,
			Modality:               domain.ModalityVideo,
			AllowedDurationsSec:    []int{6, 10},
			AllowedResolutions:     []string{"768p", "1080p"},
			ExpectedContainer:      "mp4",
			ExpectedCodec:          "h264",
			ExpectedMaxBytes:       maxBytes,
			DeliveryReadyOutput:    true,
			RequiresProbe:          probeRequired,
			TranscodeAllowed:       transcodeAllowed,
			MaxProviderAttempts:    1,
			MaxFallbackAttempts:    0,
			MaxProviderCostCredits: 2,
		})
	}
	for _, model := range []struct {
		id          string
		class       string
		durations   []int
		resolutions []string
		maxCost     int64
	}{
		{id: poyo.ModelKlingO3Standard, class: "kling_o3_standard", durations: []int{5, 10}, resolutions: []string{"720p", "1080p"}, maxCost: 200},
		{id: poyo.ModelSeedance20Fast, class: "seedance_2_0_fast", durations: []int{5, 10}, resolutions: []string{"720p"}, maxCost: 560},
		{id: poyo.ModelRunwayGen45, class: "runway_gen4_5", durations: []int{5, 10}, resolutions: []string{"720p", "1080p"}, maxCost: 0},
	} {
		contracts = append(contracts, domain.ProviderMediaContract{
			Provider:               domain.ProviderPoYo,
			Model:                  model.id,
			ModelClass:             model.class,
			Modality:               domain.ModalityVideo,
			AllowedDurationsSec:    model.durations,
			AllowedAspectRatios:    []string{"16:9", "9:16", "1:1"},
			AllowedResolutions:     model.resolutions,
			ExpectedContainer:      "mp4",
			ExpectedCodec:          "h264",
			ExpectedMaxBytes:       maxBytes,
			DeliveryReadyOutput:    true,
			RequiresProbe:          probeRequired,
			TranscodeAllowed:       transcodeAllowed,
			MaxProviderAttempts:    1,
			MaxFallbackAttempts:    0,
			MaxProviderCostCredits: model.maxCost,
		})
	}
	contracts = append(contracts, domain.ProviderMediaContract{
		Provider:               domain.ProviderRunway,
		Model:                  runway.ModelGen4Turbo,
		ModelClass:             "runway_gen4_turbo",
		Modality:               domain.ModalityVideo,
		AllowedDurationsSec:    []int{2, 3, 4, 5, 6, 7, 8, 9, 10},
		AllowedAspectRatios:    []string{"16:9", "9:16", "4:3", "3:4", "1:1", "21:9"},
		AllowedResolutions:     []string{"720p"},
		ExpectedContainer:      "mp4",
		ExpectedCodec:          "h264",
		ExpectedMaxBytes:       maxBytes,
		DeliveryReadyOutput:    true,
		RequiresProbe:          probeRequired,
		TranscodeAllowed:       transcodeAllowed,
		MaxProviderAttempts:    1,
		MaxFallbackAttempts:    0,
		MaxProviderCostCredits: 100,
	})
	return contracts
}

func shouldRunJobWorkers(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", config.WorkerModeAll, config.WorkerModeJobs:
		return true
	default:
		return false
	}
}

func shouldRunMaintenance(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", config.WorkerModeAll, config.WorkerModeMaintenance:
		return true
	default:
		return false
	}
}
