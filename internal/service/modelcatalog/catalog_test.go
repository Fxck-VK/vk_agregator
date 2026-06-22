package modelcatalog

import (
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestListMiniAppImageModelsIncludesDeepInfraWithoutLegacyDuplicates(t *testing.T) {
	models := ListMiniAppModels(domain.OperationImageGenerate)
	seen := map[string]int{}
	for _, model := range models {
		seen[model.ModelID]++
	}

	for _, id := range []string{MiniAppImageSeedream45, MiniAppImageSDXLTurbo} {
		if seen[id] != 1 {
			t.Fatalf("public model %s count = %d, want 1; models=%+v", id, seen[id], models)
		}
	}
	for _, legacy := range []string{"sdxl", "kandinsky", MiniAppImageNanoBananaFlash} {
		if seen[legacy] != 0 {
			t.Fatalf("legacy alias %s leaked into public list: %+v", legacy, models)
		}
	}
}

func TestResolveMiniAppImageLegacyDeepInfraAliases(t *testing.T) {
	for _, alias := range []string{"sdxl", MiniAppImageNanoBananaFlash} {
		model, ok := ResolveMiniAppModel(domain.OperationImageGenerate, alias)
		if !ok {
			t.Fatalf("alias %s did not resolve", alias)
		}
		if model.ModelID != MiniAppImageSDXLTurbo || model.Provider != domain.ProviderDeepInfra || model.ModelCode != ModelCodeSDXLTurbo {
			t.Fatalf("alias %s resolved to %+v", alias, model)
		}
	}
}
