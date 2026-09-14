// Package modelcontract describes model admission evidence. It performs no
// network requests and never stores credentials or private provider payloads.
package modelcontract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
)

type Support string

const (
	Unknown     Support = "unknown"
	Unsupported Support = "unsupported"
	Supported   Support = "supported"
)

type Contract struct {
	SchemaVersion       int         `json:"schema_version"`
	PublicID            string      `json:"public_id"`
	Provider            string      `json:"provider"`
	ProviderModelID     string      `json:"provider_model_id"`
	Revision            string      `json:"revision"`
	Endpoint            string      `json:"endpoint"`
	RegistryFingerprint string      `json:"registry_fingerprint"`
	Status              string      `json:"status"`
	Categories          []string    `json:"categories"`
	Sources             []Source    `json:"sources"`
	Operations          []Operation `json:"operations"`
	Checks              []Check     `json:"checks"`
}

type Source struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	CheckedAt string `json:"checked_at"`
}

type Check struct {
	Scenario       string `json:"scenario"`
	Status         string `json:"status"`   // passed, failed, not_run
	Evidence       string `json:"evidence"` // sanitized report or test reference
	EvidenceSHA256 string `json:"evidence_sha256"`
	CheckedAt      string `json:"checked_at"`
	ContractDigest string `json:"contract_digest"`
}

type Operation struct {
	ID     string       `json:"id"`
	Kind   string       `json:"kind"` // text, image, video, audio; one output type per operation
	Inputs Inputs       `json:"inputs"`
	Text   *TextOutput  `json:"text,omitempty"`
	Image  *ImageOutput `json:"image,omitempty"`
	Video  *VideoOutput `json:"video,omitempty"`
	Audio  *AudioOutput `json:"audio,omitempty"`
}

type Inputs struct {
	Images        Input `json:"images"`
	Video         Input `json:"video"`
	Audio         Input `json:"audio"`
	Documents     Input `json:"documents"`
	MaxTotalBytes int64 `json:"max_total_bytes"`
}

// Support records the researched provider capability. Enabled records our
// implementation decision; admission additionally requires current evidence.
type Input struct {
	Support        Support      `json:"support"`
	Enabled        bool         `json:"enabled"`
	Required       bool         `json:"required,omitempty"`
	Processing     string       `json:"processing,omitempty"` // native or extracted_text
	Formats        []FileFormat `json:"formats,omitempty"`
	MaxCount       int          `json:"max_count,omitempty"`
	AllowedCounts  []int        `json:"allowed_counts,omitempty"`
	MaxBytes       int64        `json:"max_bytes,omitempty"`
	MaxPages       int          `json:"max_pages,omitempty"`
	MaxDurationSec int          `json:"max_duration_sec,omitempty"`
	MaxWidth       int          `json:"max_width,omitempty"`
	MaxHeight      int          `json:"max_height,omitempty"`
}

type FileFormat struct {
	Extension string `json:"extension"`
	MIME      string `json:"mime"`
}

type TextOutput struct {
	ContextTokens   int `json:"context_tokens"`
	MaxOutputTokens int `json:"max_output_tokens"`
}

// Image variants enumerate valid combinations rather than independent values
// whose Cartesian product might advertise unsupported provider requests.
type ImageVariant struct {
	AspectRatio string `json:"aspect_ratio"`
	Resolution  string `json:"resolution"`
}

type ImageOutput struct {
	Variants              []ImageVariant `json:"variants"`
	DefaultVariant        int            `json:"default_variant"`
	Formats               []string       `json:"formats"`
	MaxOutputCount        int            `json:"max_output_count"`
	Mask                  string         `json:"mask"` // unsupported, optional, required
	TransparentBackground bool           `json:"transparent_background"`
}

type VideoVariant struct {
	DurationSec int    `json:"duration_sec"`
	Resolution  string `json:"resolution"`
	AspectRatio string `json:"aspect_ratio"`
	FPS         int    `json:"fps"`
	Audio       bool   `json:"audio"`
}

type VideoOutput struct {
	Variants       []VideoVariant `json:"variants"`
	DefaultVariant int            `json:"default_variant"`
	Formats        []string       `json:"formats"`
	StartImage     string         `json:"start_image"` // unsupported, optional, required
	EndImage       string         `json:"end_image"`
}

type AudioOutput struct {
	Tasks          []string `json:"tasks"` // transcribe, speech, music, transform
	Languages      []string `json:"languages"`
	Voices         []string `json:"voices"`
	Formats        []string `json:"formats"`
	MaxDurationSec int      `json:"max_duration_sec"`
}

func Decode(r io.Reader) (Contract, error) {
	var c Contract
	d := json.NewDecoder(io.LimitReader(r, 2<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return c, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return c, fmt.Errorf("model contract: expected one JSON document")
	}
	return c, nil
}

// Digest excludes the report and rollout decision, but includes all declared
// capabilities, source revisions and the exact provider registry binding.
func (c Contract) Digest() string {
	c.Checks = nil
	c.Status = ""
	data, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
