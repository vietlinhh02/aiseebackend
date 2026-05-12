CREATE TABLE ai_cost_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    usage_ledger_id UUID REFERENCES usage_ledger(id) ON DELETE SET NULL,
    session_id UUID REFERENCES usage_sessions(id) ON DELETE SET NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    modality TEXT NOT NULL DEFAULT 'live_audio',
    source TEXT NOT NULL,
    source_id TEXT NOT NULL,
    input_tokens BIGINT NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
    output_tokens BIGINT NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
    audio_input_seconds NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (audio_input_seconds >= 0),
    audio_output_seconds NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (audio_output_seconds >= 0),
    image_count BIGINT NOT NULL DEFAULT 0 CHECK (image_count >= 0),
    video_seconds NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (video_seconds >= 0),
    cost_usd NUMERIC(14,6) NOT NULL DEFAULT 0 CHECK (cost_usd >= 0),
    cost_vnd NUMERIC(16,2) NOT NULL DEFAULT 0 CHECK (cost_vnd >= 0),
    currency TEXT NOT NULL DEFAULT 'USD',
    estimated BOOLEAN NOT NULL DEFAULT true,
    pricing_version TEXT NOT NULL DEFAULT '',
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, source, source_id, provider, model, modality, occurred_at)
);

CREATE INDEX idx_ai_cost_events_user_period
    ON ai_cost_events(user_id, occurred_at DESC);
CREATE INDEX idx_ai_cost_events_model_period
    ON ai_cost_events(provider, model, occurred_at DESC);
CREATE INDEX idx_ai_cost_events_source
    ON ai_cost_events(source, source_id);
