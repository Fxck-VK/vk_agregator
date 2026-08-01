-- migrate: no-transaction
DROP INDEX CONCURRENTLY IF EXISTS jobs_web_image_history_cursor_idx;
