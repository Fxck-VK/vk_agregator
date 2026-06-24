package modelcatalog

import (
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestListMiniAppImageModelsExcludesDeepInfraAndLegacyDuplicates(t *testing.T) {
	models := ListMiniAppModels(domain.OperationImageGenerate)
	seen := map[string]int{}
	for _, model := range models {
		seen[model.ModelID]++
	}

	for _, id := range []string{MiniAppImageSeedream45, MiniAppImageSDXLTurbo} {
		if seen[id] != 0 {
			t.Fatalf("deepinfra model %s leaked into public list: %+v", id, models)
		}
	}
	for _, legacy := range []string{"sdxl", "kandinsky", MiniAppImageNanoBananaFlash} {
		if seen[legacy] != 0 {
			t.Fatalf("legacy alias %s leaked into public list: %+v", legacy, models)
		}
	}
}

func TestResolveMiniAppImageDeepInfraModelsFailClosed(t *testing.T) {
	for _, modelID := range []string{MiniAppImageSeedream45, MiniAppImageSDXLTurbo, "sdxl", MiniAppImageNanoBananaFlash} {
		if model, ok := ResolveMiniAppModel(domain.OperationImageGenerate, modelID); ok {
			t.Fatalf("deepinfra image model %s resolved unexpectedly: %+v", modelID, model)
		}
	}
}

func TestResolveMiniAppMockImageModel(t *testing.T) {
	model, ok := ResolveMiniAppModel(domain.OperationImageGenerate, MiniAppImageMock)
	if !ok {
		t.Fatal("mock image model did not resolve")
	}
	if model.ModelID != MiniAppImageMock || model.Provider != domain.ProviderMock || model.ModelCode != ModelCodeMockImage {
		t.Fatalf("mock image resolved to %+v", model)
	}
}
