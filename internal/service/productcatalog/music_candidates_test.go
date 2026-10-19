package productcatalog

import (
	"encoding/json"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/service/providermodels"
)

func TestPendingMediaCatalogIsInformational(t *testing.T) {
	list := WorkspaceCatalog(WorkspaceConfig{IncludePendingMedia: true})
	if len(list.Items) != len(providermodels.MediaCandidates()) {
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
			if op.Music != nil && op.Music.EstimateCredits <= 0 {
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

func TestDEVSmokeCatalogOffersOwnedSunoContinuation(t *testing.T) {
	if err := providermodels.ConfigureDEVSmoke("development", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = providermodels.ConfigureDEVSmoke("development", false) })
	list := WorkspaceCatalog(WorkspaceConfig{IncludePendingMedia: true})
	found := 0
	for _, model := range list.Items {
		if model.ID != "suno_v6" && model.ID != "suno_v6_wild" && model.ID != "suno_v6_mini" {
			continue
		}
		found++
		if model.Verification != "dev-smoke" {
			t.Fatal("permanent admission changed")
		}
		for _, op := range model.Operations {
			want := providermodels.RuntimeRegistry().MediaCandidateRunnable(model.ID, op.ID)
			if op.Enabled != want {
				t.Fatalf("unexpected availability %s/%s", model.ID, op.ID)
			}
			if op.ID == "extend" && (op.Music == nil || op.Music.UnavailableReason != "" || op.Music.EstimateCredits != 30) {
				t.Fatal("missing continuation quote/controls")
			}
		}
	}
	if found != 3 {
		t.Fatalf("Suno count %d", found)
	}
}
