package worker

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/provider/apimart"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
)

func TestCatalogExpansionHydrationDurableIntentAndAdmission(t *testing.T) {
	for _, model := range []string{apimart.ModelNanoBanana, apimart.ModelGrokImagineVideo, apimart.ModelKling26, apimart.ModelSeedance20, apimart.ModelSeedance20Mini} {
		t.Run(model, func(t *testing.T) {
			ctx := context.Background()
			repo := memory.NewProviderTaskRepo()
			g := &GenerationWorker{processor: processor{tasks: repo, imageSize: "1024x1024", videoResolution: "720p"}}
			params, _ := json.Marshal(map[string]any{"prompt": "A tree in the wind", "provider": "apimart", "model_code": model, "aspect_ratio": "16:9", "duration_sec": 6, "output_count": 1})
			job := &domain.Job{ID: uuid.New(), AccountID: uuid.New(), OperationType: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, Params: params}
			if model == apimart.ModelNanoBanana {
				job.OperationType = domain.OperationImageGenerate
				job.Modality = domain.ModalityImage
			}
			if model == apimart.ModelKling26 {
				params, _ = json.Marshal(map[string]any{"prompt": "A tree in the wind", "provider": "apimart", "model_code": model, "aspect_ratio": "16:9", "duration_sec": 5})
				job.Params = params
			}
			r, err := g.buildRequest(ctx, job, 1)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"test-task"}]}`))
			}))
			defer srv.Close()
			p := apimart.New(apimart.Config{BaseURL: srv.URL + "/v1", HTTPClient: srv.Client()})
			if _, err := NewRegistry(p).ForRequest(ctx, r); err == nil {
				t.Fatal("unverified model admitted")
			}
			if calls != 0 {
				t.Fatal("admission failure contacted provider")
			}
			if _, err := p.Estimate(ctx, r); err != nil {
				t.Fatalf("hydrated request rejected: %v", err)
			}
			intent, err := g.claimPaidSubmit(ctx, &r)
			if err != nil {
				t.Fatal(err)
			}
			if !unresolvedPaidSubmitIntent(intent) || r.AttemptNo != 1 {
				t.Fatal("missing durable claim")
			}
			if _, err := p.Submit(ctx, r); err != nil {
				t.Fatal(err)
			}
			// Simulate loss of the process before the accepted task ID can be saved.
			restarted := &GenerationWorker{processor: processor{tasks: repo}}
			if _, err := restarted.claimPaidSubmit(ctx, &r); !errors.Is(err, domain.ErrConflict) {
				t.Fatalf("restart reclaims paid intent: %v", err)
			}
			if calls != 1 {
				t.Fatalf("paid submit count = %d", calls)
			}
		})
	}
}
