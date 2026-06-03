package worker_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	vkinbound "vk-ai-aggregator/internal/adapter/inbound/vk"
	"vk-ai-aggregator/internal/adapter/provider/mock"
	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/service/artifactservice"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/commandrouter"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/worker"
)

// TestEndToEnd exercises the full pipeline with in-memory adapters:
// VK webhook -> Job -> (queue) -> Generation worker -> Provider -> Artifact ->
// Delivery worker -> VK send -> Billing capture -> Job success.
func TestEndToEnd(t *testing.T) {
	ctx := context.Background()

	// Storage + infra.
	users := memory.NewUserRepo()
	jobs := memory.NewJobRepo()
	commands := memory.NewCommandRepo()
	inbound := memory.NewInboundRepo()
	idem := memory.NewIdempotencyRepo()
	outbox := memory.NewOutboxRepo()
	ptasks := memory.NewProviderTaskRepo()
	deliveries := memory.NewDeliveryRepo()
	artRepo := memory.NewArtifactRepo()
	objects := memory.NewObjectStore()
	billingRepo := memory.NewBillingRepo()

	// Services.
	billing := billingservice.New(billingRepo)
	uowMgr := memory.NewUnitOfWork(jobs, outbox)
	publisher := queue.NewMemoryPublisher()
	orch := joborchestrator.New(jobs, uowMgr, billing, publisher)
	router := commandrouter.New()

	// VK inbound gateway.
	vkHandler := vkinbound.NewHandler(vkinbound.Config{ConfirmationToken: "tok"}, vkinbound.Deps{
		Idempotency:  idem,
		Inbound:      inbound,
		Users:        users,
		Commands:     commands,
		Billing:      billing,
		Orchestrator: orch,
		Router:       router,
	})

	// Workers (provider is only ever called here).
	artSvc := artifactservice.New(artRepo, objects, "artifacts",
		artifactservice.WithDownloader(stubDownloader{data: []byte("pixels"), contentType: "image/png"}))
	streams := newFakeStreams()
	gen := worker.NewGenerationWorker(worker.Deps{
		Jobs:      jobs,
		Tasks:     ptasks,
		Artifacts: artSvc,
		Providers: worker.NewRegistry(mock.New()),
		Streams:   streams,
	})
	vkClient := vkdelivery.NewMockClient()
	del := worker.NewDeliveryWorker(worker.DeliveryDeps{
		Jobs:       jobs,
		Deliveries: deliveries,
		Artifacts:  artRepo,
		Objects:    objects,
		VK:         vkClient,
		Billing:    billing,
	})

	// 1. VK delivers a message_new event.
	body := `{"type":"message_new","group_id":1,"event_id":"evt-1","object":{"message":{"from_id":777,"peer_id":777,"text":"/image a neon cat"}}}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks/vk", strings.NewReader(body))
	rec := httptest.NewRecorder()
	vkHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "ok" {
		t.Fatalf("webhook: code=%d body=%q", rec.Code, rec.Body.String())
	}

	// 2. A queued image job now exists.
	jobList, err := jobs.List(ctx, domain.JobFilter{}, 10, 0)
	if err != nil || len(jobList) != 1 {
		t.Fatalf("expected one job, got %d (err %v)", len(jobList), err)
	}
	job := jobList[0]
	if job.Status != domain.JobStatusQueued || job.OperationType != domain.OperationImageGenerate {
		t.Fatalf("unexpected job: status=%q op=%q", job.Status, job.OperationType)
	}

	// 3. Generation worker submits to the provider, stores the artifact and
	//    enqueues delivery.
	if err := gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("generation: %v", err)
	}
	job, _ = jobs.GetByID(ctx, job.ID)
	if job.Status != domain.JobStatusResultReady {
		t.Fatalf("after generation status=%q, want result_ready", job.Status)
	}
	if len(streams.byStream[redisqueue.StreamDelivery]) != 1 {
		t.Fatalf("expected delivery enqueue, got %v", streams.byStream)
	}

	// 4. Delivery worker sends to VK and captures credits.
	if err := del.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("delivery: %v", err)
	}

	// 5. Final assertions across the whole flow.
	job, _ = jobs.GetByID(ctx, job.ID)
	if job.Status != domain.JobStatusSucceeded {
		t.Fatalf("final status=%q, want succeeded", job.Status)
	}
	if job.CostCaptured != 10 {
		t.Fatalf("captured=%d, want 10", job.CostCaptured)
	}
	if sent := vkClient.Sent(); len(sent) != 1 || sent[0].Type != "photo" {
		t.Fatalf("expected one photo send, got %+v", sent)
	}
	acc, _ := billingRepo.GetAccountByUser(ctx, job.UserID, domain.CurrencyCredits)
	if acc.BalanceCached != billingservice.DefaultStartingBalance-10 {
		t.Fatalf("balance=%d, want %d", acc.BalanceCached, billingservice.DefaultStartingBalance-10)
	}
}
