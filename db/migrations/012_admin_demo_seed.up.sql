-- 012_admin_demo_seed.up.sql
-- Demo data for the admin user detail and analytics screens.

WITH seed_guard AS (
    SELECT COALESCE(
        lower(current_setting('app.seed_demo_data', true)),
        ''
    ) IN ('1', 'true', 'on') AS enabled
),
demo_users (
    id,
    google_id,
    email,
    display_name,
    role,
    is_volunteer,
    avatar_url,
    created_at,
    updated_at
) AS (
    VALUES
        (
            '35c2b773-78c9-44cb-9b2d-7b5da977bd46'::uuid,
            'demo-google-nguyen-an',
            'nguyen.an.demo@aaisee.me',
            'Nguyễn An',
            'user',
            false,
            NULL,
            '2026-03-18 09:00:00+07'::timestamptz,
            '2026-04-29 09:30:00+07'::timestamptz
        ),
        (
            '2c2f8846-bd9a-4eb7-b895-7c82323db002'::uuid,
            'demo-google-lan-pham',
            'lan.pham.demo@aaisee.me',
            'Phạm Lan',
            'family',
            false,
            NULL,
            '2026-03-19 10:15:00+07'::timestamptz,
            '2026-04-29 08:15:00+07'::timestamptz
        ),
        (
            '90bb7db8-d730-4421-871e-97f8b20bb334'::uuid,
            'demo-google-minh-tran',
            'minh.tran.demo@aaisee.me',
            'Trần Minh',
            'user',
            true,
            NULL,
            '2026-04-03 14:20:00+07'::timestamptz,
            '2026-04-28 18:00:00+07'::timestamptz
        )
)
INSERT INTO users (
    id,
    google_id,
    email,
    display_name,
    role,
    is_volunteer,
    avatar_url,
    settings,
    created_at,
    updated_at
)
SELECT
    id,
    google_id,
    email,
    display_name,
    role,
    is_volunteer,
    avatar_url,
    '{"seed":"admin_demo"}'::jsonb,
    created_at,
    updated_at
FROM demo_users
CROSS JOIN seed_guard
WHERE seed_guard.enabled
ON CONFLICT (id) DO UPDATE SET
    settings = users.settings || '{"admin_demo_seed":true}'::jsonb,
    updated_at = GREATEST(users.updated_at, EXCLUDED.updated_at);

INSERT INTO family_links (
    id,
    user_id,
    family_id,
    relationship,
    is_primary_contact,
    created_at
)
SELECT
    seed.id,
    seed.user_id,
    seed.family_id,
    seed.relationship,
    seed.is_primary_contact,
    seed.created_at
FROM (
    VALUES (
        '2af6e255-76c1-4f12-8970-9460ef214b8a'::uuid,
        '35c2b773-78c9-44cb-9b2d-7b5da977bd46'::uuid,
        '2c2f8846-bd9a-4eb7-b895-7c82323db002'::uuid,
        'Con gái',
        true,
        '2026-03-19 10:30:00+07'::timestamptz
    )
) AS seed (
    id,
    user_id,
    family_id,
    relationship,
    is_primary_contact,
    created_at
)
CROSS JOIN (
    SELECT COALESCE(
        lower(current_setting('app.seed_demo_data', true)),
        ''
    ) IN ('1', 'true', 'on') AS enabled
) AS seed_guard
WHERE seed_guard.enabled
ON CONFLICT (user_id, family_id) DO UPDATE SET
    relationship = EXCLUDED.relationship,
    is_primary_contact = EXCLUDED.is_primary_contact,
    deleted_at = NULL;

INSERT INTO subscriptions (
    id,
    user_id,
    plan_id,
    status,
    current_period_start,
    current_period_end,
    cancel_at_period_end,
    provider,
    provider_subscription_id,
    metadata,
    created_at,
    updated_at
)
SELECT
    seed.id,
    seed.user_id,
    plans.id,
    seed.status,
    seed.current_period_start,
    seed.current_period_end,
    false,
    'revenuecat',
    seed.provider_subscription_id,
    '{"seed":"admin_demo"}'::jsonb,
    seed.created_at,
    seed.updated_at
FROM (
    VALUES
        (
            'b3a3dc2b-8463-4803-88fa-f018087da950'::uuid,
            '35c2b773-78c9-44cb-9b2d-7b5da977bd46'::uuid,
            'plus_399k',
            'active',
            '2026-04-01 00:00:00+07'::timestamptz,
            '2026-05-01 00:00:00+07'::timestamptz,
            'rc_demo_plus_nguyen_an',
            '2026-04-01 00:00:00+07'::timestamptz,
            '2026-04-29 09:00:00+07'::timestamptz
        ),
        (
            'fd6fef35-1a8a-4902-bc82-497683b25ad6'::uuid,
            '90bb7db8-d730-4421-871e-97f8b20bb334'::uuid,
            'premium_599k',
            'trialing',
            '2026-04-20 00:00:00+07'::timestamptz,
            '2026-05-04 00:00:00+07'::timestamptz,
            'rc_demo_premium_minh_tran',
            '2026-04-20 00:00:00+07'::timestamptz,
            '2026-04-28 18:00:00+07'::timestamptz
        )
) AS seed (
    id,
    user_id,
    plan_code,
    status,
    current_period_start,
    current_period_end,
    provider_subscription_id,
    created_at,
    updated_at
)
JOIN plans ON plans.code = seed.plan_code
CROSS JOIN (
    SELECT COALESCE(
        lower(current_setting('app.seed_demo_data', true)),
        ''
    ) IN ('1', 'true', 'on') AS enabled
) AS seed_guard
WHERE seed_guard.enabled
ON CONFLICT (id) DO UPDATE SET
    plan_id = EXCLUDED.plan_id,
    status = EXCLUDED.status,
    current_period_start = EXCLUDED.current_period_start,
    current_period_end = EXCLUDED.current_period_end,
    cancel_at_period_end = EXCLUDED.cancel_at_period_end,
    provider = EXCLUDED.provider,
    provider_subscription_id = EXCLUDED.provider_subscription_id,
    metadata = EXCLUDED.metadata,
    updated_at = EXCLUDED.updated_at;

INSERT INTO usage_ledger (
    id,
    user_id,
    usage_type,
    quantity,
    unit,
    source,
    source_id,
    started_at,
    ended_at,
    metadata
)
SELECT
    seed.id,
    seed.user_id,
    seed.usage_type,
    seed.quantity,
    seed.unit,
    seed.source,
    seed.source_id,
    seed.started_at,
    seed.ended_at,
    seed.metadata
FROM (
    VALUES
        ('0d72854e-2844-40cf-b29e-c2d1ed174501'::uuid, '35c2b773-78c9-44cb-9b2d-7b5da977bd46'::uuid, 'ai_minutes', 18.50, 'minutes', 'admin_demo_seed', 'an-ai-2026-04-24', '2026-04-24 08:10:00+07'::timestamptz, '2026-04-24 08:28:30+07'::timestamptz, '{"scenario":"morning_assistant"}'::jsonb),
        ('0d72854e-2844-40cf-b29e-c2d1ed174502'::uuid, '35c2b773-78c9-44cb-9b2d-7b5da977bd46'::uuid, 'camera_activations', 4.00, 'activations', 'admin_demo_seed', 'an-camera-2026-04-24', '2026-04-24 10:00:00+07'::timestamptz, '2026-04-24 10:00:05+07'::timestamptz, '{"scenario":"medicine_check"}'::jsonb),
        ('0d72854e-2844-40cf-b29e-c2d1ed174503'::uuid, '35c2b773-78c9-44cb-9b2d-7b5da977bd46'::uuid, 'video_call_minutes', 12.00, 'minutes', 'admin_demo_seed', 'an-video-2026-04-25', '2026-04-25 19:00:00+07'::timestamptz, '2026-04-25 19:12:00+07'::timestamptz, '{"scenario":"family_call"}'::jsonb),
        ('0d72854e-2844-40cf-b29e-c2d1ed174504'::uuid, '35c2b773-78c9-44cb-9b2d-7b5da977bd46'::uuid, 'ai_minutes', 24.75, 'minutes', 'admin_demo_seed', 'an-ai-2026-04-26', '2026-04-26 07:45:00+07'::timestamptz, '2026-04-26 08:09:45+07'::timestamptz, '{"scenario":"health_brief"}'::jsonb),
        ('0d72854e-2844-40cf-b29e-c2d1ed174505'::uuid, '35c2b773-78c9-44cb-9b2d-7b5da977bd46'::uuid, 'care_coach_questions', 7.00, 'questions', 'admin_demo_seed', 'an-coach-2026-04-27', '2026-04-27 16:30:00+07'::timestamptz, '2026-04-27 16:44:00+07'::timestamptz, '{"scenario":"caregiver_questions"}'::jsonb),
        ('0d72854e-2844-40cf-b29e-c2d1ed174506'::uuid, '35c2b773-78c9-44cb-9b2d-7b5da977bd46'::uuid, 'ai_minutes', 31.20, 'minutes', 'admin_demo_seed', 'an-ai-2026-04-28', '2026-04-28 20:00:00+07'::timestamptz, '2026-04-28 20:31:12+07'::timestamptz, '{"scenario":"evening_companion"}'::jsonb),
        ('0d72854e-2844-40cf-b29e-c2d1ed174507'::uuid, '90bb7db8-d730-4421-871e-97f8b20bb334'::uuid, 'ai_minutes', 11.40, 'minutes', 'admin_demo_seed', 'minh-ai-2026-04-28', '2026-04-28 09:00:00+07'::timestamptz, '2026-04-28 09:11:24+07'::timestamptz, '{"scenario":"volunteer_training"}'::jsonb),
        ('0d72854e-2844-40cf-b29e-c2d1ed174508'::uuid, '90bb7db8-d730-4421-871e-97f8b20bb334'::uuid, 'volunteer_matches', 3.00, 'matches', 'admin_demo_seed', 'minh-volunteer-2026-04-28', '2026-04-28 11:00:00+07'::timestamptz, '2026-04-28 11:20:00+07'::timestamptz, '{"scenario":"support_queue"}'::jsonb)
) AS seed (
    id,
    user_id,
    usage_type,
    quantity,
    unit,
    source,
    source_id,
    started_at,
    ended_at,
    metadata
)
CROSS JOIN (
    SELECT COALESCE(
        lower(current_setting('app.seed_demo_data', true)),
        ''
    ) IN ('1', 'true', 'on') AS enabled
) AS seed_guard
WHERE seed_guard.enabled
ON CONFLICT (id) DO UPDATE SET
    quantity = EXCLUDED.quantity,
    unit = EXCLUDED.unit,
    source = EXCLUDED.source,
    source_id = EXCLUDED.source_id,
    started_at = EXCLUDED.started_at,
    ended_at = EXCLUDED.ended_at,
    metadata = EXCLUDED.metadata;

INSERT INTO ai_cost_events (
    id,
    user_id,
    usage_ledger_id,
    provider,
    model,
    modality,
    source,
    source_id,
    input_tokens,
    output_tokens,
    audio_input_seconds,
    audio_output_seconds,
    image_count,
    video_seconds,
    cost_usd,
    cost_vnd,
    currency,
    estimated,
    pricing_version,
    occurred_at,
    metadata
)
SELECT
    seed.id,
    seed.user_id,
    seed.usage_ledger_id,
    seed.provider,
    seed.model,
    seed.modality,
    seed.source,
    seed.source_id,
    seed.input_tokens,
    seed.output_tokens,
    seed.audio_input_seconds,
    seed.audio_output_seconds,
    seed.image_count,
    seed.video_seconds,
    seed.cost_usd,
    seed.cost_vnd,
    seed.currency,
    seed.estimated,
    seed.pricing_version,
    seed.occurred_at,
    seed.metadata
FROM (
    VALUES
        ('7e4cf68f-5b90-422e-b8d2-48a919e6c001'::uuid, '35c2b773-78c9-44cb-9b2d-7b5da977bd46'::uuid, '0d72854e-2844-40cf-b29e-c2d1ed174501'::uuid, 'google', 'gemini-live-2.5-flash', 'live_audio', 'admin_demo_seed', 'an-ai-cost-2026-04-24', 18200, 6400, 1110.00, 260.00, 0, 0, 0.094500, 2394.00, 'USD', true, 'demo-2026-04', '2026-04-24 08:28:30+07'::timestamptz, '{"scenario":"morning_assistant"}'::jsonb),
        ('7e4cf68f-5b90-422e-b8d2-48a919e6c002'::uuid, '35c2b773-78c9-44cb-9b2d-7b5da977bd46'::uuid, '0d72854e-2844-40cf-b29e-c2d1ed174504'::uuid, 'google', 'gemini-2.5-flash', 'text', 'admin_demo_seed', 'an-ai-cost-2026-04-26', 9600, 3100, 0.00, 0.00, 0, 0, 0.031200, 790.40, 'USD', true, 'demo-2026-04', '2026-04-26 08:09:45+07'::timestamptz, '{"scenario":"health_brief"}'::jsonb),
        ('7e4cf68f-5b90-422e-b8d2-48a919e6c003'::uuid, '35c2b773-78c9-44cb-9b2d-7b5da977bd46'::uuid, '0d72854e-2844-40cf-b29e-c2d1ed174506'::uuid, 'google', 'gemini-live-2.5-flash', 'live_audio', 'admin_demo_seed', 'an-ai-cost-2026-04-28', 24100, 8800, 1872.00, 510.00, 0, 0, 0.141800, 3591.20, 'USD', true, 'demo-2026-04', '2026-04-28 20:31:12+07'::timestamptz, '{"scenario":"evening_companion"}'::jsonb),
        ('7e4cf68f-5b90-422e-b8d2-48a919e6c004'::uuid, '90bb7db8-d730-4421-871e-97f8b20bb334'::uuid, '0d72854e-2844-40cf-b29e-c2d1ed174507'::uuid, 'google', 'gemini-2.5-flash', 'text', 'admin_demo_seed', 'minh-ai-cost-2026-04-28', 5200, 1900, 0.00, 0.00, 0, 0, 0.016500, 417.60, 'USD', true, 'demo-2026-04', '2026-04-28 09:11:24+07'::timestamptz, '{"scenario":"volunteer_training"}'::jsonb)
) AS seed (
    id,
    user_id,
    usage_ledger_id,
    provider,
    model,
    modality,
    source,
    source_id,
    input_tokens,
    output_tokens,
    audio_input_seconds,
    audio_output_seconds,
    image_count,
    video_seconds,
    cost_usd,
    cost_vnd,
    currency,
    estimated,
    pricing_version,
    occurred_at,
    metadata
)
CROSS JOIN (
    SELECT COALESCE(
        lower(current_setting('app.seed_demo_data', true)),
        ''
    ) IN ('1', 'true', 'on') AS enabled
) AS seed_guard
WHERE seed_guard.enabled
ON CONFLICT (id) DO UPDATE SET
    usage_ledger_id = EXCLUDED.usage_ledger_id,
    provider = EXCLUDED.provider,
    model = EXCLUDED.model,
    modality = EXCLUDED.modality,
    source = EXCLUDED.source,
    source_id = EXCLUDED.source_id,
    input_tokens = EXCLUDED.input_tokens,
    output_tokens = EXCLUDED.output_tokens,
    audio_input_seconds = EXCLUDED.audio_input_seconds,
    audio_output_seconds = EXCLUDED.audio_output_seconds,
    image_count = EXCLUDED.image_count,
    video_seconds = EXCLUDED.video_seconds,
    cost_usd = EXCLUDED.cost_usd,
    cost_vnd = EXCLUDED.cost_vnd,
    currency = EXCLUDED.currency,
    estimated = EXCLUDED.estimated,
    pricing_version = EXCLUDED.pricing_version,
    occurred_at = EXCLUDED.occurred_at,
    metadata = EXCLUDED.metadata;
