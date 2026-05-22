CREATE TABLE family_invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL UNIQUE,
    cared_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    relationship TEXT NOT NULL,
    created_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    max_uses INTEGER CHECK (max_uses IS NULL OR max_uses > 0),
    usage_count INTEGER NOT NULL DEFAULT 0 CHECK (usage_count >= 0),
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_family_invites_cared_user
    ON family_invites(cared_user_id, created_at DESC);

CREATE INDEX idx_family_invites_active
    ON family_invites(expires_at, revoked_at)
    WHERE revoked_at IS NULL;
