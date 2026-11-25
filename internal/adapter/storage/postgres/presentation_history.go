package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

type PresentationHistoryRepository struct{ db Querier }

func NewPresentationHistoryRepository(db Querier) *PresentationHistoryRepository {
	return &PresentationHistoryRepository{db: db}
}

func (r *PresentationHistoryRepository) ListPresentationHistory(ctx context.Context, accountID uuid.UUID, limit int, after *domain.JobCursor) ([]domain.PresentationHistoryRow, error) {
	if accountID == uuid.Nil || limit < 1 || limit > 51 {
		return nil, domain.ErrInvalidInput
	}
	var at *time.Time
	var id uuid.UUID
	if after != nil {
		if after.CreatedAt.IsZero() || after.ID == uuid.Nil {
			return nil, domain.ErrInvalidInput
		}
		at, id = &after.CreatedAt, after.ID
	}
	marker := "^" + domain.WebPresentationJobKeyPrefix(accountID) + presentationUUIDPattern + "$"
	rows, err := r.db.Query(ctx, presentationHistorySQL, accountID, marker, limit, at, id)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := make([]domain.PresentationHistoryRow, 0, limit)
	for rows.Next() {
		var item domain.PresentationHistoryRow
		if err := rows.Scan(&item.AccountID, &item.ConversationID, &item.JobID, &item.Title, &item.TitleOrigin,
			&item.Status, &item.CreatedAt, &item.ModelName, &item.ErrorCode, &item.TitlePrompt); err != nil {
			return nil, mapError(err)
		}
		items = append(items, item)
	}
	return items, mapError(rows.Err())
}

const presentationUUIDPattern = "[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}"

// Match the durable admission marker and web delivery contract before applying
// the account keyset limit. A guarded cast makes malformed legacy params inert.
// Only a bounded prefix of the retained, job-bound user message can supply the
// display topic for an older presentation. Never read its answer for a listing.
const presentationHistorySQL = `
WITH page AS MATERIALIZED (
 SELECT j.account_id,c.id AS conversation_id,j.id AS job_id,c.title,c.title_origin,j.status,j.created_at,
  left(COALESCE(j.params->>'model_name',''),160) AS model_name,COALESCE(j.error_code,'') AS error_code
 FROM jobs j
 JOIN conversations c ON c.id=CASE WHEN j.params->>'conversation_id' ~ '^` + presentationUUIDPattern + `$'
  THEN (j.params->>'conversation_id')::uuid END
  AND c.account_id=j.account_id AND c.source='web' AND c.status='active'
 WHERE j.account_id=$1 AND j.source='web' AND j.channel='web' AND j.result_mode='account_history'
 AND j.operation_type='text_generate' AND j.modality='text' AND j.params->>'conversation_source'='web'
 AND j.idempotency_key ~ $2 AND right(j.idempotency_key,36)<>'00000000-0000-0000-0000-000000000000'
 AND j.user_id IS NULL AND COALESCE(j.vk_peer_id,0)=0 AND j.command_id IS NULL
 AND j.recipient_ref IS NULL AND j.thread_ref IS NULL
 AND j.target_channel IS NULL AND j.target_recipient_ref IS NULL AND j.target_thread_ref IS NULL
 AND ($4::timestamptz IS NULL OR (j.created_at,j.id)<($4::timestamptz,$5::uuid))
 ORDER BY j.created_at DESC,j.id DESC LIMIT $3
)
SELECT p.account_id,p.conversation_id,p.job_id,p.title,p.title_origin,p.status,p.created_at,p.model_name,p.error_code,
 COALESCE(left(m.text,512),'')
FROM page p LEFT JOIN conversation_messages m ON m.job_id=p.job_id AND m.role='user' AND m.conversation_id=p.conversation_id
 AND m.deleted_at IS NULL AND m.redacted_at IS NULL
ORDER BY p.created_at DESC,p.job_id DESC`
