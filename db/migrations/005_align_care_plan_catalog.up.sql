-- db/migrations/005_align_care_plan_catalog.up.sql

INSERT INTO plans (code, name, price_vnd, billing_period, status, sort_order)
VALUES
    ('free', 'Free / Essential', 0, 'month', 'active', 0),
    ('basic_149k', 'Care Basic', 149000, 'month', 'active', 10),
    ('plus_399k', 'Care Plus', 399000, 'month', 'active', 20),
    ('premium_599k', 'Care Premium', 599000, 'month', 'active', 30)
ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name,
    price_vnd = EXCLUDED.price_vnd,
    billing_period = EXCLUDED.billing_period,
    status = EXCLUDED.status,
    sort_order = EXCLUDED.sort_order,
    updated_at = now();

DELETE FROM plan_entitlements
WHERE plan_id IN (
    SELECT id FROM plans
    WHERE code IN ('free', 'basic_149k', 'plus_399k', 'premium_599k')
);

WITH seed(code, feature_key, limit_value, limit_unit, reset_period, metadata) AS (
    VALUES
        ('free', 'sos_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb),
        ('free', 'medication_reminders_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb),
        ('free', 'read_messages_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb),
        ('free', 'ai_minutes_per_month', '50'::jsonb, 'minutes', 'month', '{}'::jsonb),
        ('free', 'camera_activations_per_day', '1'::jsonb, 'activations', 'day', '{}'::jsonb),
        ('free', 'video_calls_per_day', '0'::jsonb, 'calls', 'day', '{}'::jsonb),
        ('free', 'family_members_limit', '1'::jsonb, 'members', 'never', '{}'::jsonb),
        ('free', 'cared_users_limit', '1'::jsonb, 'users', 'never', '{}'::jsonb),
        ('free', 'care_timeline_days', '0'::jsonb, 'days', 'never', '{}'::jsonb),
        ('free', 'daily_brief_enabled', 'false'::jsonb, NULL, 'never', '{}'::jsonb),
        ('free', 'weekly_report_enabled', 'false'::jsonb, NULL, 'never', '{}'::jsonb),
        ('free', 'permission_matrix_level', '"none"'::jsonb, NULL, 'never', '{}'::jsonb),
        ('free', 'volunteer_priority_level', '"none"'::jsonb, NULL, 'never', '{}'::jsonb),
        ('free', 'pdf_exports_per_month', '0'::jsonb, 'exports', 'month', '{}'::jsonb),
        ('free', 'csv_exports_per_month', '0'::jsonb, 'exports', 'month', '{}'::jsonb),
        ('free', 'export_format', '"none"'::jsonb, NULL, 'never', '{}'::jsonb),
        ('free', 'priority_support_enabled', 'false'::jsonb, NULL, 'never', '{}'::jsonb),

        ('basic_149k', 'sos_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb),
        ('basic_149k', 'medication_reminders_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb),
        ('basic_149k', 'read_messages_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb),
        ('basic_149k', 'ai_minutes_per_month', '150'::jsonb, 'minutes', 'month', '{}'::jsonb),
        ('basic_149k', 'camera_activations_per_day', '3'::jsonb, 'activations', 'day', '{}'::jsonb),
        ('basic_149k', 'video_calls_per_day', '5'::jsonb, 'calls', 'day', '{}'::jsonb),
        ('basic_149k', 'family_members_limit', '2'::jsonb, 'members', 'never', '{}'::jsonb),
        ('basic_149k', 'cared_users_limit', '1'::jsonb, 'users', 'never', '{}'::jsonb),
        ('basic_149k', 'care_timeline_days', '7'::jsonb, 'days', 'never', '{}'::jsonb),
        ('basic_149k', 'daily_brief_enabled', 'false'::jsonb, NULL, 'never', '{}'::jsonb),
        ('basic_149k', 'weekly_report_enabled', 'false'::jsonb, NULL, 'never', '{}'::jsonb),
        ('basic_149k', 'permission_matrix_level', '"none"'::jsonb, NULL, 'never', '{}'::jsonb),
        ('basic_149k', 'volunteer_priority_level', '"standard"'::jsonb, NULL, 'never', '{}'::jsonb),
        ('basic_149k', 'pdf_exports_per_month', '0'::jsonb, 'exports', 'month', '{}'::jsonb),
        ('basic_149k', 'csv_exports_per_month', '0'::jsonb, 'exports', 'month', '{}'::jsonb),
        ('basic_149k', 'export_format', '"none"'::jsonb, NULL, 'never', '{}'::jsonb),
        ('basic_149k', 'priority_support_enabled', 'false'::jsonb, NULL, 'never', '{}'::jsonb),

        ('plus_399k', 'sos_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb),
        ('plus_399k', 'medication_reminders_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb),
        ('plus_399k', 'read_messages_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb),
        ('plus_399k', 'ai_minutes_per_month', '600'::jsonb, 'minutes', 'month', '{}'::jsonb),
        ('plus_399k', 'camera_activations_per_day', '20'::jsonb, 'activations', 'day', '{}'::jsonb),
        ('plus_399k', 'video_calls_per_day', '"fair_use"'::jsonb, 'policy', 'never', '{"description":"reasonable unlimited"}'::jsonb),
        ('plus_399k', 'family_members_limit', '5'::jsonb, 'members', 'never', '{}'::jsonb),
        ('plus_399k', 'cared_users_limit', '3'::jsonb, 'users', 'never', '{}'::jsonb),
        ('plus_399k', 'care_timeline_days', '90'::jsonb, 'days', 'never', '{}'::jsonb),
        ('plus_399k', 'daily_brief_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb),
        ('plus_399k', 'weekly_report_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb),
        ('plus_399k', 'permission_matrix_level', '"basic"'::jsonb, NULL, 'never', '{}'::jsonb),
        ('plus_399k', 'volunteer_priority_level', '"priority"'::jsonb, NULL, 'never', '{}'::jsonb),
        ('plus_399k', 'pdf_exports_per_month', '0'::jsonb, 'exports', 'month', '{}'::jsonb),
        ('plus_399k', 'csv_exports_per_month', '4'::jsonb, 'exports', 'month', '{}'::jsonb),
        ('plus_399k', 'export_format', '"csv"'::jsonb, NULL, 'never', '{}'::jsonb),
        ('plus_399k', 'priority_support_enabled', 'false'::jsonb, NULL, 'never', '{}'::jsonb),
        ('plus_399k', 'smart_alert_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb),

        ('premium_599k', 'sos_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb),
        ('premium_599k', 'medication_reminders_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb),
        ('premium_599k', 'read_messages_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb),
        ('premium_599k', 'ai_minutes_per_month', '"fair_use"'::jsonb, 'policy', 'never', '{"description":"near unlimited fair use"}'::jsonb),
        ('premium_599k', 'camera_activations_per_day', '"fair_use"'::jsonb, 'policy', 'never', '{"description":"reasonable unlimited"}'::jsonb),
        ('premium_599k', 'video_calls_per_day', '"fair_use"'::jsonb, 'policy', 'never', '{"description":"reasonable unlimited"}'::jsonb),
        ('premium_599k', 'family_members_limit', '10'::jsonb, 'members', 'never', '{}'::jsonb),
        ('premium_599k', 'cared_users_limit', '4'::jsonb, 'users', 'never', '{}'::jsonb),
        ('premium_599k', 'care_timeline_days', '365'::jsonb, 'days', 'never', '{}'::jsonb),
        ('premium_599k', 'daily_brief_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb),
        ('premium_599k', 'daily_brief_customization_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb),
        ('premium_599k', 'weekly_report_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb),
        ('premium_599k', 'weekly_report_pdf_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb),
        ('premium_599k', 'permission_matrix_level', '"advanced"'::jsonb, NULL, 'never', '{}'::jsonb),
        ('premium_599k', 'volunteer_priority_level', '"highest"'::jsonb, NULL, 'never', '{}'::jsonb),
        ('premium_599k', 'pdf_exports_per_month', '10'::jsonb, 'exports', 'month', '{}'::jsonb),
        ('premium_599k', 'csv_exports_per_month', '10'::jsonb, 'exports', 'month', '{}'::jsonb),
        ('premium_599k', 'export_format', '"pdf_csv"'::jsonb, NULL, 'never', '{}'::jsonb),
        ('premium_599k', 'priority_support_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb),
        ('premium_599k', 'incident_report_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb),
        ('premium_599k', 'smart_alert_enabled', 'true'::jsonb, NULL, 'never', '{}'::jsonb)
)
INSERT INTO plan_entitlements (
    plan_id,
    feature_key,
    limit_value,
    limit_unit,
    reset_period,
    metadata
)
SELECT
    p.id,
    seed.feature_key,
    seed.limit_value,
    seed.limit_unit,
    seed.reset_period,
    seed.metadata
FROM seed
JOIN plans p ON p.code = seed.code
ON CONFLICT (plan_id, feature_key) DO UPDATE SET
    limit_value = EXCLUDED.limit_value,
    limit_unit = EXCLUDED.limit_unit,
    reset_period = EXCLUDED.reset_period,
    metadata = EXCLUDED.metadata,
    updated_at = now();
