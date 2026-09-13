package videorouter_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/videorouter"
)

func TestKlingVeoPublicRoutesExposeSafeCatalogBounds(t *testing.T) {
	catalog := newConfiguredCatalog(t, map[domain.VideoRouteAlias]bool{
		domain.VideoRouteKlingV3:       true,
		domain.VideoRouteKling26Motion: true,
		domain.VideoRouteVeo31Lite:     true,
		domain.VideoRouteVeo31Fast:     true,
		domain.VideoRouteVeo31Quality:  true,
	})

	routes := catalog.PublicRoutes()
	for _, alias := range []domain.VideoRouteAlias{
		domain.VideoRouteKlingV3,
		domain.VideoRouteKling26Motion,
		domain.VideoRouteVeo31Lite,
		domain.VideoRouteVeo31Fast,
		domain.VideoRouteVeo31Quality,
	} {
		if route := publicRouteByAlias(routes, alias); route == nil {
			t.Fatalf("public route %s missing from %+v", alias, routes)
		}
	}
	for _, route := range routes {
		raw, err := json.Marshal(route)
		if err != nil {
			t.Fatalf("marshal public route: %v", err)
		}
		for _, private := range []string{"provider", "model_id", "kling-v3", "veo3.1", "cost", "price"} {
			if strings.Contains(string(raw), private) {
				t.Fatalf("public route leaked private detail %q: %s", private, raw)
			}
		}
	}

	kling := publicRouteByAlias(routes, domain.VideoRouteKlingV3)
	if kling == nil ||
		!kling.SupportsAudio ||
		kling.RequiresReferenceVideo ||
		!kling.SupportsReferenceImage ||
		kling.MaxReferenceImages != 2 ||
		kling.DefaultDurationSec != 5 ||
		kling.DefaultResolution != "720p" ||
		!containsInt(kling.AllowedDurationsSec, 3) ||
		!containsInt(kling.AllowedDurationsSec, 15) {
		t.Fatalf("Kling V3 public constraints mismatch: %+v", kling)
	}

	motion := publicRouteByAlias(routes, domain.VideoRouteKling26Motion)
	if motion == nil ||
		motion.SupportsAudio ||
		!motion.RequiresReferenceVideo ||
		!motion.RequiresStartImage ||
		!motion.SupportsReferenceImage ||
		motion.MaxReferenceImages != 1 ||
		motion.DefaultDurationSec != 3 ||
		motion.DefaultResolution != "std" ||
		!containsInt(motion.AllowedDurationsSec, 3) ||
		!containsInt(motion.AllowedDurationsSec, 30) {
		t.Fatalf("Kling 2.6 Motion public constraints mismatch: %+v", motion)
	}

	for _, tc := range []struct {
		alias   domain.VideoRouteAlias
		maxRefs int
		refs    bool
	}{
		{alias: domain.VideoRouteVeo31Lite, maxRefs: 0, refs: false},
		{alias: domain.VideoRouteVeo31Fast, maxRefs: 3, refs: true},
		{alias: domain.VideoRouteVeo31Quality, maxRefs: 2, refs: true},
	} {
		route := publicRouteByAlias(routes, tc.alias)
		if route == nil ||
			route.SupportsReferenceImage != tc.refs ||
			route.MaxReferenceImages != tc.maxRefs ||
			route.DefaultDurationSec != 8 ||
			route.DefaultResolution != "720p" ||
			!containsString(route.AllowedResolutions, "4k") {
			t.Fatalf("%s public constraints mismatch: %+v", tc.alias, route)
		}
	}
}

func TestKlingV3VideoAudioChangesResolvedProviderFloor(t *testing.T) {
	catalog := newConfiguredCatalog(t, map[domain.VideoRouteAlias]bool{
		domain.VideoRouteKlingV3: true,
	})

	for _, tc := range []struct {
		name         string
		audio        bool
		providerCost int64
		internalCost int64
	}{
		{name: "720p without audio", providerCost: 4, internalCost: 240},
		{name: "720p with audio", audio: true, providerCost: 6, internalCost: 360},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolution, err := catalog.Resolve(context.Background(), videorouter.Request{
				Operation: domain.OperationVideoGenerate,
				Modality:  domain.ModalityVideo,
				Params: rawJSON(t, map[string]any{
					"video_route_alias": string(domain.VideoRouteKlingV3),
					"duration_sec":      5,
					"resolution":        "720p",
					"aspect_ratio":      "16:9",
					"video_audio":       tc.audio,
				}),
			})
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if resolution.Snapshot.Provider != domain.ProviderAPIMart ||
				resolution.Snapshot.ProviderModelID != "kling-v3" ||
				resolution.Snapshot.VideoAudio != tc.audio ||
				resolution.Snapshot.ProviderCostCredits != tc.providerCost ||
				resolution.Snapshot.InternalCostCredits != tc.internalCost ||
				resolution.InternalCostCredits != tc.internalCost {
				t.Fatalf("unexpected Kling V3 snapshot: %+v", resolution.Snapshot)
			}
		})
	}
}

func TestKlingMotionRequiresOwnedVideoIDAndOneImage(t *testing.T) {
	catalog := newConfiguredCatalog(t, map[domain.VideoRouteAlias]bool{
		domain.VideoRouteKling26Motion: true,
	})
	imageID := uuid.MustParse("11111111-1111-1111-8111-111111111111")
	videoID := uuid.MustParse("22222222-2222-2222-8222-222222222222")

	imageOrientation, err := catalog.Resolve(context.Background(), klingMotionRequest(t, map[string]any{
		"duration_sec":                10,
		"resolution":                  "std",
		"reference_artifact_ids":      []string{imageID.String()},
		"reference_video_artifact_id": videoID.String(),
	}))
	if err != nil {
		t.Fatalf("image-oriented motion resolve: %v", err)
	}
	if imageOrientation.Snapshot.ReferenceVideoArtifactID != videoID.String() ||
		imageOrientation.Snapshot.CharacterOrientation != "image" ||
		!imageOrientation.Snapshot.KeepOriginalSound ||
		imageOrientation.Snapshot.ProviderCostCredits != 6 ||
		imageOrientation.Snapshot.InternalCostCredits != 360 {
		t.Fatalf("unexpected image-oriented motion snapshot: %+v", imageOrientation.Snapshot)
	}

	videoOrientation, err := catalog.Resolve(context.Background(), klingMotionRequest(t, map[string]any{
		"duration_sec":                30,
		"resolution":                  "std",
		"character_orientation":       "video",
		"keep_original_sound":         false,
		"reference_artifact_ids":      []string{imageID.String()},
		"reference_video_artifact_id": videoID.String(),
	}))
	if err != nil {
		t.Fatalf("video-oriented motion resolve: %v", err)
	}
	if videoOrientation.Snapshot.CharacterOrientation != "video" ||
		videoOrientation.Snapshot.KeepOriginalSound ||
		videoOrientation.Snapshot.ProviderCostCredits != 18 ||
		videoOrientation.Snapshot.InternalCostCredits != 1080 {
		t.Fatalf("unexpected video-oriented motion snapshot: %+v", videoOrientation.Snapshot)
	}

	for _, tc := range []struct {
		name    string
		params  map[string]any
		wantErr error
	}{
		{
			name: "missing owned video artifact id",
			params: map[string]any{
				"duration_sec":           10,
				"resolution":             "std",
				"reference_artifact_ids": []string{imageID.String()},
			},
			wantErr: videorouter.ErrInvalidRouteRequest,
		},
		{
			name: "missing required image reference",
			params: map[string]any{
				"duration_sec":                10,
				"resolution":                  "std",
				"reference_video_artifact_id": videoID.String(),
			},
			wantErr: videorouter.ErrMissingStartImage,
		},
		{
			name: "image orientation capped at 10 seconds",
			params: map[string]any{
				"duration_sec":                11,
				"resolution":                  "std",
				"character_orientation":       "image",
				"reference_artifact_ids":      []string{imageID.String()},
				"reference_video_artifact_id": videoID.String(),
			},
			wantErr: videorouter.ErrUnsupportedDuration,
		},
		{
			name: "client video URL rejected",
			params: map[string]any{
				"duration_sec":                10,
				"resolution":                  "std",
				"reference_artifact_ids":      []string{imageID.String()},
				"reference_video_artifact_id": videoID.String(),
				"reference_video_url":         "https://example.com/private.mp4",
			},
			wantErr: videorouter.ErrInvalidRouteRequest,
		},
		{
			name: "direct audio flag rejected",
			params: map[string]any{
				"duration_sec":                10,
				"resolution":                  "std",
				"video_audio":                 true,
				"reference_artifact_ids":      []string{imageID.String()},
				"reference_video_artifact_id": videoID.String(),
			},
			wantErr: videorouter.ErrInvalidRouteRequest,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := catalog.Resolve(context.Background(), klingMotionRequest(t, tc.params))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("resolve error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func klingMotionRequest(t *testing.T, params map[string]any) videorouter.Request {
	t.Helper()
	params["video_route_alias"] = string(domain.VideoRouteKling26Motion)
	return videorouter.Request{
		Operation: domain.OperationVideoGenerate,
		Modality:  domain.ModalityVideo,
		Params:    rawJSON(t, params),
	}
}

func containsInt(values []int, want int) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
