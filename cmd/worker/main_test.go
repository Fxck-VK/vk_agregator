package main

import (
	"reflect"
	"testing"

	"vk-ai-aggregator/internal/adapter/provider/runway"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
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
