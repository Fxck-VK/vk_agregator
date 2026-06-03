package postgres

import (
	"context"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// CommandRepository is the PostgreSQL implementation of domain.CommandRepository.
type CommandRepository struct {
	db Querier
}

// NewCommandRepository builds a CommandRepository over the given querier.
func NewCommandRepository(db Querier) *CommandRepository {
	return &CommandRepository{db: db}
}

var _ domain.CommandRepository = (*CommandRepository)(nil)

const commandColumns = `id, user_id, vk_peer_id, inbound_event_id, type, raw_text,
	args, attachment_artifact_ids, idempotency_key, correlation_id, created_at, updated_at`

// Create inserts a new command.
func (r *CommandRepository) Create(ctx context.Context, cmd *domain.Command) error {
	if cmd.ID == uuid.Nil {
		cmd.ID = uuid.New()
	}
	if len(cmd.Args) == 0 {
		cmd.Args = []byte("{}")
	}
	const q = `
		INSERT INTO commands (
			id, user_id, vk_peer_id, inbound_event_id, type, raw_text,
			args, attachment_artifact_ids, idempotency_key, correlation_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING ` + commandColumns
	row := r.db.QueryRow(ctx, q,
		cmd.ID, cmd.UserID, cmd.VKPeerID, cmd.InboundEventID, cmd.Type, cmd.RawText,
		[]byte(cmd.Args), cmd.AttachmentArtifactIDs, cmd.IdempotencyKey, cmd.CorrelationID,
	)
	return mapError(scanCommand(row, cmd))
}

// GetByID fetches a command by id.
func (r *CommandRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Command, error) {
	const q = `SELECT ` + commandColumns + ` FROM commands WHERE id = $1`
	var cmd domain.Command
	if err := mapError(scanCommand(r.db.QueryRow(ctx, q, id), &cmd)); err != nil {
		return nil, err
	}
	return &cmd, nil
}

// GetByIdempotencyKey fetches a command by idempotency key.
func (r *CommandRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Command, error) {
	const q = `SELECT ` + commandColumns + ` FROM commands WHERE idempotency_key = $1`
	var cmd domain.Command
	if err := mapError(scanCommand(r.db.QueryRow(ctx, q, key), &cmd)); err != nil {
		return nil, err
	}
	return &cmd, nil
}

// ListByUser returns the most recent commands for a user, newest first.
func (r *CommandRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Command, error) {
	const q = `SELECT ` + commandColumns + `
		FROM commands WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var cmds []*domain.Command
	for rows.Next() {
		var cmd domain.Command
		if err := scanCommand(rows, &cmd); err != nil {
			return nil, mapError(err)
		}
		cmds = append(cmds, &cmd)
	}
	return cmds, mapError(rows.Err())
}

func scanCommand(row rowScanner, cmd *domain.Command) error {
	return row.Scan(
		&cmd.ID, &cmd.UserID, &cmd.VKPeerID, &cmd.InboundEventID, &cmd.Type, &cmd.RawText,
		&cmd.Args, &cmd.AttachmentArtifactIDs, &cmd.IdempotencyKey, &cmd.CorrelationID,
		&cmd.CreatedAt, &cmd.UpdatedAt,
	)
}
