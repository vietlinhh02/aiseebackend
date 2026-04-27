# Quota Enforcement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce and record all quota-backed billable usage paths.

**Architecture:** Add quota primitives to `billing.Service`, fix video calls to use a dedicated `video_calls` usage type, then wire high-value call/tool/app RPC paths through those primitives. Backend remains authoritative; Flutter emits analytics from stable backend/RPC errors.

**Tech Stack:** Go backend, PostgreSQL migrations, Flutter/Dart analytics.

---

### Task 1: Billing Quota Primitives

**Files:**
- Modify: `internal/billing/types.go`
- Modify: `internal/billing/service.go`
- Modify: `internal/billing/service_test.go`

- [ ] Add failing service tests for quota allow/deny and `video_calls` mapping.
- [ ] Add `UsageVideoCalls`, `QuotaCheckInput`, `QuotaDecision`, and `QuotaExceededError`.
- [ ] Implement `CheckQuota`, `RequireQuota`, and `RecordBillableUsage`.
- [ ] Verify `go test ./internal/billing`.

### Task 2: Database Usage Type Alignment

**Files:**
- Modify: `db/migrations/006_usage_ledger.up.sql`
- Create: `db/migrations/009_video_calls_usage_type.up.sql`
- Create: `db/migrations/009_video_calls_usage_type.down.sql`
- Modify: `internal/billing/catalog_migration_test.go`

- [ ] Add `video_calls` to usage constraints for new databases.
- [ ] Add migration that replaces existing usage table check constraints with a list including `video_calls`.
- [ ] Verify billing migration tests.

### Task 3: Backend Usage Wiring

**Files:**
- Modify: `cmd/server/main.go`
- Modify: `internal/toolcall/handler.go`
- Modify: `internal/call/service.go`
- Modify: `internal/call/handler.go`

- [ ] Inject billing into tool-call and call services.
- [ ] Enforce/record `video_calls`, `music_sessions`, and `volunteer_matches`.
- [ ] Start/record assistant `ai_minutes` where LiveKit token/session information is available.
- [ ] Preserve existing emergency behavior unless quota semantics are explicit.

### Task 4: App/Agent RPC and Analytics

**Files:**
- Modify: `agent_worker/backend.py`
- Modify: `agent_worker/tools.py`
- Modify: `lib/services/aisee_livekit_voice_session.dart`
- Modify: `lib/services/aisee_analytics_service.dart`
- Modify: `lib/screens/subscription_screen.dart`

- [ ] Add internal usage helpers to the agent backend client.
- [ ] Gate camera activation and record navigation sessions through trusted backend endpoints.
- [ ] Emit `quota_exceeded`, `feature_gate_seen`, and `subscription_started` at stable UI points.
- [ ] Verify relevant Go and Flutter tests.
