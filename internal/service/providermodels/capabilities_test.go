package providermodels

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCapabilitiesCoverEveryRealModelWithoutPrivateMetadata(t *testing.T) {
	r := StaticRegistry()
	ids := []string{}
	for _, m := range r.TextAliases {
		ids = append(ids, m.PublicID)
	}
	for _, m := range r.ImageModels {
		ids = append(ids, m.PublicID)
	}
	for _, m := range r.VideoRouteModels {
		if !m.LoadTestOnly {
			ids = append(ids, string(m.Alias))
		}
	}
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			c := Capabilities(id)
			if c == nil || c.SchemaVersion != 1 {
				t.Fatal("model has no versioned capability record")
			}
			if (c.API.Text == nil) != (c.Application.Text == nil) || (c.API.Image == nil) != (c.Application.Image == nil) || (c.API.Video == nil) != (c.Application.Video == nil) || (c.API.Audio == nil) != (c.Application.Audio == nil) {
				t.Fatal("API/application purposes differ")
			}
			for _, profile := range []CapabilityProfile{c.API, c.Application} {
				count := 0
				if profile.Text != nil {
					count++
				}
				if profile.Image != nil {
					count++
				}
				if profile.Video != nil {
					count++
				}
				if profile.Audio != nil {
					count++
				}
				if count != 1 {
					t.Fatal("capabilities must have exactly one purpose")
				}
			}
			data, err := json.Marshal(c)
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"provider_model", "api_key", "base_url", "http://", "https://", "pricing_keys", "provider_cost", "readiness", "feature_flag"} {
				if strings.Contains(strings.ToLower(string(data)), forbidden) {
					t.Fatalf("public metadata leaked %s", forbidden)
				}
			}
		})
	}
	if Capabilities("unknown_model") != nil || Capabilities(LoadTestImageMock) != nil {
		t.Fatal("unknown and mock models must not gain public capability records")
	}
}
