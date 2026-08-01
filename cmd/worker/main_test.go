package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"vk-ai-aggregator/internal/adapter/provider/runway"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestOutboxRelayOwnerUsesStableSanitizedProcessIdentity(t *testing.T) {
	if got := outboxRelayOwner(func() (string, error) { return "worker-01", nil }, 4321); got != "worker-01-4321" {
		t.Fatalf("outboxRelayOwner() = %q, want worker-01-4321", got)
	}

	unsafe := strings.Repeat("host name/secret@", 20)
	got := outboxRelayOwner(func() (string, error) { return unsafe, nil }, 4321)
	if len(got) > maxOutboxRelayOwnerLength {
		t.Fatalf("owner length = %d, want <= %d", len(got), maxOutboxRelayOwnerLength)
	}
	if strings.ContainsAny(got, " /@") {
		t.Fatalf("owner contains unsafe characters: %q", got)
	}

	if got := outboxRelayOwner(func() (string, error) { return "", errors.New("hostname unavailable") }, 4321); got != "worker-4321" {
		t.Fatalf("fallback owner = %q, want worker-4321", got)
	}
}

func TestOutboxRelayWorkerConfigurationIsBounded(t *testing.T) {
	if outboxRelayLeaseDuration < 10*time.Second || outboxRelayLeaseDuration > 5*time.Minute {
		t.Fatalf("lease duration = %s, want enough for one Redis call and bounded", outboxRelayLeaseDuration)
	}
	if outboxRelayMaxAttempts < 1 || outboxRelayMaxAttempts > 10 {
		t.Fatalf("max attempts = %d, want within [1, 10]", outboxRelayMaxAttempts)
	}
	if outboxRelayRetryBase <= 0 || outboxRelayRetryBase > outboxRelayLeaseDuration {
		t.Fatalf("retry base = %s, want positive and no longer than lease", outboxRelayRetryBase)
	}
	if outboxRelayRetryMax < outboxRelayRetryBase || outboxRelayRetryMax > 10*time.Minute {
		t.Fatalf("retry max = %s, want bounded and >= base", outboxRelayRetryMax)
	}
}

func TestWorkerRuntimeLoopSelection(t *testing.T) {
	tests := []struct {
		name        string
		mode        string
		jobs        bool
		relay       bool
		maintenance bool
	}{
		{name: "default", mode: "", jobs: true, relay: true, maintenance: true},
		{name: "all", mode: config.WorkerModeAll, jobs: true, relay: true, maintenance: true},
		{name: "jobs", mode: config.WorkerModeJobs, jobs: true, relay: true, maintenance: false},
		{name: "relay only", mode: config.WorkerModeRelay, jobs: false, relay: true, maintenance: false},
		{name: "maintenance", mode: config.WorkerModeMaintenance, jobs: false, relay: false, maintenance: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRunJobWorkers(tt.mode); got != tt.jobs {
				t.Fatalf("shouldRunJobWorkers(%q) = %t, want %t", tt.mode, got, tt.jobs)
			}
			if got := shouldRunOutboxRelay(tt.mode); got != tt.relay {
				t.Fatalf("shouldRunOutboxRelay(%q) = %t, want %t", tt.mode, got, tt.relay)
			}
			if got := shouldRunMaintenance(tt.mode); got != tt.maintenance {
				t.Fatalf("shouldRunMaintenance(%q) = %t, want %t", tt.mode, got, tt.maintenance)
			}
		})
	}
}

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
