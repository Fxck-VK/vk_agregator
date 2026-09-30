package providermodels

import (
	"testing"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

func TestDEVSmokeRegistryDoesNotGrantAdmission(t *testing.T) {
	if err := ConfigureDEVSmoke("production", true); err == nil {
		t.Fatal("production accepted smoke mode")
	}
	if err := ConfigureDEVSmoke("loadtest", true); err == nil {
		t.Fatal("paid routes accepted in loadtest")
	}
	if err := ConfigureDEVSmoke("development", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ConfigureDEVSmoke("development", false) })
	r := RuntimeRegistry()
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := StaticRegistry().Validate(); err != nil {
		t.Fatal(err)
	}
	for _, c := range MediaCandidates() {
		op := "generate"
		if c.Kind == "video" {
			op = "text_to_video"
		}
		want := c.PublicID != "gpt_4o_mini_tts" && c.PublicID != "whisper_1"
		if got := r.MediaCandidateRunnable(c.PublicID, op); got != want {
			t.Errorf("%s runnable=%t want %t", c.PublicID, got, want)
		}
		if r.MediaCandidateAdmitted(c.PublicID, op) || StaticRegistry().MediaCandidateRunnable(c.PublicID, op) {
			t.Errorf("%s smoke forged admission", c.PublicID)
		}
		if r.MediaCandidateRunnable(c.PublicID, "unpriced") {
			t.Fatal("unknown operation enabled")
		}
	}
	for _, id := range []string{"suno_v6", "suno_v6_wild", "suno_v6_mini", "lyria_3_5"} {
		if r.MediaCandidateRunnable(id, "upload_cover") {
			t.Fatal("unverified input operation enabled")
		}
	}
	prices, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if err := prices.AddSupplemental(r.DEVSmokePrices()); err != nil {
		t.Fatal(err)
	}
	if err := r.ValidatePricingCoverage(prices, pricingcatalog.DisabledStaticProductPrices()); err != nil {
		t.Fatal(err)
	}
	// Runtime settings cannot alter the pinned smoke definition and retain admission.
	r.ImageModels[len(r.ImageModels)-1].Limits.MaxOutputCount = 999
	if err := r.ValidateOnboarding(); err == nil {
		t.Fatal("changed smoke binding accepted")
	}
}
