package productcatalog

import (
	"testing"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestTextCatalogKeepsProviderVerificationIndependent(t *testing.T) {
	for _, model := range providermodels.PaidTextModels() {
		t.Setenv(model.FeatureFlag, "true")
	}
	cfg := config.Load()
	cfg.KIEProviderEnabled, cfg.APIMartProviderEnabled = true, true
	cfg.KIEAPIKey, cfg.APIMartAPIKey = "synthetic", "synthetic"
	cfg.KIEBaseURL, cfg.APIMartBaseURL = "https://api.kie.ai", "https://api.apimart.ai/v1"
	prices, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		kie, apimart bool
		count        int
	}{{false, false, 1}, {true, false, 11}, {false, true, 2}, {true, true, 12}} {
		cfg.KIETextLimitsVerified, cfg.APIMartTextLimitsVerified = tc.kie, tc.apimart
		catalog, err := FromConfig(cfg, prices)
		if err != nil {
			t.Fatal(err)
		}
		if len(catalog.TextModels) != tc.count {
			t.Fatalf("kie=%t apimart=%t models=%d want=%d", tc.kie, tc.apimart, len(catalog.TextModels), tc.count)
		}
	}
	cfg.KIEAPIKey, cfg.APIMartAPIKey = "", ""
	catalog, err := FromConfig(cfg, prices)
	if err != nil || len(catalog.TextModels) != 1 {
		t.Fatal("missing credentials accepted")
	}
}
