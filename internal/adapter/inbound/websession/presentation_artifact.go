package websession

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

const (
	presentationAnswerMIME    = "text/plain; charset=utf-8"
	maxPresentationAnswerSize = 128 * 1024
)

type presentationObjectReader interface {
	GetObjectBounded(context.Context, string, string, int64) ([]byte, error)
}

// readPresentationArtifact recovers a legacy job whose finalizer recorded
// success before writing the assistant row. The caller has already verified the
// original retained prompt, active conversation and account-owned job.
func (h *Handler) readPresentationArtifact(ctx context.Context, job *domain.Job) (string, error) {
	objects, ok := h.deps.ImageArtifactURLSigner.(presentationObjectReader)
	if !ok || h.deps.ImageResults == nil || h.deps.ImageArtifacts == nil || len(job.OutputArtifactIDs) != 1 || job.OutputArtifactIDs[0] == uuid.Nil {
		return "", nil
	}
	// A deleted or redacted message keeps its row. Do not re-expose it through
	// the artifact merely because the normal retained-message read omits it.
	recorded, err := h.deps.Conversations.HasRecordedMessageByJobID(ctx, job.ID, domain.ConversationRoleAssistant)
	if err != nil || recorded {
		return "", err
	}
	result, err := h.deps.ImageResults.GetResult(ctx, job.AccountID, job.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if result.ID != job.ID || result.Status != domain.JobStatusSucceeded || result.Modality != domain.ModalityText || result.Operation != domain.OperationTextGenerate || len(result.Artifacts) != 1 {
		return "", nil
	}
	metadata := result.Artifacts[0]
	if metadata.ID != job.OutputArtifactIDs[0] || metadata.Deleted || metadata.MediaType != domain.MediaTypeText || metadata.MIMEType != presentationAnswerMIME || metadata.SizeBytes <= 0 || metadata.SizeBytes > maxPresentationAnswerSize {
		return "", nil
	}
	artifact, err := h.deps.ImageArtifacts.GetByIDForAccount(ctx, job.AccountID, metadata.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if artifact == nil || artifact.ID != metadata.ID || artifact.OwnerAccountID != job.AccountID || artifact.JobID == nil || *artifact.JobID != job.ID ||
		artifact.Status != domain.ArtifactStatusReady || artifact.Kind != domain.ArtifactKindOutput || artifact.MediaType != metadata.MediaType || artifact.MimeType != metadata.MIMEType ||
		artifact.SizeBytes != metadata.SizeBytes || artifact.StorageBucket == "" || artifact.StorageKey == "" {
		return "", nil
	}
	data, err := objects.GetObjectBounded(ctx, artifact.StorageBucket, artifact.StorageKey, artifact.SizeBytes)
	if err != nil {
		return "", err
	}
	if int64(len(data)) != artifact.SizeBytes || !utf8.Valid(data) {
		return "", nil
	}
	checksum := sha256.Sum256(data)
	if hex.EncodeToString(checksum[:]) != artifact.SHA256 || strings.TrimSpace(string(data)) == "" {
		return "", nil
	}
	return string(data), nil
}
