package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/provider/apimart"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/worker"
)

type interruptedAPIMartImageTasks struct {
	domain.ProviderTaskRepository
	rejectCreate bool
	expire       atomic.Bool
}

func (r *interruptedAPIMartImageTasks) Create(ctx context.Context, task *domain.ProviderTask) error {
	if r.rejectCreate {
		return errors.New("synthetic database failure")
	}
	return r.ProviderTaskRepository.Create(ctx, task)
}
func (r *interruptedAPIMartImageTasks) Update(ctx context.Context, task *domain.ProviderTask) error {
	if task.ExternalID != "" {
		return errors.New("synthetic interruption after provider acceptance")
	}
	return r.ProviderTaskRepository.Update(ctx, task)
}
func (r *interruptedAPIMartImageTasks) ListByJob(ctx context.Context, id uuid.UUID) ([]*domain.ProviderTask, error) {
	tasks, err := r.ProviderTaskRepository.ListByJob(ctx, id)
	if r.expire.Load() {
		for _, task := range tasks {
			task.CreatedAt = time.Now().Add(-24 * time.Hour)
		}
	}
	return tasks, err
}

func TestMidjourneyPersistsSubmitIntentBeforeNetworkAndSurvivesRestart(t *testing.T) {
	testAPIMartImageSubmitRestart(t, "midjourney_v7", "midjourney", "relax", "/v1/midjourney/generations", 30)
}

func TestFlux2PersistsSubmitIntentBeforeNetworkAndSurvivesRestart(t *testing.T) {
	testAPIMartImageSubmitRestart(t, "flux_2_pro", "flux-2-pro", "4MP", "/v1/images/generations", 40)
}

func TestOmniVideoPersistsSubmitIntentBeforeNetworkAndSurvivesRestart(t *testing.T) {
	for _, model := range []string{apimart.ModelOmni11Flash, apimart.ModelOmni11FlashExt} {
		t.Run(model, func(t *testing.T) {
			testAPIMartImageSubmitRestart(t, "", model, "360p", "/v1/videos/generations", 180)
		})
	}
}

func TestKlingVeoSubmitSurvivesRestart(t *testing.T) {
	for _, model := range []string{apimart.ModelKlingV3, apimart.ModelVeo31Fast, apimart.ModelVeo31Quality, apimart.ModelVeo31Lite} {
		t.Run(model, func(t *testing.T) { testAPIMartImageSubmitRestart(t, "", model, "720p", "/v1/videos/generations", 600) })
	}
}

func testAPIMartImageSubmitRestart(t *testing.T, publicModel, providerModel, resolution, endpoint string, credits int64) {
	t.Helper()
	for _, rejectCreate := range []bool{true, false} {
		t.Run(map[bool]string{true: "intent write fails", false: "accepted task persistence interrupted"}[rejectCreate], func(t *testing.T) {
			ctx := context.Background()
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != "POST" || r.URL.Path != endpoint {
					t.Error("unexpected provider request")
				}
				_, _ = w.Write([]byte(`{"code":200,"data":[{"task_id":"accepted-task","status":"submitted"}]}`))
			}))
			defer srv.Close()
			cfg := apimart.Config{APIKey: "test-key", BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()}
			billingRepo := memory.NewBillingRepo()
			billing := billingservice.New(billingRepo)
			var deps worker.Deps
			var tasks *interruptedAPIMartImageTasks
			h := newHarnessWithProvider(t, apimart.New(cfg), func(d *worker.Deps) {
				d.ProviderMediaContracts = providermodels.StaticRegistry().ProviderMediaContracts(providermodels.MediaContractRuntime{})
				if strings.HasPrefix(providerModel, "gemini-omni-") || providermodels.IsKlingVeoVideoRoute(domain.ProviderAPIMart, providerModel) {
					// This recovery fixture has no route snapshot, so the worker uses its runtime resolution.
					d.VideoResolution = resolution
				}
				tasks = &interruptedAPIMartImageTasks{ProviderTaskRepository: d.Tasks, rejectCreate: rejectCreate}
				d.Tasks = tasks
				d.Releaser = billing
				deps = *d
			})
			owner := uuid.New()
			params, _ := json.Marshal(map[string]any{"prompt": "Synthetic scene", "provider": domain.ProviderAPIMart, "model_code": providerModel, "model_id": publicModel, "resolution": resolution, "image_quality": resolution, "aspect_ratio": "16:9", "output_count": 1})
			job := &domain.Job{ID: uuid.New(), AccountID: owner, UserID: owner, Source: "web", ResultMode: domain.ResultModeAccountHistory, ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb}, OperationType: domain.OperationImageGenerate, Modality: domain.ModalityImage, Status: domain.JobStatusQueued, IdempotencyKey: uuid.NewString(), CostEstimate: credits, CostReserved: credits, Params: params}
			if strings.HasPrefix(providerModel, "gemini-omni-") || providermodels.IsKlingVeoVideoRoute(domain.ProviderAPIMart, providerModel) {
				job.OperationType, job.Modality = domain.OperationVideoGenerate, domain.ModalityVideo
				job.Params, _ = json.Marshal(map[string]any{"prompt": "Synthetic scene", "provider": domain.ProviderAPIMart, "model_code": providerModel, "resolution": resolution, "aspect_ratio": "16:9", "duration_sec": func() int {
					if strings.HasPrefix(providerModel, "veo3.1-") {
						return 8
					}
					return 10
				}()})
			}
			if err := billing.Grant(ctx, owner, credits, "restart-test-funding", "test funding"); err != nil {
				t.Fatal(err)
			}
			if err := h.jobs.Create(ctx, job); err != nil {
				t.Fatal(err)
			}
			if _, err := billing.Reserve(ctx, owner, job.ID, credits); err != nil {
				t.Fatal(err)
			}
			if err := h.gen.Process(ctx, taskFor(job)); err == nil {
				after := h.reload(t, job.ID)
				t.Fatalf("expected persistence interruption: calls=%d status=%s class=%s message=%s", calls.Load(), after.Status, after.ErrorCode, after.ErrorMessage)
			}
			if rejectCreate {
				if calls.Load() != 0 {
					t.Fatal("provider called before durable intent")
				}
				return
			}
			if calls.Load() != 1 {
				t.Fatal("expected one accepted request")
			}
			persisted, err := h.tasks.ListByJob(ctx, job.ID)
			if err != nil || len(persisted) != 1 || persisted[0].ExternalID != "" {
				t.Fatal("durable intent missing")
			}
			// A fresh adapter/worker has no in-memory submission cache.
			deps.Providers = worker.NewRegistry(apimart.New(cfg))
			restarted := worker.NewGenerationWorker(deps)
			if err := restarted.Process(ctx, taskFor(job)); err == nil {
				t.Fatal("fresh intent must wait for the submitting worker")
			}
			restartedPoll := worker.NewPollWorker(deps)
			if err := restartedPoll.Process(ctx, taskFor(job)); err == nil {
				t.Fatal("poll worker must not query an unaccepted task")
			}
			tasks.expire.Store(true)
			if err := restartedPoll.Process(ctx, taskFor(job)); err != nil {
				t.Fatal(err)
			}
			if err := restarted.Process(ctx, taskFor(job)); err != nil {
				t.Fatal(err)
			}
			if err := restarted.Process(ctx, taskFor(job)); err != nil {
				t.Fatal(err)
			}
			after := h.reload(t, job.ID)
			account, err := billingRepo.GetAccountByUser(ctx, owner, domain.CurrencyCredits)
			if calls.Load() != 1 || err != nil || after.Status != domain.JobStatusFailedTerminal || after.CostCaptured != 0 || account.BalanceCached != billingservice.DefaultStartingBalance+credits {
				t.Fatal("restart repeated or charged ambiguous submission")
			}
		})
	}
}
