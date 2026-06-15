ALTER TABLE jobs
    ADD COLUMN source TEXT NOT NULL DEFAULT 'unknown';

UPDATE jobs
SET source = CASE
    WHEN command_id IS NULL THEN 'miniapp'
    ELSE 'vk_bot'
END
WHERE source = 'unknown';

CREATE INDEX jobs_source_created_idx ON jobs (source, created_at DESC);
