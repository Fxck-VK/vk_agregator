package providermodels_test

import (
	"reflect"
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestProviderMediaContractsMatchRegistryVideoRouteLimits(t *testing.T) {
	registry := providermodels.StaticRegistry()
	contracts := registry.ProviderMediaContracts(providermodels.MediaContractRuntime{
		ExpectedMaxVideoBytes: 42 << 20,
		RequireVideoProbe:     true,
		VideoTranscodeAllowed: true,
	})
	routes := registry.VideoRoutes()
	if len(contracts) != len(routes) {
		t.Fatalf("contracts = %d, want %d", len(contracts), len(routes))
	}

	byModel := map[string]domain.ProviderMediaContract{}
	for _, contract := range contracts {
		if err := contract.Validate(); err != nil {
			t.Fatalf("contract %+v did not validate: %v", contract, err)
		}
		key := string(contract.Provider) + "\x00" + contract.Model
		if _, exists := byModel[key]; exists {
			t.Fatalf("duplicate contract for %s/%s", contract.Provider, contract.Model)
		}
		byModel[key] = contract
	}

	for _, route := range routes {
		key := string(route.Provider) + "\x00" + route.ProviderModelID
		contract, ok := byModel[key]
		if !ok {
			t.Fatalf("missing contract for route %s provider model %s/%s", route.Alias, route.Provider, route.ProviderModelID)
		}
		if contract.ModelClass != route.ModelClass || contract.Modality != domain.ModalityVideo {
			t.Fatalf("route %s contract metadata = %+v", route.Alias, contract)
		}
		if !reflect.DeepEqual(contract.AllowedDurationsSec, route.Spec.AllowedDurationsSec) {
			t.Fatalf("route %s durations = %#v, want %#v", route.Alias, contract.AllowedDurationsSec, route.Spec.AllowedDurationsSec)
		}
		if !reflect.DeepEqual(contract.AllowedAspectRatios, route.Spec.AllowedAspectRatios) {
			t.Fatalf("route %s aspects = %#v, want %#v", route.Alias, contract.AllowedAspectRatios, route.Spec.AllowedAspectRatios)
		}
		if !reflect.DeepEqual(contract.AllowedResolutions, route.Spec.AllowedResolutions) {
			t.Fatalf("route %s resolutions = %#v, want %#v", route.Alias, contract.AllowedResolutions, route.Spec.AllowedResolutions)
		}
		if contract.ExpectedContainer != route.MediaContract.ExpectedContainer || contract.ExpectedCodec != route.MediaContract.ExpectedCodec {
			t.Fatalf("route %s container/codec = %s/%s, want %s/%s", route.Alias, contract.ExpectedContainer, contract.ExpectedCodec, route.MediaContract.ExpectedContainer, route.MediaContract.ExpectedCodec)
		}
		if contract.ExpectedMaxBytes != 42<<20 || !contract.RequiresProbe || !contract.TranscodeAllowed {
			t.Fatalf("route %s runtime policy not applied: %+v", route.Alias, contract)
		}
		if contract.MaxProviderCostCredits != route.Spec.MaxProviderCostCredits {
			t.Fatalf("route %s max provider cost = %d, want %d", route.Alias, contract.MaxProviderCostCredits, route.Spec.MaxProviderCostCredits)
		}
	}
}

func TestProviderMediaContractsDefaultMaxVideoBytes(t *testing.T) {
	contracts := providermodels.StaticRegistry().ProviderMediaContracts(providermodels.MediaContractRuntime{})
	if len(contracts) == 0 {
		t.Fatal("expected default provider media contracts")
	}
	for _, contract := range contracts {
		if contract.DeliveryReadyOutput && contract.ExpectedMaxBytes <= 0 {
			t.Fatalf("contract %s/%s missing default max bytes: %+v", contract.Provider, contract.Model, contract)
		}
	}
}
