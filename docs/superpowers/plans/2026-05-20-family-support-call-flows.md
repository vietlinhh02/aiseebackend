# Family And Support Call Flows Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make AI family/support calling reliable across backend and Flutter with caller auto-join, realtime contact resolution, and bounded timeout outcomes.

**Architecture:** Add two assistant intent tools (`call_family_contact`, `call_support_contact`) and route them through websocket hybrid handlers that combine backend call preparation with a phone-local `start_call` command. Reuse the existing call room flow for family contacts and add a synchronous assistant-oriented volunteer matching helper for support calls.

**Tech Stack:** Go backend, Python agent worker, Flutter app, LiveKit, Redis.

---

### Task 1: Assistant Tool Contracts

**Files:**
- Modify: `agent_worker/gemini_bridge/schemas.py`
- Modify: `agent_worker/gemini_bridge/tool_router.py`
- Modify: `agent_worker/prompts.py`
- Test: `agent_worker/tests/test_schemas.py`
- Test: `agent_worker/tests/test_tool_router.py`

- [ ] Add new tool declarations for `call_family_contact` and `call_support_contact`.
- [ ] Add hybrid websocket handlers that call backend first, then dispatch phone-local `start_call`.
- [ ] Update prompt guidance to prefer the new tools by intent.
- [ ] Add schema and router tests for the new tools and `start_call` orchestration.

### Task 2: Backend Family Call Tooling

**Files:**
- Modify: `internal/toolcall/handler.go`
- Modify: `internal/call/service.go`
- Modify: `internal/user/service.go` if structured resolve errors need shaping
- Test: `internal/toolcall/handler_test.go`
- Test: `internal/call/service_test.go`

- [ ] Add `call_family_contact` backend dispatch with structured success/error payloads.
- [ ] Keep realtime contact resolution and preserve disambiguation outcomes.
- [ ] Return room/token/contact metadata that the websocket router can forward to `start_call`.
- [ ] Add targeted tests for successful family call setup and resolve failures.

### Task 3: Backend Support Call Tooling

**Files:**
- Modify: `internal/toolcall/handler.go`
- Modify: `internal/volunteer/service.go`
- Modify: `internal/volunteer/handler.go` only if shared helpers improve consistency
- Test: `internal/toolcall/handler_test.go`
- Test: `internal/volunteer/service_test.go`

- [ ] Add `call_support_contact` backend dispatch.
- [ ] If a named family contact is provided and resolves, reuse family call setup.
- [ ] Otherwise add an assistant-oriented helper that waits for volunteer match up to the support timeout and returns structured statuses.
- [ ] Add tests for matched, search-timeout, and unavailable outcomes.

### Task 4: Flutter Phone-Local Outgoing Call Start

**Files:**
- Modify: `lib/services/aisee_assistant_device_command_handler.dart`
- Modify: `lib/screens/call_screen.dart`
- Modify or Create helper near call flow to await answered/no-answer completion
- Test: Flutter call/device-command tests near current assistant/call coverage

- [ ] Add phone-local command `start_call`.
- [ ] Open `CallScreen` from the assistant flow with provided room/token/contact metadata.
- [ ] Resolve the tool result only after answered/no-answer/failed.
- [ ] Close temporary outgoing UI cleanly on timeout.

### Task 5: Verification And Compatibility

**Files:**
- Modify tests only as needed

- [ ] Run Go tests for toolcall/call/volunteer packages.
- [ ] Run Python agent-worker tests for schemas/router.
- [ ] Run targeted Flutter tests for the new outgoing call flow.
- [ ] Re-check that the earlier `list_family_contacts` change still passes and remains compatible with the new prompts.
