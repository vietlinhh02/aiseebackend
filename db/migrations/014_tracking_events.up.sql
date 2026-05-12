CREATE TABLE tracking_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    category TEXT NOT NULL CHECK (category IN ('ai_ops', 'safety_care', 'admin_support', 'system')),
    severity TEXT NOT NULL DEFAULT 'info' CHECK (severity IN ('info', 'warning', 'critical')),
    source TEXT NOT NULL,
    source_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'recorded',
    duration_ms BIGINT CHECK (duration_ms IS NULL OR duration_ms >= 0),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_tracking_events_user_period
    ON tracking_events(user_id, occurred_at DESC);

CREATE INDEX idx_tracking_events_category_period
    ON tracking_events(category, occurred_at DESC);

CREATE INDEX idx_tracking_events_type_period
    ON tracking_events(event_type, occurred_at DESC);

CREATE INDEX idx_tracking_events_source
    ON tracking_events(source, source_id);

