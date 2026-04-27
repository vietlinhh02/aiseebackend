ALTER TABLE usage_sessions
    DROP CONSTRAINT IF EXISTS usage_sessions_usage_type_check,
    ADD CONSTRAINT usage_sessions_usage_type_check CHECK (
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
    );

ALTER TABLE usage_ledger
    DROP CONSTRAINT IF EXISTS usage_ledger_usage_type_check,
    ADD CONSTRAINT usage_ledger_usage_type_check CHECK (
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
    );

ALTER TABLE usage_quota_grants
    DROP CONSTRAINT IF EXISTS usage_quota_grants_usage_type_check,
    ADD CONSTRAINT usage_quota_grants_usage_type_check CHECK (
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
    );
