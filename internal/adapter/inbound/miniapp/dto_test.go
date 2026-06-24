package miniapp

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

func TestPublicPricingDTOsSerializeOnlyContractFields(t *testing.T) {
	catalogFields := assertJSONAllowedKeys(t, "ModelCatalogItemDTO", ModelCatalogItemDTO{
		Type:                   "video",
		ID:                     "video_kling_o3_standard",
		Alias:                  "video_kling_o3_standard",
		Name:                   "Kling O3 Standard",
		Description:            "Public route",
		EstimateCredits:        100,
		Enabled:                true,
		QualityOptions:         []string{"1K", "2K"},
		DefaultQuality:         "1K",
		AllowedDurationsSec:    []int{5, 10},
		AllowedResolutions:     []string{"720p"},
		AllowedAspectRatios:    []string{"16:9"},
		DefaultDurationSec:     5,
		DefaultResolution:      "720p",
		DefaultAspectRatio:     "16:9",
		RequiresStartImage:     true,
		SupportsReferenceImage: true,
		MaxReferenceImages:     1,
	}, map[string]bool{
		"type": true, "id": true, "alias": true, "name": true, "description": true,
		"estimate_credits": true, "enabled": true, "quality_options": true,
		"default_quality": true, "allowed_durations_sec": true,
		"allowed_resolutions": true, "allowed_aspect_ratios": true,
		"default_duration_sec": true, "default_resolution": true,
		"default_aspect_ratio": true, "requires_start_image": true,
		"supports_reference_image": true, "max_reference_images": true,
	})
	assertJSONHasKeys(t, "ModelCatalogItemDTO", catalogFields, "estimate_credits")
	assertNoPrivatePricingProviderKeys(t, "ModelCatalogItemDTO", catalogFields, "cost_estimate")

	estimateFields := assertJSONAllowedKeys(t, "EstimateDTO", EstimateDTO{
		Operation:       "image_generate",
		ModelID:         "nano_banana_2",
		ModelName:       "Nano Banana 2",
		VideoRouteAlias: "video_kling_o3_standard",
		ImageQuality:    "2K",
		CostEstimate:    16,
		BalanceCredits:  100,
		EnoughCredits:   true,
	}, map[string]bool{
		"operation": true, "model_id": true, "model_name": true,
		"video_route_alias": true, "image_quality": true,
		"cost_estimate": true, "balance_credits": true, "enough_credits": true,
	})
	assertJSONHasKeys(t, "EstimateDTO", estimateFields, "cost_estimate", "balance_credits", "enough_credits")
	assertNoPrivatePricingProviderKeys(t, "EstimateDTO", estimateFields)
}

func TestCreateJobRequestSerializesNoClientPricingOrProviderFields(t *testing.T) {
	fields := assertJSONAllowedKeys(t, "CreateJobRequest", CreateJobRequest{
		Operation:            "video_generate",
		Prompt:               "public prompt",
		ModelID:              "nano_banana_2",
		VideoRouteAlias:      "video_kling_o3_standard",
		ImageQuality:         "2K",
		ReferenceArtifactIDs: []uuid.UUID{uuid.New()},
		DurationSec:          5,
		AspectRatio:          "16:9",
	}, map[string]bool{
		"operation": true, "prompt": true, "model_id": true,
		"video_route_alias": true, "image_quality": true,
		"reference_artifact_ids": true, "duration_sec": true,
	})
	assertJSONHasKeys(t, "CreateJobRequest", fields,
		"operation", "prompt", "model_id", "video_route_alias",
		"image_quality", "reference_artifact_ids", "duration_sec",
	)
	assertNoPrivatePricingProviderKeys(t, "CreateJobRequest", fields,
		"cost", "cost_estimate", "aspect_ratio",
	)
}

func TestNewJobDTOMapsLegacyMediaErrorCodesToSafeClasses(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{
			name: "probe failure",
			code: string(domain.ProviderErrMediaProbeFailed),
			want: domain.JobErrMediaProviderOutputInvalid,
		},
		{
			name: "transcode failure",
			code: string(domain.ProviderErrMediaTranscodeFailed),
			want: domain.JobErrMediaProcessingUnavailable,
		},
		{
			name: "delivery failure",
			code: "delivery_failed",
			want: domain.JobErrMediaDeliveryFailed,
		},
		{
			name: "overloaded",
			code: "media_transcode_overloaded",
			want: domain.JobErrMediaOverloadedRetryLater,
		},
		{
			name: "provider internal fallback",
			code: string(domain.ProviderErrInternal),
			want: domain.JobErrMediaProcessingUnavailable,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dto := newJobDTO(&domain.Job{
				ID:        uuid.New(),
				Modality:  domain.ModalityVideo,
				Status:    domain.JobStatusFailedTerminal,
				ErrorCode: tc.code,
			})
			if dto.ErrorCode != tc.want {
				t.Fatalf("ErrorCode = %q, want %q", dto.ErrorCode, tc.want)
			}
		})
	}
}

func TestNewJobDTOHidesOutputArtifactIDsUntilSucceeded(t *testing.T) {
	artifactID := uuid.New()

	pending := newJobDTO(&domain.Job{
		ID:                uuid.New(),
		Status:            domain.JobStatusResultReady,
		OutputArtifactIDs: []uuid.UUID{artifactID},
	})
	if len(pending.OutputArtifactIDs) != 0 {
		t.Fatalf("result_ready DTO leaked output artifact ids: %+v", pending.OutputArtifactIDs)
	}

	rejected := newJobDTO(&domain.Job{
		ID:                uuid.New(),
		Status:            domain.JobStatusRejected,
		OutputArtifactIDs: []uuid.UUID{artifactID},
	})
	if len(rejected.OutputArtifactIDs) != 0 {
		t.Fatalf("rejected DTO leaked output artifact ids: %+v", rejected.OutputArtifactIDs)
	}

	succeeded := newJobDTO(&domain.Job{
		ID:                uuid.New(),
		Status:            domain.JobStatusSucceeded,
		OutputArtifactIDs: []uuid.UUID{artifactID},
	})
	if len(succeeded.OutputArtifactIDs) != 1 || succeeded.OutputArtifactIDs[0] != artifactID {
		t.Fatalf("succeeded DTO did not expose output artifact id: %+v", succeeded.OutputArtifactIDs)
	}
}

func assertJSONAllowedKeys(t *testing.T, name string, value any, allowed map[string]bool) map[string]json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %s: %v", name, err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("unmarshal %s json: %v; raw=%s", name, err, raw)
	}
	for key := range fields {
		if !allowed[key] {
			t.Fatalf("%s serialized non-contract field %q: %s", name, key, raw)
		}
	}
	return fields
}

func assertJSONHasKeys(t *testing.T, name string, fields map[string]json.RawMessage, required ...string) {
	t.Helper()
	for _, key := range required {
		if _, ok := fields[key]; !ok {
			t.Fatalf("%s did not serialize required contract field %q: %+v", name, key, fields)
		}
	}
}

func assertNoPrivatePricingProviderKeys(t *testing.T, name string, fields map[string]json.RawMessage, extraForbidden ...string) {
	t.Helper()
	forbidden := append([]string{
		"provider",
		"provider_model_id",
		"provider_native_model_id",
		"model_code",
		"price",
		"provider_cost",
		"provider_cost_credits",
		"price_multiplier",
		"multiplier",
		"floor",
		"floor_amount",
		"floor_unit",
		"raw_provider_payload",
		"resolved_snapshot",
	}, extraForbidden...)
	for _, key := range forbidden {
		if _, ok := fields[key]; ok {
			t.Fatalf("%s leaked private pricing/provider field %q: %+v", name, key, fields)
		}
	}
}
