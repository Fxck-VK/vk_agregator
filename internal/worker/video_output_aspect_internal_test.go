package worker

import (
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestAPIMartObservedOutputDimensions(t *testing.T) {
	if err := providermodels.ConfigureDEVSmoke("development", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = providermodels.ConfigureDEVSmoke("development", false) })
	contracts := providermodels.RuntimeRegistry().ProviderMediaContracts(providermodels.MediaContractRuntime{})
	byModel := make(map[string]domain.ProviderMediaContract)
	for _, contract := range contracts {
		if contract.Provider == domain.ProviderAPIMart {
			byModel[contract.Model] = contract
		}
	}
	// Sanitized dimensions observed in the authorized APIMart smoke on 2026-11-13.
	for _, tc := range []struct {
		model         string
		width, height int
		durationMS    int64
	}{
		{"viduq3", 960, 528, 3042},
		{"viduq3-pro", 960, 528, 1042},
		{"viduq3-turbo", 960, 528, 1042},
		{"wan3.0-video-prime", 854, 480, 2000},
		{"seedance-2.0", 864, 496, 4042},
		{"seedance-2.0-mini", 864, 496, 4042},
		{"wan3.0-video", 854, 480, 2000},
		{"grok-imagine-1.5-video-ext", 752, 416, 6042},
		{"MiniMax-H3-Max", 832, 480, 5184},
		{"flux-3-video", 1280, 704, 5042},
	} {
		t.Run(tc.model, func(t *testing.T) {
			contract, ok := byModel[tc.model]
			if !ok {
				t.Fatal("runtime media contract missing")
			}
			metadata := domain.ArtifactMediaMetadata{
				ProbeStatus: domain.MediaProbePassed, Container: "mp4", Codec: "h264",
				Width: tc.width, Height: tc.height, DurationMS: tc.durationMS,
			}
			processor := processor{rawVideoPolicy: rawProviderVideoPolicyIfProbePassed}
			if !processor.videoFastPathAllowed(&contract, metadata, 1<<20) {
				t.Fatalf("observed %dx%d output rejected after successful probe", tc.width, tc.height)
			}
			registry := newMediaContractRegistry(contracts, false, false)
			_, err := registry.validateProviderSubmit(domain.ProviderAPIMart, domain.ProviderRequest{
				Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo,
				ModelCode: tc.model, AspectRatio: "20:11", Resolution: "720p",
				DurationSec: int(tc.durationMS / 1000),
			}, domain.CostEstimate{})
			if err == nil {
				t.Fatal("output rounding allowance leaked into provider request admission")
			}
		})
	}

	h3Max := byModel["MiniMax-H3-Max"]
	h3Max.AllowedAspectRatios = []string{"16:9"}
	for _, dims := range [][2]int{{833, 480}, {832, 479}, {1664, 960}, {480, 832}, {832, 600}} {
		m := domain.ArtifactMediaMetadata{ProbeStatus: domain.MediaProbePassed, Container: "mp4", Codec: "h264", Width: dims[0], Height: dims[1], DurationMS: 5184}
		if deliveryReadyVideoOutput(&h3Max, m, 1024) {
			t.Fatalf("unverified MiniMax H3 Max dimensions %v accepted", dims)
		}
	}
	for _, mutate := range []func(*domain.ArtifactMediaMetadata){
		func(m *domain.ArtifactMediaMetadata) { m.ProbeStatus = domain.MediaProbeFailed },
		func(m *domain.ArtifactMediaMetadata) { m.Codec = "hevc" },
		func(m *domain.ArtifactMediaMetadata) { m.Container = "webm" },
		func(m *domain.ArtifactMediaMetadata) { m.DurationMS = 60000 },
	} {
		m := domain.ArtifactMediaMetadata{ProbeStatus: domain.MediaProbePassed, Container: "mp4", Codec: "h264", Width: 832, Height: 480, DurationMS: 5184}
		mutate(&m)
		if deliveryReadyVideoOutput(&h3Max, m, 1024) {
			t.Fatal("exact raster exception bypassed independent media check")
		}
	}
	exactRaster := domain.ArtifactMediaMetadata{ProbeStatus: domain.MediaProbePassed, Container: "mp4", Codec: "h264", Width: 832, Height: 480, DurationMS: 5184}
	if deliveryReadyVideoOutput(&h3Max, exactRaster, h3Max.ExpectedMaxBytes+1) {
		t.Fatal("exact raster exception bypassed size limit")
	}
	strict := byModel["pixverse-v6"]
	if deliveryReadyVideoOutput(&strict, exactRaster, 1024) {
		t.Fatal("exact raster exception leaked to another model")
	}
	limited := h3Max
	limited.AllowedOutputResolutions = []string{"360p"}
	if deliveryReadyVideoOutput(&limited, exactRaster, 1024) {
		t.Fatal("exact raster exception bypassed resolution limit")
	}
	h3Max.AllowedAspectRatios = []string{"1:1"}
	if deliveryReadyVideoOutput(&h3Max, domain.ArtifactMediaMetadata{ProbeStatus: domain.MediaProbePassed, Container: "mp4", Codec: "h264", Width: 832, Height: 480, DurationMS: 5184}, 1024) {
		t.Fatal("observed raster bypassed allowed aspect")
	}

	contract := byModel["viduq3-pro"]
	contract.AllowedAspectRatios = []string{"16:9"}
	contract.AllowedResolutions = []string{"4k"}
	base := domain.ArtifactMediaMetadata{
		ProbeStatus: domain.MediaProbePassed, Container: "mp4", Codec: "h264",
		Width: 960, Height: 528, DurationMS: 1042,
	}
	for _, tc := range []struct {
		name          string
		width, height int
	}{
		{"portrait with landscape-only contract", 528, 960},
		{"wrong aspect", 960, 720},
		{"more than sixteen pixels", 3840, 2112},
		{"more than relative tolerance", 320, 172},
		{"zero width", 0, 528},
		{"negative height", 960, -528},
	} {
		t.Run(tc.name, func(t *testing.T) {
			metadata := base
			metadata.Width, metadata.Height = tc.width, tc.height
			if deliveryReadyVideoOutput(&contract, metadata, 1<<20) {
				t.Fatalf("invalid %dx%d output accepted", tc.width, tc.height)
			}
		})
	}
	for _, tc := range []struct {
		name   string
		change func(*domain.ArtifactMediaMetadata)
	}{
		{"unsafe codec", func(m *domain.ArtifactMediaMetadata) { m.Codec = "hevc" }},
		{"wrong container", func(m *domain.ArtifactMediaMetadata) { m.Container = "webm" }},
		{"probe failed", func(m *domain.ArtifactMediaMetadata) { m.ProbeStatus = domain.MediaProbeFailed }},
		{"excessive duration", func(m *domain.ArtifactMediaMetadata) { m.DurationMS = 18000 }},
		{"excessive resolution", func(m *domain.ArtifactMediaMetadata) { m.Width, m.Height = 7680, 4320 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			metadata := base
			tc.change(&metadata)
			if deliveryReadyVideoOutput(&contract, metadata, 1<<20) {
				t.Fatal("rounded dimensions bypassed an independent output check")
			}
		})
	}
	if deliveryReadyVideoOutput(&contract, base, contract.ExpectedMaxBytes+1) {
		t.Fatal("rounded dimensions bypassed the file size limit")
	}
	// Other models retain exact output matching until separately verified.
	unverified := byModel["pixverse-v6"]
	if deliveryReadyVideoOutput(&unverified, base, 1<<20) {
		t.Fatal("rounding allowance leaked to an unverified model")
	}
}

func TestVideoOutputAspectRoundingBounds(t *testing.T) {
	for _, tc := range []struct {
		name          string
		width, height int
		aspect        string
		rounding      bool
		want          bool
	}{
		{"exact without opt-in", 1280, 720, "16:9", false, true},
		{"rounded without opt-in", 960, 528, "16:9", false, false},
		{"pixel boundary", 1920, 1064, "16:9", true, true},
		{"outside pixel boundary", 1920, 1063, "16:9", true, false},
		{"relative boundary", 820, 450, "16:9", true, true},
		{"outside relative boundary", 821, 450, "16:9", true, false},
		{"portrait rounding", 528, 960, "9:16", true, true},
		{"portrait relative boundary", 450, 820, "9:16", true, true},
		{"outside portrait boundary", 450, 821, "9:16", true, false},
		{"zero denominator", 960, 528, "16:0", true, false},
		{"negative numerator", 960, 528, "-16:9", true, false},
		{"malformed ratio", 960, 528, "16:no", true, false},
		{"overflowing ratio", 960, 528, "999999999999999999999999999999:9", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			contract := domain.ProviderMediaContract{
				DeliveryReadyOutput: true, ExpectedContainer: "mp4", ExpectedCodec: "h264",
				ExpectedMaxBytes: 1 << 20, AllowedDurationsSec: []int{5},
				AllowedResolutions: []string{"4k"}, AllowedAspectRatios: []string{tc.aspect},
				OutputMayRoundAspect: tc.rounding,
			}
			metadata := domain.ArtifactMediaMetadata{
				ProbeStatus: domain.MediaProbePassed, Container: "mp4", Codec: "h264",
				Width: tc.width, Height: tc.height, DurationMS: 5000,
			}
			if got := deliveryReadyVideoOutput(&contract, metadata, 1024); got != tc.want {
				t.Fatalf("%dx%d output accepted=%v, want %v", tc.width, tc.height, got, tc.want)
			}
		})
	}
}
