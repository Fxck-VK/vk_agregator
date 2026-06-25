package worker

import (
	"testing"

	"vk-ai-aggregator/internal/domain"
)

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
