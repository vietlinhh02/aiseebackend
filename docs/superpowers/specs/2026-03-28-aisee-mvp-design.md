# AiSee MVP — Technical Design Spec

**Date:** 2026-03-28
**Team:** Đắng | VentureX Season 3 | VinUniversity ELAB
**Status:** Draft — pending review

---

## 1. Product Overview

AiSee is a Flutter mobile app that serves as an AI-powered assistant for visually impaired users in Vietnam. The AI communicates via real-time voice conversation (Gemini Live API through LiveKit), can see through the phone camera on demand, and performs actions (tool calls) on behalf of the user.

**Target users (MVP):**
- Elderly with declining vision (cataracts, macular degeneration) — living in major cities (HN, HCM)
- Middle-aged with chronic eye conditions (diabetic retinopathy, glaucoma, severe myopia) — still active, need support to maintain daily life

**Core principle:** Voice-first. User speaks, AI listens, understands, acts. Camera activates only when needed. No background services. Minimal Google dependency.

---

## 2. Architecture Overview

### System Components

```
┌───────────────────────────────────────────────────┐
│              FLUTTER APP (1 app, 2 UIs)            │
│                                                   │
│  ┌─────────────────────────────────────────────┐  │
│  │  LiveKit Client SDK (livekit_client)         │  │
│  │  Single WebRTC connection for ALL:           │  │
│  │  - AI voice conversation (Gemini Live)       │  │
│  │  - Camera frames → AI vision (on-demand)     │  │
│  │  - Human VoIP calls (same room)              │  │
│  │  - Data channel (tool actions ↔ agent)       │  │
│  └──────────────────┬──────────────────────────┘  │
└─────────────────────┼─────────────────────────────┘
                      │ WebRTC
                      ▼
┌─────────────────────────────────────────────────────┐
│              LIVEKIT SERVER (Go, self-hosted)         │
│              WebRTC SFU — manages rooms              │
└──────────┬──────────────────────────┬────────────────┘
           │                          │
           ▼                          ▼
┌─────────────────────┐  ┌──────────────────────────────┐
│  AGENT WORKER        │  │  Other Participants           │
│  (Python, thin)      │  │  (family, volunteers join     │
│                      │  │   same room for calls)        │
│  Gemini Live         │  └──────────────────────────────┘
│  (RealtimeModel)     │
│                      │
│  Tool Router ────────┼──→ Go Backend API
│  Provider Tools:     │
│  - GoogleSearch      │
│  - Mapbox            │
└──────────────────────┘
           │
           ▼
┌──────────────────────────────────────┐
│         GO BACKEND API               │
│                                     │
│  Auth, Users, Chat, Calls,          │
│  Health, Emergency, Music,          │
│  Volunteers, Dashboard, Audit       │
│                                     │
│  PostgreSQL + Redis                  │
└──────────────────────────────────────┘
```

### Key architectural decisions

1. **Single LiveKit connection** handles AI conversation, camera vision, and human calls — no separate connections.
2. **Audio-only by default.** Camera activates on-demand (user asks or Gemini decides), then deactivates. This extends session limit from 2 min (audio+video) to 15 min (audio-only) and saves battery/tokens. See Section 18 for session continuation strategy.
3. **Agent Worker (Python) is thin** (~200-500 LOC). Only runs Gemini Live + routes tool calls to Go Backend. All business logic lives in Go.
4. **No background services.** App opens via Google Assistant / Siri voice command. Incoming calls wake app via VoIP Push (iOS) / FCM high-priority (Android).
5. **Minimal Google dependency.** No Firebase SDK. Auth via Google OAuth2 directly. Push via APNs (iOS) + FCM HTTP API (Android, backend-only).

---

## 3. Tech Stack

### Flutter App

| Purpose | Package |
|---|---|
| State management | `riverpod` |
| LiveKit client | `livekit_client` |
| Native call UI | `flutter_callkit_incoming` |
| Local notifications | `flutter_local_notifications` |
| Chat UI | `flutter_chat_ui` |
| Local DB | `drift` + `sqlite3_flutter_libs` |
| HTTP client | `dio` |
| Audio player | `just_audio` |
| Camera | `camera` |
| Contacts | `flutter_contacts` |
| Permissions | `permission_handler` |
| Maps/Navigation | `mapbox_maps_flutter` |
| Auth | `google_sign_in` |
| Secure storage | `flutter_secure_storage` |
| Deep links | `url_launcher` |

### LiveKit Agent Worker (Python)

| Purpose | Package |
|---|---|
| Agent framework | `livekit-agents` |
| Gemini Live | `livekit-plugins-google` |
| Provider tools | GoogleSearch (built-in) |
| HTTP to Go Backend | `httpx` |

### Go Backend

| Purpose | Package/Tool |
|---|---|
| HTTP framework | `chi` router |
| WebSocket | `nhooyr.io/websocket` |
| Database | `pgx` + `sqlc` |
| Migrations | `golang-migrate` |
| Redis | `go-redis/redis` |
| LiveKit SDK | `livekit/server-sdk-go` |
| Push (iOS APNs) | `sideshow/apns2` |
| Push (Android FCM) | FCM HTTP v1 API (net/http) |
| Auth | Google OAuth2 token verification + `golang-jwt/jwt` |
| Music | `yt-dlp` subprocess |
| Config | env vars |
| Logging | `slog` (stdlib) |

### Infrastructure

| Component | Spec (MVP) | Cost/month |
|---|---|---|
| VPS (Go Backend + PostgreSQL + Redis) | 2-4 vCPU, 4-8GB RAM | $30-80 |
| LiveKit Server (same or separate VPS) | 2-4 vCPU, 4GB RAM | $30-50 |
| Agent Worker (same VPS) | Python process, ~200MB RAM | Included |
| Mapbox | Free tier 25k req/month | $0 |
| Gemini Live API | Free tier | $0 |
| **Total** | | **$60-130/month** |

---

## 4. Tool System (21 tools)

### 4.1 Communication (5 tools)

| Tool | Description | Handler | Params |
|---|---|---|---|
| `make_voice_call` | In-app voice call via LiveKit | Agent → Go Backend (create room + push) | `contact_name: str` |
| `make_video_call` | In-app video call via LiveKit | Agent → Go Backend (create room + push) | `contact_name: str` |
| `send_chat_message` | Send voice/text message | Agent → Go Backend (save + push) | `contact_name: str, message: str` |
| `read_unread_messages` | Read unread messages aloud | Agent → Go Backend (fetch) → Gemini reads | `contact_name?: str` |
| ~~`call_emergency_service`~~ | ~~Removed — accidental calls to 115/113 are dangerous in a voice-first app~~ | — | — |

### 4.2 Emergency & Escalation (3 tools)

| Tool | Description | Handler | Params |
|---|---|---|---|
| `emergency_alert` | SOS: push GPS + create call room for all family | Agent → Go Backend (broadcast push + LiveKit room) | — |
| `escalate_to_family` | Call specific family member when AI cannot help | Agent → Go Backend (push) → LiveKit call | `contact_name?: str` |
| `escalate_to_volunteer` | Connect to available volunteer | Agent → Go Backend (match + push) → LiveKit room | `reason?: str` |

### 4.3 Health (6 tools)

| Tool | Description | Handler | Params |
|---|---|---|---|
| `set_medication_reminder` | Schedule medication reminder | Agent → Go Backend (save) + App local notif | `medicine_name: str, dosage: str, times: str[], frequency: str` |
| `list_medication_reminders` | List today's medications | Agent → Go Backend (fetch) → Gemini reads | — |
| `mark_medication_taken` | Mark medication as taken | Agent → Go Backend (update) | `medicine_name: str` |
| `set_appointment_reminder` | Schedule doctor appointment reminder | Agent → Go Backend (save) + App local notif | `doctor: str, datetime: str, notes?: str` |
| `log_health_symptom` | Log a symptom by voice | Agent → Go Backend (save) | `symptom: str, severity?: str, notes?: str` |
| `log_health_metric` | Log a health metric | Agent → Go Backend (save) | `metric_type: str, value: str, unit?: str` |

### 4.4 Vision & Camera (2 tools)

| Tool | Description | Handler | Params |
|---|---|---|---|
| `activate_camera` | Enable camera, stream frames to Gemini | Agent → App data channel → publish video track | `reason?: str` |
| `deactivate_camera` | Disable camera, return to audio-only | Agent → App data channel → unpublish video track | — |

**Camera activation logic:** Gemini decides to activate when user's request implies visual input ("read this label", "what's in front of me"). Gemini asks confirmation first: "Do you want me to look through the camera?" User can also manually request or press camera button.

### 4.5 Navigation (1 tool)

| Tool | Description | Handler | Params |
|---|---|---|---|
| `navigate_to` | Open Mapbox navigation to destination | Agent → App data channel → Mapbox SDK | `destination: str` |

### 4.6 Information (1 tool)

| Tool | Description | Handler | Params |
|---|---|---|---|
| `search_information` | Search for information | Gemini + GoogleSearch provider tool | `query: str` |

*Note: Weather, news, general Q&A, education/tutoring are handled by Gemini directly without tools.*

### 4.7 Entertainment (3 tools)

| Tool | Description | Handler | Params |
|---|---|---|---|
| `play_music` | Search and play music/radio/podcast | Agent → Go Backend (yt-dlp search) → App audio player | `query: str, source?: "youtube" \| "radio" \| "podcast"` |
| `control_music` | Control playback | Agent → App data channel → audio player | `action: "pause" \| "resume" \| "next" \| "previous" \| "volume_up" \| "volume_down" \| "stop"` |
| `add_to_queue` | Add song to queue | Agent → Go Backend (search) → App queue | `query: str` |

**Music architecture (Discord bot style):**
- **MVP:** Radio streams (VOV1, VOV3, VOH — legal, free) + Podcast RSS feeds
- **Post-MVP:** YouTube audio via `yt-dlp` (demo/internal only — YouTube ToS violation, not for store distribution without licensed API)
- Backend maintains per-user queue in Redis
- App plays audio via `just_audio` package
- Pre-configured radio stations list managed by backend

---

## 5. Auto-Escalation Logic

When Gemini cannot handle a request (low confidence, repeated failures, user frustration):

```
Gemini attempts to help
    │
    ├─ Success → respond normally
    │
    └─ Cannot help (after 2 attempts or explicit "I don't know")
            │
            ▼
       Gemini: "Tôi không chắc chắn lắm. Bạn muốn tôi
                gọi cho [primary contact] hay tình nguyện
                viên hỗ trợ không?"
            │
            ├─ User: "Gọi con gái" → escalate_to_family
            ├─ User: "Gọi tình nguyện viên" → escalate_to_volunteer
            ├─ User: "Không cần" → continue conversation
            └─ User in danger → emergency_alert (bypass confirmation)
```

**Agent safety rules:**
- All action tools (call, message, emergency) require voice confirmation before execution
- Contact whitelist: can only call/message registered family members
- Rate limits: configurable per tool (e.g., max 10 calls/day)
- Audit log: every tool call logged with timestamp, params, result
- No native dialer access: emergency calls (113/115) are NOT made by the app to prevent accidental calls. Emergency flow is in-app only (push to family + volunteer escalation).

---

## 6. Database Schema

```sql
-- Users & Auth
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    google_id TEXT UNIQUE NOT NULL,
    email TEXT NOT NULL,
    display_name TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('user', 'family')),
    is_volunteer BOOLEAN DEFAULT false,
    avatar_url TEXT,
    fcm_token TEXT,
    apns_token TEXT,
    apns_voip_token TEXT,
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- Family Links
CREATE TABLE family_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    family_id UUID NOT NULL REFERENCES users(id),
    relationship TEXT NOT NULL,
    is_primary_contact BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(user_id, family_id)
);

-- Chat Messages
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id UUID NOT NULL REFERENCES users(id),
    receiver_id UUID NOT NULL REFERENCES users(id),
    content_text TEXT,
    voice_url TEXT,
    is_read BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_messages_receiver_unread
    ON messages(receiver_id, is_read) WHERE NOT is_read;

-- Medications
CREATE TABLE medications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    dosage TEXT NOT NULL,
    times TEXT[] NOT NULL,
    frequency TEXT NOT NULL DEFAULT 'daily',
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

-- Medication Logs
CREATE TABLE medication_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    medication_id UUID NOT NULL REFERENCES medications(id),
    taken_at TIMESTAMPTZ DEFAULT now(),
    skipped BOOLEAN DEFAULT false
);

-- Health Symptoms
CREATE TABLE health_symptoms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    symptom TEXT NOT NULL,
    severity TEXT,
    notes TEXT,
    logged_at TIMESTAMPTZ DEFAULT now()
);

-- Health Metrics
CREATE TABLE health_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    metric_type TEXT NOT NULL,
    value TEXT NOT NULL,
    unit TEXT,
    logged_at TIMESTAMPTZ DEFAULT now()
);

-- Appointments
CREATE TABLE appointments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    title TEXT NOT NULL,
    location TEXT,
    datetime TIMESTAMPTZ NOT NULL,
    notes TEXT,
    reminded BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

-- Volunteers
CREATE TABLE volunteers (
    user_id UUID PRIMARY KEY REFERENCES users(id),
    available BOOLEAN DEFAULT false,
    languages TEXT[] DEFAULT '{vi}',
    rating REAL DEFAULT 5.0,
    total_calls INTEGER DEFAULT 0
);

-- Audit Logs
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    tool_name TEXT NOT NULL,
    params JSONB,
    result TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_audit_logs_user_time
    ON audit_logs(user_id, created_at DESC);

-- Additional indexes
CREATE INDEX idx_medications_user ON medications(user_id) WHERE active;
CREATE INDEX idx_health_symptoms_user_time ON health_symptoms(user_id, logged_at DESC);
CREATE INDEX idx_health_metrics_user_type_time ON health_metrics(user_id, metric_type, logged_at DESC);
CREATE INDEX idx_appointments_user_time ON appointments(user_id, datetime) WHERE datetime > now();
CREATE INDEX idx_family_links_user ON family_links(user_id);
CREATE INDEX idx_family_links_family ON family_links(family_id);
CREATE INDEX idx_messages_sender ON messages(sender_id, created_at DESC);
```

**Deletion strategy:** Soft-delete via `deleted_at TIMESTAMPTZ` column on `medications`, `family_links`, and `appointments`. Queries filter `WHERE deleted_at IS NULL`. `messages`, `health_symptoms`, `health_metrics`, and `audit_logs` are append-only and never deleted.

---

## 7. API Endpoints (Go Backend)

```
Auth:
  POST /api/v1/auth/google       # Google OAuth2 token → JWT
  POST /api/v1/auth/refresh      # Refresh JWT

Users:
  GET    /api/v1/users/me
  PUT    /api/v1/users/me
  POST   /api/v1/users/family-link  # Invite family member (generates invite code)
  POST   /api/v1/users/join-family  # Join via invite code
  GET    /api/v1/users/me/family
  DELETE /api/v1/users/family-link/:id

Chat:
  GET  /api/v1/messages/:contact_id?limit=50&before=cursor
  POST /api/v1/messages/send        # { receiver_id, content_text?, voice_url? }
  PUT  /api/v1/messages/:id/read
  GET  /api/v1/messages/unread-count
  WS   /api/v1/ws/chat              # Real-time message delivery

Calls (LiveKit):
  POST /api/v1/calls/token          # Get LiveKit room token for user
  POST /api/v1/calls/create-room    # Create room + notify recipient
  POST /api/v1/calls/end-room       # End room

Emergency:
  POST /api/v1/emergency/alert      # Broadcast to all family (push + GPS)
  POST /api/v1/emergency/escalate   # Match volunteer + create room

Health:
  CRUD /api/v1/health/medications
  POST /api/v1/health/medications/:id/taken
  POST /api/v1/health/medications/:id/skipped
  CRUD /api/v1/health/symptoms
  CRUD /api/v1/health/metrics
  CRUD /api/v1/health/appointments
  GET  /api/v1/health/summary?user_id=&period=week

Music:
  GET  /api/v1/music/radio-stations  # Pre-configured radio list
  GET  /api/v1/music/podcasts        # Podcast feed list
  POST /api/v1/music/queue/add
  GET  /api/v1/music/queue/:user_id
  DELETE /api/v1/music/queue/:user_id

Volunteers:
  POST /api/v1/volunteers/register
  PUT  /api/v1/volunteers/availability
  POST /api/v1/volunteers/match      # Find available volunteer (see Section 19)

Family Dashboard (requires family_link authorization — see Section 20):
  GET /api/v1/dashboard/:user_id/health
  GET /api/v1/dashboard/:user_id/audit-log
  GET /api/v1/dashboard/:user_id/medications
  GET /api/v1/dashboard/:user_id/location   # Last known GPS

Internal (Agent Worker → Backend, authenticated via shared secret):
  POST /api/internal/tool-call   # Generic tool execution endpoint
  POST /api/internal/push        # Send push notification
  # Auth: X-Internal-Key header, value from INTERNAL_API_KEY env var
  # Network: only accessible within Docker network (port not exposed)
```

---

## 8. Go Backend Project Structure

```
aiseebackend/
├── cmd/
│   └── server/
│       └── main.go                 # Entry point
├── internal/
│   ├── auth/                       # Google OAuth2 verify, JWT issue
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── google.go               # Token verification
│   ├── user/                       # User CRUD, family links
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── invite.go               # Invite code generation
│   ├── chat/                       # Messages, WebSocket hub
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── ws_hub.go               # WebSocket connection manager
│   ├── call/                       # LiveKit room management
│   │   ├── handler.go
│   │   └── service.go
│   ├── health/                     # Medications, symptoms, metrics
│   │   ├── handler.go
│   │   └── service.go
│   ├── emergency/                  # Alert broadcast, escalation
│   │   ├── handler.go
│   │   └── service.go
│   ├── music/                      # yt-dlp, radio, queue
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── ytdlp.go               # Subprocess wrapper
│   ├── volunteer/                  # Registry, matching
│   │   ├── handler.go
│   │   └── service.go
│   ├── dashboard/                  # Family dashboard, audit log
│   │   ├── handler.go
│   │   └── service.go
│   ├── toolcall/                   # Internal tool execution router
│   │   └── handler.go
│   └── middleware/                  # Auth, logging, rate limit, CORS
│       ├── auth.go
│       ├── logging.go
│       └── ratelimit.go
├── pkg/
│   ├── livekit/                    # LiveKit server SDK helpers
│   │   └── client.go
│   ├── push/                       # APNs + FCM push sender
│   │   ├── apns.go
│   │   └── fcm.go
│   └── response/                   # Standard API response helpers
│       └── json.go
├── db/
│   ├── migrations/                 # SQL migration files
│   │   ├── 001_init.up.sql
│   │   └── 001_init.down.sql
│   └── queries/                    # SQL files for sqlc
│       ├── users.sql
│       ├── messages.sql
│       ├── health.sql
│       └── audit.sql
├── sqlc.yaml
├── docker-compose.yml              # PostgreSQL + Redis + LiveKit
├── Dockerfile                      # Multi-stage Go build
├── .env.example
└── go.mod
```

---

## 9. Agent Worker (Python)

Thin layer: ~200-500 LOC. Only responsibilities: run Gemini Live, define tools, route tool calls to Go Backend.

```python
# agent_worker/main.py (simplified)
from livekit.agents import AgentSession, RoomOptions
from livekit.plugins import google
import httpx

GO_BACKEND_URL = "http://localhost:8080/api/internal"

# Define tools
@tool
async def make_voice_call(contact_name: str, ctx: RunContext):
    """Call a family member by name."""
    resp = await httpx.AsyncClient().post(
        f"{GO_BACKEND_URL}/tool-call",
        json={"tool": "make_voice_call", "user_id": ctx.user_id,
              "params": {"contact_name": contact_name}}
    )
    return resp.json()

@tool
async def activate_camera(reason: str, ctx: RunContext):
    """Enable camera to see through user's phone."""
    await ctx.session.send_data("enable_camera")
    return {"status": "camera_enabled"}

# ... 19 more tools defined similarly ...

async def entrypoint(session: AgentSession):
    session.llm = google.realtime.RealtimeModel(
        model="<GEMINI_LIVE_MODEL_ID>",  # e.g. "gemini-2.5-flash-live-preview"
        system_instructions=SYSTEM_PROMPT_VI,
    )
    session.tools = [
        make_voice_call, make_video_call, send_chat_message,
        read_unread_messages,
        emergency_alert, escalate_to_family, escalate_to_volunteer,
        set_medication_reminder, list_medication_reminders,
        mark_medication_taken, set_appointment_reminder,
        log_health_symptom, log_health_metric,
        activate_camera, deactivate_camera,
        navigate_to, search_information,
        play_music, control_music, add_to_queue,
    ]
    session.provider_tools = [google.tools.GoogleSearch()]
    await session.start(room_options=RoomOptions(video_input=False))
```

### System Prompt (Vietnamese)

```
Bạn là AiSee, trợ lý AI cho người suy giảm thị lực tại Việt Nam.

Nguyên tắc:
- Luôn nói tiếng Việt, giọng thân thiện, rõ ràng, chậm vừa phải
- Trả lời ngắn gọn, đi thẳng vào vấn đề
- Khi cần thực hiện hành động (gọi, nhắn, etc.), hỏi xác nhận trước
- Khi cần nhìn (đọc nhãn, mô tả), hỏi "Bạn muốn tôi xem qua camera không?"
- Khi không chắc chắn, đề xuất gọi người thân hoặc tình nguyện viên
- Không bao giờ bịa thông tin y tế
- Khi user có vẻ nguy hiểm hoặc cần giúp khẩn cấp, ưu tiên emergency_alert
```

---

## 10. Flutter App Structure

```
aisee_app/
├── lib/
│   ├── main.dart
│   ├── app.dart                      # MaterialApp, routing, theme
│   │
│   ├── core/
│   │   ├── theme/                    # Colors, typography, accessibility
│   │   │   ├── app_theme.dart
│   │   │   ├── user_theme.dart       # High contrast, large fonts
│   │   │   └── family_theme.dart     # Standard theme
│   │   ├── constants.dart
│   │   └── router.dart               # GoRouter config
│   │
│   ├── services/
│   │   ├── api_client.dart           # Dio HTTP client to Go Backend
│   │   ├── livekit_service.dart      # LiveKit connect, data channel
│   │   ├── push_service.dart         # APNs/FCM token management
│   │   ├── local_notif_service.dart  # Medication reminders
│   │   ├── audio_player_service.dart # Music/radio/podcast
│   │   ├── auth_service.dart         # Google Sign-In + JWT
│   │   └── local_db_service.dart     # Drift database
│   │
│   ├── models/                       # Data classes
│   │   ├── user.dart
│   │   ├── message.dart
│   │   ├── medication.dart
│   │   ├── health_record.dart
│   │   └── call_record.dart
│   │
│   ├── providers/                    # Riverpod providers
│   │   ├── auth_provider.dart
│   │   ├── livekit_provider.dart
│   │   ├── chat_provider.dart
│   │   ├── health_provider.dart
│   │   ├── music_provider.dart
│   │   └── contact_provider.dart
│   │
│   ├── features/
│   │   ├── auth/
│   │   │   ├── login_screen.dart     # Google Sign-In
│   │   │   └── role_select_screen.dart
│   │   │
│   │   ├── user/                     # Voice-first UI
│   │   │   ├── user_home_screen.dart # Main screen (3 big buttons + Gemini)
│   │   │   ├── camera_overlay.dart   # Camera view when activated
│   │   │   ├── call_screen.dart      # In-call UI
│   │   │   └── widgets/
│   │   │       ├── sos_button.dart
│   │   │       ├── call_button.dart
│   │   │       ├── camera_button.dart
│   │   │       ├── status_bar.dart   # "Gemini listening..." / "Offline"
│   │   │       ├── unread_badge.dart
│   │   │       └── medication_card.dart
│   │   │
│   │   ├── family/                   # Screen-based UI
│   │   │   ├── family_home_screen.dart
│   │   │   ├── chat_screen.dart
│   │   │   ├── health_screen.dart
│   │   │   ├── calls_screen.dart
│   │   │   ├── settings_screen.dart
│   │   │   └── widgets/
│   │   │       ├── health_chart.dart
│   │   │       ├── medication_list.dart
│   │   │       ├── symptom_timeline.dart
│   │   │       └── location_card.dart
│   │   │
│   │   └── shared/                   # Shared between both UIs
│   │       ├── chat/
│   │       │   ├── chat_page.dart
│   │       │   └── voice_message_bubble.dart
│   │       ├── contacts/
│   │       │   ├── contact_list.dart
│   │       │   └── invite_family.dart
│   │       └── incoming_call_handler.dart
│   │
│   └── db/
│       ├── app_database.dart         # Drift database definition
│       └── tables/                   # Drift table definitions
│           ├── cached_messages.dart
│           ├── cached_medications.dart
│           └── cached_contacts.dart
│
├── android/
│   └── app/src/main/
│       ├── AndroidManifest.xml
│       └── res/xml/shortcuts.xml     # Google Assistant "Open AiSee"
│
├── ios/
│   └── Runner/
│       ├── AppDelegate.swift
│       ├── Info.plist                # VoIP background mode, camera, mic
│       └── AiSeeIntents.swift        # Siri "Open AiSee"
│
├── pubspec.yaml
└── analysis_options.yaml
```

---

## 11. Two UIs

### User UI (Voice-First)

**Design principles:**
- Font size >= 24sp everywhere
- High contrast (dark background, bright text)
- Minimum touch target 64x64dp
- Every element has accessibilityLabel
- Tap anywhere on screen = activate Gemini mic
- 3 emergency buttons always visible (SOS, Call, Camera)
- Gemini auto-connects on app open, greets user
- No navigation required — everything via voice

**Screens:**
1. **User Home** — main screen, 3 buttons + Gemini status + notification cards
2. **Camera Overlay** — appears when Gemini activates camera, dismisses when done
3. **Call Screen** — in-call UI with mute/speaker/end buttons (large)

### Family UI (Screen-Based)

**Design principles:**
- Standard material design
- Tab-based navigation
- Full information display (health charts, message history, audit log)
- Can manage settings for the user (medications, contacts, appointments)

**Tabs:**
1. **Chat** — messages with the user (text + voice messages)
2. **Health** — medication schedule, symptom log, metric charts
3. **Calls** — call the user, call history
4. **Settings** — manage medications, contacts, appointments, notifications

---

## 12. Connection Flows

### App Open (User)

```
1. App opens → auto-login (cached JWT, refresh if expired)
2. GET /api/calls/token → LiveKit room token
3. livekit_client.connect(url, token) → join personal room
4. Agent Worker auto-dispatched to room → Gemini Live starts
5. Gemini greets: "Xin chào [name], tôi có thể giúp gì?"
6. Audio-only session (15 min, see Section 18 for continuation)
```

### Camera On-Demand

```
1. User: "Đọc cái nhãn thuốc này giúp tôi"
2. Gemini: "Bạn muốn tôi xem qua camera không?"
3. User: "Ừ" / "Có"
4. Gemini calls tool: activate_camera
5. Agent sends data channel message: { "action": "enable_camera" }
6. App receives → starts camera → publishes video track (1 FPS)
7. Gemini receives frames → describes: "Đây là thuốc Amlodipine 5mg..."
8. Gemini calls tool: deactivate_camera
9. Agent sends: { "action": "disable_camera" }
10. App stops camera → unpublishes video track → back to audio-only
```

### Human Call (AI → Family)

```
1. User: "Gọi cho con gái"
2. Gemini: "Tôi sẽ gọi cho Lan nhé?"
3. User: "Ừ"
4. Gemini calls tool: make_voice_call(contact_name="con gái")
5. Agent → Go Backend: POST /api/calls/create-room
6. Backend: create LiveKit room, send push to Lan
7. Lan's phone rings (native call UI via flutter_callkit_incoming)
8. Lan answers → joins same LiveKit room
9. Agent mutes (or leaves) → User and Lan talk directly
10. Lan hangs up → leaves room → Agent unmutes
11. Gemini: "Cuộc gọi với Lan đã kết thúc. Bạn cần gì nữa không?"
```

### Incoming Call (Family → User)

```
1. Family member taps "Call" in Family UI
2. Family app → POST /api/calls/create-room
3. Backend sends:
   - iOS: VoIP Push (PushKit) → native call UI
   - Android: FCM high-priority → flutter_callkit_incoming
4. User's phone rings with native incoming call screen
5. User answers (swipe or "Hey Siri/Google, answer call")
6. Both join LiveKit room → talk
7. During call, Gemini agent is available if user asks for help
```

---

## 13. Voice Assistant Integration

### Minimum (zero code for "open app"):

- Android: `android:label="AiSee"` in manifest → "Hey Google, mở AiSee"
- iOS: `CFBundleDisplayName="AiSee"` → "Hey Siri, mở AiSee"

### Enhanced (shortcuts for common actions):

**Android (`shortcuts.xml`):**
```xml
<capability android:name="actions.intent.OPEN_APP_FEATURE">
  <intent android:action="android.intent.action.VIEW"
          android:targetPackage="com.dang.aisee"
          android:targetClass="com.dang.aisee.MainActivity" />
</capability>
```

**iOS (`AiSeeIntents.swift`):**
```swift
struct OpenAiSeeIntent: AppIntent {
    static var title: LocalizedStringResource = "Open AiSee"
    static var openAppWhenRun = true
    func perform() async throws -> some IntentResult { .result() }
}
```

---

## 14. Offline Behavior

| Scenario | Behavior |
|---|---|
| No internet | Show "Offline" status. Gemini unavailable. |
| | Cached messages readable, medication schedule viewable |
| | Local notification reminders still work |
| | 3 big buttons disabled with voice alert |
| | SOS button: sends emergency_alert to all family members (push + GPS) |
| Lost connection mid-session | LiveKit auto-reconnect (built-in) |
| | Messages queued offline → sync when back online |

---

## 15. Phase Plan

### Phase 0: Setup + Proof of Concept (Week 1)
- Flutter project init
- Go Backend init (auth, user service, Docker Compose)
- LiveKit Server deploy (Docker)
- Agent Worker: Gemini Live conversation prototype
- **Validate Vietnamese voice quality before committing**

### Phase 1: Core Voice + Vision (Weeks 2-4)
- Gemini Live full session (audio + on-demand camera)
- 2 UI shells (User voice-first + Family tabs)
- Auth (Google Sign-In → JWT)
- Chat service (messages + WebSocket + push)
- Tools: `make_voice_call`, `send_chat_message`, `read_unread_messages`
- Tools: `activate_camera`, `deactivate_camera`
- CallKit/ConnectionService for incoming calls

### Phase 2: Full Agent Mode (Weeks 5-7)
- Tools: `emergency_alert`, `escalate_to_family`, `escalate_to_volunteer`
- Tools: `set_medication_reminder`, `list_medication_reminders`, `mark_medication_taken`
- Tools: `log_health_symptom`, `log_health_metric`, `set_appointment_reminder`
- Tools: `navigate_to` (Mapbox), `search_information`
- Tools: `play_music`, `control_music`, `add_to_queue` (music bot)
- Agent safety: confirmation flow, rate limiting, audit log
- Auto-escalation logic

### Phase 3: Family Features + Support Network (Weeks 8-9)
- Family Dashboard (health log, audit log, location)
- Volunteer registry + matching
- Family medication management
- Emergency alert broadcast system
- Google Assistant + Siri shortcuts

### Phase 4: Polish + Demo (Weeks 10-12)
- Accessibility audit (VoiceOver, TalkBack)
- User testing 50+ early adopters
- Performance optimization (session reconnect, battery)
- Fallback strategy if Gemini down
- Feature freeze + demo build

---

## 16. Risks

| Risk | Probability | Impact | Mitigation |
|---|---|---|---|
| Gemini Vietnamese voice quality insufficient | Medium | High | Phase 0 validates before committing. Fallback: STT→LLM→TTS pipeline via LiveKit |
| Gemini free tier gets rate-limited or removed | Medium | High | Architecture allows swapping to other realtime models (OpenAI, etc.) via LiveKit plugin system |
| LiveKit self-hosted downtime | Low | High | Docker restart policy, health checks, monitoring |
| Gemini hallucination / wrong tool call | High | Medium | Confirmation flow, whitelist contacts, audit log, escalation |
| Session 15-min limit disruptive for users | Low | Medium | See Section 18: transparent reconnect with context summary. Most interactions are short. |

---

## 17. Google Dependency Summary

| Google Service | Usage | Avoidable? |
|---|---|---|
| Gemini Live API | Core AI (via LiveKit) | No — core product |
| Google OAuth2 | Login only | Could add email/password later |
| FCM (Android) | Push delivery backend-only | No — Android requires it |
| Google Play Services | Required on Android | No — platform requirement |
| ~~Firebase SDK~~ | ~~Not used~~ | Fully removed |
| ~~Google Maps~~ | ~~Not used~~ | Replaced by Mapbox |
| ~~Google ML Kit~~ | ~~Not used~~ | Replaced by Gemini vision |

---

## 18. Session Continuation Strategy

Gemini Live API imposes server-side session limits: 15 min (audio-only), 2 min (audio+video). Sessions cannot be extended in-place. Strategy:

**Detection:** Agent Worker monitors session duration. At ~13 min mark (2 min before expiry), agent tells user: "Tôi cần kết nối lại một chút, xin chờ."

**Reconnect flow:**
```
1. Agent Worker creates a summary of current conversation context
   (last 5-10 turns, active tasks, pending tool results)
2. Agent Worker closes current Gemini session
3. Agent Worker opens new Gemini session with context summary
   injected as system instruction / initial context
4. New session starts — agent says: "Đã kết nối lại. Chúng ta đang nói về [topic]."
5. Total downtime: 1-3 seconds
```

**Context preservation:** The Agent Worker maintains a rolling conversation summary (last 10 turns as text). On reconnect, this summary is passed as initial context to the new session via `send_client_content`. The user does not lose context.

**Camera during reconnect:** If camera is active, it is paused during the 1-3s reconnect window. Video track remains published; only the Gemini session is recycled.

**Assumption to validate in Phase 0:** The method `send_client_content` (or equivalent) supports injecting prior conversation context into a new Gemini Live session. If not supported, fallback: start fresh session with a system instruction that includes the conversation summary as text.

**Most users will not hit the limit.** Typical interactions (ask a question, make a call, check medication) take 1-5 minutes. The 15-min limit only affects extended conversations.

---

## 19. Data Channel Protocol

Agent Worker and Flutter App communicate via LiveKit data channel for actions that require the app to do something locally (enable camera, play music, open maps).

### Message format (JSON)

**Agent → App:**
```json
{
  "type": "tool_action",
  "request_id": "uuid-v4",
  "action": "enable_camera | disable_camera | open_maps | play_audio | control_audio | open_dialer",
  "params": { ... }
}
```

**App → Agent (acknowledgment):**
```json
{
  "type": "tool_action_ack",
  "request_id": "uuid-v4",
  "status": "ok | error",
  "error_message": "Camera permission denied"
}
```

### Actions

| Action | Params | App behavior |
|---|---|---|
| `enable_camera` | `{}` | Start camera, publish video track |
| `disable_camera` | `{}` | Stop camera, unpublish video track |
| `open_maps` | `{ "destination": "..." }` | Open Mapbox navigation |
| `play_audio` | `{ "url": "...", "title": "..." }` | Start audio playback |
| `control_audio` | `{ "command": "pause\|resume\|next\|..." }` | Control audio player |
| ~~`open_dialer`~~ | ~~Removed~~ | ~~Removed~~ |

### Error handling

- If app does not ACK within 5 seconds, agent retries once.
- If retry fails, agent tells user: "Tôi không thể thực hiện được. Bạn thử [manual action] nhé."
- Camera permission denied → agent tells user to grant permission in settings.
- Audio playback failure → agent suggests trying another source.

---

## 20. Authorization Model

### Role-based access

- `role = 'user'`: Visually impaired user. Selected at signup. Has personal LiveKit room with AI agent. Can have family members linked.
- `role = 'family'`: Family member. Selected at signup. Can view linked user's data via Family Dashboard. Can manage medications/appointments for linked users.
- `is_volunteer = true`: Any user (regardless of role) can opt-in as volunteer. Volunteers receive escalation calls from any user.

**A person has one role (user or family).** If a family member is also visually impaired, they create a separate "user" account. The login email determines the account.

### Data access rules

All Family Dashboard endpoints (`/api/v1/dashboard/:user_id/*`) verify:
1. Requester is authenticated (valid JWT)
2. A `family_links` row exists where `family_id = requester.id AND user_id = :user_id AND deleted_at IS NULL`
3. If no link exists → 403 Forbidden

Medication/appointment management by family members uses the same check.

### Chat access

Users can only send/receive messages to/from users they have a `family_links` relationship with. No arbitrary messaging.

---

## 21. Error Handling

For visually impaired users, **silent failures are unacceptable.** Every error must produce a voice response.

### Error categories and responses

| Error | Detection | User experience |
|---|---|---|
| **No internet** | App connectivity check on launch | Voice: "Bạn đang offline. Tôi chỉ có thể nhắc thuốc và đọc tin nhắn đã lưu." |
| **LiveKit connection failed** | Connect timeout (10s) | Voice: "Không kết nối được. Đang thử lại..." (3 retries, then "Vui lòng kiểm tra internet.") |
| **LiveKit drops mid-session** | Disconnect event | Auto-reconnect (built-in). If fails after 3 attempts: voice alert |
| **Gemini session error** | Agent Worker exception | Agent Worker logs error, creates new session. User hears brief silence (1-3s) then agent resumes. |
| **Tool call to Backend fails** | HTTP 5xx or timeout (10s) | Agent tells user: "Tôi không thực hiện được. Bạn thử lại sau nhé." Logged to audit. |
| **Push delivery fails** | Backend HTTP error from APNs/FCM | Backend retries 3x with exponential backoff. If all fail, log warning. |
| **Google OAuth token revoked** | 401 from token refresh | App shows login screen. Voice: "Bạn cần đăng nhập lại." |
| **Camera permission denied** | App ACK with error | Agent: "Bạn cần cho phép camera trong cài đặt." |
| **Incoming call app killed** | VoIP Push (iOS) / FCM (Android) fails | No fallback — depends on OS push delivery. |

### Backend error strategy

- All HTTP handlers return structured JSON: `{ "error": { "code": "...", "message": "..." } }`
- Internal errors log full stack trace via `slog`
- Never expose internal details to client
- Health check endpoint `GET /healthz` returns 200 if DB + Redis reachable

---

## 22. Volunteer Matching (MVP)

Async matching for MVP (non-blocking):

```
Step 1 — Agent Worker calls POST /api/v1/volunteers/match
   Backend immediately returns: { "match_id": "uuid", "room_id": "uuid" }
   Agent tells user: "Đang tìm tình nguyện viên hỗ trợ, xin chờ..."

Step 2 — Backend runs matching in background goroutine:
   a. Query: SELECT user_id FROM volunteers
      WHERE available = true
        AND user_id != requesting_user_id
        AND user_id NOT IN (SELECT family_id FROM family_links WHERE user_id = requesting_user_id)
      ORDER BY rating DESC, total_calls ASC
      LIMIT 5
   b. Send push to candidate #1, wait 30s for room join
   c. If no join → push candidate #2, etc.

Step 3 — Agent Worker polls GET /api/v1/volunteers/match/{match_id}/status
   every 5 seconds (lightweight, <10ms response):
   - { "status": "searching", "attempt": 2 }
   - { "status": "matched", "volunteer_name": "...", "room_token": "..." }
   - { "status": "failed" }

Step 4 — On "matched": Agent connects user to room
   On "failed" after all 5 candidates:
   Agent: "Không có tình nguyện viên nào rảnh. Bạn muốn gọi người thân không?"
```

Total wait: up to 2.5 min, but non-blocking. Agent provides periodic voice feedback during search.

**Availability:** Volunteers toggle availability manually in the app. No automatic scheduling for MVP.

**After call:** Volunteer's `total_calls` incremented. User can rate (1-5 stars via voice). Rating updates volunteer's `rating` (rolling average).

---

## 23. Redis Usage

| Purpose | Key pattern | Data |
|---|---|---|
| Music queue | `music:queue:{user_id}` | List of audio URLs + metadata |
| Rate limiting | `ratelimit:{user_id}:{tool_name}` | Counter with TTL |
| LiveKit room tokens | `room:{room_id}` | Room metadata, participant list, TTL=20min (matches session limit + buffer) |
| Session context | `session:{user_id}` | Rolling conversation summary (last 10 turns) for reconnect |

If Redis is unavailable, the app still functions — music queue falls back to in-memory, rate limiting is skipped, session context is lost on reconnect. Redis is a performance optimization, not a hard dependency.

---

## 24. Monitoring (MVP)

| Component | Tool | Cost |
|---|---|---|
| Uptime check | UptimeRobot (free tier, 5-min interval) | $0 |
| Health endpoints | `GET /healthz` (Backend), LiveKit built-in health | $0 |
| Logs | Docker stdout → `docker logs` | $0 |
| Alerts | UptimeRobot email/Telegram alert on downtime | $0 |
| Post-MVP | Grafana + Loki (self-hosted) for log aggregation | $0 (self-hosted) |

All services log to stdout in JSON format. Docker captures logs. For MVP, `docker logs` + UptimeRobot is sufficient.
