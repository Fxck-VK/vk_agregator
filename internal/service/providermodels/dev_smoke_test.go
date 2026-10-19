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
			if VideoCandidateRequiresImages(c.PublicID) {
				op = "reference_image_to_video"
			}
		}
		if c.PublicID == "whisper_1" {
			op = "transcribe"
		}
		want := c.PublicID != "gpt_4o_mini_tts"
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
	if r.MediaCandidateRunnable("lyria_3_5", "upload_cover") {
		t.Fatal("Suno input operation enabled for Lyria")
	}
	for _, op := range []string{"generate", "speak", "translate"} {
		if r.MediaCandidateRunnable("whisper_1", op) {
			t.Fatalf("unsupported Whisper operation enabled: %s", op)
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

func TestDEVSmokeSunoKeepsUnpreparedToolsClosed(t *testing.T) {
	if err := ConfigureDEVSmoke("development", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ConfigureDEVSmoke("development", false) })
	for _, id := range []string{"suno_v6", "suno_v6_wild", "suno_v6_mini"} {
		if !RuntimeRegistry().MediaCandidateRunnable(id, "extend") {
			t.Errorf("%s continuation is not wired on DEV", id)
		}
		if StaticRegistry().MediaCandidateRunnable(id, "extend") || RuntimeRegistry().MediaCandidateAdmitted(id, "extend") {
			t.Fatal("DEV continuation changed permanent admission")
		}
		for _, operation := range []string{"create_model", "voice", "persona", "midi", "export"} {
			if RuntimeRegistry().MediaCandidateRunnable(id, operation) {
				t.Fatalf("unrequested operation %s became runnable", operation)
			}
		}
	}
	if RuntimeRegistry().MediaCandidateRunnable("lyria_3_5", "extend") {
		t.Fatal("Lyria continuation is unsupported")
	}
}
