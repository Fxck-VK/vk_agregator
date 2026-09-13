package vk_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	"vk-ai-aggregator/internal/adapter/inbound/vk"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/productcatalog"
)

func TestNewAPIMartImagesCreateReservedVKJobs(t *testing.T) {
	runtime, err := productcatalog.FromConfig(config.Config{APIMartProviderEnabled: true, APIMartAPIKey: "test-key", APIMartBaseURL: "https://example.test/v1",
		FeatureAPIMartGPTImage25FlareEnabled: true, FeatureAPIMartGPTImage25SunburstEnabled: true, FeatureAPIMartSeedream50LiteEnabled: true, FeatureAPIMartSeedream50ProEnabled: true}, staticPricingCatalogForVKTest())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id, quality string
		credits     int64
	}{
		{"gpt_image_2_5_flare", "1K-medium", 20}, {"gpt_image_2_5_sunburst", "2K-medium", 25}, {"seedream_5_0_lite", "3K", 20}, {"seedream_5_0_pro", "1.5K", 20},
	} {
		t.Run(tc.id, func(t *testing.T) {
			control := vkdelivery.NewMockClient()
			h := newHarnessWithConfig(control, vk.Config{Secret: "s3cr3t", ImageModels: runtime.ImageModels()})
			postGrokVKEvent(t, h, "select", "Model", fmt.Sprintf(`{"command":"menu.image.select","model_id":%q}`, tc.id))
			for _, sent := range control.Sent() {
				var keyboard struct {
					Buttons [][]json.RawMessage `json:"buttons"`
				}
				if sent.Keyboard != "" && json.Unmarshal([]byte(sent.Keyboard), &keyboard) == nil && len(keyboard.Buttons) > 6 {
					t.Fatal("quality selector exceeds VK keyboard limit")
				}
			}
			postGrokVKEvent(t, h, "quality", "Quality", fmt.Sprintf(`{"command":"menu.image.quality.select","model_id":%q,"image_quality":%q}`, tc.id, tc.quality))
			h.grantTestCredits(t, 5678, 100)
			postGrokVKEvent(t, h, "prompt", "Synthetic image of a mountain", "")
			postGrokVKEvent(t, h, "prompt", "Synthetic image of a mountain", "")
			user, err := h.users.GetByVKUserID(context.Background(), 5678)
			if err != nil {
				t.Fatal(err)
			}
			jobs, err := h.jobs.ListByUser(context.Background(), user.ID, 10, 0)
			if err != nil || len(jobs) != 1 {
				t.Fatalf("expected one job, got %d: %v", len(jobs), err)
			}
			if jobs[0].CostReserved != tc.credits {
				t.Fatalf("reserved %d, want %d", jobs[0].CostReserved, tc.credits)
			}
		})
	}
}
