package miniapp

import (
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

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
