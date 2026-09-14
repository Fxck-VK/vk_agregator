package providermodels

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/modelcontract"
)

// Migration records preserve only unchanged pre-onboarding definitions. They
// are NOT capability verification and must never be renewed to admit a change.
//
//go:embed onboarding/legacy.json onboarding/approved
var onboardingFiles embed.FS

type Binding struct {
	Kind            string `json:"kind"`
	PublicID        string `json:"public_id"`
	Provider        string `json:"provider"`
	ProviderModelID string `json:"provider_model_id"`
	Fingerprint     string `json:"fingerprint"`
}

func (r Registry) Bindings() []Binding {
	var bindings []Binding
	add := func(kind, id, provider, model string, value any) {
		data, _ := json.Marshal(value)
		digest := sha256.Sum256(data)
		bindings = append(bindings, Binding{kind, id, provider, model, hex.EncodeToString(digest[:])})
	}
	for _, m := range r.TextAliases {
		add("text", m.PublicID, string(m.Provider), m.ProviderModelID, m)
	}
	for _, m := range r.ImageModels {
		add("image", m.PublicID, string(m.Provider), m.ProviderModelID, m)
	}
	for _, m := range r.LoadTestImageModels {
		add("image", m.PublicID, string(m.Provider), m.ProviderModelID, m)
	}
	for _, m := range r.VideoRouteModels {
		var aliases []ProviderModelAlias
		for _, alias := range r.ModelAliases {
			if alias.Alias == m.Alias {
				aliases = append(aliases, alias)
			}
		}
		add("video", string(m.Alias), string(m.Provider), m.ProviderModelID, struct {
			Route   VideoRoute
			Aliases []ProviderModelAlias
		}{m, aliases})
	}
	return bindings
}

func loadOnboardingContracts() (map[string]modelcontract.Contract, error) {
	entries, err := onboardingFiles.ReadDir("onboarding/approved")
	if err != nil {
		return nil, err
	}
	reports, err := fs.Sub(onboardingFiles, "onboarding/approved")
	if err != nil {
		return nil, err
	}
	contracts := map[string]modelcontract.Contract{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := onboardingFiles.ReadFile("onboarding/approved/" + entry.Name())
		if err != nil {
			return nil, err
		}
		c, err := modelcontract.Decode(strings.NewReader(string(data)))
		if err != nil {
			return nil, fmt.Errorf("onboarding %s: %w", entry.Name(), err)
		}
		if err := c.ValidateEvidenceFiles(reports); err != nil {
			return nil, fmt.Errorf("onboarding %s: %w", entry.Name(), err)
		}
		if _, duplicate := contracts[c.PublicID]; duplicate {
			return nil, fmt.Errorf("duplicate onboarding contract %s", c.PublicID)
		}
		contracts[c.PublicID] = c
	}
	return contracts, nil
}

func (r Registry) ValidateOnboarding() error {
	if r.onboardingError != nil {
		return r.onboardingError
	}
	modelIDs := map[string]domain.VideoRouteAlias{}
	for _, route := range r.VideoRouteModels {
		key := strings.ToLower(strings.TrimSpace(route.Spec.ProviderModelID))
		if existing, exists := modelIDs[key]; exists {
			return fmt.Errorf("provider model ID collision %q for %s and %s", key, existing, route.Alias)
		}
		modelIDs[key] = route.Alias
	}
	for _, alias := range r.ModelAliases {
		key := strings.ToLower(strings.TrimSpace(alias.ProviderModelID))
		if _, ok := r.VideoRoute(alias.Alias); !ok || key == "" {
			return fmt.Errorf("invalid provider model alias %q", alias.ProviderModelID)
		}
		if existing, exists := modelIDs[key]; exists && existing != alias.Alias {
			return fmt.Errorf("provider model alias collision %q for %s and %s", key, existing, alias.Alias)
		}
		modelIDs[key] = alias.Alias
	}
	data, err := onboardingFiles.ReadFile("onboarding/legacy.json")
	if err != nil {
		return err
	}
	var baseline map[string]string
	if err := json.Unmarshal(data, &baseline); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, b := range r.Bindings() {
		if seen[b.PublicID] {
			return fmt.Errorf("providermodels: public ID %s is duplicated across model types", b.PublicID)
		}
		seen[b.PublicID] = true
		c, exists := r.Contracts[b.PublicID]
		if !exists {
			if baseline[b.Kind+"/"+b.PublicID] == b.Fingerprint {
				continue
			}
			return fmt.Errorf("providermodels: %s/%s requires a verified onboarding contract (new or changed definition)", b.Kind, b.PublicID)
		}
		if c.PublicID != b.PublicID || c.Provider != b.Provider || c.ProviderModelID != b.ProviderModelID || c.RegistryFingerprint != b.Fingerprint {
			return fmt.Errorf("providermodels: onboarding binding changed for %s", b.PublicID)
		}
		if err := c.ValidateReady(); err != nil {
			return fmt.Errorf("providermodels: %s: %w", b.PublicID, err)
		}
		foundKind := false
		for _, op := range c.Operations {
			if op.Kind == b.Kind {
				foundKind = true
			}
		}
		if !foundKind {
			return fmt.Errorf("providermodels: %s missing %s capabilities", b.PublicID, b.Kind)
		}
		if err := r.validateContractLimits(c, b.Kind); err != nil {
			return err
		}
	}
	for id := range r.Contracts {
		if !seen[id] {
			return fmt.Errorf("orphan onboarding contract %s", id)
		}
	}
	return nil
}

func (r Registry) validateContractLimits(c modelcontract.Contract, kind string) error {
	if kind == "video" {
		return r.validateVideoContract(c)
	}
	if kind != "image" {
		return nil
	}
	m, ok := r.PublicImageModel(c.PublicID)
	if !ok {
		return nil
	}
	for _, op := range c.Operations {
		if op.Image == nil {
			continue
		}
		if op.Inputs.Images.Enabled != m.Limits.SupportsReferenceImage || (op.Inputs.Images.Enabled && op.Inputs.Images.MaxCount != m.Limits.MaxReferenceImages) || op.Image.MaxOutputCount != m.Limits.MaxOutputCount {
			return fmt.Errorf("onboarding image counts differ from registry for %s", c.PublicID)
		}
		if op.Inputs.Images.Enabled && !slices.Equal(inputCounts(op.Inputs.Images, 0), integerRange(0, m.Limits.MaxReferenceImages)) {
			return fmt.Errorf("onboarding image reference counts differ for %s", c.PublicID)
		}
		qualities := map[string]bool{}
		ratios := map[string]bool{}
		pairs := map[modelcontract.ImageVariant]bool{}
		for _, v := range op.Image.Variants {
			qualities[v.Resolution] = true
			ratios[v.AspectRatio] = true
			pairs[v] = true
		}
		if !sameChoices(qualities, m.Limits.AllowedQualities) || !sameChoices(ratios, m.Limits.AllowedAspectRatios) || len(pairs) != len(qualities)*len(ratios) {
			return fmt.Errorf("onboarding image options differ from registry for %s", c.PublicID)
		}
	}
	return nil
}

func (r Registry) validateVideoContract(c modelcontract.Contract) error {
	route, ok := r.VideoRoute(domain.VideoRouteAlias(c.PublicID))
	if !ok {
		return fmt.Errorf("onboarding video route %s missing", c.PublicID)
	}
	spec := route.Spec
	for _, op := range c.Operations {
		if op.Video == nil {
			continue
		}
		if op.Inputs.Images.Enabled != spec.SupportsReferenceImage || op.Inputs.Images.Enabled && op.Inputs.Images.MaxCount != spec.MaxReferenceImages {
			return fmt.Errorf("onboarding video image limits differ for %s", c.PublicID)
		}
		if op.Inputs.Video.Enabled != spec.SupportsReferenceVideo || op.Inputs.Audio.Enabled != spec.SupportsReferenceAudio || (op.Video.StartImage == "required") != spec.RequiresStartImage {
			return fmt.Errorf("onboarding video input modes differ for %s", c.PublicID)
		}
		if op.Inputs.Video.Required != spec.RequiresReferenceVideo {
			return fmt.Errorf("onboarding video reference requirement differs for %s", c.PublicID)
		}
		if op.Inputs.Video.Enabled {
			// Resolve currently accepts one reference_video_artifact_id only on
			// routes that require it; optional reference video is not implemented.
			if !spec.RequiresReferenceVideo {
				return fmt.Errorf("onboarding optional video reference is not supported by runtime for %s", c.PublicID)
			}
			if op.Inputs.Video.MaxCount != 1 || !slices.Equal(inputCounts(op.Inputs.Video, 1), []int{1}) {
				return fmt.Errorf("onboarding video reference counts differ for %s", c.PublicID)
			}
		}
		if op.Inputs.Images.Enabled {
			minimum := 0
			if spec.RequiresStartImage {
				minimum = 1
			}
			want := append([]int(nil), spec.AllowedReferenceImageCounts...)
			if len(want) == 0 {
				want = integerRange(minimum, spec.MaxReferenceImages)
			}
			slices.Sort(want)
			if !slices.Equal(inputCounts(op.Inputs.Images, minimum), want) {
				return fmt.Errorf("onboarding video image counts differ for %s", c.PublicID)
			}
		}
		expected := map[modelcontract.VideoVariant]bool{}
		for _, resolution := range spec.AllowedResolutions {
			durations := spec.AllowedDurationsSec
			for candidate, allowed := range spec.ResolutionDurationsSec {
				if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(resolution)) {
					durations = nil
					for _, duration := range spec.AllowedDurationsSec {
						if slices.Contains(allowed, duration) {
							durations = append(durations, duration)
						}
					}
					break
				}
			}
			for _, duration := range durations {
				for _, ratio := range spec.AllowedAspectRatios {
					v := modelcontract.VideoVariant{Resolution: resolution, DurationSec: duration, AspectRatio: ratio}
					expected[v] = true
					if spec.SupportsAudio {
						v.Audio = true
						expected[v] = true
					}
				}
			}
		}
		if len(expected) == 0 {
			return fmt.Errorf("onboarding %s: dynamic video options need an explicit contract schema", c.PublicID)
		}
		covered := map[modelcontract.VideoVariant]bool{}
		for _, variant := range op.Video.Variants {
			// The current route API does not expose FPS as an input parameter.
			// Contract FPS is checked output metadata, not another runtime option.
			variant.FPS = 0
			if !expected[variant] {
				return fmt.Errorf("onboarding video combination differs from registry for %s", c.PublicID)
			}
			covered[variant] = true
		}
		if len(covered) != len(expected) {
			return fmt.Errorf("onboarding video combinations do not cover runtime options for %s", c.PublicID)
		}
	}
	return nil
}

func sameChoices(got map[string]bool, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for _, value := range want {
		if !got[value] {
			return false
		}
	}
	return true
}

func inputCounts(in modelcontract.Input, minimum int) []int {
	if len(in.AllowedCounts) > 0 {
		counts := append([]int(nil), in.AllowedCounts...)
		slices.Sort(counts)
		return counts
	}
	if in.Required && minimum == 0 {
		minimum = 1
	}
	return integerRange(minimum, in.MaxCount)
}

func integerRange(minimum, maximum int) []int {
	var values []int
	for n := minimum; n <= maximum; n++ {
		values = append(values, n)
	}
	return values
}
