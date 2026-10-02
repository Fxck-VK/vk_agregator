package productcatalog

import (
	"testing"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestVideoExpansionStaysPendingOutsideDEVWithPricedControls(t *testing.T) {
	list := WorkspaceCatalog(WorkspaceConfig{IncludePendingMedia: true})
	for _, id := range []string{"flux_3_video", "pixverse_v6", "vidu_q3", "vidu_q3_mix", "vidu_q3_turbo", "kling_video_o1", "minimax_h3_max", "wan_3_0_prime", "wan_2_7", "gemini_omni_flash_preview"} {
		m := pendingByID(t, list, id)
		if len(m.Operations) != 1 || m.Operations[0].Enabled || m.Verification != "pending-verification" {
			t.Fatalf("%s enabled outside DEV", id)
		}
		v := m.Operations[0].Video
		if v == nil || len(v.Variants) == 0 || len(v.AllowedDurationsSec) == 0 || len(v.PriceByOption) == 0 {
			t.Fatalf("%s incomplete pending controls", id)
		}
		for _, input := range []bool{m.Operations[0].Inputs.Images.Enabled, m.Operations[0].Inputs.Video.Enabled} {
			if input {
				t.Fatal("unverified input enabled")
			}
		}
	}
}

func TestVideoExpansionCatalogRequiresStorageAndDescribesActualControls(t *testing.T) {
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
	runtime, err := FromConfig(config.Config{Env: "development", FeatureDEVModelSmokeEnabled: true, FeatureVideoRouterEnabled: true, APIMartProviderEnabled: true, APIMartAPIKey: "test-key", APIMartBaseURL: "https://api.example.com"}, prices)
	if err != nil {
		t.Fatal(err)
	}
	for _, uploads := range []bool{false, true} {
		list := WorkspaceCatalog(WorkspaceConfig{VideoRoutes: runtime.VideoRoutes(), Pricing: prices, IncludePendingMedia: true, ImageReferenceUploads: uploads})
		for _, id := range []string{"vidu_q3", "vidu_q3_mix"} {
			model := pendingByID(t, list, id)
			op := model.Operations[0]
			if op.Enabled != uploads || op.Inputs.Images.Enabled != uploads {
				t.Fatal("Vidu execution did not follow storage availability")
			}
			if uploads && (!op.Inputs.Images.Required || op.Inputs.Images.MaxCount != 7 || model.Capabilities.Application.Video.Images.Support != providermodels.Supported) {
				t.Fatal("Vidu required inputs missing")
			}
		}
		model := pendingByID(t, list, "gemini_omni_flash_preview")
		v := model.Operations[0].Video
		if !v.AutomaticDuration || len(v.AllowedDurationsSec) != 1 || v.AllowedDurationsSec[0] != 10 || v.PriceByOption["720p:10"] != 530 {
			t.Fatal("automatic fixed-quote controls inconsistent")
		}
		for _, variant := range v.Variants {
			if variant.FPS == nil || *variant.FPS != 24 || variant.Audio == nil || !*variant.Audio {
				t.Fatal("Gemini output metadata missing")
			}
		}
	}
}
