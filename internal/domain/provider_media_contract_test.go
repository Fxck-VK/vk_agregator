package domain_test

import (
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestProviderMediaContractValidateAllowsDeliveryReadyVideo(t *testing.T) {
	contract := domain.ProviderMediaContract{
		Provider:                     domain.ProviderDeepInfra,
		Model:                        "video-model",
		ModelClass:                   "deepinfra_video",
		Modality:                     domain.ModalityVideo,
		AllowedDurationsSec:          []int{5},
		AllowedAspectRatios:          []string{"16:9"},
		AllowedResolutions:           []string{"720p"},
		ExpectedContainer:            "mp4",
		ExpectedCodec:                "h264",
		ExpectedMaxBytes:             128 << 20,
		DeliveryReadyOutput:          true,
		RequiresProbe:                true,
		MaxProviderAttempts:          1,
		MaxProviderCostCredits:       10,
		ProviderIdempotencyGuarantee: domain.ProviderIdempotencyNone,
	}

	if err := contract.Validate(); err != nil {
		t.Fatalf("valid contract rejected: %v", err)
	}
}

func TestProviderMediaContractRejectsDuplicatePaidRetryRisk(t *testing.T) {
	contract := domain.ProviderMediaContract{
		Provider:               domain.ProviderDeepInfra,
		Model:                  "video-model",
		ModelClass:             "deepinfra_video",
		Modality:               domain.ModalityVideo,
		ExpectedContainer:      "mp4",
		ExpectedCodec:          "h264",
		ExpectedMaxBytes:       1,
		DeliveryReadyOutput:    true,
		MaxProviderAttempts:    2,
		MaxFallbackAttempts:    1,
		MaxProviderCostCredits: 10,
	}

	err := contract.Validate()
	if err == nil || !strings.Contains(err.Error(), "provider idempotency") {
		t.Fatalf("expected idempotency validation error, got %v", err)
	}
}

func TestProviderMediaContractRequiresBoundedModelClass(t *testing.T) {
	contract := domain.ProviderMediaContract{
		Provider:            domain.ProviderDeepInfra,
		Model:               "video-model",
		Modality:            domain.ModalityVideo,
		ExpectedContainer:   "mp4",
		ExpectedCodec:       "h264",
		ExpectedMaxBytes:    1,
		DeliveryReadyOutput: true,
		MaxProviderAttempts: 1,
		MaxFallbackAttempts: 0,
	}

	err := contract.Validate()
	if err == nil || !strings.Contains(err.Error(), "model_class") {
		t.Fatalf("expected model_class validation error, got %v", err)
	}
}
