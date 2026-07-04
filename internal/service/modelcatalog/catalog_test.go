package modelcatalog

import (
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestMiniAppSeedream45PublicIDComesFromProviderRegistry(t *testing.T) {
	if MiniAppImageSeedream45 != providermodels.PublicImageSeedream45 {
		t.Fatalf("Seedream public id = %q, want registry id %q", MiniAppImageSeedream45, providermodels.PublicImageSeedream45)
	}
	if model, ok := ResolveMiniAppModel(domain.OperationImageGenerate, ModelCodeSeedream45); ok {
		t.Fatalf("legacy DeepInfra Seedream provider id must not resolve as public input: %+v", model)
	}
}

func TestListMiniAppImageModelsExcludesLegacyDuplicates(t *testing.T) {
	models := ListMiniAppModels(domain.OperationImageGenerate)
	seen := map[string]int{}
	for _, model := range models {
		seen[model.ModelID]++
	}

	if seen[MiniAppImageSeedream45] != 1 {
		t.Fatalf("Seedream 4.5 should be listed once from the provider registry: %+v", models)
	}
	for _, id := range []string{MiniAppImageSDXLTurbo} {
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

func TestResolveMiniAppImageSeedream45UsesPoyoRegistryModel(t *testing.T) {
	model, ok := ResolveMiniAppModel(domain.OperationImageGenerate, MiniAppImageSeedream45)
	if !ok {
		t.Fatal("Seedream 4.5 did not resolve from provider registry")
	}
	if model.Provider != domain.ProviderPoYo || model.ModelCode != "seedream-4.5" || !model.SupportsReferenceImage || model.MaxReferenceImages != 10 {
		t.Fatalf("Seedream 4.5 registry model = %+v", model)
	}
}

func TestResolveMiniAppImageDeepInfraModelsFailClosed(t *testing.T) {
	for _, modelID := range []string{MiniAppImageSDXLTurbo, "sdxl", MiniAppImageNanoBananaFlash} {
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
