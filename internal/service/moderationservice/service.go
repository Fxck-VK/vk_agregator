// Package moderationservice enforces content safety. It exposes a Moderator
// interface so real providers (OpenAI moderation, Google, custom classifiers)
// can be plugged in later, and ships a default keyword-based implementation for
// MVP/local use. Output moderation gates delivery (invariant #15): no user
// output is delivered before a moderation check passes.
package moderationservice

import (
	"context"
	"strings"

	"vk-ai-aggregator/internal/domain"
)

// Input is the content presented for a moderation check.
type Input struct {
	Stage    domain.ModerationStage
	Modality domain.Modality
	// Prompt is the originating user prompt (always available).
	Prompt string
	// Text is the generated/text content for text outputs (may be empty for
	// media, where only metadata is available at MVP).
	Text string
}

// Outcome is the verdict of a moderation check.
type Outcome struct {
	Decision   domain.ModerationDecision
	Categories []string
}

// Moderator checks content and returns a verdict. Implementations must be safe
// for concurrent use.
type Moderator interface {
	// Name identifies the moderation provider for audit records.
	Name() string
	// Check evaluates the input and returns a verdict.
	Check(ctx context.Context, in Input) (Outcome, error)
}

// defaultBannedTerms is the seed blocklist. Real deployments should replace the
// keyword moderator with a provider-backed classifier.
var defaultBannedTerms = []string{
	"banned_content",
	"nsfw",
	"csam",
	"child_abuse",
	"terrorist",
	"make a bomb",
}

// KeywordModerator is a deterministic blocklist moderator. It blocks when the
// prompt or text contains any banned term. It is intentionally simple but
// satisfies the Moderator contract so it can be swapped for a real provider.
type KeywordModerator struct {
	banned []string
}

// NewKeywordModerator builds a KeywordModerator. Extra terms are appended to the
// default blocklist (case-insensitive).
func NewKeywordModerator(extra ...string) *KeywordModerator {
	terms := make([]string, 0, len(defaultBannedTerms)+len(extra))
	terms = append(terms, defaultBannedTerms...)
	for _, t := range extra {
		if t != "" {
			terms = append(terms, strings.ToLower(t))
		}
	}
	return &KeywordModerator{banned: terms}
}

var _ Moderator = (*KeywordModerator)(nil)

// Name returns the moderator identifier.
func (m *KeywordModerator) Name() string { return "keyword" }

// Check blocks when any banned term appears in the prompt or text.
func (m *KeywordModerator) Check(_ context.Context, in Input) (Outcome, error) {
	haystack := strings.ToLower(in.Prompt + "\n" + in.Text)
	for _, term := range m.banned {
		if term != "" && strings.Contains(haystack, term) {
			return Outcome{Decision: domain.ModerationBlock, Categories: []string{"blocklist:" + term}}, nil
		}
	}
	return Outcome{Decision: domain.ModerationAllow}, nil
}
