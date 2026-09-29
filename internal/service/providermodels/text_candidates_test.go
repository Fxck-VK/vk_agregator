package providermodels

import "testing"

func TestTextCandidatesOnlyInDEVRuntime(t *testing.T) {
	if err := ConfigureDEVSmoke("development", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ConfigureDEVSmoke("development", false) })
	r := RuntimeRegistry()
	if _, ok := r.TextAlias("gpt_5"); !ok {
		t.Fatal("DEV text route missing")
	}
	if !r.IsDEVSmokeModel("gpt_5") {
		t.Fatal("missing manual verification status")
	}
	if r.MediaCandidateRunnable("gpt_5", "generate") {
		t.Fatal("text route exposed as media")
	}
	if _, ok := StaticRegistry().TextAlias("gpt_5"); ok {
		t.Fatal("unverified text added to static registry")
	}
	for i := range r.TextAliases {
		if r.TextAliases[i].PublicID == "gpt_5" {
			r.TextAliases[i].ProviderModelID = "unverified-version"
		}
	}
	if r.ValidateOnboarding() == nil {
		t.Fatal("changed binding accepted")
	}
	if err := RuntimeRegistry().Validate(); err != nil {
		t.Fatalf("copy corrupted runtime: %v", err)
	}
}

func TestTextCandidateContractsRemainHonestDrafts(t *testing.T) {
	seen := map[string]bool{}
	for _, candidate := range TextCandidates() {
		if seen[candidate.PublicID] {
			t.Fatal("duplicate public model")
		}
		seen[candidate.PublicID] = true
		c := DraftTextContract(candidate)
		if c.ValidateReady() == nil || c.Status != "draft" || len(c.Sources) < 3 || c.RegistryFingerprint != "" {
			t.Fatal("candidate forged admission")
		}
		for _, check := range c.Checks {
			if check.Status != "not_run" || check.CheckedAt != "" {
				t.Fatal("fabricated verification")
			}
		}
		for _, source := range c.Sources {
			if source.CheckedAt == "" || source.CheckedAt != candidate.CheckedAt {
				t.Fatal("undated source")
			}
		}
		if len(c.Operations) != 1 || c.Operations[0].ID != "reply" || c.Operations[0].Inputs.Images.Enabled || c.Operations[0].Inputs.Documents.Enabled {
			t.Fatal("unwired inputs enabled")
		}
	}
	if len(seen) != 34 {
		t.Fatal("incomplete documented list")
	}
}

func TestStudy24TextVersionsNeverSubstituteUnlistedModels(t *testing.T) {
	for _, id := range []string{"claude_sonnet_5_5", "gemini_3_1_flash"} {
		if _, ok := TextCandidateByID(id); ok {
			t.Fatalf("unlisted model %s enabled", id)
		}
	}
	qwen, ok := TextCandidateByID("qwen_3_8_max")
	if !ok {
		t.Fatal("missing Qwen")
	}
	c := DraftTextContract(qwen)
	if c.Endpoint != "POST /v1/responses" || c.Revision != "2026-09-29" || len(c.Sources) != 5 {
		t.Fatal("Qwen contract lost model-specific protocol evidence")
	}
	old, _ := TextCandidateByID("gpt_5")
	if DraftTextContract(old).Revision != "2026-09-28" {
		t.Fatal("old evidence dates refreshed without re-verification")
	}
}
