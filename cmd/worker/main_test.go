package main

import (
	"reflect"
	"testing"

	"vk-ai-aggregator/internal/adapter/provider/runway"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestDefaultProviderMediaContractsRunwayMatchesRouteAspects(t *testing.T) {
	contracts := defaultProviderMediaContracts(config.Config{})
	for _, contract := range contracts {
		if contract.Provider != domain.ProviderRunway || contract.Model != runway.ModelGen4Turbo {
			continue
		}
		want := []string{"16:9", "9:16", "4:3", "3:4", "1:1", "21:9"}
		if !reflect.DeepEqual(contract.AllowedAspectRatios, want) {
			t.Fatalf("runway aspect ratios = %#v, want %#v", contract.AllowedAspectRatios, want)
		}
		return
	}
	t.Fatal("runway gen4_turbo media contract missing")
}

func TestDefaultProviderMediaContractsMatchRegistryRoutes(t *testing.T) {
	cfg := config.Config{
		MediaMaxVideoSizeBytes:    42 << 20,
		MediaVideoProbePolicy:     config.MediaVideoProbePolicyProbeRequired,
		MediaVideoTranscodePolicy: config.MediaVideoTranscodePolicyFallback,
	}
	contracts := defaultProviderMediaContracts(cfg)
	routes := providermodels.StaticRegistry().VideoRoutes()
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
		if contract.ModelClass != route.ModelClass {
			t.Fatalf("route %s model_class = %q, want %q", route.Alias, contract.ModelClass, route.ModelClass)
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
		if contract.ExpectedMaxBytes != 42<<20 || !contract.RequiresProbe || !contract.TranscodeAllowed {
			t.Fatalf("route %s runtime policy not applied: %+v", route.Alias, contract)
		}
		if contract.MaxProviderCostCredits != route.Spec.MaxProviderCostCredits {
			t.Fatalf("route %s max provider cost = %d, want %d", route.Alias, contract.MaxProviderCostCredits, route.Spec.MaxProviderCostCredits)
		}
	}
}

func TestEffectiveProviderMediaContractsKeepsConfigOverridesLast(t *testing.T) {
	override := domain.ProviderMediaContract{
		Provider:               domain.ProviderRunway,
		Model:                  runway.ModelGen4Turbo,
		ModelClass:             "custom_runway_override",
		Modality:               domain.ModalityVideo,
		AllowedDurationsSec:    []int{5},
		AllowedAspectRatios:    []string{"16:9"},
		AllowedResolutions:     []string{"720p"},
		ExpectedContainer:      "mp4",
		ExpectedCodec:          "h264",
		ExpectedMaxBytes:       1 << 20,
		DeliveryReadyOutput:    true,
		MaxProviderAttempts:    1,
		MaxFallbackAttempts:    0,
		MaxProviderCostCredits: 7,
	}
	contracts := effectiveProviderMediaContracts(config.Config{
		MediaProviderContracts: []domain.ProviderMediaContract{override},
	})
	if len(contracts) == 0 {
		t.Fatal("expected contracts")
	}
	got := contracts[len(contracts)-1]
	if !reflect.DeepEqual(got, override) {
		t.Fatalf("config override must remain last for reverse lookup precedence:\ngot  %+v\nwant %+v", got, override)
	}
}
