package worker

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"strings"
)

const (
	maxSanitizedReferenceImageDimension = 4096
	maxSanitizedReferenceImagePixels    = 4096 * 4096
)

var (
	errInvalidReferenceImage     = errors.New("invalid reference image")
	errUnsupportedReferenceImage = errors.New("unsupported reference image format")
)

func sanitizedReferenceImageDataURL(data []byte, declaredMIME string) (string, error) {
	mime := referenceImageMIME(data, declaredMIME)
	sanitized, outMIME, err := sanitizeReferenceImageBytes(data, mime)
	if err != nil {
		return "", err
	}
	return "data:" + outMIME + ";base64," + base64.StdEncoding.EncodeToString(sanitized), nil
}

func referenceImageMIME(data []byte, declaredMIME string) string {
	if isWebP(data) {
		return "image/webp"
	}
	detected := http.DetectContentType(data)
	switch detected {
	case "image/jpeg", "image/png":
		return detected
	}
	declaredMIME = strings.ToLower(strings.TrimSpace(strings.Split(declaredMIME, ";")[0]))
	switch declaredMIME {
	case "image/jpeg", "image/png", "image/webp":
		return declaredMIME
	default:
		return ""
	}
}

func sanitizeReferenceImageBytes(data []byte, mime string) ([]byte, string, error) {
	switch mime {
	case "image/jpeg", "image/png":
	case "image/webp":
		return nil, "", fmt.Errorf("worker: %w", errUnsupportedReferenceImage)
	default:
		return nil, "", fmt.Errorf("worker: %w", errUnsupportedReferenceImage)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, "", fmt.Errorf("worker: %w", errInvalidReferenceImage)
	}
	if cfg.Width > maxSanitizedReferenceImageDimension || cfg.Height > maxSanitizedReferenceImageDimension {
		return nil, "", fmt.Errorf("worker: %w", errInvalidReferenceImage)
	}
	if cfg.Width*cfg.Height > maxSanitizedReferenceImagePixels {
		return nil, "", fmt.Errorf("worker: %w", errInvalidReferenceImage)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("worker: %w", errInvalidReferenceImage)
	}
	var buf bytes.Buffer
	switch mime {
	case "image/jpeg":
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92}); err != nil {
			return nil, "", fmt.Errorf("worker: %w", errInvalidReferenceImage)
		}
		return buf.Bytes(), "image/jpeg", nil
	case "image/png":
		if err := png.Encode(&buf, img); err != nil {
			return nil, "", fmt.Errorf("worker: %w", errInvalidReferenceImage)
		}
		return buf.Bytes(), "image/png", nil
	default:
		return nil, "", fmt.Errorf("worker: %w", errUnsupportedReferenceImage)
	}
}

func isWebP(data []byte) bool {
	return len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
}
