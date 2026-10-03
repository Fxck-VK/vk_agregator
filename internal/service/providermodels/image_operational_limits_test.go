package providermodels

import (
	"reflect"
	"slices"
	"testing"
)

func TestPublicImageOperationalLimitsPreserveAdmission(t *testing.T) {
	r := StaticRegistry()
	before := r.Bindings()
	for _, id := range []string{PublicImageNanoBanana2, PublicImageNanoBananaPro, PublicImageGPTImage2} {
		m, ok := r.PublicImageModel(id)
		if !ok || m.Limits.MaxOutputCount != 1 {
			t.Errorf("%s public output limit = %d, want 1", id, m.Limits.MaxOutputCount)
		}
		if id == PublicImageGPTImage2 && m.Limits.MaxReferenceImages != 15 {
			t.Errorf("GPT public references = %d, want 15", m.Limits.MaxReferenceImages)
		}
		if id == PublicImageGPTImage2 && ImageReferenceTotalByteLimit(id) != 256<<20 {
			t.Errorf("GPT public reference bytes = %d, want %d", ImageReferenceTotalByteLimit(id), int64(256<<20))
		}
		caps := Capabilities(id)
		if caps.Application.Image.MaxOutputCount == nil || *caps.Application.Image.MaxOutputCount != 1 {
			t.Errorf("%s application capabilities expose multiple results", id)
		}
		if id == PublicImageNanoBanana2 && slices.Contains(caps.API.Image.AspectRatios, "1:8") {
			t.Error("Nano Banana 2 new route exposes official-family extreme ratios")
		}
	}
	if !reflect.DeepEqual(before, r.Bindings()) {
		t.Fatal("operational projection mutated frozen admission definitions")
	}
	if err := r.Validate(); err != nil {
		t.Fatalf("existing admission must remain valid: %v", err)
	}
}
