package providermodels

import (
	"reflect"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestVideoCapabilitiesExposeExactApplicationMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		alias       domain.VideoRouteAlias
		duration    DurationCapability
		imagesMax   int
		videos      InputCapability
		counts      []int
		resolutions []string
		qualities   []string
		audio       VideoAudioCapability
		startFrame  string
		endFrame    string
	}{
		{domain.VideoRouteKlingV3, selectedDuration(3, 15), 2, noInput(), []int{0, 1, 2}, []string{"720p", "1080p", "4k"}, nil, VideoAudioCapability{Mode: "optional", Selectable: true}, "optional", "optional"},
		{domain.VideoRouteKling26Motion, referenceVideoDuration(), 1, inputCapability(Supported, integer(1), "mp4", "mov"), []int{1}, nil, []string{"std", "pro"}, VideoAudioCapability{Mode: "preserve_source", Selectable: true}, "required", "unsupported"},
		{domain.VideoRouteVeo31Fast, fixedDuration(8), 3, noInput(), []int{0, 1, 2, 3}, []string{"720p", "1080p", "4k"}, nil, VideoAudioCapability{Mode: "unknown", Selectable: false}, "optional", "optional"},
		{domain.VideoRouteVeo31Quality, fixedDuration(8), 2, noInput(), []int{0, 1, 2}, []string{"720p", "1080p", "4k"}, nil, VideoAudioCapability{Mode: "unknown", Selectable: false}, "optional", "optional"},
		{domain.VideoRouteVeo31Lite, fixedDuration(8), 0, noInput(), []int{0}, []string{"720p", "1080p", "4k"}, nil, VideoAudioCapability{Mode: "unknown", Selectable: false}, "unsupported", "unsupported"},
		{domain.VideoRouteKling30Turbo, selectedDuration(3, 15), 1, noInput(), []int{0, 1}, []string{"720p", "1080p"}, nil, VideoAudioCapability{Mode: "unknown", Selectable: false}, "optional", "unsupported"},
		{domain.VideoRouteMiniMaxH3, selectedDuration(4, 15), 1, noInput(), []int{0, 1}, []string{"2k", "768p"}, nil, VideoAudioCapability{Mode: "unknown", Selectable: false}, "optional", "unsupported"},
		{domain.VideoRouteOmni11Flash, DurationCapability{Mode: "automatic", MinSeconds: integer(3), MaxSeconds: integer(10)}, 10, noInput(), []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, []string{"720p", "1080p", "360p", "4k"}, nil, VideoAudioCapability{Mode: "generated", Selectable: false}, "unsupported", "unsupported"},
		{domain.VideoRouteOmni11FlashExt, selectedDurations([]int{4, 6, 8, 10}), 3, noInput(), []int{0, 1, 3}, []string{"720p", "1080p", "360p", "4k"}, nil, VideoAudioCapability{Mode: "generated", Selectable: false}, "optional", "unsupported"},
		{domain.VideoRouteSeedance25, selectedDurations([]int{5, 10, 15, 30}), 4, noInput(), []int{0, 1, 2, 3, 4}, []string{"480p", "720p", "1080p"}, nil, VideoAudioCapability{Mode: "generated", Selectable: false}, "unsupported", "unsupported"},
		{domain.VideoRouteHailuo23Fast, hailuoDuration(), 1, noInput(), []int{1}, []string{"768p", "1080p"}, nil, VideoAudioCapability{Mode: "unknown", Selectable: false}, "required", "unsupported"},
		{domain.VideoRouteHailuo23Standard, hailuoDuration(), 1, noInput(), []int{0, 1}, []string{"768p", "1080p"}, nil, VideoAudioCapability{Mode: "unknown", Selectable: false}, "optional", "unsupported"},
		{domain.VideoRouteKlingO3Standard, selectedDurations([]int{5, 10}), 1, noInput(), []int{0, 1}, []string{"720p", "1080p"}, nil, VideoAudioCapability{Mode: "silent", Selectable: false}, "unsupported", "unsupported"},
		{domain.VideoRouteRunwayGen4Turbo, selectedDurations([]int{5, 10}), 1, noInput(), []int{1}, []string{"720p"}, nil, VideoAudioCapability{Mode: "unknown", Selectable: false}, "required", "unsupported"},
		{domain.VideoRouteSeedance20Fast, selectedDurations([]int{5, 10}), 4, noInput(), []int{0, 1, 2, 3, 4}, []string{"720p"}, nil, VideoAudioCapability{Mode: "silent", Selectable: false}, "unsupported", "unsupported"},
		{domain.VideoRouteRunwayGen45, selectedDurations([]int{5, 10}), 1, noInput(), []int{0, 1}, []string{"720p", "1080p"}, nil, VideoAudioCapability{Mode: "unknown", Selectable: false}, "unsupported", "unsupported"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(string(tc.alias), func(t *testing.T) {
			t.Parallel()
			caps := capabilityForVideoRoute(t, tc.alias)
			app := caps.Application.Video
			if app == nil {
				t.Fatal("application video capabilities are nil")
			}
			if !reflect.DeepEqual(app.Duration, tc.duration) {
				t.Fatalf("duration mismatch\nwant %#v\n got %#v", tc.duration, app.Duration)
			}
			expectImageInput(t, app.Images, tc.imagesMax)
			if !reflect.DeepEqual(app.Videos, tc.videos) {
				t.Fatalf("video input mismatch\nwant %#v\n got %#v", tc.videos, app.Videos)
			}
			if !reflect.DeepEqual(app.AllowedImageCounts, tc.counts) {
				t.Fatalf("image counts mismatch\nwant %#v\n got %#v", tc.counts, app.AllowedImageCounts)
			}
			if !reflect.DeepEqual(app.Resolutions, tc.resolutions) {
				t.Fatalf("resolutions mismatch\nwant %#v\n got %#v", tc.resolutions, app.Resolutions)
			}
			if !reflect.DeepEqual(app.QualityModes, tc.qualities) {
				t.Fatalf("quality modes mismatch\nwant %#v\n got %#v", tc.qualities, app.QualityModes)
			}
			if app.Audio != tc.audio || app.StartFrame != tc.startFrame || app.EndFrame != tc.endFrame {
				t.Fatalf("audio/frame mismatch: audio=%#v start=%q end=%q", app.Audio, app.StartFrame, app.EndFrame)
			}
		})
	}
}

func TestVideoCapabilitiesKeepNativeUnknownVideoSeparateFromApplicationPath(t *testing.T) {
	t.Parallel()

	seedance := capabilityForVideoRoute(t, domain.VideoRouteSeedance25)
	if seedance.API.Video.Videos.Support != Unknown || seedance.API.Video.Videos.MaxCount != nil {
		t.Fatalf("native video input should stay unknown when not documented, got %#v", seedance.API.Video.Videos)
	}
	if !reflect.DeepEqual(seedance.Application.Video.Videos, noInput()) {
		t.Fatalf("application video input should be unsupported, got %#v", seedance.Application.Video.Videos)
	}
	if seedance.API.Video.Images.Support != Supported || seedance.API.Video.Images.MaxCount != nil || len(seedance.API.Video.AllowedImageCounts) != 0 {
		t.Fatal("native image limit must not be inferred from product or adapter safety guard")
	}
	expectImageInput(t, seedance.Application.Video.Images, 4)

	runway := capabilityForVideoRoute(t, domain.VideoRouteRunwayGen4Turbo)
	if !reflect.DeepEqual(runway.API.Video.Duration, selectedDurations([]int{5, 10})) {
		t.Fatalf("Runway API duration should not expose wider safety guard, got %#v", runway.API.Video.Duration)
	}
	if !reflect.DeepEqual(runway.Application.Video.Duration, selectedDurations([]int{5, 10})) {
		t.Fatalf("Runway application duration should expose route contract, got %#v", runway.Application.Video.Duration)
	}
}
func TestVideoCapabilitiesForOptionsFiltersApplicationOnly(t *testing.T) {
	t.Parallel()

	caps := VideoCapabilitiesForOptions(string(domain.VideoRouteHailuo23Standard), []int{6, 10}, []string{"1080p"})
	if caps == nil || caps.API.Video == nil || caps.Application.Video == nil {
		t.Fatal("capabilities are nil")
	}
	if !reflect.DeepEqual(caps.API.Video.Resolutions, []string{"768p", "1080p"}) {
		t.Fatalf("API resolutions should stay immutable, got %#v", caps.API.Video.Resolutions)
	}
	if !reflect.DeepEqual(caps.Application.Video.Resolutions, []string{"1080p"}) {
		t.Fatalf("application resolutions were not filtered, got %#v", caps.Application.Video.Resolutions)
	}
	if !reflect.DeepEqual(caps.Application.Video.Duration.AllowedSeconds, []int{6}) {
		t.Fatalf("1080p Hailuo should allow only 6s after filtering, got %#v", caps.Application.Video.Duration.AllowedSeconds)
	}
	if !reflect.DeepEqual(caps.Application.Video.Duration.ByResolution, map[string][]int{"1080p": {6}}) {
		t.Fatalf("by-resolution durations not filtered, got %#v", caps.Application.Video.Duration.ByResolution)
	}

	omni := VideoCapabilitiesForOptions(string(domain.VideoRouteOmni11Flash), []int{10}, []string{"720p"})
	if omni.Application.Video.Duration.Mode != "automatic" || len(omni.Application.Video.Duration.AllowedSeconds) != 0 || *omni.Application.Video.Duration.MinSeconds != 3 || *omni.Application.Video.Duration.MaxSeconds != 10 {
		t.Fatalf("automatic Omni must expose output bounds without a selectable billing duration, got %#v", omni.Application.Video.Duration)
	}
	if omni.API.Video.Duration.Mode != "automatic" || *omni.API.Video.Duration.MinSeconds != 3 || *omni.API.Video.Duration.MaxSeconds != 10 {
		t.Fatalf("API automatic output bounds should stay immutable, got %#v", omni.API.Video.Duration)
	}
}

func TestCapabilitiesExcludeLoadtestVideoRoute(t *testing.T) {
	t.Parallel()

	if caps := Capabilities(string(domain.VideoRouteMockTextToVideo)); caps != nil {
		t.Fatalf("loadtest mock video route should not be public capabilities: %#v", caps)
	}
}

func capabilityForVideoRoute(t *testing.T, alias domain.VideoRouteAlias) *ModelCapabilities {
	t.Helper()
	route, ok := StaticRegistry().VideoRoute(alias)
	if !ok {
		t.Fatalf("missing video route %s", alias)
	}
	caps := videoCapabilities(route)
	if caps == nil || caps.API.Video == nil || caps.Application.Video == nil {
		t.Fatalf("missing video capabilities for %s: %#v", alias, caps)
	}
	return caps
}

func expectInputCapability(t *testing.T, got InputCapability, support Support, max int, extensions []string) {
	t.Helper()
	want := inputCapability(support, integer(max), extensions...)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("input capability mismatch\nwant %#v\n got %#v", want, got)
	}
}
func expectImageInput(t *testing.T, got InputCapability, max int) {
	t.Helper()
	if max == 0 {
		if !reflect.DeepEqual(got, noInput()) {
			t.Fatalf("image input mismatch\nwant %#v\n got %#v", noInput(), got)
		}
		return
	}
	want := inputCapability(Supported, integer(max), "jpg", "jpeg", "png")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("image input mismatch\nwant %#v\n got %#v", want, got)
	}
}

func selectedDuration(min, max int) DurationCapability {
	return selectedDurations(intRange(min, max))
}

func selectedDurations(values []int) DurationCapability {
	out := DurationCapability{Mode: "selected", AllowedSeconds: append([]int(nil), values...)}
	if len(values) > 0 {
		out.MinSeconds = integer(values[0])
		out.MaxSeconds = integer(values[len(values)-1])
	}
	return out
}

func fixedDuration(value int) DurationCapability { return selectedDurations([]int{value}) }

func referenceVideoDuration() DurationCapability {
	return DurationCapability{Mode: "reference_video", MinSeconds: integer(3), MaxSeconds: integer(30), AllowedSeconds: intRange(3, 30), ByOrientation: map[string]int{"image": 10, "video": 30}}
}

func hailuoDuration() DurationCapability {
	return DurationCapability{Mode: "selected", MinSeconds: integer(6), MaxSeconds: integer(10), AllowedSeconds: []int{6, 10}, ByResolution: map[string][]int{"768p": {6, 10}, "1080p": {6}}}
}

func intRange(min, max int) []int {
	out := make([]int, 0, max-min+1)
	for n := min; n <= max; n++ {
		out = append(out, n)
	}
	return out
}
