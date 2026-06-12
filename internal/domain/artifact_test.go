package domain

import (
	"strings"
	"testing"
)

func TestArtifactMediaMetadataNormalize(t *testing.T) {
	metadata := ArtifactMediaMetadata{
		Width:       -1,
		Height:      720,
		DurationMS:  -100,
		Codec:       " H.264 / suspicious:value ",
		Container:   "MP4; DROP",
		BitrateBPS:  -1,
		ProbeStatus: MediaProbeStatus("raw-ffprobe-state"),
	}

	got := metadata.Normalize()
	if got.Width != 0 || got.Height != 720 || got.DurationMS != 0 || got.BitrateBPS != 0 {
		t.Fatalf("unexpected numeric normalization: %+v", got)
	}
	if got.ProbeStatus != MediaProbeUnknown {
		t.Fatalf("ProbeStatus = %q, want unknown", got.ProbeStatus)
	}
	for _, forbidden := range []string{"/", ":", ";", " "} {
		if strings.Contains(got.Codec, forbidden) || strings.Contains(got.Container, forbidden) {
			t.Fatalf("metadata token still contains forbidden %q: codec=%q container=%q", forbidden, got.Codec, got.Container)
		}
	}
	if got.Codec == "" || got.Container == "" {
		t.Fatalf("expected sanitized codec/container, got codec=%q container=%q", got.Codec, got.Container)
	}
}

func TestApplyMediaMetadataDefaultsProbeStatus(t *testing.T) {
	var artifact Artifact
	artifact.ApplyMediaMetadata(ArtifactMediaMetadata{})
	if artifact.ProbeStatus != MediaProbeUnknown {
		t.Fatalf("artifact ProbeStatus = %q, want unknown", artifact.ProbeStatus)
	}

	var variant ArtifactVariant
	variant.ApplyMediaMetadata(ArtifactMediaMetadata{ProbeStatus: MediaProbePassed, Codec: "H.265", Container: "MP4"})
	if variant.ProbeStatus != MediaProbePassed || variant.Codec != "h.265" || variant.Container != "mp4" {
		t.Fatalf("variant metadata not normalized: %+v", variant)
	}
}
