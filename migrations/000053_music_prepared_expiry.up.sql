-- migrate: no-transaction
-- Add a covering index for bounded image/music confirmation reconciliation.
-- The preceding image-only index remains available to old application builds.
CREATE INDEX CONCURRENTLY IF NOT EXISTS jobs_web_media_prepared_expiry_idx
ON jobs (expires_at ASC, id ASC)
WHERE account_id IS NOT NULL AND source = 'web'
  AND ((operation_type = 'image_generate' AND modality = 'image') OR (operation_type = 'audio_music' AND modality = 'audio'))
  AND status = 'prepared' AND expires_at IS NOT NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS jobs_account_media_prepared_expiry_idx
ON jobs (account_id, expires_at ASC, id ASC)
WHERE source = 'web'
  AND ((operation_type = 'image_generate' AND modality = 'image') OR (operation_type = 'audio_music' AND modality = 'audio'))
  AND status = 'prepared' AND expires_at IS NOT NULL;
