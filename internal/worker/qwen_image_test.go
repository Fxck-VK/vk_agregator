package worker_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/provider/apimart"
	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/moderationservice"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/resultservice"
	"vk-ai-aggregator/internal/worker"
)

type qwenOutputModerator struct {
	blocked bool
	calls   int
}

func (m *qwenOutputModerator) Name() string { return "qwen-test-moderator" }
func (m *qwenOutputModerator) Check(context.Context, moderationservice.Input) (moderationservice.Outcome, error) {
	m.calls++
	decision := domain.ModerationAllow
	if m.blocked {
		decision = domain.ModerationBlock
	}
	return moderationservice.Outcome{Decision: decision}, nil
}

func TestQwenImageAsyncLifecycle(t *testing.T) {
	testAPIMartImageLifecycle(t, "qwen_image_3", "qwen-image-3.0", "2K", "2K", 15, true)
}

func TestGrokImageAsyncLifecycle(t *testing.T) {
	t.Run("1.5", func(t *testing.T) {
		testAPIMartImageLifecycle(t, "grok_image_1_5", apimart.ModelGrokImage15, "standard", "", 10, true)
	})
	t.Run("2.0", func(t *testing.T) {
		testAPIMartImageLifecycle(t, "grok_image_2_0", apimart.ModelGrokImage20, "standard", "quality", 10, false)
	})
}

func testAPIMartImageLifecycle(t *testing.T, publicModel, providerModel, quality, resolution string, credits int64, withReference bool) {
	t.Helper()
	scenarios := []string{"allowed", "moderation blocked"}
	if providerModel == apimart.ModelGrokImage20 {
		scenarios = append(scenarios, "submit indeterminate")
	}
	for _, name := range scenarios {
		blocked := name == "moderation blocked"
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			var submits atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/v1/images/generations":
					submits.Add(1)
					if name == "submit indeterminate" {
						w.WriteHeader(http.StatusConflict)
						_, _ = w.Write([]byte(`{"error":{"code":"idempotency_result_indeterminate"}}`))
						return
					}
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if body["model"] != providerModel || body["n"] != float64(1) {
						t.Error("wrong worker request")
					}
					if resolution == "" && body["resolution"] != nil || resolution != "" && body["resolution"] != resolution {
						t.Error("wrong provider resolution")
					}
					if withReference && body["image_urls"] == nil || !withReference && body["image_urls"] != nil {
						t.Error("wrong reference input")
					}
					if providerModel == apimart.ModelGrokImage20 {
						if r.Header.Get("X-APIMart-Response-Version") != "2026-07-27" || r.Header.Get("Idempotency-Key") == "" {
							t.Error("missing Grok 2.0 headers")
						}
						w.WriteHeader(http.StatusAccepted)
						_, _ = w.Write([]byte(`{"code":202,"data":{"id":"qwen-task","status":"queued"}}`))
						return
					}
					_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"qwen-task"}]}`))
				case r.Method == http.MethodGet && r.URL.Path == "/v1/tasks/qwen-task":
					_, _ = w.Write([]byte(`{"code":200,"data":{"status":"completed","result":{"images":[{"url":["https://example.com/qwen.png"]}]}}}`))
				default:
					t.Error("unexpected provider request")
					w.WriteHeader(400)
				}
			}))
			defer srv.Close()
			provider := apimart.New(apimart.Config{APIKey: "test-key", BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()})
			billingRepo := memory.NewBillingRepo()
			billing := billingservice.New(billingRepo)
			moderator := &qwenOutputModerator{blocked: blocked}
			h := newHarnessWithProvider(t, provider, func(d *worker.Deps) { d.Moderator = moderator; d.Releaser = billing })
			prices, err := pricingcatalog.NewStaticCatalog()
			if err != nil {
				t.Fatal(err)
			}
			snapshot, err := prices.Snapshot(pricingcatalog.ProductKey{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ImageModelID: publicModel, Quality: quality})
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			owner := uuid.New()
			input := map[string]any{"prompt": "Synthetic test image", "provider": domain.ProviderAPIMart, "model_code": providerModel, "model_id": publicModel, "resolution": resolution, "image_quality": quality, "size": "1:1"}
			if withReference {
				reference := h.createInputImageArtifact(t, owner, validPNGBytes(t), "image/png")
				input["reference_artifact_ids"] = []string{reference.ID.String()}
			}
			params, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			job := &domain.Job{ID: uuid.New(), AccountID: owner, UserID: owner, Source: "web", ResultMode: domain.ResultModeAccountHistory, ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb}, OperationType: domain.OperationImageGenerate, Modality: domain.ModalityImage, Status: domain.JobStatusQueued, IdempotencyKey: uuid.NewString(), CostEstimate: credits, CostReserved: credits, PricingSnapshot: raw, Params: params}
			if err := h.jobs.Create(ctx, job); err != nil {
				t.Fatal(err)
			}
			if _, err := billing.Reserve(ctx, owner, job.ID, credits); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if err := h.gen.Process(ctx, taskFor(job)); err != nil {
					t.Fatal(err)
				}
			}
			if submits.Load() != 1 {
				t.Fatal("Job replay resubmitted provider request")
			}
			if name == "submit indeterminate" {
				job = h.reload(t, job.ID)
				if job.Status != domain.JobStatusFailedTerminal || job.CostCaptured != 0 || moderator.calls != 0 || len(job.OutputArtifactIDs) != 0 {
					t.Fatal("uncertain submission retried or became visible")
				}
				account, err := billingRepo.GetAccountByUser(ctx, owner, domain.CurrencyCredits)
				if err != nil || account.BalanceCached != billingservice.DefaultStartingBalance {
					t.Fatal("uncertain submission charged")
				}
				return
			}
			pollTasks := h.streams.byStream[redisqueue.StreamProviderPoll]
			if len(pollTasks) == 0 {
				t.Fatal("no async polling task")
			}
			for i := 0; i < 2; i++ {
				if err := h.poll.Process(ctx, pollTasks[0]); err != nil {
					t.Fatal(err)
				}
			}
			job = h.reload(t, job.ID)
			if moderator.calls != 1 {
				t.Fatalf("moderation calls = %d", moderator.calls)
			}
			if blocked {
				if job.Status != domain.JobStatusRejected || job.CostCaptured != 0 || resultReadyEventCount(h.outbox, job.ID) != 0 {
					t.Fatal("blocked result became available or captured")
				}
				account, err := billingRepo.GetAccountByUser(ctx, owner, domain.CurrencyCredits)
				if err != nil {
					t.Fatal(err)
				}
				if account.BalanceCached != billingservice.DefaultStartingBalance {
					t.Fatal("blocked result charged")
				}
				return
			}
			if job.Status != domain.JobStatusResultReady || len(job.OutputArtifactIDs) != 1 || resultReadyEventCount(h.outbox, job.ID) != 1 {
				t.Fatal("result not persisted exactly once")
			}
			artifact, err := h.artRepo.GetByID(ctx, job.OutputArtifactIDs[0])
			if err != nil {
				t.Fatal(err)
			}
			if artifact.Status != domain.ArtifactStatusReady || artifact.MediaType != domain.MediaTypeImage || artifact.StorageKey == "" {
				t.Fatal("missing stored output artifact")
			}
			readiness := resultservice.New(h.jobs, h.artRepo, h.modRepo)
			if err := readiness.RequireCompletionReady(ctx, uuid.New(), job.ID); err == nil {
				t.Fatal("result accessible by wrong owner")
			}
			del := worker.NewDeliveryWorker(worker.DeliveryDeps{Jobs: h.jobs, Artifacts: h.artRepo, Billing: billing, Readiness: readiness})
			for i := 0; i < 2; i++ {
				if err := del.Process(ctx, taskFor(job)); err != nil {
					t.Fatal(err)
				}
			}
			job = h.reload(t, job.ID)
			if job.Status != domain.JobStatusSucceeded || job.CostCaptured != credits {
				t.Fatal("wrong final status or capture")
			}
			account, err := billingRepo.GetAccountByUser(ctx, owner, domain.CurrencyCredits)
			if err != nil {
				t.Fatal(err)
			}
			if account.BalanceCached != billingservice.DefaultStartingBalance-credits {
				t.Fatal("wrong ledger balance after replay")
			}
		})
	}
}
