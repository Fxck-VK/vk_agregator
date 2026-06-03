package mock

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Downloader resolves the mock provider's "mock://" output URLs into concrete
// bytes so the artifact service can store them, and falls back to plain HTTP for
// any other scheme. The mock provider returns synthetic output URLs (it has no
// real backend), so a matching synthetic downloader is required to run the full
// pipeline against real storage.
type Downloader struct {
	client *http.Client
}

// NewDownloader builds a mock-aware Downloader.
func NewDownloader() *Downloader {
	return &Downloader{client: http.DefaultClient}
}

// Download returns deterministic synthetic content for mock:// URLs, keyed off
// the file extension the mock provider encodes, or performs a real HTTP GET
// otherwise.
func (d *Downloader) Download(ctx context.Context, url string) ([]byte, string, error) {
	if strings.HasPrefix(url, "mock://") {
		return mockContent(url)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return data, resp.Header.Get("Content-Type"), nil
}

// mockContent maps a mock:// URL to representative bytes and content type by its
// extension.
func mockContent(url string) ([]byte, string, error) {
	switch {
	case strings.HasSuffix(url, ".txt"):
		return []byte("Mock generated text result.\nsource=" + url + "\n"), "text/plain; charset=utf-8", nil
	case strings.HasSuffix(url, ".png"):
		return onePixelPNG, "image/png", nil
	case strings.HasSuffix(url, ".mp4"):
		return []byte("MOCKMP4\x00minimal-video-bytes"), "video/mp4", nil
	case strings.HasSuffix(url, ".mp3"):
		return []byte("MOCKMP3\x00minimal-audio-bytes"), "audio/mpeg", nil
	default:
		return []byte("mock-binary-output"), "application/octet-stream", nil
	}
}

// onePixelPNG is a valid 1x1 transparent PNG so image artifacts are real,
// decodable files end to end.
var onePixelPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
	0x0D, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x62, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
	0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
}
