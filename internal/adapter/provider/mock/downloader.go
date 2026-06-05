package mock

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxDataURLBytes = 256 << 20

// Downloader resolves the mock provider's "mock://" output URLs into concrete
// bytes so the artifact service can store them, accepts provider-normalized
// data: URLs, and falls back to plain HTTP for any other public provider URL.
// The mock provider returns synthetic output URLs (it has no real backend), so a
// matching synthetic downloader is required to run the full pipeline against
// real storage.
type Downloader struct {
	client *http.Client
}

// NewDownloader builds a mock-aware Downloader.
func NewDownloader() *Downloader {
	return &Downloader{client: http.DefaultClient}
}

// Download returns deterministic synthetic content for mock:// URLs, keyed off
// the file extension the mock provider encodes; decodes data: URLs from real
// providers; or performs a real HTTP GET otherwise.
func (d *Downloader) Download(ctx context.Context, rawURL string) ([]byte, string, error) {
	if strings.HasPrefix(rawURL, "mock://") {
		return mockContent(rawURL)
	}
	if strings.HasPrefix(rawURL, "data:") {
		return decodeDataURL(rawURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
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

func decodeDataURL(raw string) ([]byte, string, error) {
	const prefix = "data:"
	headerAndData := strings.SplitN(strings.TrimPrefix(raw, prefix), ",", 2)
	if len(headerAndData) != 2 {
		return nil, "", fmt.Errorf("malformed data url")
	}
	header := headerAndData[0]
	payload := headerAndData[1]
	contentType := "text/plain;charset=US-ASCII"
	if header != "" {
		parts := strings.Split(header, ";")
		if parts[0] != "" {
			contentType = parts[0]
		}
		if parts[len(parts)-1] == "base64" {
			data, err := base64.StdEncoding.DecodeString(payload)
			if err != nil {
				return nil, "", fmt.Errorf("decode data url: %w", err)
			}
			if len(data) > maxDataURLBytes {
				return nil, "", fmt.Errorf("data url too large")
			}
			return data, contentType, nil
		}
	}
	data, err := url.QueryUnescape(payload)
	if err != nil {
		return nil, "", fmt.Errorf("decode data url: %w", err)
	}
	if len(data) > maxDataURLBytes {
		return nil, "", fmt.Errorf("data url too large")
	}
	return []byte(data), contentType, nil
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
