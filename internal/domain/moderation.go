package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ModerationStage is the pipeline point at which a moderation check runs.
type ModerationStage string

const (
	// ModerationStageInput checks the user's request before processing.
	ModerationStageInput ModerationStage = "input"
	// ModerationStageOutput checks generated content before delivery.
	ModerationStageOutput ModerationStage = "output"
)

// ModerationDecision is the verdict of a moderation check.
type ModerationDecision string

const (
	// ModerationAllow permits the content to proceed.
	ModerationAllow ModerationDecision = "allow"
	// ModerationBlock forbids delivery of the content.
	ModerationBlock ModerationDecision = "block"
	// ModerationReview flags the content for manual review.
	ModerationReview ModerationDecision = "review"
	// ModerationSanitize permits the content after sanitization.
	ModerationSanitize ModerationDecision = "sanitize"
)

// Allowed reports whether the decision permits delivery to the user.
func (d ModerationDecision) Allowed() bool {
	return d == ModerationAllow || d == ModerationSanitize
}

// ModerationResult is the persisted audit record of one moderation check
// (invariant #15: no user output before moderation passes).
type ModerationResult struct {
	// ID is the internal primary key.
	ID uuid.UUID `json:"id"`
	// JobID is the job the check belongs to.
	JobID uuid.UUID `json:"job_id"`
	// ArtifactID is the moderated output artifact, nil for input-stage checks.
	ArtifactID *uuid.UUID `json:"artifact_id,omitempty"`
	// Stage is input or output.
	Stage ModerationStage `json:"stage"`
	// Decision is the verdict.
	Decision ModerationDecision `json:"decision"`
	// Categories are the matched policy categories, if any.
	Categories []string `json:"categories,omitempty"`
	// Provider is the moderation provider that produced the verdict.
	Provider string `json:"provider"`
	// CreatedAt is the row creation timestamp.
	CreatedAt time.Time `json:"created_at"`
}

// ModerationResultRepository persists moderation verdicts for audit.
type ModerationResultRepository interface {
	Create(ctx context.Context, result *ModerationResult) error
	ListByJob(ctx context.Context, jobID uuid.UUID) ([]*ModerationResult, error)
}
