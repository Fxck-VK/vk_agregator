package mock

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
)

const maxDataURLBytes = 256 << 20

// FallbackDownloader is the minimal contract used to delegate real provider
// URLs back to the platform downloader without coupling this package to the
// artifact service.
type FallbackDownloader interface {
	Download(ctx context.Context, url string) (data []byte, contentType string, err error)
}

// Downloader resolves the mock provider's "mock://" output URLs into concrete
// bytes so the artifact service can store them, accepts provider-normalized
// data: URLs, and delegates any other provider URL to the configured fallback.
// The mock provider returns synthetic output URLs (it has no real backend), so a
// matching synthetic downloader is required to run the full pipeline against
// real storage.
type Downloader struct {
	fallback FallbackDownloader
}

// NewDownloader builds a mock-aware Downloader.
func NewDownloader(fallback ...FallbackDownloader) *Downloader {
	d := &Downloader{}
	if len(fallback) > 0 {
		d.fallback = fallback[0]
	}
	return d
}

// Download returns deterministic synthetic content for mock:// URLs, keyed off
// the file extension the mock provider encodes; decodes data: URLs from real
// providers; or delegates other URLs to the hardened fallback downloader.
func (d *Downloader) Download(ctx context.Context, rawURL string) ([]byte, string, error) {
	if strings.HasPrefix(rawURL, "mock://") {
		return mockContent(rawURL)
	}
	if strings.HasPrefix(rawURL, "data:") {
		return decodeDataURL(rawURL)
	}
	if d.fallback != nil {
		return d.fallback.Download(ctx, rawURL)
	}
	return nil, "", fmt.Errorf("mock downloader: unsupported non-mock url")
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
