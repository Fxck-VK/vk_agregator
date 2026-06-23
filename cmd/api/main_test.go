package main

import (
	"context"
	"encoding/json"
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/joborchestrator"
)

func TestVideoRouteResolverFromConfigUsesMockLoadTestRoute(t *testing.T) {
	resolver, err := videoRouteResolverFromConfig(config.Config{
		Env:                                     "loadtest",
		Provider:                                "mock",
		ProviderChain:                           []string{"mock"},
		VideoProvider:                           "mock",
		FeatureVideoRouterEnabled:               true,
		FeatureVideoRouteMockTextToVideoEnabled: true,
	})
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	params, err := json.Marshal(map[string]any{
		"video_route_alias": string(domain.VideoRouteMockTextToVideo),
		"duration_sec":      5,
		"resolution":        "720p",
		"aspect_ratio":      "16:9",
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	resolution, err := resolver.ResolveVideoRoute(context.Background(), joborchestrator.VideoRouteCheckInput{
		Operation: domain.OperationVideoGenerate,
		Modality:  domain.ModalityVideo,
		Params:    params,
	})
	if err != nil {
		t.Fatalf("resolve route: %v", err)
	}
	if !resolution.Resolved || !resolution.Snapshot.Valid() {
		t.Fatalf("route was not resolved: %+v", resolution)
	}
	if resolution.Snapshot.Provider != domain.ProviderMock || resolution.Snapshot.ProviderModelID != "mock-video" {
		t.Fatalf("unexpected snapshot: %+v", resolution.Snapshot)
	}
}
