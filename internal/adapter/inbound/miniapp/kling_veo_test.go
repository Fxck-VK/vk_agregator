package miniapp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	miniappinbound "vk-ai-aggregator/internal/adapter/inbound/miniapp"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/videorouter"
)

func newKlingVeoFixture(t *testing.T) *testFixture {
	t.Helper()
	catalog, err := productcatalog.FromConfig(config.Config{FeatureVideoRouterEnabled: true, FeatureAPIMartKlingV3Enabled: true, FeatureAPIMartKling26MotionEnabled: true, FeatureAPIMartVeo31FastEnabled: true, FeatureAPIMartVeo31QualityEnabled: true, FeatureAPIMartVeo31LiteEnabled: true, APIMartProviderEnabled: true, APIMartAPIKey: "test", APIMartBaseURL: "https://example.com"}, mustStaticPricingCatalog())
	if err != nil {
		t.Fatal(err)
	}
	return newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
		data, _ := json.Marshal(catalog.VideoRoutes())
		if err := json.Unmarshal(data, &cfg.VideoRoutes); err != nil {
			t.Fatal(err)
		}
		cfg.VideoRouteResolver = joborchestrator.VideoRouteResolverFunc(func(ctx context.Context, in joborchestrator.VideoRouteCheckInput) (joborchestrator.VideoRouteResolution, error) {
			r, e := catalog.VideoRouteCatalog.Resolve(ctx, videorouter.Request{Operation: in.Operation, Modality: in.Modality, Params: in.Params, InputArtifactIDs: in.InputArtifactIDs})
			return joborchestrator.VideoRouteResolution{Resolved: r.Resolved, Params: r.Params, Snapshot: r.Snapshot, InternalCostCredits: r.InternalCostCredits}, e
		})
	})
}

func TestKlingVeoQuoteCreateAndMotionDurationTrust(t *testing.T) {
	f := newKlingVeoFixture(t)
	ctx := context.Background()
	owner := f.createVKUserWithCredits(t, 777, 20000)
	photo := &domain.Artifact{ID: uuid.New(), OwnerUserID: owner.ID, Kind: domain.ArtifactKindInput, MediaType: domain.MediaTypeImage, MimeType: "image/png", Status: domain.ArtifactStatusReady, StorageBucket: "artifacts", StorageKey: "image", Width: 1280, Height: 720, SizeBytes: 123}
	video := &domain.Artifact{ID: uuid.New(), OwnerUserID: owner.ID, Kind: domain.ArtifactKindInput, MediaType: domain.MediaTypeVideo, MimeType: "video/mp4", Status: domain.ArtifactStatusReady, StorageBucket: "artifacts", StorageKey: "video", Width: 1280, Height: 720, SizeBytes: 1234, DurationMS: 5100, Container: "mp4", Codec: "h264", BitrateBPS: 100000, ProbeStatus: domain.MediaProbePassed}
	for _, a := range []*domain.Artifact{photo, video} {
		if err := f.artifactRepo.Create(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name, alias, res string
		duration         int
		audio, motion    bool
		price            int64
	}{
		{"kling", string(domain.VideoRouteKlingV3), "720p", 5, false, false, 205},
		{"kling audio", string(domain.VideoRouteKlingV3), "720p", 5, true, false, 305},
		{"veo lite", string(domain.VideoRouteVeo31Lite), "1080p", 8, false, false, 45},
		{"veo fast", string(domain.VideoRouteVeo31Fast), "4k", 8, false, false, 385},
		{"veo quality", string(domain.VideoRouteVeo31Quality), "720p", 8, false, false, 600},
		{"motion", string(domain.VideoRouteKling26Motion), "std", 0, false, true, 210},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{"operation": "video_generate", "prompt": "Synthetic scene", "video_route_alias": tc.alias, "video_resolution": tc.res, "duration_sec": tc.duration, "video_audio": tc.audio}
			if tc.motion {
				body["reference_artifact_ids"] = []uuid.UUID{photo.ID}
				body["reference_video_artifact_id"] = video.ID
				body["character_orientation"] = "image"
				body["keep_original_sound"] = false
			}
			request := func(path string) *httptest.ResponseRecorder {
				raw, _ := json.Marshal(body)
				req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
				req.Header.Set("X-Launch-Params", devLaunchParams(777))
				req.Header.Set("X-Idempotency-Key", tc.alias+"-"+strings.ReplaceAll(tc.name, " ", "-"))
				resp := httptest.NewRecorder()
				f.handler.Routes().ServeHTTP(resp, req)
				return resp
			}
			for _, path := range []string{"/miniapp/estimate", "/miniapp/jobs", "/miniapp/jobs"} {
				resp := request(path)
				if resp.Code != 200 && resp.Code != 201 {
					t.Fatalf("status=%d %s", resp.Code, resp.Body.String())
				}
				var dto miniappinbound.JobDTO
				if err := json.Unmarshal(resp.Body.Bytes(), &dto); err != nil {
					t.Fatal(err)
				}
				if dto.CostEstimate != tc.price {
					t.Fatalf("price=%d want%d", dto.CostEstimate, tc.price)
				}
				if path == "/miniapp/jobs" {
					job, e := f.jobRepo.GetByID(ctx, dto.ID)
					if e != nil {
						t.Fatal(e)
					}
					var params struct {
						Route domain.VideoRouteSnapshot `json:"resolved_video_route"`
					}
					_ = json.Unmarshal(job.Params, &params)
					if params.Route.VideoAudio != tc.audio || job.CostReserved != tc.price {
						t.Fatal("options/reservation lost")
					}
					if tc.motion && (params.Route.DurationSec != 6 || params.Route.ReferenceVideoArtifactID != video.ID.String() || params.Route.KeepOriginalSound || len(job.InputArtifactIDs) != 2) {
						t.Fatal("trusted motion fields lost")
					}
				}
			}
			if tc.motion {
				for _, path := range []string{"/miniapp/estimate", "/miniapp/jobs"} {
					body["duration_sec"] = 3
					if resp := request(path); resp.Code != 400 {
						t.Fatal("client underbilling duration accepted")
					}
					body["duration_sec"] = 0
					body["reference_video_artifact_id"] = uuid.New()
					if resp := request(path); resp.Code != 404 {
						t.Fatal("unowned reference accepted")
					}
					body["reference_video_artifact_id"] = video.ID
				}
			}
		})
	}
}
