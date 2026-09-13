package videorouter_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/videorouter"
)

func TestTurboH3PublicRoutesAndRequestBounds(t *testing.T) {
	for _, tc := range []struct {
		alias              domain.VideoRouteAlias
		model, resolution  string
		minimum, promptCap int
	}{
		{"video_kling_3_0_turbo", "kling-3.0-turbo", "720p", 3, 3072},
		{"video_minimax_h3", "MiniMax-H3", "2k", 4, 7000},
	} {
		t.Run(tc.model, func(t *testing.T) {
			catalog := newConfiguredCatalog(t, map[domain.VideoRouteAlias]bool{tc.alias: true})
			route := publicRouteByAlias(catalog.PublicRoutes(), tc.alias)
			if route == nil || route.DefaultDurationSec != 5 || route.DefaultResolution != tc.resolution || route.MaxReferenceImages != 1 || !route.SupportsReferenceImage || route.SupportsAudio || route.RequiresReferenceVideo {
				t.Fatalf("missing or incorrect route: %+v", route)
			}
			for n := tc.minimum; n <= 15; n++ {
				if !containsInt(route.AllowedDurationsSec, n) {
					t.Fatalf("missing duration %d", n)
				}
			}
			raw, _ := json.Marshal(route)
			for _, private := range []string{"provider", "model_id", tc.model, "cost"} {
				if strings.Contains(string(raw), private) {
					t.Fatalf("private field exposed: %s", private)
				}
			}
			for _, count := range []int{0, 1, 2} {
				ids := make([]uuid.UUID, count)
				for i := range ids {
					ids[i] = uuid.New()
				}
				for _, res := range route.AllowedResolutions {
					for _, duration := range []int{tc.minimum - 1, tc.minimum, 5, 15, 16} {
						params, _ := json.Marshal(map[string]any{"video_route_alias": tc.alias, "prompt": "Synthetic scene", "duration_sec": duration, "resolution": res})
						resolved, err := catalog.Resolve(context.Background(), videorouter.Request{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, Params: params, InputArtifactIDs: ids})
						valid := duration >= tc.minimum && duration <= 15 && count <= 1
						if valid != (err == nil) {
							t.Fatalf("res=%s duration=%d refs=%d valid=%v error=%v", res, duration, count, valid, err)
						}
						if valid && (resolved.Snapshot.Provider != domain.ProviderAPIMart || resolved.Snapshot.ProviderModelID != tc.model) {
							t.Fatal("incorrect worker route")
						}
					}
				}
			}
			for _, options := range []map[string]any{{"video_audio": true}, {"resolution": "4k"}, {"provider": "apimart"}, {"first_frame_image": "https://example.com/a.png"}, {"last_frame_image": "https://example.com/a.png"}, {"image_urls": []string{"https://example.com/a.png"}}, {"image_with_roles": []any{}}, {"callback_url": "https://example.com/hook"}, {"audio_urls": []string{"https://example.com/a.mp3"}}, {"prompt": strings.Repeat("я", tc.promptCap+1)}} {
				options["video_route_alias"] = tc.alias
				if _, ok := options["prompt"]; !ok {
					options["prompt"] = "Synthetic scene"
				}
				params, _ := json.Marshal(options)
				if _, err := catalog.Resolve(context.Background(), videorouter.Request{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, Params: params}); err == nil {
					t.Fatal("unsupported input accepted")
				}
			}
		})
	}
}

func TestTurboH3PromptOptionalOnlyForKlingFirstFrame(t *testing.T) {
	for _, alias := range []domain.VideoRouteAlias{domain.VideoRouteKling30Turbo, domain.VideoRouteMiniMaxH3} {
		catalog := newConfiguredCatalog(t, map[domain.VideoRouteAlias]bool{alias: true})
		for _, hasImage := range []bool{false, true} {
			var ids []uuid.UUID
			if hasImage {
				ids = []uuid.UUID{uuid.New()}
			}
			for _, withEmptyPrompt := range []bool{false, true} {
				values := map[string]any{"video_route_alias": alias}
				if withEmptyPrompt {
					values["prompt"] = ""
				}
				params, _ := json.Marshal(values)
				_, err := catalog.Resolve(context.Background(), videorouter.Request{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, Params: params, InputArtifactIDs: ids})
				want := alias == domain.VideoRouteKling30Turbo && hasImage
				if (err == nil) != want {
					t.Fatalf("alias=%s image=%v emptyPrompt=%v error=%v", alias, hasImage, withEmptyPrompt, err)
				}
			}
		}
	}
}
