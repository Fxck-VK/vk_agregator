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

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	"vk-ai-aggregator/internal/adapter/provider/mock"
	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/adapter/storage/s3"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/artifactservice"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("postgres connect failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb := redisqueue.NewClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
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

	// Repositories and services.
	jobs := postgres.NewJobRepository(pool)
	tasks := postgres.NewProviderTaskRepository(pool)
	artRepo := postgres.NewArtifactRepository(pool)
	deliveries := postgres.NewDeliveryRepository(pool)
	billingRepo := postgres.NewBillingRepository(pool)

	billing := billingservice.New(billingRepo)
	// The mock provider emits synthetic mock:// output URLs, so use a matching
	// downloader to resolve them into real bytes for storage.
	artSvc := artifactservice.New(artRepo, store, cfg.S3Bucket, artifactservice.WithDownloader(mock.NewDownloader()))
	publisher := redisqueue.NewPublisher(rdb, 100000)

	// Provider registry: only the mock provider is implemented so far.
	providers := worker.NewRegistry(mock.New())

	gen := worker.NewGenerationWorker(worker.Deps{
		Jobs:      jobs,
		Tasks:     tasks,
		Artifacts: artSvc,
		Providers: providers,
		Streams:   publisher,
	})
	poll := worker.NewPollWorker(worker.Deps{
		Jobs:      jobs,
		Tasks:     tasks,
		Artifacts: artSvc,
		Providers: providers,
		Streams:   publisher,
	})
	delivery := worker.NewDeliveryWorker(worker.DeliveryDeps{
		Jobs:       jobs,
		Deliveries: deliveries,
		Artifacts:  artRepo,
		Objects:    store,
		VK:         vkdelivery.NewMockClient(),
		Billing:    billing,
	})

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
	logger.Info("workers started", "group", cfg.WorkerGroup, "consumer", cfg.WorkerConsumer)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	logger.Info("shutting down workers")
	cancel()
	wg.Wait()
	logger.Info("workers stopped")
}
