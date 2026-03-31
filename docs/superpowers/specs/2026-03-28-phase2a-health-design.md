# Phase 2a: Health Tools — Technical Design Spec

**Date:** 2026-03-28
**Team:** Dang | VentureX Season 3 | VinUniversity ELAB
**Status:** Draft
**Depends on:** Phase 1 (completed)
**Spec:** Sections 4.3, 6 of `docs/superpowers/specs/2026-03-28-aisee-mvp-design.md`

---

## 1. Goal

Add health management tools to the AI agent: medication reminders, health logging, and appointment scheduling. After Phase 2a, a user can:

- Ask the AI agent to set medication reminders (with backend push + app local notifications)
- List today's medications and mark them as taken/skipped
- Log health symptoms and metrics by voice
- Set appointment reminders (with push notifications before the appointment)
- Receive push notifications for medication and appointment reminders even when app is closed

## 2. Scope

### In scope (backend only — this repo)

- `internal/medication/` package: CRUD, fuzzy name matching, taken/skipped logging
- `internal/health/` package: symptom and metric logging with predefined categories
- `internal/appointment/` package: CRUD, soft delete
- `pkg/reminder/` package: shared Redis-based scheduler for medication + appointment reminders
- Redis integration: sorted sets for reminder scheduling
- Push notifications for reminders (reuse existing `pkg/push/`)
- Data channel `schedule_reminder` action for app local notifications
- Tool-call router: 7 new tool routes
- Agent worker: 7 new tool functions
- New sqlc queries for medications, health, appointments
- Migration: replace `reminded` column with `reminded_1h` + `reminded_15m` on appointments

### Out of scope (deferred)

- Family dashboard health endpoints (Phase 3)
- Health summary/aggregation endpoints (Phase 3)
- Medication interaction warnings
- Health data export

## 3. Architecture

```
Go Backend
  pkg/reminder/         <- Shared Redis-based scheduler
  internal/medication/  <- Medication CRUD + name matching + logging
  internal/health/      <- Symptom + metric logging (predefined categories)
  internal/appointment/ <- Appointment CRUD + soft delete
  db/queries/           <- New sqlc queries (medications, health, appointments)

  cmd/server/main.go    <- Wire Redis, 3 services, scheduler, toolcall deps

Agent Worker (Python)
  tools.py              <- 6 new tool functions (all via BackendClient)
  backend.py            <- _call_and_forward helper for schedule_action
```

## 4. Database Queries

### 4.1 `db/queries/medications.sql`

```sql
-- name: CreateMedication :one
INSERT INTO medications (user_id, name, dosage, times, frequency)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetMedicationsByUser :many
SELECT * FROM medications
WHERE user_id = $1 AND active = true
ORDER BY name ASC;

-- name: GetMedicationByID :one
SELECT * FROM medications
WHERE id = $1 AND user_id = $2;

-- name: GetMedicationByName :many
SELECT * FROM medications
WHERE user_id = $1 AND active = true AND lower(name) = lower($2);

-- name: UpdateMedication :one
UPDATE medications
SET name = $3, dosage = $4, times = $5, frequency = $6
WHERE id = $1 AND user_id = $2 AND active = true
RETURNING *;

-- name: DeactivateMedication :exec
UPDATE medications
SET active = false, deleted_at = now()
WHERE id = $1 AND user_id = $2;

-- name: GetActiveMedicationsAll :many
SELECT * FROM medications WHERE active = true;

-- name: CreateMedicationLog :one
INSERT INTO medication_logs (medication_id, skipped)
VALUES ($1, $2)
RETURNING *;

-- name: GetMedicationLogsByDate :many
SELECT ml.* FROM medication_logs ml
JOIN medications m ON ml.medication_id = m.id
WHERE m.user_id = $1
  AND ml.taken_at >= $2::timestamptz
  AND ml.taken_at < $3::timestamptz
ORDER BY ml.taken_at ASC;

-- name: GetMedicationLogToday :many
SELECT ml.* FROM medication_logs ml
WHERE ml.medication_id = $1
  AND ml.taken_at >= (now() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date::timestamptz
  AND ml.taken_at < ((now() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date + INTERVAL '1 day')::timestamptz;
```

### 4.2 `db/queries/health.sql`

```sql
-- name: CreateHealthSymptom :one
INSERT INTO health_symptoms (user_id, symptom, severity, notes)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetHealthSymptomsByUser :many
SELECT * FROM health_symptoms
WHERE user_id = $1
ORDER BY logged_at DESC
LIMIT $2 OFFSET $3;

-- name: CreateHealthMetric :one
INSERT INTO health_metrics (user_id, metric_type, value, unit)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetHealthMetricsByUser :many
SELECT * FROM health_metrics
WHERE user_id = $1
ORDER BY logged_at DESC
LIMIT $2 OFFSET $3;

-- name: GetHealthMetricsByType :many
SELECT * FROM health_metrics
WHERE user_id = $1 AND metric_type = $2
ORDER BY logged_at DESC
LIMIT $3 OFFSET $4;
```

### 4.3 `db/queries/appointments.sql`

```sql
-- name: CreateAppointment :one
INSERT INTO appointments (user_id, title, location, datetime, notes)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetAppointmentsByUser :many
SELECT * FROM appointments
WHERE user_id = $1 AND deleted_at IS NULL AND datetime >= now()
ORDER BY datetime ASC;

-- name: GetAppointmentByID :one
SELECT * FROM appointments
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: UpdateAppointment :one
UPDATE appointments
SET title = $3, location = $4, datetime = $5, notes = $6
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: DeleteAppointment :exec
UPDATE appointments SET deleted_at = now()
WHERE id = $1 AND user_id = $2;

-- name: GetUpcomingAppointmentsAll :many
SELECT * FROM appointments
WHERE deleted_at IS NULL
  AND datetime >= now()
  AND datetime <= now() + INTERVAL '2 hours';

-- name: MarkAppointmentReminded1h :exec
UPDATE appointments SET reminded_1h = true WHERE id = $1;

-- name: MarkAppointmentReminded15m :exec
UPDATE appointments SET reminded_15m = true WHERE id = $1;
```

## 5. Migration

New file `db/migrations/002_appointment_reminders.up.sql`:

```sql
ALTER TABLE appointments DROP COLUMN reminded;
ALTER TABLE appointments ADD COLUMN reminded_1h BOOLEAN DEFAULT false;
ALTER TABLE appointments ADD COLUMN reminded_15m BOOLEAN DEFAULT false;
```

Down migration `db/migrations/002_appointment_reminders.down.sql`:

```sql
ALTER TABLE appointments DROP COLUMN IF EXISTS reminded_1h;
ALTER TABLE appointments DROP COLUMN IF EXISTS reminded_15m;
ALTER TABLE appointments ADD COLUMN reminded BOOLEAN DEFAULT false;
```

## 6. Package Design

### 6.1 `internal/medication/`

**File structure:**

```
internal/medication/
├── service.go       # Business logic: CRUD, fuzzy name match, taken/skipped
└── service_test.go  # Tests
```

**Service:**

```go
type Service struct {
    db        *dbgen.Queries
    scheduler *reminder.Scheduler
    pusher    *push.Pusher
}

func (s *Service) Create(ctx, userID, name, dosage, times[], frequency) (*dbgen.Medication, error)
func (s *Service) List(ctx, userID) ([]dbgen.Medication, error)
func (s *Service) FindByName(ctx, userID, name) (*dbgen.Medication, error)
func (s *Service) MarkTaken(ctx, userID, medicationName) (*dbgen.MedicationLog, error)
func (s *Service) MarkSkipped(ctx, userID, medicationName) (*dbgen.MedicationLog, error)
func (s *Service) Deactivate(ctx, userID, medicationName) error
func (s *Service) ListWithStatus(ctx, userID) ([]MedicationWithStatus, error)
```

**FindByName logic** (uses `golang.org/x/text/transform` for Vietnamese normalization, same as `user.ResolveContact`):
1. Query `GetMedicationByName` (exact match, case-insensitive)
2. If 0 results: load all active medications via `GetMedicationsByUser`, normalize both input and each medication name (lowercase + NFD decomposition + strip combining marks), then substring match
3. If 1 match: return
4. If 0 matches: error "medication not found"
5. If multiple: error "multiple medications match"

**MedicationWithStatus** (for `list_medication_reminders` tool):

```go
type MedicationWithStatus struct {
    dbgen.Medication
    TodayTaken bool
}
```

**ReminderHandler implementation:**

```go
func (s *Service) LoadAll(ctx) ([]reminder.Reminder, error)
func (s *Service) OnDue(ctx, memberKey string) error
```

`LoadAll`: query all active medications, for each medication and each time slot, compute next occurrence and return as Reminder.

`OnDue` flow:
1. Parse medication_id and time_index from memberKey
2. Query medication by ID to get user_id, name, dosage
3. Query `GetMedicationLogToday` to check if already taken today
4. If already taken: skip push, reschedule for tomorrow
5. If not taken: query `GetUserByID` to get push tokens → send push notification via `pusher.Send()`
6. Reschedule: compute next occurrence and call `scheduler.Schedule()`

**Frequency handling (MVP):** Only `"daily"` is fully supported. Next occurrence = same time tomorrow. For non-daily frequencies (`"every_other_day"`, `"weekly"`), the `Create` method stores the frequency value but the scheduler treats all as daily. Non-daily scheduling is deferred to a future iteration.

### 6.2 `internal/health/`

**File structure:**

```
internal/health/
├── service.go       # Symptom + metric logging with category validation
├── service_test.go  # Tests
└── categories.go    # Predefined symptom and metric categories
```

**Predefined categories (`categories.go`):**

```go
var SymptomCategories = []string{
    "dau_dau",      // đau đầu
    "chong_mat",    // chóng mặt
    "met_moi",      // mệt mỏi
    "kho_tho",      // khó thở
    "buon_non",     // buồn nôn
    "dau_nguc",     // đau ngực
    "sot",          // sốt
    "ho",           // ho
    "dau_bung",     // đau bụng
    "mo_mat",       // mờ mắt
}

var MetricTypes = map[string]string{
    "huyet_ap":    "mmHg",    // huyết áp
    "duong_huyet": "mg/dL",   // đường huyết
    "nhiet_do":    "°C",      // nhiệt độ
    "can_nang":    "kg",      // cân nặng
    "nhip_tim":    "bpm",     // nhịp tim
    "spo2":        "%",       // SpO2
}
```

**Validation logic:**
1. Normalize input (lowercase, strip Vietnamese diacritics)
2. Match against predefined categories/types
3. If match: use canonical name + default unit
4. If no match: store as-is (free-text fallback)

**Service:**

```go
type Service struct {
    db *dbgen.Queries
}

func (s *Service) LogSymptom(ctx, userID, symptom, severity, notes) (*dbgen.HealthSymptom, error)
func (s *Service) LogMetric(ctx, userID, metricType, value, unit) (*dbgen.HealthMetric, error)
func (s *Service) GetSymptoms(ctx, userID, limit, offset) ([]dbgen.HealthSymptom, error)
func (s *Service) GetMetrics(ctx, userID, metricType, limit, offset) ([]dbgen.HealthMetric, error)
```

### 6.3 `internal/appointment/`

**File structure:**

```
internal/appointment/
├── service.go       # CRUD + reminder handler
└── service_test.go  # Tests
```

**Service:**

```go
type Service struct {
    db        *dbgen.Queries
    scheduler *reminder.Scheduler
    pusher    *push.Pusher
}

func (s *Service) Create(ctx, userID, title, location, datetime, notes) (*dbgen.Appointment, error)
func (s *Service) List(ctx, userID) ([]dbgen.Appointment, error)
func (s *Service) Update(ctx, userID, appointmentID, ...) (*dbgen.Appointment, error)
func (s *Service) Delete(ctx, userID, appointmentID) error
```

`Create` calls `scheduler.Schedule()` to add 1h and 15m reminders after DB insert.
`Update` calls `scheduler.Unschedule()` then `scheduler.Schedule()` with new datetime.
`Delete` calls `scheduler.Unschedule()` to remove pending reminders.

**ReminderHandler implementation:**

```go
func (s *Service) LoadAll(ctx) ([]reminder.Reminder, error)
func (s *Service) OnDue(ctx, memberKey string) error
```

`LoadAll`: query upcoming appointments, for each generate 2 reminders (1h before, 15m before) if not already reminded.

`OnDue` flow:
1. Parse appointment_id and type (1h/15m) from memberKey
2. Query appointment by ID to get user_id, title, location, datetime
3. Query `GetUserByID` to get push tokens
4. Send push notification via `pusher.Send()`
5. Call `MarkAppointmentReminded1h` or `MarkAppointmentReminded15m`

### 6.4 `pkg/reminder/`

**File structure:**

```
pkg/reminder/
├── scheduler.go      # Generic Redis-based scheduler
└── scheduler_test.go # Tests
```

**Interface:**

```go
type Reminder struct {
    Key   string    // e.g. "med-uuid:0" or "apt-uuid:1h"
    DueAt time.Time // when to fire
}

type Handler interface {
    LoadAll(ctx context.Context) ([]Reminder, error)
    OnDue(ctx context.Context, key string) error
}

type Scheduler struct {
    redis    *redis.Client
    handlers map[string]Handler // "medications" -> medication.Service, "appointments" -> appointment.Service
    interval time.Duration      // 30s
}

func New(redis *redis.Client, interval time.Duration) *Scheduler
func (s *Scheduler) Register(name string, h Handler)
func (s *Scheduler) Start(ctx context.Context) error  // blocks, call in goroutine
func (s *Scheduler) Stop()
func (s *Scheduler) Schedule(ctx context.Context, name string, r Reminder) error    // add/update reminder
func (s *Scheduler) Unschedule(ctx context.Context, name string, key string) error  // remove reminder
```

**Redis keys:**

```
reminders:medications    — sorted set (score=unix timestamp, member=key)
reminders:appointments   — sorted set (score=unix timestamp, member=key)
```

**Tick loop:**

```
every 30s:
    for each registered handler (name, handler):
        members = ZRANGEBYSCORE reminders:{name} -inf {now}
        for each member:
            ZREM reminders:{name} member
            handler.OnDue(ctx, member)
```

**Startup:**

```
for each handler:
    reminders = handler.LoadAll(ctx)
    for each reminder:
        ZADD reminders:{name} {score=reminder.DueAt.Unix()} {member=reminder.Key}
```

## 7. Tool-Call Router Updates

### 7.1 New handler methods

```go
func (h *Handler) handleSetMedication(ctx, req) (any, error)
func (h *Handler) handleListMedications(ctx, req) (any, error)
func (h *Handler) handleMarkMedicationTaken(ctx, req) (any, error)
func (h *Handler) handleMarkMedicationSkipped(ctx, req) (any, error)
func (h *Handler) handleLogSymptom(ctx, req) (any, error)
func (h *Handler) handleLogMetric(ctx, req) (any, error)
func (h *Handler) handleSetAppointment(ctx, req) (any, error)
```

### 7.2 Tool routing additions

| Tool name | Handler method | Service |
|-----------|---------------|---------|
| `set_medication_reminder` | handleSetMedication | medication.Create (calls scheduler.Schedule internally) |
| `list_medication_reminders` | handleListMedications | medication.ListWithStatus |
| `mark_medication_taken` | handleMarkMedicationTaken | medication.MarkTaken |
| `mark_medication_skipped` | handleMarkMedicationSkipped | medication.MarkSkipped |
| `log_health_symptom` | handleLogSymptom | health.LogSymptom |
| `log_health_metric` | handleLogMetric | health.LogMetric |
| `set_appointment_reminder` | handleSetAppointment | appointment.Create (calls scheduler.Schedule internally) |

### 7.3 Schedule action in response

Tools that create reminders include a `schedule_action` field in the response:

```json
{
  "status": "created",
  "medication": {"name": "Amlodipine", "times": ["07:00", "21:00"]},
  "schedule_action": {
    "action": "schedule_reminder",
    "params": {
      "type": "medication",
      "name": "Amlodipine",
      "times": ["07:00", "21:00"],
      "frequency": "daily"
    }
  }
}
```

Agent worker detects `schedule_action` and forwards it to app via data channel.

### 7.4 Handler dependencies update

```go
type Handler struct {
    chat        *chat.Service
    call        *call.Service
    user        *user.Service
    medication  *medication.Service
    health      *health.Service
    appointment *appointment.Service
    db          *dbgen.Queries
}
```

Note: `push` and `lk` are NOT added to the toolcall handler. Push notifications for reminders are handled by the service layer (medication.Service, appointment.Service) via their `pusher` field. The toolcall handler only dispatches to service methods.

## 8. Agent Worker Updates

### 8.1 New tool functions (`tools.py`)

```python
@function_tool()
async def set_medication_reminder(
    medicine_name: str, dosage: str,
    times: list[str], frequency: str = "daily",
):
    """Dat nhac nho uong thuoc."""

@function_tool()
async def list_medication_reminders():
    """Liet ke thuoc hom nay va trang thai da uong."""

@function_tool()
async def mark_medication_taken(medicine_name: str):
    """Xac nhan da uong thuoc."""

@function_tool()
async def mark_medication_skipped(medicine_name: str):
    """Xac nhan bo qua thuoc hom nay."""

@function_tool()
async def log_health_symptom(
    symptom: str, severity: str = "", notes: str = "",
):
    """Ghi nhan trieu chung suc khoe."""

@function_tool()
async def log_health_metric(
    metric_type: str, value: str, unit: str = "",
):
    """Ghi nhan chi so suc khoe."""

@function_tool()
async def set_appointment_reminder(
    title: str, datetime: str, location: str = "", notes: str = "",
):
    """Dat nhac nho lich hen bac si."""
```

All use `_call_and_forward` helper.

### 8.2 `_call_and_forward` helper (`tools.py`)

```python
async def _call_and_forward(tool_name: str, params: dict) -> dict:
    result = await _backend.tool_call(tool_name, _user_id, params)
    if "schedule_action" in result:
        action = result.pop("schedule_action")
        # Fire-and-forget: schedule_reminder is best-effort.
        # Backend push is the primary reminder mechanism.
        # App local notification is a bonus — don't fail the tool
        # if app is not connected or slow to ACK.
        try:
            await _bridge.send_action(
                _local_participant, action["action"], action["params"],
            )
        except Exception:
            pass  # Backend push will still fire
    return result
```

### 8.3 Tool registration (`main.py`)

Add 6 new tools to the session tools list alongside existing 6:

```python
session.tools = [
    # Phase 1 (existing)
    make_voice_call, make_video_call, send_chat_message,
    read_unread_messages, activate_camera, deactivate_camera,
    # Phase 2a (new)
    set_medication_reminder, list_medication_reminders,
    mark_medication_taken, mark_medication_skipped,
    log_health_symptom, log_health_metric,
    set_appointment_reminder,
]
```

## 9. Push Notification Formats

**Medication reminder:**

```
Title: "Nhac uong thuoc"
Body: "Den gio uong [name] [dosage]"
Type: "normal"
Data: {"type": "medication_reminder", "medication_id": "<uuid>"}
```

**Appointment reminder (1h):**

```
Title: "Lich hen sap toi"
Body: "[title] luc [time] tai [location] (con 1 gio)"
Type: "normal"
Data: {"type": "appointment_reminder", "appointment_id": "<uuid>", "advance": "1h"}
```

**Appointment reminder (15m):**

```
Title: "Lich hen sap toi"
Body: "[title] luc [time] tai [location] (con 15 phut)"
Type: "normal"
Data: {"type": "appointment_reminder", "appointment_id": "<uuid>", "advance": "15m"}
```

## 10. New Dependencies

### Go

| Package | Purpose |
|---------|---------|
| `github.com/redis/go-redis/v9` | Redis client for reminder scheduler |

### Python

No new dependencies. `httpx` already available.

## 11. Environment Variables

### Existing (unchanged)

```
REDIS_URL=redis://localhost:6379/0
```

Redis is already in docker-compose.yml but not yet used by Go backend. Phase 2a is the first consumer.

## 12. Implementation Order

1. Install `go-redis/v9` dependency
2. Migration `002_appointment_reminders` + new sqlc queries + `sqlc generate`
3. `pkg/reminder/` — shared Redis scheduler
4. `internal/health/` — symptom + metric service (simplest, no Redis)
5. `internal/medication/` — CRUD + name matching + ReminderHandler
6. `internal/appointment/` — CRUD + ReminderHandler
7. `internal/toolcall/` upgrade — 7 new tool routes
8. Wire everything in `cmd/server/main.go` (Redis client, services, scheduler)
9. `agent_worker/` — 7 new tools + `_call_and_forward` helper
