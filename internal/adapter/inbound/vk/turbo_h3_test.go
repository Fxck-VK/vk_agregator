package vk_test

import (
	"encoding/json"
	"testing"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	"vk-ai-aggregator/internal/adapter/inbound/vk"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestVideoMenuWithTurboH3FitsVKKeyboard(t *testing.T) {
	var routes []productcatalog.VideoRoute
	for _, route := range providermodels.StaticRegistry().VideoRoutes() {
		if route.LoadTestOnly || len(route.PricingKeys) == 0 {
			continue
		}
		routes = append(routes, productcatalog.VideoRoute{Alias: string(route.Alias), Name: route.ModelClass, Enabled: true})
	}
	for _, mode := range []string{"text", "callback"} {
		t.Run(mode, func(t *testing.T) {
			control := vkdelivery.NewMockClient()
			h := newHarnessWithConfig(control, vk.Config{Secret: "s3cr3t", MenuButtonMode: mode, VideoRoutes: routes})
			postGrokVKEvent(t, h, "video-menu", "Video", `{"command":"menu.video"}`)
			sent := control.Sent()
			if len(sent) != 1 {
				t.Fatalf("sent %d messages", len(sent))
			}
			var keyboard struct {
				Inline  bool `json:"inline"`
				Buttons [][]struct {
					Action struct{ Type, Payload string } `json:"action"`
				} `json:"buttons"`
			}
			if err := json.Unmarshal([]byte(sent[0].Keyboard), &keyboard); err != nil {
				t.Fatal(err)
			}
			if !keyboard.Inline || len(keyboard.Buttons) > 6 {
				t.Fatalf("VK keyboard has %d rows; maximum6", len(keyboard.Buttons))
			}
			seen := map[string]bool{}
			for _, row := range keyboard.Buttons {
				if len(row) == 0 || len(row) > 5 {
					t.Fatal("unsupported row width")
				}
				for _, button := range row {
					if button.Action.Type != mode {
						t.Fatal("button mode lost")
					}
					var payload struct {
						Command string `json:"command"`
						Alias   string `json:"video_route_alias"`
					}
					if err := json.Unmarshal([]byte(button.Action.Payload), &payload); err != nil {
						t.Fatal(err)
					}
					if payload.Command == string(domain.CommandMenuVideoRouteSelect) {
						seen[payload.Alias] = true
					}
				}
			}
			if len(seen) != len(routes) || !seen[string(domain.VideoRouteKling30Turbo)] || !seen[string(domain.VideoRouteMiniMaxH3)] {
				t.Fatal("video model missing from keyboard")
			}
		})
	}
}
