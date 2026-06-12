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
// removed from object storage. Ready artifacts are only eligible when they are
// outside their class retention and no job/delivery history references them.
func (r *MaintenanceRepository) MediaCleanupCandidates(ctx context.Context, policy domain.MediaCleanupPolicy, limit int) ([]domain.MediaCleanupCandidate, error) {
	if limit <= 0 {
		limit = 100
	}
	if !policy.Enabled() {
		return nil, nil
	}
	const q = `
		SELECT kind, cleanup_class, artifact_id, variant_id, variant_type, media_type, storage_bucket, storage_key, size_bytes
		FROM (
			SELECT
				$1::text AS kind,
				$2::text AS cleanup_class,
				a.id AS artifact_id,
				'00000000-0000-0000-0000-000000000000'::uuid AS variant_id,
				$3::text AS variant_type,
				a.media_type,
				a.storage_bucket,
				a.storage_key,
				a.size_bytes
			FROM artifacts a
			WHERE $4::boolean
			  AND a.lifecycle_class = $2
			  AND a.kind = $5
			  AND a.media_type = $6
			  AND a.status = $7
			  AND a.storage_bucket <> ''
			  AND a.storage_key <> ''
			  AND a.updated_at <= $8
			  AND NOT EXISTS (
			      SELECT 1 FROM jobs j
			      WHERE j.input_artifact_ids @> ARRAY[a.id]::uuid[]
			  )
			UNION ALL
			SELECT
				$1::text AS kind,
				$9::text AS cleanup_class,
				a.id AS artifact_id,
				'00000000-0000-0000-0000-000000000000'::uuid AS variant_id,
				$3::text AS variant_type,
				a.media_type,
				a.storage_bucket,
				a.storage_key,
				a.size_bytes
			FROM artifacts a
			WHERE $10::boolean
			  AND (a.lifecycle_class = $9 OR a.status IN ($11, $12))
			  AND a.media_type IN ($6, $13, $14, $15)
			  AND a.storage_bucket <> ''
			  AND a.storage_key <> ''
			  AND a.updated_at <= $16
			UNION ALL
			SELECT
				$1::text AS kind,
				$17::text AS cleanup_class,
				a.id AS artifact_id,
				'00000000-0000-0000-0000-000000000000'::uuid AS variant_id,
				$3::text AS variant_type,
				a.media_type,
				a.storage_bucket,
				a.storage_key,
				a.size_bytes
			FROM artifacts a
			WHERE $18::boolean
			  AND a.lifecycle_class = $17
			  AND a.status = $7
			  AND a.storage_bucket <> ''
			  AND a.storage_key <> ''
			  AND a.updated_at <= $19
			  AND a.job_id IS NULL
			  AND NOT EXISTS (
			      SELECT 1 FROM jobs j
			      WHERE j.output_artifact_ids @> ARRAY[a.id]::uuid[]
			         OR j.input_artifact_ids @> ARRAY[a.id]::uuid[]
			  )
			  AND NOT EXISTS (
			      SELECT 1 FROM deliveries d
			      WHERE d.artifact_id = a.id
			  )
			UNION ALL
			SELECT
				$20::text AS kind,
				$9::text AS cleanup_class,
				a.id AS artifact_id,
				v.id AS variant_id,
				v.variant_type,
				a.media_type,
				v.storage_bucket,
				v.storage_key,
				v.size_bytes
			FROM artifact_variants v
			JOIN artifacts a ON a.id = v.artifact_id
			WHERE $10::boolean
			  AND a.status IN ($11, $12)
			  AND a.media_type IN ($6, $13, $14, $15)
			  AND v.storage_bucket <> ''
			  AND v.storage_key <> ''
			  AND v.updated_at <= $16
			UNION ALL
			SELECT
				$20::text AS kind,
				$21::text AS cleanup_class,
				a.id AS artifact_id,
				v.id AS variant_id,
				v.variant_type,
				a.media_type,
				v.storage_bucket,
				v.storage_key,
				v.size_bytes
			FROM artifact_variants v
			JOIN artifacts a ON a.id = v.artifact_id
			WHERE $22::boolean
			  AND v.lifecycle_class = $21
			  AND a.media_type IN ($6, $13, $14, $15)
			  AND v.storage_bucket <> ''
			  AND v.storage_key <> ''
			  AND v.updated_at <= $23
			  AND a.status = $7
			  AND a.job_id IS NULL
			  AND NOT EXISTS (
			      SELECT 1 FROM jobs j
			      WHERE j.output_artifact_ids @> ARRAY[a.id]::uuid[]
			         OR j.input_artifact_ids @> ARRAY[a.id]::uuid[]
			  )
			  AND NOT EXISTS (
			      SELECT 1 FROM deliveries d
			      WHERE d.artifact_id = a.id
			  )
		) cleanup_candidates
		ORDER BY cleanup_class, kind, artifact_id, variant_id
		LIMIT $24`
	rows, err := r.db.Query(ctx, q,
		domain.MediaCleanupOriginal,
		domain.MediaCleanupInputReference,
		domain.VariantOriginal,
		!policy.InputReferenceCutoff.IsZero(),
		domain.ArtifactKindInput,
		domain.MediaTypeImage,
		domain.ArtifactStatusReady,
		policy.InputReferenceCutoff,
		domain.MediaCleanupFailedDeleted,
		!policy.FailedDeletedCutoff.IsZero(),
		domain.ArtifactStatusFailed,
		domain.ArtifactStatusDeleted,
		domain.MediaTypeVideo,
		domain.MediaTypeAudio,
		domain.MediaTypeDocument,
		policy.FailedDeletedCutoff,
		domain.MediaCleanupProviderOriginal,
		!policy.ProviderOriginalCutoff.IsZero(),
		policy.ProviderOriginalCutoff,
		domain.MediaCleanupVariant,
		domain.MediaCleanupDeliveryVariant,
		!policy.DeliveryVariantCutoff.IsZero(),
		policy.DeliveryVariantCutoff,
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
			&candidate.CleanupClass,
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
				lifecycle_class = $3,
				updated_at = now()
			WHERE id = $1
			  AND storage_bucket = $4
			  AND storage_key = $5
			  AND (
			      status IN ($6, $7)
			      OR (
			          lifecycle_class = $8
			          AND status = $9
			          AND NOT EXISTS (
			              SELECT 1 FROM jobs j
			              WHERE j.input_artifact_ids @> ARRAY[artifacts.id]::uuid[]
			          )
			      )
			      OR (
			          lifecycle_class = $10
			          AND status = $9
			          AND job_id IS NULL
			          AND NOT EXISTS (
			              SELECT 1 FROM jobs j
			              WHERE j.output_artifact_ids @> ARRAY[artifacts.id]::uuid[]
			                 OR j.input_artifact_ids @> ARRAY[artifacts.id]::uuid[]
			          )
			          AND NOT EXISTS (
			              SELECT 1 FROM deliveries d
			              WHERE d.artifact_id = artifacts.id
			          )
			      )
			  )`
		_, err := r.db.Exec(ctx, q,
			candidate.ArtifactID,
			domain.ArtifactStatusDeleted,
			domain.ArtifactLifecycleFailedDeleted,
			candidate.StorageBucket,
			candidate.StorageKey,
			domain.ArtifactStatusFailed,
			domain.ArtifactStatusDeleted,
			domain.ArtifactLifecycleInputReference,
			domain.ArtifactStatusReady,
			domain.ArtifactLifecycleProviderOriginal,
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
			  AND v.storage_bucket = $2
			  AND v.storage_key = $3
			  AND (
			      a.status IN ($4, $5)
			      OR (
			          a.status = $6
			          AND a.job_id IS NULL
			          AND NOT EXISTS (
			              SELECT 1 FROM jobs j
			              WHERE j.output_artifact_ids @> ARRAY[a.id]::uuid[]
			                 OR j.input_artifact_ids @> ARRAY[a.id]::uuid[]
			          )
			          AND NOT EXISTS (
			              SELECT 1 FROM deliveries d
			              WHERE d.artifact_id = a.id
			          )
			      )
			  )`
		_, err := r.db.Exec(ctx, q,
			candidate.VariantID,
			candidate.StorageBucket,
			candidate.StorageKey,
			domain.ArtifactStatusFailed,
			domain.ArtifactStatusDeleted,
			domain.ArtifactStatusReady,
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
