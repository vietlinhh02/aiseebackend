# Gemini Live API Optimizations for Demo

**Date:** 2026-03-30
**Goal:** Optimize agent_worker for Phase 4 demo using Gemini 3.1 Flash Live capabilities.
**Model:** `gemini-3.1-flash-live-preview`

## Scope

Five optimizations, ordered by impact:

1. User context injection into system prompt
2. Video-on-demand (audio-only by default)
3. Session resumption via `send_client_content`
4. VAD sensitivity tuning
5. thinkingLevel = `low`

**Out of scope:** audio transcription, affective dialog, proactive audio, async function calling (all require Gemini 2.5), mediaResolution LOW (need high-res for reading labels/signs).

---

## 1. User Context Injection

### Problem

`user_context` dict (`user_name`, `accessibility`, `has_family_link`) is available in `AiSeeAgent._uctx` but never passed to Gemini. The model cannot address users by name or adapt behavior to their accessibility level or family link status.

### Design

Modify `select_prompt()` in `prompts.py` to accept `user_context: dict` and append a context block to the system prompt:

**Vietnamese:**
```
=== THÔNG TIN NGƯỜI DÙNG ===
- Tên: {user_name}
- Khả năng thị lực: {accessibility}
- Đã liên kết người thân: {has_family_link}

Dùng tên người dùng khi trò chuyện. Điều chỉnh mô tả theo khả năng thị lực.
Nếu chưa liên kết người thân, không đề xuất gọi người thân hay SOS.
```

**English:**
```
=== USER INFO ===
- Name: {user_name}
- Vision level: {accessibility}
- Family linked: {has_family_link}

Use the user's name in conversation. Adjust descriptions to their vision level.
If no family is linked, do not suggest calling family or SOS.
```

### Changes

- `prompts.py`: `select_prompt(locale, wake_up, user_context)` — append user context block
- `tools.py`: `AiSeeAgent.__init__` — pass `user_context` to `select_prompt()`

---

## 2. Video-on-Demand

### Problem

`RoomOptions(video_input=True)` is always set at session start. Per Gemini Live API docs, audio+video sessions are limited to 2 minutes; audio-only sessions get 15 minutes. Most interactions are audio-only — camera is activated on demand via `activate_camera` tool.

### Design

Start sessions in audio-only mode. When camera is toggled, restart the session with the new video setting. Session resumption (Section 3) preserves context across restarts.

**State tracking:** Add a module-level `video_enabled: bool = False` flag in the entrypoint scope (alongside `active_session`, `first_start`, etc.).

**Session start:** `_start_session()` reads `video_enabled` to set `RoomOptions(video_input=video_enabled)`.

**Camera toggle flow:**
1. `activate_camera` tool: RPC to enable camera on client → set `video_enabled = True` → call `_restart_session()`
2. `deactivate_camera` tool: RPC to disable camera on client → set `video_enabled = False` → call `_restart_session()`

**`_restart_session()`:** New function that:
1. Requests summary from current session (Section 3)
2. Calls `_stop_session()`
3. Calls `_start_session()` — which picks up `video_enabled` and seeds summary

**Tool changes:** `activate_camera` and `deactivate_camera` need access to the entrypoint's `video_enabled` flag and `_restart_session()`. Two options:
- Pass a callback to `AiSeeAgent` that tools can call
- Store a mutable state object in `AiSeeAgent._uctx`

Recommended: Add a `_session_ctrl` dict to `AiSeeAgent` with a `restart_with_video(enabled: bool)` async callback set by entrypoint.

### Changes

- `main.py`: Add `video_enabled` flag, `_restart_session()` function, pass session control callback to agent
- `tools.py`: `activate_camera` / `deactivate_camera` call `self._session_ctrl['restart_with_video']` after RPC

---

## 3. Session Resumption

### Problem

When a session closes (user away timeout, video toggle), all conversation context is lost. Users must repeat themselves.

### Design

**Save summary on stop:**
- Before `active_session.aclose()`, send a prompt to the model asking for a 2-3 sentence summary of the conversation
- Store the summary text in `last_session_summary: str` (in-memory, entrypoint scope)
- Timeout: 3 seconds. If model doesn't respond in time, proceed without summary
- Summary prompt: `"Summarize our conversation so far in 2-3 sentences for context continuity."`

**Seed summary on start:**
- Gemini 3.1 supports `send_client_content` for initial context seeding (only at session start)
- In `_start_session()`, if `last_session_summary` is non-empty, send it via `send_client_content` with role `user` and prefix: `"[Tóm tắt cuộc trò chuyện trước: {summary}]"` (or English equivalent based on locale)
- Clear `last_session_summary` after seeding

**Edge cases:**
- User away 30s → returns → new session has summary → agent remembers context
- Video toggle → session restart → summary preserves continuity
- App disconnect → summary lost (in-memory only) → acceptable for demo
- Summary request times out → session stops normally, no summary → next session starts fresh

### Changes

- `main.py`: Add `last_session_summary` variable, `_request_summary()` async function (with 3s timeout), modify `_stop_session()` to call it before close, modify `_start_session()` to seed summary via `send_client_content`

### Open Question

LiveKit's `AgentSession` may not expose a direct `send_client_content` method. The actual API depends on how `livekit-plugins-google` wraps the Gemini Live API. Need to verify during implementation whether:
- `AgentSession` exposes `send_client_content` directly
- Or we need to access the underlying `google.realtime.RealtimeSession` to call it

Similarly, requesting a summary before close requires sending a text message to the model and waiting for a response — need to verify the API for this.

---

## 4. VAD Sensitivity Tuning

### Problem

Gemini's built-in VAD runs with default sensitivity. Users with low vision may speak more slowly or hesitantly — default settings may cut them off prematurely.

### Design

Configure Gemini RealtimeModel VAD parameters:
- `end_of_speech_sensitivity: LOW` — wait longer before treating silence as end-of-speech
- `start_of_speech_sensitivity: HIGH` — detect user speech quickly to interrupt agent
- `prefix_padding_ms: 100` — capture extra audio at start of utterance
- `silence_duration_ms: 500` — require 500ms silence to mark end-of-speech

These are Gemini API parameters, independent of the Silero VAD used for lazy session startup. Both systems coexist: Silero detects speech to trigger session creation; Gemini VAD handles turn-taking within the session.

### Changes

- `main.py`: Pass VAD config to `google.realtime.RealtimeModel()` constructor

### Open Question

Need to verify how `livekit-plugins-google` exposes Gemini VAD configuration. The params may be passed as a dict, a config object, or may not be exposed at all (requiring a plugin update or direct API call).

---

## 5. thinkingLevel

### Problem

Default `thinkingLevel` is `minimal` — lowest latency but basic reasoning. For demo quality, image descriptions and multi-step emergency handling benefit from more reasoning.

### Design

Set `thinkingLevel: "low"` in RealtimeModel config. This adds a small latency cost but improves response quality for:
- Camera scene descriptions (safety-first, spatial reasoning)
- Multi-step emergency escalation logic
- Complex tool selection (which contact to call, which escalation level)

Not using `medium` or `high` — latency too high for real-time voice.

### Changes

- `main.py`: Add `thinking_level="low"` to `google.realtime.RealtimeModel()` constructor

---

## Implementation Order

1. **User context injection** — standalone, no dependencies, fixes a real bug
2. **thinkingLevel** — one-line config change
3. **VAD sensitivity** — config change, may need API verification
4. **Video-on-demand** — requires session restart logic
5. **Session resumption** — depends on video-on-demand (restart triggers summary), most complex

Items 1-3 can be done in parallel. Item 4 before 5.

---

## Risk Assessment

| Item | Risk | Mitigation |
|------|------|------------|
| User context injection | Low — prompt change only | Test with/without user_name set |
| Video-on-demand | Medium — session restart may cause audio gap | Buffer audio during restart (existing pattern) |
| Session resumption | High — depends on unverified LiveKit/Gemini API | Graceful fallback: skip if API unavailable |
| VAD tuning | Low-Medium — params may not be exposed by plugin | Fallback: use defaults |
| thinkingLevel | Low — config only | Easy to revert to minimal if too slow |
