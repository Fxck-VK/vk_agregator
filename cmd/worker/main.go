// Command worker runs the worker pools: generation (text/image/video), provider
// polling and delivery. Workers consume Redis Streams via consumer groups,
// recover un-acked work on startup and are the only place AI providers are
// called.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
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
	"vk-ai-aggregator/internal/service/artifactservice"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/moderationservice"
	"vk-ai-aggregator/internal/service/outboxrelay"
	"vk-ai-aggregator/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := postgres.NewPoolConfigured(ctx, cfg.DatabaseURL, cfg.DBMaxConns, cfg.DBMinConns)
	if err != nil {
		logger.Error("postgres connect failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb := redisqueue.NewClientWithPool(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, cfg.RedisPoolSize)
	defer rdb.Close()

	store, err := s3.New(ctx, s3.Config{
		Endpoint:  cfg.S3Endpoint,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
		UseSSL:    cfg.S3UseSSL,
	})
	if err != nil {
		logger.Error("s3 connect failed", "error", err)
		os.Exit(1)
	}
	if err := store.EnsureBucket(ctx, cfg.S3Bucket); err != nil {
		logger.Error("s3 ensure bucket failed", "error", err)
		os.Exit(1)
	}
	// Configure object retention so generated artifacts are purged on a schedule
	// (audit ST1).
	if cfg.ArtifactRetentionDays > 0 {
		if err := store.SetRetention(ctx, cfg.S3Bucket, cfg.ArtifactRetentionDays); err != nil {
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

	billing := billingservice.New(billingRepo, billingservice.WithPriceOverrides(cfg.PriceOverrides))
	publisher := redisqueue.NewPublisher(rdb, 100000)

	// Provider selection: the mock provider is the default; the OpenAI adapter is
	// used only when PROVIDER=openai and a key is configured (audit P1).
	var provider domain.Provider
	var artOpts []artifactservice.Option
	switch cfg.Provider {
	case "openai":
		provider = openai.New(openai.Config{
			APIKey:     cfg.OpenAIAPIKey,
			BaseURL:    cfg.OpenAIBaseURL,
			ImageModel: cfg.OpenAIImageModel,
		})
		logger.Info("using openai provider")
	default:
		provider = mock.New()
		// The mock provider emits synthetic mock:// output URLs, so use a matching
		// downloader to resolve them into real bytes for storage.
		artOpts = append(artOpts, artifactservice.WithDownloader(mock.NewDownloader()))
	}
	artSvc := artifactservice.New(artRepo, store, cfg.S3Bucket, artOpts...)
	providers := worker.NewRegistry(provider)

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
	// Output moderation gates delivery (invariant #15). The keyword moderator is
	// the default; swap for a provider-backed Moderator when available.
	moderator := moderationservice.NewKeywordModerator(cfg.ModerationExtraTerms...)

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
	if err := consumer.EnsureGroups(ctx, redisqueue.AllStreams...); err != nil {
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
			_ = eng.Run(ctx)
		}(e)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		relay.Run(ctx, time.Second)
	}()
	logger.Info("workers started", "group", cfg.WorkerGroup, "consumer", cfg.WorkerConsumer)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	logger.Info("shutting down workers")
	cancel()
	wg.Wait()
	logger.Info("workers stopped")
}
