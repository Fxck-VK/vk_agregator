package mediaprobe

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"vk-ai-aggregator/internal/domain"
)

type fakeRunner struct {
	out      string
	err      error
	deadline bool
}

func (r *fakeRunner) Run(ctx context.Context, _ string, _ []string, _ []byte) ([]byte, error) {
	_, r.deadline = ctx.Deadline()
	if r.err != nil {
		return nil, r.err
	}
	return []byte(r.out), nil
}

type blockingRunner struct{}

func (blockingRunner) Run(ctx context.Context, _ string, _ []string, _ []byte) ([]byte, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestFFProbeParsesAndValidatesVideoMetadata(t *testing.T) {
	runner := &fakeRunner{out: validProbeJSON()}
	prober := NewFFProbe(testConfig(), WithRunner(runner))

	metadata, err := prober.ProbeVideo(context.Background(), []byte("video"), 1024)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !runner.deadline {
		t.Fatal("expected probe context deadline")
	}
	if metadata.ProbeStatus != domain.MediaProbePassed {
		t.Fatalf("ProbeStatus = %q, want passed", metadata.ProbeStatus)
	}
	if metadata.Codec != "h264" || metadata.Container != "mp4" {
		t.Fatalf("unexpected codec/container: %+v", metadata)
	}
	if metadata.Width != 1280 || metadata.Height != 720 || metadata.DurationMS != 5120 || metadata.BitrateBPS != 2500000 {
		t.Fatalf("unexpected numeric metadata: %+v", metadata)
	}
}

func TestFFProbeRejectsDisallowedCodec(t *testing.T) {
	cfg := testConfig()
	cfg.AllowedVideoCodecs = []string{"vp9"}
	prober := NewFFProbe(cfg, WithRunner(&fakeRunner{out: validProbeJSON()}))

	metadata, err := prober.ProbeVideo(context.Background(), []byte("video"), 1024)
	if err == nil {
		t.Fatal("expected probe error")
	}
	if metadata.ProbeStatus != domain.MediaProbeFailed {
		t.Fatalf("ProbeStatus = %q, want failed", metadata.ProbeStatus)
	}
	if !strings.Contains(err.Error(), "video_codec_not_allowed") {
		t.Fatalf("unexpected safe error: %v", err)
	}
}

func TestFFProbeRejectsOversizeBeforeRunningCommand(t *testing.T) {
	runner := &fakeRunner{out: validProbeJSON()}
	cfg := testConfig()
	cfg.MaxVideoSizeBytes = 10
	prober := NewFFProbe(cfg, WithRunner(runner))

	_, err := prober.ProbeVideo(context.Background(), []byte("video"), 11)
	if err == nil || !strings.Contains(err.Error(), "video_size_exceeded") {
		t.Fatalf("expected size error, got %v", err)
	}
	if runner.deadline {
		t.Fatal("runner should not execute when size is already rejected")
	}
}

func TestFFProbeTimeoutReturnsSafeError(t *testing.T) {
	cfg := testConfig()
	cfg.Timeout = time.Millisecond
	prober := NewFFProbe(cfg, WithRunner(blockingRunner{}))

	_, err := prober.ProbeVideo(context.Background(), []byte("video"), 1024)
	if err == nil || !strings.Contains(err.Error(), "probe_timeout") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestFFProbeRunnerFailureDoesNotLeakRawDetails(t *testing.T) {
	prober := NewFFProbe(testConfig(), WithRunner(&fakeRunner{
		err: errors.New("LEAK_PATH_MARKER LEAK_AUTH_MARKER LEAK_STDERR_MARKER"),
	}))

	_, err := prober.ProbeVideo(context.Background(), []byte("video"), 1024)
	if err == nil {
		t.Fatal("expected probe error")
	}
	for _, forbidden := range []string{"LEAK_PATH_MARKER", "LEAK_AUTH_MARKER", "LEAK_STDERR_MARKER"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("probe error leaked %q in %q", forbidden, err.Error())
		}
	}
}

func testConfig() Config {
	return Config{
		FFProbePath:            "ffprobe",
		MaxVideoSizeBytes:      1024 << 20,
		MaxVideoDurationSec:    60,
		MaxVideoWidth:          1920,
		MaxVideoHeight:         1080,
		MaxVideoBitrate:        8000000,
		AllowedVideoContainers: []string{"mp4", "webm"},
		AllowedVideoCodecs:     []string{"h264", "vp9"},
		Timeout:                time.Second,
	}
}

func validProbeJSON() string {
	return `{
		"streams": [
			{
				"codec_type": "video",
				"codec_name": "h264",
				"width": 1280,
				"height": 720,
				"duration": "5.120000",
				"bit_rate": "2500000"
			}
		],
		"format": {
			"format_name": "mov,mp4,m4a,3gp,3g2,mj2",
			"duration": "5.120000",
			"bit_rate": "2500000"
		}
	}`
}
