package providermodels

import (
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/modelcontract"
)

func TestOnboardingRejectsNewOrChangedUnverifiedModels(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Registry)
	}{
		{"new image", func(r *Registry) { r.ImageModels[0].PublicID = "new_image" }},
		{"provider model change", func(r *Registry) { r.ImageModels[0].ProviderModelID = "different-version" }},
		{"capability change", func(r *Registry) { r.ImageModels[0].Limits.MaxReferenceImages++ }},
		{"legacy ratio declaration change", func(r *Registry) {
			for i := range r.ImageModels {
				if r.ImageModels[i].PublicID == PublicImageQwenImage3 {
					r.ImageModels[i].Limits.AllowedAspectRatios = []string{"1:1", "21:9"}
				}
			}
		}},
		{"text provider change", func(r *Registry) { r.TextAliases[0].Provider = "different" }},
		{"video duration change", func(r *Registry) { r.VideoRouteModels[0].Limits.AllowedDurationsSec = []int{99} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := StaticRegistry()
			tc.change(&r)
			if err := r.Validate(); err == nil {
				t.Fatal("model accepted without onboarding evidence")
			}
		})
	}
}

func TestExistingRegistryHasExplicitMigrationBaseline(t *testing.T) {
	if err := StaticRegistry().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestAudioRegistryCannotBypassOnboarding(t *testing.T) {
	r := StaticRegistry()
	r.AudioModels = []AudioModel{{PublicID: "new_audio", Provider: "synthetic", ProviderModelID: "new-audio-v1"}}
	if err := r.ValidateOnboarding(); err == nil || !strings.Contains(err.Error(), "audio/new_audio requires a verified onboarding contract") {
		t.Fatalf("audio declaration bypassed admission: %v", err)
	}
}

// Synthetic evidence tests admission mechanics only; it is never registered or
// presented as verification of the real provider model used as a shape fixture.
func imageAdmissionFixture(r Registry) modelcontract.Contract {
	m := r.ImageModels[0]
	var fingerprint string
	for _, b := range r.Bindings() {
		if b.PublicID == m.PublicID {
			fingerprint = b.Fingerprint
		}
	}
	variants := []modelcontract.ImageVariant{}
	for _, quality := range m.Limits.AllowedQualities {
		for _, ratio := range m.Limits.AllowedAspectRatios {
			variants = append(variants, modelcontract.ImageVariant{Resolution: quality, AspectRatio: ratio})
		}
	}
	c := modelcontract.Contract{SchemaVersion: 1, PublicID: m.PublicID, Provider: string(m.Provider), ProviderModelID: m.ProviderModelID, Revision: "synthetic-v1", Endpoint: "POST /synthetic", RegistryFingerprint: fingerprint, Status: "ready", Categories: []string{"images"}, Sources: []modelcontract.Source{{ID: "synthetic", URL: "https://provider.example/docs", CheckedAt: "2026-09-14"}}, Operations: []modelcontract.Operation{{ID: "generate", Kind: "image", Inputs: modelcontract.Inputs{Images: modelcontract.Input{Support: "unknown"}, Video: modelcontract.Input{Support: "unknown"}, Audio: modelcontract.Input{Support: "unknown"}, Documents: modelcontract.Input{Support: "unknown"}}, Image: &modelcontract.ImageOutput{Variants: variants, MaxOutputCount: m.Limits.MaxOutputCount, Formats: []string{"image/png"}, Mask: "unsupported"}}}}
	for _, name := range []string{"adapter", "negative", "boundaries", "pricing", "job-lifecycle", "catalog", "live-output"} {
		c.Checks = append(c.Checks, modelcontract.Check{Scenario: "generate/" + name, Status: "passed", Evidence: "synthetic.md", EvidenceSHA256: strings.Repeat("b", 64), CheckedAt: "2026-09-14", ContractDigest: c.Digest()})
	}
	return c
}

func TestContractAdmissionChecksBindingAndRuntimeLimits(t *testing.T) {
	r := StaticRegistry()
	c := imageAdmissionFixture(r)
	r.Contracts[c.PublicID] = c
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	c.Operations[0].Image.MaxOutputCount++
	for i := range c.Checks {
		c.Checks[i].ContractDigest = c.Digest()
	}
	r.Contracts[c.PublicID] = c
	if err := r.Validate(); err == nil || !strings.Contains(err.Error(), "counts differ") {
		t.Fatalf("inconsistent capabilities admitted: %v", err)
	}
	c = imageAdmissionFixture(r)
	c.RegistryFingerprint = strings.Repeat("0", 64)
	r.Contracts[c.PublicID] = c
	if err := r.Validate(); err == nil {
		t.Fatal("mismatched registry binding admitted")
	}
}

func TestInvalidContractCannotFallBackToLegacyExemption(t *testing.T) {
	r := StaticRegistry()
	c := imageAdmissionFixture(r)
	c.Status = "draft"
	r.Contracts[c.PublicID] = c
	if err := r.Validate(); err == nil {
		t.Fatal("draft fell back to migration baseline")
	}
}

func TestVideoContractCannotAdvertiseUnsupportedDuration(t *testing.T) {
	r := StaticRegistry()
	m := r.VideoRouteModels[0]
	c := modelcontract.Contract{PublicID: string(m.Alias), Operations: []modelcontract.Operation{{ID: "video", Kind: "video", Video: &modelcontract.VideoOutput{Variants: []modelcontract.VideoVariant{{DurationSec: 999999, Resolution: "unsupported", AspectRatio: "1:1", FPS: 24}}}}}}
	if err := r.validateContractLimits(c, "video"); err == nil {
		t.Fatal("unsupported video parameters admitted")
	}
}

func TestImageContractMustCoverRuntimeCrossProduct(t *testing.T) {
	r := StaticRegistry()
	c := imageAdmissionFixture(r)
	m := r.ImageModels[0]
	c.Operations[0].Image.Variants = nil
	for i, quality := range m.Limits.AllowedQualities {
		c.Operations[0].Image.Variants = append(c.Operations[0].Image.Variants, modelcontract.ImageVariant{Resolution: quality, AspectRatio: m.Limits.AllowedAspectRatios[i%len(m.Limits.AllowedAspectRatios)]})
	}
	if err := r.validateContractLimits(c, "image"); err == nil {
		t.Fatal("partial diagonal coverage accepted while runtime accepts all pairs")
	}
}

func TestVideoContractMustCoverRuntimeCrossProduct(t *testing.T) {
	r := Registry{VideoRouteModels: []VideoRoute{{Alias: "synthetic", Spec: domain.VideoRouteSpec{AllowedDurationsSec: []int{5, 10}, AllowedResolutions: []string{"720p", "1080p"}, AllowedAspectRatios: []string{"16:9"}, SupportsAudio: true}}}}
	c := modelcontract.Contract{PublicID: "synthetic", Operations: []modelcontract.Operation{{Kind: "video", Video: &modelcontract.VideoOutput{StartImage: "unsupported", EndImage: "unsupported", Variants: []modelcontract.VideoVariant{{DurationSec: 5, Resolution: "720p", AspectRatio: "16:9", FPS: 24}}}}}}
	if err := r.validateContractLimits(c, "video"); err == nil {
		t.Fatal("partial video coverage accepted")
	}
	c.Operations[0].Video.Variants = nil
	for _, res := range []string{"720p", "1080p"} {
		for _, seconds := range []int{5, 10} {
			for _, audio := range []bool{false, true} {
				c.Operations[0].Video.Variants = append(c.Operations[0].Video.Variants, modelcontract.VideoVariant{DurationSec: seconds, Resolution: res, AspectRatio: "16:9", FPS: 24, Audio: audio})
			}
		}
	}
	if err := r.validateContractLimits(c, "video"); err != nil {
		t.Fatal(err)
	}
}

func TestAliasRemappingNeedsReverification(t *testing.T) {
	r := StaticRegistry()
	r.ModelAliases[0].Alias = domain.VideoRouteRunwayGen4Turbo
	if err := r.Validate(); err == nil {
		t.Fatal("alias silently redirected without onboarding")
	}
}

func TestVideoReferenceCountsAreNotJustAMaximum(t *testing.T) {
	r := Registry{VideoRouteModels: []VideoRoute{{Alias: "synthetic", Spec: domain.VideoRouteSpec{AllowedDurationsSec: []int{5}, AllowedResolutions: []string{"720p"}, AllowedAspectRatios: []string{"16:9"}, SupportsReferenceImage: true, MaxReferenceImages: 2, AllowedReferenceImageCounts: []int{0, 2}}}}}
	c := modelcontract.Contract{PublicID: "synthetic", Operations: []modelcontract.Operation{{Kind: "video", Inputs: modelcontract.Inputs{Images: modelcontract.Input{Enabled: true, MaxCount: 2}}, Video: &modelcontract.VideoOutput{StartImage: "optional", EndImage: "optional", Variants: []modelcontract.VideoVariant{{DurationSec: 5, Resolution: "720p", AspectRatio: "16:9", FPS: 24}}}}}}
	if err := r.validateContractLimits(c, "video"); err == nil {
		t.Fatal("contract allowed unsupported one-image request")
	}
	c.Operations[0].Inputs.Images.AllowedCounts = []int{0, 2}
	if err := r.validateContractLimits(c, "video"); err != nil {
		t.Fatal(err)
	}
}

func TestVideoContractCoversSparseResolutionDurations(t *testing.T) {
	r := Registry{VideoRouteModels: []VideoRoute{{Alias: "synthetic", Spec: domain.VideoRouteSpec{AllowedDurationsSec: []int{5, 10}, AllowedResolutions: []string{"720p", "1080p"}, AllowedAspectRatios: []string{"16:9"}, ResolutionDurationsSec: map[string][]int{" 1080P ": {5}}}}}}
	c := modelcontract.Contract{PublicID: "synthetic", Operations: []modelcontract.Operation{{Kind: "video", Video: &modelcontract.VideoOutput{Variants: []modelcontract.VideoVariant{
		{DurationSec: 5, Resolution: "720p", AspectRatio: "16:9"},
		{DurationSec: 10, Resolution: "720p", AspectRatio: "16:9"},
		{DurationSec: 5, Resolution: "1080p", AspectRatio: "16:9"},
	}}}}}
	if err := r.validateContractLimits(c, "video"); err != nil {
		t.Fatalf("runtime-supported combinations rejected: %v", err)
	}
	c.Operations[0].Video.Variants = c.Operations[0].Video.Variants[2:]
	if err := r.validateContractLimits(c, "video"); err == nil {
		t.Fatal("missing fallback-resolution combinations accepted")
	}
}

func TestProviderAliasCollisionsUseRuntimeNormalization(t *testing.T) {
	for _, tc := range []struct {
		name    string
		aliases []ProviderModelAlias
	}{
		{"primary model", []ProviderModelAlias{{Alias: "second", ProviderModelID: " FIRST-MODEL "}}},
		{"other alias", []ProviderModelAlias{{Alias: "first", ProviderModelID: "shared"}, {Alias: "second", ProviderModelID: " SHARED "}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Registry{VideoRouteModels: []VideoRoute{{Alias: "first", Spec: domain.VideoRouteSpec{ProviderModelID: "first-model"}}, {Alias: "second", Spec: domain.VideoRouteSpec{ProviderModelID: "second-model"}}}, ModelAliases: tc.aliases}
			if err := r.ValidateOnboarding(); err == nil || !strings.Contains(err.Error(), "collision") {
				t.Fatalf("routing collision was not identified: %v", err)
			}
		})
	}
}

func TestVideoReferenceInputCountMatchesSingleArtifact(t *testing.T) {
	for _, required := range []bool{false, true} {
		r := Registry{VideoRouteModels: []VideoRoute{{Alias: "synthetic", Spec: domain.VideoRouteSpec{AllowedDurationsSec: []int{5}, AllowedResolutions: []string{"720p"}, AllowedAspectRatios: []string{"16:9"}, SupportsReferenceVideo: true, RequiresReferenceVideo: required}}}}
		c := modelcontract.Contract{PublicID: "synthetic", Operations: []modelcontract.Operation{{Kind: "video", Inputs: modelcontract.Inputs{Video: modelcontract.Input{Enabled: true, Required: required, MaxCount: 1}}, Video: &modelcontract.VideoOutput{Variants: []modelcontract.VideoVariant{{DurationSec: 5, Resolution: "720p", AspectRatio: "16:9"}}}}}}
		if !required {
			if err := r.validateContractLimits(c, "video"); err == nil || !strings.Contains(err.Error(), "optional video reference") {
				t.Fatalf("unimplemented optional reference video admitted: %v", err)
			}
			continue
		}
		if err := r.validateContractLimits(c, "video"); err != nil {
			t.Fatalf("single video rejected (required=%v): %v", required, err)
		}
		c.Operations[0].Inputs.Video.MaxCount = 2
		if err := r.validateContractLimits(c, "video"); err == nil {
			t.Fatal("multiple reference videos accepted by single-artifact API")
		}
		c.Operations[0].Inputs.Video.MaxCount = 1
		c.Operations[0].Inputs.Video.AllowedCounts = []int{0}
		if err := r.validateContractLimits(c, "video"); err == nil {
			t.Fatal("incorrect reference video counts accepted")
		}
	}
}
