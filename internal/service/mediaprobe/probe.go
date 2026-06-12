// Package mediaprobe extracts bounded, safe media facts from video bytes.
package mediaprobe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"vk-ai-aggregator/internal/domain"
)

const defaultProbeTimeout = 10 * time.Second

// Config controls ffprobe execution and media validation.
type Config struct {
	FFProbePath            string
	MaxVideoSizeBytes      int64
	MaxVideoDurationSec    int
	MaxVideoWidth          int
	MaxVideoHeight         int
	MaxVideoBitrate        int64
	AllowedVideoContainers []string
	AllowedVideoCodecs     []string
	Timeout                time.Duration
}

// ProbeError is intentionally safe to store in job error fields or traces. It
// never includes raw paths, URLs, ffprobe stderr or provider payloads.
type ProbeError struct {
	Reason string
}

func (e ProbeError) Error() string {
	if e.Reason == "" {
		return "mediaprobe: probe_failed"
	}
	return "mediaprobe: " + e.Reason
}

type commandRunner interface {
	Run(ctx context.Context, path string, args []string, stdin []byte) ([]byte, error)
}

// Option customizes FFProbe.
type Option func(*FFProbe)

// WithRunner replaces the command runner, primarily for tests.
func WithRunner(r commandRunner) Option {
	return func(p *FFProbe) {
		if r != nil {
			p.runner = r
		}
	}
}

// FFProbe probes video bytes through ffprobe JSON output.
type FFProbe struct {
	cfg    Config
	runner commandRunner
}

// NewFFProbe builds a video prober. It does not check tool existence up front;
// missing ffprobe fails controlled at probe time.
func NewFFProbe(cfg Config, opts ...Option) *FFProbe {
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultProbeTimeout
	}
	cfg.AllowedVideoContainers = normalizeList(cfg.AllowedVideoContainers)
	cfg.AllowedVideoCodecs = normalizeList(cfg.AllowedVideoCodecs)
	p := &FFProbe{cfg: cfg, runner: execRunner{}}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// ProbeVideo validates video bytes and returns safe metadata. Input bytes are
// passed to ffprobe through stdin, so no private temp path is needed.
func (p *FFProbe) ProbeVideo(ctx context.Context, data []byte, sizeBytes int64) (domain.ArtifactMediaMetadata, error) {
	actualSize := sizeBytes
	if actualSize <= 0 {
		actualSize = int64(len(data))
	}
	if actualSize <= 0 || len(data) == 0 {
		return failedMetadata(), ProbeError{Reason: "empty_video"}
	}
	if p.cfg.MaxVideoSizeBytes > 0 && actualSize > p.cfg.MaxVideoSizeBytes {
		return failedMetadata(), ProbeError{Reason: "video_size_exceeded"}
	}
	if strings.TrimSpace(p.cfg.FFProbePath) == "" {
		return failedMetadata(), ProbeError{Reason: "ffprobe_not_configured"}
	}

	probeCtx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()
	out, err := p.runner.Run(probeCtx, p.cfg.FFProbePath, []string{
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-i", "pipe:0",
	}, data)
	if err != nil {
		if probeCtx.Err() != nil {
			return failedMetadata(), ProbeError{Reason: "probe_timeout"}
		}
		return failedMetadata(), ProbeError{Reason: "probe_failed"}
	}

	metadata, err := p.parseAndValidate(out)
	if err != nil {
		return metadata, err
	}
	metadata.ProbeStatus = domain.MediaProbePassed
	return metadata, nil
}

func (p *FFProbe) parseAndValidate(raw []byte) (domain.ArtifactMediaMetadata, error) {
	var payload ffprobeOutput
	if err := json.Unmarshal(raw, &payload); err != nil {
		return failedMetadata(), ProbeError{Reason: "probe_json_invalid"}
	}

	stream := firstVideoStream(payload.Streams)
	if stream == nil {
		return failedMetadata(), ProbeError{Reason: "video_stream_missing"}
	}

	metadata := domain.ArtifactMediaMetadata{
		Width:       stream.Width,
		Height:      stream.Height,
		Codec:       normalizeToken(stream.CodecName),
		Container:   p.allowedContainer(payload.Format.FormatName),
		DurationMS:  durationMillis(firstNonZero(stream.Duration, payload.Format.Duration)),
		BitrateBPS:  firstPositive(parseInt64(stream.BitRate), parseInt64(payload.Format.BitRate)),
		ProbeStatus: domain.MediaProbeFailed,
	}.Normalize()

	if metadata.Container == "" {
		return metadata, ProbeError{Reason: "video_container_not_allowed"}
	}
	if !containsToken(p.cfg.AllowedVideoCodecs, metadata.Codec) {
		return metadata, ProbeError{Reason: "video_codec_not_allowed"}
	}
	if metadata.Width <= 0 || metadata.Height <= 0 {
		return metadata, ProbeError{Reason: "video_dimensions_missing"}
	}
	if p.cfg.MaxVideoWidth > 0 && metadata.Width > p.cfg.MaxVideoWidth {
		return metadata, ProbeError{Reason: "video_width_exceeded"}
	}
	if p.cfg.MaxVideoHeight > 0 && metadata.Height > p.cfg.MaxVideoHeight {
		return metadata, ProbeError{Reason: "video_height_exceeded"}
	}
	if metadata.DurationMS <= 0 {
		return metadata, ProbeError{Reason: "video_duration_missing"}
	}
	if p.cfg.MaxVideoDurationSec > 0 && metadata.DurationMS > int64(p.cfg.MaxVideoDurationSec)*1000 {
		return metadata, ProbeError{Reason: "video_duration_exceeded"}
	}
	if metadata.BitrateBPS <= 0 {
		return metadata, ProbeError{Reason: "video_bitrate_missing"}
	}
	if p.cfg.MaxVideoBitrate > 0 && metadata.BitrateBPS > p.cfg.MaxVideoBitrate {
		return metadata, ProbeError{Reason: "video_bitrate_exceeded"}
	}
	metadata.ProbeStatus = domain.MediaProbePassed
	return metadata, nil
}

func (p *FFProbe) allowedContainer(formatName string) string {
	for _, token := range strings.Split(formatName, ",") {
		token = normalizeToken(token)
		if containsToken(p.cfg.AllowedVideoContainers, token) {
			return token
		}
	}
	return ""
}

func failedMetadata() domain.ArtifactMediaMetadata {
	return domain.ArtifactMediaMetadata{ProbeStatus: domain.MediaProbeFailed}
}

type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	CodecType string `json:"codec_type"`
	CodecName string `json:"codec_name"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Duration  string `json:"duration"`
	BitRate   string `json:"bit_rate"`
}

type ffprobeFormat struct {
	FormatName string `json:"format_name"`
	Duration   string `json:"duration"`
	BitRate    string `json:"bit_rate"`
}

func firstVideoStream(streams []ffprobeStream) *ffprobeStream {
	for i := range streams {
		if strings.EqualFold(strings.TrimSpace(streams[i].CodecType), "video") {
			return &streams[i]
		}
	}
	return nil
}

func durationMillis(value string) int64 {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || seconds <= 0 {
		return 0
	}
	return int64(math.Round(seconds * 1000))
}

func parseInt64(value string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func firstNonZero(values ...string) string {
	for _, value := range values {
		if durationMillis(value) > 0 {
			return value
		}
	}
	return ""
}

func firstPositive(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func normalizeList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		token := normalizeToken(value)
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}

func containsToken(values []string, want string) bool {
	want = normalizeToken(want)
	for _, value := range values {
		if normalizeToken(value) == want {
			return true
		}
	}
	return false
}

func normalizeToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-' || r == '.' || r == '+':
			b.WriteRune(r)
		}
		if b.Len() >= 64 {
			break
		}
	}
	return strings.Trim(b.String(), "_.+-")
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, path string, args []string, stdin []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed")
	}
	return out, nil
}
