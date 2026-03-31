# Phase 2b: Emergency & Escalation — Technical Design Spec

**Date:** 2026-03-28
**Team:** Dang | VentureX Season 3 | VinUniversity ELAB
**Status:** Draft
**Depends on:** Phase 1 (completed), Phase 2a (completed)
**Spec:** Sections 4.2, 5, 19 of `docs/superpowers/specs/2026-03-28-aisee-mvp-design.md`

---

## 1. Goal

Add emergency alert and family escalation tools to the AI agent. After Phase 2b, a user can:

- Trigger an SOS alert that broadcasts push notifications with GPS location to all family members and creates a call room
- Ask the AI to escalate to a family member when the AI cannot help, with automatic fallback to the next contact if no answer within 30 seconds

## 2. Scope

### In scope (backend only — this repo)

- `internal/emergency/` package: Alert broadcast + escalate with timeout
- `pkg/livekit/client.go`: add `ListParticipants` method
- New sqlc query: `GetFamilyMembersOrdered` (sorted by is_primary_contact, created_at)
- Tool-call router: 2 new tool routes
- Agent worker: 2 new tool functions
- Agent worker: `tool_call_with_timeout` method on BackendClient for extended timeouts
- Agent worker: fix `DataChannelBridge.send_action` to return full ACK payload (needed for `get_location`)
- Data channel: new `get_location` action (agent worker → app)
- SOS deduplication: 60s cooldown per user to prevent duplicate alerts

### Out of scope (deferred)

- `escalate_to_volunteer` (Phase 2b.2 — async volunteer matching)
- Offline SOS (app-only, no AI session)
- Background location tracking
- Emergency contact phone dialer

## 3. Architecture

```
Agent Worker (Python)
  emergency_alert tool:
    1. Data channel "get_location" → App → {lat, lng}
    2. POST /api/internal/tool-call {tool: "emergency_alert", params: {lat, lng}}
       → Go Backend → broadcast push to all family + create SOS room

  escalate_to_family tool:
    1. POST /api/internal/tool-call {tool: "escalate_to_family", params: {...}}
       → Go Backend → create room + push contact + poll 30s
       → Return answered/no_answer

Go Backend
  internal/emergency/service.go
    Alert()      → broadcast push + create room
    Escalate()   → try one contact, 30s timeout, poll ListParticipants
  pkg/livekit/client.go
    ListParticipants()  → check if callee joined room
```

## 4. Database Query

New query in `db/queries/users.sql`:

```sql
-- name: GetFamilyMembersOrdered :many
SELECT u.id, u.google_id, u.email, u.display_name, u.role,
       u.is_volunteer, u.avatar_url, u.fcm_token, u.apns_token,
       u.apns_voip_token, u.settings, u.created_at, u.updated_at,
       fl.relationship, fl.is_primary_contact
FROM users u
JOIN family_links fl ON (
    (fl.family_id = u.id AND fl.user_id = $1)
    OR
    (fl.user_id = u.id AND fl.family_id = $1)
)
WHERE fl.deleted_at IS NULL AND u.id != $1
ORDER BY fl.is_primary_contact DESC, fl.created_at ASC;
```

After adding, run `sqlc generate`.

## 5. LiveKit Client Addition

Add to `pkg/livekit/client.go`:

```go
func (c *Client) ListParticipants(
    ctx context.Context, roomName string,
) ([]*livekit.ParticipantInfo, error)
```

Uses existing `RoomServiceClient().ListParticipants()`.

## 6. DataChannelBridge Fix

**Problem:** `DataChannelBridge.send_action` currently returns `{"status": "ok"}` on success, discarding the full ACK payload. The `get_location` action needs `lat`, `lng`, `accuracy` from the ACK.

**Fix:** Change `send_action` to return the full `result` dict from the ACK message. Existing camera tools check `result["status"] == "ok"` which still works because the full dict also contains `"status": "ok"`.

In `agent_worker/data_channel.py`, change the return value to return the full message dict instead of just `{"status": "ok"}`.

## 7. Emergency Service

### 7.1 File structure

```
internal/emergency/
├── service.go       # Alert + Escalate logic
└── service_test.go  # Tests
```

### 7.2 Service

```go
type Service struct {
    queries   *dbgen.Queries
    pusher    *push.Pusher
    lk        *livekit.Client
    lastAlert sync.Map // map[userID]time.Time for deduplication
}

type AlertResult struct {
    RoomName       string
    Token          string
    FamilyNotified int
    Lat            float64
    Lng            float64
}

type EscalateResult struct {
    Status      string   // "answered" | "no_answer" | "no_contacts"
    RoomName    string   // set if answered
    Token       string   // set if answered
    ContactName string   // who was tried (maps to "tried" in JSON response)
    Remaining   []string // remaining contacts not yet tried
}
```

### 7.3 Alert method

```go
func (s *Service) Alert(
    ctx context.Context,
    userID string,
    lat, lng float64,
) (AlertResult, error)
```

Flow:
1. **Deduplication check:** Look up `lastAlert[userID]`. If less than 60s ago, return the previous result (or error "alert already sent, please wait"). Otherwise, record current time.
2. Get user info via `GetUserByID` for display name
3. Query `GetFamilyMembersOrdered` for all family members
4. Create LiveKit room `"sos-<8-char-uuid>"` with `EmptyTimeout: 300` (5 min auto-cleanup if nobody joins)
5. Generate LiveKit token for user
6. Send VoIP push to ALL family members. Build `push.UserTokens` from the `GetFamilyMembersOrderedRow` fields (FcmToken, ApnsToken, ApnsVoipToken):
   - Title: "SOS - [user display_name]"
   - Body: "Can giup do khan cap!"
   - Type: "voip"
   - Data: `{type: "emergency", room_name, caller_id, caller_name, lat: fmt.Sprintf("%f", lat), lng: fmt.Sprintf("%f", lng)}` — lat/lng must be converted to strings since `push.Notification.Data` is `map[string]string`
7. Return AlertResult with room info and count notified

### 7.4 Escalate method

```go
func (s *Service) Escalate(
    ctx context.Context,
    userID, contactName string,
    skip []string,
) (EscalateResult, error)
```

Flow:
1. Query `GetFamilyMembersOrdered` for all family members (returns `[]GetFamilyMembersOrderedRow` which includes `IsPrimaryContact` field — used for ordering, not needed in service logic beyond that)
2. If `contactName` provided: filter to that contact only (Vietnamese-normalized substring match on DisplayName/Relationship)
3. If `skip` provided: remove contacts whose DisplayName matches any skip entry (Vietnamese-normalized)
4. If no contacts remaining: return `{Status: "no_contacts"}`
5. Take first contact from the filtered list. Collect remaining display names for the `Remaining` field.
6. Create LiveKit room `"esc-<8-char-uuid>"`
7. Generate LiveKit token for user
8. Send VoIP push to the contact (build `push.UserTokens` from row fields)
9. Poll `ListParticipants` every 5s for 30s, **respecting `ctx.Done()`**:
   ```go
   ticker := time.NewTicker(5 * time.Second)
   defer ticker.Stop()
   deadline := time.After(30 * time.Second)
   for {
       select {
       case <-ctx.Done():
           s.lk.DeleteRoom(ctx, roomName)
           return EscalateResult{Status: "no_answer", ...}, ctx.Err()
       case <-deadline:
           s.lk.DeleteRoom(ctx, roomName)
           return EscalateResult{Status: "no_answer", ContactName: name, Remaining: remaining}, nil
       case <-ticker.C:
           participants, _ := s.lk.ListParticipants(ctx, roomName)
           if len(participants) > 0 {
               return EscalateResult{Status: "answered", RoomName: roomName, Token: token, ContactName: name}, nil
           }
       }
   }
   ```
10. If timeout: delete room, return `{Status: "no_answer", ContactName: name, Remaining: [...]}`

### 7.5 Remaining contacts

The `Remaining` field in `EscalateResult` contains display names of family members not yet tried, so the agent can tell the user who else is available and Gemini can pass them in the `skip` param on the next call.

## 8. Tool-Call Router Updates

### 8.1 New handler methods

```go
func (h *Handler) handleEmergencyAlert(ctx, req) (any, error)
func (h *Handler) handleEscalateToFamily(ctx, req) (any, error)
```

### 8.2 Tool routing additions

| Tool name | Handler method | Description |
|-----------|---------------|-------------|
| `emergency_alert` | handleEmergencyAlert | Parse lat/lng from params → emergency.Alert |
| `escalate_to_family` | handleEscalateToFamily | Parse contact_name/skip → emergency.Escalate |

### 8.3 Response formats

**emergency_alert:**
```json
{
  "status": "alert_sent",
  "room_name": "sos-abc12345",
  "token": "...",
  "family_notified": 3,
  "location": {"lat": 21.0285, "lng": 105.8542}
}
```

**escalate_to_family (answered):**
```json
{
  "status": "answered",
  "room_name": "esc-xyz98765",
  "token": "...",
  "contact_name": "Lan"
}
```

**escalate_to_family (no answer):**
```json
{
  "status": "no_answer",
  "tried": "Lan",
  "remaining": ["Minh", "Hoa"]
}
```

### 8.4 Handler dependencies update

Add `emergency *emergency.Service` to the toolcall Handler struct. The `NewHandler` constructor already takes 7 params (from Phase 2a). Adding 1 more makes 8, which exceeds the 5-param guideline. Since this is an internal constructor called once in `main.go`, accept the deviation for now — refactor to an options struct when the next phase adds more.

Updated `NewHandler` signature:

```go
func NewHandler(
    chatSvc *chat.Service,
    callSvc *call.Service,
    userSvc *user.Service,
    medSvc *medication.Service,
    healthSvc *health.Service,
    aptSvc *appointment.Service,
    emergSvc *emergency.Service,
    queries *dbgen.Queries,
) *Handler
```

### 8.5 Wiring in `cmd/server/main.go`

```go
emergService := emergency.NewService(queries, pusher, lkClient)

toolHandler := toolcall.NewHandler(
    chatService, callService, userService,
    medService, healthService, aptService,
    emergService,
    queries,
)
```

## 9. Agent Worker Updates

### 9.1 DataChannelBridge fix (`data_channel.py`)

Change `send_action` return value to return the full ACK dict instead of `{"status": "ok"}`. The existing camera tools check `result["status"] == "ok"` which still works.

### 9.2 BackendClient addition (`backend.py`)

Add `tool_call_with_timeout` method for escalation calls:

```python
async def tool_call_with_timeout(
    self, tool: str, user_id: str, params: dict,
    timeout: float = 35.0,
) -> dict:
    resp = await self.client.post(
        "/tool-call",
        json={"tool": tool, "user_id": user_id, "params": params},
        headers={"X-Internal-Key": self.api_key},
        timeout=timeout,
    )
    resp.raise_for_status()
    return resp.json()
```

### 9.3 New tool functions (`tools.py`)

```python
@function_tool()
async def emergency_alert() -> dict:
    """SOS khan cap — gui thong bao den tat ca nguoi than kem vi tri."""
    try:
        location = await _bridge.send_action(
            _local_participant, "get_location", {},
        )
        lat = location.get("lat", 0)
        lng = location.get("lng", 0)
    except DataChannelTimeoutError:
        logger.warning("get_location timed out, using unknown location")
        lat, lng = 0, 0
    return await _backend.tool_call_with_timeout(
        "emergency_alert", _user_id,
        {"lat": lat, "lng": lng},
        timeout=20.0,
    )

@function_tool()
async def escalate_to_family(
    contact_name: str = "",
    skip: list[str] | None = None,
) -> dict:
    """Goi nguoi than ho tro khi AI khong giup duoc."""
    params = {}
    if contact_name:
        params["contact_name"] = contact_name
    if skip:
        params["skip"] = skip
    return await _backend.tool_call_with_timeout(
        "escalate_to_family", _user_id, params, timeout=40.0,
    )
```

Note: `emergency_alert` uses 20s timeout (push broadcast to many contacts may take time). `escalate_to_family` uses 40s timeout (30s polling + 10s buffer).

### 9.4 Tool registration

Add both tools to `ALL_TOOLS` list.

## 10. Data Channel Protocol

### New action: `get_location`

**Agent → App:**
```json
{
  "type": "tool_action",
  "request_id": "uuid",
  "action": "get_location",
  "params": {}
}
```

**App → Agent (ACK):**
```json
{
  "type": "tool_action_ack",
  "request_id": "uuid",
  "status": "ok",
  "lat": 21.0285,
  "lng": 105.8542,
  "accuracy": 10.5
}
```

Timeout: 5s. If app doesn't respond, use lat=0, lng=0 (location unknown).

## 11. New Dependencies

None. All required packages already in go.mod (livekit SDK, push, etc.).

## 12. Implementation Order

1. `db/queries/users.sql` addition + `sqlc generate`
2. `pkg/livekit/client.go` — add `ListParticipants`
3. `internal/emergency/` — Alert + Escalate service
4. `internal/toolcall/handler.go` — 2 new tool routes
5. `cmd/server/main.go` — wire emergency service
6. `agent_worker/data_channel.py` — fix `send_action` to return full ACK payload
7. `agent_worker/backend.py` — add `tool_call_with_timeout`
8. `agent_worker/tools.py` — 2 new tools + update ALL_TOOLS
