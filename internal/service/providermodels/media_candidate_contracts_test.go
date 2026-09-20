package providermodels_test

import (
	"strings"
	"testing"

	"vk-ai-aggregator/internal/service/providermodels"
)

func TestMediaCandidatesDoNotEnterActiveBindings(t *testing.T) {
	registry := providermodels.StaticRegistry()
	if err := registry.Validate(); err != nil {
		t.Fatalf("active registry validate: %v", err)
	}
	active := map[string]bool{}
	for _, binding := range registry.Bindings() {
		active[binding.PublicID] = true
	}
	for _, candidate := range providermodels.MediaCandidates() {
		if active[candidate.PublicID] {
			t.Fatalf("candidate %s unexpectedly has active binding", candidate.PublicID)
		}
		if _, exists := registry.Contracts[candidate.PublicID]; exists {
			t.Fatalf("candidate %s unexpectedly has active onboarding contract", candidate.PublicID)
		}
		facts := providermodels.MediaCandidateOperationFacts(candidate.PublicID)
		if len(facts) == 0 {
			t.Fatalf("candidate %s has no operation facts", candidate.PublicID)
		}
		if registry.MediaCandidateAdmitted(candidate.PublicID, facts[0].ID) {
			t.Fatalf("candidate %s unexpectedly admitted for %s", candidate.PublicID, facts[0].ID)
		}
	}
}

func TestDraftMediaContractsRemainNotReady(t *testing.T) {
	for _, candidate := range providermodels.MediaCandidates() {
		contract := providermodels.DraftMediaContract(candidate)
		if contract.Status != "draft" {
			t.Fatalf("%s status = %q, want draft", candidate.PublicID, contract.Status)
		}
		if contract.Revision != "2026-09-16" {
			t.Fatalf("%s revision = %q", candidate.PublicID, contract.Revision)
		}
		if contract.ProviderModelID != candidate.ModelCode || contract.Provider != string(candidate.Provider) {
			t.Fatalf("%s provider identity mismatch: %+v", candidate.PublicID, contract)
		}
		if err := contract.ValidateReady(); err == nil {
			t.Fatalf("%s ValidateReady unexpectedly passed", candidate.PublicID)
		}
		if len(contract.Sources) == 0 || len(contract.Operations) == 0 || len(contract.Checks) == 0 {
			t.Fatalf("%s missing sources/operations/checks: %+v", candidate.PublicID, contract)
		}
		for _, source := range contract.Sources {
			if source.CheckedAt != "2026-09-16" {
				t.Fatalf("%s source %s checked_at = %q", candidate.PublicID, source.ID, source.CheckedAt)
			}
			if !strings.HasPrefix(source.URL, "https://docs.apimart.ai/ru/api-reference/") {
				t.Fatalf("%s source %s URL = %q", candidate.PublicID, source.ID, source.URL)
			}
		}
		for _, check := range contract.Checks {
			if check.Status == "passed" {
				t.Fatalf("%s has fabricated passed check %s", candidate.PublicID, check.Scenario)
			}
		}
	}
}

func TestVideoDraftsKeepUnknownOutputFPSZero(t *testing.T) {
	for _, id := range []string{"happyhorse_1_0", "happyhorse_1_1", "skyreels_v4_fast", "skyreels_v4_std"} {
		candidate, ok := providermodels.MediaCandidateByID(id)
		if !ok {
			t.Fatalf("missing candidate %s", id)
		}
		contract := providermodels.DraftMediaContract(candidate)
		if contract.Endpoint != "POST /v1/videos/generations" {
			t.Fatalf("%s endpoint = %q", id, contract.Endpoint)
		}
		for _, op := range contract.Operations {
			if op.Video == nil {
				continue
			}
			for _, variant := range op.Video.Variants {
				if variant.FPS != 0 {
					t.Fatalf("%s/%s FPS = %d; unknown output FPS must stay zero in draft", id, op.ID, variant.FPS)
				}
				if variant.Audio {
					t.Fatalf("%s/%s advertised output audio before live verification", id, op.ID)
				}
			}
		}
	}
}

func TestSunoDraftsKeepUnknownVoicesAndLanguagesEmpty(t *testing.T) {
	for _, id := range []string{"suno_v6", "suno_v6_wild", "suno_v6_mini"} {
		candidate, ok := providermodels.MediaCandidateByID(id)
		if !ok {
			t.Fatalf("missing candidate %s", id)
		}
		contract := providermodels.DraftMediaContract(candidate)
		for _, op := range contract.Operations {
			if op.Audio == nil {
				continue
			}
			if len(op.Audio.Voices) != 0 || len(op.Audio.Languages) != 0 {
				t.Fatalf("%s/%s advertised voices/languages before verification: %+v", id, op.ID, op.Audio)
			}
		}
	}
}

func TestMediaCandidateOperationFactsExposeNativeEndpointsAndVersions(t *testing.T) {
	tests := []struct {
		publicID string
		opID     string
		endpoint string
		version  string
	}{
		{"happyhorse_1_0", "video_edit", "POST /v1/videos/generations", "happyhorse-1.0"},
		{"happyhorse_1_1", "reference_image_to_video", "POST /v1/videos/generations", "happyhorse-1.1"},
		{"skyreels_v4_fast", "omni_reference_video", "POST /v1/videos/generations", "skyreels-v4-fast"},
		{"skyreels_v4_std", "omni_extend_video", "POST /v1/videos/generations", "skyreels-v4-std"},
		{"suno_v6", "generate", "POST /v1/music/generations", "v6"},
		{"suno_v6_wild", "inspo", "POST /v1/music/generations/inspo", "v6-wild"},
		{"suno_v6_mini", "upload", "POST /v1/music/generations/uploadTask", ""},
		{"suno_v6_mini", "export", "POST /v1/music/generations/download", ""},
	}
	for _, tt := range tests {
		fact, ok := findOperationFact(tt.publicID, tt.opID)
		if !ok {
			t.Fatalf("missing operation fact %s/%s", tt.publicID, tt.opID)
		}
		if fact.Endpoint != tt.endpoint || fact.NativeVersion != tt.version {
			t.Fatalf("%s/%s endpoint/version = %q/%q, want %q/%q", tt.publicID, tt.opID, fact.Endpoint, fact.NativeVersion, tt.endpoint, tt.version)
		}
		if fact.SourceID == "" || len(fact.InputModes) == 0 || len(fact.KnownLimits) == 0 {
			t.Fatalf("%s/%s missing source/input/limit facts: %+v", tt.publicID, tt.opID, fact)
		}
	}
}

func TestSunoDraftSourcesCoverEveryOperationFact(t *testing.T) {
	for _, id := range []string{"suno_v6", "suno_v6_wild", "suno_v6_mini"} {
		candidate, ok := providermodels.MediaCandidateByID(id)
		if !ok {
			t.Fatalf("missing candidate %s", id)
		}
		contract := providermodels.DraftMediaContract(candidate)
		sources := map[string]bool{}
		for _, source := range contract.Sources {
			sources[source.ID] = true
		}
		for _, fact := range providermodels.MediaCandidateOperationFacts(id) {
			if !sources[fact.SourceID] {
				t.Fatalf("%s operation %s source %s absent from contract sources", id, fact.ID, fact.SourceID)
			}
		}
		if _, ok := sources["suno_vox"]; ok {
			t.Fatalf("%s includes deprecated vox source", id)
		}
	}
}

func findOperationFact(publicID, opID string) (providermodels.MediaCandidateOperationFact, bool) {
	for _, fact := range providermodels.MediaCandidateOperationFacts(publicID) {
		if fact.ID == opID {
			return fact, true
		}
	}
	return providermodels.MediaCandidateOperationFact{}, false
}
