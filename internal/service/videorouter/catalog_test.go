package videorouter_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/videorouter"
)

func TestCatalogRejectsUnknownRoute(t *testing.T) {
	catalog := newConfiguredCatalog(t, map[domain.VideoRouteAlias]bool{})

	err := catalog.Validate(context.Background(), videorouter.Request{
		Operation: domain.OperationVideoGenerate,
		Modality:  domain.ModalityVideo,
		Params:    rawJSON(t, map[string]any{"video_route_alias": "video_unknown"}),
	})
	if !errors.Is(err, videorouter.ErrUnknownRoute) {
		t.Fatalf("expected ErrUnknownRoute, got %v", err)
	}
}

func TestCatalogRejectsDisabledRoute(t *testing.T) {
	catalog := newConfiguredCatalog(t, map[domain.VideoRouteAlias]bool{})

	err := catalog.Validate(context.Background(), videorouter.Request{
		Operation: domain.OperationVideoGenerate,
		Modality:  domain.ModalityVideo,
		Params:    rawJSON(t, map[string]any{"video_route_alias": string(domain.VideoRouteHailuo23Standard)}),
	})
	if !errors.Is(err, videorouter.ErrRouteDisabled) {
		t.Fatalf("expected ErrRouteDisabled, got %v", err)
	}
}

func TestCatalogRejectsUnsupportedDuration(t *testing.T) {
	catalog := newConfiguredCatalog(t, map[domain.VideoRouteAlias]bool{
		domain.VideoRouteHailuo23Standard: true,
	})

	err := catalog.Validate(context.Background(), videorouter.Request{
		Operation: domain.OperationVideoGenerate,
		Modality:  domain.ModalityVideo,
		Params: rawJSON(t, map[string]any{
			"video_route_alias": string(domain.VideoRouteHailuo23Standard),
			"duration_sec":      5,
			"resolution":        "768p",
		}),
	})
	if !errors.Is(err, videorouter.ErrUnsupportedDuration) {
		t.Fatalf("expected ErrUnsupportedDuration, got %v", err)
	}
}

func TestCatalogRejectsMissingProviderKey(t *testing.T) {
	catalog, err := videorouter.NewCatalog(videorouter.Config{
		RouterEnabled: true,
		Providers: map[domain.ProviderName]videorouter.ProviderConfig{
			domain.ProviderAPIMart: {
				Enabled:           true,
				RequireAPIKey:     true,
				APIKeyConfigured:  false,
				RequireBaseURL:    true,
				BaseURLConfigured: true,
			},
		},
		EnabledRoutes: map[domain.VideoRouteAlias]bool{
			domain.VideoRouteHailuo23Standard: true,
		},
	})
	if err != nil {
		t.Fatalf("new catalog: %v", err)
	}

	err = catalog.Validate(context.Background(), videorouter.Request{
		Operation: domain.OperationVideoGenerate,
		Modality:  domain.ModalityVideo,
		Params:    rawJSON(t, map[string]any{"video_route_alias": string(domain.VideoRouteHailuo23Standard)}),
	})
	if !errors.Is(err, videorouter.ErrProviderUnconfigured) {
		t.Fatalf("expected ErrProviderUnconfigured, got %v", err)
	}
}

func TestCatalogRejectsProviderModelIDFromClientParams(t *testing.T) {
	catalog := newConfiguredCatalog(t, map[domain.VideoRouteAlias]bool{})

	err := catalog.Validate(context.Background(), videorouter.Request{
		Operation: domain.OperationVideoGenerate,
		Modality:  domain.ModalityVideo,
		Params:    rawJSON(t, map[string]any{"model_code": "MiniMax-Hailuo-2.3-Fast"}),
	})
	if !errors.Is(err, videorouter.ErrProviderModelIDNotAllowed) {
		t.Fatalf("expected ErrProviderModelIDNotAllowed, got %v", err)
	}
}

func TestCatalogRejectsProviderNativeInputURLFromClientParams(t *testing.T) {
	catalog := newConfiguredCatalog(t, map[domain.VideoRouteAlias]bool{})

	err := catalog.Validate(context.Background(), videorouter.Request{
		Operation: domain.OperationVideoGenerate,
		Modality:  domain.ModalityVideo,
		Params:    rawJSON(t, map[string]any{"first_frame_image": "https://example.test/image.png"}),
	})
	if !errors.Is(err, videorouter.ErrProviderModelIDNotAllowed) {
		t.Fatalf("expected ErrProviderModelIDNotAllowed, got %v", err)
	}
}

func TestCatalogAllowsLegacyServerSelectedModelWithoutNewRoute(t *testing.T) {
	catalog := newConfiguredCatalog(t, map[domain.VideoRouteAlias]bool{})

	err := catalog.Validate(context.Background(), videorouter.Request{
		Operation: domain.OperationVideoGenerate,
		Modality:  domain.ModalityVideo,
		Params: rawJSON(t, map[string]any{
			"model_id":   "kling",
			"provider":   "deepinfra",
			"model_code": "PrunaAI/p-video",
		}),
	})
	if err != nil {
		t.Fatalf("legacy model params should pass, got %v", err)
	}
}

func TestCatalogResolveReturnsRouteEstimateAndSanitizedSnapshot(t *testing.T) {
	catalog := newConfiguredCatalog(t, map[domain.VideoRouteAlias]bool{
		domain.VideoRouteKlingO3Standard: true,
	})

	resolution, err := catalog.Resolve(context.Background(), videorouter.Request{
		Operation: domain.OperationVideoGenerate,
		Modality:  domain.ModalityVideo,
		Params: rawJSON(t, map[string]any{
			"prompt":            "clean prompt",
			"video_route_alias": string(domain.VideoRouteKlingO3Standard),
			"duration_sec":      10,
			"resolution":        "720p",
			"aspect_ratio":      "16:9",
		}),
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !resolution.Resolved {
		t.Fatal("route should be resolved")
	}
	if resolution.InternalCostCredits != 200 {
		t.Fatalf("internal cost = %d, want 200", resolution.InternalCostCredits)
	}
	if resolution.Snapshot.Provider != domain.ProviderPoYo || resolution.Snapshot.ProviderModelID != "kling-o3/standard" {
		t.Fatalf("unexpected snapshot: %+v", resolution.Snapshot)
	}
	var params map[string]json.RawMessage
	if err := json.Unmarshal(resolution.Params, &params); err != nil {
		t.Fatalf("unmarshal resolved params: %v", err)
	}
	if _, ok := params["price"]; ok {
		t.Fatal("resolved params must not carry client price")
	}
	var snapshot domain.VideoRouteSnapshot
	if err := json.Unmarshal(params["resolved_video_route"], &snapshot); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if !snapshot.Valid() || snapshot.InternalCostCredits != 200 || snapshot.ProviderCostCredits != 100 {
		t.Fatalf("bad resolved snapshot: %+v", snapshot)
	}
}

func TestCatalogResolveHailuoRouteUsesFixedProviderCost(t *testing.T) {
	catalog := newConfiguredCatalog(t, map[domain.VideoRouteAlias]bool{
		domain.VideoRouteHailuo23Standard: true,
	})

	resolution, err := catalog.Resolve(context.Background(), videorouter.Request{
		Operation: domain.OperationVideoGenerate,
		Modality:  domain.ModalityVideo,
		Params: rawJSON(t, map[string]any{
			"prompt":            "clean prompt",
			"video_route_alias": string(domain.VideoRouteHailuo23Standard),
			"duration_sec":      6,
			"resolution":        "768p",
		}),
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolution.InternalCostCredits != 2 || resolution.Snapshot.ProviderCostCredits != 1 {
		t.Fatalf("unexpected cost snapshot: %+v", resolution.Snapshot)
	}
	if resolution.Snapshot.Provider != domain.ProviderAPIMart || resolution.Snapshot.ProviderModelID != "MiniMax-Hailuo-2.3" {
		t.Fatalf("unexpected route snapshot: %+v", resolution.Snapshot)
	}
}

func TestCatalogResolveRunwaySupportsOfficialPortraitAspect(t *testing.T) {
	catalog := newConfiguredCatalog(t, map[domain.VideoRouteAlias]bool{
		domain.VideoRouteRunwayGen4Turbo: true,
	})

	resolution, err := catalog.Resolve(context.Background(), videorouter.Request{
		Operation: domain.OperationVideoGenerate,
		Modality:  domain.ModalityVideo,
		Params: rawJSON(t, map[string]any{
			"prompt":            "clean prompt",
			"video_route_alias": string(domain.VideoRouteRunwayGen4Turbo),
			"duration_sec":      5,
			"resolution":        "720p",
			"aspect_ratio":      "3:4",
			"reference_artifact_ids": []string{
				"11111111-1111-1111-8111-111111111111",
			},
		}),
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolution.Snapshot.Provider != domain.ProviderRunway || resolution.Snapshot.ProviderModelID != "gen4_turbo" {
		t.Fatalf("unexpected Runway snapshot: %+v", resolution.Snapshot)
	}
	if resolution.Snapshot.AspectRatio != "3:4" {
		t.Fatalf("aspect ratio = %q, want 3:4", resolution.Snapshot.AspectRatio)
	}
	if resolution.InternalCostCredits != 50 {
		t.Fatalf("internal cost = %d, want 50", resolution.InternalCostCredits)
	}
}

func TestCatalogResolveRejectsClientPriceFields(t *testing.T) {
	catalog := newConfiguredCatalog(t, map[domain.VideoRouteAlias]bool{
		domain.VideoRouteKlingO3Standard: true,
	})

	err := catalog.Validate(context.Background(), videorouter.Request{
		Operation: domain.OperationVideoGenerate,
		Modality:  domain.ModalityVideo,
		Params: rawJSON(t, map[string]any{
			"video_route_alias": string(domain.VideoRouteKlingO3Standard),
			"duration_sec":      10,
			"price":             1,
		}),
	})
	if !errors.Is(err, videorouter.ErrProviderModelIDNotAllowed) {
		t.Fatalf("expected ErrProviderModelIDNotAllowed, got %v", err)
	}
}

func TestCatalogPublicRoutesHidePrivateProviderDetails(t *testing.T) {
	catalog := newConfiguredCatalog(t, map[domain.VideoRouteAlias]bool{
		domain.VideoRouteHailuo23Standard: true,
		domain.VideoRouteRunwayGen45:      true,
	})

	routes := catalog.PublicRoutes()
	if len(routes) != 1 {
		t.Fatalf("public routes = %d, want only costed enabled route: %+v", len(routes), routes)
	}
	route := routes[0]
	if route.Alias != domain.VideoRouteHailuo23Standard {
		t.Fatalf("unexpected public route: %+v", route)
	}
	raw, err := json.Marshal(route)
	if err != nil {
		t.Fatalf("marshal public route: %v", err)
	}
	for _, private := range []string{"provider", "model", "MiniMax", "cost", "price"} {
		if strings.Contains(string(raw), private) {
			t.Fatalf("public route leaked private detail %q: %s", private, raw)
		}
	}
	if route.MaxReferenceImages != 1 || route.DefaultDurationSec != 6 {
		t.Fatalf("missing route constraints: %+v", route)
	}
}

func TestCatalogRejectsTooManyImageReferences(t *testing.T) {
	catalog := newConfiguredCatalog(t, map[domain.VideoRouteAlias]bool{
		domain.VideoRouteHailuo23Standard: true,
	})

	err := catalog.Validate(context.Background(), videorouter.Request{
		Operation: domain.OperationVideoGenerate,
		Modality:  domain.ModalityVideo,
		Params: rawJSON(t, map[string]any{
			"video_route_alias": string(domain.VideoRouteHailuo23Standard),
			"duration_sec":      6,
			"resolution":        "768p",
			"reference_artifact_ids": []string{
				"11111111-1111-1111-8111-111111111111",
				"22222222-2222-2222-8222-222222222222",
			},
		}),
	})
	if !errors.Is(err, videorouter.ErrTooManyReferenceImages) {
		t.Fatalf("expected ErrTooManyReferenceImages, got %v", err)
	}
}

func TestCatalogResolveFailsClosedWhenRouteCostMissing(t *testing.T) {
	catalog := newConfiguredCatalog(t, map[domain.VideoRouteAlias]bool{
		domain.VideoRouteRunwayGen45: true,
	})

	_, err := catalog.Resolve(context.Background(), videorouter.Request{
		Operation: domain.OperationVideoGenerate,
		Modality:  domain.ModalityVideo,
		Params: rawJSON(t, map[string]any{
			"video_route_alias": string(domain.VideoRouteRunwayGen45),
			"duration_sec":      5,
			"resolution":        "720p",
			"reference_artifact_ids": []string{
				"11111111-1111-1111-1111-111111111111",
			},
		}),
	})
	if !errors.Is(err, videorouter.ErrRouteCostUnavailable) {
		t.Fatalf("expected ErrRouteCostUnavailable, got %v", err)
	}
}

func newConfiguredCatalog(t *testing.T, enabledRoutes map[domain.VideoRouteAlias]bool) *videorouter.Catalog {
	t.Helper()
	catalog, err := videorouter.NewCatalog(videorouter.Config{
		RouterEnabled: true,
		Providers: map[domain.ProviderName]videorouter.ProviderConfig{
			domain.ProviderAPIMart: {
				Enabled:           true,
				RequireAPIKey:     true,
				APIKeyConfigured:  true,
				RequireBaseURL:    true,
				BaseURLConfigured: true,
			},
			domain.ProviderPoYo: {
				Enabled:           true,
				RequireAPIKey:     true,
				APIKeyConfigured:  true,
				RequireBaseURL:    true,
				BaseURLConfigured: true,
			},
			domain.ProviderRunway: {
				Enabled:           true,
				RequireAPIKey:     true,
				APIKeyConfigured:  true,
				RequireBaseURL:    true,
				BaseURLConfigured: true,
			},
		},
		EnabledRoutes: enabledRoutes,
	})
	if err != nil {
		t.Fatalf("new catalog: %v", err)
	}
	return catalog
}

func rawJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	return raw
}
