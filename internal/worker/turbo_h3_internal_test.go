package worker

import (
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestTurboH3PaidSubmissionIsDurable(t *testing.T) {
	for _, model := range []string{"kling-3.0-turbo", "MiniMax-H3"} {
		if !durablePaidSubmitRoute(domain.ProviderAPIMart, model) {
			t.Fatalf("%s could resubmit paid calls", model)
		}
	}
}

func TestH3OutputResolutionHandlesPortrait2K(t *testing.T) {
	for _, tc := range []struct {
		w, h int
		want bool
	}{{2560, 1440, true}, {1440, 2560, true}, {2048, 2048, true}, {3000, 1440, false}, {1440, 3000, false}, {0, 0, false}} {
		if got := outputResolutionMatches(tc.w, tc.h, "2k"); got != tc.want {
			t.Errorf("%dx%d accepted=%v want=%v", tc.w, tc.h, got, tc.want)
		}
	}
}

func TestTurboH3MediaContractsAcceptFrameAspectAndPortrait(t *testing.T) {
	for _, contract := range providermodels.StaticRegistry().ProviderMediaContracts(providermodels.MediaContractRuntime{}) {
		if !providermodels.IsTurboH3VideoRoute(contract.Provider, contract.Model) {
			continue
		}
		for _, dimensions := range [][2]int{{1080, 1920}, {960, 1440}, {1440, 960}} {
			metadata := domain.ArtifactMediaMetadata{ProbeStatus: domain.MediaProbePassed, Container: "mp4", Codec: "h264", Width: dimensions[0], Height: dimensions[1], DurationMS: 5000}
			if !deliveryReadyVideoOutput(&contract, metadata, 8<<20) {
				t.Errorf("%s rejected provider first-frame aspect %v", contract.Model, dimensions)
			}
		}
	}
}
