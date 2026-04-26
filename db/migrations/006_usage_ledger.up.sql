-- db/migrations/006_usage_ledger.up.sql

CREATE TABLE usage_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    usage_type TEXT NOT NULL CHECK (
        usage_type IN (
            'ai_minutes',
            'camera_activations',
            'voice_call_minutes',
            'video_call_minutes',
            'music_sessions',
            'music_minutes',
            'navigation_sessions',
            'volunteer_matches',
            'pdf_exports',
            'care_coach_questions',
            'pattern_detection_runs'
        )
    ),
    reserved_quantity NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (reserved_quantity >= 0),
    committed_quantity NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (committed_quantity >= 0),
    unit TEXT NOT NULL,
    source TEXT NOT NULL,
    source_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'started' CHECK (
        status IN ('started', 'reserved', 'committed', 'canceled')
    ),
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, source, source_id, usage_type)
);

CREATE TABLE usage_ledger (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID REFERENCES usage_sessions(id) ON DELETE SET NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    usage_type TEXT NOT NULL CHECK (
        usage_type IN (
            'ai_minutes',
            'camera_activations',
            'voice_call_minutes',
            'video_call_minutes',
            'music_sessions',
            'music_minutes',
            'navigation_sessions',
            'volunteer_matches',
            'pdf_exports',
            'care_coach_questions',
            'pattern_detection_runs'
        )
    ),
    quantity NUMERIC(12,2) NOT NULL CHECK (quantity > 0),
    unit TEXT NOT NULL,
    source TEXT NOT NULL,
    source_id TEXT NOT NULL,
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    occurred_at TIMESTAMPTZ GENERATED ALWAYS AS (
        COALESCE(ended_at, started_at, created_at)
    ) STORED,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    UNIQUE(user_id, source, source_id, usage_type)
);

CREATE TABLE usage_quota_grants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    usage_type TEXT NOT NULL CHECK (
        usage_type IN (
            'ai_minutes',
            'camera_activations',
            'voice_call_minutes',
            'video_call_minutes',
            'music_sessions',
            'music_minutes',
            'navigation_sessions',
            'volunteer_matches',
            'pdf_exports',
            'care_coach_questions',
            'pattern_detection_runs'
        )
    ),
    quantity NUMERIC(12,2) NOT NULL CHECK (quantity > 0),
    unit TEXT NOT NULL,
    reason TEXT NOT NULL,
    source TEXT NOT NULL,
    source_id TEXT NOT NULL,
    granted_by_id UUID REFERENCES users(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, source, source_id, usage_type)
);

CREATE INDEX idx_usage_sessions_user_status
    ON usage_sessions(user_id, status, created_at DESC);
CREATE INDEX idx_usage_sessions_source
    ON usage_sessions(source, source_id);
CREATE INDEX idx_usage_ledger_user_period
    ON usage_ledger(user_id, occurred_at DESC, usage_type);
CREATE INDEX idx_usage_ledger_source
    ON usage_ledger(source, source_id);
CREATE INDEX idx_usage_quota_grants_user_period
    ON usage_quota_grants(user_id, created_at DESC, usage_type);
CREATE INDEX idx_usage_quota_grants_source
    ON usage_quota_grants(source, source_id);
