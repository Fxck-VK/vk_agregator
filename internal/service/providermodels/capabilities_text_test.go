package providermodels

import "testing"

func TestTextCapabilitiesKeepApplicationInputsUnsupported(t *testing.T) {
	aliases := textAliases()
	if len(aliases) != 12 {
		t.Fatalf("text aliases = %d, want 12", len(aliases))
	}

	for _, alias := range aliases {
		got := textCapabilities(alias)
		if got == nil || got.Application.Text == nil {
			t.Fatalf("%s application text capabilities missing", alias.PublicID)
		}

		assertUnsupportedInput(t, alias.PublicID, "application.images", got.Application.Text.Images)
		assertUnsupportedInput(t, alias.PublicID, "application.videos", got.Application.Text.Videos)
		assertUnsupportedInput(t, alias.PublicID, "application.files", got.Application.Text.Files)
	}
}

func TestTextCapabilitiesKeepUnverifiedNativeInputsUnknown(t *testing.T) {
	alias, ok := PaidTextModel(PublicTextClaudeOpus47)
	if !ok {
		t.Fatal("claude opus 4.7 alias missing")
	}

	got := textCapabilities(alias)
	if got == nil || got.API.Text == nil {
		t.Fatal("api text capabilities missing")
	}

	assertUnknownInput(t, "api.images", got.API.Text.Images)
	assertUnknownInput(t, "api.videos", got.API.Text.Videos)
	assertUnknownInput(t, "api.files", got.API.Text.Files)
}

func TestTextCapabilitiesSeparateVerifiedNativeInputsFromApplicationPath(t *testing.T) {
	alias, ok := PaidTextModel(PublicTextGemini31Pro)
	if !ok {
		t.Fatal("gemini 3.1 pro alias missing")
	}

	got := textCapabilities(alias)
	if got == nil || got.API.Text == nil || got.Application.Text == nil {
		t.Fatal("text capabilities missing")
	}

	assertSupportedInput(t, "api.images", got.API.Text.Images)
	assertSupportedInput(t, "api.videos", got.API.Text.Videos)
	assertSupportedInput(t, "api.files", got.API.Text.Files)
	assertUnsupportedInput(t, alias.PublicID, "application.images", got.Application.Text.Images)
	assertUnsupportedInput(t, alias.PublicID, "application.videos", got.Application.Text.Videos)
	assertUnsupportedInput(t, alias.PublicID, "application.files", got.Application.Text.Files)
}

func assertUnsupportedInput(t *testing.T, model, name string, got InputCapability) {
	t.Helper()
	if got.Support != Unsupported {
		t.Fatalf("%s %s support = %q, want %q", model, name, got.Support, Unsupported)
	}
	if got.MaxCount == nil || *got.MaxCount != 0 {
		t.Fatalf("%s %s max count = %v, want 0", model, name, got.MaxCount)
	}
	if len(got.Extensions) != 0 {
		t.Fatalf("%s %s extensions = %v, want empty", model, name, got.Extensions)
	}
}

func assertUnknownInput(t *testing.T, name string, got InputCapability) {
	t.Helper()
	if got.Support != Unknown {
		t.Fatalf("%s support = %q, want %q", name, got.Support, Unknown)
	}
	if got.MaxCount != nil {
		t.Fatalf("%s max count = %v, want nil", name, got.MaxCount)
	}
	if len(got.Extensions) != 0 {
		t.Fatalf("%s extensions = %v, want empty", name, got.Extensions)
	}
}

func assertSupportedInput(t *testing.T, name string, got InputCapability) {
	t.Helper()
	if got.Support != Supported {
		t.Fatalf("%s support = %q, want %q", name, got.Support, Supported)
	}
	if got.MaxCount != nil {
		t.Fatalf("%s max count = %v, want nil because provider docs do not list a bound", name, got.MaxCount)
	}
}
