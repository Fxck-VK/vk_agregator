package productcatalog

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/textgeneration"
)

func TestTextCandidateCatalogToSubmission(t *testing.T) {
	if err := providermodels.ConfigureDEVSmoke("development", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = providermodels.ConfigureDEVSmoke("development", false) })
	prices, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if err := prices.AddSupplemental(providermodels.RuntimeRegistry().DEVSmokePrices()); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Env: "development", FeatureDEVModelSmokeEnabled: true, FeatureVideoRouterEnabled: true, APIMartProviderEnabled: true, APIMartAPIKey: "fixture", APIMartBaseURL: "https://api.apimart.ai/v1"}
	runtime, err := FromConfig(cfg, prices)
	if err != nil {
		t.Fatal(err)
	}
	list := WorkspaceCatalog(WorkspaceConfig{TextModels: runtime.TextModels, Pricing: prices, IncludePendingText: true})
	if len(list.Items) != 35 || len(cfg.APIMartTextModels()) != 34 {
		t.Fatal("missing or duplicated text candidates")
	}
	for _, c := range providermodels.TextCandidates() {
		m := pendingByID(t, list, c.PublicID)
		if m.Kind != "text" || !slices.Contains(m.Categories, "text") || m.Verification != "dev-smoke" || !m.Operations[0].Enabled {
			t.Fatalf("incorrect DEV catalog: %s", c.PublicID)
		}
		if !slices.Contains(cfg.APIMartTextModels(), c.ModelCode) {
			t.Fatal("worker and API disagree")
		}
		resolved, q, err := textgeneration.Resolve(c.PublicID, "Synthetic", runtime.TextModels, prices)
		if err != nil || resolved.ModelCode != c.ModelCode || resolved.Provider != domain.ProviderAPIMart || q.InternalCredits != m.Operations[0].Text.EstimateCredits {
			t.Fatalf("unresolvable advertised text: %s", c.PublicID)
		}
		if _, _, err := textgeneration.Resolve(c.PublicID, strings.Repeat("a", q.TextInputTokenCap-511), runtime.TextModels, prices); err == nil {
			t.Fatal("oversized prompt accepted")
		}
		if _, _, err := textgeneration.Resolve(c.PublicID, "Synthetic", nil, prices); err == nil {
			t.Fatal("disabled selection accepted")
		}
		if m.Capabilities.API.Text.Images.Support != providermodels.Unknown || m.Capabilities.Application.Text.Files.Support != providermodels.Unsupported {
			t.Fatal("invented attachment support")
		}
	}
	raw, _ := json.Marshal(list)
	if strings.Contains(string(raw), "api.apimart.ai") || strings.Contains(string(raw), "provider_model_id") {
		t.Fatal("routing leaked to UI")
	}
	for _, mutate := range []func(*config.Config){func(c *config.Config) { c.APIMartProviderEnabled = false }, func(c *config.Config) { c.APIMartAPIKey = "" }, func(c *config.Config) { c.FeatureDEVModelSmokeEnabled = false }, func(c *config.Config) { c.Env = "production" }} {
		closed := cfg
		mutate(&closed)
		if len(closed.APIMartTextModels()) != 0 {
			t.Fatal("worker candidate gate failed")
		}
		runtime, err := FromConfig(closed, prices)
		if err != nil || len(runtime.TextModels) != 1 {
			t.Fatal("API candidate gate failed")
		}
	}
	if err := providermodels.ConfigureDEVSmoke("development", false); err != nil {
		t.Fatal(err)
	}
	pending := WorkspaceCatalog(WorkspaceConfig{IncludePendingText: true})
	for _, m := range pending.Items {
		if m.Verification != "pending-verification" || m.Operations[0].Enabled {
			t.Fatal("pending text runs outside DEV")
		}
	}
}
