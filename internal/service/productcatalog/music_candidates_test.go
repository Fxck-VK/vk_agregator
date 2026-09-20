package productcatalog

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPendingMediaCatalogIsInformational(t *testing.T) {
	list := WorkspaceCatalog(WorkspaceConfig{IncludePendingMedia: true})
	if len(list.Items) != 7 {
		t.Fatalf("candidate count %d", len(list.Items))
	}
	for _, m := range list.Items {
		if m.Verification != "pending-verification" || m.Capabilities == nil {
			t.Fatalf("invalid pending model %s", m.ID)
		}
		for _, op := range m.Operations {
			if op.Enabled || op.Inputs.Audio.Enabled || op.Inputs.Images.Enabled || op.Inputs.Video.Enabled {
				t.Fatalf("unverified operation enabled %s/%s", m.ID, op.ID)
			}
			if m.Kind == "audio" && (op.Music == nil || op.Music.EstimateCredits <= 0) {
				t.Fatalf("music controls absent %s/%s", m.ID, op.ID)
			}
		}
	}
	raw, _ := json.Marshal(list)
	for _, private := range []string{"provider_model_id", "model_code", "apimart.ai", "registry_fingerprint", "live-output"} {
		if strings.Contains(string(raw), private) {
			t.Fatalf("private catalog data leaked: %s", private)
		}
	}
}
