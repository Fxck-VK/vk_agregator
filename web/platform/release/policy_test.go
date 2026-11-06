package releasepolicy

import "testing"

func TestProfileOnlyTrustedDevelopmentCanUseDEV(t *testing.T) {
	for _, tc := range []struct{ environment, requested, want string }{
		{"development", "", "dev"}, {"development", "dev", "dev"}, {"development", "prod", "prod"},
		{"production", "dev", "prod"}, {"production", "", "prod"}, {"", "dev", "prod"},
		{"staging", "dev", "prod"}, {"Development", "dev", "prod"},
	} {
		if got := Profile(tc.environment, tc.requested); got != tc.want {
			t.Errorf("Profile(%q,%q)=%q, want %q", tc.environment, tc.requested, got, tc.want)
		}
	}
}

func TestFeatureInventoryRestrictsProductionAndUnknownIDs(t *testing.T) {
	if !Enabled("images", "prod") || !Enabled("music", "dev") {
		t.Fatal("known enabled feature rejected")
	}
	for _, profile := range []string{"prod", "dev", "", "invalid"} {
		if Enabled("unregistered", profile) {
			t.Fatalf("unknown feature enabled in %q", profile)
		}
	}
	for _, profile := range []string{"prod", "", "invalid"} {
		if Enabled("music", profile) || Enabled("transcription", profile) || Enabled("tts", profile) {
			t.Fatalf("DEV-only feature enabled in %q", profile)
		}
	}
}
