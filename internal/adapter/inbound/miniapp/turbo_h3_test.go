package miniapp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	miniappinbound "vk-ai-aggregator/internal/adapter/inbound/miniapp"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/videorouter"
)

func TestTurboH3MiniAppQuoteCreateAndReplayPersistRoute(t *testing.T) {
	fixture := newTurboH3Fixture(t)
	ctx := context.Background()
	owner := fixture.createVKUserWithCredits(t, 777, 30000)
	firstFrame := &domain.Artifact{
		ID:            uuid.New(),
		OwnerUserID:   owner.ID,
		Kind:          domain.ArtifactKindInput,
		MediaType:     domain.MediaTypeImage,
		MimeType:      "image/png",
		Status:        domain.ArtifactStatusReady,
		StorageBucket: "artifacts",
		StorageKey:    "first-frame",
		Width:         1280,
		Height:        720,
		SizeBytes:     1234,
	}
	if err := fixture.artifactRepo.Create(ctx, firstFrame); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		alias               domain.VideoRouteAlias
		resolution          string
		price               int64
		providerModelID     string
		providerCostCredits int64
		useFirstFrame       bool
	}{
		{domain.VideoRouteKling30Turbo, "720p", 345, "kling-3.0-turbo", 6, false},
		{domain.VideoRouteKling30Turbo, "1080p", 430, "kling-3.0-turbo", 8, true},
		{domain.VideoRouteMiniMaxH3, "768p", 175, "MiniMax-H3", 3, false},
		{domain.VideoRouteMiniMaxH3, "2k", 275, "MiniMax-H3", 5, true},
	} {
		t.Run(fmt.Sprintf("%s/%s", tc.alias, tc.resolution), func(t *testing.T) {
			body := map[string]any{
				"operation":         "video_generate",
				"prompt":            "Synthetic scene",
				"video_route_alias": tc.alias,
				"video_resolution":  tc.resolution,
				"duration_sec":      5,
			}
			if tc.useFirstFrame {
				body["reference_artifact_ids"] = []uuid.UUID{firstFrame.ID}
			}
			estimate := postTurboH3MiniApp(t, fixture, "/miniapp/estimate", body, string(tc.alias)+"-"+tc.resolution+"-estimate")
			if estimate.Code != http.StatusOK {
				t.Fatalf("estimate status=%d body=%s", estimate.Code, estimate.Body.String())
			}
			var estimateDTO miniappinbound.JobDTO
			if err := json.Unmarshal(estimate.Body.Bytes(), &estimateDTO); err != nil {
				t.Fatal(err)
			}
			if estimateDTO.CostEstimate != tc.price {
				t.Fatalf("estimate=%d want %d", estimateDTO.CostEstimate, tc.price)
			}

			key := string(tc.alias) + "-" + tc.resolution + "-job"
			first := postTurboH3MiniApp(t, fixture, "/miniapp/jobs", body, key)
			second := postTurboH3MiniApp(t, fixture, "/miniapp/jobs", body, key)
			if first.Code != http.StatusCreated || (second.Code != http.StatusOK && second.Code != http.StatusCreated) {
				t.Fatalf("create/replay statuses=%d/%d bodies=%q/%q", first.Code, second.Code, first.Body.String(), second.Body.String())
			}
			var firstDTO, secondDTO miniappinbound.JobDTO
			if err := json.Unmarshal(first.Body.Bytes(), &firstDTO); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(second.Body.Bytes(), &secondDTO); err != nil {
				t.Fatal(err)
			}
			if firstDTO.ID != secondDTO.ID || secondDTO.CostEstimate != tc.price {
				t.Fatalf("idempotent replay mismatch: first=%+v second=%+v", firstDTO, secondDTO)
			}
			job, err := fixture.jobRepo.GetByID(ctx, firstDTO.ID)
			if err != nil {
				t.Fatal(err)
			}
			if job.CostReserved != tc.price {
				t.Fatalf("reserved=%d want %d", job.CostReserved, tc.price)
			}
			if tc.useFirstFrame && len(job.InputArtifactIDs) != 1 {
				t.Fatalf("first-frame artifact not attached: %+v", job.InputArtifactIDs)
			}
			var params struct {
				VideoRouteAlias string                    `json:"video_route_alias"`
				Resolution      string                    `json:"resolution"`
				DurationSec     int                       `json:"duration_sec"`
				Route           domain.VideoRouteSnapshot `json:"resolved_video_route"`
			}
			if err := json.Unmarshal(job.Params, &params); err != nil {
				t.Fatal(err)
			}
			if params.VideoRouteAlias != string(tc.alias) ||
				params.Resolution != tc.resolution ||
				params.DurationSec != 5 ||
				params.Route.Alias != tc.alias ||
				params.Route.Provider != domain.ProviderAPIMart ||
				params.Route.ProviderModelID != tc.providerModelID ||
				params.Route.Resolution != tc.resolution ||
				params.Route.DurationSec != 5 ||
				params.Route.InternalCostCredits != tc.price ||
				params.Route.ProviderCostCredits != tc.providerCostCredits {
				t.Fatalf("persisted route mismatch: %+v", params)
			}
		})
	}
}

func TestTurboH3MiniAppRejectsClosedInputs(t *testing.T) {
	fixture := newTurboH3Fixture(t)
	ctx := context.Background()
	owner := fixture.createVKUserWithCredits(t, 777, 30000)
	other := fixture.createVKUserWithCredits(t, 888, 30000)
	image := func(ownerID uuid.UUID) uuid.UUID {
		id := uuid.New()
		err := fixture.artifactRepo.Create(ctx, &domain.Artifact{
			ID:            id,
			OwnerUserID:   ownerID,
			Kind:          domain.ArtifactKindInput,
			MediaType:     domain.MediaTypeImage,
			MimeType:      "image/png",
			Status:        domain.ArtifactStatusReady,
			StorageBucket: "artifacts",
			StorageKey:    id.String(),
			Width:         1280,
			Height:        720,
			SizeBytes:     1234,
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	ownedOne := image(owner.ID)
	ownedTwo := image(owner.ID)
	unowned := image(other.ID)
	for _, tc := range []struct {
		alias       domain.VideoRouteAlias
		resolution  string
		minDuration int
		maxPrompt   int
	}{
		{domain.VideoRouteKling30Turbo, "720p", 3, 3072},
		{domain.VideoRouteMiniMaxH3, "2k", 4, 7000},
	} {
		t.Run(string(tc.alias), func(t *testing.T) {
			valid := map[string]any{
				"operation":         "video_generate",
				"prompt":            "Synthetic scene",
				"video_route_alias": tc.alias,
				"video_resolution":  tc.resolution,
				"duration_sec":      tc.minDuration,
			}
			cases := []struct {
				name string
				body map[string]any
				want int
			}{
				{"duration below min", withTurboH3Override(valid, "duration_sec", tc.minDuration-1), http.StatusBadRequest},
				{"duration above max", withTurboH3Override(valid, "duration_sec", 16), http.StatusBadRequest},
				{"invalid resolution", withTurboH3Override(valid, "video_resolution", "4k"), http.StatusBadRequest},
				{"audio closed", withTurboH3Override(valid, "video_audio", true), http.StatusBadRequest},
				{"too many first-frame refs", withTurboH3Override(valid, "reference_artifact_ids", []uuid.UUID{ownedOne, ownedTwo}), http.StatusBadRequest},
				{"unowned first-frame", withTurboH3Override(valid, "reference_artifact_ids", []uuid.UUID{unowned}), http.StatusNotFound},
				{"prompt cap", withTurboH3Override(valid, "prompt", makeTurboH3Prompt(tc.maxPrompt+1)), http.StatusBadRequest},
			}
			for _, entry := range cases {
				t.Run(entry.name, func(t *testing.T) {
					resp := postTurboH3MiniApp(t, fixture, "/miniapp/estimate", entry.body, string(tc.alias)+"-"+entry.name)
					if resp.Code != entry.want {
						t.Fatalf("status=%d want %d body=%s", resp.Code, entry.want, resp.Body.String())
					}
				})
			}
			nativeJSON := []byte(fmt.Sprintf(`{"operation":"video_generate","prompt":"Synthetic scene","video_route_alias":"%s","video_resolution":"%s","duration_sec":5,"first_frame_image":"https://example.com/first.png"}`, tc.alias, tc.resolution))
			req := httptest.NewRequest(http.MethodPost, "/miniapp/estimate", bytes.NewReader(nativeJSON))
			req.Header.Set("X-Launch-Params", devLaunchParams(777))
			resp := httptest.NewRecorder()
			fixture.handler.Routes().ServeHTTP(resp, req)
			if resp.Code != http.StatusBadRequest {
				t.Fatalf("native first_frame_image accepted: status=%d body=%s", resp.Code, resp.Body.String())
			}
		})
	}
}

func newTurboH3Fixture(t *testing.T) *testFixture {
	t.Helper()
	catalog, err := productcatalog.FromConfig(config.Config{
		FeatureVideoRouterEnabled:         true,
		FeatureAPIMartKling30TurboEnabled: true,
		FeatureAPIMartMiniMaxH3Enabled:    true,
		APIMartProviderEnabled:            true,
		APIMartAPIKey:                     "test",
		APIMartBaseURL:                    "https://example.com",
	}, mustStaticPricingCatalog())
	if err != nil {
		t.Fatal(err)
	}
	return newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
		data, _ := json.Marshal(catalog.VideoRoutes())
		if err := json.Unmarshal(data, &cfg.VideoRoutes); err != nil {
			t.Fatal(err)
		}
		cfg.VideoRouteResolver = joborchestrator.VideoRouteResolverFunc(func(ctx context.Context, in joborchestrator.VideoRouteCheckInput) (joborchestrator.VideoRouteResolution, error) {
			resolved, err := catalog.VideoRouteCatalog.Resolve(ctx, videorouter.Request{
				Operation:        in.Operation,
				Modality:         in.Modality,
				Params:           in.Params,
				InputArtifactIDs: in.InputArtifactIDs,
			})
			return joborchestrator.VideoRouteResolution{
				Resolved:            resolved.Resolved,
				Params:              resolved.Params,
				Snapshot:            resolved.Snapshot,
				InternalCostCredits: resolved.InternalCostCredits,
			}, err
		})
	})
}

func TestTurboH3MiniAppKlingFirstFrameWithoutPrompt(t *testing.T) {
	fixture := newTurboH3Fixture(t)
	owner := fixture.createVKUserWithCredits(t, 777, 3000)
	frame := &domain.Artifact{ID: uuid.New(), OwnerUserID: owner.ID, Kind: domain.ArtifactKindInput, MediaType: domain.MediaTypeImage, MimeType: "image/png", Status: domain.ArtifactStatusReady, StorageBucket: "artifacts", StorageKey: "first-frame", Width: 1280, Height: 720, SizeBytes: 1234}
	if err := fixture.artifactRepo.Create(context.Background(), frame); err != nil {
		t.Fatal(err)
	}
	for _, alias := range []domain.VideoRouteAlias{domain.VideoRouteKling30Turbo, domain.VideoRouteMiniMaxH3} {
		for _, image := range []bool{false, true} {
			body := map[string]any{"operation": "video_generate", "video_route_alias": alias}
			if image {
				body["reference_artifact_ids"] = []uuid.UUID{frame.ID}
			}
			for _, path := range []string{"/miniapp/estimate", "/miniapp/jobs"} {
				resp := postTurboH3MiniApp(t, fixture, path, body, "first-frame-only")
				valid := image && alias == domain.VideoRouteKling30Turbo
				if valid {
					if resp.Code != 200 && resp.Code != 201 {
						t.Fatalf("first-frame-only request failed: %d %s", resp.Code, resp.Body.String())
					}
				} else if resp.Code != http.StatusBadRequest {
					t.Fatalf("empty prompt accepted for%s image%v: %d", alias, image, resp.Code)
				}
			}
		}
	}
}

func postTurboH3MiniApp(t *testing.T, fixture *testFixture, path string, body map[string]any, key string) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("X-Launch-Params", devLaunchParams(777))
	if path == "/miniapp/jobs" {
		req.Header.Set("X-Idempotency-Key", key)
	}
	resp := httptest.NewRecorder()
	fixture.handler.Routes().ServeHTTP(resp, req)
	return resp
}

func withTurboH3Override(base map[string]any, key string, value any) map[string]any {
	out := make(map[string]any, len(base)+1)
	for k, v := range base {
		out[k] = v
	}
	out[key] = value
	return out
}

func makeTurboH3Prompt(chars int) string {
	buf := make([]rune, chars)
	for i := range buf {
		buf[i] = 'x'
	}
	return string(buf)
}
