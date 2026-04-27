-- db/migrations/007_care_groups_billing.up.sql

CREATE TABLE care_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    owner_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE care_group_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    care_group_id UUID NOT NULL REFERENCES care_groups(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('owner', 'billing_admin', 'family')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(care_group_id, user_id)
);

CREATE TABLE care_group_cared_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    care_group_id UUID NOT NULL REFERENCES care_groups(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(care_group_id, user_id)
);

ALTER TABLE subscriptions
    ADD COLUMN care_group_id UUID REFERENCES care_groups(id) ON DELETE CASCADE;

ALTER TABLE subscription_events
    ADD COLUMN care_group_id UUID REFERENCES care_groups(id) ON DELETE SET NULL;

CREATE INDEX idx_care_groups_owner ON care_groups(owner_user_id, status);
CREATE INDEX idx_care_group_members_user
    ON care_group_members(user_id, deleted_at);
CREATE INDEX idx_care_group_members_group
    ON care_group_members(care_group_id, deleted_at);
CREATE INDEX idx_care_group_cared_users_user
    ON care_group_cared_users(user_id, deleted_at);
CREATE INDEX idx_care_group_cared_users_group
    ON care_group_cared_users(care_group_id, deleted_at);
CREATE INDEX idx_subscriptions_care_group_status
    ON subscriptions(care_group_id, status, created_at DESC)
    WHERE care_group_id IS NOT NULL;
CREATE INDEX idx_subscription_events_care_group
    ON subscription_events(care_group_id, created_at DESC)
    WHERE care_group_id IS NOT NULL;

WITH user_groups AS (
    INSERT INTO care_groups (name, owner_user_id)
    SELECT 'AiSee Care Group', u.id
    FROM users u
    WHERE u.role = 'user'
    RETURNING id, owner_user_id
)
INSERT INTO care_group_cared_users (care_group_id, user_id)
SELECT id, owner_user_id
FROM user_groups
ON CONFLICT (care_group_id, user_id) DO NOTHING;

INSERT INTO care_group_members (care_group_id, user_id, role)
SELECT
    cg.id,
    fl.family_id,
    CASE WHEN fl.is_primary_contact THEN 'billing_admin' ELSE 'family' END
FROM care_groups cg
JOIN family_links fl ON fl.user_id = cg.owner_user_id
WHERE fl.deleted_at IS NULL
ON CONFLICT (care_group_id, user_id) DO UPDATE SET
    role = EXCLUDED.role,
    deleted_at = NULL;

WITH candidates AS (
    SELECT
        s.id AS subscription_id,
        cgu.care_group_id,
        0 AS priority
    FROM subscriptions s
    JOIN care_group_cared_users cgu
        ON cgu.user_id = s.user_id
       AND cgu.deleted_at IS NULL

    UNION ALL

    SELECT
        s.id AS subscription_id,
        cgm.care_group_id,
        CASE cgm.role
            WHEN 'owner' THEN 1
            WHEN 'billing_admin' THEN 2
            ELSE 3
        END AS priority
    FROM subscriptions s
    JOIN care_group_members cgm
        ON cgm.user_id = s.user_id
       AND cgm.deleted_at IS NULL
),
ranked AS (
    SELECT
        subscription_id,
        care_group_id,
        ROW_NUMBER() OVER (
            PARTITION BY subscription_id
            ORDER BY priority, care_group_id
        ) AS rn
    FROM candidates
)
UPDATE subscriptions s
SET care_group_id = ranked.care_group_id
FROM ranked
WHERE s.id = ranked.subscription_id
  AND ranked.rn = 1
  AND s.care_group_id IS NULL;

UPDATE subscription_events se
SET care_group_id = s.care_group_id
FROM subscriptions s
WHERE se.subscription_id = s.id
  AND se.care_group_id IS NULL
  AND s.care_group_id IS NOT NULL;

WITH candidates AS (
    SELECT
        se.id AS event_id,
        cgu.care_group_id,
        0 AS priority
    FROM subscription_events se
    JOIN care_group_cared_users cgu
        ON cgu.user_id = se.user_id
       AND cgu.deleted_at IS NULL
    WHERE se.care_group_id IS NULL

    UNION ALL

    SELECT
        se.id AS event_id,
        cgm.care_group_id,
        CASE cgm.role
            WHEN 'owner' THEN 1
            WHEN 'billing_admin' THEN 2
            ELSE 3
        END AS priority
    FROM subscription_events se
    JOIN care_group_members cgm
        ON cgm.user_id = se.user_id
       AND cgm.deleted_at IS NULL
    WHERE se.care_group_id IS NULL
),
ranked AS (
    SELECT
        event_id,
        care_group_id,
        ROW_NUMBER() OVER (
            PARTITION BY event_id
            ORDER BY priority, care_group_id
        ) AS rn
    FROM candidates
)
UPDATE subscription_events se
SET care_group_id = ranked.care_group_id
FROM ranked
WHERE se.id = ranked.event_id
  AND ranked.rn = 1
  AND se.care_group_id IS NULL;
