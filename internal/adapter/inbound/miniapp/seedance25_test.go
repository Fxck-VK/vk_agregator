package miniapp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	miniappinbound "vk-ai-aggregator/internal/adapter/inbound/miniapp"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/videorouter"
)

func TestSeedance25MiniAppQuoteAndCreateMatchResolution(t *testing.T) {
	catalog, err := productcatalog.FromConfig(config.Config{FeatureVideoRouterEnabled: true, FeatureAPIMartSeedance25Enabled: true, APIMartProviderEnabled: true, APIMartAPIKey: "test", APIMartBaseURL: "https://example.com"}, mustStaticPricingCatalog())
	if err != nil {
		t.Fatal(err)
	}
	fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
		data, _ := json.Marshal(catalog.VideoRoutes())
		if err := json.Unmarshal(data, &cfg.VideoRoutes); err != nil {
			t.Fatal(err)
		}
		cfg.VideoRouteResolver = joborchestrator.VideoRouteResolverFunc(func(ctx context.Context, in joborchestrator.VideoRouteCheckInput) (joborchestrator.VideoRouteResolution, error) {
			resolved, err := catalog.VideoRouteCatalog.Resolve(ctx, videorouter.Request{Operation: in.Operation, Modality: in.Modality, Params: in.Params, InputArtifactIDs: in.InputArtifactIDs})
			return joborchestrator.VideoRouteResolution{Resolved: resolved.Resolved, Params: resolved.Params, Snapshot: resolved.Snapshot, InternalCostCredits: resolved.InternalCostCredits}, err
		})
	})
	fixture.createVKUserWithCredits(t, 777, 20000)
	for _, tc := range []struct {
		resolution string
		duration   int
		credits    int64
	}{{"480p", 5, 290}, {"720p", 15, 1945}, {"1080p", 30, 6930}} {
		t.Run(fmt.Sprintf("%s/%d", tc.resolution, tc.duration), func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"operation":"video_generate","prompt":"Synthetic scene","video_route_alias":"video_seedance_2_5","video_resolution":"%s","duration_sec":%d}`, tc.resolution, tc.duration))
			for _, path := range []string{"/miniapp/estimate", "/miniapp/jobs", "/miniapp/jobs"} {
				req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
				req.Header.Set("X-Launch-Params", devLaunchParams(777))
				req.Header.Set("X-Idempotency-Key", "seedance-"+tc.resolution)
				resp := httptest.NewRecorder()
				fixture.handler.Routes().ServeHTTP(resp, req)
				if resp.Code != 200 && resp.Code != 201 {
					t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
				}
				var result struct {
					CostEstimate int64 `json:"cost_estimate"`
				}
				if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if result.CostEstimate != tc.credits {
					t.Fatalf("%s credits=%d, want %d", path, result.CostEstimate, tc.credits)
				}
				if path == "/miniapp/jobs" {
					var dto miniappinbound.JobDTO
					if err := json.Unmarshal(resp.Body.Bytes(), &dto); err != nil {
						t.Fatal(err)
					}
					job, err := fixture.jobRepo.GetByID(context.Background(), dto.ID)
					if err != nil {
						t.Fatal(err)
					}
					var params struct {
						Route domain.VideoRouteSnapshot `json:"resolved_video_route"`
					}
					if err := json.Unmarshal(job.Params, &params); err != nil {
						t.Fatal(err)
					}
					if params.Route.Resolution != tc.resolution || job.CostReserved != tc.credits {
						t.Fatal("create lost priced resolution or reservation")
					}
				}
			}
		})
	}
	for _, options := range []string{`"video_resolution":"4k"`, `"video_resolution":"1080p","duration_sec":7`, `"video_resolution":"480p","provider_cost_credits":1`} {
		req := httptest.NewRequest(http.MethodPost, "/miniapp/estimate", bytes.NewBufferString(`{"operation":"video_generate","prompt":"Synthetic scene","video_route_alias":"video_seedance_2_5",`+options+`}`))
		req.Header.Set("X-Launch-Params", devLaunchParams(777))
		resp := httptest.NewRecorder()
		fixture.handler.Routes().ServeHTTP(resp, req)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("invalid public selection accepted: %d", resp.Code)
		}
	}
}
