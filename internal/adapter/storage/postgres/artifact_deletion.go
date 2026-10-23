package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"vk-ai-aggregator/internal/domain"
)

type artifactTransactionBeginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// RequestOutputDeletionForAccount commits the access tombstone and durable
// cleanup deadlines together. No metadata/job/ledger rows are removed.
func (r *ArtifactRepository) RequestOutputDeletionForAccount(ctx context.Context, accountID, id uuid.UUID) ([]domain.MediaCleanupCandidate, error) {
	if accountID == uuid.Nil || id == uuid.Nil {
		return nil, domain.ErrNotFound
	}
	beginner, ok := r.db.(artifactTransactionBeginner)
	if !ok {
		return nil, errors.New("artifact deletion requires transaction support")
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var jobID uuid.UUID
	var status domain.ArtifactStatus
	err = tx.QueryRow(ctx, `SELECT a.job_id,a.status FROM artifacts a JOIN jobs j ON j.id=a.job_id
 WHERE a.id=$1 AND a.owner_account_id=$2 AND a.kind='output'
 AND j.account_id=$2 AND j.source='web' AND j.status='succeeded'
 AND j.output_artifact_ids @> ARRAY[a.id]::uuid[]
 AND a.status IN ('ready','deleted') FOR UPDATE OF a`, id, accountID).Scan(&jobID, &status)
	if err != nil {
		return nil, mapError(err)
	}
	if status != domain.ArtifactStatusDeleted {
		var busy bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM jobs WHERE
		   (input_artifact_ids @> ARRAY[$1::uuid] OR params->>'source_job_id'=$2
           OR params->>'persona_job_id'=$2 OR params->>'custom_model_job_id'=$2
           OR params @> jsonb_build_object('sources',jsonb_build_array(jsonb_build_object('job_id',$2::text))))
   AND status NOT IN ('succeeded','failed_terminal','cancelled','expired','refunded','rejected'))`, id, jobID.String()).Scan(&busy)
		if err != nil {
			return nil, err
		}
		if busy {
			return nil, domain.ErrConflict
		}
	}
	_, err = tx.Exec(ctx, `UPDATE artifacts SET status='deleted', lifecycle_class='failed_deleted',
 expires_at=LEAST(COALESCE(expires_at,now()),now()), public_url='',updated_at=now() WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE artifact_variants SET expires_at=LEAST(COALESCE(expires_at,now()),now()),updated_at=now() WHERE artifact_id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT 'original',a.id,'00000000-0000-0000-0000-000000000000'::uuid,'original',a.media_type,a.storage_bucket,a.storage_key,a.size_bytes
 FROM artifacts a WHERE a.id=$1 AND a.deleted_at IS NULL AND a.storage_key<>''
 UNION ALL SELECT 'variant',a.id,v.id,v.variant_type,a.media_type,v.storage_bucket,v.storage_key,v.size_bytes
 FROM artifact_variants v JOIN artifacts a ON a.id=v.artifact_id WHERE a.id=$1 AND v.deleted_at IS NULL AND v.storage_key<>''`, id)
	if err != nil {
		return nil, err
	}
	var candidates []domain.MediaCleanupCandidate
	for rows.Next() {
		var c domain.MediaCleanupCandidate
		c.CleanupClass = domain.MediaCleanupFailedDeleted
		if err = rows.Scan(&c.Kind, &c.ArtifactID, &c.VariantID, &c.VariantType, &c.MediaType, &c.StorageBucket, &c.StorageKey, &c.SizeBytes); err != nil {
			rows.Close()
			return nil, err
		}
		candidates = append(candidates, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return candidates, nil
}

// WithArtifactWrite serializes a complete variant write (including blob upload)
// against deletion. A late thumbnail can never recreate a purged object.
func (r *ArtifactRepository) WithArtifactWrite(ctx context.Context, id uuid.UUID, write func(domain.ArtifactRepository) error) error {
	beginner, ok := r.db.(artifactTransactionBeginner)
	if !ok {
		return errors.New("artifact write requires transaction support")
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var status domain.ArtifactStatus
	if err = tx.QueryRow(ctx, `SELECT status FROM artifacts WHERE id=$1 FOR UPDATE`, id).Scan(&status); err != nil {
		return mapError(err)
	}
	if status == domain.ArtifactStatusDeleted {
		return domain.ErrNotFound
	}
	if err = write(NewArtifactRepository(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
