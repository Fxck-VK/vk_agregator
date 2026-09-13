package worker

import (
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestOmniAutomaticOutputDurationAcceptsProviderChosenLength(t *testing.T) {
	contracts := providermodels.StaticRegistry().ProviderMediaContracts(providermodels.MediaContractRuntime{})
	for _, contract := range contracts {
		if contract.Model != providermodels.ProviderModelOmni11Flash {
			continue
		}
		for _, duration := range []int64{3000, 3500, 6200, 9999, 10000, 12000} {
			metadata := domain.ArtifactMediaMetadata{ProbeStatus: domain.MediaProbePassed, Container: "mp4", Codec: "h264", Width: 1280, Height: 720, DurationMS: duration}
			if got := deliveryReadyVideoOutput(&contract, metadata, 8<<20); got != (duration <= 10000) {
				t.Fatalf("duration=%d accepted=%v", duration, got)
			}
		}
		return
	}
	t.Fatal("Omni contract missing")
}

func TestOmni4KOutputPassesProbeContractInBothOrientations(t *testing.T) {
	contracts := providermodels.StaticRegistry().ProviderMediaContracts(providermodels.MediaContractRuntime{})
	for _, contract := range contracts {
		if !providermodels.IsOmniVideoRoute(contract.Provider, contract.Model) {
			continue
		}
		for _, dimensions := range [][2]int{{3840, 2160}, {2160, 3840}, {7680, 4320}} {
			metadata := domain.ArtifactMediaMetadata{ProbeStatus: domain.MediaProbePassed, Container: "mp4", Codec: "h264", Width: dimensions[0], Height: dimensions[1], DurationMS: 6000}
			want := dimensions[0] != 7680
			if got := deliveryReadyVideoOutput(&contract, metadata, 32<<20); got != want {
				t.Errorf("model=%s size=%v accepted=%v want=%v", contract.Model, dimensions, got, want)
			}
		}
	}
}

func TestDeliveryReadyVideoOutputAllowsSmallDurationOverrun(t *testing.T) {
	contract := &domain.ProviderMediaContract{
		DeliveryReadyOutput: true,
		ExpectedContainer:   "mp4",
		ExpectedCodec:       "h264",
		ExpectedMaxBytes:    256 << 20,
		AllowedDurationsSec: []int{5, 10},
		AllowedAspectRatios: []string{"16:9"},
		AllowedResolutions:  []string{"720p"},
	}
	metadata := domain.ArtifactMediaMetadata{
		ProbeStatus: domain.MediaProbePassed,
		Container:   "mp4",
		Codec:       "h264",
		Width:       1280,
		Height:      720,
		DurationMS:  5200,
	}

	if !deliveryReadyVideoOutput(contract, metadata, 8<<20) {
		t.Fatal("delivery-ready provider output with codec/container duration overrun was rejected")
	}
}

func TestDeliveryReadyVideoOutputRejectsLargeDurationOverrun(t *testing.T) {
	contract := &domain.ProviderMediaContract{
		DeliveryReadyOutput: true,
		ExpectedContainer:   "mp4",
		ExpectedCodec:       "h264",
		ExpectedMaxBytes:    256 << 20,
		AllowedDurationsSec: []int{5, 10},
		AllowedAspectRatios: []string{"16:9"},
		AllowedResolutions:  []string{"720p"},
	}
	metadata := domain.ArtifactMediaMetadata{
		ProbeStatus: domain.MediaProbePassed,
		Container:   "mp4",
		Codec:       "h264",
		Width:       1280,
		Height:      720,
		DurationMS:  6200,
	}

	if deliveryReadyVideoOutput(contract, metadata, 8<<20) {
		t.Fatal("provider output with large duration overrun was accepted")
	}
}
