-- 001_init.up.sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    google_id TEXT UNIQUE NOT NULL,
    email TEXT NOT NULL,
    display_name TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('user', 'family')),
    is_volunteer BOOLEAN DEFAULT false,
    avatar_url TEXT,
    fcm_token TEXT,
    apns_token TEXT,
    apns_voip_token TEXT,
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE family_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    family_id UUID NOT NULL REFERENCES users(id),
    relationship TEXT NOT NULL,
    is_primary_contact BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(user_id, family_id)
);

CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id UUID NOT NULL REFERENCES users(id),
    receiver_id UUID NOT NULL REFERENCES users(id),
    content_text TEXT,
    voice_url TEXT,
    is_read BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE medications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    dosage TEXT NOT NULL,
    times TEXT[] NOT NULL,
    frequency TEXT NOT NULL DEFAULT 'daily',
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE medication_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    medication_id UUID NOT NULL REFERENCES medications(id),
    taken_at TIMESTAMPTZ DEFAULT now(),
    skipped BOOLEAN DEFAULT false
);

CREATE TABLE health_symptoms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    symptom TEXT NOT NULL,
    severity TEXT,
    notes TEXT,
    logged_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE health_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    metric_type TEXT NOT NULL,
    value TEXT NOT NULL,
    unit TEXT,
    logged_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE appointments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    title TEXT NOT NULL,
    location TEXT,
    datetime TIMESTAMPTZ NOT NULL,
    notes TEXT,
    reminded BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE volunteers (
    user_id UUID PRIMARY KEY REFERENCES users(id),
    available BOOLEAN DEFAULT false,
    languages TEXT[] DEFAULT '{vi}',
    rating REAL DEFAULT 5.0,
    total_calls INTEGER DEFAULT 0
);

CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    tool_name TEXT NOT NULL,
    params JSONB,
    result TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Indexes
CREATE INDEX idx_messages_receiver_unread ON messages(receiver_id, is_read) WHERE NOT is_read;
CREATE INDEX idx_messages_sender ON messages(sender_id, created_at DESC);
CREATE INDEX idx_medications_user ON medications(user_id) WHERE active;
CREATE INDEX idx_health_symptoms_user_time ON health_symptoms(user_id, logged_at DESC);
CREATE INDEX idx_health_metrics_user_type_time ON health_metrics(user_id, metric_type, logged_at DESC);
CREATE INDEX idx_appointments_user_time ON appointments(user_id, datetime) WHERE deleted_at IS NULL;
CREATE INDEX idx_family_links_user ON family_links(user_id);
CREATE INDEX idx_family_links_family ON family_links(family_id);
CREATE INDEX idx_audit_logs_user_time ON audit_logs(user_id, created_at DESC);
