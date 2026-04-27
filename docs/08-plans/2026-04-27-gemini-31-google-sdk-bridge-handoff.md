# Gemini 3.1 Google SDK Bridge Handoff Plan

**Date:** 2026-04-27

**Status:** Proposed implementation handoff

**Goal:** Move AiSee's Gemini 3.1 Live integration away from LiveKit's Google realtime plugin while keeping LiveKit as the app's WebRTC room, media, RPC, and call transport layer.

---

## Executive Summary

AiSee should not rewrite the whole Flutter app to call Gemini directly. The current app depends on LiveKit for more than AI audio: room lifecycle, microphone and camera transport, agent dispatch, participant state, app-side RPC, family calls, and media publishing.

The recommended migration is a narrower server-side rewrite:

1. Keep LiveKit Server, Go backend room/token dispatch, and Flutter `livekit_client`.
2. Replace only the Python agent's `livekit.plugins.google.RealtimeModel` usage.
3. Build a custom bridge inside `aiseebackend/agent_worker` using Google's official `google-genai` Live API SDK.
4. Stream user audio/video from LiveKit room tracks to Gemini 3.1 with the official SDK.
5. Publish Gemini audio responses back into the same LiveKit room as an agent audio track.
6. Keep tool execution server-side and continue using existing Flutter RPC methods for device actions.

This keeps the working product surface intact while avoiding the LiveKit Google plugin compatibility problem with Gemini 3.1.

---

## Why This Migration Exists

Gemini 3.1 Live has a different realtime contract than Gemini 2.5. In Gemini 3.1, `send_client_content` is restricted to initial history seeding. After the first model turn, sending more client content can fail with WebSocket close code `1007` / invalid argument.

LiveKit's Google realtime plugin currently exposes agent conveniences such as:

- `generate_reply()`
- `update_instructions()`
- `update_chat_ctx()`
- some mid-session context updates

Those paths are not compatible with Gemini 3.1 in the same way they were with Gemini 2.5. LiveKit documents that these calls are ignored or unsupported for Gemini 3.1.

The app owner observed that Gemini 2.5 is less reliable in actual AiSee tests, while Gemini 3.1 behaves better. Therefore the product direction is to keep Gemini 3.1 and remove the framework layer that conflicts with Gemini 3.1's official API behavior.

References:

- LiveKit Gemini plugin docs: https://docs.livekit.io/agents/models/realtime/plugins/gemini/
- Google Gemini 3.1 Live migration guide: https://ai.google.dev/gemini-api/docs/models/gemini-3.1-flash-live-preview#migrating
- Gemini Live API capabilities: https://ai.google.dev/gemini-api/docs/live-api/capabilities

---

## Current Architecture

### Components

- Flutter app: `aiseeapp/`
- Go backend: `aiseebackend/`
- Python agent worker: `aiseebackend/agent_worker/`
- LiveKit server: realtime room, SFU, RPC, app-to-agent media transport
- Gemini Live API: native realtime audio/video model

### Current Voice Flow

1. Flutter calls `POST /api/v1/calls/token`.
2. Go backend creates or reuses LiveKit room `user-{userID}`.
3. Go backend dispatches LiveKit agent `aisee-agent`.
4. Flutter joins LiveKit room using `livekit_client`.
5. Python worker joins the same LiveKit room.
6. Python worker starts `AgentSession`.
7. `AgentSession` uses `google.realtime.RealtimeModel`.
8. LiveKit Agents framework handles:
   - subscribing to user audio/video
   - sending media to Gemini
   - receiving Gemini audio
   - publishing agent audio to the room
   - mapping LiveKit agent tools to Gemini function calls

### Important Current Files

- `aiseebackend/agent_worker/main.py`
  - Current LiveKit `AgentSession` and `google.realtime.RealtimeModel` setup.
- `aiseebackend/agent_worker/tools.py`
  - Current `AiSeeAgent` tool definitions using LiveKit `@function_tool`.
- `aiseebackend/agent_worker/prompts.py`
  - Vietnamese and English system prompts.
- `aiseebackend/pkg/livekit/client.go`
  - Room creation, agent dispatch, token creation.
- `aiseebackend/internal/call/service.go`
  - `/calls/token` flow and AI usage session start.
- `aiseeapp/lib/services/aisee_livekit_voice_session.dart`
  - Flutter LiveKit session, app RPC handlers, camera/location/audio controls.

---

## Decision

Use a hybrid bridge:

```text
Flutter app
  |
  | LiveKit WebRTC audio/video + RPC
  v
LiveKit room
  |
  | Python worker subscribes/publishes tracks
  v
Custom agent_worker bridge
  |
  | Google official google-genai Live session
  v
Gemini 3.1 Live API
```

LiveKit remains the realtime transport between app and server. Google SDK becomes the realtime AI transport between server and Gemini.

---

## Explicit Non-Goals

Do not rewrite the Flutter app to call Gemini directly in the first migration.

Do not replace LiveKit with Voximplant, Pipecat, Daily, or another RTC stack in this migration.

Do not remove existing family call rooms or LiveKit call flows.

Do not move `GOOGLE_API_KEY` or privileged model credentials to the mobile app.

Do not reimplement all tools at once before proving the audio loop works.

---

## Options Considered

### Option A: Stay on LiveKit Google Plugin

Rejected for Gemini 3.1 because the plugin currently has documented incompatibilities with mid-session content updates and related helper calls.

Good:

- Lowest code change.
- Keeps LiveKit Agents abstractions.

Bad:

- Gemini 3.1 behavior remains constrained by LiveKit plugin compatibility.
- The app remains exposed to `send_client_content` contract mismatch.
- Hard to control exact Google Live API messages.

### Option B: Downgrade to Gemini 2.5

Rejected because product testing showed more errors with 2.5 than 3.1.

Good:

- Better LiveKit compatibility.
- Less custom bridge code.

Bad:

- Lower observed reliability for AiSee.
- Does not solve product quality issue.

### Option C: Rewrite Flutter App to Official Gemini SDK

Rejected for first migration.

Good:

- Direct official API path.

Bad:

- Mobile app would need new realtime auth/session infrastructure.
- High risk of exposing API credentials or building a complex ephemeral token proxy.
- Requires replacing LiveKit app RPC and device-control flow.
- Breaks current call room architecture.
- Larger test surface.

### Option D: Replace LiveKit with Voximplant

Rejected for first migration because AiSee is not only telephony.

Good:

- Voximplant has clear Gemini 3.1 examples for phone/SIP flows.

Bad:

- App is LiveKit/WebRTC-first, not PSTN-first.
- Would rewrite call/media infrastructure.

### Option E: Replace LiveKit Agents with Pipecat

Deferred.

Good:

- Pipecat supports Gemini Live and custom pipelines.

Bad:

- Still a framework migration.
- May not reduce scope versus a focused bridge inside the existing worker.

### Option F: Keep LiveKit, Replace Only Google Plugin With Official SDK

Selected.

Good:

- Avoids the specific broken compatibility layer.
- Preserves Flutter and Go backend contracts.
- Keeps API key server-side.
- Can be feature-flagged and rolled back.
- Lets AiSee use Gemini 3.1's official realtime API semantics.

Bad:

- Requires custom audio/video frame bridge.
- Requires custom tool-call mapping.
- Requires careful interruption and lifecycle handling.

---

## Target Architecture

### Runtime Responsibilities

#### Flutter App

Continue to:

- join LiveKit rooms
- publish microphone audio when assistant mic is enabled
- publish camera only when requested
- handle local device RPC methods:
  - `enable_camera`
  - `disable_camera`
  - `get_location`
  - `open_maps`
  - `schedule_reminder`
  - `play_audio`
  - `control_audio`
  - local call/message/health helpers already registered
- display agent state from participant attributes
- handle wake word / mic handoff

#### Go Backend

Continue to:

- authenticate users
- issue LiveKit tokens
- create room `user-{userID}`
- dispatch `aisee-agent`
- record billing and usage sessions
- execute trusted backend tool calls via `/api/internal/tool-call`

Minimal changes expected:

- Add env flag to configure agent implementation, if needed.
- Keep existing `/calls/token` response contract.

#### Python Agent Worker

Change from:

```python
AgentSession(
    llm=google.realtime.RealtimeModel(...)
)
```

to:

```python
LiveKitRoomBridge
  -> GeminiLiveSession
  -> ToolRouter
  -> AgentAudioPublisher
```

The worker still joins as LiveKit participant `aisee-agent`, but it no longer depends on LiveKit's Google realtime plugin for Gemini.

---

## Proposed Python Module Layout

Create a new bridge-oriented module structure under `aiseebackend/agent_worker`:

```text
agent_worker/
├── main.py                         # LiveKit worker entrypoint and feature flag
├── config.py                       # Env vars
├── prompts.py                      # Existing prompts
├── backend.py                      # Existing internal backend client
├── tools.py                        # Existing tool definitions, later adapted
├── gemini_bridge/
│   ├── __init__.py
│   ├── session.py                  # Google GenAI Live session wrapper
│   ├── livekit_io.py               # Subscribe/publish LiveKit audio/video
│   ├── audio.py                    # PCM conversion, resampling, frame pacing
│   ├── video.py                    # Video frame sampling and encoding
│   ├── tool_router.py              # Gemini function call -> backend/RPC call
│   ├── schemas.py                  # Tool declarations for Gemini
│   ├── state.py                    # session state, idle/ready/listening/speaking
│   └── errors.py                   # retryable/fatal error classification
└── tests/
    └── ...
```

Keep the existing LiveKit plugin path temporarily:

```text
agent_worker/
└── livekit_plugin_agent.py         # optional extraction of current logic
```

This allows `AI_ENGINE=livekit_plugin` rollback.

---

## Environment Variables

Add:

```bash
# Keeps rollback simple.
AI_ENGINE=google_sdk

# Existing.
GEMINI_MODEL=gemini-3.1-flash-live-preview
GEMINI_API_KEY=...

# Bridge tuning.
GEMINI_VOICE=Aoede
GEMINI_THINKING_LEVEL=low
GEMINI_AUDIO_SAMPLE_RATE=16000
GEMINI_RESPONSE_AUDIO_SAMPLE_RATE=24000
GEMINI_VIDEO_ENABLED=true
GEMINI_VIDEO_FPS_SPEAKING=1
GEMINI_VIDEO_FPS_IDLE=0.33
```

Default rollout:

```bash
AI_ENGINE=livekit_plugin
```

Enable new path only in staging or a test room first:

```bash
AI_ENGINE=google_sdk
```

---

## Gemini Live SDK Contract

Use the official Google GenAI SDK from Python:

- dependency already exists in lockfile as `google-genai`
- current lockfile shows `google-genai` is installed transitively through `livekit-plugins-google`
- pin it explicitly in `agent_worker/pyproject.toml` during implementation to make the dependency intentional

Expected Gemini 3.1 behavior:

- initial prompt/config goes into session setup
- realtime user text/audio/video should be sent through realtime input APIs
- do not use mid-session `send_client_content` for normal user turns
- do not rely on mid-session system instruction mutation
- use `thinkingLevel`, not `thinkingBudget`
- do not use affective dialog or proactive audio with Gemini 3.1
- async function calling is not supported; send tool response before continuing

---

## Audio Bridge Design

### Input: LiveKit User Audio to Gemini

The worker subscribes to the user's microphone track.

Pipeline:

```text
LiveKit AudioStream
  -> PCM frames
  -> normalize/sample-rate convert if needed
  -> Gemini realtime audio input
```

Implementation notes:

- LiveKit Python audio frames are typically PCM.
- Confirm actual sample rate/channels from `rtc.AudioFrame`.
- Gemini Live expects a supported PCM/audio payload format.
- Keep conversion isolated in `gemini_bridge/audio.py`.
- Avoid buffering too much audio; low latency matters more than perfect batching.
- Respect mic enable/disable from Flutter by naturally following published LiveKit audio.

### Output: Gemini Audio to LiveKit

The bridge creates a LiveKit local audio source/track and publishes it as agent audio.

Pipeline:

```text
Gemini server audio output
  -> decode/PCM normalize
  -> LiveKit AudioSource.capture_frame(...)
  -> published agent audio track
  -> Flutter hears assistant
```

Implementation notes:

- Preserve voice selection through `GEMINI_VOICE`.
- Match LiveKit audio source sample rate to Gemini output or resample once.
- Add backpressure protection:
  - if Gemini emits faster than playback, queue bounded frames
  - on interruption, clear queued assistant audio

---

## Turn Detection and Interruption

Initial migration should let Gemini 3.1 handle turn detection using native realtime activity detection.

Keep local Silero VAD only for AiSee's current idle/wake optimization:

- current worker closes Gemini session when `user_away_timeout` fires
- current worker can restart on speech via VAD
- new bridge may preserve this later, but first version can keep Gemini session open during an active room

Minimum viable behavior:

- user speaks
- Gemini responds
- if user starts speaking during Gemini response, interrupt assistant audio queue
- send Gemini the new audio stream continuously

Possible implementation:

- use LiveKit active speaker or VAD event to detect barge-in
- stop publishing queued assistant frames
- notify Gemini session if official SDK exposes interruption/activity signal

Do not overbuild this before the basic audio loop is stable.

---

## Video Bridge Design

Current LiveKit AgentSession uses:

```python
room_options=RoomOptions(video_input=True)
```

The custom bridge must replace this behavior.

Initial video behavior:

1. Keep camera off by default.
2. When Gemini calls `activate_camera`, worker calls Flutter RPC `enable_camera`.
3. Flutter publishes camera track.
4. Worker subscribes to video track.
5. Worker samples frames and sends them to Gemini realtime input.
6. When Gemini calls `deactivate_camera`, worker calls Flutter RPC `disable_camera`.

Frame sampling policy:

- one frame per second while user is actively speaking
- one frame every three seconds otherwise
- lower rate if latency or cost becomes a problem

Implementation notes:

- Keep frame sampler in `gemini_bridge/video.py`.
- Convert frames to a Gemini-supported image format.
- Avoid sending raw video at camera frame rate.
- Preserve Flutter-side NSFW content guard; do not remove app guard.

---

## Tool Calling Strategy

The current tool definitions live in `agent_worker/tools.py` as LiveKit `@function_tool` methods. The bridge needs equivalent Gemini function declarations plus a router.

### Tool Categories

#### Backend-only Tools

These call Go backend via `/api/internal/tool-call`:

- `make_voice_call`
- `make_video_call`
- `send_chat_message`
- `read_unread_messages`
- `set_medication_reminder`
- `list_medication_reminders`
- `mark_medication_taken`
- `mark_medication_skipped`
- `log_health_symptom`
- `log_health_metric`
- `set_appointment_reminder`
- `emergency_alert`
- `escalate_to_family`
- `escalate_to_volunteer`
- `rate_volunteer`
- music/search tools if present later in `tools.py`

#### Device/RPC Tools

These require calling the Flutter participant via LiveKit RPC:

- `activate_camera` -> `enable_camera`
- `deactivate_camera` -> `disable_camera`
- `get_location`
- `open_maps`
- `schedule_reminder`
- `play_audio`
- `control_audio`
- local call handoff methods

Some tools are hybrid:

- set medication reminder:
  - backend records reminder
  - backend may return `schedule_action`
  - worker calls Flutter `schedule_reminder`

### Router Design

Create:

```python
class ToolRouter:
    async def call(self, name: str, args: dict) -> dict:
        ...
```

Tool router dependencies:

- `BackendClient`
- `rtc.LocalParticipant`
- `user_context`
- current user identity

The router should return plain JSON-compatible dicts.

### Function Declarations

Create Gemini function declarations in `gemini_bridge/schemas.py`.

Guidelines:

- Keep names identical to current LiveKit tool names.
- Keep parameter names identical where possible.
- Keep descriptions Vietnamese-friendly but technically clear.
- Avoid complex schemas in first pass.
- Use required fields only where the app truly needs them.

### Tool Response

Gemini 3.1 Live does not support async function calling in the same way as older models. Treat tool calls as synchronous:

1. receive function call
2. call router
3. send tool response
4. let model continue

Add per-tool timeouts:

- normal backend tool: 10 seconds
- emergency/call tool: 20-35 seconds
- app RPC: 10-15 seconds

---

## Prompt and Context Strategy

Gemini 3.1 cannot rely on mid-session `update_instructions()` semantics through LiveKit.

Therefore:

- build full system prompt before opening Gemini session
- include user context at session creation
- do not mutate system prompt mid-session
- if user attributes change materially, reconnect Gemini session with new prompt
- use tools/state results for dynamic information instead of prompt mutation

Current prompt source:

- `agent_worker/prompts.py`
- `select_prompt(locale, wake_up, user_context)`

Recommended approach:

```python
instructions = select_prompt(locale, wake_up, user_context)
```

Pass this during Google Live session config.

If wake-up wording matters, reconnect with `wake_up=True` only when starting a new Gemini session after idle.

---

## Session Lifecycle

### First Version

Simplify lifecycle:

1. worker joins LiveKit room
2. bridge opens Gemini Live session
3. bridge stays open while room is active
4. bridge closes when room disconnects or duplicate agent is detected

### Later Optimization

Reintroduce idle close/resume:

- close Gemini session after user is away for 30 seconds
- keep LiveKit room alive
- detect new speech through Silero VAD
- start new Gemini session
- optionally use Gemini session resumption handle if official SDK supports it reliably for 3.1

Do not block first migration on session resumption.

---

## Agent State Contract

Flutter reads agent state from LiveKit participant attribute:

```text
lk.agent.state
```

Keep publishing this attribute.

Recommended states:

- `initializing`
- `idle`
- `listening`
- `thinking`
- `speaking`
- `disconnected`
- `error`

Mapping:

- LiveKit room connected, Gemini not ready: `initializing`
- Gemini ready, no active user speech: `idle`
- user audio flowing: `listening`
- waiting for Gemini response or tool: `thinking`
- publishing assistant audio: `speaking`
- room disconnect: `disconnected`
- unrecoverable Gemini error: `error`

---

## Rollout Plan

### Phase 0: Preserve Current Path

- Extract current LiveKit plugin implementation into a separate function/module if needed.
- Add `AI_ENGINE` env flag.
- Default `AI_ENGINE=livekit_plugin`.
- Add startup log showing selected engine.

Acceptance:

- Existing behavior unchanged when `AI_ENGINE=livekit_plugin`.

### Phase 1: Audio Spike

Implement only:

- LiveKit worker joins room.
- Google SDK opens Gemini 3.1 Live session.
- User microphone audio streams to Gemini.
- Gemini audio response publishes back to LiveKit.
- No tools.
- No camera.
- No idle resume.

Acceptance:

- User can speak Vietnamese to AiSee.
- Assistant replies in audio.
- No `1007 send_client_content` error appears.
- Flutter hears assistant through existing LiveKit room.

### Phase 2: Core Tool Calling

Implement:

- Gemini function declarations for the highest-value tools:
  - `activate_camera`
  - `deactivate_camera`
  - `get_location`
  - `make_voice_call`
  - `send_chat_message`
  - `set_medication_reminder`
  - `emergency_alert`
- `ToolRouter`
- synchronous tool response to Gemini

Acceptance:

- User can say "bật camera".
- Worker calls Flutter `enable_camera`.
- User can ask for location or SOS.
- Tool result is spoken back naturally.

### Phase 3: Video Input

Implement:

- LiveKit video track subscription.
- frame sampler.
- send sampled frames to Gemini.
- camera on/off flow controlled by tools.

Acceptance:

- User says "mô tả trước mặt tôi".
- Agent turns camera on.
- Gemini describes camera scene.
- Agent can turn camera off.

### Phase 4: Full Tool Parity

Port all current `tools.py` methods to Gemini declarations and `ToolRouter`.

Acceptance:

- Existing tool list has parity.
- No current user-facing voice capability is intentionally lost.

### Phase 5: Production Hardening

Implement:

- retry and reconnect on transient Gemini errors
- bounded audio queues
- interruption handling
- usage/billing session correctness
- metrics:
  - Gemini connect latency
  - first response latency
  - tool call success/failure
  - reconnect count
  - audio queue underrun/overrun
- structured logs with room/user redaction

Acceptance:

- Staging soak test passes.
- Rollback path verified.

---

## Testing Plan

### Unit Tests

Add tests for:

- tool schema generation
- tool router dispatch by name
- parameter validation
- backend tool timeout handling
- app RPC timeout handling
- prompt assembly with user context
- audio frame conversion helpers

### Integration Tests

Add test harnesses where possible:

- fake Gemini Live session object
- fake LiveKit audio source/sink
- fake local participant RPC
- backend client mock

Test cases:

- user audio frame is forwarded to Gemini session
- Gemini audio frame is published to LiveKit audio source
- Gemini tool call invokes router
- router returns tool response to Gemini
- camera tool calls expected RPC
- backend tool calls expected internal endpoint

### Manual Staging Tests

Run on a real Android device:

1. Open AiSee.
2. Wake assistant.
3. Speak Vietnamese normal conversation.
4. Interrupt while assistant is speaking.
5. Ask to turn camera on.
6. Ask "trước mặt tôi có gì".
7. Ask to turn camera off.
8. Ask for current location.
9. Ask to call a family member.
10. Ask to set medication reminder.
11. Trigger SOS only in a controlled test account.
12. Leave room and rejoin.
13. Keep room idle for more than 30 seconds if idle close is enabled.

### Regression Checks

Verify unchanged:

- `/api/v1/calls/token`
- LiveKit room join from Flutter
- family voice/video calls
- push notification call flow
- billing usage sessions
- local reminder scheduling
- foreground voice service
- wake word handoff

---

## Verification Commands

Backend:

```bash
cd /home/eddiesngu/Desktop/Dang/aisee/aiseebackend
go test ./...
```

Agent worker:

```bash
cd /home/eddiesngu/Desktop/Dang/aisee/aiseebackend/agent_worker
uv sync
uv run python -m pytest
```

Flutter:

```bash
cd /home/eddiesngu/Desktop/Dang/aisee/aiseeapp
flutter analyze
flutter test
```

Docker smoke:

```bash
cd /home/eddiesngu/Desktop/Dang/aisee/aiseebackend
docker compose build agent
docker compose up agent
```

Manual voice smoke is required before calling the migration complete.

---

## Rollback Plan

Rollback must be available at runtime.

Use:

```bash
AI_ENGINE=livekit_plugin
```

Rollback should:

- start the old `AgentSession` path
- use `livekit.plugins.google.RealtimeModel`
- ignore bridge modules
- require no Flutter app update
- require no Go backend update

Keep both paths for at least one release cycle after the bridge is stable.

---

## Risk Register

### Risk: Audio Format Mismatch

Impact: no audio, distorted audio, high latency.

Mitigation:

- isolate conversion in `audio.py`
- log sample rate/channels at startup
- build a small local conversion test
- publish known tone/audio sample through LiveKit if needed

### Risk: Interruption Feels Worse Than LiveKit AgentSession

Impact: assistant talks over user.

Mitigation:

- first clear assistant output queue on user speech start
- later send official interruption/activity signal if SDK supports it
- use LiveKit active speaker events or Silero VAD

### Risk: Tool Schemas Diverge From Existing Tools

Impact: Gemini calls invalid arguments.

Mitigation:

- keep function names and parameters close to `tools.py`
- unit test every schema against router
- return helpful JSON errors instead of throwing raw exceptions

### Risk: Camera Costs or Latency Increase

Impact: expensive sessions, slow responses.

Mitigation:

- sample frames, do not stream full FPS
- default to camera off
- preserve app-side content guard

### Risk: Session Resumption Is Not Reliable

Impact: context loss after idle reconnect.

Mitigation:

- do not depend on resumption in first version
- summarize recent context in app/server state if needed
- reconnect with full current user context

### Risk: Hidden LiveKit Agents Convenience Loss

Impact: current framework did more than expected.

Mitigation:

- phase rollout
- manually compare with old behavior
- keep old path behind `AI_ENGINE`

---

## Implementation Checklist

### Preparation

- [ ] Add explicit `google-genai` dependency to `agent_worker/pyproject.toml`.
- [ ] Add `AI_ENGINE` and bridge env vars to `config.py`.
- [ ] Add startup logging for selected engine.
- [ ] Preserve old engine path for rollback.

### Bridge Skeleton

- [ ] Create `gemini_bridge/session.py`.
- [ ] Create `gemini_bridge/livekit_io.py`.
- [ ] Create `gemini_bridge/audio.py`.
- [ ] Create `gemini_bridge/tool_router.py`.
- [ ] Create `gemini_bridge/schemas.py`.
- [ ] Create `gemini_bridge/state.py`.

### Audio MVP

- [ ] Subscribe to first non-agent participant audio track.
- [ ] Forward audio frames to Gemini.
- [ ] Receive Gemini audio frames.
- [ ] Publish assistant audio track to LiveKit.
- [ ] Publish `lk.agent.state`.
- [ ] Handle room disconnect and duplicate agent exit.

### Tool MVP

- [ ] Declare top-priority Gemini tools.
- [ ] Implement router for backend-only tools.
- [ ] Implement router for Flutter RPC tools.
- [ ] Send synchronous function responses back to Gemini.
- [ ] Add timeout and error wrapping.

### Video

- [ ] Implement camera activation via Flutter RPC.
- [ ] Subscribe to camera video track.
- [ ] Sample frames.
- [ ] Send frames to Gemini.
- [ ] Implement camera deactivation.

### Hardening

- [ ] Add retry/reconnect policy.
- [ ] Add audio queue bounds.
- [ ] Add interruption behavior.
- [ ] Add metrics/logging.
- [ ] Add tests.
- [ ] Run staging manual QA.

---

## Suggested First Pull Request

Keep the first PR small:

Title:

```text
Add feature-flagged Gemini SDK bridge skeleton
```

Scope:

- `config.py`: add `AI_ENGINE`
- `main.py`: branch on `AI_ENGINE`
- new `gemini_bridge/` package with skeleton classes
- no behavior change when flag is off
- unit tests for config selection

Do not include full audio bridge in the first PR if it makes review too large.

---

## Suggested Commit Message

Use the repo's Lore commit protocol:

```text
Enable a controlled path away from LiveKit's Gemini plugin

Gemini 3.1 Live rejects post-turn client content in places the
LiveKit Google plugin still abstracts over, so the agent worker needs
a feature-flagged official-SDK bridge while preserving the existing
LiveKit transport and rollback path.

Constraint: Flutter and Go backend depend on LiveKit room/RPC contracts
Rejected: Rewrite Flutter to call Gemini directly | too much media, auth, and tool transport risk for first migration
Rejected: Downgrade to Gemini 2.5 | product testing found more model errors
Confidence: medium
Scope-risk: moderate
Directive: Keep AI_ENGINE=livekit_plugin working until the SDK bridge has passed real-device voice and camera QA
Tested: go test ./...; flutter analyze; manual voice smoke
Not-tested: production Gemini 3.1 soak
```

---

## Open Questions

Resolve during Phase 1 spike:

- Exact audio sample rate emitted by LiveKit Python `AudioStream` in the current deployment.
- Exact audio sample rate and MIME format expected by current Google GenAI Python Live SDK version.
- Whether Gemini 3.1 Live session resumption handles are stable enough for AiSee idle resume.
- Whether the official SDK exposes a clean interruption signal or whether clearing local audio output is enough.
- Whether video frames should be sent as JPEG, raw frame data, or SDK-native media objects in the selected SDK version.

---

## Final Recommendation

Proceed with the Google SDK bridge in `agent_worker`, not a full app rewrite.

The first success milestone is not full feature parity. It is:

```text
Flutter LiveKit mic -> Python bridge -> Gemini 3.1 -> Python bridge -> LiveKit audio -> Flutter speaker
```

Once that loop is stable and free of the Gemini 3.1 `send_client_content` issue, port tools and camera incrementally.
