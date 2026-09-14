package modelcontract

import (
	"encoding/hex"
	"fmt"
	"mime"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

func (c Contract) Validate() error {
	if c.SchemaVersion != 1 {
		return fmt.Errorf("model contract: unsupported schema_version")
	}
	for _, value := range []string{c.PublicID, c.Provider, c.ProviderModelID, c.Revision, c.Endpoint} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("model contract: identity, revision and endpoint are required")
		}
	}
	if c.Status != "draft" && c.Status != "ready" {
		return fmt.Errorf("model contract: status must be draft or ready")
	}
	if data, err := hex.DecodeString(c.RegistryFingerprint); err != nil || len(data) != 32 {
		return fmt.Errorf("model contract: registry fingerprint must be SHA-256")
	}
	if err := stringChoices(c.Categories); err != nil {
		return fmt.Errorf("categories: %w", err)
	}
	for _, category := range c.Categories {
		if !slices.Contains([]string{"popular", "images", "text", "video-audio", "free", "study-work"}, category) {
			return fmt.Errorf("unknown category %q", category)
		}
	}
	if len(c.Sources) == 0 {
		return fmt.Errorf("model contract: provider documentation sources are required")
	}
	sources := map[string]bool{}
	for _, source := range c.Sources {
		u, err := url.Parse(source.URL)
		if source.ID == "" || sources[source.ID] || err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || !validDate(source.CheckedAt) {
			return fmt.Errorf("invalid documentation source %q: use public HTTPS URL and checked date", source.ID)
		}
		sources[source.ID] = true
	}
	if len(c.Operations) == 0 {
		return fmt.Errorf("model contract: operations are required")
	}
	seen := map[string]bool{}
	for _, op := range c.Operations {
		if strings.TrimSpace(op.ID) == "" || strings.Contains(op.ID, "/") || seen[op.ID] {
			return fmt.Errorf("operation IDs must be unique and contain no slash")
		}
		seen[op.ID] = true
		if err := op.validate(); err != nil {
			return fmt.Errorf("operation %s: %w", op.ID, err)
		}
	}
	return nil
}

func (op Operation) validate() error {
	count := 0
	for _, present := range []bool{op.Text != nil, op.Image != nil, op.Video != nil, op.Audio != nil} {
		if present {
			count++
		}
	}
	if count != 1 {
		return fmt.Errorf("exactly one typed output is required")
	}
	for _, entry := range op.Inputs.entries() {
		if err := entry.input.validate(entry.name); err != nil {
			return err
		}
		if entry.input.Enabled && op.Inputs.MaxTotalBytes < entry.input.MaxBytes {
			return fmt.Errorf("total attachment limit must cover each file limit")
		}
	}
	if op.Inputs.MaxTotalBytes < 0 {
		return fmt.Errorf("negative total attachment limit")
	}
	switch op.Kind {
	case "text":
		if op.Text == nil || op.Text.ContextTokens <= 0 || op.Text.MaxOutputTokens <= 0 || op.Text.MaxOutputTokens > op.Text.ContextTokens {
			return fmt.Errorf("invalid text context/output limits")
		}
	case "image":
		p := op.Image
		if p == nil || len(p.Variants) == 0 || p.DefaultVariant < 0 || p.DefaultVariant >= len(p.Variants) || p.MaxOutputCount <= 0 {
			return fmt.Errorf("invalid image variants/default/count")
		}
		if err := mediaFormats(p.Formats, "image/"); err != nil {
			return err
		}
		if !requirement(p.Mask) || p.Mask != "unsupported" && !op.Inputs.Images.Enabled {
			return fmt.Errorf("mask requires enabled image input")
		}
		seen := map[ImageVariant]bool{}
		for _, v := range p.Variants {
			if !aspectRatio(v.AspectRatio) || strings.TrimSpace(v.Resolution) == "" || seen[v] {
				return fmt.Errorf("invalid or duplicate image combination")
			}
			seen[v] = true
		}
	case "video":
		p := op.Video
		if p == nil || len(p.Variants) == 0 || p.DefaultVariant < 0 || p.DefaultVariant >= len(p.Variants) {
			return fmt.Errorf("invalid video variants/default")
		}
		if err := mediaFormats(p.Formats, "video/"); err != nil {
			return err
		}
		if !requirement(p.StartImage) || !requirement(p.EndImage) {
			return fmt.Errorf("invalid start/end frame requirement")
		}
		if (p.StartImage != "unsupported" || p.EndImage != "unsupported") && !op.Inputs.Images.Enabled {
			return fmt.Errorf("start/end frame requires enabled image input")
		}
		if p.StartImage == "required" && p.EndImage == "required" && op.Inputs.Images.MaxCount < 2 {
			return fmt.Errorf("two required frames need two image slots")
		}
		seen := map[VideoVariant]bool{}
		for _, v := range p.Variants {
			if v.DurationSec <= 0 || v.FPS <= 0 || !aspectRatio(v.AspectRatio) || strings.TrimSpace(v.Resolution) == "" || seen[v] {
				return fmt.Errorf("invalid or duplicate video combination")
			}
			seen[v] = true
		}
	case "audio":
		p := op.Audio
		if p == nil || p.MaxDurationSec <= 0 {
			return fmt.Errorf("invalid audio duration")
		}
		if err := stringChoices(p.Tasks); err != nil {
			return err
		}
		for _, task := range p.Tasks {
			if !slices.Contains([]string{"transcribe", "speech", "music", "transform"}, task) {
				return fmt.Errorf("unknown audio task %q", task)
			}
			if task == "speech" && (len(p.Voices) == 0 || len(p.Languages) == 0) {
				return fmt.Errorf("speech requires voices and languages")
			}
			if (task == "transcribe" || task == "transform") && !op.Inputs.Audio.Enabled {
				return fmt.Errorf("audio task requires enabled audio input")
			}
		}
		if err := mediaFormats(p.Formats, ""); err != nil {
			return err
		}
		transcription := slices.Contains(p.Tasks, "transcribe")
		audioTask := len(p.Tasks) > 1 || !transcription
		for _, format := range p.Formats {
			if strings.HasPrefix(format, "audio/") && audioTask {
				continue
			}
			if transcription && (format == "text/plain" || format == "application/json") {
				continue
			}
			return fmt.Errorf("audio output format does not match its task")
		}
	default:
		return fmt.Errorf("unknown output kind %q", op.Kind)
	}
	return nil
}

type inputEntry struct {
	name  string
	input Input
}

func (in Inputs) entries() []inputEntry {
	return []inputEntry{{"images", in.Images}, {"video", in.Video}, {"audio", in.Audio}, {"documents", in.Documents}}
}

func (in Input) validate(name string) error {
	if in.Support != Unknown && in.Support != Unsupported && in.Support != Supported {
		return fmt.Errorf("%s: support must be explicit", name)
	}
	if in.MaxBytes < 0 || in.MaxCount < 0 || in.MaxPages < 0 || in.MaxDurationSec < 0 || in.MaxWidth < 0 || in.MaxHeight < 0 {
		return fmt.Errorf("%s: negative limit", name)
	}
	if !in.Enabled {
		if in.Required {
			return fmt.Errorf("%s: disabled input cannot be required", name)
		}
		return nil
	}
	if in.Support != Supported {
		return fmt.Errorf("%s: unknown/unsupported input cannot be enabled", name)
	}
	if in.Processing != "native" && in.Processing != "extracted_text" {
		return fmt.Errorf("%s: processing must be native or extracted_text", name)
	}
	if in.Processing == "extracted_text" && name != "documents" {
		return fmt.Errorf("%s: extracted_text is a document pipeline", name)
	}
	if in.MaxCount <= 0 || in.MaxBytes <= 0 || len(in.Formats) == 0 {
		return fmt.Errorf("%s: formats, count and bytes are required", name)
	}
	counts := map[int]bool{}
	for _, count := range in.AllowedCounts {
		if count < 0 || count > in.MaxCount || in.Required && count == 0 || counts[count] {
			return fmt.Errorf("%s: invalid allowed counts", name)
		}
		counts[count] = true
	}
	if len(counts) > 0 && !counts[in.MaxCount] {
		return fmt.Errorf("%s: max count is not allowed", name)
	}
	if (name == "video" || name == "audio") && in.MaxDurationSec <= 0 {
		return fmt.Errorf("%s: duration limit required", name)
	}
	if name == "documents" && in.MaxPages <= 0 {
		return fmt.Errorf("documents: page limit required")
	}
	seen := map[string]bool{}
	for _, format := range in.Formats {
		if len(format.Extension) < 2 || !strings.HasPrefix(format.Extension, ".") || strings.ContainsAny(format.Extension[1:], ". /\\*") || seen[format.Extension] {
			return fmt.Errorf("%s: invalid/duplicate extension", name)
		}
		seen[format.Extension] = true
		prefix := map[string]string{"images": "image/", "video": "video/", "audio": "audio/"}[name]
		if err := mediaFormats([]string{format.MIME}, prefix); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

func requirement(value string) bool {
	return value == "unsupported" || value == "optional" || value == "required"
}

func aspectRatio(value string) bool {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return false
	}
	a, errA := strconv.Atoi(parts[0])
	b, errB := strconv.Atoi(parts[1])
	return errA == nil && errB == nil && a > 0 && b > 0
}

func stringChoices(values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("nonempty values required")
	}
	seen := map[string]bool{}
	for _, value := range values {
		if strings.TrimSpace(value) == "" || seen[value] {
			return fmt.Errorf("empty/duplicate value")
		}
		seen[value] = true
	}
	return nil
}

func mediaFormats(values []string, prefix string) error {
	if err := stringChoices(values); err != nil {
		return err
	}
	for _, value := range values {
		parsed, params, err := mime.ParseMediaType(value)
		if err != nil || parsed != value || len(params) > 0 || !strings.Contains(value, "/") || strings.Contains(value, "*") || !strings.HasPrefix(value, prefix) {
			return fmt.Errorf("explicit MIME format required: %q", value)
		}
	}
	return nil
}

func validDate(value string) bool {
	d, err := time.Parse("2006-01-02", value)
	return err == nil && !d.After(time.Now().UTC())
}
