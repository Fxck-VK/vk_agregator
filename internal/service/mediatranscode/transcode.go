// Package mediatranscode creates bounded delivery-ready media variants.
package mediatranscode

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"vk-ai-aggregator/internal/domain"
)

const (
	defaultTranscodeTimeout = 10 * time.Minute
	defaultMaxOutputBytes   = 256 << 20
	vkVideoMimeType         = "video/mp4"
)

// Config controls ffmpeg execution for VK-ready video variants.
type Config struct {
	FFmpegPath          string
	MaxVideoSizeBytes   int64
	MaxVideoWidth       int
	MaxVideoHeight      int
	MaxVideoBitrate     int64
	TranscodeTimeout    time.Duration
	AudioBitrate        string
	VideoEncoderPreset  string
	VideoEncoderProfile string
	TempDir             string
}

// TranscodeError is safe for job error fields and traces. It never includes raw
// temp paths, provider payloads, prompts, ffmpeg stderr or private URLs.
type TranscodeError struct {
	Reason string
}

func (e TranscodeError) Error() string {
	if e.Reason == "" {
		return "mediatranscode: transcode_failed"
	}
	return "mediatranscode: " + e.Reason
}

type commandRunner interface {
	Run(ctx context.Context, path string, args []string, stdin []byte) error
}

// Option customizes FFmpeg.
type Option func(*FFmpeg)

// WithRunner replaces the command runner, primarily for tests.
func WithRunner(r commandRunner) Option {
	return func(t *FFmpeg) {
		if r != nil {
			t.runner = r
		}
	}
}

// FFmpeg transcodes video bytes through ffmpeg into VK-ready MP4/H.264.
type FFmpeg struct {
	cfg    Config
	runner commandRunner
}

// NewFFmpeg builds a transcoder. It does not check tool existence up front;
// missing ffmpeg fails controlled at transcode time.
func NewFFmpeg(cfg Config, opts ...Option) *FFmpeg {
	if cfg.TranscodeTimeout <= 0 {
		cfg.TranscodeTimeout = defaultTranscodeTimeout
	}
	if cfg.MaxVideoSizeBytes <= 0 {
		cfg.MaxVideoSizeBytes = defaultMaxOutputBytes
	}
	if strings.TrimSpace(cfg.AudioBitrate) == "" {
		cfg.AudioBitrate = "128k"
	}
	if strings.TrimSpace(cfg.VideoEncoderPreset) == "" {
		cfg.VideoEncoderPreset = "veryfast"
	}
	if strings.TrimSpace(cfg.VideoEncoderProfile) == "" {
		cfg.VideoEncoderProfile = "high"
	}
	t := &FFmpeg{cfg: cfg, runner: execRunner{}}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// TranscodeVKVideo returns MP4/H.264 bytes prepared for VK upload. The caller
// should probe the returned bytes before persisting the variant as ready.
func (t *FFmpeg) TranscodeVKVideo(ctx context.Context, data []byte, metadata domain.ArtifactMediaMetadata) ([]byte, domain.ArtifactMediaMetadata, error) {
	if len(data) == 0 {
		return nil, failedMetadata(), TranscodeError{Reason: "empty_video"}
	}
	if int64(len(data)) > t.cfg.MaxVideoSizeBytes {
		return nil, failedMetadata(), TranscodeError{Reason: "video_size_exceeded"}
	}
	if strings.TrimSpace(t.cfg.FFmpegPath) == "" {
		return nil, failedMetadata(), TranscodeError{Reason: "ffmpeg_not_configured"}
	}

	tmp, err := os.CreateTemp(t.cfg.TempDir, "vk-ai-video-*.mp4")
	if err != nil {
		return nil, failedMetadata(), TranscodeError{Reason: "temp_output_unavailable"}
	}
	outPath := tmp.Name()
	if closeErr := tmp.Close(); closeErr != nil {
		_ = os.Remove(outPath)
		return nil, failedMetadata(), TranscodeError{Reason: "temp_output_unavailable"}
	}
	defer func() {
		_ = os.Remove(outPath)
	}()

	transcodeCtx, cancel := context.WithTimeout(ctx, t.cfg.TranscodeTimeout)
	defer cancel()
	if err := t.runner.Run(transcodeCtx, t.cfg.FFmpegPath, t.args(outPath), data); err != nil {
		if transcodeCtx.Err() != nil {
			return nil, failedMetadata(), TranscodeError{Reason: "transcode_timeout"}
		}
		return nil, failedMetadata(), TranscodeError{Reason: "transcode_failed"}
	}

	out, err := readBoundedFile(outPath, t.cfg.MaxVideoSizeBytes)
	if err != nil {
		return nil, failedMetadata(), err
	}
	return out, t.expectedMetadata(metadata), nil
}

func (t *FFmpeg) args(outputPath string) []string {
	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-i", "pipe:0",
		"-map", "0:v:0",
		"-map", "0:a?",
		"-c:v", "libx264",
		"-preset", t.cfg.VideoEncoderPreset,
		"-profile:v", t.cfg.VideoEncoderProfile,
		"-pix_fmt", "yuv420p",
	}
	if filter := t.scaleFilter(); filter != "" {
		args = append(args, "-vf", filter)
	}
	if t.cfg.MaxVideoBitrate > 0 {
		bitrate := strconv.FormatInt(t.cfg.MaxVideoBitrate, 10)
		args = append(args,
			"-b:v", bitrate,
			"-maxrate", bitrate,
			"-bufsize", strconv.FormatInt(t.cfg.MaxVideoBitrate*2, 10),
		)
	}
	args = append(args,
		"-c:a", "aac",
		"-b:a", t.cfg.AudioBitrate,
		"-ac", "2",
		"-movflags", "+faststart",
		"-f", "mp4",
		"-fs", strconv.FormatInt(t.cfg.MaxVideoSizeBytes, 10),
		outputPath,
	)
	return args
}

func (t *FFmpeg) scaleFilter() string {
	if t.cfg.MaxVideoWidth <= 0 || t.cfg.MaxVideoHeight <= 0 {
		return ""
	}
	return fmt.Sprintf("scale=w='min(%d,iw)':h='min(%d,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2", t.cfg.MaxVideoWidth, t.cfg.MaxVideoHeight)
}

func (t *FFmpeg) expectedMetadata(source domain.ArtifactMediaMetadata) domain.ArtifactMediaMetadata {
	metadata := source.Normalize()
	metadata.Codec = "h264"
	metadata.Container = "mp4"
	metadata.ProbeStatus = domain.MediaProbePending
	if t.cfg.MaxVideoBitrate > 0 && (metadata.BitrateBPS == 0 || metadata.BitrateBPS > t.cfg.MaxVideoBitrate) {
		metadata.BitrateBPS = t.cfg.MaxVideoBitrate
	}
	if t.cfg.MaxVideoWidth > 0 && metadata.Width > t.cfg.MaxVideoWidth {
		metadata.Width = t.cfg.MaxVideoWidth
	}
	if t.cfg.MaxVideoHeight > 0 && metadata.Height > t.cfg.MaxVideoHeight {
		metadata.Height = t.cfg.MaxVideoHeight
	}
	return metadata.Normalize()
}

func failedMetadata() domain.ArtifactMediaMetadata {
	return domain.ArtifactMediaMetadata{ProbeStatus: domain.MediaProbeFailed}
}

func readBoundedFile(path string, maxBytes int64) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, TranscodeError{Reason: "output_missing"}
	}
	if info.Size() <= 0 {
		return nil, TranscodeError{Reason: "output_empty"}
	}
	if info.Size() > maxBytes {
		return nil, TranscodeError{Reason: "output_size_exceeded"}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, TranscodeError{Reason: "output_unreadable"}
	}
	defer func() {
		_ = f.Close()
	}()
	out, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, TranscodeError{Reason: "output_unreadable"}
	}
	if int64(len(out)) > maxBytes {
		return nil, TranscodeError{Reason: "output_size_exceeded"}
	}
	return out, nil
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, path string, args []string, stdin []byte) error {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed")
	}
	return nil
}

// VKVideoMimeType is the content type produced by TranscodeVKVideo.
func VKVideoMimeType() string {
	return vkVideoMimeType
}
