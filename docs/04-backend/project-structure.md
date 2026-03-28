# Go Backend Project Structure

## Directory tree

```
aiseebackend/
├── cmd/
│   └── server/
│       └── main.go                 # Entry point: khoi tao config, DB, router, start server
├── internal/
│   ├── auth/                       # Google OAuth2 verify, JWT issue/refresh
│   │   ├── handler.go              # HTTP handlers: POST /auth/google, /auth/refresh
│   │   ├── service.go              # Business logic: verify token, tao JWT
│   │   └── google.go               # Google ID token verification
│   ├── user/                       # User CRUD, family links
│   │   ├── handler.go              # HTTP handlers: GET/PUT /users/me, family endpoints
│   │   ├── service.go              # Business logic: user profile, family management
│   │   └── invite.go               # Tao va validate invite code cho family link
│   ├── chat/                       # Messages, WebSocket hub
│   │   ├── handler.go              # HTTP handlers: GET/POST messages, unread count
│   │   ├── service.go              # Business logic: gui/nhan tin nhan, danh dau da doc
│   │   └── ws_hub.go               # WebSocket connection manager: real-time message delivery
│   ├── call/                       # LiveKit room management
│   │   ├── handler.go              # HTTP handlers: POST /calls/token, /calls/create-room
│   │   └── service.go              # Tao LiveKit room, phat hanh token, gui push
│   ├── health/                     # Medications, symptoms, metrics, appointments
│   │   ├── handler.go              # CRUD handlers cho tat ca health endpoints
│   │   └── service.go              # Business logic: medication reminders, health logging
│   ├── emergency/                  # Alert broadcast, escalation
│   │   ├── handler.go              # POST /emergency/alert, /emergency/escalate
│   │   └── service.go              # Broadcast push toi family, tao emergency room
│   ├── music/                      # Radio, podcast, audio queue
│   │   ├── handler.go              # GET radio/podcasts, POST/GET/DELETE queue
│   │   ├── service.go              # Quan ly queue trong Redis, search music
│   │   └── ytdlp.go               # yt-dlp subprocess wrapper (post-MVP)
│   ├── volunteer/                  # Registry, matching
│   │   ├── handler.go              # POST register, PUT availability, POST match
│   │   └── service.go              # Tim volunteer phu hop, async matching flow
│   ├── dashboard/                  # Family dashboard, audit log
│   │   ├── handler.go              # GET health, audit-log, medications, location
│   │   └── service.go              # Aggregation queries, family authorization check
│   ├── toolcall/                   # Internal tool execution router
│   │   └── handler.go              # POST /api/internal/tool-call — nhan tool call tu Agent
│   └── middleware/                  # Shared middleware cho tat ca routes
│       ├── auth.go                 # JWT verification, extract user tu token
│       ├── logging.go              # Request/response logging voi slog
│       └── ratelimit.go            # Token bucket rate limiting
├── pkg/
│   ├── livekit/                    # LiveKit server SDK helpers
│   │   └── client.go              # Tao room, phat hanh token, kick participant
│   ├── push/                       # Push notification sender
│   │   ├── apns.go                # iOS APNs + VoIP Push (sideshow/apns2)
│   │   └── fcm.go                 # Android FCM HTTP v1 API
│   └── response/                   # Standard API response helpers
│       └── json.go                # JSON response format, error response
├── db/
│   ├── migrations/                 # SQL migration files (golang-migrate)
│   │   ├── 001_init.up.sql        # Tao tat ca tables
│   │   └── 001_init.down.sql      # Rollback tables
│   └── queries/                    # SQL files cho sqlc code generation
│       ├── users.sql              # User queries
│       ├── messages.sql           # Message queries
│       ├── health.sql             # Health-related queries
│       └── audit.sql              # Audit log queries
├── sqlc.yaml                       # sqlc configuration
├── docker-compose.yml              # PostgreSQL + Redis + LiveKit
├── Dockerfile                      # Multi-stage Go build
├── .env.example                    # Template environment variables
└── go.mod                          # Go module dependencies
```

## Giải thích tung directory

### `cmd/server/`

Entry point duy nhat của ứng dụng. File `main.go` chiu trách nhiệm:
- Đọc environment variables
- Khoi tạo database connection pool (`pgx`)
- Khoi tạo Redis client
- Khoi tao LiveKit client
- Dang ky tất cả routes với `chi` router
- Bật dau HTTP server

### `internal/`

Code nội bộ, không export ra ngoai module. Mỗi feature la mot package rieng biet.

Mỗi package trong `internal/` theo cấu trúc nhat quan:

| File | Vai tro |
|------|---------|
| `handler.go` | Nhan HTTP request, parse input, goi service, tra response |
| `service.go` | Business logic, goi database, goi external services |
| File phu (`google.go`, `invite.go`, `ws_hub.go`, `ytdlp.go`) | Logic chuyen biet của package đó |

### `pkg/`

Code có thể tại sử dụng boi nhiều package trong `internal/`. Ba package chinh:

- **`pkg/livekit/`** — Wrapper quanh LiveKit Server SDK. Được sử dụng boi `call`, `emergency`, `volunteer`.
- **`pkg/push/`** — Gửi push notification. Được sử dụng boi `call`, `emergency`, `chat`.
- **`pkg/response/`** — Format JSON response chuan. Được sử dụng boi tất cả handlers.

### `db/`

- **`migrations/`** — SQL files cho `golang-migrate`. Mỗi migration co file `.up.sql` (apply) và `.down.sql` (rollback).
- **`queries/`** — SQL files cho `sqlc`. Viết raw SQL, `sqlc generate` tạo Go code type-safe.

## Nguyên tắc to chuc code

### Handler -> Service -> Database

```
HTTP Request
    │
    ▼
Handler (handler.go)
    │  Parse request body/params
    │  Validate input
    │  Goi service method
    │  Format response
    │
    ▼
Service (service.go)
    │  Business logic
    │  Goi database (sqlc generated code)
    │  Goi external services (LiveKit, push, Redis)
    │  Return result hoac error
    │
    ▼
Database (sqlc generated)
    │  Type-safe SQL queries
    │  Return Go structs
    │
    ▼
HTTP Response
```

### Quy tắc quan trọng

1. **Handler không chưa business logic.** Chi parse input, goi service, tra response.
2. **Service không biet ve HTTP.** Nhan Go types, tra Go types hoặc error.
3. **Database queries viet bang SQL thuan.** `sqlc` generate Go code, không dùng ORM.
4. **Middleware xử lý cross-cutting concerns.** Auth, logging, rate limit ap dùng cho tất cả routes.
5. **`pkg/` chi chưa code dùng chung.** Nếu code chi dùng trong 1 package, giu no trong `internal/`.
6. **Mỗi package tu quản lý dependencies.** Khong co global state. Dependencies truyen qua constructor (dependency injection).
