# AiSee Phase 3: Family Dashboard + Volunteer Matching

**Date:** 2026-03-28
**Author:** Eddie (Backend/AI Engineer)
**Status:** Approved

## Overview

Phase 3 adds family oversight and volunteer support to AiSee:

1. **Family Dashboard** — family members view and manage linked user's health data
2. **Volunteer Registry + Matching** — async volunteer discovery with push-based matching
3. **Family Medication Management** — family members CRUD medications on behalf of users
4. **Authorization** — verify family_links before granting dashboard access
5. **Location Tracking** — lightweight last-known location on users table

## Architecture

Approach: domain-separated packages following existing codebase patterns.

```
internal/
  dashboard/     # NEW — aggregation endpoints for family
    handler.go
    service.go
  volunteer/     # NEW — registry, matching, rating
    handler.go
    service.go
  middleware/
    auth.go      # MODIFIED — add AuthorizeFamily helper
  emergency/
    service.go   # MODIFIED — update location on SOS
  toolcall/
    handler.go   # MODIFIED — add escalate_to_volunteer, rate_volunteer tools
cmd/server/
  main.go        # MODIFIED — register new routes
db/
  migrations/
    003_phase3.up.sql    # NEW
    003_phase3.down.sql  # NEW
  queries/
    volunteers.sql       # NEW
    dashboard.sql        # NEW
```

## 1. Database Schema Changes

### Migration 003

**Alter `volunteers` table** (already exists in 001_init with `rating DEFAULT 5.0` and no `created_at`):

```sql
ALTER TABLE volunteers
    ALTER COLUMN rating SET NOT NULL,
    ALTER COLUMN rating SET DEFAULT 0,
    ALTER COLUMN available SET NOT NULL,
    ALTER COLUMN total_calls SET NOT NULL,
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Reset existing rating default from 5.0 to 0 for unrated volunteers
UPDATE volunteers SET rating = 0 WHERE total_calls = 0;

CREATE INDEX idx_volunteers_available ON volunteers (rating DESC, total_calls ASC)
    WHERE available = true;
```

**Create `volunteer_ratings` table:**

```sql
CREATE TABLE volunteer_ratings (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    volunteer_id UUID NOT NULL REFERENCES volunteers(user_id),
    rater_id     UUID NOT NULL REFERENCES users(id),
    match_id     TEXT NOT NULL,
    score        SMALLINT NOT NULL CHECK (score BETWEEN 1 AND 5),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(match_id, rater_id)
);

CREATE INDEX idx_volunteer_ratings_volunteer ON volunteer_ratings (volunteer_id);
```

**Add location columns to `users`:**

```sql
ALTER TABLE users
    ADD COLUMN last_lat         DOUBLE PRECISION,
    ADD COLUMN last_lng         DOUBLE PRECISION,
    ADD COLUMN last_location_at TIMESTAMPTZ;
```

## 2. Authorization

**`AuthorizeFamily` helper in `internal/middleware/auth.go`:**

- Takes `ctx` + `targetUserID`
- Extracts `requesterID` from JWT context
- Checks `RoleFromContext(ctx) == "family"` — only family role can access dashboard
- Calls existing `CheckFamilyLink(requesterID, targetUserID)` query
- Returns `nil` if role is family AND link exists, `403 Forbidden` otherwise

**Role constraint:** Only users with `role='family'` can access dashboard endpoints. A visually impaired user (`role='user'`) cannot view another user's dashboard even if a family link exists. This is intentional — dashboard is a caregiver feature.

Used at service level (not middleware) because `targetUserID` comes from URL params.

```
Request → Auth middleware (JWT) → Handler → AuthorizeFamily(ctx, targetUserID) → Service call
                                              ↓ check role == "family"
                                              ↓ check family_links exists
                                              ↓ 403 if either fails
```

## 3. Family Dashboard Endpoints

**Package:** `internal/dashboard/`

Dashboard service is an aggregation layer — no business logic, just:
1. Call `AuthorizeFamily()`
2. Call existing services (medication, health)
3. Write audit log for mutations
4. Compose response

### Read Endpoints

| Endpoint | Response |
|----------|----------|
| `GET /api/v1/dashboard/:user_id/health` | Active medications + today's adherence + 10 recent symptoms + 10 recent metrics |
| `GET /api/v1/dashboard/:user_id/medications` | Full medication list + today's taken/skipped log per medication |
| `GET /api/v1/dashboard/:user_id/audit-log` | Paginated audit logs (`?page=1&limit=20`), newest first |
| `GET /api/v1/dashboard/:user_id/location` | `last_lat`, `last_lng`, `last_location_at` from users table |

### Mutation Endpoints (Family Medication Management)

| Endpoint | Description | Audit |
|----------|-------------|-------|
| `POST /api/v1/dashboard/:user_id/medications` | Create medication for user | `family_create_medication` |
| `PUT /api/v1/dashboard/:user_id/medications/:med_id` | Update medication | `family_update_medication` |
| `DELETE /api/v1/dashboard/:user_id/medications/:med_id` | Soft-delete medication | `family_delete_medication` |

All mutations write to `audit_logs` with `tool_name = "family_manage_medication"`, including `family_id` in params.

### New SQL Queries (`db/queries/dashboard.sql`)

```sql
-- name: GetMedicationLogsToday :many
-- Get all medication logs for today for all of a user's medications
SELECT ml.* FROM medication_logs ml
JOIN medications m ON m.id = ml.medication_id
WHERE m.user_id = $1
  AND ml.taken_at >= (now() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date
  AND ml.taken_at < (now() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date + INTERVAL '1 day';

-- name: GetRecentSymptoms :many
SELECT * FROM health_symptoms
WHERE user_id = $1
ORDER BY logged_at DESC
LIMIT $2;

-- name: GetRecentMetrics :many
SELECT * FROM health_metrics
WHERE user_id = $1
ORDER BY logged_at DESC
LIMIT $2;

-- name: UpdateUserLocation :exec
UPDATE users SET last_lat = $2, last_lng = $3, last_location_at = now()
WHERE id = $1;

-- name: GetUserLocation :one
SELECT last_lat, last_lng, last_location_at FROM users WHERE id = $1;
```

## 4. Volunteer Registry + Matching

**Package:** `internal/volunteer/`

### REST Endpoints

| Endpoint | Description |
|----------|-------------|
| `POST /api/v1/volunteers/register` | Register as volunteer, create row in `volunteers` |
| `PUT /api/v1/volunteers/availability` | Toggle `available` boolean |
| `POST /api/v1/volunteers/match` | Start async matching, return `{match_id, status}` immediately |
| `GET /api/v1/volunteers/match/:match_id/status` | Poll: `searching` / `matched` / `failed` |
| `POST /api/v1/volunteers/rate` | Rate volunteer 1-5 stars (REST for app UI — user + family) |

### Async Matching Flow

```
POST /match
  → Generate match_id (UUID)
  → Redis HSET match:{match_id} status "searching" EX 300
  → Spawn background goroutine:

    1. Query: available=true, != requesting_user, not in requester's family
       ORDER BY rating DESC, total_calls ASC LIMIT 5
    2. For each candidate (up to 5):
       a. Create LiveKit room "vol-{match_id[:8]}"
       b. Send push notification to candidate
       c. Poll LiveKit room participant count for 30s
       d. If candidate joins:
          → Redis HSET: status="matched", volunteer_id, room_name, room_token
          → BREAK
       e. If timeout → try next candidate
    3. All candidates exhausted:
       → Redis HSET: status="failed"
```

**State stored in Redis** with 5-minute TTL. Match data:
- `match:{match_id}` hash: `status`, `volunteer_id`, `room_name`, `user_token`, `volunteer_name`

### Rating Flow

After a call ends:
- **Voice flow:** Agent asks "bạn đánh giá tình nguyện viên mấy sao?" → calls `rate_volunteer` tool
- **App UI:** Family/user taps rating popup → calls `POST /api/v1/volunteers/rate`

Both write to `volunteer_ratings` table, then recalculate `volunteers.rating` as average of all ratings.

`UNIQUE(match_id, rater_id)` allows both user and family member to rate the same match, but each person only once.

**`total_calls` increment:** Called once per match when status transitions to `"matched"` (inside the matching goroutine), NOT on rating. `UpdateVolunteerStats` is split into two separate queries: `IncrementVolunteerCalls` (on match) and `RecalcVolunteerRating` (on rating).

### New SQL Queries (`db/queries/volunteers.sql`)

```sql
-- name: CreateVolunteer :one
INSERT INTO volunteers (user_id, languages)
VALUES ($1, $2)
RETURNING *;

-- name: GetVolunteerByUserID :one
SELECT * FROM volunteers WHERE user_id = $1;

-- name: UpdateVolunteerAvailability :exec
UPDATE volunteers SET available = $1 WHERE user_id = $2;

-- name: GetAvailableVolunteers :many
SELECT v.*, u.display_name, u.fcm_token, u.apns_token
FROM volunteers v
JOIN users u ON u.id = v.user_id
WHERE v.available = true
  AND v.user_id != $1
  AND v.user_id NOT IN (
    SELECT family_id FROM family_links WHERE user_id = $1 AND deleted_at IS NULL
    UNION
    SELECT user_id FROM family_links WHERE family_id = $1 AND deleted_at IS NULL
  )
ORDER BY v.rating DESC, v.total_calls ASC
LIMIT 5;

-- name: CreateVolunteerRating :exec
INSERT INTO volunteer_ratings (volunteer_id, rater_id, match_id, score)
VALUES ($1, $2, $3, $4)
ON CONFLICT (match_id, rater_id) DO NOTHING;

-- name: IncrementVolunteerCalls :exec
-- Called once per match when volunteer joins the room
UPDATE volunteers SET total_calls = total_calls + 1 WHERE user_id = $1;

-- name: RecalcVolunteerRating :exec
-- Called after each rating submission
UPDATE volunteers SET
    rating = COALESCE((SELECT AVG(score)::REAL FROM volunteer_ratings WHERE volunteer_id = $1), 0)
WHERE user_id = $1;
```

## 5. Location Tracking

Lightweight approach: 3 columns on `users` table (`last_lat`, `last_lng`, `last_location_at`).

**Updated from 2 sources:**

1. **Emergency alert:** `emergency.Alert()` already receives `lat, lng` — add `UpdateUserLocation()` call after broadcast
2. **Future: Agent heartbeat** — `update_location` tool for when mobile team implements background location

## 6. Tool Handler Changes

Add to `internal/toolcall/handler.go`:

| Tool | Params | Action |
|------|--------|--------|
| `escalate_to_volunteer` | `user_id` | Call volunteer matching service, return `match_id` for polling |
| `rate_volunteer` | `match_id`, `score` (1-5) | Write rating, update volunteer stats |
| `update_location` | `lat`, `lng` | Call `UpdateUserLocation` |

## 7. Route Registration

Add to `cmd/server/main.go`:

```go
// Dashboard (family) — requires JWT auth
r.Route("/api/v1/dashboard/{user_id}", func(r chi.Router) {
    r.Use(middleware.Auth(jwtSecret))
    r.Get("/health", dashboardHandler.Health)
    r.Get("/medications", dashboardHandler.Medications)
    r.Get("/audit-log", dashboardHandler.AuditLog)
    r.Get("/location", dashboardHandler.Location)
    r.Post("/medications", dashboardHandler.CreateMedication)
    r.Put("/medications/{med_id}", dashboardHandler.UpdateMedication)
    r.Delete("/medications/{med_id}", dashboardHandler.DeleteMedication)
})

// Volunteers — requires JWT auth
r.Route("/api/v1/volunteers", func(r chi.Router) {
    r.Use(middleware.Auth(jwtSecret))
    r.Post("/register", volunteerHandler.Register)
    r.Put("/availability", volunteerHandler.SetAvailability)
    r.Post("/match", volunteerHandler.StartMatch)
    r.Get("/match/{match_id}/status", volunteerHandler.MatchStatus)
    r.Post("/rate", volunteerHandler.Rate)
})
```

## 8. Testing Strategy

- **Unit tests** for each new service (dashboard, volunteer) with mocked dependencies
- **AuthorizeFamily** tested with valid link, no link, deleted link
- **Volunteer matching** tested with mock Redis + mock LiveKit client
- **Rating** tested for duplicate prevention (UNIQUE constraint)
- **Integration** of dashboard endpoints with real DB queries
