package postgres

import (
	"context"
	"time"

	"vk-ai-aggregator/internal/domain"
)

// MaintenanceRepository contains operational cleanup and audit queries that do
// not belong to business repositories.
type MaintenanceRepository struct {
	db Querier
}

// NewMaintenanceRepository builds a MaintenanceRepository over db.
func NewMaintenanceRepository(db Querier) *MaintenanceRepository {
	return &MaintenanceRepository{db: db}
}

// CleanupExpiredIdempotencyKeys deletes expired idempotency records.
func (r *MaintenanceRepository) CleanupExpiredIdempotencyKeys(ctx context.Context, now time.Time) (int64, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM idempotency_keys WHERE expires_at <= $1`, now)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

// CleanupOutboxEvents deletes terminal outbox events older than cutoff.
func (r *MaintenanceRepository) CleanupOutboxEvents(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM outbox_events
		WHERE status IN ($1, $2) AND created_at <= $3`,
		domain.OutboxPublished, domain.OutboxFailed, cutoff,
	)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

// MediaCleanupCandidates returns old inactive media objects that can be safely
// removed from object storage. It deliberately excludes ready/stored artifacts
// so Mini App owner access and delivery retries keep working.
func (r *MaintenanceRepository) MediaCleanupCandidates(ctx context.Context, cutoff time.Time, limit int) ([]domain.MediaCleanupCandidate, error) {
	if limit <= 0 {
		limit = 100
	}
	const q = `
		SELECT kind, artifact_id, variant_id, variant_type, media_type, storage_bucket, storage_key, size_bytes
		FROM (
			SELECT
				$1::text AS kind,
				a.id AS artifact_id,
				'00000000-0000-0000-0000-000000000000'::uuid AS variant_id,
				$2::text AS variant_type,
				a.media_type,
				a.storage_bucket,
				a.storage_key,
				a.size_bytes
			FROM artifacts a
			WHERE a.status IN ($3, $4)
			  AND a.media_type IN ($5, $6, $7, $8)
			  AND a.storage_bucket <> ''
			  AND a.storage_key <> ''
			  AND a.updated_at <= $9
			UNION ALL
			SELECT
				$10::text AS kind,
				a.id AS artifact_id,
				v.id AS variant_id,
				v.variant_type,
				a.media_type,
				v.storage_bucket,
				v.storage_key,
				v.size_bytes
			FROM artifact_variants v
			JOIN artifacts a ON a.id = v.artifact_id
			WHERE a.status IN ($3, $4)
			  AND a.media_type IN ($5, $6, $7, $8)
			  AND v.variant_type IN ($2, $11, $12, $13, $14, $15)
			  AND v.storage_bucket <> ''
			  AND v.storage_key <> ''
			  AND a.updated_at <= $9
			  AND v.updated_at <= $9
		) cleanup_candidates
		ORDER BY kind, artifact_id, variant_id
		LIMIT $16`
	rows, err := r.db.Query(ctx, q,
		domain.MediaCleanupOriginal,
		domain.VariantOriginal,
		domain.ArtifactStatusFailed,
		domain.ArtifactStatusDeleted,
		domain.MediaTypeImage,
		domain.MediaTypeVideo,
		domain.MediaTypeAudio,
		domain.MediaTypeDocument,
		cutoff,
		domain.MediaCleanupVariant,
		domain.VariantPreview,
		domain.VariantThumbnail,
		domain.VariantVKPhoto,
		domain.VariantVKDoc,
		domain.VariantVKVideo,
		limit,
	)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var out []domain.MediaCleanupCandidate
	for rows.Next() {
		var candidate domain.MediaCleanupCandidate
		if err := rows.Scan(
			&candidate.Kind,
			&candidate.ArtifactID,
			&candidate.VariantID,
			&candidate.VariantType,
			&candidate.MediaType,
			&candidate.StorageBucket,
			&candidate.StorageKey,
			&candidate.SizeBytes,
		); err != nil {
			return nil, mapError(err)
		}
		out = append(out, candidate)
	}
	return out, mapError(rows.Err())
}

// MarkMediaCleanupDeleted clears storage coordinates after object deletion. It
// never targets active artifacts.
func (r *MaintenanceRepository) MarkMediaCleanupDeleted(ctx context.Context, candidate domain.MediaCleanupCandidate) error {
	switch candidate.Kind {
	case domain.MediaCleanupOriginal:
		const q = `
			UPDATE artifacts
			SET storage_bucket = '',
				storage_key = '',
				public_url = '',
				size_bytes = 0,
				status = $2,
				updated_at = now()
			WHERE id = $1
			  AND status IN ($3, $4)
			  AND storage_bucket = $5
			  AND storage_key = $6`
		_, err := r.db.Exec(ctx, q,
			candidate.ArtifactID,
			domain.ArtifactStatusDeleted,
			domain.ArtifactStatusFailed,
			domain.ArtifactStatusDeleted,
			candidate.StorageBucket,
			candidate.StorageKey,
		)
		return mapError(err)
	case domain.MediaCleanupVariant:
		const q = `
			UPDATE artifact_variants v
			SET storage_bucket = '',
				storage_key = '',
				size_bytes = 0,
				updated_at = now()
			FROM artifacts a
			WHERE v.id = $1
			  AND v.artifact_id = a.id
			  AND a.status IN ($2, $3)
			  AND v.storage_bucket = $4
			  AND v.storage_key = $5`
		_, err := r.db.Exec(ctx, q,
			candidate.VariantID,
			domain.ArtifactStatusFailed,
			domain.ArtifactStatusDeleted,
			candidate.StorageBucket,
			candidate.StorageKey,
		)
		return mapError(err)
	default:
		return nil
	}
}

// ProductActiveUserCounts returns distinct users with at least one job since
// the cutoff, grouped by bounded product dimensions.
func (r *MaintenanceRepository) ProductActiveUserCounts(ctx context.Context, since time.Time) ([]domain.ProductActiveUserCount, error) {
	const q = `
		SELECT
			CASE WHEN command_id IS NULL THEN 'miniapp_or_api' ELSE 'vk_bot' END AS surface,
			operation_type,
			modality,
			COUNT(DISTINCT user_id)::bigint
		FROM jobs
		WHERE created_at >= $1
		GROUP BY 1, 2, 3
		ORDER BY 1, 2, 3`
	rows, err := r.db.Query(ctx, q, since)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var out []domain.ProductActiveUserCount
	for rows.Next() {
		var item domain.ProductActiveUserCount
		if err := rows.Scan(&item.Surface, &item.Operation, &item.Modality, &item.Count); err != nil {
			return nil, mapError(err)
		}
		out = append(out, item)
	}
	return out, mapError(rows.Err())
}

// BalanceMismatches returns accounts whose cached balance differs from the sum
// of committed ledger entries.
func (r *MaintenanceRepository) BalanceMismatches(ctx context.Context, limit int) ([]domain.BalanceMismatch, error) {
	if limit <= 0 {
		limit = 100
	}
	const q = `
		SELECT c.id, c.user_id, c.currency, c.balance_cached,
		       COALESCE(SUM(l.amount) FILTER (WHERE l.status = $1), 0)::bigint AS ledger_balance
		FROM credit_accounts c
		LEFT JOIN ledger_entries l ON l.account_id = c.id
		GROUP BY c.id, c.user_id, c.currency, c.balance_cached
		HAVING c.balance_cached <> COALESCE(SUM(l.amount) FILTER (WHERE l.status = $1), 0)
		ORDER BY c.updated_at ASC
		LIMIT $2`
	rows, err := r.db.Query(ctx, q, domain.LedgerStatusCommitted, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var out []domain.BalanceMismatch
	for rows.Next() {
		var m domain.BalanceMismatch
		if err := rows.Scan(&m.AccountID, &m.UserID, &m.Currency, &m.BalanceCached, &m.LedgerBalance); err != nil {
			return nil, mapError(err)
		}
		m.Difference = m.BalanceCached - m.LedgerBalance
		out = append(out, m)
	}
	return out, mapError(rows.Err())
}
