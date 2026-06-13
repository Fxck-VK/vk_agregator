package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// OperatorAuditRepository stores sanitized protected-operator action records.
type OperatorAuditRepository struct {
	db Querier
}

// NewOperatorAuditRepository builds an OperatorAuditRepository.
func NewOperatorAuditRepository(db Querier) *OperatorAuditRepository {
	return &OperatorAuditRepository{db: db}
}

var _ domain.OperatorAuditRepository = (*OperatorAuditRepository)(nil)

const operatorAuditColumns = `id, actor_ref, action, target_type, target_ref, result, request_ref, created_at`

func (r *OperatorAuditRepository) Create(ctx context.Context, entry *domain.OperatorAuditEntry) error {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	const q = `
		INSERT INTO operator_audit_entries (
			id, actor_ref, action, target_type, target_ref, result, request_ref
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING ` + operatorAuditColumns
	return mapError(scanOperatorAuditEntry(r.db.QueryRow(ctx, q,
		entry.ID,
		entry.ActorRef,
		entry.Action,
		entry.TargetType,
		entry.TargetRef,
		entry.Result,
		entry.RequestRef,
	), entry))
}

func (r *OperatorAuditRepository) List(ctx context.Context, filter domain.OperatorAuditFilter, limit, offset int) ([]*domain.OperatorAuditEntry, error) {
	q := `SELECT ` + operatorAuditColumns + ` FROM operator_audit_entries`
	var (
		conds []string
		args  []any
	)
	if filter.Action != "" {
		args = append(args, filter.Action)
		conds = append(conds, fmt.Sprintf("action = $%d", len(args)))
	}
	if filter.TargetType != "" {
		args = append(args, filter.TargetType)
		conds = append(conds, fmt.Sprintf("target_type = $%d", len(args)))
	}
	if filter.Result != "" {
		args = append(args, filter.Result)
		conds = append(conds, fmt.Sprintf("result = $%d", len(args)))
	}
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, limit)
	q += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", len(args))
	args = append(args, offset)
	q += fmt.Sprintf(" OFFSET $%d", len(args))

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var entries []*domain.OperatorAuditEntry
	for rows.Next() {
		var entry domain.OperatorAuditEntry
		if err := scanOperatorAuditEntry(rows, &entry); err != nil {
			return nil, mapError(err)
		}
		entries = append(entries, &entry)
	}
	return entries, mapError(rows.Err())
}

func scanOperatorAuditEntry(row rowScanner, entry *domain.OperatorAuditEntry) error {
	return row.Scan(
		&entry.ID,
		&entry.ActorRef,
		&entry.Action,
		&entry.TargetType,
		&entry.TargetRef,
		&entry.Result,
		&entry.RequestRef,
		&entry.CreatedAt,
	)
}
