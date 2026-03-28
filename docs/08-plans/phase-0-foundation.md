# Phase 0: Foundation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Set up the Go backend, PostgreSQL database, LiveKit server, and Python agent worker with basic Gemini Live voice conversation — the foundation everything else depends on.

**Architecture:** Go backend (chi + pgx + sqlc) serves REST API with JWT auth. LiveKit server handles WebRTC. Python agent worker connects Gemini Live to LiveKit rooms. Docker Compose runs all services locally.

**Tech Stack:** Go 1.22+, PostgreSQL 16, Redis 7, LiveKit Server, Python 3.12+ (uv), livekit-agents, livekit-plugins-google, Docker Compose

**Spec:** `docs/superpowers/specs/2026-03-28-aisee-mvp-design.md` (Sections 2, 3, 6, 7, 8, 9)

---

## File Structure

### Go Backend

```
aiseebackend/
├── cmd/server/main.go                    # Entry point, wires everything
├── internal/
│   ├── auth/
│   │   ├── handler.go                    # POST /auth/google, /auth/refresh
│   │   ├── service.go                    # Verify Google token, issue JWT
│   │   └── google.go                     # Google public key fetcher
│   ├── user/
│   │   ├── handler.go                    # GET/PUT /users/me, family-link
│   │   ├── service.go                    # User CRUD, family link logic
│   │   └── invite.go                     # Invite code generation
│   ├── call/
│   │   ├── handler.go                    # POST /calls/token, /calls/create-room
│   │   └── service.go                    # LiveKit room management
│   ├── toolcall/
│   │   └── handler.go                    # POST /internal/tool-call router
│   └── middleware/
│       ├── auth.go                       # JWT verification middleware
│       ├── logging.go                    # Request logging (slog)
│       └── internal_auth.go             # X-Internal-Key check
├── pkg/
│   ├── livekit/client.go                # LiveKit server SDK wrapper
│   └── response/json.go                # Standard JSON response helpers
├── db/
│   ├── migrations/
│   │   ├── 001_init.up.sql              # All tables + indexes
│   │   └── 001_init.down.sql            # Drop all
│   └── queries/
│       ├── users.sql                     # sqlc queries for users + family_links
│       └── audit.sql                     # sqlc queries for audit_logs
├── sqlc.yaml
├── docker-compose.yml                    # postgres, redis, livekit-server
├── Dockerfile
├── .env.example
├── go.mod
└── go.sum
```

### Python Agent Worker

```
agent_worker/
├── main.py                               # Agent entrypoint + Gemini Live config
├── tools.py                              # Tool definitions (stubs for Phase 0)
├── config.py                             # Environment variables
├── requirements.txt                      # or pyproject.toml
└── Dockerfile
```

### Tests

```
aiseebackend/
├── internal/
│   ├── auth/handler_test.go
│   ├── auth/service_test.go
│   ├── user/handler_test.go
│   ├── user/service_test.go
│   ├── call/service_test.go
│   ├── middleware/auth_test.go
│   └── middleware/internal_auth_test.go
agent_worker/
└── test_main.py
```

---

## Task 1: Docker Compose + Database Migration

**Files:**
- Create: `docker-compose.yml`
- Create: `db/migrations/001_init.up.sql`
- Create: `db/migrations/001_init.down.sql`
- Create: `.env.example`

- [ ] **Step 1: Create `.env.example`**

```bash
# .env.example
DATABASE_URL=postgres://aisee:aisee@localhost:5432/aisee?sslmode=disable
REDIS_URL=redis://localhost:6379/0
LIVEKIT_URL=ws://localhost:7880
LIVEKIT_API_KEY=devkey
LIVEKIT_API_SECRET=devsecret
GOOGLE_CLIENT_ID=your-google-client-id
JWT_SECRET=your-jwt-secret-min-32-chars-long
INTERNAL_API_KEY=your-internal-api-key
GEMINI_API_KEY=your-gemini-api-key
PORT=8080
```

- [ ] **Step 2: Create `docker-compose.yml`**

```yaml
# docker-compose.yml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: aisee
      POSTGRES_PASSWORD: aisee
      POSTGRES_DB: aisee
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  livekit:
    image: livekit/livekit-server:latest
    command: --dev --bind 0.0.0.0
    ports:
      - "7880:7880"
      - "7881:7881"
      - "50000-50060:50000-50060/udp"
    environment:
      LIVEKIT_KEYS: "devkey: devsecret"

volumes:
  pgdata:
```

- [ ] **Step 3: Create migration `db/migrations/001_init.up.sql`**

```sql
-- 001_init.up.sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

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

CREATE TABLE family_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    family_id UUID NOT NULL REFERENCES users(id),
    relationship TEXT NOT NULL,
    is_primary_contact BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(user_id, family_id)
);

CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id UUID NOT NULL REFERENCES users(id),
    receiver_id UUID NOT NULL REFERENCES users(id),
    content_text TEXT,
    voice_url TEXT,
    is_read BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now()
);

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

CREATE TABLE medication_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    medication_id UUID NOT NULL REFERENCES medications(id),
    taken_at TIMESTAMPTZ DEFAULT now(),
    skipped BOOLEAN DEFAULT false
);

CREATE TABLE health_symptoms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    symptom TEXT NOT NULL,
    severity TEXT,
    notes TEXT,
    logged_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE health_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    metric_type TEXT NOT NULL,
    value TEXT NOT NULL,
    unit TEXT,
    logged_at TIMESTAMPTZ DEFAULT now()
);

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

CREATE TABLE volunteers (
    user_id UUID PRIMARY KEY REFERENCES users(id),
    available BOOLEAN DEFAULT false,
    languages TEXT[] DEFAULT '{vi}',
    rating REAL DEFAULT 5.0,
    total_calls INTEGER DEFAULT 0
);

CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    tool_name TEXT NOT NULL,
    params JSONB,
    result TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Indexes
CREATE INDEX idx_messages_receiver_unread ON messages(receiver_id, is_read) WHERE NOT is_read;
CREATE INDEX idx_messages_sender ON messages(sender_id, created_at DESC);
CREATE INDEX idx_medications_user ON medications(user_id) WHERE active;
CREATE INDEX idx_health_symptoms_user_time ON health_symptoms(user_id, logged_at DESC);
CREATE INDEX idx_health_metrics_user_type_time ON health_metrics(user_id, metric_type, logged_at DESC);
CREATE INDEX idx_appointments_user_time ON appointments(user_id, datetime) WHERE datetime > now();
CREATE INDEX idx_family_links_user ON family_links(user_id);
CREATE INDEX idx_family_links_family ON family_links(family_id);
CREATE INDEX idx_audit_logs_user_time ON audit_logs(user_id, created_at DESC);
```

- [ ] **Step 4: Create `db/migrations/001_init.down.sql`**

```sql
-- 001_init.down.sql
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS volunteers;
DROP TABLE IF EXISTS appointments;
DROP TABLE IF EXISTS health_metrics;
DROP TABLE IF EXISTS health_symptoms;
DROP TABLE IF EXISTS medication_logs;
DROP TABLE IF EXISTS medications;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS family_links;
DROP TABLE IF EXISTS users;
```

- [ ] **Step 5: Start services and run migration**

```bash
cp .env.example .env
docker compose up -d
# Install golang-migrate
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
# Run migration
migrate -path db/migrations -database "postgres://aisee:aisee@localhost:5432/aisee?sslmode=disable" up
```

Expected: All tables created. Verify:
```bash
docker compose exec postgres psql -U aisee -c "\dt"
```
Expected: 10 tables listed.

- [ ] **Step 6: Commit**

```bash
git init
git add docker-compose.yml db/ .env.example
git commit -m "feat: add Docker Compose and database migrations"
```

---

## Task 2: Go Module + Response Helpers + Healthcheck

**Files:**
- Create: `go.mod`
- Create: `cmd/server/main.go`
- Create: `pkg/response/json.go`

- [ ] **Step 1: Initialize Go module**

```bash
cd /home/eddiesngu/Desktop/Dang/aiseebackend
go mod init github.com/team-dang/aiseebackend
```

- [ ] **Step 2: Create `pkg/response/json.go`**

```go
package response

import (
	"encoding/json"
	"net/http"
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error Error `json:"error"`
}

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func Err(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, ErrorResponse{
		Error: Error{Code: code, Message: message},
	})
}
```

- [ ] **Step 3: Create `cmd/server/main.go`**

```go
package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/team-dang/aiseebackend/pkg/response"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	r := chi.NewRouter()
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	slog.Info("starting server", "port", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Install dependencies and run**

```bash
go get github.com/go-chi/chi/v5
go mod tidy
go run cmd/server/main.go
```

- [ ] **Step 5: Verify healthcheck**

```bash
curl http://localhost:8080/healthz
```

Expected: `{"status":"ok"}`

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum cmd/ pkg/
git commit -m "feat: Go server skeleton with healthcheck"
```

---

## Task 3: Database Connection + sqlc Setup

**Files:**
- Create: `sqlc.yaml`
- Create: `db/queries/users.sql`
- Create: `db/queries/audit.sql`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Install sqlc**

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

- [ ] **Step 2: Create `sqlc.yaml`**

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "db/queries"
    schema: "db/migrations"
    gen:
      go:
        package: "dbgen"
        out: "internal/dbgen"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_empty_slices: true
```

- [ ] **Step 3: Create `db/queries/users.sql`**

```sql
-- name: GetUserByGoogleID :one
SELECT * FROM users WHERE google_id = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (google_id, email, display_name, role)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET display_name = $2, avatar_url = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateFCMToken :exec
UPDATE users SET fcm_token = $2, updated_at = now() WHERE id = $1;

-- name: UpdateAPNSToken :exec
UPDATE users SET apns_token = $2, updated_at = now() WHERE id = $1;

-- name: GetFamilyMembers :many
SELECT u.* FROM users u
JOIN family_links fl ON fl.family_id = u.id
WHERE fl.user_id = $1 AND fl.deleted_at IS NULL;

-- name: CreateFamilyLink :one
INSERT INTO family_links (user_id, family_id, relationship, is_primary_contact)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: DeleteFamilyLink :exec
UPDATE family_links SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;
```

- [ ] **Step 4: Create `db/queries/audit.sql`**

```sql
-- name: CreateAuditLog :exec
INSERT INTO audit_logs (user_id, tool_name, params, result)
VALUES ($1, $2, $3, $4);

-- name: GetAuditLogsByUser :many
SELECT * FROM audit_logs
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
```

- [ ] **Step 5: Generate Go code from sqlc**

```bash
go get github.com/jackc/pgx/v5
sqlc generate
```

Expected: `internal/dbgen/` directory created with generated Go files.

- [ ] **Step 6: Add DB connection to main.go**

```go
// Add to cmd/server/main.go imports
import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Add before router setup in main()
func main() {
	// ... logger setup ...

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://aisee:aisee@localhost:5432/aisee?sslmode=disable"
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Update healthcheck to verify DB
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			response.Err(w, http.StatusServiceUnavailable, "db_error", "database unreachable")
			return
		}
		response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// ... rest of main ...
}
```

- [ ] **Step 7: Verify**

```bash
go mod tidy
go run cmd/server/main.go &
curl http://localhost:8080/healthz
```

Expected: `{"status":"ok"}` (DB connected)

- [ ] **Step 8: Commit**

```bash
git add sqlc.yaml db/queries/ internal/dbgen/ cmd/server/main.go go.mod go.sum
git commit -m "feat: add sqlc codegen and database connection"
```

---

## Task 4: JWT Auth Middleware

**Files:**
- Create: `internal/middleware/auth.go`
- Create: `internal/middleware/auth_test.go`
- Create: `internal/middleware/internal_auth.go`
- Create: `internal/middleware/internal_auth_test.go`

- [ ] **Step 1: Install JWT library**

```bash
go get github.com/golang-jwt/jwt/v5
```

- [ ] **Step 2: Write test for auth middleware**

```go
// internal/middleware/auth_test.go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/team-dang/aiseebackend/internal/middleware"
)

func TestAuthMiddleware_ValidToken(t *testing.T) {
	secret := "test-secret-at-least-32-characters"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  "user-123",
		"role": "user",
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	tokenStr, _ := token.SignedString([]byte(secret))

	handler := middleware.Auth(secret)(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			userID := middleware.UserIDFromContext(r.Context())
			if userID != "user-123" {
				t.Errorf("expected user-123, got %s", userID)
			}
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	handler := middleware.Auth("secret")(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			t.Error("should not reach handler")
		},
	))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	secret := "test-secret-at-least-32-characters"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-123",
		"exp": time.Now().Add(-time.Hour).Unix(),
	})
	tokenStr, _ := token.SignedString([]byte(secret))

	handler := middleware.Auth(secret)(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			t.Error("should not reach handler")
		},
	))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
go test ./internal/middleware/... -v
```

Expected: FAIL — package does not exist yet.

- [ ] **Step 4: Implement auth middleware**

```go
// internal/middleware/auth.go
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type contextKey string

const (
	userIDKey contextKey = "user_id"
	roleKey   contextKey = "role"
)

func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}

func RoleFromContext(ctx context.Context) string {
	v, _ := ctx.Value(roleKey).(string)
	return v
}

func Auth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				response.Err(w, http.StatusUnauthorized, "missing_token", "Authorization header required")
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				response.Err(w, http.StatusUnauthorized, "invalid_token", "Token is invalid or expired")
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				response.Err(w, http.StatusUnauthorized, "invalid_claims", "Cannot parse token claims")
				return
			}

			sub, _ := claims["sub"].(string)
			role, _ := claims["role"].(string)

			ctx := context.WithValue(r.Context(), userIDKey, sub)
			ctx = context.WithValue(ctx, roleKey, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
go test ./internal/middleware/... -v
```

Expected: PASS (3 tests)

- [ ] **Step 6: Write test for internal auth**

```go
// internal/middleware/internal_auth_test.go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/team-dang/aiseebackend/internal/middleware"
)

func TestInternalAuth_ValidKey(t *testing.T) {
	handler := middleware.InternalAuth("test-key")(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("X-Internal-Key", "test-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestInternalAuth_WrongKey(t *testing.T) {
	handler := middleware.InternalAuth("test-key")(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			t.Error("should not reach handler")
		},
	))

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("X-Internal-Key", "wrong-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
```

- [ ] **Step 7: Implement internal auth**

```go
// internal/middleware/internal_auth.go
package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/team-dang/aiseebackend/pkg/response"
)

func InternalAuth(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-Internal-Key")
			if subtle.ConstantTimeCompare([]byte(key), []byte(apiKey)) != 1 {
				response.Err(w, http.StatusUnauthorized, "invalid_key", "Invalid internal API key")
				return
			}
			next.ServeHTTP(w, r.WithContext(r.Context()))
		})
	}
}
```

- [ ] **Step 8: Run all tests**

```bash
go test ./internal/middleware/... -v
```

Expected: PASS (5 tests)

- [ ] **Step 9: Commit**

```bash
git add internal/middleware/
git commit -m "feat: add JWT auth and internal API key middleware"
```

---

## Task 5: Auth Service (Google OAuth2 → JWT)

**Files:**
- Create: `internal/auth/handler.go`
- Create: `internal/auth/service.go`
- Create: `internal/auth/google.go`
- Create: `internal/auth/handler_test.go`

- [ ] **Step 1: Create `internal/auth/google.go`**

```go
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type GoogleUserInfo struct {
	Sub        string `json:"sub"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	Picture    string `json:"picture"`
}

func VerifyGoogleToken(ctx context.Context, idToken string) (*GoogleUserInfo, error) {
	url := "https://oauth2.googleapis.com/tokeninfo?id_token=" + idToken
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to verify token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid token: status %d", resp.StatusCode)
	}

	var info GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &info, nil
}
```

- [ ] **Step 2: Create `internal/auth/service.go`**

```go
package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/team-dang/aiseebackend/internal/dbgen"
)

type Service struct {
	db        *pgxpool.Pool
	queries   *dbgen.Queries
	jwtSecret string
}

func NewService(db *pgxpool.Pool, jwtSecret string) *Service {
	return &Service{
		db:        db,
		queries:   dbgen.New(db),
		jwtSecret: jwtSecret,
	}
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func (s *Service) AuthenticateGoogle(ctx context.Context, idToken, role string) (*TokenPair, error) {
	info, err := VerifyGoogleToken(ctx, idToken)
	if err != nil {
		return nil, err
	}

	user, err := s.queries.GetUserByGoogleID(ctx, info.Sub)
	if err != nil {
		user, err = s.queries.CreateUser(ctx, dbgen.CreateUserParams{
			GoogleID:    info.Sub,
			Email:       info.Email,
			DisplayName: info.Name,
			Role:        role,
		})
		if err != nil {
			return nil, err
		}
	}

	return s.generateTokenPair(user.ID.String(), user.Role)
}

func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	token, err := jwt.Parse(refreshToken, func(t *jwt.Token) (any, error) {
		return []byte(s.jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid refresh token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}

	typ, _ := claims["type"].(string)
	if typ != "refresh" {
		return nil, fmt.Errorf("not a refresh token")
	}

	sub, _ := claims["sub"].(string)
	role, _ := claims["role"].(string)
	return s.generateTokenPair(sub, role)
}

func (s *Service) generateTokenPair(userID, role string) (*TokenPair, error) {
	accessExp := time.Now().Add(time.Hour)
	access := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"type": "access",
		"exp":  accessExp.Unix(),
	})
	accessStr, err := access.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, err
	}

	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"type": "refresh",
		"exp":  time.Now().Add(30 * 24 * time.Hour).Unix(),
	})
	refreshStr, err := refresh.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessStr,
		RefreshToken: refreshStr,
		ExpiresIn:    3600,
	}, nil
}
```

- [ ] **Step 3: Create `internal/auth/handler.go`**

```go
package auth

import (
	"encoding/json"
	"net/http"

	"github.com/team-dang/aiseebackend/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type GoogleAuthRequest struct {
	IDToken string `json:"id_token"`
	Role    string `json:"role"`
}

func (h *Handler) GoogleAuth(w http.ResponseWriter, r *http.Request) {
	var req GoogleAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	if req.IDToken == "" {
		response.Err(w, http.StatusBadRequest, "missing_token", "id_token is required")
		return
	}

	if req.Role != "user" && req.Role != "family" {
		response.Err(w, http.StatusBadRequest, "invalid_role", "role must be 'user' or 'family'")
		return
	}

	tokens, err := h.service.AuthenticateGoogle(r.Context(), req.IDToken, req.Role)
	if err != nil {
		response.Err(w, http.StatusUnauthorized, "auth_failed", "Google authentication failed")
		return
	}

	response.JSON(w, http.StatusOK, tokens)
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	tokens, err := h.service.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		response.Err(w, http.StatusUnauthorized, "invalid_refresh", "Invalid or expired refresh token")
		return
	}

	response.JSON(w, http.StatusOK, tokens)
}
```

- [ ] **Step 4: Wire auth routes in main.go**

Add to `cmd/server/main.go`:

```go
import (
	"github.com/team-dang/aiseebackend/internal/auth"
	"github.com/team-dang/aiseebackend/internal/middleware"
)

// In main(), after pool creation:
jwtSecret := os.Getenv("JWT_SECRET")
internalKey := os.Getenv("INTERNAL_API_KEY")

authService := auth.NewService(pool, jwtSecret)
authHandler := auth.NewHandler(authService)

r.Route("/api/v1", func(r chi.Router) {
	r.Post("/auth/google", authHandler.GoogleAuth)
	r.Post("/auth/refresh", authHandler.Refresh)
})

r.Route("/api/internal", func(r chi.Router) {
	r.Use(middleware.InternalAuth(internalKey))
	// Internal endpoints added in later tasks
})
```

- [ ] **Step 5: Verify compilation**

```bash
go mod tidy
go build ./cmd/server/
```

Expected: builds without errors.

- [ ] **Step 6: Commit**

```bash
git add internal/auth/ cmd/server/main.go go.mod go.sum
git commit -m "feat: add Google OAuth2 auth with JWT tokens"
```

---

## Task 6: User Service (CRUD + Family Links)

**Files:**
- Create: `internal/user/handler.go`
- Create: `internal/user/service.go`
- Create: `internal/user/invite.go`

- [ ] **Step 1: Create `internal/user/invite.go`**

```go
package user

import (
	"crypto/rand"
	"encoding/hex"
)

func GenerateInviteCode() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
```

- [ ] **Step 2: Create `internal/user/service.go`**

```go
package user

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/team-dang/aiseebackend/internal/dbgen"
)

type Service struct {
	queries *dbgen.Queries
	db      *pgxpool.Pool
	invites map[string]InviteData // In-memory for MVP, Redis later
}

type InviteData struct {
	UserID       string
	Relationship string
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		queries: dbgen.New(db),
		db:      db,
		invites: make(map[string]InviteData),
	}
}

func (s *Service) GetMe(ctx context.Context, userID string) (dbgen.User, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return dbgen.User{}, fmt.Errorf("invalid user id")
	}
	return s.queries.GetUserByID(ctx, uid)
}

func (s *Service) GetFamily(ctx context.Context, userID string) ([]dbgen.User, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id")
	}
	return s.queries.GetFamilyMembers(ctx, uid)
}

func (s *Service) CreateInvite(ctx context.Context, userID, relationship string) string {
	code := GenerateInviteCode()
	s.invites[code] = InviteData{UserID: userID, Relationship: relationship}
	return code
}

func (s *Service) JoinFamily(ctx context.Context, familyMemberID, code string) error {
	invite, ok := s.invites[code]
	if !ok {
		return fmt.Errorf("invalid invite code")
	}
	delete(s.invites, code)

	uid, _ := uuid.Parse(invite.UserID)
	fid, _ := uuid.Parse(familyMemberID)

	_, err := s.queries.CreateFamilyLink(ctx, dbgen.CreateFamilyLinkParams{
		UserID:           uid,
		FamilyID:         fid,
		Relationship:     invite.Relationship,
		IsPrimaryContact: false,
	})
	return err
}
```

- [ ] **Step 3: Create `internal/user/handler.go`**

```go
package user

import (
	"encoding/json"
	"net/http"

	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	user, err := h.service.GetMe(r.Context(), userID)
	if err != nil {
		response.Err(w, http.StatusNotFound, "user_not_found", "User not found")
		return
	}
	response.JSON(w, http.StatusOK, user)
}

func (h *Handler) GetFamily(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	family, err := h.service.GetFamily(r.Context(), userID)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "fetch_failed", "Failed to fetch family")
		return
	}
	response.JSON(w, http.StatusOK, family)
}

type CreateInviteRequest struct {
	Relationship string `json:"relationship"`
}

func (h *Handler) CreateFamilyLink(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req CreateInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	code := h.service.CreateInvite(r.Context(), userID, req.Relationship)
	response.JSON(w, http.StatusOK, map[string]string{"invite_code": code})
}

type JoinFamilyRequest struct {
	InviteCode string `json:"invite_code"`
}

func (h *Handler) JoinFamily(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req JoinFamilyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	if err := h.service.JoinFamily(r.Context(), userID, req.InviteCode); err != nil {
		response.Err(w, http.StatusBadRequest, "join_failed", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "joined"})
}
```

- [ ] **Step 4: Wire user routes in main.go**

```go
import "github.com/team-dang/aiseebackend/internal/user"

// In main(), after auth setup:
userService := user.NewService(pool)
userHandler := user.NewHandler(userService)

r.Route("/api/v1", func(r chi.Router) {
	r.Post("/auth/google", authHandler.GoogleAuth)
	r.Post("/auth/refresh", authHandler.Refresh)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(jwtSecret))
		r.Get("/users/me", userHandler.GetMe)
		r.Post("/users/family-link", userHandler.CreateFamilyLink)
		r.Post("/users/join-family", userHandler.JoinFamily)
		r.Get("/users/me/family", userHandler.GetFamily)
	})
})
```

- [ ] **Step 5: Verify**

```bash
go mod tidy
go build ./cmd/server/
```

- [ ] **Step 6: Commit**

```bash
git add internal/user/ cmd/server/main.go go.mod go.sum
git commit -m "feat: add user service with family links"
```

---

## Task 7: LiveKit Room Token Endpoint

**Files:**
- Create: `pkg/livekit/client.go`
- Create: `internal/call/handler.go`
- Create: `internal/call/service.go`

- [ ] **Step 1: Install LiveKit server SDK**

```bash
go get github.com/livekit/server-sdk-go/v2
```

- [ ] **Step 2: Create `pkg/livekit/client.go`**

```go
package livekit

import (
	"time"

	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/livekit/protocol/auth"
)

type Client struct {
	apiKey    string
	apiSecret string
	host      string
}

func NewClient(host, apiKey, apiSecret string) *Client {
	return &Client{host: host, apiKey: apiKey, apiSecret: apiSecret}
}

func (c *Client) CreateToken(roomName, identity, name string) (string, error) {
	at := auth.NewAccessToken(c.apiKey, c.apiSecret)
	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     roomName,
	}
	at.SetVideoGrant(grant).
		SetIdentity(identity).
		SetName(name).
		SetValidFor(time.Hour)

	return at.ToJWT()
}

func (c *Client) RoomClient() *lksdk.RoomServiceClient {
	return lksdk.NewRoomServiceClient(c.host, c.apiKey, c.apiSecret)
}
```

- [ ] **Step 3: Create `internal/call/service.go`**

```go
package call

import (
	"fmt"

	lk "github.com/team-dang/aiseebackend/pkg/livekit"
)

type Service struct {
	lk *lk.Client
}

func NewService(lkClient *lk.Client) *Service {
	return &Service{lk: lkClient}
}

func (s *Service) GetUserToken(userID, userName string) (string, error) {
	roomName := fmt.Sprintf("user-%s", userID)
	return s.lk.CreateToken(roomName, userID, userName)
}
```

- [ ] **Step 4: Create `internal/call/handler.go`**

```go
package call

import (
	"net/http"

	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetToken(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	token, err := h.service.GetUserToken(userID, "User")
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "token_failed", "Failed to create room token")
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{
		"token":     token,
		"room_name": "user-" + userID,
	})
}
```

- [ ] **Step 5: Wire call routes in main.go**

```go
import (
	"github.com/team-dang/aiseebackend/internal/call"
	lk "github.com/team-dang/aiseebackend/pkg/livekit"
)

// In main():
lkClient := lk.NewClient(
	os.Getenv("LIVEKIT_URL"),
	os.Getenv("LIVEKIT_API_KEY"),
	os.Getenv("LIVEKIT_API_SECRET"),
)
callService := call.NewService(lkClient)
callHandler := call.NewHandler(callService)

// Inside authenticated routes:
r.Post("/calls/token", callHandler.GetToken)
```

- [ ] **Step 6: Verify**

```bash
go mod tidy
go build ./cmd/server/
go run cmd/server/main.go &
# Test with a valid JWT (create one manually for testing)
curl -H "Authorization: Bearer <test-jwt>" http://localhost:8080/api/v1/calls/token
```

Expected: JSON with `token` and `room_name`.

- [ ] **Step 7: Commit**

```bash
git add pkg/livekit/ internal/call/ cmd/server/main.go go.mod go.sum
git commit -m "feat: add LiveKit room token endpoint"
```

---

## Task 8: Python Agent Worker — Gemini Live Conversation

**Files:**
- Create: `agent_worker/config.py`
- Create: `agent_worker/tools.py`
- Create: `agent_worker/main.py`
- Create: `agent_worker/pyproject.toml`

- [ ] **Step 1: Create `agent_worker/pyproject.toml`**

```toml
[project]
name = "aisee-agent"
version = "0.1.0"
requires-python = ">=3.12"
dependencies = [
    "livekit-agents>=1.0.0",
    "livekit-plugins-google>=1.0.0",
    "httpx>=0.27.0",
]

[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[tool.ruff]
line-length = 100
```

- [ ] **Step 2: Create `agent_worker/config.py`**

```python
import os

LIVEKIT_URL = os.environ.get("LIVEKIT_URL", "ws://localhost:7880")
LIVEKIT_API_KEY = os.environ.get("LIVEKIT_API_KEY", "devkey")
LIVEKIT_API_SECRET = os.environ.get("LIVEKIT_API_SECRET", "devsecret")
GO_BACKEND_URL = os.environ.get("GO_BACKEND_URL", "http://localhost:8080/api/internal")
INTERNAL_API_KEY = os.environ.get("INTERNAL_API_KEY", "your-internal-api-key")
GEMINI_API_KEY = os.environ.get("GEMINI_API_KEY", "")
GEMINI_MODEL = os.environ.get("GEMINI_MODEL", "gemini-2.5-flash-live-preview")
```

- [ ] **Step 3: Create `agent_worker/tools.py`** (stubs for Phase 0)

```python
"""Tool stubs for Phase 0. Real implementations added in Phase 1-2."""

from livekit.agents import function_tool


@function_tool()
async def activate_camera(reason: str = "User requested") -> dict:
    """Enable camera to see through user's phone."""
    return {"status": "not_implemented_yet"}


@function_tool()
async def deactivate_camera() -> dict:
    """Disable camera, return to audio-only."""
    return {"status": "not_implemented_yet"}


ALL_TOOLS = [activate_camera, deactivate_camera]
```

- [ ] **Step 4: Create `agent_worker/main.py`**

```python
import logging

from livekit.agents import AgentSession, Agent, RoomInputOptions
from livekit.plugins import google

from config import GEMINI_MODEL
from tools import ALL_TOOLS

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("aisee-agent")

SYSTEM_PROMPT = """
Bạn là AiSee, trợ lý AI cho người suy giảm thị lực tại Việt Nam.

RESPOND IN VIETNAMESE. YOU MUST RESPOND UNMISTAKABLY IN VIETNAMESE.

Nguyên tắc:
- Luôn nói tiếng Việt, giọng thân thiện, rõ ràng, chậm vừa phải
- Trả lời ngắn gọn, đi thẳng vào vấn đề
- Không lặp lại câu người dùng vừa nói
- Mỗi câu trả lời phải là thông tin mới, không tóm tắt lại

Lời chào: Khi bắt đầu cuộc trò chuyện, hãy nói:
"Xin chào, tôi là AiSee. Tôi có thể giúp gì cho bạn?"
"""


class AiSeeAgent(Agent):
    def __init__(self):
        super().__init__(
            instructions=SYSTEM_PROMPT,
        )


async def entrypoint(ctx):
    agent = AiSeeAgent()

    session = AgentSession(
        llm=google.realtime.RealtimeModel(
            model=GEMINI_MODEL,
        ),
    )

    for tool in ALL_TOOLS:
        session.register_tool(tool)

    await session.start(
        agent=agent,
        room=ctx.room,
        room_input_options=RoomInputOptions(video_enabled=False),
    )


if __name__ == "__main__":
    from livekit.agents import WorkerOptions, cli

    cli.run_app(
        WorkerOptions(
            entrypoint_fnc=entrypoint,
        )
    )
```

- [ ] **Step 5: Install and run**

```bash
cd agent_worker
uv venv
uv pip install -e .
# Set env vars
export GEMINI_API_KEY="your-key"
export LIVEKIT_URL="ws://localhost:7880"
export LIVEKIT_API_KEY="devkey"
export LIVEKIT_API_SECRET="devsecret"
# Run
python main.py dev
```

Expected: Agent worker connects to LiveKit, logs `"Worker registered"`.

- [ ] **Step 6: Test voice conversation**

Open LiveKit Playground (https://agents-playground.livekit.io/) or use LiveKit CLI:
```bash
lk room join --url ws://localhost:7880 --api-key devkey --api-secret devsecret --room user-test
```

Speak in Vietnamese. Verify Gemini responds in Vietnamese.

- [ ] **Step 7: Commit**

```bash
cd /home/eddiesngu/Desktop/Dang/aiseebackend
git add agent_worker/
git commit -m "feat: add Python agent worker with Gemini Live voice conversation"
```

---

## Task 9: Dockerfile + Full Docker Compose

**Files:**
- Create: `Dockerfile`
- Create: `agent_worker/Dockerfile`
- Modify: `docker-compose.yml`

- [ ] **Step 1: Create Go `Dockerfile`**

```dockerfile
# Dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /server ./cmd/server/

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /server /server
EXPOSE 8080
CMD ["/server"]
```

- [ ] **Step 2: Create agent `agent_worker/Dockerfile`**

```dockerfile
# agent_worker/Dockerfile
FROM python:3.12-slim
WORKDIR /app
RUN pip install uv
COPY pyproject.toml .
RUN uv pip install --system -e .
COPY . .
CMD ["python", "main.py", "start"]
```

- [ ] **Step 3: Update `docker-compose.yml` with all services**

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: aisee
      POSTGRES_PASSWORD: aisee
      POSTGRES_DB: aisee
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  livekit:
    image: livekit/livekit-server:latest
    command: --dev --bind 0.0.0.0
    ports:
      - "7880:7880"
      - "7881:7881"
      - "50000-50060:50000-50060/udp"
    environment:
      LIVEKIT_KEYS: "devkey: devsecret"

  backend:
    build: .
    ports:
      - "8080:8080"
    depends_on:
      - postgres
      - redis
      - livekit
    env_file: .env

  agent:
    build: ./agent_worker
    depends_on:
      - livekit
      - backend
    environment:
      LIVEKIT_URL: ws://livekit:7880
      LIVEKIT_API_KEY: devkey
      LIVEKIT_API_SECRET: devsecret
      GO_BACKEND_URL: http://backend:8080/api/internal
      INTERNAL_API_KEY: ${INTERNAL_API_KEY}
      GEMINI_API_KEY: ${GEMINI_API_KEY}

volumes:
  pgdata:
```

- [ ] **Step 4: Build and start everything**

```bash
docker compose up --build -d
docker compose logs -f
```

Expected: All 5 services start. Backend logs `"starting server"`. Agent logs `"Worker registered"`.

- [ ] **Step 5: End-to-end verify**

```bash
# Healthcheck
curl http://localhost:8080/healthz
# Expected: {"status":"ok"}
```

- [ ] **Step 6: Commit**

```bash
git add Dockerfile agent_worker/Dockerfile docker-compose.yml
git commit -m "feat: add Dockerfiles and full Docker Compose"
```

---

## Task 10: Internal Tool Call Stub Handler

**Files:**
- Create: `internal/toolcall/handler.go`

- [ ] **Step 1: Create `internal/toolcall/handler.go`**

```go
package toolcall

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/team-dang/aiseebackend/pkg/response"
)

type ToolCallRequest struct {
	Tool   string         `json:"tool"`
	UserID string         `json:"user_id"`
	Params map[string]any `json:"params"`
}

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) HandleToolCall(w http.ResponseWriter, r *http.Request) {
	var req ToolCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	slog.Info("tool call received", "tool", req.Tool, "user_id", req.UserID)

	// Phase 0: all tools return stub response
	// Real implementations added in Phase 1-2
	response.JSON(w, http.StatusOK, map[string]any{
		"tool":   req.Tool,
		"status": "not_implemented",
		"message": "Tool will be implemented in Phase 1-2",
	})
}
```

- [ ] **Step 2: Wire in main.go**

Add to the internal routes block:
```go
import "github.com/team-dang/aiseebackend/internal/toolcall"

// In main(), inside r.Route("/api/internal", ...):
toolHandler := toolcall.NewHandler()
r.Route("/api/internal", func(r chi.Router) {
	r.Use(middleware.InternalAuth(internalKey))
	r.Post("/tool-call", toolHandler.HandleToolCall)
})
```

- [ ] **Step 3: Verify**

```bash
go build ./cmd/server/
go run cmd/server/main.go &
sleep 1
curl -X POST http://localhost:8080/api/internal/tool-call \
  -H "X-Internal-Key: your-internal-api-key" \
  -H "Content-Type: application/json" \
  -d '{"tool":"make_voice_call","user_id":"test","params":{"contact_name":"mom"}}'
```

Expected: `{"tool":"make_voice_call","status":"not_implemented","message":"Tool will be implemented in Phase 1-2"}`

- [ ] **Step 4: Commit**

```bash
git add internal/toolcall/ cmd/server/main.go
git commit -m "feat: add internal tool-call stub handler"
```

---

## Summary

After completing all 10 tasks, you have:

| Component | Status |
|---|---|
| PostgreSQL + 10 tables + indexes | Running |
| Redis | Running |
| LiveKit Server (dev mode) | Running |
| Go Backend (`/healthz`, `/auth/google`, `/users/me`, `/calls/token`, `/internal/tool-call`) | Running |
| JWT auth middleware | Working + tested |
| Internal API auth (shared secret) | Working + tested |
| Internal tool-call stub handler | Ready for agent worker |
| sqlc generated queries | Ready |
| Python Agent Worker + Gemini Live | Running, responding in Vietnamese |
| Docker Compose (5 services) | All services orchestrated |

**Next:** Phase 1 (Voice + Vision) builds on this foundation — adds chat service, more tool implementations, and camera on-demand.
