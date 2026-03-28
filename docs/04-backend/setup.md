# Backend Setup Guide

## Prerequisites

| Tool | Version | Ghi chú |
|------|---------|---------|
| Go | 1.22+ | `go version` |
| Docker & Docker Compose | Latest | `docker compose version` |
| PostgreSQL | 16+ | Chạy trong Docker |
| Redis | 7+ | Chạy trong Docker |
| golang-migrate | Latest | `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest` |
| sqlc | Latest | `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest` |

## Cài đặt

### 1. Clone repository

```bash
git clone <repo-url> aiseebackend
cd aiseebackend
```

### 2. Cấu hình environment variables

```bash
cp .env.example .env
```

Mo file `.env` và dien các giá trị:

```env
# PostgreSQL
DATABASE_URL=postgres://aisee:aisee@localhost:5432/aisee?sslmode=disable

# Redis
REDIS_URL=redis://localhost:6379

# LiveKit
LIVEKIT_URL=ws://localhost:7880
LIVEKIT_API_KEY=devkey
LIVEKIT_API_SECRET=devsecret

# Google OAuth2
GOOGLE_CLIENT_ID=<your-google-client-id>

# JWT
JWT_SECRET=<random-secret-string>

# Internal API (Agent Worker -> Backend)
INTERNAL_API_KEY=<shared-secret-for-agent>

# Push Notifications
APNS_KEY_PATH=./certs/AuthKey.p8
APNS_KEY_ID=<key-id>
APNS_TEAM_ID=<team-id>
APNS_BUNDLE_ID=com.dang.aisee
FCM_SERVICE_ACCOUNT_PATH=./certs/fcm-service-account.json

# Server
PORT=8080
ENV=development
```

### 3. Khoi dong infrastructure bang Docker Compose

```bash
docker compose up -d
```

File `docker-compose.yml` se khởi động:
- PostgreSQL tren port 5432
- Redis tren port 6379
- LiveKit Server tren port 7880

Kiem tra tất cả container da chạy:

```bash
docker compose ps
```

### 4. Chạy database migrations

```bash
migrate -path db/migrations \
  -database "$DATABASE_URL" \
  up
```

Kiem tra migration da ap dùng thành công:

```bash
migrate -path db/migrations \
  -database "$DATABASE_URL" \
  version
```

### 5. Generate code tu sqlc

```bash
sqlc generate
```

### 6. Khoi dong server

```bash
go run cmd/server/main.go
```

Server se lang nghe tren port 8080 (hoặc port trong bien `PORT`).

## Xac nhan server hoạt động

### Healthcheck endpoint

```bash
curl http://localhost:8080/healthz
```

Response thành công:

```json
{
  "status": "ok",
  "db": "connected",
  "redis": "connected"
}
```

Nếu trả về HTTP 200 và status `"ok"`, server da sẵn sàng.

### Kiem tra các vấn đề thuong gap

| Vấn đề | Nguyen nhan | Cach xử lý |
|--------|-------------|------------|
| `connection refused` port 5432 | PostgreSQL chưa chay | `docker compose up -d postgres` |
| `connection refused` port 6379 | Redis chưa chay | `docker compose up -d redis` |
| `migration failed` | Database chưa ton tại | Kiem tra `DATABASE_URL` trong `.env` |
| `panic: missing env` | Thieu bien môi trường | Kiem tra `.env` da copy và dien du |

## Development workflow

```bash
# Chay server voi hot-reload (can cai air)
go install github.com/air-verse/air@latest
air

# Chay tests
go test ./...

# Lint
golangci-lint run
```
