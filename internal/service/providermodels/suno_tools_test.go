package providermodels

import "testing"

func TestSunoToolsAvailableOnlyInDEV(t *testing.T) {
	if err := ConfigureDEVSmoke("development", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ConfigureDEVSmoke("development", false) })
	if !MusicReferencesRunnable() || MusicReferencesAdmitted() {
		t.Fatal("DEV uploads must start signed references without granting admission")
	}
	for _, id := range []string{"suno_v6", "suno_v6_wild", "suno_v6_mini"} {
		for _, action := range []string{"lyrics", "sounds", "cover", "mashup", "stems", "stems_all", "add_vocals", "add_instrumental", "upload", "upload_cover", "upload_extend", "upsample_tags"} {
			if !RuntimeRegistry().MediaCandidateRunnable(id, action) {
				t.Errorf("DEV tool unavailable: %s/%s", id, action)
			}
			if StaticRegistry().MediaCandidateRunnable(id, action) || RuntimeRegistry().MediaCandidateAdmitted(id, action) {
				t.Fatalf("permanent admission changed: %s/%s", id, action)
			}
			if RuntimeRegistry().MediaCandidateRunnable("lyria_3_5", action) {
				t.Fatalf("Suno tool leaked into Lyria: %s", action)
			}
		}
	}
}
