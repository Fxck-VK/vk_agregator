package imagepreview

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"vk-ai-aggregator/internal/service/artifactservice"
)

func TestRendererRejectsTooManyPixelsBeforeEncoding(t *testing.T) {
	renderer := NewFFmpegRenderer("ffmpeg", "ffprobe")
	calls := 0
	renderer.run = func(_ context.Context, _ string, _ []string, _ []byte, _ int) ([]byte, error) {
		calls++
		return []byte(`{"streams":[{"width":10000,"height":10000}]}`), nil
	}
	if _, _, err := renderer.Render(context.Background(), []byte("png"), "image/png"); err == nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
func TestRendererUsesBoundedFixedCommandsAndValidatesOutput(t *testing.T) {
	renderer := NewFFmpegRenderer("ffmpeg", "ffprobe")
	calls := 0
	renderer.run = func(_ context.Context, path string, args []string, _ []byte, limit int) ([]byte, error) {
		calls++
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "-protocol_whitelist pipe") || !strings.Contains(joined, "-max_alloc 67108864") {
			t.Fatal("codec protocols or heap block size are unbounded")
		}
		if path == "ffmpeg" {
			if limit != MaxThumbnailBytes || !strings.Contains(joined, "-frames:v 1") || !strings.Contains(joined, "force_original_aspect_ratio=decrease") || !strings.Contains(joined, "-map_metadata -1") || !strings.Contains(joined, "-max_pixels 40000000") || !strings.Contains(joined, "-filter_threads 1") {
				t.Fatalf("unsafe conversion args: %s", joined)
			}
			return []byte("RIFF1234WEBPencoded"), nil
		}
		if calls == 1 {
			return []byte(`{"streams":[{"width":1600,"height":3200}]}`), nil
		}
		return []byte(`{"streams":[{"width":320,"height":640}]}`), nil
	}
	data, metadata, err := renderer.Render(context.Background(), []byte("png"), "image/png")
	if err != nil || len(data) == 0 || metadata.Width != 320 || metadata.Height != 640 {
		t.Fatalf("metadata=%+v err=%v", metadata, err)
	}
}
func TestBoundedProcessOutputStopsAtTheLimit(t *testing.T) {
	out := limitedOutput{limit: 4}
	if _, err := out.Write([]byte("oversized")); err == nil || out.Len() > 4 {
		t.Fatal("unbounded process output")
	}
	renderer := NewFFmpegRenderer("ffmpeg", "ffprobe")
	renderer.run = func(context.Context, string, []string, []byte, int) ([]byte, error) {
		return nil, errors.New("private stderr must not escape")
	}
	if _, _, err := renderer.Render(context.Background(), []byte("png"), "image/png"); err == nil || strings.Contains(err.Error(), "private") {
		t.Fatalf("unsafe error=%v", err)
	}
}
func TestFFmpegRendererIntegration(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not installed")
	}
	renderer := NewFFmpegRenderer(ffmpeg, ffprobe)
	for _, size := range []image.Point{{X: 1800, Y: 3200}, {X: 100, Y: 50}} {
		input := image.NewNRGBA(image.Rect(0, 0, size.X, size.Y))
		input.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
		var source bytes.Buffer
		if err := png.Encode(&source, input); err != nil {
			t.Fatal(err)
		}
		data, metadata, err := renderer.Render(context.Background(), source.Bytes(), "image/png")
		if err != nil {
			t.Fatal(err)
		}
		if string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" || int(binary.LittleEndian.Uint32(data[4:8]))+8 != len(data) {
			t.Fatal("invalid WebP")
		}
		expected := size
		if size.Y > 640 {
			expected = image.Point{X: 360, Y: 640}
		}
		if metadata.Width != expected.X || metadata.Height != expected.Y {
			t.Fatalf("size=%+v metadata=%+v", size, metadata)
		}
		pixels, err := runCommand(context.Background(), ffmpeg, []string{"-v", "error", "-nostdin", "-protocol_whitelist", "pipe", "-i", "pipe:0", "-frames:v", "1", "-vf", "format=rgba,crop=1:1:0:1", "-pix_fmt", "rgba", "-f", "rawvideo", "pipe:1"}, data, 4)
		if err != nil || len(pixels) != 4 || pixels[3] != 0 {
			t.Fatalf("transparent PNG lost its alpha: pixels=%v err=%v", pixels, err)
		}
		if size.X == 100 {
			a, repo, objects, _ := fixture()
			a.SizeBytes = int64(source.Len())
			objects.data["original"] = source.Bytes()
			saver := artifactservice.New(repo, objects, "private")
			if _, err := New(repo, objects, saver, renderer).GetThumbnail(context.Background(), a); err != nil {
				t.Fatal(err)
			}
			delete(objects.data, "original")
			if _, err := New(repo, objects, saver, renderer).GetThumbnail(context.Background(), a); err != nil || objects.reads["original"] != 1 || objects.puts != 1 {
				t.Fatalf("real persisted thumbnail re-read the original: err=%v", err)
			}
		}
	}
	// JPEG orientation must reach the thumbnail, including small images that
	// need rotation but must never be enlarged.
	var jpegSource bytes.Buffer
	if err := jpeg.Encode(&jpegSource, image.NewRGBA(image.Rect(0, 0, 100, 50)), nil); err != nil {
		t.Fatal(err)
	}
	exif := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	oriented := append([]byte{0xff, 0xd8, 0xff, 0xe1, 0, byte(len(exif) + 2)}, exif...)
	oriented = append(oriented, jpegSource.Bytes()[2:]...)
	webp, metadata, err := renderer.Render(context.Background(), oriented, "image/jpeg")
	if err != nil || metadata.Width != 50 || metadata.Height != 100 {
		t.Fatalf("JPEG orientation metadata=%+v err=%v", metadata, err)
	}
	_, repeated, err := renderer.Render(context.Background(), webp, "image/webp")
	if err != nil || repeated.Width != 50 || repeated.Height != 100 {
		t.Fatalf("WebP source metadata=%+v err=%v", repeated, err)
	}
	// AVIF muxing needs a seekable output; the production reader must still be
	// able to accept the resulting complete file through bounded binary stdin.
	avifPath := filepath.Join(t.TempDir(), "synthetic.avif")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = runCommand(ctx, ffmpeg, []string{"-v", "error", "-nostdin", "-protocol_whitelist", "pipe,file", "-i", "pipe:0", "-frames:v", "1", "-c:v", "libaom-av1", "-cpu-used", "8", "-still-picture", "1", "-crf", "40", "-f", "avif", avifPath}, jpegSource.Bytes(), 4096)
	if err != nil {
		t.Fatalf("synthetic AVIF fixture encoding: %v", err)
	}
	avif, err := os.ReadFile(avifPath)
	if err != nil {
		t.Fatal(err)
	}
	_, avifMetadata, err := renderer.Render(context.Background(), avif, "image/avif")
	if err != nil || avifMetadata.Width != 100 || avifMetadata.Height != 50 {
		t.Fatalf("AVIF source metadata=%+v err=%v", avifMetadata, err)
	}
}

func TestRendererSerializesCodecProcessesWithinTheAPIMemoryBudget(t *testing.T) {
	renderer := NewFFmpegRenderer("ffmpeg", "ffprobe")
	var active, maximum atomic.Int32
	entered := make(chan struct{}, 12)
	release := make(chan struct{})
	var unblock sync.Once
	defer unblock.Do(func() { close(release) })
	renderer.run = func(ctx context.Context, path string, _ []string, _ []byte, _ int) ([]byte, error) {
		current := active.Add(1)
		for old := maximum.Load(); current > old && !maximum.CompareAndSwap(old, current); old = maximum.Load() {
		}
		defer active.Add(-1)
		entered <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		if path == "ffmpeg" {
			return []byte("RIFF1234WEBPencoded"), nil
		}
		return []byte(`{"streams":[{"width":100,"height":50}]}`), nil
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := renderer.Render(context.Background(), []byte("png"), "image/png"); err != nil {
				t.Error(err)
			}
		}()
	}
	<-entered
	select {
	case <-entered:
		unblock.Do(func() { close(release) })
		wg.Wait()
		t.Fatal("parallel codec processes can exhaust the 512 MiB API container")
	case <-time.After(100 * time.Millisecond):
	}
	unblock.Do(func() { close(release) })
	wg.Wait()
	if maximum.Load() != 1 {
		t.Fatalf("simultaneous codec processes=%d", maximum.Load())
	}
}
