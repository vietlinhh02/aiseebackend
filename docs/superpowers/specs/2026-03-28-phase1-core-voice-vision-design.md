# Phase 1: Core Voice + Vision — Technical Design Spec

**Date:** 2026-03-28
**Team:** Đắng | VentureX Season 3 | VinUniversity ELAB
**Status:** Draft
**Depends on:** Phase 0 (completed)
**Spec:** Sections 2, 4, 5, 6 of `docs/superpowers/specs/2026-03-28-aisee-mvp-design.md`

---

## 1. Goal

Build the chat service, push notifications, voice call flow, and real agent tool implementations on top of the Phase 0 foundation. After Phase 1, a user can:

- Send and receive text messages with family members (with push notifications and WebSocket realtime delivery)
- Ask the AI agent to call a family member (agent creates room, backend sends push, family joins)
- Ask the AI agent to read unread messages or send messages on their behalf
- Use on-demand camera (agent tells app to enable/disable camera via LiveKit data channel)

## 2. Scope

### In scope (backend only — this repo)

- Push notification package: FCM HTTP v1 (Android) + APNs via sideshow/apns2 (iOS standard + VoIP)
- Chat service: messages CRUD, WebSocket hub, push on new message
- Call service upgrade: create-room with push notification, end-room
- Tool call router: real dispatch to services (replacing Phase 0 stubs)
- Contact name resolution: match "con gái" / "Lan" to a family member UUID
- Agent worker: real tool implementations (4 backend tools + 2 data channel tools)
- User profile update endpoint (`PUT /users/me`) for push token registration
- Audit logging on every tool call

### Out of scope (deferred to Phase 2+)

- Emergency alert, escalate_to_family, escalate_to_volunteer
- `DELETE /api/v1/users/family-link/:id` (sqlc query exists, wire when needed)
- `POST /api/internal/push` (push is triggered internally by services, not exposed as separate endpoint)
- Medication reminders, health logging, appointment reminders
- Music/radio/podcast tools
- Navigation tool (navigate_to)
- Volunteer registry and matching
- Family dashboard
- Agent safety: confirmation flow, rate limiting

## 3. Architecture

```
┌─────────────────────────────────────────────────────┐
│                   Go Backend                         │
│                                                      │
│  pkg/push/            ← FCM + APNs                  │
│  internal/chat/       ← Messages CRUD + WebSocket   │
│  internal/call/       ← create-room + end-room      │
│  internal/toolcall/   ← Real router (not stubs)     │
│  internal/user/       ← + ResolveContact helper     │
│                                                      │
│  db/queries/messages.sql  ← New sqlc queries         │
└────────────────────────┬─────────────────────────────┘
                         │
            POST /api/internal/tool-call
                         │
┌────────────────────────┴─────────────────────────────┐
│               Agent Worker (Python)                   │
│                                                       │
│  backend.py           ← HTTP client → Go backend     │
│  data_channel.py      ← LiveKit data channel → App   │
│  tools.py             ← 6 real tool implementations  │
└───────────────────────────────────────────────────────┘
```

## 4. Push Notification Package

### 4.1 File structure

```
pkg/push/
├── push.go       # Pusher interface + composite implementation
├── fcm.go        # FCM HTTP v1 (Android)
└── apns.go       # APNs via sideshow/apns2 (iOS standard + VoIP)
```

### 4.2 Interface

```go
type Notification struct {
    Title string
    Body  string
    Type  string            // "normal" | "voip"
    Data  map[string]string // custom payload (type, sender_id, room_name, etc.)
}

type Pusher interface {
    Send(ctx context.Context, user dbgen.User, n Notification) error
}
```

A composite `Pusher` implementation checks which tokens the user has registered (fcm_token, apns_token, apns_voip_token) and sends to all available channels.

### 4.3 FCM HTTP v1

- Uses Go stdlib `net/http` + `golang.org/x/oauth2/google` for service account OAuth2 token
- Endpoint: `POST https://fcm.googleapis.com/v1/projects/{project}/messages:send`
- Incoming calls: `priority: HIGH` + data-only message (no notification block, so app handles display via flutter_callkit_incoming)
- Normal notifications: notification block + data block

### 4.4 APNs

- Uses `github.com/sideshow/apns2` with token-based auth (.p8 key)
- Standard push: `apns_token`, topic `com.dang.aisee`, priority high
- VoIP push: `apns_voip_token`, topic `com.dang.aisee.voip`, PushType VoIP
- VoIP push is ONLY used for incoming calls (Apple requirement)

### 4.5 Retry and error handling

- 3 attempts: immediate → 1s → 3s
- Token invalid/unregistered → delete token from DB, do not retry
- Rate limited (429) → retry with backoff
- Service unavailable (503) → retry

### 4.6 Environment variables

```
APNS_KEY_PATH=./certs/AuthKey_XXXXXXXXXX.p8
APNS_KEY_ID=XXXXXXXXXX
APNS_TEAM_ID=YYYYYYYYYY
APNS_BUNDLE_ID=com.dang.aisee
FCM_SERVICE_ACCOUNT_PATH=./certs/fcm-service-account.json
```

## 5. Chat Service

### 5.1 File structure

```
internal/chat/
├── handler.go    # REST endpoints + WebSocket upgrade
├── service.go    # Business logic (CRUD, push, broadcast)
└── hub.go        # WebSocket connection manager
```

### 5.2 Database queries

New file `db/queries/messages.sql`:

```sql
-- name: CreateMessage :one
INSERT INTO messages (sender_id, receiver_id, content_text, voice_url)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetMessagesByContact :many
SELECT * FROM messages
WHERE ((sender_id = $1 AND receiver_id = $2) OR (sender_id = $2 AND receiver_id = $1))
  AND ($3::UUID IS NULL OR created_at < (SELECT created_at FROM messages WHERE id = $3))
ORDER BY created_at DESC
LIMIT $4;

-- name: GetUnreadMessages :many
SELECT * FROM messages
WHERE receiver_id = $1 AND is_read = false
ORDER BY created_at ASC;

-- name: GetUnreadMessagesByContact :many
SELECT * FROM messages
WHERE receiver_id = $1 AND sender_id = $2 AND is_read = false
ORDER BY created_at ASC;

-- name: MarkMessageRead :exec
UPDATE messages SET is_read = true WHERE id = $1 AND receiver_id = $2;

-- name: CountUnreadMessages :one
SELECT count(*) FROM messages WHERE receiver_id = $1 AND is_read = false;
```

After adding, run `sqlc generate` to regenerate `internal/dbgen/`.

### 5.3 REST endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/messages/send` | JWT | Send message → DB → push → WebSocket broadcast |
| GET | `/api/v1/messages/:contact_id` | JWT | Message history (cursor pagination via `?before=<uuid>&limit=50`) |
| PUT | `/api/v1/messages/:id/read` | JWT | Mark message as read |
| GET | `/api/v1/messages/unread-count` | JWT | Count unread messages |

### 5.4 Authorization

- `POST /messages/send`: verify family_link exists between sender and receiver
- `GET /messages/:contact_id`: verify family_link exists between user and contact
- `PUT /messages/:id/read`: verify user is the receiver of the message

### 5.5 WebSocket

**Endpoint:** `WS /api/v1/ws/chat?token=<jwt>`

**Hub pattern:**
- `Hub` struct holds `map[userID]*websocket.Conn` (in-memory, single instance)
- On connect: verify JWT from query param, add connection to hub
- On disconnect: remove from hub
- On new message: check if receiver is online in hub → push via WebSocket
- Push notification is ALWAYS sent regardless of WebSocket status (user may be on a different device)

**Library:** `nhooyr.io/websocket`

**Limitation:** Single connection per user (`map[userID]*websocket.Conn`). If a user opens the app on two devices, the second connection replaces the first. Acceptable for MVP — visually impaired users typically use one device. Push notifications are always sent as fallback regardless of WebSocket status.

**Server → client message format:**

```json
{
  "type": "new_message",
  "message": {
    "id": "uuid",
    "sender_id": "uuid",
    "content_text": "string",
    "voice_url": "string|null",
    "created_at": "timestamp"
  }
}
```

### 5.6 Send message flow

```
POST /api/v1/messages/send { receiver_id, content_text }
    │
    ├─ Verify family_link between sender and receiver
    ├─ Insert message into DB
    ├─ Push notification to receiver (pkg/push)
    │   Title: sender display_name
    │   Body: content_text (truncated to 100 chars)
    │   Data: { type: "message", sender_id }
    ├─ WebSocket broadcast to receiver (if online in hub)
    └─ Return created message
```

## 6. Call Service Upgrade

### 6.1 New endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/calls/create-room` | JWT | Create LiveKit room + push incoming call |
| POST | `/api/v1/calls/end-room` | JWT | Delete LiveKit room |

### 6.2 create-room flow

```
POST /api/v1/calls/create-room { callee_id, call_type }
    │
    ├─ Verify family_link between caller and callee
    ├─ Create LiveKit room: "call-<short-uuid>"
    ├─ Generate LiveKit token for caller
    ├─ Send push notification to callee:
    │   ├─ iOS: VoIP Push via APNs (triggers CallKit)
    │   └─ Android: FCM high-priority data message
    │   Payload: { type: "incoming_call", caller_name, caller_id, room_name, call_type }
    └─ Return { room_name, token }
```

**Note:** `call_type` ("voice" | "video") is passed through to the push notification payload so the receiving app knows whether to show voice-only or video UI. It does not change LiveKit room configuration — video enablement is controlled client-side.

### 6.3 end-room flow

```
POST /api/v1/calls/end-room { room_name }
    │
    └─ Call LiveKit API DeleteRoom
```

### 6.4 pkg/livekit additions

Add to `pkg/livekit/client.go`:

```go
func (c *Client) CreateRoom(roomName string) error
func (c *Client) DeleteRoom(roomName string) error
```

Both use the existing `RoomServiceClient()`.

## 6.5 User profile update endpoint

Phase 0 only registered `GET /api/v1/users/me`. Phase 1 adds `PUT /api/v1/users/me` to allow the app to register push tokens (fcm_token, apns_token, apns_voip_token) and update display_name, avatar_url, and settings.

This requires:
- A new `UpdateMe` handler in `internal/user/handler.go`
- A new sqlc query `UpdateUserProfile` that updates the relevant fields
- Wiring `r.Put("/users/me", userHandler.UpdateMe)` in `cmd/server/main.go`

This is a prerequisite for push notifications — without it, the app has no way to register device tokens.

## 7. Tool Call Router

### 7.1 Upgrade from stub to real router

Replace the stub handler with a real dispatcher that routes tool calls to the appropriate service.

### 7.2 Handler dependencies

```go
type Handler struct {
    chat *chat.Service
    call *call.Service
    user *user.Service
    push push.Pusher
    lk   *livekit.Client
    db   *dbgen.Queries // for audit logging
}
```

### 7.3 Tool routing

| Tool name | Service method | Description |
|-----------|---------------|-------------|
| `make_voice_call` | user.ResolveContact → call.CreateRoom(voice) → push | Resolve contact name, create voice call room, send incoming call push |
| `make_video_call` | user.ResolveContact → call.CreateRoom(video) → push | Same as make_voice_call but with call_type "video" |
| `send_chat_message` | user.ResolveContact → chat.SendMessage | Resolve contact name, send message |
| `read_unread_messages` | user.ResolveContact (optional) → chat.GetUnread | Get unread messages, optionally filtered by contact |

### 7.4 Contact name resolution

Add to `internal/user/service.go`:

```go
func (s *Service) ResolveContact(ctx context.Context, userID, contactName string) (dbgen.User, error)
```

Logic:
1. Get all family_links for user_id (with joined user data)
2. Match `contactName` against `display_name` or `relationship` (case-insensitive, Unicode-normalized)
3. If exactly 1 match → return that user
4. If 0 matches → return error "contact not found"
5. If multiple matches → return error "multiple contacts match, please be more specific"

**Note:** Family links are bidirectional in practice. The `user_id` column is the visually impaired user and `family_id` is the family member, but either side may need to resolve contacts. The query checks both directions.

Requires a new sqlc query to get family members with their user details:

```sql
-- name: GetFamilyMembersWithDetails :many
SELECT u.*, fl.relationship FROM users u
JOIN family_links fl ON (
    (fl.family_id = u.id AND fl.user_id = $1)
    OR
    (fl.user_id = u.id AND fl.family_id = $1)
)
WHERE fl.deleted_at IS NULL AND u.id != $1;
```

### 7.5 Audit logging

Every tool call is logged after execution:

```go
queries.CreateAuditLog(ctx, dbgen.CreateAuditLogParams{
    UserID:   userID,
    ToolName: req.Tool,
    Params:   paramsJSON,
    Result:   resultJSON,
})
```

Uses the existing `audit.sql` query from Phase 0. Note: the `result` column is `TEXT` (not `JSONB`), so results are stored as serialized JSON strings. This is intentional — results may sometimes be simple status messages.

## 8. Agent Worker

### 8.1 File structure

```
agent_worker/
├── main.py              # Entrypoint (inject dependencies)
├── config.py            # Env vars (unchanged)
├── tools.py             # 5 tool definitions
├── backend.py           # BackendClient (HTTP → Go backend)
├── data_channel.py      # DataChannelBridge (LiveKit data channel → App)
├── pyproject.toml       # Dependencies (unchanged)
└── Dockerfile           # Unchanged
```

### 8.2 BackendClient (`backend.py`)

```python
class BackendClient:
    def __init__(self, base_url: str, api_key: str):
        self.client = httpx.AsyncClient(base_url=base_url)
        self.api_key = api_key

    async def tool_call(self, tool: str, user_id: str, params: dict) -> dict:
        resp = await self.client.post(
            "/tool-call",
            json={"tool": tool, "user_id": user_id, "params": params},
            headers={"X-Internal-Key": self.api_key},
        )
        resp.raise_for_status()
        return resp.json()
```

### 8.3 DataChannelBridge (`data_channel.py`)

```python
class DataChannelBridge:
    async def send_action(self, participant, action: str, params: dict) -> dict:
        request_id = str(uuid.uuid4())
        message = {
            "type": "tool_action",
            "request_id": request_id,
            "action": action,
            "params": params,
        }
        # Send via LiveKit data channel
        # Wait for tool_action_ack with matching request_id
        # Timeout 5s, retry once
        # Return ack or raise error
```

Uses the LiveKit agents SDK data channel API to send reliable messages to the app participant.

### 8.4 Tool implementations (`tools.py`)

**Backend tools** (call Go backend via HTTP):

| Tool | Params | Backend tool name |
|------|--------|------------------|
| `make_voice_call` | `contact_name: str` | `make_voice_call` |
| `make_video_call` | `contact_name: str` | `make_video_call` |
| `send_chat_message` | `contact_name: str, message: str` | `send_chat_message` |
| `read_unread_messages` | `contact_name: str = ""` | `read_unread_messages` |

**Data channel tools** (send to app via LiveKit data channel):

| Tool | Data channel action | Response |
|------|-------------------|----------|
| `activate_camera(reason: str)` | `enable_camera` | `{ "status": "camera_enabled" }` |
| `deactivate_camera()` | `disable_camera` | `{ "status": "camera_disabled" }` |

**Tool registration strategy:** Only register the 6 implemented tools with the Gemini agent session. Do NOT register unimplemented tools as stubs — Gemini would try to call them and get confusing "not_implemented" responses. Unimplemented tools are added in Phase 2+ as they are built.

### 8.5 User ID from room context

The agent worker gets the user_id from the LiveKit room participant identity. When the Go backend creates a room token, it sets `identity = user_id`. The agent reads this from `ctx.room.remote_participants`.

### 8.6 main.py changes

- Create `BackendClient` and `DataChannelBridge` instances
- Pass them to tool functions (via closure or global injection)
- Register all 6 tools with the agent session

## 9. New Dependencies

### Go

| Package | Purpose |
|---------|---------|
| `nhooyr.io/websocket` | WebSocket for chat |
| `github.com/sideshow/apns2` | iOS push notifications |
| `golang.org/x/oauth2/google` | FCM OAuth2 service account token |

### Python (unchanged)

`httpx` is already in `pyproject.toml`. No new Python dependencies needed.

## 10. Environment Variables

### New (Phase 1)

```
APNS_KEY_PATH=./certs/AuthKey_XXXXXXXXXX.p8
APNS_KEY_ID=XXXXXXXXXX
APNS_TEAM_ID=YYYYYYYYYY
APNS_BUNDLE_ID=com.dang.aisee
FCM_SERVICE_ACCOUNT_PATH=./certs/fcm-service-account.json
```

### Existing (Phase 0, unchanged)

```
DATABASE_URL, REDIS_URL, LIVEKIT_URL, LIVEKIT_API_KEY, LIVEKIT_API_SECRET
GOOGLE_CLIENT_ID, JWT_SECRET, INTERNAL_API_KEY, GEMINI_API_KEY, PORT
```

## 11. Implementation Order

1. `pkg/push/` — FCM + APNs (shared infrastructure)
2. `db/queries/messages.sql` + `db/queries/users.sql` additions + `sqlc generate`
3. `internal/user/` — add UpdateMe handler + ResolveContact + new sqlc queries
4. `internal/chat/` — service, handler, hub
5. `internal/call/` upgrade — create-room, end-room + `pkg/livekit` additions
6. `internal/toolcall/` upgrade — real router + audit logging
7. Wire everything in `cmd/server/main.go`
8. `agent_worker/` — backend.py, data_channel.py, tools.py upgrade (6 tools)
