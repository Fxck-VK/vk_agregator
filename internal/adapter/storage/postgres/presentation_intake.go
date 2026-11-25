package postgres

import (
	"context"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// GetLatestPresentationChatTurn includes accepted jobs before their worker has
// written the first message. The caller already verified the conversation owner.
func (r *JobRepository) GetLatestPresentationChatTurn(ctx context.Context, accountID, conversationID uuid.UUID) (*domain.Job, error) {
	if accountID == uuid.Nil || conversationID == uuid.Nil {
		return nil, domain.ErrInvalidInput
	}
	const query = `SELECT ` + jobColumns + ` FROM jobs
		WHERE account_id = $1 AND params->>'conversation_id' = $2
		AND source='web' AND channel='web' AND result_mode='account_history'
		AND operation_type='text_generate' AND modality='text'
		AND left(idempotency_key, length($3)) = $3
		ORDER BY created_at DESC, id DESC LIMIT 1`
	var job domain.Job
	if err := mapError(scanJob(r.db.QueryRow(ctx, query, accountID, conversationID.String(), domain.WebPresentationIntakeJobKeyPrefix(accountID)), &job)); err != nil {
		return nil, err
	}
	return &job, nil
}
