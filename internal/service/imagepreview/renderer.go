package imagepreview

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"time"

	"vk-ai-aggregator/internal/domain"
)

type FFmpegRenderer struct {
	ffmpeg  string
	ffprobe string
	run     func(context.Context, string, []string, []byte, int) ([]byte, error)
	codec   chan struct{}
}

func NewFFmpegRenderer(ffmpeg, ffprobe string) *FFmpegRenderer {
	return &FFmpegRenderer{ffmpeg: ffmpeg, ffprobe: ffprobe, run: runCommand, codec: make(chan struct{}, 1)}
}
func (r *FFmpegRenderer) Render(ctx context.Context, data []byte, mimeType string) ([]byte, domain.ArtifactMediaMetadata, error) {
	if len(data) == 0 || len(data) > MaxSourceBytes || !supportedMime(mimeType) {
		return nil, domain.ArtifactMediaMetadata{}, errUnavailable
	}
	// The API container has 512 MiB shared with Go. Serialize codec processes;
	// source reads can overlap, and saved thumbnails bypass the codec entirely.
	select {
	case r.codec <- struct{}{}:
		defer func() { <-r.codec }()
	case <-ctx.Done():
		return nil, domain.ArtifactMediaMetadata{}, ctx.Err()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	original, err := r.probe(ctx, data)
	if err != nil || original.Width > 40_000_000/original.Height {
		return nil, domain.ArtifactMediaMetadata{}, errUnavailable
	}
	args := []string{"-v", "error", "-nostdin", "-max_alloc", "67108864", "-filter_threads", "1", "-protocol_whitelist", "pipe", "-threads", "1", "-max_pixels", "40000000", "-i", "pipe:0",
		"-map", "0:v:0", "-frames:v", "1", "-vf", `scale=min(640\,iw):min(640\,ih):force_original_aspect_ratio=decrease`,
		"-map_metadata", "-1", "-map_chapters", "-1", "-an", "-c:v", "libwebp", "-quality", "78", "-threads", "1", "-f", "image2pipe", "pipe:1"}
	output, err := r.run(ctx, r.ffmpeg, args, data, MaxThumbnailBytes)
	if err != nil || len(output) < 12 || len(output) > MaxThumbnailBytes || string(output[:4]) != "RIFF" || string(output[8:12]) != "WEBP" {
		return nil, domain.ArtifactMediaMetadata{}, errUnavailable
	}
	metadata, err := r.probe(ctx, output)
	if err != nil || metadata.Width > 640 || metadata.Height > 640 {
		return nil, domain.ArtifactMediaMetadata{}, errUnavailable
	}
	metadata.Codec = "webp"
	metadata.Container = "webp"
	return output, metadata, nil
}
func (r *FFmpegRenderer) probe(ctx context.Context, data []byte) (domain.ArtifactMediaMetadata, error) {
	output, err := r.run(ctx, r.ffprobe, []string{"-v", "error", "-max_alloc", "67108864", "-protocol_whitelist", "pipe", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "json", "-i", "pipe:0"}, data, 64<<10)
	if err != nil {
		return domain.ArtifactMediaMetadata{}, errUnavailable
	}
	var result struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}
	if json.Unmarshal(output, &result) != nil || len(result.Streams) != 1 || result.Streams[0].Width <= 0 || result.Streams[0].Height <= 0 {
		return domain.ArtifactMediaMetadata{}, errUnavailable
	}
	return domain.ArtifactMediaMetadata{Width: result.Streams[0].Width, Height: result.Streams[0].Height}, nil
}

type limitedOutput struct {
	bytes.Buffer
	limit int
}

func (b *limitedOutput) Write(data []byte) (int, error) {
	if len(data) > b.limit-b.Len() {
		return 0, errUnavailable
	}
	return b.Buffer.Write(data)
}
func runCommand(ctx context.Context, path string, args []string, input []byte, limit int) ([]byte, error) {
	// Operator paths, fixed flags and binary stdin only; never a shell or URL input.
	cmd := exec.CommandContext(ctx, path, args...) // #nosec G204
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stderr = io.Discard
	output := &limitedOutput{limit: limit}
	cmd.Stdout = output
	if cmd.Run() != nil {
		return nil, errUnavailable
	}
	return output.Bytes(), nil
}
