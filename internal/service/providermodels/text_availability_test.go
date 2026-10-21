package providermodels

import "testing"

func TestDEVTextRuntimeExcludesUnavailableExactVersions(t *testing.T) {
	if err := ConfigureDEVSmoke("development", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ConfigureDEVSmoke("development", false) })
	unavailable := map[string]bool{
		"gpt_5_chat_latest":  true,
		"deepseek_v3_2":      true,
		"deepseek_v3_2_exp":  true,
		"deepseek_r1_250528": true,
		"deepseek_v3_0324":   true,
	}
	runtime := RuntimeRegistry()
	available := 0
	for _, candidate := range TextCandidates() {
		_, present := runtime.TextAlias(candidate.PublicID)
		want := !unavailable[candidate.PublicID]
		if present != want || runtime.IsDEVSmokeModel(candidate.PublicID) != want {
			t.Errorf("%s runtime available=%t, want %t", candidate.PublicID, present, want)
		}
		if present {
			available++
		}
		if _, known := PaidTextModel(candidate.PublicID); !known {
			t.Errorf("exact definition lost for %s", candidate.PublicID)
		}
	}
	if available != 29 {
		t.Errorf("available DEV text candidates=%d, want 29", available)
	}
	for _, price := range runtime.DEVSmokePrices() {
		if unavailable[price.Key.TextModelID] {
			t.Errorf("unavailable text candidate has active DEV price: %s", price.Key.TextModelID)
		}
	}
}
