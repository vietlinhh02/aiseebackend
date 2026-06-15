-- Bảng lưu lịch sử cuốc đi bộ (in-app walking navigation)
-- Agent ghi vào bằng record_usage với usage_type='navigation_trips', unit='trips'
-- App đọc lại qua GET /api/v1/me/navigation-history
CREATE TABLE IF NOT EXISTS navigation_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    destination_query TEXT NOT NULL,
    destination_name TEXT,
    origin_lat DOUBLE PRECISION NOT NULL,
    origin_lng DOUBLE PRECISION NOT NULL,
    destination_lat DOUBLE PRECISION,
    destination_lng DOUBLE PRECISION,
    total_distance_m DOUBLE PRECISION NOT NULL DEFAULT 0,
    total_duration_estimated_s DOUBLE PRECISION NOT NULL DEFAULT 0,
    actual_duration_s INTEGER NOT NULL DEFAULT 0,
    total_steps INTEGER NOT NULL DEFAULT 0,
    steps_completed INTEGER NOT NULL DEFAULT 0,
    deviation_count INTEGER NOT NULL DEFAULT 0,
    outcome TEXT NOT NULL DEFAULT 'in_progress'
        CHECK (outcome IN ('in_progress', 'completed', 'cancelled', 'deviated', 'error')),
    mode TEXT NOT NULL DEFAULT 'in_app_walking',
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at TIMESTAMPTZ
);

-- Index truy vấn "lịch sử 30 ngày gần nhất của user X"
CREATE INDEX IF NOT EXISTS idx_navigation_history_user_started
    ON navigation_history (user_id, started_at DESC);

-- Index cho weekly report (group theo outcome)
CREATE INDEX IF NOT EXISTS idx_navigation_history_user_outcome
    ON navigation_history (user_id, outcome, started_at DESC);

-- Unique constraint for idempotent upsert (record_usage may retry with same started_at)
CREATE UNIQUE INDEX IF NOT EXISTS uq_navigation_history_user_started
    ON navigation_history (user_id, started_at);
