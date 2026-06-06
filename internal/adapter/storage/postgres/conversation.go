package postgres

import (
	"context"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// ConversationRepository is the PostgreSQL implementation of
// domain.ConversationRepository.
type ConversationRepository struct {
	db Querier
}

// NewConversationRepository builds a ConversationRepository over the querier.
func NewConversationRepository(db Querier) *ConversationRepository {
	return &ConversationRepository{db: db}
}

var _ domain.ConversationRepository = (*ConversationRepository)(nil)

const conversationColumns = `id, user_id, vk_peer_id, status, title, created_at, updated_at`
const conversationMessageColumns = `id, conversation_id, job_id, seq, role, text, token_count, created_at`
const conversationSummaryColumns = `id, conversation_id, text, token_count, summarized_until_seq, created_at, updated_at`

func (r *ConversationRepository) GetActiveByUserPeer(ctx context.Context, userID uuid.UUID, vkPeerID int64) (*domain.Conversation, error) {
	const q = `SELECT ` + conversationColumns + `
		FROM conversations
		WHERE user_id = $1 AND vk_peer_id = $2 AND status = 'active'`
	var c domain.Conversation
	if err := mapError(scanConversation(r.db.QueryRow(ctx, q, userID, vkPeerID), &c)); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ConversationRepository) CreateConversation(ctx context.Context, c *domain.Conversation) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if c.Status == "" {
		c.Status = domain.ConversationActive
	}
	const q = `
		INSERT INTO conversations (id, user_id, vk_peer_id, status, title)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING ` + conversationColumns
	return mapError(scanConversation(r.db.QueryRow(ctx, q, c.ID, c.UserID, c.VKPeerID, c.Status, c.Title), c))
}

func (r *ConversationRepository) UpsertMessage(ctx context.Context, m *domain.ConversationMessage) (*domain.ConversationMessage, error) {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	const q = `
		INSERT INTO conversation_messages (id, conversation_id, job_id, role, text, token_count)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (job_id, role) DO UPDATE
		SET text = conversation_messages.text
		RETURNING ` + conversationMessageColumns
	var out domain.ConversationMessage
	if err := mapError(scanConversationMessage(r.db.QueryRow(ctx, q,
		m.ID, m.ConversationID, m.JobID, m.Role, m.Text, m.TokenCount), &out)); err != nil {
		return nil, err
	}
	_, _ = r.db.Exec(ctx, `UPDATE conversations SET updated_at = now() WHERE id = $1`, out.ConversationID)
	return &out, nil
}

func (r *ConversationRepository) ListRecentMessagesBefore(ctx context.Context, conversationID uuid.UUID, beforeSeq, minSeq int64, limit int) ([]*domain.ConversationMessage, error) {
	if limit <= 0 {
		return nil, nil
	}
	const q = `
		SELECT ` + conversationMessageColumns + `
		FROM (
			SELECT ` + conversationMessageColumns + `
			FROM conversation_messages
			WHERE conversation_id = $1 AND seq < $2 AND seq > $3
			ORDER BY seq DESC
			LIMIT $4
		) recent
		ORDER BY seq ASC`
	rows, err := r.db.Query(ctx, q, conversationID, beforeSeq, minSeq, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	return scanConversationMessages(rows)
}

func (r *ConversationRepository) ListMessagesAfter(ctx context.Context, conversationID uuid.UUID, afterSeq int64, limit int) ([]*domain.ConversationMessage, error) {
	if limit <= 0 {
		return nil, nil
	}
	const q = `SELECT ` + conversationMessageColumns + `
		FROM conversation_messages
		WHERE conversation_id = $1 AND seq > $2
		ORDER BY seq ASC
		LIMIT $3`
	rows, err := r.db.Query(ctx, q, conversationID, afterSeq, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	return scanConversationMessages(rows)
}

func (r *ConversationRepository) LatestSummary(ctx context.Context, conversationID uuid.UUID) (*domain.ConversationSummary, error) {
	const q = `SELECT ` + conversationSummaryColumns + `
		FROM conversation_summaries
		WHERE conversation_id = $1`
	var s domain.ConversationSummary
	if err := mapError(scanConversationSummary(r.db.QueryRow(ctx, q, conversationID), &s)); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *ConversationRepository) UpsertSummary(ctx context.Context, s *domain.ConversationSummary) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	const q = `
		INSERT INTO conversation_summaries (id, conversation_id, text, token_count, summarized_until_seq)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (conversation_id) DO UPDATE
		SET text = EXCLUDED.text,
		    token_count = EXCLUDED.token_count,
		    summarized_until_seq = EXCLUDED.summarized_until_seq,
		    updated_at = now()
		RETURNING ` + conversationSummaryColumns
	return mapError(scanConversationSummary(r.db.QueryRow(ctx, q,
		s.ID, s.ConversationID, s.Text, s.TokenCount, s.SummarizedUntilSeq), s))
}

func scanConversation(row rowScanner, c *domain.Conversation) error {
	return row.Scan(&c.ID, &c.UserID, &c.VKPeerID, &c.Status, &c.Title, &c.CreatedAt, &c.UpdatedAt)
}

func scanConversationMessage(row rowScanner, m *domain.ConversationMessage) error {
	return row.Scan(&m.ID, &m.ConversationID, &m.JobID, &m.Seq, &m.Role, &m.Text, &m.TokenCount, &m.CreatedAt)
}

func scanConversationMessages(rows rowScannerRows) ([]*domain.ConversationMessage, error) {
	var messages []*domain.ConversationMessage
	for rows.Next() {
		var m domain.ConversationMessage
		if err := scanConversationMessage(rows, &m); err != nil {
			return nil, mapError(err)
		}
		messages = append(messages, &m)
	}
	return messages, mapError(rows.Err())
}

func scanConversationSummary(row rowScanner, s *domain.ConversationSummary) error {
	return row.Scan(&s.ID, &s.ConversationID, &s.Text, &s.TokenCount, &s.SummarizedUntilSeq, &s.CreatedAt, &s.UpdatedAt)
}

type rowScannerRows interface {
	rowScanner
	Next() bool
	Err() error
}
