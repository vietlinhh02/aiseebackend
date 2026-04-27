# Quota Enforcement Design

## Goal

Make quota operational instead of display-only by recording trusted usage and blocking billable actions when the effective care-group plan is depleted.

## Architecture

Billing remains the source of truth. `billing.Service` exposes reusable quota primitives that calculate the effective care-group plan, aggregate usage across cared users, and return a typed `quota_exceeded` error before side effects run. One-shot actions record usage after success; duration actions start or commit usage sessions.

## Usage Mapping

- `ai_minutes`: LiveKit assistant sessions, measured in minutes.
- `camera_activations`: camera enable requests, measured as one activation.
- `voice_call_minutes`: voice calls, measured in minutes when call lifecycle data is available.
- `video_calls`: video call starts, measured as calls per day.
- `music_sessions`: successful music search/play dispatch, measured as sessions.
- `music_minutes`: reserved for future reliable playback duration commits.
- `navigation_sessions`: successful map launch requests, measured as sessions.
- `volunteer_matches`: volunteer match requests, measured as matches.
- `pdf_exports`, `care_coach_questions`, `pattern_detection_runs`: enforced wherever backend services expose those actions.

## Error Handling

Quota failures use code `quota_exceeded` and include `usage_type`, `remaining`, `required`, `plan_code`, and `reset_period` where available. The app and agent convert that stable code into upgrade prompts and `quota_exceeded` analytics.

## Testing

Billing service tests cover depleted care-group quota, successful one-shot usage recording, and `video_calls_per_day` mapping to `video_calls`. Tool/call/app tests cover integration points when the existing service boundaries make that practical.
