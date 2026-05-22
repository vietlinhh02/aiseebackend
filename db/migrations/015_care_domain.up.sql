-- Care domain: events, alerts, incidents, tasks, member permissions, refund notes

CREATE TABLE care_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    actor_role TEXT NOT NULL DEFAULT 'system',
    event_type TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'info',
    title TEXT NOT NULL,
    summary TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_care_events_user_time ON care_events(user_id, occurred_at DESC);
CREATE INDEX idx_care_events_type_time ON care_events(event_type, occurred_at DESC);

CREATE TABLE incidents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'acknowledged', 'resolved', 'closed')),
    room_name TEXT,
    lat DOUBLE PRECISION,
    lng DOUBLE PRECISION,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    opened_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_incidents_user_opened ON incidents(user_id, opened_at DESC);
CREATE INDEX idx_incidents_status ON incidents(status, opened_at DESC);

CREATE TABLE incident_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_incident_events_incident_time ON incident_events(incident_id, occurred_at);

CREATE TABLE care_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rule_code TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'info',
    title TEXT NOT NULL,
    body TEXT,
    status TEXT NOT NULL DEFAULT 'open' CHECK (
        status IN ('open', 'acknowledged', 'resolved', 'dismissed')
    ),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    acknowledged_by UUID REFERENCES users(id) ON DELETE SET NULL,
    acknowledged_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    dismissed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_care_alerts_user_status ON care_alerts(user_id, status, created_at DESC);

CREATE TABLE care_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    due_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'completed', 'cancelled')),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_care_tasks_user_status ON care_tasks(user_id, status, due_at);

CREATE TABLE care_member_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cared_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    family_member_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scopes JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (cared_user_id, family_member_id)
);

CREATE INDEX idx_care_member_permissions_family ON care_member_permissions(family_member_id);

CREATE TABLE billing_refund_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    billing_transaction_id UUID NOT NULL REFERENCES billing_transactions(id) ON DELETE CASCADE,
    note TEXT NOT NULL,
    created_by_admin_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_billing_refund_notes_txn ON billing_refund_notes(billing_transaction_id, created_at DESC);
