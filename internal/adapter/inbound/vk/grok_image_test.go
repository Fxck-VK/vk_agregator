package vk_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	"vk-ai-aggregator/internal/adapter/inbound/vk"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/productcatalog"
)

func grokVKImageModels(t *testing.T) []productcatalog.ImageModel {
	t.Helper()
	runtime, err := productcatalog.FromConfig(config.Config{
		APIMartProviderEnabled: true, APIMartAPIKey: "test-key", APIMartBaseURL: "https://example.test/v1",
		PoYoProviderEnabled: true, PoYoAPIKey: "test-key", PoYoBaseURL: "https://example.test",
		FeatureImageModelNanoBanana2Enabled:   true,
		FeatureImageModelNanoBananaProEnabled: true,
		FeatureImageModelGPTImage2Enabled:     true,
		FeatureAPIMartQwenImage3Enabled:       true,
		FeatureAPIMartGrokImage15Enabled:      true,
		FeatureAPIMartGrokImage20Enabled:      true,
	}, staticPricingCatalogForVKTest())
	if err != nil {
		t.Fatal(err)
	}
	if len(runtime.ImageModels()) != 6 {
		t.Fatalf("DEV catalog contains %d models, want six", len(runtime.ImageModels()))
	}
	return runtime.ImageModels()
}

func TestPhotoMenuWithGrokFitsVKInlineKeyboard(t *testing.T) {
	for _, mode := range []string{"text", "callback"} {
		t.Run(mode, func(t *testing.T) {
			models := grokVKImageModels(t)
			control := vkdelivery.NewMockClient()
			h := newHarnessWithConfig(control, vk.Config{Secret: "s3cr3t", MenuButtonMode: mode, ImageModels: models})
			postGrokVKEvent(t, h, "menu", "Photo", `{"command":"menu.image"}`)
			sent := control.Sent()
			if len(sent) != 1 {
				t.Fatalf("photo menu messages = %d, want one", len(sent))
			}
			var keyboard struct {
				Inline  bool `json:"inline"`
				Buttons [][]struct {
					Action struct{ Type, Label, Payload string } `json:"action"`
				} `json:"buttons"`
			}
			if err := json.Unmarshal([]byte(sent[0].Keyboard), &keyboard); err != nil {
				t.Fatal(err)
			}
			if !keyboard.Inline || len(keyboard.Buttons) > 6 {
				t.Fatalf("VK rejects photo menu: inline=%v rows=%d (maximum 6)", keyboard.Inline, len(keyboard.Buttons))
			}
			seen := map[string]bool{}
			total, back := 0, 0
			for _, row := range keyboard.Buttons {
				if len(row) == 0 || len(row) > 5 {
					t.Fatalf("invalid VK keyboard row width: %d", len(row))
				}
				for _, button := range row {
					total++
					if button.Action.Type != mode {
						t.Fatalf("button action = %s, want %s", button.Action.Type, mode)
					}
					var payload struct {
						Command string `json:"command"`
						ModelID string `json:"model_id"`
					}
					if err := json.Unmarshal([]byte(button.Action.Payload), &payload); err != nil {
						t.Fatal(err)
					}
					if payload.Command == string(domain.CommandMenuImageSelect) {
						seen[payload.ModelID] = true
					}
					if payload.Command == string(domain.CommandShowMenu) {
						back++
					}
				}
			}
			if total > 10 || back != 1 || len(seen) != len(models) {
				t.Fatalf("invalid model menu: buttons=%d back=%d models=%d", total, back, len(seen))
			}
			for _, model := range models {
				if !seen[model.ID] {
					t.Fatalf("missing model %s", model.ID)
				}
			}
			user, err := h.users.GetByVKUserID(context.Background(), 5678)
			if err != nil {
				t.Fatal(err)
			}
			jobs, err := h.jobs.ListByUser(context.Background(), user.ID, 10, 0)
			if err != nil || len(jobs) != 0 {
				t.Fatal("opening photo menu created a job")
			}
		})
	}
}

func TestPhotoGrokCreatesReservedVKJobWithValidWorkerOptions(t *testing.T) {
	for _, tc := range []struct{ id, model, resolution string }{
		{"grok_image_1_5", "grok-imagine-1.5-apimart", ""},
		{"grok_image_2_0", "grok-imagine-2.0-ext", "quality"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			h := newHarnessWithConfig(vkdelivery.NewMockClient(), vk.Config{Secret: "s3cr3t", ImageModels: grokVKImageModels(t)})
			postGrokVKEvent(t, h, "select", "Grok", fmt.Sprintf(`{"command":"menu.image.select","model_id":%q}`, tc.id))
			postGrokVKEvent(t, h, "quality", "Standard", fmt.Sprintf(`{"command":"menu.image.quality.select","model_id":%q,"image_quality":"standard"}`, tc.id))
			h.grantTestCredits(t, 5678, 100)
			postGrokVKEvent(t, h, "prompt", "Synthetic image of a blue cube", "")
			postGrokVKEvent(t, h, "prompt", "Synthetic image of a blue cube", "")
			user, err := h.users.GetByVKUserID(context.Background(), 5678)
			if err != nil {
				t.Fatal(err)
			}
			jobs, err := h.jobs.ListByUser(context.Background(), user.ID, 10, 0)
			if err != nil || len(jobs) != 1 {
				t.Fatalf("jobs=%d, error=%v; want one idempotent job", len(jobs), err)
			}
			job := jobs[0]
			if job.Source != "vk_bot" || job.OperationType != domain.OperationImageGenerate || job.CostReserved != 10 || job.CostEstimate != 10 {
				t.Fatal("incorrect VK job routing or credit reservation")
			}
			if price, ok := job.PricingSnapshotCredits(); !ok || price != 10 {
				t.Fatal("wrong immutable tariff")
			}
			var params struct {
				ModelID    string `json:"model_id"`
				ModelCode  string `json:"model_code"`
				Provider   string `json:"provider"`
				Resolution string `json:"resolution"`
				Quality    string `json:"image_quality"`
				Size       string `json:"size"`
			}
			if err := json.Unmarshal(job.Params, &params); err != nil {
				t.Fatal(err)
			}
			if params.ModelID != tc.id || params.ModelCode != tc.model || params.Provider != "apimart" || params.Resolution != tc.resolution || params.Quality != "standard" || params.Size != "1:1" {
				t.Fatalf("invalid Grok worker options: model=%s resolution=%q quality=%q size=%q", params.ModelID, params.Resolution, params.Quality, params.Size)
			}
		})
	}
}

func postGrokVKEvent(t *testing.T, h *harness, event, text, payload string) {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"type": "message_new", "group_id": 1, "event_id": "grok-" + event, "secret": "s3cr3t",
		"object": map[string]any{"message": map[string]any{"from_id": 5678, "peer_id": 5678, "text": text, "payload": payload}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec := h.post(string(body)); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("VK event %s failed: HTTP %d", event, rec.Code)
	}
}
