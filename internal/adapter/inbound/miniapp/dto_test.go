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
