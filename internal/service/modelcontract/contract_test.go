package modelcontract

import (
	"strings"
	"testing"
)

func readyTextContract() Contract {
	c := Contract{
		SchemaVersion: 1, PublicID: "example_text", Provider: "example", ProviderModelID: "text-v1",
		Revision: "v1", Endpoint: "POST /text", RegistryFingerprint: strings.Repeat("a", 64), Status: "ready",
		Categories: []string{"text", "study-work"},
		Sources:    []Source{{ID: "api", URL: "https://provider.example/docs/text-v1", CheckedAt: "2026-09-14"}},
		Operations: []Operation{{ID: "reply", Kind: "text", Inputs: Inputs{Images: Input{Support: "unknown"}, Video: Input{Support: "unsupported"}, Audio: Input{Support: "unsupported"}, Documents: Input{Support: "unsupported"}}, Text: &TextOutput{ContextTokens: 1000, MaxOutputTokens: 200}}},
	}
	for _, scenario := range []string{"reply/adapter", "reply/negative", "reply/boundaries", "reply/pricing", "reply/job-lifecycle", "reply/catalog", "reply/live-output"} {
		c.Checks = append(c.Checks, Check{Scenario: scenario, Status: "passed", Evidence: "synthetic-verification.md", EvidenceSHA256: strings.Repeat("b", 64), CheckedAt: "2026-09-14", ContractDigest: c.Digest()})
	}
	return c
}

func TestReadyContractRequiresCurrentEvidence(t *testing.T) {
	c := readyTextContract()
	if err := c.ValidateReady(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*Contract)
	}{
		{"missing check", func(c *Contract) { c.Checks = c.Checks[1:] }},
		{"failed check", func(c *Contract) { c.Checks[0].Status = "failed" }},
		{"missing report", func(c *Contract) { c.Checks[0].Evidence = "" }},
		{"check predates researched contract", func(c *Contract) { c.Checks[0].CheckedAt = "2020-01-01" }},
		{"stale after limit change", func(c *Contract) { c.Operations[0].Text.MaxOutputTokens = 201 }},
		{"stale after provider change", func(c *Contract) { c.Provider = "different" }},
		{"draft", func(c *Contract) { c.Status = "draft" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := readyTextContract()
			tc.change(&c)
			if err := c.ValidateReady(); err == nil {
				t.Fatal("unverified contract admitted")
			}
		})
	}
}

func TestInputCapabilitiesAreExplicitAndBounded(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input Input
		valid bool
	}{
		{"unknown disabled", Input{Support: "unknown"}, true},
		{"unknown enabled", Input{Support: "unknown", Enabled: true}, false},
		{"unsupported enabled", Input{Support: "unsupported", Enabled: true}, false},
		{"missing formats", Input{Support: "supported", Enabled: true, Processing: "native", MaxCount: 1, MaxBytes: 1024}, false},
		{"missing byte limit", Input{Support: "supported", Enabled: true, Processing: "native", MaxCount: 1, Formats: []FileFormat{{Extension: ".png", MIME: "image/png"}}}, false},
		{"native image", Input{Support: "supported", Enabled: true, Processing: "native", MaxCount: 1, MaxBytes: 1024, Formats: []FileFormat{{Extension: ".png", MIME: "image/png"}}}, true},
		{"wildcard MIME", Input{Support: "supported", Enabled: true, Processing: "native", MaxCount: 1, MaxBytes: 1024, Formats: []FileFormat{{Extension: ".png", MIME: "image/*"}}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := readyTextContract()
			c.Operations[0].Inputs.Images = tc.input
			c.Operations[0].Inputs.MaxTotalBytes = 2048
			if err := c.Validate(); (err == nil) != tc.valid {
				t.Fatalf("valid=%v: %v", tc.valid, err)
			}
		})
	}
}

func TestTypedOutputAndDefaults(t *testing.T) {
	c := readyTextContract()
	c.Operations[0].Image = &ImageOutput{}
	if err := c.Validate(); err == nil {
		t.Fatal("text operation accepted image settings")
	}
	c = readyTextContract()
	c.Operations[0].Text.MaxOutputTokens = 1001
	if err := c.Validate(); err == nil {
		t.Fatal("output beyond context accepted")
	}
	c = readyTextContract()
	c.Categories = []string{"made-up"}
	if err := c.Validate(); err == nil {
		t.Fatal("unknown category accepted")
	}
}

func TestEnabledFileFormatNeedsItsOwnLiveCheck(t *testing.T) {
	c := readyTextContract()
	c.Operations[0].Inputs.MaxTotalBytes = 4096
	c.Operations[0].Inputs.Documents = Input{Support: "supported", Enabled: true, Processing: "extracted_text", MaxCount: 1, MaxBytes: 4096, MaxPages: 2, Formats: []FileFormat{{Extension: ".pdf", MIME: "application/pdf"}}}
	for i := range c.Checks {
		c.Checks[i].ContractDigest = c.Digest()
	}
	if err := c.ValidateReady(); err == nil {
		t.Fatal("file enabled without semantic input verification")
	}
	c.Checks = append(c.Checks, Check{Scenario: "reply/live-input/documents/.pdf", Status: "passed", Evidence: "pdf-verification.md", EvidenceSHA256: strings.Repeat("b", 64), CheckedAt: "2026-09-14", ContractDigest: c.Digest()})
	if err := c.ValidateReady(); err != nil {
		t.Fatal(err)
	}
}

func TestStrictDecodeRejectsMisspelledFieldsAndTrailingJSON(t *testing.T) {
	for _, raw := range []string{`{"schema_version":1,"suports_video":true}`, `{} {}`} {
		if _, err := Decode(strings.NewReader(raw)); err == nil {
			t.Fatal("ambiguous JSON accepted")
		}
	}
}
