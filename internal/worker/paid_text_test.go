package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"vk-ai-aggregator/internal/adapter/provider/apimart"
	"vk-ai-aggregator/internal/adapter/provider/kie"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/dialogcontext"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/resultservice"
	"vk-ai-aggregator/internal/service/textgeneration"
	"vk-ai-aggregator/internal/worker"
)

type interruptedTextCheckpoint struct {
	domain.ProviderTaskRepository
	interrupt bool
}

func (r *interruptedTextCheckpoint) Update(ctx context.Context, task *domain.ProviderTask) error {
	if r.interrupt && task.ExternalID != "" {
		r.interrupt = false
		return errors.New("synthetic checkpoint interruption")
	}
	return r.ProviderTaskRepository.Update(ctx, task)
}

func TestPaidTextRestartUsesArtifactWithoutSecondProviderCall(t *testing.T) {
	for _, model := range providermodels.PaidTextModels() {
		t.Run(model.PublicID, func(t *testing.T) {
			ctx := context.Background()
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var body struct {
					MaxOutputTokens int `json:"max_output_tokens"`
					MaxTokens       int `json:"max_tokens"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.MaxOutputTokens+body.MaxTokens != 2048 {
					t.Error("paid snapshot output cap replaced by default chat limit")
				}
				response := `{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Synthetic answer"}]}],"usage":{"input_tokens":20,"output_tokens":8}}`
				if strings.Contains(r.URL.Path, "messages") {
					response = `{"stop_reason":"end_turn","content":[{"type":"text","text":"Synthetic answer"}],"usage":{"input_tokens":20,"output_tokens":8}}`
				}
				if strings.Contains(r.URL.Path, "chat/completions") {
					response = `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"Synthetic answer"}}],"usage":{"prompt_tokens":20,"completion_tokens":8}}`
				}
				_, _ = w.Write([]byte(response))
			}))
			defer srv.Close()
			billingRepo := memory.NewBillingRepo()
			billing := billingservice.New(billingRepo, billingservice.WithStartingBalance(1000))
			provider := func() domain.Provider {
				if model.Provider == domain.ProviderAPIMart {
					return apimart.New(apimart.Config{APIKey: "test-key", BaseURL: srv.URL, EnabledTextModels: []string{model.ProviderModelID}})
				}
				return kie.New(kie.Config{APIKey: "test-key", BaseURL: srv.URL, EnabledModels: []string{model.ProviderModelID}})
			}
			var deps worker.Deps
			h := newHarnessWithProvider(t, provider(), func(d *worker.Deps) {
				d.TextContext = dialogcontext.New(memory.NewConversationRepo(), dialogcontext.Config{Enabled: false, MaxOutputTokens: 7})
				d.Tasks = &interruptedTextCheckpoint{ProviderTaskRepository: d.Tasks, interrupt: true}
				deps = *d
			})
			prices, _ := pricingcatalog.NewStaticCatalog()
			snapshot, _ := prices.Snapshot(textgeneration.Key(model.PublicID))
			raw, _ := json.Marshal(snapshot)
			owner := uuid.New()
			params, _ := json.Marshal(map[string]string{"prompt": "Synthetic question", "provider": string(model.Provider), "model_id": model.PublicID, "model_code": model.ProviderModelID})
			job := &domain.Job{ID: uuid.New(), UserID: owner, AccountID: owner, Source: "web", ResultMode: domain.ResultModeAccountHistory, ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb}, OperationType: domain.OperationTextGenerate, Modality: domain.ModalityText, Status: domain.JobStatusQueued, IdempotencyKey: uuid.NewString(), CostReserved: snapshot.InternalCredits, CostEstimate: snapshot.InternalCredits, PricingSnapshot: raw, Params: params}
			if err := h.jobs.Create(ctx, job); err != nil {
				t.Fatal(err)
			}
			if _, err := billing.Reserve(ctx, owner, job.ID, snapshot.InternalCredits); err != nil {
				t.Fatal(err)
			}
			if err := h.gen.Process(ctx, taskFor(job)); err == nil {
				t.Fatal("checkpoint failure was not injected")
			}
			saved := h.reload(t, job.ID)
			if len(saved.OutputArtifactIDs) != 1 || calls != 1 {
				t.Fatal("text not durable before checkpoint")
			}
			deps.Providers = worker.NewRegistry(provider())
			restarted := worker.NewGenerationWorker(deps)
			if err := restarted.Process(ctx, taskFor(job)); err != nil {
				t.Fatal(err)
			}
			if err := restarted.Process(ctx, taskFor(job)); err != nil {
				t.Fatal(err)
			}
			saved = h.reload(t, job.ID)
			if calls != 1 || len(saved.OutputArtifactIDs) != 1 || saved.Status != domain.JobStatusResultReady {
				t.Fatalf("recovery failed: calls=%d status=%s", calls, saved.Status)
			}
			delivery := worker.NewDeliveryWorker(worker.DeliveryDeps{Jobs: h.jobs, Deliveries: memory.NewDeliveryRepo(), Artifacts: h.artRepo, Billing: billing, Readiness: resultservice.New(h.jobs, h.artRepo, h.modRepo)})
			if err := delivery.Process(ctx, deliveryTask(saved)); err != nil {
				t.Fatal(err)
			}
			if err := delivery.Process(ctx, deliveryTask(saved)); err != nil {
				t.Fatal(err)
			}
			saved = h.reload(t, job.ID)
			account, err := billingRepo.GetAccountByUser(ctx, owner, domain.CurrencyCredits)
			if err != nil || saved.Status != domain.JobStatusSucceeded || saved.CostCaptured != snapshot.InternalCredits || account.BalanceCached != 1000-snapshot.InternalCredits {
				t.Fatal("capture was not exact and idempotent")
			}
			tasks, _ := h.tasks.ListByJob(ctx, job.ID)
			for _, task := range tasks {
				var result map[string]any
				_ = json.Unmarshal(task.Result, &result)
				if _, ok := result["text"]; ok {
					t.Fatal("inline output leaked")
				}
			}
		})
	}
}

func TestPaidTextMissingReserveNeverCallsProvider(t *testing.T) {
	for _, model := range providermodels.PaidTextModels() {
		t.Run(model.PublicID, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
			defer srv.Close()
			var provider domain.Provider
			if model.Provider == domain.ProviderAPIMart {
				provider = apimart.New(apimart.Config{APIKey: "synthetic", BaseURL: srv.URL, EnabledTextModels: []string{model.ProviderModelID}})
			} else {
				provider = kie.New(kie.Config{APIKey: "synthetic", BaseURL: srv.URL, EnabledModels: []string{model.ProviderModelID}})
			}
			h := newHarnessWithProvider(t, provider, nil)
			params, _ := json.Marshal(map[string]string{"prompt": "Synthetic", "provider": string(model.Provider), "model_id": model.PublicID, "model_code": model.ProviderModelID})
			job := &domain.Job{ID: uuid.New(), UserID: uuid.New(), OperationType: domain.OperationTextGenerate, Modality: domain.ModalityText, Status: domain.JobStatusQueued, IdempotencyKey: uuid.NewString(), Params: params}
			if err := h.jobs.Create(context.Background(), job); err != nil {
				t.Fatal(err)
			}
			if err := h.gen.Process(context.Background(), taskFor(job)); err != nil {
				t.Fatal(err)
			}
			if calls != 0 {
				t.Fatal("provider called without price/reservation")
			}
		})
	}
}

func TestPaidTextAmbiguousResponseReleasesReserveWithoutRetry(t *testing.T) {
	for _, id := range []string{"gpt_6_astra", "claude_fable_5_1"} {
		t.Run(id, func(t *testing.T) {
			model, _ := providermodels.PaidTextModel(id)
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer srv.Close()
			var provider domain.Provider
			if model.Provider == domain.ProviderAPIMart {
				provider = apimart.New(apimart.Config{APIKey: "synthetic", BaseURL: srv.URL, EnabledTextModels: []string{model.ProviderModelID}})
			} else {
				provider = kie.New(kie.Config{APIKey: "synthetic", BaseURL: srv.URL, EnabledModels: []string{model.ProviderModelID}})
			}
			billingRepo := memory.NewBillingRepo()
			billing := billingservice.New(billingRepo, billingservice.WithStartingBalance(1000))
			h := newHarnessWithProvider(t, provider, func(d *worker.Deps) { d.Releaser = billing })
			prices, _ := pricingcatalog.NewStaticCatalog()
			snapshot, _ := prices.Snapshot(textgeneration.Key(id))
			raw, _ := json.Marshal(snapshot)
			params, _ := json.Marshal(map[string]string{"prompt": "Synthetic", "provider": string(model.Provider), "model_id": id, "model_code": model.ProviderModelID})
			owner := uuid.New()
			job := &domain.Job{ID: uuid.New(), UserID: owner, AccountID: owner, OperationType: domain.OperationTextGenerate, Modality: domain.ModalityText, Status: domain.JobStatusQueued, IdempotencyKey: uuid.NewString(), Params: params, PricingSnapshot: raw, CostEstimate: snapshot.InternalCredits, CostReserved: snapshot.InternalCredits}
			ctx := context.Background()
			if err := h.jobs.Create(ctx, job); err != nil {
				t.Fatal(err)
			}
			if _, err := billing.Reserve(ctx, owner, job.ID, snapshot.InternalCredits); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 3; i++ {
				if err := h.gen.Process(ctx, taskFor(job)); err != nil {
					t.Fatal(err)
				}
			}
			saved := h.reload(t, job.ID)
			account, err := billingRepo.GetAccountByUser(ctx, owner, domain.CurrencyCredits)
			if err != nil || calls != 1 || saved.Status != domain.JobStatusFailedTerminal || saved.CostCaptured != 0 || account.BalanceCached != 1000 {
				t.Fatal("ambiguous response retried or user charged")
			}
		})
	}
}
