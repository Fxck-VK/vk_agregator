package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"vk-ai-aggregator/internal/domain"
)

const refreshDailyUserActivitySQL = `
	INSERT INTO daily_user_activity (
		activity_date,
		surface,
		active_users,
		new_users,
		returning_users,
		message_users,
		generation_users,
		updated_at
	)
	WITH generation_activity AS (
		SELECT
			date_trunc('day', j.created_at)::date AS activity_date,
			COALESCE(NULLIF(j.source, ''), 'unknown') AS surface,
			j.user_id,
			false AS is_message,
			true AS is_generation
		FROM jobs j
		WHERE j.created_at >= $1
		  AND j.created_at < $2
		  AND j.deleted_at IS NULL
	),
	message_activity AS (
		SELECT
			date_trunc('day', m.created_at)::date AS activity_date,
			COALESCE(NULLIF(c.source, ''), 'unknown') AS surface,
			c.user_id,
			true AS is_message,
			false AS is_generation
		FROM conversation_messages m
		JOIN conversations c ON c.id = m.conversation_id
		WHERE m.created_at >= $1
		  AND m.created_at < $2
		  AND m.deleted_at IS NULL
		  AND c.deleted_at IS NULL
	),
	activity AS (
		SELECT * FROM generation_activity
		UNION ALL
		SELECT * FROM message_activity
	),
	aggregated AS (
		SELECT
			a.activity_date,
			a.surface,
			COUNT(DISTINCT a.user_id)::bigint AS active_users,
			(COUNT(DISTINCT a.user_id) FILTER (
				WHERE date_trunc('day', u.first_seen_at)::date = a.activity_date
			))::bigint AS new_users,
			(COUNT(DISTINCT a.user_id) FILTER (
				WHERE date_trunc('day', u.first_seen_at)::date < a.activity_date
			))::bigint AS returning_users,
			(COUNT(DISTINCT a.user_id) FILTER (WHERE a.is_message))::bigint AS message_users,
			(COUNT(DISTINCT a.user_id) FILTER (WHERE a.is_generation))::bigint AS generation_users
		FROM activity a
		JOIN users u ON u.id = a.user_id
		GROUP BY a.activity_date, a.surface
	)
	SELECT
		activity_date,
		surface,
		active_users,
		new_users,
		returning_users,
		message_users,
		generation_users,
		now()
	FROM aggregated
	ON CONFLICT (activity_date, surface)
	DO UPDATE SET
		active_users = EXCLUDED.active_users,
		new_users = EXCLUDED.new_users,
		returning_users = EXCLUDED.returning_users,
		message_users = EXCLUDED.message_users,
		generation_users = EXCLUDED.generation_users,
		updated_at = now()`

const refreshDailyGenerationStatsSQL = `
	INSERT INTO daily_generation_stats (
		activity_date,
		surface,
		operation_type,
		modality,
		jobs_created,
		jobs_succeeded,
		jobs_failed,
		credits_reserved,
		credits_captured,
		artifacts_created,
		updated_at
	)
	SELECT
		date_trunc('day', j.created_at)::date AS activity_date,
		COALESCE(NULLIF(j.source, ''), 'unknown') AS surface,
		COALESCE(NULLIF(j.operation_type::text, ''), 'unknown') AS operation_type,
		COALESCE(NULLIF(j.modality::text, ''), 'unknown') AS modality,
		COUNT(*)::bigint AS jobs_created,
		(COUNT(*) FILTER (WHERE j.status = 'succeeded'))::bigint AS jobs_succeeded,
		(COUNT(*) FILTER (WHERE j.status IN ('failed_terminal', 'provider_failed', 'cancelled', 'expired')))::bigint AS jobs_failed,
		COALESCE(SUM(GREATEST(j.cost_reserved, 0)), 0)::bigint AS credits_reserved,
		COALESCE(SUM(GREATEST(j.cost_captured, 0)), 0)::bigint AS credits_captured,
		COALESCE(SUM(COALESCE(cardinality(j.output_artifact_ids), 0)), 0)::bigint AS artifacts_created,
		now()
	FROM jobs j
	WHERE j.created_at >= $1
	  AND j.created_at < $2
	  AND j.deleted_at IS NULL
	GROUP BY 1, 2, 3, 4
	ON CONFLICT (activity_date, surface, operation_type, modality)
	DO UPDATE SET
		jobs_created = EXCLUDED.jobs_created,
		jobs_succeeded = EXCLUDED.jobs_succeeded,
		jobs_failed = EXCLUDED.jobs_failed,
		credits_reserved = EXCLUDED.credits_reserved,
		credits_captured = EXCLUDED.credits_captured,
		artifacts_created = EXCLUDED.artifacts_created,
		updated_at = now()`

const refreshDailyProviderStatsSQL = `
	INSERT INTO daily_provider_stats (
		activity_date,
		provider,
		model_code,
		operation_type,
		modality,
		tasks_created,
		tasks_succeeded,
		tasks_failed,
		avg_latency_ms,
		total_cost_units,
		updated_at
	)
	SELECT
		date_trunc('day', COALESCE(p.completed_at, p.updated_at, p.created_at))::date AS activity_date,
		COALESCE(NULLIF(p.provider::text, ''), 'unknown') AS provider,
		COALESCE(NULLIF(p.model_code, ''), 'unknown') AS model_code,
		COALESCE(NULLIF(j.operation_type::text, ''), 'unknown') AS operation_type,
		COALESCE(NULLIF(j.modality::text, ''), 'unknown') AS modality,
		COUNT(*)::bigint AS tasks_created,
		(COUNT(*) FILTER (WHERE p.status = 'succeeded'))::bigint AS tasks_succeeded,
		(COUNT(*) FILTER (WHERE p.status IN ('failed', 'cancelled')))::bigint AS tasks_failed,
		COALESCE(AVG(
			CASE
				WHEN p.completed_at IS NULL THEN NULL
				ELSE GREATEST(
					EXTRACT(EPOCH FROM (p.completed_at - COALESCE(p.submitted_at, p.created_at))) * 1000,
					0
				)
			END
		), 0)::bigint AS avg_latency_ms,
		COALESCE(SUM(CASE WHEN p.status = 'succeeded' THEN GREATEST(j.cost_captured, 0) ELSE 0 END), 0)::bigint AS total_cost_units,
		now()
	FROM provider_tasks p
	JOIN jobs j ON j.id = p.job_id
	WHERE COALESCE(p.completed_at, p.updated_at, p.created_at) >= $1
	  AND COALESCE(p.completed_at, p.updated_at, p.created_at) < $2
	  AND p.deleted_at IS NULL
	  AND j.deleted_at IS NULL
	GROUP BY 1, 2, 3, 4, 5
	ON CONFLICT (activity_date, provider, model_code, operation_type, modality)
	DO UPDATE SET
		tasks_created = EXCLUDED.tasks_created,
		tasks_succeeded = EXCLUDED.tasks_succeeded,
		tasks_failed = EXCLUDED.tasks_failed,
		avg_latency_ms = EXCLUDED.avg_latency_ms,
		total_cost_units = EXCLUDED.total_cost_units,
		updated_at = now()`

const refreshDailyRevenueStatsSQL = `
	INSERT INTO daily_revenue_stats (
		activity_date,
		provider,
		currency,
		payment_intents_created,
		payments_succeeded,
		payments_canceled,
		refunds_succeeded,
		gross_amount_minor,
		refunded_amount_minor,
		net_amount_minor,
		credits_sold,
		updated_at
	)
	WITH payment_days AS (
		SELECT
			date_trunc('day', i.created_at)::date AS activity_date,
			COALESCE(NULLIF(i.provider::text, ''), 'unknown') AS provider,
			COALESCE(NULLIF(i.currency::text, ''), 'rub') AS currency,
			COUNT(*)::bigint AS payment_intents_created,
			(COUNT(*) FILTER (WHERE i.status = 'succeeded'))::bigint AS payments_succeeded,
			(COUNT(*) FILTER (WHERE i.status IN ('canceled', 'failed', 'expired')))::bigint AS payments_canceled,
			COALESCE((SUM(i.amount) FILTER (WHERE i.status = 'succeeded')), 0)::bigint AS gross_amount_minor,
			COALESCE((SUM(i.credits) FILTER (WHERE i.status = 'succeeded')), 0)::bigint AS credits_sold
		FROM payment_intents i
		WHERE i.created_at >= $1
		  AND i.created_at < $2
		GROUP BY 1, 2, 3
	),
	refund_days AS (
		SELECT
			date_trunc('day', r.updated_at)::date AS activity_date,
			COALESCE(NULLIF(i.provider::text, ''), 'unknown') AS provider,
			COALESCE(NULLIF(i.currency::text, ''), 'rub') AS currency,
			(COUNT(*) FILTER (WHERE r.status = 'succeeded'))::bigint AS refunds_succeeded,
			COALESCE((SUM(r.amount) FILTER (WHERE r.status = 'succeeded')), 0)::bigint AS refunded_amount_minor
		FROM payment_refunds r
		JOIN payment_intents i ON i.id = r.intent_id
		WHERE r.updated_at >= $1
		  AND r.updated_at < $2
		GROUP BY 1, 2, 3
	),
	combined AS (
		SELECT activity_date, provider, currency FROM payment_days
		UNION
		SELECT activity_date, provider, currency FROM refund_days
	)
	SELECT
		c.activity_date,
		c.provider,
		c.currency,
		COALESCE(p.payment_intents_created, 0),
		COALESCE(p.payments_succeeded, 0),
		COALESCE(p.payments_canceled, 0),
		COALESCE(r.refunds_succeeded, 0),
		COALESCE(p.gross_amount_minor, 0),
		COALESCE(r.refunded_amount_minor, 0),
		COALESCE(p.gross_amount_minor, 0) - COALESCE(r.refunded_amount_minor, 0),
		COALESCE(p.credits_sold, 0),
		now()
	FROM combined c
	LEFT JOIN payment_days p USING (activity_date, provider, currency)
	LEFT JOIN refund_days r USING (activity_date, provider, currency)
	ON CONFLICT (activity_date, provider, currency)
	DO UPDATE SET
		payment_intents_created = EXCLUDED.payment_intents_created,
		payments_succeeded = EXCLUDED.payments_succeeded,
		payments_canceled = EXCLUDED.payments_canceled,
		refunds_succeeded = EXCLUDED.refunds_succeeded,
		gross_amount_minor = EXCLUDED.gross_amount_minor,
		refunded_amount_minor = EXCLUDED.refunded_amount_minor,
		net_amount_minor = EXCLUDED.net_amount_minor,
		credits_sold = EXCLUDED.credits_sold,
		updated_at = now()`

const refreshDailyReferralStatsSQL = `
	INSERT INTO daily_referral_stats (
		activity_date,
		source,
		link_opened_count,
		registered_count,
		activated_count,
		rewarded_count,
		first_generation_count,
		first_payment_count,
		updated_at
	)
	SELECT
		date_trunc('day', e.created_at)::date AS activity_date,
		COALESCE(NULLIF(e.source, ''), 'unknown') AS source,
		(COUNT(*) FILTER (WHERE e.event_type = 'link_opened'))::bigint AS link_opened_count,
		(COUNT(*) FILTER (WHERE e.event_type = 'registered'))::bigint AS registered_count,
		(COUNT(*) FILTER (WHERE e.event_type = 'activated'))::bigint AS activated_count,
		(COUNT(*) FILTER (WHERE e.event_type = 'rewarded'))::bigint AS rewarded_count,
		(COUNT(*) FILTER (WHERE e.event_type = 'first_generation'))::bigint AS first_generation_count,
		(COUNT(*) FILTER (WHERE e.event_type = 'first_payment'))::bigint AS first_payment_count,
		now()
	FROM referral_events e
	WHERE e.created_at >= $1
	  AND e.created_at < $2
	  AND e.deleted_at IS NULL
	GROUP BY 1, 2
	ON CONFLICT (activity_date, source)
	DO UPDATE SET
		link_opened_count = EXCLUDED.link_opened_count,
		registered_count = EXCLUDED.registered_count,
		activated_count = EXCLUDED.activated_count,
		rewarded_count = EXCLUDED.rewarded_count,
		first_generation_count = EXCLUDED.first_generation_count,
		first_payment_count = EXCLUDED.first_payment_count,
		updated_at = now()`

const refreshDailyRetentionStatsSQL = `
	INSERT INTO daily_retention_stats (
		activity_date,
		cohort_date,
		surface,
		day_number,
		cohort_users,
		retained_users,
		updated_at
	)
	WITH generation_activity AS (
		SELECT
			date_trunc('day', j.created_at)::date AS activity_date,
			COALESCE(NULLIF(j.source, ''), 'unknown') AS surface,
			j.user_id
		FROM jobs j
		WHERE j.created_at >= $1
		  AND j.created_at < $2
		  AND j.deleted_at IS NULL
	),
	message_activity AS (
		SELECT
			date_trunc('day', m.created_at)::date AS activity_date,
			COALESCE(NULLIF(c.source, ''), 'unknown') AS surface,
			c.user_id
		FROM conversation_messages m
		JOIN conversations c ON c.id = m.conversation_id
		WHERE m.created_at >= $1
		  AND m.created_at < $2
		  AND m.deleted_at IS NULL
		  AND c.deleted_at IS NULL
	),
	activity AS (
		SELECT * FROM generation_activity
		UNION
		SELECT * FROM message_activity
	),
	cohorts AS (
		SELECT
			date_trunc('day', first_seen_at)::date AS cohort_date,
			COUNT(*)::bigint AS cohort_users
		FROM users
		GROUP BY 1
	),
	retained AS (
		SELECT
			a.activity_date,
			date_trunc('day', u.first_seen_at)::date AS cohort_date,
			a.surface,
			COUNT(DISTINCT a.user_id)::bigint AS retained_users
		FROM activity a
		JOIN users u ON u.id = a.user_id
		WHERE date_trunc('day', u.first_seen_at)::date <= a.activity_date
		GROUP BY 1, 2, 3
	)
	SELECT
		r.activity_date,
		r.cohort_date,
		r.surface,
		(r.activity_date - r.cohort_date)::integer AS day_number,
		c.cohort_users,
		LEAST(r.retained_users, c.cohort_users),
		now()
	FROM retained r
	JOIN cohorts c ON c.cohort_date = r.cohort_date
	ON CONFLICT (activity_date, cohort_date, surface)
	DO UPDATE SET
		day_number = EXCLUDED.day_number,
		cohort_users = EXCLUDED.cohort_users,
		retained_users = EXCLUDED.retained_users,
		updated_at = now()`

const refreshDailyFunnelStatsSQL = `
	INSERT INTO daily_funnel_stats (
		activity_date,
		surface,
		funnel_step,
		users_count,
		events_count,
		updated_at
	)
	WITH events AS (
		SELECT
			date_trunc('day', u.first_seen_at)::date AS activity_date,
			'all'::text AS surface,
			'registered_user'::text AS funnel_step,
			u.id AS user_id,
			1::bigint AS event_count
		FROM users u
		WHERE u.first_seen_at >= $1
		  AND u.first_seen_at < $2
		UNION ALL
		SELECT
			date_trunc('day', m.created_at)::date,
			COALESCE(NULLIF(c.source, ''), 'unknown'),
			'message_sent',
			c.user_id,
			1::bigint
		FROM conversation_messages m
		JOIN conversations c ON c.id = m.conversation_id
		WHERE m.created_at >= $1
		  AND m.created_at < $2
		  AND m.deleted_at IS NULL
		  AND c.deleted_at IS NULL
		UNION ALL
		SELECT
			date_trunc('day', j.created_at)::date,
			COALESCE(NULLIF(j.source, ''), 'unknown'),
			'generation_requested',
			j.user_id,
			1::bigint
		FROM jobs j
		WHERE j.created_at >= $1
		  AND j.created_at < $2
		  AND j.deleted_at IS NULL
		UNION ALL
		SELECT
			date_trunc('day', j.updated_at)::date,
			COALESCE(NULLIF(j.source, ''), 'unknown'),
			'generation_succeeded',
			j.user_id,
			1::bigint
		FROM jobs j
		WHERE j.updated_at >= $1
		  AND j.updated_at < $2
		  AND j.deleted_at IS NULL
		  AND j.status = 'succeeded'
		UNION ALL
		SELECT
			date_trunc('day', i.updated_at)::date,
			'all',
			'payment_succeeded',
			i.user_id,
			1::bigint
		FROM payment_intents i
		WHERE i.updated_at >= $1
		  AND i.updated_at < $2
		  AND i.status = 'succeeded'
		UNION ALL
		SELECT
			date_trunc('day', e.created_at)::date,
			COALESCE(NULLIF(e.source, ''), 'unknown'),
			'referral_activated',
			COALESCE(e.referred_user_id, e.referrer_user_id),
			1::bigint
		FROM referral_events e
		WHERE e.created_at >= $1
		  AND e.created_at < $2
		  AND e.deleted_at IS NULL
		  AND e.event_type = 'activated'
	)
	SELECT
		activity_date,
		surface,
		funnel_step,
		COUNT(DISTINCT user_id)::bigint AS users_count,
		SUM(event_count)::bigint AS events_count,
		now()
	FROM events
	GROUP BY 1, 2, 3
	ON CONFLICT (activity_date, surface, funnel_step)
	DO UPDATE SET
		users_count = EXCLUDED.users_count,
		events_count = EXCLUDED.events_count,
		updated_at = now()`

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

// AggregateJobErrors snapshots bounded provider/job error dimensions before
// short-lived diagnostics are redacted. It never stores prompts or raw payloads.
func (r *MaintenanceRepository) AggregateJobErrors(ctx context.Context, since time.Time) (int64, error) {
	const q = `
		INSERT INTO job_error_aggregates (
			bucket_date,
			surface,
			operation_type,
			modality,
			provider,
			model_code,
			job_status,
			error_class,
			count,
			last_seen_at,
			updated_at
		)
		SELECT
			date_trunc('day', COALESCE(pt.updated_at, j.updated_at, j.created_at))::date AS bucket_date,
			CASE WHEN j.command_id IS NULL THEN 'miniapp_or_api' ELSE 'vk_bot' END AS surface,
			COALESCE(NULLIF(j.operation_type::text, ''), 'unknown') AS operation_type,
			COALESCE(NULLIF(j.modality::text, ''), 'unknown') AS modality,
			COALESCE(NULLIF(pt.provider::text, ''), 'unknown') AS provider,
			COALESCE(NULLIF(pt.model_code, ''), 'unknown') AS model_code,
			COALESCE(NULLIF(j.status::text, ''), 'unknown') AS job_status,
			COALESCE(NULLIF(pt.error_class, ''), NULLIF(j.error_code, ''), 'unknown') AS error_class,
			COUNT(*)::bigint AS count,
			MAX(COALESCE(pt.updated_at, j.updated_at, j.created_at)) AS last_seen_at,
			now() AS updated_at
		FROM jobs j
		LEFT JOIN LATERAL (
			SELECT provider, model_code, error_class, updated_at
			FROM provider_tasks p
			WHERE p.job_id = j.id
			ORDER BY p.updated_at DESC, p.attempt_no DESC, p.id DESC
			LIMIT 1
		) pt ON true
		WHERE COALESCE(pt.updated_at, j.updated_at, j.created_at) >= $1
		  AND (NULLIF(pt.error_class, '') IS NOT NULL OR NULLIF(j.error_code, '') IS NOT NULL)
		GROUP BY 1, 2, 3, 4, 5, 6, 7, 8
		ON CONFLICT (
			bucket_date,
			surface,
			operation_type,
			modality,
			provider,
			model_code,
			job_status,
			error_class
		)
		DO UPDATE SET
			count = EXCLUDED.count,
			last_seen_at = EXCLUDED.last_seen_at,
			updated_at = now()`
	tag, err := r.db.Exec(ctx, q, since)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

// RefreshDailyAnalyticsAggregates upserts no-PII daily snapshots for dashboards.
// It intentionally reads raw hot tables once in maintenance and stores only
// bounded aggregate dimensions.
func (r *MaintenanceRepository) RefreshDailyAnalyticsAggregates(ctx context.Context, from, to time.Time) (int64, error) {
	if from.IsZero() || to.IsZero() || !from.Before(to) {
		return 0, nil
	}
	queries := []string{
		refreshDailyUserActivitySQL,
		refreshDailyGenerationStatsSQL,
		refreshDailyProviderStatsSQL,
		refreshDailyRevenueStatsSQL,
		refreshDailyReferralStatsSQL,
		refreshDailyRetentionStatsSQL,
		refreshDailyFunnelStatsSQL,
	}
	var total int64
	for _, q := range queries {
		tag, err := r.db.Exec(ctx, q, from, to)
		if err != nil {
			return total, mapError(err)
		}
		total += tag.RowsAffected()
	}
	return total, nil
}

// CleanupJobEvents removes old lifecycle diagnostics when the optional
// job_events table exists. The current schema may not have this table yet.
func (r *MaintenanceRepository) CleanupJobEvents(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 500
	}
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT to_regclass('public.job_events') IS NOT NULL`).Scan(&exists); err != nil {
		return 0, mapError(err)
	}
	if !exists {
		return 0, nil
	}
	const q = `
		WITH candidates AS (
			SELECT id
			FROM job_events
			WHERE created_at <= $1
			ORDER BY created_at ASC, id ASC
			LIMIT $2
		)
		DELETE FROM job_events e
		USING candidates c
		WHERE e.id = c.id`
	tag, err := r.db.Exec(ctx, q, cutoff, limit)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

// ExpireProviderPayloads marks terminal provider payloads for redaction while
// preserving provider task ids, status, timing and error class for support.
func (r *MaintenanceRepository) ExpireProviderPayloads(ctx context.Context, cutoff, expiresAt time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 500
	}
	const q = `
		WITH candidates AS (
			SELECT id
			FROM provider_tasks
			WHERE retention_class = $1
			  AND deleted_at IS NULL
			  AND redacted_at IS NULL
			  AND expires_at IS NULL
			  AND created_at <= $2
			  AND (completed_at IS NOT NULL OR error_class <> '')
			ORDER BY created_at ASC, id ASC
			LIMIT $4
		)
		UPDATE provider_tasks p
		SET expires_at = $3,
		    updated_at = now()
		FROM candidates c
		WHERE p.id = c.id`
	tag, err := r.db.Exec(ctx, q, domain.DataClassProviderPayload, cutoff, expiresAt, limit)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

// RedactExpiredProviderPayloads removes raw provider request/response JSON from
// expired diagnostics. Normalized task metadata remains available.
func (r *MaintenanceRepository) RedactExpiredProviderPayloads(ctx context.Context, now time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 500
	}
	const q = `
		WITH candidates AS (
			SELECT id
			FROM provider_tasks
			WHERE retention_class = $1
			  AND deleted_at IS NULL
			  AND redacted_at IS NULL
			  AND expires_at IS NOT NULL
			  AND expires_at <= $2
			ORDER BY expires_at ASC, id ASC
			LIMIT $3
		)
		UPDATE provider_tasks p
		SET request = '{}'::jsonb,
		    result = NULL,
		    redacted_at = $2,
		    updated_at = now()
		FROM candidates c
		WHERE p.id = c.id`
	tag, err := r.db.Exec(ctx, q, domain.DataClassProviderPayload, now, limit)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

// ExpireInboundEvents marks old VK inbound payloads for redaction while
// preserving idempotency and support metadata columns.
func (r *MaintenanceRepository) ExpireInboundEvents(ctx context.Context, cutoff, expiresAt time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 500
	}
	const q = `
		WITH candidates AS (
			SELECT id
			FROM inbound_events
			WHERE retention_class = $1
			  AND source = 'vk'
			  AND deleted_at IS NULL
			  AND redacted_at IS NULL
			  AND expires_at IS NULL
			  AND created_at <= $2
			ORDER BY created_at ASC, id ASC
			LIMIT $4
		)
		UPDATE inbound_events e
		SET expires_at = $3,
		    updated_at = now()
		FROM candidates c
		WHERE e.id = c.id`
	tag, err := r.db.Exec(ctx, q, domain.DataClassUserContent, cutoff, expiresAt, limit)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

// RedactExpiredInboundEvents replaces raw VK callback JSON with bounded
// metadata. Idempotency keys, source ids and status columns remain intact.
func (r *MaintenanceRepository) RedactExpiredInboundEvents(ctx context.Context, now time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 500
	}
	const q = `
		WITH candidates AS (
			SELECT id
			FROM inbound_events
			WHERE retention_class = $1
			  AND source = 'vk'
			  AND deleted_at IS NULL
			  AND redacted_at IS NULL
			  AND expires_at IS NOT NULL
			  AND expires_at <= $2
			ORDER BY expires_at ASC, id ASC
			LIMIT $3
		)
		UPDATE inbound_events e
		SET payload = jsonb_build_object(
		        'redacted', true,
		        'source', e.source,
		        'event_type', e.event_type,
		        'payload_class', 'vk_callback_metadata',
		        'has_vk_event_id', e.vk_event_id <> '',
		        'has_group_id', e.group_id <> 0,
		        'has_peer_id', e.peer_id <> 0,
		        'has_vk_user_id', e.vk_user_id <> 0
		    ),
		    redacted_at = $2,
		    updated_at = now()
		FROM candidates c
		WHERE e.id = c.id`
	tag, err := r.db.Exec(ctx, q, domain.DataClassUserContent, now, limit)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

// ExpireCommandRawText marks old normalized command text for redaction. Command
// rows, idempotency keys and job links stay intact. Commands with unfinished
// linked jobs are skipped so active work can still reconstruct from durable job
// params and metadata.
func (r *MaintenanceRepository) ExpireCommandRawText(ctx context.Context, cutoff, expiresAt time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 500
	}
	const q = `
		WITH candidates AS (
			SELECT c.id
			FROM commands c
			WHERE c.retention_class = $1
			  AND c.deleted_at IS NULL
			  AND c.redacted_at IS NULL
			  AND c.expires_at IS NULL
			  AND c.created_at <= $2
			  AND NOT EXISTS (
			      SELECT 1
			      FROM jobs j
			      WHERE j.command_id = c.id
			        AND j.status <> ALL($5::text[])
			  )
			ORDER BY c.created_at ASC, c.id ASC
			LIMIT $4
		)
		UPDATE commands c
		SET expires_at = $3,
		    updated_at = now()
		FROM candidates candidate
		WHERE c.id = candidate.id`
	tag, err := r.db.Exec(ctx, q, domain.DataClassUserContent, cutoff, expiresAt, limit, commandRawTextSafeJobStatuses())
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

// RedactExpiredCommandRawText removes old raw user message text while keeping
// command identity, args, attachment references and job relationships.
func (r *MaintenanceRepository) RedactExpiredCommandRawText(ctx context.Context, now time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 500
	}
	const q = `
		WITH candidates AS (
			SELECT c.id
			FROM commands c
			WHERE c.retention_class = $1
			  AND c.deleted_at IS NULL
			  AND c.redacted_at IS NULL
			  AND c.expires_at IS NOT NULL
			  AND c.expires_at <= $2
			  AND NOT EXISTS (
			      SELECT 1
			      FROM jobs j
			      WHERE j.command_id = c.id
			        AND j.status <> ALL($4::text[])
			  )
			ORDER BY c.expires_at ASC, c.created_at ASC, c.id ASC
			LIMIT $3
		)
		UPDATE commands c
		SET raw_text = '',
		    redacted_at = $2,
		    updated_at = now()
		FROM candidates candidate
		WHERE c.id = candidate.id`
	tag, err := r.db.Exec(ctx, q, domain.DataClassUserContent, now, limit, commandRawTextSafeJobStatuses())
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

func commandRawTextSafeJobStatuses() []string {
	return []string{
		string(domain.JobStatusSucceeded),
		string(domain.JobStatusFailedTerminal),
		string(domain.JobStatusCancelled),
		string(domain.JobStatusExpired),
		string(domain.JobStatusRefunded),
		string(domain.JobStatusRejected),
	}
}

// ExpireConversationMessages marks old raw conversation messages for redaction.
// It preserves rows so sequence numbers and support metadata stay stable.
func (r *MaintenanceRepository) ExpireConversationMessages(ctx context.Context, cutoff, expiresAt time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 500
	}
	const q = `
		WITH candidates AS (
			SELECT id
			FROM conversation_messages
			WHERE retention_class = $1
			  AND deleted_at IS NULL
			  AND redacted_at IS NULL
			  AND expires_at IS NULL
			  AND created_at <= $2
			ORDER BY created_at ASC, seq ASC
			LIMIT $4
		)
		UPDATE conversation_messages m
		SET expires_at = $3,
		    updated_at = now()
		FROM candidates c
		WHERE m.id = c.id`
	tag, err := r.db.Exec(ctx, q, domain.DataClassUserContent, cutoff, expiresAt, limit)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

// RedactExpiredConversationMessages removes old raw prompt/answer text while
// keeping message metadata available for ordering and support.
func (r *MaintenanceRepository) RedactExpiredConversationMessages(ctx context.Context, now time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 500
	}
	const q = `
		WITH candidates AS (
			SELECT id
			FROM conversation_messages
			WHERE retention_class = $1
			  AND deleted_at IS NULL
			  AND redacted_at IS NULL
			  AND expires_at IS NOT NULL
			  AND expires_at <= $2
			ORDER BY expires_at ASC, seq ASC
			LIMIT $3
		)
		UPDATE conversation_messages m
		SET text = '',
		    token_count = 0,
		    redacted_at = $2,
		    updated_at = now()
		FROM candidates c
		WHERE m.id = c.id`
	tag, err := r.db.Exec(ctx, q, domain.DataClassUserContent, now, limit)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

// ExpireConversationSummaries marks compact memory summaries after their longer
// retention window. Summaries intentionally live longer than raw messages.
func (r *MaintenanceRepository) ExpireConversationSummaries(ctx context.Context, cutoff, expiresAt time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 500
	}
	const q = `
		WITH candidates AS (
			SELECT id
			FROM conversation_summaries
			WHERE retention_class = $1
			  AND deleted_at IS NULL
			  AND redacted_at IS NULL
			  AND expires_at IS NULL
			  AND updated_at <= $2
			ORDER BY updated_at ASC, id ASC
			LIMIT $4
		)
		UPDATE conversation_summaries s
		SET expires_at = $3,
		    updated_at = now()
		FROM candidates c
		WHERE s.id = c.id`
	tag, err := r.db.Exec(ctx, q, domain.DataClassUserContent, cutoff, expiresAt, limit)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

// RedactExpiredConversationSummaries removes compact memory text only after the
// summary retention window, keeping raw message retention shorter.
func (r *MaintenanceRepository) RedactExpiredConversationSummaries(ctx context.Context, now time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 500
	}
	const q = `
		WITH candidates AS (
			SELECT id
			FROM conversation_summaries
			WHERE retention_class = $1
			  AND deleted_at IS NULL
			  AND redacted_at IS NULL
			  AND expires_at IS NOT NULL
			  AND expires_at <= $2
			ORDER BY expires_at ASC, id ASC
			LIMIT $3
		)
		UPDATE conversation_summaries s
		SET text = '',
		    token_count = 0,
		    redacted_at = $2,
		    updated_at = now()
		FROM candidates c
		WHERE s.id = c.id`
	tag, err := r.db.Exec(ctx, q, domain.DataClassUserContent, now, limit)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

// ExpireMediaArtifacts marks media objects whose lifecycle retention window has
// elapsed. Physical object deletion is a separate maintenance step.
func (r *MaintenanceRepository) ExpireMediaArtifacts(ctx context.Context, policy domain.MediaCleanupPolicy, expiresAt time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 100
	}
	const artifactsQ = `
		WITH candidates AS (
			SELECT a.id
			FROM artifacts a
			WHERE a.deleted_at IS NULL
			  AND a.expires_at IS NULL
			  AND a.storage_bucket <> ''
			  AND a.storage_key <> ''
			  AND (
			      ($1::boolean AND a.artifact_tier = $2 AND a.updated_at <= $3)
			      OR ($4::boolean AND a.artifact_tier = $5 AND a.updated_at <= $6)
			      OR ($7::boolean AND (a.artifact_tier = $8 OR a.lifecycle_class = $9) AND a.updated_at <= $10)
			      OR ($11::boolean AND (a.status IN ($12, $13) OR a.lifecycle_class = $14) AND a.updated_at <= $15)
			      OR (
			          $16::boolean
			          AND a.lifecycle_class = $17
			          AND a.kind = $18
			          AND a.status = $19
			          AND a.updated_at <= $20
			          AND NOT EXISTS (
			              SELECT 1 FROM jobs j
			              WHERE j.input_artifact_ids @> ARRAY[a.id]::uuid[]
			          )
			      )
			      OR (
			          $21::boolean
			          AND a.lifecycle_class = $22
			          AND a.status = $19
			          AND a.updated_at <= $23
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
			      OR (
			          $24::boolean
			          AND a.updated_at <= $25
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
			  )
			ORDER BY a.updated_at ASC, a.id ASC
			LIMIT $26
		)
		UPDATE artifacts a
		SET expires_at = $27,
		    retention_class = $28,
		    updated_at = now()
		FROM candidates c
		WHERE a.id = c.id`
	artifactsTag, err := r.db.Exec(ctx, artifactsQ,
		!policy.FreeArtifactCutoff.IsZero(), domain.ArtifactTierFree, policy.FreeArtifactCutoff,
		!policy.PaidArtifactCutoff.IsZero(), domain.ArtifactTierPaid, policy.PaidArtifactCutoff,
		!policy.TemporaryArtifactCutoff.IsZero(), domain.ArtifactTierTemporary, domain.ArtifactLifecycleTempUpload, policy.TemporaryArtifactCutoff,
		!policy.FailedDeletedCutoff.IsZero(), domain.ArtifactStatusFailed, domain.ArtifactStatusDeleted, domain.ArtifactLifecycleFailedDeleted, policy.FailedDeletedCutoff,
		!policy.InputReferenceCutoff.IsZero(), domain.ArtifactLifecycleInputReference, domain.ArtifactKindInput, domain.ArtifactStatusReady, policy.InputReferenceCutoff,
		!policy.ProviderOriginalCutoff.IsZero(), domain.ArtifactLifecycleProviderOriginal, policy.ProviderOriginalCutoff,
		!policy.OrphanArtifactCutoff.IsZero(), policy.OrphanArtifactCutoff,
		limit, expiresAt, domain.DataClassArtifactMetadata,
	)
	if err != nil {
		return 0, mapError(err)
	}

	const variantsQ = `
		WITH candidates AS (
			SELECT v.id
			FROM artifact_variants v
			JOIN artifacts a ON a.id = v.artifact_id
			WHERE v.deleted_at IS NULL
			  AND v.expires_at IS NULL
			  AND v.storage_bucket <> ''
			  AND v.storage_key <> ''
			  AND (
			      (a.expires_at IS NOT NULL AND a.expires_at <= $1)
			      OR ($2::boolean AND v.artifact_tier = $3 AND v.updated_at <= $4)
			      OR ($5::boolean AND v.artifact_tier = $6 AND v.updated_at <= $7)
			      OR ($8::boolean AND (v.artifact_tier = $9 OR v.lifecycle_class = $10) AND v.updated_at <= $11)
			      OR ($12::boolean AND (a.status IN ($13, $14) OR a.lifecycle_class = $15 OR v.lifecycle_class = $15) AND v.updated_at <= $16)
			      OR ($17::boolean AND v.lifecycle_class = $18 AND v.updated_at <= $19)
			      OR (
			          $20::boolean
			          AND v.updated_at <= $21
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
			  )
			ORDER BY v.updated_at ASC, v.id ASC
			LIMIT $22
		)
		UPDATE artifact_variants v
		SET expires_at = $23,
		    retention_class = $24,
		    updated_at = now()
		FROM candidates c
		WHERE v.id = c.id`
	variantsTag, err := r.db.Exec(ctx, variantsQ,
		expiresAt,
		!policy.FreeArtifactCutoff.IsZero(), domain.ArtifactTierFree, policy.FreeArtifactCutoff,
		!policy.PaidArtifactCutoff.IsZero(), domain.ArtifactTierPaid, policy.PaidArtifactCutoff,
		!policy.TemporaryArtifactCutoff.IsZero(), domain.ArtifactTierTemporary, domain.ArtifactLifecycleTempUpload, policy.TemporaryArtifactCutoff,
		!policy.FailedDeletedCutoff.IsZero(), domain.ArtifactStatusFailed, domain.ArtifactStatusDeleted, domain.ArtifactLifecycleFailedDeleted, policy.FailedDeletedCutoff,
		!policy.DeliveryVariantCutoff.IsZero(), domain.ArtifactLifecycleDeliveryVariant, policy.DeliveryVariantCutoff,
		!policy.OrphanArtifactCutoff.IsZero(), policy.OrphanArtifactCutoff,
		limit, expiresAt, domain.DataClassArtifactMetadata,
	)
	if err != nil {
		return 0, mapError(err)
	}
	return artifactsTag.RowsAffected() + variantsTag.RowsAffected(), nil
}

// MediaCleanupCandidates returns media objects already marked expired by the DB
// lifecycle phase. Physical deletion must never run ahead of expires_at.
func (r *MaintenanceRepository) MediaCleanupCandidates(ctx context.Context, policy domain.MediaCleanupPolicy, limit int) ([]domain.MediaCleanupCandidate, error) {
	if limit <= 0 {
		limit = 100
	}
	if policy.ExpiredCutoff.IsZero() {
		return nil, nil
	}
	const q = `
		SELECT kind, cleanup_class, artifact_id, variant_id, variant_type, media_type, storage_bucket, storage_key, size_bytes
		FROM (
			SELECT
				$1::text AS kind,
				a.lifecycle_class AS cleanup_class,
				a.id AS artifact_id,
				'00000000-0000-0000-0000-000000000000'::uuid AS variant_id,
				$2::text AS variant_type,
				a.media_type,
				a.storage_bucket,
				a.storage_key,
				a.size_bytes
			FROM artifacts a
			WHERE a.expires_at IS NOT NULL
			  AND a.expires_at <= $3
			  AND a.deleted_at IS NULL
			  AND a.storage_bucket <> ''
			  AND a.storage_key <> ''
			UNION ALL
			SELECT
				$4::text AS kind,
				v.lifecycle_class AS cleanup_class,
				a.id AS artifact_id,
				v.id AS variant_id,
				v.variant_type,
				a.media_type,
				v.storage_bucket,
				v.storage_key,
				v.size_bytes
			FROM artifact_variants v
			JOIN artifacts a ON a.id = v.artifact_id
			WHERE v.expires_at IS NOT NULL
			  AND v.expires_at <= $3
			  AND v.deleted_at IS NULL
			  AND v.storage_bucket <> ''
			  AND v.storage_key <> ''
		) cleanup_candidates
		ORDER BY cleanup_class, kind, artifact_id, variant_id
		LIMIT $5`
	rows, err := r.db.Query(ctx, q,
		domain.MediaCleanupOriginal,
		domain.VariantOriginal,
		policy.ExpiredCutoff,
		domain.MediaCleanupVariant,
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
				deleted_at = COALESCE(deleted_at, now()),
				expires_at = COALESCE(expires_at, now()),
				updated_at = now()
			WHERE id = $1
			  AND storage_bucket = $4
			  AND storage_key = $5
			  AND (
			      (expires_at IS NOT NULL AND expires_at <= now())
			      OR status IN ($6, $7)
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
				deleted_at = COALESCE(v.deleted_at, now()),
				expires_at = COALESCE(v.expires_at, now()),
				updated_at = now()
			FROM artifacts a
			WHERE v.id = $1
			  AND v.artifact_id = a.id
			  AND v.storage_bucket = $2
			  AND v.storage_key = $3
			  AND (
			      (v.expires_at IS NOT NULL AND v.expires_at <= now())
			      OR a.status IN ($4, $5)
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

type retentionStatusSpec struct {
	tableName      string
	retentionClass domain.DataClass
	createdColumn  string
	sizeColumn     string
}

func retentionStatusSpecs() []retentionStatusSpec {
	return []retentionStatusSpec{
		{tableName: "jobs", retentionClass: domain.DataClassOperational, createdColumn: "created_at"},
		{tableName: "commands", retentionClass: domain.DataClassUserContent, createdColumn: "created_at"},
		{tableName: "provider_tasks", retentionClass: domain.DataClassProviderPayload, createdColumn: "created_at"},
		{tableName: "inbound_events", retentionClass: domain.DataClassUserContent, createdColumn: "created_at"},
		{tableName: "conversation_messages", retentionClass: domain.DataClassUserContent, createdColumn: "created_at"},
		{tableName: "conversation_summaries", retentionClass: domain.DataClassUserContent, createdColumn: "created_at"},
		{tableName: "artifacts", retentionClass: domain.DataClassArtifactMetadata, createdColumn: "created_at", sizeColumn: "size_bytes"},
		{tableName: "artifact_variants", retentionClass: domain.DataClassArtifactMetadata, createdColumn: "created_at", sizeColumn: "size_bytes"},
	}
}

// RetentionStatus returns a safe read-only retention posture snapshot for the
// admin/operator API. It never selects raw prompts, payloads, ids or storage
// coordinates.
func (r *MaintenanceRepository) RetentionStatus(ctx context.Context, now time.Time) (domain.RetentionStatus, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	out := domain.RetentionStatus{GeneratedAt: now}
	for _, spec := range retentionStatusSpecs() {
		exists, err := r.tableExists(ctx, spec.tableName)
		if err != nil {
			return out, err
		}
		if !exists {
			continue
		}
		item, err := r.retentionStatusItem(ctx, spec, now)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

func (r *MaintenanceRepository) retentionStatusItem(ctx context.Context, spec retentionStatusSpec, now time.Time) (domain.RetentionStatusItem, error) {
	q := fmt.Sprintf(`
		SELECT
			COUNT(*)::bigint,
			(COUNT(*) FILTER (WHERE expires_at IS NOT NULL AND expires_at <= $1 AND deleted_at IS NULL))::bigint,
			(COUNT(*) FILTER (WHERE redacted_at IS NOT NULL))::bigint,
			(COUNT(*) FILTER (WHERE deleted_at IS NOT NULL))::bigint,
			MIN(%[1]s) FILTER (WHERE deleted_at IS NULL AND redacted_at IS NULL AND (expires_at IS NULL OR expires_at > $1)),
			MIN(expires_at) FILTER (WHERE expires_at IS NOT NULL AND expires_at <= $1 AND deleted_at IS NULL)
		FROM %[2]s
		WHERE retention_class = $2`, spec.createdColumn, spec.tableName)
	var item domain.RetentionStatusItem
	var oldestHot sql.NullTime
	var oldestExpired sql.NullTime
	if err := r.db.QueryRow(ctx, q, now, string(spec.retentionClass)).Scan(
		&item.TotalRows,
		&item.ExpiredRows,
		&item.RedactedRows,
		&item.DeletedRows,
		&oldestHot,
		&oldestExpired,
	); err != nil {
		return item, mapError(err)
	}
	item.TableName = spec.tableName
	item.RetentionClass = spec.retentionClass
	item.OldestHotAt = nullableSQLTime(oldestHot)
	item.OldestExpiredAt = nullableSQLTime(oldestExpired)
	return item, nil
}

// RetentionDryRun reports rows/bytes the maintenance cleanup would be able to
// process right now based on persisted retention markers. It performs no
// mutation.
func (r *MaintenanceRepository) RetentionDryRun(ctx context.Context, now time.Time, limit int) (domain.RetentionDryRun, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 {
		limit = 100
	}
	out := domain.RetentionDryRun{GeneratedAt: now}
	for _, spec := range retentionStatusSpecs() {
		exists, err := r.tableExists(ctx, spec.tableName)
		if err != nil {
			return out, err
		}
		if !exists {
			continue
		}
		item, err := r.retentionDryRunItem(ctx, spec, now)
		if err != nil {
			return out, err
		}
		if item.Count > 0 {
			out.Items = append(out.Items, item)
			if len(out.Items) >= limit {
				break
			}
		}
	}
	orphan, err := r.OrphanArtifactsCount(ctx, now)
	if err != nil {
		return out, err
	}
	if orphan.Total > 0 && len(out.Items) < limit {
		out.Items = append(out.Items, domain.RetentionDryRunItem{
			Action:         "delete_orphan_artifact_objects",
			TableName:      "artifacts",
			RetentionClass: domain.DataClassArtifactMetadata,
			Count:          orphan.Total,
			Bytes:          orphan.Bytes,
			OldestAt:       oldestOrphanArtifactAt(orphan.Items),
		})
	}
	return out, nil
}

func (r *MaintenanceRepository) retentionDryRunItem(ctx context.Context, spec retentionStatusSpec, now time.Time) (domain.RetentionDryRunItem, error) {
	bytesExpr := "0::bigint"
	if spec.sizeColumn != "" {
		bytesExpr = fmt.Sprintf("COALESCE(SUM(%s) FILTER (WHERE expires_at IS NOT NULL AND expires_at <= $1 AND deleted_at IS NULL), 0)::bigint", spec.sizeColumn)
	}
	q := fmt.Sprintf(`
		SELECT
			(COUNT(*) FILTER (WHERE expires_at IS NOT NULL AND expires_at <= $1 AND deleted_at IS NULL))::bigint,
			%[1]s,
			MIN(%[2]s) FILTER (WHERE expires_at IS NOT NULL AND expires_at <= $1 AND deleted_at IS NULL)
		FROM %[3]s
		WHERE retention_class = $2`, bytesExpr, spec.createdColumn, spec.tableName)
	var count int64
	var bytes int64
	var oldest sql.NullTime
	if err := r.db.QueryRow(ctx, q, now, string(spec.retentionClass)).Scan(&count, &bytes, &oldest); err != nil {
		return domain.RetentionDryRunItem{}, mapError(err)
	}
	return domain.RetentionDryRunItem{
		Action:         "process_expired_rows",
		TableName:      spec.tableName,
		RetentionClass: spec.retentionClass,
		Count:          count,
		Bytes:          bytes,
		OldestAt:       nullableSQLTime(oldest),
	}, nil
}

// AnalyticsAggregationStatus reports population freshness for no-PII aggregate
// tables used by dashboards.
func (r *MaintenanceRepository) AnalyticsAggregationStatus(ctx context.Context) (domain.AnalyticsAggregationStatus, error) {
	now := time.Now().UTC()
	out := domain.AnalyticsAggregationStatus{GeneratedAt: now}
	for _, table := range []string{
		"daily_user_activity",
		"daily_generation_stats",
		"daily_provider_stats",
		"daily_revenue_stats",
		"daily_referral_stats",
		"daily_retention_stats",
		"daily_funnel_stats",
		"job_error_aggregates",
	} {
		exists, err := r.tableExists(ctx, table)
		if err != nil {
			return out, err
		}
		if !exists {
			out.Items = append(out.Items, domain.AnalyticsAggregationStatusItem{
				TableName: table,
				Status:    "not_wired",
			})
			continue
		}
		item, err := r.analyticsAggregationStatusItem(ctx, table)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

func (r *MaintenanceRepository) analyticsAggregationStatusItem(ctx context.Context, tableName string) (domain.AnalyticsAggregationStatusItem, error) {
	dateColumn := "activity_date"
	if tableName == "job_error_aggregates" {
		dateColumn = "bucket_date"
	}
	q := fmt.Sprintf(`
		SELECT
			COUNT(*)::bigint,
			MAX(%[1]s)::timestamptz,
			MAX(updated_at)
		FROM %[2]s`, dateColumn, tableName)
	var rows int64
	var latest sql.NullTime
	var updated sql.NullTime
	if err := r.db.QueryRow(ctx, q).Scan(&rows, &latest, &updated); err != nil {
		return domain.AnalyticsAggregationStatusItem{}, mapError(err)
	}
	status := "ok"
	if rows == 0 {
		status = "empty"
	}
	return domain.AnalyticsAggregationStatusItem{
		TableName:          tableName,
		Status:             status,
		Rows:               rows,
		LatestActivityDate: nullableSQLTime(latest),
		LastUpdatedAt:      nullableSQLTime(updated),
	}, nil
}

// OldestHotRows returns bounded age signals for hot tables. It never returns
// raw content or entity identifiers.
func (r *MaintenanceRepository) OldestHotRows(ctx context.Context) (domain.OldestHotRowsReport, error) {
	now := time.Now().UTC()
	out := domain.OldestHotRowsReport{GeneratedAt: now}
	for _, spec := range retentionStatusSpecs() {
		exists, err := r.tableExists(ctx, spec.tableName)
		if err != nil {
			return out, err
		}
		if !exists {
			continue
		}
		item, err := r.oldestHotRowsItem(ctx, spec, now)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

func (r *MaintenanceRepository) oldestHotRowsItem(ctx context.Context, spec retentionStatusSpec, now time.Time) (domain.OldestHotRow, error) {
	q := fmt.Sprintf(`
		SELECT
			COUNT(*)::bigint,
			MIN(%[1]s)
		FROM %[2]s
		WHERE retention_class = $1
		  AND deleted_at IS NULL
		  AND redacted_at IS NULL`, spec.createdColumn, spec.tableName)
	var count int64
	var oldest sql.NullTime
	if err := r.db.QueryRow(ctx, q, string(spec.retentionClass)).Scan(&count, &oldest); err != nil {
		return domain.OldestHotRow{}, mapError(err)
	}
	oldestPtr := nullableSQLTime(oldest)
	return domain.OldestHotRow{
		TableName:      spec.tableName,
		RetentionClass: spec.retentionClass,
		Count:          count,
		OldestAt:       oldestPtr,
		AgeSeconds:     ageSeconds(now, oldestPtr),
	}, nil
}

// OrphanArtifactsCount returns object-cleanup candidates grouped by safe
// artifact metadata. It does not expose storage coordinates or owners.
func (r *MaintenanceRepository) OrphanArtifactsCount(ctx context.Context, now time.Time) (domain.OrphanArtifactsReport, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	out := domain.OrphanArtifactsReport{GeneratedAt: now}
	exists, err := r.tableExists(ctx, "artifacts")
	if err != nil {
		return out, err
	}
	if !exists {
		return out, nil
	}
	const q = `
		SELECT
			artifact_tier,
			lifecycle_class,
			status,
			media_type,
			COUNT(*)::bigint,
			COALESCE(SUM(size_bytes), 0)::bigint,
			MIN(updated_at)
		FROM artifacts a
		WHERE a.deleted_at IS NULL
		  AND a.storage_bucket <> ''
		  AND a.storage_key <> ''
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
		GROUP BY artifact_tier, lifecycle_class, status, media_type
		ORDER BY COUNT(*) DESC, artifact_tier, lifecycle_class, status, media_type`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return out, mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var item domain.OrphanArtifactCount
		var oldest sql.NullTime
		if err := rows.Scan(
			&item.ArtifactTier,
			&item.LifecycleClass,
			&item.Status,
			&item.MediaType,
			&item.Count,
			&item.Bytes,
			&oldest,
		); err != nil {
			return out, mapError(err)
		}
		item.OldestAt = nullableSQLTime(oldest)
		out.Total += item.Count
		out.Bytes += item.Bytes
		out.Items = append(out.Items, item)
	}
	return out, mapError(rows.Err())
}

func (r *MaintenanceRepository) tableExists(ctx context.Context, tableName string) (bool, error) {
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, tableName).Scan(&exists); err != nil {
		return false, mapError(err)
	}
	return exists, nil
}

func nullableSQLTime(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

func ageSeconds(now time.Time, t *time.Time) int64 {
	if t == nil || now.Before(*t) {
		return 0
	}
	return int64(now.Sub(*t).Seconds())
}

func oldestOrphanArtifactAt(items []domain.OrphanArtifactCount) *time.Time {
	var oldest *time.Time
	for _, item := range items {
		if item.OldestAt == nil {
			continue
		}
		if oldest == nil || item.OldestAt.Before(*oldest) {
			value := *item.OldestAt
			oldest = &value
		}
	}
	return oldest
}
