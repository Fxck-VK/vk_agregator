package modelcatalog

import (
	"reflect"
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestPaidTextModelsKeepExactRoutesAndRejectPrivateAliases(t *testing.T) {
	for _, alias := range providermodels.PaidTextModels() {
		t.Run(alias.PublicID, func(t *testing.T) {
			model, ok := ResolvePublicModel(domain.OperationTextGenerate, alias.PublicID)
			if !ok || model.Provider != alias.Provider || model.ModelCode != alias.ProviderModelID || !model.ExposeID {
				t.Fatalf("paid model route = %+v/%v", model, ok)
			}
			for _, privateID := range []string{alias.DisplayName, alias.ProviderModelID} {
				if _, ok := ResolvePublicModel(domain.OperationTextGenerate, privateID); ok {
					t.Fatalf("private alias accepted: %s", privateID)
				}
			}
		})
	}
}

func TestPublicSeedream45PublicIDComesFromProviderRegistry(t *testing.T) {
	if PublicImageSeedream45 != providermodels.PublicImageSeedream45 {
		t.Fatalf("Seedream public id = %q, want registry id %q", PublicImageSeedream45, providermodels.PublicImageSeedream45)
	}
	if model, ok := ResolvePublicModel(domain.OperationImageGenerate, ModelCodeSeedream45); ok {
		t.Fatalf("legacy DeepInfra Seedream provider id must not resolve as public input: %+v", model)
	}
}

func TestListPublicImageModelsExcludesLegacyDuplicates(t *testing.T) {
	models := ListPublicModels(domain.OperationImageGenerate)
	seen := map[string]int{}
	for _, model := range models {
		seen[model.ModelID]++
	}

	if seen[PublicImageSeedream45] != 1 {
		t.Fatalf("Seedream 4.5 should be listed once from the provider registry: %+v", models)
	}
	for _, id := range []string{PublicImageSDXLTurbo} {
		if seen[id] != 0 {
			t.Fatalf("deepinfra model %s leaked into public list: %+v", id, models)
		}
	}
	for _, legacy := range []string{"sdxl", "kandinsky", PublicImageNanoBananaFlash} {
		if seen[legacy] != 0 {
			t.Fatalf("legacy alias %s leaked into public list: %+v", legacy, models)
		}
	}
}

func TestResolvePublicImageSeedream45UsesPoyoRegistryModel(t *testing.T) {
	model, ok := ResolvePublicModel(domain.OperationImageGenerate, PublicImageSeedream45)
	if !ok {
		t.Fatal("Seedream 4.5 did not resolve from provider registry")
	}
	if model.Provider != domain.ProviderPoYo || model.ModelCode != "seedream-4.5" || !model.SupportsReferenceImage || model.MaxReferenceImages != 10 {
		t.Fatalf("Seedream 4.5 registry model = %+v", model)
	}
}

func TestResolvePublicImageDeepInfraModelsFailClosed(t *testing.T) {
	for _, modelID := range []string{PublicImageSDXLTurbo, "sdxl", PublicImageNanoBananaFlash} {
		if model, ok := ResolvePublicModel(domain.OperationImageGenerate, modelID); ok {
			t.Fatalf("deepinfra image model %s resolved unexpectedly: %+v", modelID, model)
		}
	}
}

func TestResolvePublicMockImageModel(t *testing.T) {
	model, ok := ResolvePublicModel(domain.OperationImageGenerate, PublicImageMock)
	if !ok {
		t.Fatal("mock image model did not resolve")
	}
	if model.ModelID != PublicImageMock || model.Provider != domain.ProviderMock || model.ModelCode != ModelCodeMockImage {
		t.Fatalf("mock image resolved to %+v", model)
	}
}

func TestResolvePublicModelKeepsLegacyPublicResolutionCompatible(t *testing.T) {
	legacy, legacyOK := ResolvePublicModel(domain.OperationImageGenerate, PublicImageNanoBanana2)
	public, publicOK := ResolvePublicModel(domain.OperationImageGenerate, PublicImageNanoBanana2)
	if !legacyOK || !publicOK || !reflect.DeepEqual(public, legacy) {
		t.Fatalf("public resolver = %+v/%v, legacy resolver = %+v/%v", public, publicOK, legacy, legacyOK)
	}
}
