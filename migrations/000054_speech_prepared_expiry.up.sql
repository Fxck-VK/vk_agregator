-- migrate: no-transaction
CREATE INDEX CONCURRENTLY IF NOT EXISTS jobs_web_speech_prepared_expiry_idx
ON jobs (expires_at ASC, id ASC)
WHERE account_id IS NOT NULL AND source = 'web'
  AND ((operation_type = 'image_generate' AND modality = 'image')
    OR (operation_type IN ('audio_music', 'audio_tts') AND modality = 'audio')
    OR (operation_type = 'audio_stt' AND modality = 'text'))
  AND status = 'prepared' AND expires_at IS NOT NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS jobs_account_speech_prepared_expiry_idx
ON jobs (account_id, expires_at ASC, id ASC)
WHERE source = 'web'
  AND ((operation_type = 'image_generate' AND modality = 'image')
    OR (operation_type IN ('audio_music', 'audio_tts') AND modality = 'audio')
    OR (operation_type = 'audio_stt' AND modality = 'text'))
  AND status = 'prepared' AND expires_at IS NOT NULL;
