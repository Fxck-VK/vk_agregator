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
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/resultservice"
	"vk-ai-aggregator/internal/service/videorouter"
	"vk-ai-aggregator/internal/worker"
)

func TestSeedance25AsyncLifecycle(t *testing.T) {
	testAPIMartVideoLifecycle(t, domain.VideoRouteSeedance25, "seedance-2.5", "480p", 5, 290)
}

func TestOmniVideoAsyncLifecycle(t *testing.T) {
	t.Run("automatic", func(t *testing.T) {
		testAPIMartVideoLifecycle(t, domain.VideoRouteOmni11Flash, apimart.ModelOmni11Flash, "1080p", 10, 795)
	})
	t.Run("EXT", func(t *testing.T) {
		testAPIMartVideoLifecycle(t, domain.VideoRouteOmni11FlashExt, apimart.ModelOmni11FlashExt, "4k", 8, 510)
	})
}

func TestKlingVeoAsyncLifecycle(t *testing.T) {
	for _, tc := range []struct {
		alias      domain.VideoRouteAlias
		model, res string
		duration   int
		credits    int64
	}{
		{domain.VideoRouteKlingV3, apimart.ModelKlingV3, "720p", 5, 205},
		{domain.VideoRouteVeo31Fast, apimart.ModelVeo31Fast, "4k", 8, 385},
		{domain.VideoRouteVeo31Quality, apimart.ModelVeo31Quality, "1080p", 8, 600},
		{domain.VideoRouteVeo31Lite, apimart.ModelVeo31Lite, "720p", 8, 45},
	} {
		t.Run(tc.model, func(t *testing.T) { testAPIMartVideoLifecycle(t, tc.alias, tc.model, tc.res, tc.duration, tc.credits) })
	}
}

func testAPIMartVideoLifecycle(t *testing.T, alias domain.VideoRouteAlias, model, resolution string, duration int, credits int64) {
	t.Helper()
	for _, scenario := range []string{"success", "output blocked", "provider rejected", "submit indeterminate"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			var submits atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && r.URL.Path == "/videos/generations" {
					submits.Add(1)
					if scenario == "submit indeterminate" {
						w.WriteHeader(502)
						return
					}
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if body["model"] != model || (model != apimart.ModelKlingV3 && body["resolution"] != resolution) || (model == apimart.ModelKlingV3 && body["mode"] != "std") {
						t.Error("worker ignored immutable route")
					}
					if model == apimart.ModelOmni11Flash {
						if _, ok := body["duration"]; ok {
							t.Error("automatic duration sent upstream")
						}
					} else if body["duration"] != float64(duration) {
						t.Error("incorrect duration")
					}
					_, _ = w.Write([]byte(`{"code":200,"data":[{"task_id":"video-task","status":"submitted"}]}`))
					return
				}
				if r.Method == http.MethodGet && r.URL.Path == "/tasks/video-task" {
					if scenario == "provider rejected" {
						_, _ = w.Write([]byte(`{"code":200,"data":{"status":"failed","error":{"code":"content_rejected"}}}`))
						return
					}
					_, _ = w.Write([]byte(`{"code":200,"data":{"status":"completed","result":{"videos":[{"url":["https://example.com/video.mp4"]}]}}}`))
					return
				}
				t.Error("unexpected HTTP request")
				w.WriteHeader(400)
			}))
			defer srv.Close()
			provider := apimart.New(apimart.Config{BaseURL: srv.URL, APIKey: "test", HTTPClient: srv.Client()})
			ledger := memory.NewBillingRepo()
			billing := billingservice.New(ledger)
			moderator := &qwenOutputModerator{blocked: scenario == "output blocked"}
			h := newHarnessWithProvider(t, provider, func(d *worker.Deps) {
				d.Releaser = billing
				d.Moderator = moderator
				d.ProviderMediaContracts = providermodels.StaticRegistry().ProviderMediaContracts(providermodels.MediaContractRuntime{})
			})
			prices, err := pricingcatalog.NewStaticCatalog()
			if err != nil {
				t.Fatal(err)
			}
			catalog, err := productcatalog.FromConfig(config.Config{FeatureVideoRouterEnabled: true, FeatureAPIMartSeedance25Enabled: true, FeatureAPIMartOmni11FlashEnabled: true, FeatureAPIMartOmni11FlashExtEnabled: true, FeatureAPIMartKlingV3Enabled: true, FeatureAPIMartVeo31FastEnabled: true, FeatureAPIMartVeo31QualityEnabled: true, FeatureAPIMartVeo31LiteEnabled: true, APIMartProviderEnabled: true, APIMartAPIKey: "test", APIMartBaseURL: srv.URL}, prices)
			if err != nil {
				t.Fatal(err)
			}
			params, _ := json.Marshal(map[string]any{"prompt": "Synthetic video test", "video_route_alias": alias, "duration_sec": duration, "resolution": resolution})
			resolved, err := catalog.VideoRouteCatalog.Resolve(ctx, videorouter.Request{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, Params: params})
			if err != nil {
				t.Fatal(err)
			}
			price, err := prices.Snapshot(pricingcatalog.ProductKey{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, VideoRouteAlias: alias, Resolution: resolution, DurationSec: duration})
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(price)
			if err != nil {
				t.Fatal(err)
			}
			owner := uuid.New()
			if err := billing.Grant(ctx, owner, 1000, "seedance-fixture-grant", "test funding"); err != nil {
				t.Fatal(err)
			}
			job := &domain.Job{ID: uuid.New(), UserID: owner, AccountID: owner, Source: "web", ResultMode: domain.ResultModeAccountHistory, ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb}, OperationType: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, Status: domain.JobStatusQueued, IdempotencyKey: uuid.NewString(), CostEstimate: credits, CostReserved: credits, PricingSnapshot: raw, Params: resolved.Params}
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
				t.Fatalf("submits = %d", submits.Load())
			}
			if scenario != "submit indeterminate" {
				polls := h.streams.byStream[redisqueue.StreamProviderPoll]
				if len(polls) != 1 {
					t.Fatal("missing persisted polling task")
				}
				for i := 0; i < 2; i++ {
					if err := h.poll.Process(ctx, polls[0]); err != nil {
						t.Fatal(err)
					}
				}
			}
			job = h.reload(t, job.ID)
			if scenario != "success" {
				wantStatus := domain.JobStatusFailedTerminal
				if scenario == "output blocked" {
					wantStatus = domain.JobStatusRejected
				}
				if job.Status != wantStatus || job.CostCaptured != 0 || resultReadyEventCount(h.outbox, job.ID) != 0 {
					t.Fatalf("failed/blocked status=%s (want %s), captured=%d, visible events=%d", job.Status, wantStatus, job.CostCaptured, resultReadyEventCount(h.outbox, job.ID))
				}
				account, err := ledger.GetAccountByUser(ctx, owner, domain.CurrencyCredits)
				if err != nil || account.BalanceCached != billingservice.DefaultStartingBalance+1000 {
					t.Fatal("reservation not released")
				}
				return
			}
			if job.Status != domain.JobStatusResultReady || len(job.OutputArtifactIDs) != 1 || moderator.calls != 1 {
				t.Fatal("output missing or moderation bypassed")
			}
			artifact, err := h.artRepo.GetByID(ctx, job.OutputArtifactIDs[0])
			if err != nil {
				t.Fatal(err)
			}
			if artifact.MediaType != domain.MediaTypeVideo || artifact.Status != domain.ArtifactStatusReady || artifact.StorageKey == "" {
				t.Fatal("video artifact not stored")
			}
			readiness := resultservice.New(h.jobs, h.artRepo, h.modRepo)
			if err := readiness.RequireCompletionReady(ctx, uuid.New(), job.ID); err == nil {
				t.Fatal("owner check bypassed")
			}
			delivery := worker.NewDeliveryWorker(worker.DeliveryDeps{Jobs: h.jobs, Artifacts: h.artRepo, Billing: billing, Readiness: readiness})
			for i := 0; i < 2; i++ {
				if err := delivery.Process(ctx, taskFor(job)); err != nil {
					t.Fatal(err)
				}
			}
			job = h.reload(t, job.ID)
			account, err := ledger.GetAccountByUser(ctx, owner, domain.CurrencyCredits)
			if err != nil || job.Status != domain.JobStatusSucceeded || job.CostCaptured != credits || account.BalanceCached != billingservice.DefaultStartingBalance+1000-credits {
				t.Fatal("incorrect final capture or replay changed balance")
			}
		})
	}
}
