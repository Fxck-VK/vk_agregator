CREATE INDEX IF NOT EXISTS provider_tasks_health_provider_created_idx
	ON provider_tasks (provider, created_at DESC, updated_at DESC);

CREATE INDEX IF NOT EXISTS provider_tasks_health_error_idx
	ON provider_tasks (provider, updated_at DESC)
	WHERE error_class <> '';

CREATE INDEX IF NOT EXISTS deliveries_health_created_idx
	ON deliveries (created_at DESC, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS deliveries_health_error_idx
	ON deliveries (updated_at DESC)
	WHERE error_code <> '';
