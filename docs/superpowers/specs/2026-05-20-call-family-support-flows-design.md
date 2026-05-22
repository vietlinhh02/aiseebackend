# Family And Support Call Flows Design

## Goal

Make AI-initiated calling reliable and intention-preserving across backend and Flutter:

- `call_family_contact` calls a linked family contact by name or relationship.
- `call_support_contact` handles "nguoi ho tro":
  - if the user names a family contact, route to family calling;
  - if the request is generic, route to volunteer support.

Both flows must support caller auto-join, bounded timeout behavior, and structured outcomes that the assistant can explain consistently.

## Current Problems

1. The AI websocket path executes backend tools, but caller-side Flutter does not automatically join a newly-created family call room.
2. Family calling only returns `room_name` and `token`; no-answer is not surfaced as a structured assistant outcome.
3. The generic support flow is split across older LiveKit-oriented logic and current websocket logic, so behavior is inconsistent.
4. Volunteer support currently exposes asynchronous match polling, but the AI flow needs one assistant-friendly result contract.
5. Family relationship matching is realtime, but the session prompt can still mislead the model when it answers from stale context rather than checking live data.

## Design Summary

### 1. Separate AI tools by user intent

Introduce two assistant tools for the websocket transport:

- `call_family_contact(contact_name, call_type="voice")`
- `call_support_contact(contact_name="", call_type="voice")`

Behavior:

- `call_family_contact` always resolves a family contact live.
- `call_support_contact` first checks `contact_name`.
  - If `contact_name` is present and resolves to a family contact, it uses the same family-call backend path.
  - If `contact_name` is empty or intentionally generic, it uses volunteer matching.

Older `make_voice_call` / `make_video_call` remain supported for compatibility, but the prompt and websocket router move to the new intent-specific tools.

### 2. Use a hybrid backend + phone-local orchestration path

The websocket transport already supports `tool_request` / `tool_result` for phone-local actions. Human call flows should use that instead of hoping a backend tool result somehow opens Flutter UI.

Add a phone-local command:

- `start_call`

The orchestration contract becomes:

1. Backend tool logic prepares the room and tokens.
2. The websocket `ToolRouter` sends `tool_request: start_call` to the phone with room and contact metadata.
3. Flutter opens `CallScreen`, joins as caller, and only responds with `tool_result` after one of:
   - `answered`
   - `no_answer`
   - `failed`
4. The model receives the structured result and can narrate the outcome naturally.

This keeps the assistant on one logical tool call while still allowing caller auto-join.

### 3. Family-call contract

Backend `call_family_contact` contract:

- resolve contact live from `family_links`
- create room and tokens
- push incoming call to callee
- return enough metadata for `start_call`

Phone-local `start_call` for family mode:

- opens `CallScreen` immediately
- joins room with caller token
- starts ringback if nobody has joined yet
- waits up to a fixed timeout (30s)
- returns:
  - `{"status":"answered", ...}`
  - `{"status":"no_answer", "timeout_seconds":30, ...}`
  - `{"status":"failed", "message":"..."}` when the local join/setup failed

If no answer, Flutter closes the temporary outgoing call UI cleanly and the assistant can suggest another person.

### 4. Support-call contract

Support calling is split into two explicit cases:

#### A. Named family support

If `contact_name` is present and resolves to family, use the same family-call flow as above.

#### B. Generic support

If `contact_name` is empty or clearly generic, backend performs a synchronous assistant-oriented volunteer match:

1. request volunteer match
2. wait for match status up to a fixed timeout
3. when matched, return caller room/token/volunteer metadata
4. if timeout or no candidates, return a structured support-unavailable result

Unlike family calling, volunteer matching only yields a caller token once a volunteer has already accepted, so the user-facing timeout is on "finding support", not on "ringing someone who has not answered yet".

Returned statuses:

- `matched`
- `search_timeout`
- `unavailable`
- `failed`

When `matched`, the router then triggers the same `start_call` phone-local command so caller auto-join stays consistent.

### 5. Prompt and tool-selection rules

Prompt updates:

- requests like "goi me", "goi chi", "goi Linh" should use `call_family_contact`
- requests like "goi nguoi ho tro", "ket noi nguoi ho tro", "tim tinh nguyen vien ho tro" should use `call_support_contact`
- if the user asks whether family is linked, use live tools rather than trusting stale session context
- after `no_answer`, the assistant should suggest another family contact or support escalation
- after support `unavailable` / `search_timeout`, the assistant should state that no supporter answered in time and offer the next safe action

### 6. Structured backend results

Replace plain text error reliance with structured results that preserve intent.

Family errors to preserve:

- `contact_not_found`
- `multiple_contacts`
- `caller_start_failed`
- `no_answer`

Support errors to preserve:

- `support_unavailable`
- `support_search_timeout`
- `caller_start_failed`

The assistant prompt should treat these as user-safe outcomes, not technical failures.

## File-Level Design

### Backend

- `agent_worker/gemini_bridge/schemas.py`
  - add `call_family_contact`
  - add `call_support_contact`
  - add `start_call` as a phone-local command only if needed by test/doc parity
- `agent_worker/gemini_bridge/tool_router.py`
  - add hybrid handlers for family/support calling
  - reuse backend tool calls plus `DeviceCommandChannel.request("start_call", ...)`
- `agent_worker/prompts.py`
  - map intent to the new tools and fallback behavior
- `internal/toolcall/handler.go`
  - add backend dispatch for `call_family_contact` and `call_support_contact`
  - keep compatibility around existing family-call behavior where practical
- `internal/call/service.go`
  - expose/create helper that prepares outgoing family calls cleanly for assistant use
- `internal/volunteer/service.go`
  - add assistant-oriented wait-for-match helper returning structured statuses

### Flutter

- `lib/services/aisee_assistant_device_command_handler.dart`
  - support `start_call`
- `lib/services/aisee_ai_assistant_controller.dart`
  - continue using `deviceCommands`; no direct call logic here beyond request dispatch
- `lib/screens/call_screen.dart`
  - support caller completion callbacks or result futures for `answered` / `no_answer` / `failed`
- new or small helper near call UI/services
  - encapsulate "open outgoing call and resolve result"

## Timeouts

- family outgoing answer timeout: 30 seconds
- volunteer search timeout for assistant support calls: 30 seconds
- local caller connect timeout should stay shorter and produce `failed`

These values are explicit so assistant narration and UI behavior stay aligned.

## Testing Strategy

### Backend

- Go tests for:
  - family toolcall structured results
  - support toolcall volunteer timeout and match success
  - router hybrid orchestration around `start_call`

### Flutter

- device-command tests for `start_call`
- call-screen tests for no-answer timeout and answered completion
- assistant session/controller tests that `tool_request: start_call` reaches the UI handler and returns structured `tool_result`

## Non-Goals

- redesigning regular non-AI manual call flows outside the minimum shared helpers
- changing the volunteer registration/availability product
- routing human calls over the AI websocket itself
