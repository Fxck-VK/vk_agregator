package miniapp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	miniappinbound "vk-ai-aggregator/internal/adapter/inbound/miniapp"
	"vk-ai-aggregator/internal/domain"
)

func TestRunwayAutomaticResolutionRejectsUnsupportedTariffBeforeJob(t *testing.T) {
	for _, path := range []string{"/miniapp/estimate", "/miniapp/jobs"} {
		t.Run(path, func(t *testing.T) {
			fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
				enableTestVideoRoute(cfg, domain.VideoRouteRunwayGen45)
				cfg.VideoRoutes[0].AllowedResolutions = []string{"720p", "1080p"}
			})
			fixture.createVKUserWithCredits(t, 777, 100000)
			body, _ := json.Marshal(map[string]any{"operation": "video_generate", "prompt": "Synthetic video", "video_route_alias": string(domain.VideoRouteRunwayGen45), "video_resolution": "1080p", "duration_sec": 5})
			req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Launch-Params", devLaunchParams(777))
			req.Header.Set("X-Idempotency-Key", "runway-automatic-resolution")
			rec := httptest.NewRecorder()
			fixture.handler.Routes().ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest || !bytes.Contains(rec.Body.Bytes(), []byte("invalid video resolution")) {
				t.Fatalf("unsupported resolution: status %d body %s", rec.Code, rec.Body.String())
			}
			jobs, err := fixture.jobRepo.List(context.Background(), domain.JobFilter{}, 10, 0)
			if err != nil || len(jobs) != 0 {
				t.Fatalf("unsupported resolution created jobs: count %d error %v", len(jobs), err)
			}
		})
	}
}

func TestRunwayCatalogDescribesAutomaticResolution(t *testing.T) {
	fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
		enableTestVideoRoute(cfg, domain.VideoRouteRunwayGen45)
		cfg.VideoRoutes[0].AllowedResolutions = []string{"720p", "1080p"}
	})
	req := httptest.NewRequest(http.MethodGet, "/miniapp/model-catalog", nil)
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	rec := httptest.NewRecorder()
	fixture.handler.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("catalog: %d", rec.Code)
	}
	var catalog struct {
		Items []struct {
			ID          string   `json:"id"`
			Automatic   bool     `json:"automatic_resolution"`
			Resolutions []string `json:"allowed_resolutions"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &catalog); err != nil {
		t.Fatal(err)
	}
	for _, item := range catalog.Items {
		if item.ID == string(domain.VideoRouteRunwayGen45) {
			if !item.Automatic || len(item.Resolutions) != 1 || item.Resolutions[0] != "720p" {
				t.Fatalf("automatic catalog: %+v", item)
			}
			return
		}
	}
	t.Fatal("Runway missing from catalog")
}
