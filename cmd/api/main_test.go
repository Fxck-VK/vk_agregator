package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
)

func TestWebImagePreparationLimitsUseSafeDefaults(t *testing.T) {
	preparedLimit, preparedTTL, rateLimit, rateWindow := webImagePreparationLimits(config.Config{})
	if preparedLimit != 3 || preparedTTL != 15*time.Minute || rateLimit != 10 || rateWindow != time.Hour {
		t.Fatalf("web image preparation defaults = limit:%d ttl:%s rate:%d window:%s", preparedLimit, preparedTTL, rateLimit, rateWindow)
	}
}

func TestWebImagePreparationLimitsUseConfiguredValues(t *testing.T) {
	preparedLimit, preparedTTL, rateLimit, rateWindow := webImagePreparationLimits(config.Config{
		WebImagePreparedJobLimit:       4,
		WebImagePreparedJobTTL:         20 * time.Minute,
		WebImagePrepareRateLimit:       12,
		WebImagePrepareRateLimitWindow: 90 * time.Minute,
	})
	if preparedLimit != 4 || preparedTTL != 20*time.Minute || rateLimit != 12 || rateWindow != 90*time.Minute {
		t.Fatalf("web image preparation values = limit:%d ttl:%s rate:%d window:%s", preparedLimit, preparedTTL, rateLimit, rateWindow)
	}
}

func TestWebChatMessageLimitsUseSafeDefaults(t *testing.T) {
	limit, window := webChatMessageLimits(config.Config{})
	if limit != 30 || window != time.Minute {
		t.Fatalf("web chat message defaults = limit:%d window:%s", limit, window)
	}
}

func TestWebChatMessageLimitsUseConfiguredValues(t *testing.T) {
	limit, window := webChatMessageLimits(config.Config{
		WebChatMessageRateLimit:       42,
		WebChatMessageRateLimitWindow: 2 * time.Minute,
	})
	if limit != 42 || window != 2*time.Minute {
		t.Fatalf("web chat message values = limit:%d window:%s", limit, window)
	}
}

func TestWebImagePreparedJobReconciliationUsesSafeDefaultsAndConfiguredValues(t *testing.T) {
	interval, limit := webImagePreparedJobReconciliation(config.Config{})
	if interval != time.Minute || limit != 100 {
		t.Fatalf("web image expiry defaults = interval:%s limit:%d", interval, limit)
	}

	interval, limit = webImagePreparedJobReconciliation(config.Config{
		WebImagePreparedJobReconcileInterval: 45 * time.Second,
		WebImagePreparedJobReconcileLimit:    29,
	})
	if interval != 45*time.Second || limit != 29 {
		t.Fatalf("web image expiry values = interval:%s limit:%d", interval, limit)
	}
}

func TestVideoRouteResolverFromCatalogUsesMockLoadTestRoute(t *testing.T) {
	prices, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatalf("pricing catalog: %v", err)
	}
	runtimeCatalog, err := productcatalog.FromConfig(config.Config{
		Env:                                     "loadtest",
		Provider:                                "mock",
		ProviderChain:                           []string{"mock"},
		VideoProvider:                           "mock",
		FeatureVideoRouterEnabled:               true,
		FeatureVideoRouteMockTextToVideoEnabled: true,
	}, prices)
	if err != nil {
		t.Fatalf("runtime catalog: %v", err)
	}
	resolver := videoRouteResolverFromCatalog(runtimeCatalog.VideoRouteCatalog)
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

func TestWebImageModelsFromRuntimeCatalogExposesOnlyReadyProductDimensions(t *testing.T) {
	runtimeModels := []productcatalog.ImageModel{
		{
			ID:                     "nano-banana-2",
			Name:                   "Nano Banana 2",
			Enabled:                true,
			QualityOptions:         []string{"1K", "2K"},
			DefaultQuality:         "1K",
			SupportsReferenceImage: true,
			MaxReferenceImages:     4,
			MaxOutputCount:         4,
		},
		{ID: "disabled", Name: "Disabled", Enabled: false},
	}
	models := webImageModelsFromRuntimeCatalog(runtimeModels)

	if len(models) != 1 {
		t.Fatalf("models = %+v, want only the enabled runtime model", models)
	}
	if !models[0].Ready || !models[0].Enabled || models[0].ID != "nano-banana-2" || models[0].Name != "Nano Banana 2" || models[0].DefaultQuality != "1K" || len(models[0].QualityOptions) != 2 || models[0].MaxOutputCount != 4 {
		t.Fatalf("web model = %+v", models[0])
	}
	models[0].QualityOptions[0] = "mutated"
	if runtimeModels[0].QualityOptions[0] != "1K" {
		t.Fatalf("web config aliases runtime catalog quality options: %+v", runtimeModels[0].QualityOptions)
	}
}

func TestNewWebImageArtifactURLSignerFailsClosedWithoutObjectStoreConfig(t *testing.T) {
	if signer := newWebImageArtifactURLSigner(context.Background(), config.Config{}, nil); signer != nil {
		t.Fatalf("signer = %T, want nil for missing object-store configuration", signer)
	}

	signer := newWebImageArtifactURLSigner(context.Background(), config.Config{
		S3Endpoint:  "objects.example.test",
		S3AccessKey: "test-access",
		S3SecretKey: "test-secret",
	}, nil)
	if signer == nil {
		t.Fatal("signer = nil, want a configured object-store signer")
	}
}

func TestNewWebImageArtifactRedirectPolicyUsesConfiguredObjectStoreOnly(t *testing.T) {
	policy := newWebImageArtifactRedirectPolicy(config.Config{
		Env:               "development",
		S3Endpoint:        "https://objects.example.test",
		S3AddressingStyle: "path",
	}, nil)
	if !policy.Allows("https://objects.example.test/private-artifacts/signed-output?X-Amz-Signature=signature", "private-artifacts") {
		t.Fatal("configured object-store URL must be accepted")
	}
	if policy.Allows("https://attacker.example.test/private-artifacts/signed-output?X-Amz-Signature=signature", "private-artifacts") {
		t.Fatal("unconfigured object-store URL must be rejected")
	}
}
