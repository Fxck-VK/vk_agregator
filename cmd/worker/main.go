// Command worker runs the worker pools: generation (text/image/video), provider
// polling and delivery. Workers consume Redis Streams via consumer groups,
// recover un-acked work on startup and are the only place AI providers are
// called.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	"vk-ai-aggregator/internal/adapter/provider/mock"
	"vk-ai-aggregator/internal/adapter/provider/openai"
	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/adapter/storage/s3"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/tracing"
	"vk-ai-aggregator/internal/service/artifactservice"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/maintenance"
	"vk-ai-aggregator/internal/service/moderationservice"
	"vk-ai-aggregator/internal/service/outboxrelay"
	"vk-ai-aggregator/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	if err := cfg.Validate(); err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	rootCtx := context.Background()
	shutdownTracing, err := tracing.Init(rootCtx, tracing.Config{
		ServiceName: cfg.TracingServiceName + "-worker",
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

	readCtx, stopReads := context.WithCancel(rootCtx)
	handlerCtx, stopHandlers := context.WithCancel(rootCtx)
	defer stopHandlers()

	pool, err := postgres.NewPoolConfigured(readCtx, cfg.DatabaseURL, cfg.DBMaxConns, cfg.DBMinConns)
	if err != nil {
		logger.Error("postgres connect failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb := redisqueue.NewClientWithPool(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, cfg.RedisPoolSize)
	defer rdb.Close()

	store, err := s3.New(readCtx, s3.Config{
		Endpoint:  cfg.S3Endpoint,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
		UseSSL:    cfg.S3UseSSL,
	})
	if err != nil {
		logger.Error("s3 connect failed", "error", err)
		os.Exit(1)
	}
	if err := store.EnsureBucket(readCtx, cfg.S3Bucket); err != nil {
		logger.Error("s3 ensure bucket failed", "error", err)
		os.Exit(1)
	}
	// Configure object retention so generated artifacts are purged on a schedule
	// (audit ST1).
	if cfg.ArtifactRetentionDays > 0 {
		if err := store.SetRetention(readCtx, cfg.S3Bucket, cfg.ArtifactRetentionDays); err != nil {
			logger.Warn("s3 set retention failed", "error", err)
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

	billing := billingservice.New(billingRepo, billingservice.WithPriceOverrides(cfg.PriceOverrides))
	publisher := redisqueue.NewPublisher(rdb, cfg.StreamMaxLen)

	// Provider routing: the first provider is primary; later providers are
	// fallback candidates used by the worker registry when retryable submit
	// failures trip circuit breakers.
	var providerList []domain.Provider
	var artOpts []artifactservice.Option
	hasMockProvider := false
	for _, name := range cfg.ProviderChain {
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "openai":
			providerList = append(providerList, openai.New(openai.Config{
				APIKey:       cfg.OpenAIAPIKey,
				BaseURL:      cfg.OpenAIBaseURL,
				TextModel:    cfg.OpenAITextModel,
				ImageModel:   cfg.OpenAIImageModel,
				ImageSize:    cfg.OpenAIImageSize,
				VideoModel:   cfg.OpenAIVideoModel,
				VideoSeconds: cfg.OpenAIVideoSeconds,
				VideoSize:    cfg.OpenAIVideoSize,
				TextPrice:    cfg.OpenAITextPrice,
				ImagePrice:   cfg.OpenAIImagePrice,
				VideoPrice:   cfg.OpenAIVideoPrice,
			}))
			logger.Info("registered openai provider")
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
		// downloader to resolve them into real bytes for storage.
		artOpts = append(artOpts, artifactservice.WithDownloader(mock.NewDownloader()))
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
	providers := worker.NewRegistry(providerList[0], providerList[1:]...)

	// Delivery client selection: mock by default, real VK API when configured
	// (audit V1).
	var vkClient vkdelivery.Client
	switch cfg.VKDeliveryMode {
	case "real":
		vkClient = vkdelivery.NewHTTPClient(vkdelivery.HTTPConfig{
			AccessToken: cfg.VKAccessToken,
			APIVersion:  cfg.VKAPIVersion,
			BaseURL:     cfg.VKAPIBaseURL,
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
		Jobs:        jobs,
		Tasks:       tasks,
		Artifacts:   artSvc,
		Providers:   providers,
		Streams:     publisher,
		Moderator:   moderator,
		ModResults:  modResults,
		Releaser:    billing,
		MaxAttempts: cfg.MaxAttempts,
		Backoff:     worker.ExponentialBackoff(cfg.RetryBaseDelay, cfg.RetryMaxDelay),
	}
	gen := worker.NewGenerationWorker(deps)
	poll := worker.NewPollWorker(deps)
	delivery := worker.NewDeliveryWorker(worker.DeliveryDeps{
		Jobs:        jobs,
		Deliveries:  deliveries,
		Artifacts:   artRepo,
		Objects:     store,
		VK:          vkClient,
		Billing:     billing,
		Streams:     publisher,
		MaxAttempts: cfg.MaxAttempts,
		Backoff:     worker.ExponentialBackoff(cfg.RetryBaseDelay, cfg.RetryMaxDelay),
		Signer:      store,
		SignedURLs:  cfg.SignedDelivery,
		URLTTL:      cfg.ArtifactURLTTL,
	})

	// The outbox relay publishes queued jobs from the transactional outbox to the
	// worker queue, so a crash between commit and enqueue cannot lose work
	// (audit A2).
	relay := outboxrelay.New(postgres.NewUnitOfWork(pool), publisher, outboxrelay.WithLogger(logger))

	consumer := redisqueue.NewConsumer(rdb, cfg.WorkerGroup, cfg.WorkerConsumer)
	if err := consumer.EnsureGroups(readCtx, redisqueue.AllStreams...); err != nil {
		logger.Error("ensure consumer groups failed", "error", err)
		os.Exit(1)
	}

	genStreams := []string{redisqueue.StreamText, redisqueue.StreamImage, redisqueue.StreamVideo}
	engines := []*worker.Engine{
		worker.NewEngine(consumer, genStreams, gen.Process, worker.WithLogger(logger)),
		worker.NewEngine(consumer, []string{redisqueue.StreamProviderPoll}, poll.Process, worker.WithLogger(logger)),
		worker.NewEngine(consumer, []string{redisqueue.StreamDelivery}, delivery.Process, worker.WithLogger(logger)),
	}

	var wg sync.WaitGroup
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

	maintenanceSvc := maintenance.New(
		maintenanceRepo,
		redisqueue.NewTrimmer(rdb, cfg.StreamMaxLen, redisqueue.AllStreamsWithDLQ...),
		maintenance.Config{
			Interval:                      cfg.MaintenanceInterval,
			OutboxRetention:               cfg.OutboxRetention,
			BillingReconciliationInterval: cfg.BillingReconciliationInterval,
			BillingReconciliationLimit:    cfg.BillingReconciliationLimit,
		},
		maintenance.WithLogger(logger),
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		maintenanceSvc.Run(readCtx)
	}()

	var metricsSrv *http.Server
	if cfg.WorkerMetricsAddr != "" {
		mux := http.NewServeMux()
		mux.Handle("GET /metrics", metrics.Handler())
		mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
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
				logger.Error("worker metrics server error", "error", err)
				os.Exit(1)
			}
		}()
	}
	logger.Info("workers started", "group", cfg.WorkerGroup, "consumer", cfg.WorkerConsumer)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	logger.Info("shutting down workers", "grace", cfg.WorkerShutdownGrace)
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
