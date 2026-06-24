-- 000028_runtime_pricing_catalog.up.sql
-- DB-backed generation pricing catalog. Historical jobs keep their immutable
-- jobs.pricing_snapshot and are not mutated by this schema.

CREATE TABLE IF NOT EXISTS runtime_pricing_catalog_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    price_version INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    effective_from TIMESTAMPTZ NOT NULL DEFAULT now(),
    effective_until TIMESTAMPTZ,
    created_by TEXT NOT NULL DEFAULT 'system',
    updated_by TEXT NOT NULL DEFAULT 'system',
    note TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT runtime_pricing_catalog_versions_version_key UNIQUE (price_version),
    CONSTRAINT runtime_pricing_catalog_versions_version_positive CHECK (price_version > 0),
    CONSTRAINT runtime_pricing_catalog_versions_status_check CHECK (
        status IN ('draft', 'active', 'retired', 'disabled')
    ),
    CONSTRAINT runtime_pricing_catalog_versions_window_check CHECK (
        effective_until IS NULL OR effective_until > effective_from
    ),
    CONSTRAINT runtime_pricing_catalog_versions_actor_check CHECK (
        length(created_by) BETWEEN 1 AND 128 AND length(updated_by) BETWEEN 1 AND 128
    ),
    CONSTRAINT runtime_pricing_catalog_versions_metadata_object CHECK (
        jsonb_typeof(metadata) = 'object'
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS runtime_pricing_catalog_versions_single_active_idx
    ON runtime_pricing_catalog_versions ((status))
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS runtime_pricing_catalog_versions_effective_idx
    ON runtime_pricing_catalog_versions (status, effective_from DESC, effective_until);

CREATE TABLE IF NOT EXISTS runtime_generation_prices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    catalog_version_id UUID NOT NULL REFERENCES runtime_pricing_catalog_versions (id) ON DELETE CASCADE,
    operation TEXT NOT NULL,
    modality TEXT NOT NULL,
    image_model_id TEXT NOT NULL DEFAULT '',
    video_route_alias TEXT NOT NULL DEFAULT '',
    quality TEXT NOT NULL DEFAULT '',
    resolution TEXT NOT NULL DEFAULT '',
    duration_sec INTEGER NOT NULL DEFAULT 0,
    floor_amount BIGINT NOT NULL,
    floor_unit TEXT NOT NULL,
    multiplier_numerator BIGINT NOT NULL,
    multiplier_denominator BIGINT NOT NULL,
    internal_credit_cap BIGINT NOT NULL DEFAULT 0,
    floor_amount_cap BIGINT NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT false,
    created_by TEXT NOT NULL DEFAULT 'system',
    updated_by TEXT NOT NULL DEFAULT 'system',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT runtime_generation_prices_key_unique UNIQUE (
        catalog_version_id,
        operation,
        modality,
        image_model_id,
        video_route_alias,
        quality,
        resolution,
        duration_sec
    ),
    CONSTRAINT runtime_generation_prices_floor_positive CHECK (floor_amount > 0),
    CONSTRAINT runtime_generation_prices_multiplier_positive CHECK (
        multiplier_numerator > 0 AND multiplier_denominator > 0
    ),
    CONSTRAINT runtime_generation_prices_caps_non_negative CHECK (
        internal_credit_cap >= 0 AND floor_amount_cap >= 0
    ),
    CONSTRAINT runtime_generation_prices_floor_unit_check CHECK (
        floor_unit IN (
            'usd_micros',
            'poyo_credit_micros',
            'apimart_credit_micros',
            'runway_credit_micros',
            'internal_credit_micros'
        )
    ),
    CONSTRAINT runtime_generation_prices_public_key_shape CHECK (
        (
            operation IN ('image_generate', 'image_edit', 'image_upscale')
            AND modality = 'image'
            AND image_model_id <> ''
            AND video_route_alias = ''
            AND quality <> ''
            AND resolution = ''
            AND duration_sec = 0
        )
        OR (
            operation IN ('video_generate', 'video_image_to_video', 'video_extend')
            AND modality = 'video'
            AND image_model_id = ''
            AND video_route_alias <> ''
            AND quality = ''
            AND resolution <> ''
            AND duration_sec > 0
        )
    ),
    CONSTRAINT runtime_generation_prices_dimension_lengths CHECK (
        length(image_model_id) <= 128
        AND length(video_route_alias) <= 128
        AND length(quality) <= 64
        AND length(resolution) <= 64
    ),
    CONSTRAINT runtime_generation_prices_actor_check CHECK (
        length(created_by) BETWEEN 1 AND 128 AND length(updated_by) BETWEEN 1 AND 128
    ),
    CONSTRAINT runtime_generation_prices_metadata_object CHECK (
        jsonb_typeof(metadata) = 'object'
    )
);

CREATE INDEX IF NOT EXISTS runtime_generation_prices_active_lookup_idx
    ON runtime_generation_prices (
        catalog_version_id,
        operation,
        modality,
        image_model_id,
        video_route_alias,
        quality,
        resolution,
        duration_sec
    )
    WHERE enabled;

CREATE INDEX IF NOT EXISTS runtime_generation_prices_version_enabled_idx
    ON runtime_generation_prices (catalog_version_id, enabled);

CREATE TABLE IF NOT EXISTS runtime_pricing_audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    catalog_version_id UUID REFERENCES runtime_pricing_catalog_versions (id) ON DELETE SET NULL,
    generation_price_id UUID REFERENCES runtime_generation_prices (id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    actor_ref TEXT NOT NULL DEFAULT 'system',
    reason TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT runtime_pricing_audit_events_action_check CHECK (
        action IN (
            'version_created',
            'version_activated',
            'version_retired',
            'version_disabled',
            'price_created',
            'price_updated',
            'price_enabled',
            'price_disabled'
        )
    ),
    CONSTRAINT runtime_pricing_audit_events_actor_check CHECK (
        length(actor_ref) BETWEEN 1 AND 128
    ),
    CONSTRAINT runtime_pricing_audit_events_metadata_object CHECK (
        jsonb_typeof(metadata) = 'object'
    )
);

CREATE INDEX IF NOT EXISTS runtime_pricing_audit_events_version_created_idx
    ON runtime_pricing_audit_events (catalog_version_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS runtime_pricing_audit_events_price_created_idx
    ON runtime_pricing_audit_events (generation_price_id, created_at DESC, id DESC);

COMMENT ON TABLE runtime_pricing_catalog_versions IS
    'DB-backed generation pricing versions. One active version may be used at runtime; payment products are separate.';

COMMENT ON TABLE runtime_generation_prices IS
    'DB-backed user-facing generation prices keyed only by public product dimensions; no prompts, secrets, provider payloads or private URLs.';

COMMENT ON TABLE runtime_pricing_audit_events IS
    'Sanitized runtime pricing audit trail with bounded refs and metadata only.';
