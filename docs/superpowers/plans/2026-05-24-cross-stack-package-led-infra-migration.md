# AiSee Cross-Stack Package-Led Infrastructure Migration Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce hand-written infrastructure across `aiseeapp`, `aiseebackend`, and `aiseebackend/agent_worker` by adopting maintained packages where they remove repeated transport, validation, contract, and chart glue, while keeping AiSee-specific voice, LiveKit, wake-word, moderation, billing, and tool-routing behavior custom.

**Architecture:** Migrate in slices, not by big-bang rewrite.
- App: replace repeated REST/JSON/socket/chart glue first.
- Go backend: add shared request validation and optionally shared API contracts before touching domain modules.
- Python worker: add typed config/payload boundaries around existing `httpx` + `websockets` transport instead of rewriting core LiveKit/Gemini flow.

**Tech Stack:**
- Flutter: `dio`, `retrofit`, `json_serializable`, optional `freezed`, `web_socket_client`, `fl_chart`
- Go: `go-playground/validator/v10`, optional `oapi-codegen`
- Python: `pydantic`, `pydantic-settings`, existing `httpx`, existing `websockets`

**Keep as-is unless a specific pain forces change:**
- Flutter: `livekit_client`, `flutter_wake_word`, `record`, `camera`, `tflite_flutter`, RevenueCat wrappers
- Go: `chi`, `pgx`, `sqlc`, `coder/websocket`, `redis`, `minio`, LiveKit SDK
- Python: `livekit-agents`, `google-genai`, `httpx`, `websockets`, `onnxruntime`

**Evidence from current codebase:**
- App has `13` HTTP service files, `45` model `fromJson/toJson` sites, `95` `jsonDecode/jsonEncode` sites, and `25` notifier/controller state sites
- Go backend has `26` request-body `json.NewDecoder(r.Body).Decode(...)` sites
- Python worker reads env vars in `22` places and already has `23` dataclass-style typed definitions, but config is still flat module globals

**Non-goals:**
- Do not replace `sqlc` with ORM
- Do not move Go backend to gRPC/Connect in this migration
- Do not rewrite `AiseeAiSession`, `AiseeLiveKitVoiceSession`, or worker Gemini session core
- Do not rewrite `internal/toolcall` domain semantics just to fit generated code

---

### Task 1: App REST transport migration

**Files:**
- Modify: `aiseeapp/pubspec.yaml`
- Create: `aiseeapp/lib/api/`
- Modify: `aiseeapp/lib/services/aisee_auth_service.dart`
- Modify: `aiseeapp/lib/services/aisee_family_service.dart`
- Modify: `aiseeapp/lib/services/aisee_billing_service.dart`
- Modify: remaining app REST services in later slices

- [ ] **Step 1: Add app transport/codegen packages**

Add:
- `dio`
- `retrofit`
- `json_annotation`

Add dev dependencies:
- `retrofit_generator`
- `json_serializable`
- `build_runner`

Optional only if model churn becomes too high:
- `freezed`
- `freezed_annotation`

- [ ] **Step 2: Build shared `Dio` bootstrap**

Requirements:
- `AppEnv.apiBaseUrl`
- auth header injection
- timeout defaults
- debug logging only in debug mode
- explicit 401 refresh path, not hidden retry magic

- [ ] **Step 3: Migrate smallest high-value app services first**

Order:
1. `aisee_auth_service.dart`
2. `aisee_family_service.dart`
3. `aisee_billing_service.dart`

- [ ] **Step 4: Expand to remaining app REST services**

Then:
- `aisee_call_service.dart`
- `aisee_dashboard_service.dart`
- `aisee_health_service.dart`
- `aisee_chat_service.dart`
- `aisee_care_service.dart`
- `aisee_upload_service.dart`
- `aisee_supporter_service.dart`
- `aisee_volunteer_service.dart`

Keep custom wrappers only for:
- fallback logic
- UI-facing errors
- multipart validation
- session-specific orchestration

**Acceptance:**
- migrated app services stop using raw `http.Response`
- manual JSON parsing in migrated slices disappears
- Flutter tests for auth/family/billing still pass

---

### Task 2: App generic websocket and chart cleanup

**Files:**
- Modify: `aiseeapp/lib/services/aisee_chat_socket.dart`
- Modify: `aiseeapp/lib/widgets/auth_shell.dart`
- Modify: `aiseeapp/lib/screens/family_messages_tab.dart`
- Modify: `aiseeapp/lib/screens/chat_detail_screen.dart`
- Modify: `aiseeapp/lib/screens/family_dashboard_health_tab.dart`
- Modify: `aiseeapp/lib/screens/family_care_widgets.dart`
- Delete: `aiseeapp/assets/chart.umd.min.js`
- Delete: `aiseeapp/assets/chart_template.html`

- [ ] **Step 1: Replace chat transport internals with `web_socket_client`**

Only for:
- `AiseeChatSocket`

Do not apply to:
- `AiseeAiSession`

Keep same outward contract:
- message stream
- incoming call event bridge
- unread/in-app message updates

- [ ] **Step 2: Replace `chartjs_flutter` with `fl_chart`**

Migrate:
- health overview chart first
- care report charts second

Then delete JS/HTML chart assets and remove `chartjs_flutter`.

**Acceptance:**
- `aisee_chat_socket.dart` no longer owns raw reconnect logic
- app charts render natively without shipped JS assets

---

### Task 3: Go backend request validation layer

**Files:**
- Modify: `aiseebackend/go.mod`
- Create: `aiseebackend/pkg/httpinput/` or similar package
- Modify: representative handlers first:
  - `internal/auth/handler.go`
  - `internal/chat/handler.go`
  - `internal/supporter/handler.go`
  - `internal/toolcall/handler.go`
  - `internal/tracking/handler.go`

- [ ] **Step 1: Add `go-playground/validator/v10`**

Use it to stop hand-writing repeated field checks after `json.NewDecoder`.

- [ ] **Step 2: Add shared decode + validate helper**

Helper responsibilities:
- decode JSON body
- reject malformed body
- run struct validation tags
- return consistent error format through existing `pkg/response`

Do not replace:
- domain checks
- auth checks
- family membership checks

- [ ] **Step 3: Pilot on small handlers first**

Start with:
- `internal/auth/handler.go`
- `internal/chat/handler.go`

Reason:
- clear request DTOs
- repeated body validation
- heavily used endpoints

- [ ] **Step 4: Expand to remaining JSON handlers**

Goal:
- remove repeated `json.NewDecoder(r.Body).Decode(...)`
- remove repeated empty-string checks where struct tags can own them

**Acceptance:**
- handler request structs use validation tags
- invalid-body and invalid-field responses become consistent
- no domain behavior changes

---

### Task 4: Go backend contract generation, only where it pays off

**Files:**
- Modify: `aiseebackend/go.mod`
- Create: `aiseebackend/api/openapi/`
- Optional generated files under dedicated package
- Modify internal and app/worker callers only after schema is stable

- [ ] **Step 1: Add `oapi-codegen` only for stable endpoint families**

Best first candidates:
- auth
- me/family
- billing usage overview
- internal worker endpoints:
  - `/api/internal/tool-call`
  - `/api/internal/usage/check`
  - `/api/internal/usage/record`
  - `/api/internal/usage/session/*`
  - `/api/internal/ai-cost/record`

- [ ] **Step 2: Keep server domain code manual**

Use OpenAPI/codegen for:
- request structs
- response structs
- client generation
- contract docs

Do not force codegen into:
- domain service internals
- sqlc stores
- tool routing logic

- [ ] **Step 3: Use internal API contract to de-risk worker/backend drift**

This is highest-value backend contract slice because worker currently posts repeated raw JSON payloads to Go backend.

**Acceptance:**
- internal API payloads become documented and typed
- Go and Python stop duplicating payload keys by hand in migrated endpoints

---

### Task 5: Keep sqlc, extend it where raw SQL still leaks

**Files:**
- Modify: `aiseebackend/db/queries/*.sql`
- Regenerate: `aiseebackend/internal/dbgen/`
- Modify raw-query stores only when worth it:
  - `internal/tracking/store.go`
  - selected admin/care stores if they keep growing

- [ ] **Step 1: Keep `sqlc` as source of truth for structured DB access**

Do not replace with ORM.

- [ ] **Step 2: Audit raw SQL outside `internal/dbgen`**

Focus on files that are growing fast or duplicating row scans, not tiny one-off queries.

- [ ] **Step 3: Migrate only repetitive/raw-query hotspots to sqlc**

Good first candidate:
- `internal/tracking/store.go`

Reason:
- explicit insert + scan logic
- JSON metadata marshal/unmarshal path
- clear table contract

**Acceptance:**
- DB layer stays `pgx + sqlc`
- raw SQL shrinks only where duplication is real

---

### Task 6: Python worker typed config boundary

**Files:**
- Modify: `aiseebackend/agent_worker/pyproject.toml`
- Replace or wrap: `aiseebackend/agent_worker/config.py`
- Modify imports in worker modules that read config

- [ ] **Step 1: Add `pydantic-settings` and `pydantic`**

Use them for:
- env parsing
- type validation
- defaults
- centralized config loading

- [ ] **Step 2: Replace flat module globals with settings object**

Current pain:
- env reads scattered through simple globals
- no central validation for required keys
- no easy test override boundary

Create typed settings model for:
- LiveKit
- backend internal API
- Gemini model/audio/video
- Exa search
- websocket transport

- [ ] **Step 3: Keep runtime semantics unchanged**

This task should only improve:
- validation
- readability
- testability

It should not change:
- default values
- business behavior

**Acceptance:**
- worker fails fast on invalid critical config
- tests can inject config cleanly
- imports stop depending on module-global env parsing

---

### Task 7: Python worker payload typing and backend client cleanup

**Files:**
- Modify: `aiseebackend/agent_worker/backend.py`
- Modify: `aiseebackend/agent_worker/gemini_bridge/schemas.py`
- Modify: `aiseebackend/agent_worker/gemini_bridge/tool_router.py`
- Optional: add generated or typed internal API DTO module

- [ ] **Step 1: Keep `httpx`, add typed request/response models**

Do not replace `httpx`.

Use `pydantic` models for:
- tool-call request/response
- quota check
- usage record/session payloads
- AI cost record payload
- tracking event payload if migrated

- [ ] **Step 2: Deduplicate repeated POST boilerplate in `backend.py`**

Centralize:
- internal auth header
- error logging
- timeout override
- response parsing

- [ ] **Step 3: Optionally bind worker models to OpenAPI-generated internal contract**

Only do this after Task 4 stabilizes.

- [ ] **Step 4: Keep `tool_router.py` custom**

Do not package away:
- family/support call split
- camera consent policy
- device command orchestration
- usage recording semantics

Only package/type:
- payload boundaries
- request/response DTOs

**Acceptance:**
- backend client stops hand-building repeated JSON dicts in every method
- worker tool/backend payload drift risk drops

---

### Task 8: Cross-stack verification gates

**Files:**
- Verify app tests under `aiseeapp/test/`
- Verify Go packages under `aiseebackend/internal/...` and `pkg/...`
- Verify Python tests under `aiseebackend/agent_worker/tests/`

- [ ] **Step 1: Verify each slice locally before moving on**

App:
```bash
cd aiseeapp
flutter analyze
flutter test
```

Go backend:
```bash
cd aiseebackend
go test ./...
```

Python worker:
```bash
cd aiseebackend/agent_worker
uv run pytest
```

- [ ] **Step 2: Prefer targeted suites during migration**

Examples:
- auth/family/billing Flutter tests for app transport slices
- handler/service package tests for Go validation slices
- `test_tool_router.py`, `test_schemas.py`, `test_websocket_service.py` for worker slices

- [ ] **Step 3: Remove obsolete dependencies only after replacement lands**

Likely candidates:
- Flutter `http`
- Flutter `chartjs_flutter`
- Flutter `http_parser` if multipart path no longer needs it

Potential new dependencies to keep:
- `validator/v10`
- `pydantic-settings`
- `pydantic`

---

### Recommended execution order

1. Task 1
2. Task 3
3. Task 6
4. Task 7
5. Task 2
6. Task 4
7. Task 5
8. Task 8 continuously

---

### Expected outcome

After this migration:
- Flutter app stops hand-writing most REST and chart glue
- Go backend keeps strong current stack, but request validation and API contracts stop being hand-copied everywhere
- Python worker gets typed config and typed internal API payloads without risky rewrite of Gemini/LiveKit core
- AiSee-specific real-time behavior stays custom where that is still correct
