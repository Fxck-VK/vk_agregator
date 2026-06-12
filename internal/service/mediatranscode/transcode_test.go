package mediatranscode

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"vk-ai-aggregator/internal/domain"
)

type fakeRunner struct {
	out      []byte
	err      error
	args     []string
	stdin    []byte
	deadline bool
}

func (r *fakeRunner) Run(ctx context.Context, _ string, args []string, stdin []byte) error {
	_, r.deadline = ctx.Deadline()
	r.args = append([]string(nil), args...)
	r.stdin = append([]byte(nil), stdin...)
	if r.err != nil {
		return r.err
	}
	if len(args) == 0 {
		return errors.New("missing output")
	}
	return os.WriteFile(args[len(args)-1], r.out, 0o600)
}

type blockingRunner struct{}

func (blockingRunner) Run(ctx context.Context, _ string, _ []string, _ []byte) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestFFmpegTranscodesVKReadyVideo(t *testing.T) {
	runner := &fakeRunner{out: []byte("vk-ready-mp4")}
	transcoder := NewFFmpeg(testConfig(t), WithRunner(runner))

	out, metadata, err := transcoder.TranscodeVKVideo(context.Background(), []byte("raw-provider-video"), domain.ArtifactMediaMetadata{
		Width:       2560,
		Height:      1440,
		DurationMS:  5000,
		Codec:       "vp9",
		Container:   "webm",
		BitrateBPS:  9000000,
		ProbeStatus: domain.MediaProbePassed,
	})
	if err != nil {
		t.Fatalf("transcode: %v", err)
	}
	if string(out) != "vk-ready-mp4" {
		t.Fatalf("output = %q", string(out))
	}
	if !runner.deadline {
		t.Fatal("expected transcode context deadline")
	}
	if string(runner.stdin) != "raw-provider-video" {
		t.Fatalf("stdin = %q", string(runner.stdin))
	}
	for _, want := range []string{
		"-i", "pipe:0",
		"-map", "0:v:0",
		"-map", "0:a?",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-movflags", "+faststart",
		"-f", "mp4",
		"-fs", "1024",
	} {
		if !containsArg(runner.args, want) {
			t.Fatalf("expected arg %q in %v", want, runner.args)
		}
	}
	if !containsArg(runner.args, "scale=w='min(1920,iw)':h='min(1080,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2") {
		t.Fatalf("expected bounded scale filter in %v", runner.args)
	}
	if metadata.Codec != "h264" || metadata.Container != "mp4" || metadata.ProbeStatus != domain.MediaProbePending {
		t.Fatalf("unexpected metadata: %+v", metadata)
	}
	if metadata.Width != 1920 || metadata.Height != 1080 || metadata.BitrateBPS != 8000000 {
		t.Fatalf("metadata was not bounded: %+v", metadata)
	}
}

func TestFFmpegRejectsOversizeInputBeforeCommand(t *testing.T) {
	runner := &fakeRunner{out: []byte("unused")}
	cfg := testConfig(t)
	cfg.MaxVideoSizeBytes = 3
	transcoder := NewFFmpeg(cfg, WithRunner(runner))

	_, _, err := transcoder.TranscodeVKVideo(context.Background(), []byte("too-large"), domain.ArtifactMediaMetadata{})
	if err == nil || !strings.Contains(err.Error(), "video_size_exceeded") {
		t.Fatalf("expected size error, got %v", err)
	}
	if runner.deadline {
		t.Fatal("runner should not execute when input is too large")
	}
}

func TestFFmpegRejectsOversizeOutput(t *testing.T) {
	runner := &fakeRunner{out: []byte("too-large-output")}
	cfg := testConfig(t)
	cfg.MaxVideoSizeBytes = 4
	transcoder := NewFFmpeg(cfg, WithRunner(runner))

	_, _, err := transcoder.TranscodeVKVideo(context.Background(), []byte("raw"), domain.ArtifactMediaMetadata{})
	if err == nil || !strings.Contains(err.Error(), "output_size_exceeded") {
		t.Fatalf("expected output size error, got %v", err)
	}
}

func TestFFmpegTimeoutReturnsSafeError(t *testing.T) {
	cfg := testConfig(t)
	cfg.TranscodeTimeout = time.Millisecond
	transcoder := NewFFmpeg(cfg, WithRunner(blockingRunner{}))

	_, _, err := transcoder.TranscodeVKVideo(context.Background(), []byte("raw"), domain.ArtifactMediaMetadata{})
	if err == nil || !strings.Contains(err.Error(), "transcode_timeout") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestFFmpegRunnerFailureDoesNotLeakRawDetails(t *testing.T) {
	transcoder := NewFFmpeg(testConfig(t), WithRunner(&fakeRunner{
		err: errors.New("LEAK_PATH_MARKER LEAK_AUTH_MARKER LEAK_STDERR_MARKER"),
	}))

	_, _, err := transcoder.TranscodeVKVideo(context.Background(), []byte("raw"), domain.ArtifactMediaMetadata{})
	if err == nil {
		t.Fatal("expected transcode error")
	}
	for _, forbidden := range []string{"LEAK_PATH_MARKER", "LEAK_AUTH_MARKER", "LEAK_STDERR_MARKER"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("transcode error leaked %q in %q", forbidden, err.Error())
		}
	}
}

func testConfig(t *testing.T) Config {
	t.Helper()
	return Config{
		FFmpegPath:        "ffmpeg",
		MaxVideoSizeBytes: 1024,
		MaxVideoWidth:     1920,
		MaxVideoHeight:    1080,
		MaxVideoBitrate:   8000000,
		TranscodeTimeout:  time.Second,
		TempDir:           t.TempDir(),
	}
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}
