# Gemini Live API Optimizations for Demo

**Date:** 2026-03-30
**Goal:** Optimize agent_worker for Phase 4 demo using Gemini 3.1 Flash Live capabilities.
**Model:** `gemini-3.1-flash-live-preview`

## Scope

Five optimizations, ordered by impact:

1. User context injection into system prompt
2. Video-on-demand (audio-only by default)
3. Session resumption via native `SessionResumptionConfig`
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

Start sessions in audio-only mode. When camera is toggled, restart the session with the new video setting. Native session resumption (Section 3) preserves context across restarts.

**Note on 2-minute limit:** The `livekit-plugins-google` plugin has built-in session resumption that may transparently handle session timeouts via reconnect. The primary motivation for video-on-demand is still valid: avoid sending unnecessary video frames (token cost) and extend the session window. Video toggle will lose conversation context until Section 3 (session resumption) is implemented.

**State tracking:** Add an entrypoint-scoped `video_enabled: bool = False` closure variable (alongside `active_session`, `first_start`, etc.).

**Session start:** `_start_session()` reads `video_enabled` to set `RoomOptions(video_input=video_enabled)`.

**Camera toggle flow:**
1. `activate_camera` tool: RPC to enable camera on client → set `video_enabled = True` → call `_restart_session()`
2. `deactivate_camera` tool: RPC to disable camera on client → set `video_enabled = False` → call `_restart_session()`

**`_restart_session()`:** New function that:
1. Calls `_stop_session()`
2. Calls `_start_session()` — which picks up `video_enabled` and uses session resumption handle (Section 3) to preserve context

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

Use the native `SessionResumptionConfig` built into `livekit-plugins-google`. The plugin already supports:
- `RealtimeModel` accepts a `session_resumption` parameter
- `RealtimeSession` tracks a `session_resumption_handle` property
- On reconnect, the plugin passes the handle in `_build_connect_config`
- The server sends `session_resumption_update` with new handles

**Implementation:**

1. Pass `session_resumption=types.SessionResumptionConfig()` to `RealtimeModel` constructor
2. Before `_stop_session()`, capture the current `session_resumption_handle` from the active session
3. Store the handle in an entrypoint-scoped `last_resumption_handle: str | None` variable
4. On `_start_session()`, pass the stored handle to the new `RealtimeModel` via `SessionResumptionConfig(handle=last_resumption_handle)`

**Advantages over custom summary approach:**
- Preserves full conversation state (not a lossy 2-3 sentence summary)
- No need to access private `send_client_content` API
- No need to request a text summary from a voice session (which would speak it aloud)
- Zero custom code beyond passing config

**Edge cases:**
- User away 30s → returns → new session with resumption handle → full context restored
- Video toggle → session restart → handle preserves continuity
- App disconnect → handle lost (in-memory only) → acceptable for demo
- Handle expired or invalid → Gemini starts fresh session, no error

**Caveat:** When video mode changes (Section 2), the model config changes. The resumption handle may not work across config changes. If it fails, the session starts fresh — acceptable degradation. Verify during implementation.

### Changes

- `main.py`: Add `last_resumption_handle` variable, capture handle before `_stop_session()`, pass handle to `RealtimeModel` in `_start_session()`. Need to verify how to access the handle from `AgentSession` → `RealtimeSession` → `session_resumption_handle` property.

---

## 4. VAD Sensitivity Tuning

### Problem

Gemini's built-in VAD runs with default sensitivity. Users with low vision may speak more slowly or hesitantly — default settings may cut them off prematurely.

### Design

Configure Gemini RealtimeModel VAD via `realtime_input_config` parameter, which the plugin passes through to the Gemini API:

```python
realtime_input_config=types.RealtimeInputConfig(
    automatic_activity_detection=types.AutomaticActivityDetection(
        start_of_speech_sensitivity=types.StartSensitivity.START_SENSITIVITY_HIGH,
        end_of_speech_sensitivity=types.EndSensitivity.END_SENSITIVITY_LOW,
        prefix_padding_ms=100,
        silence_duration_ms=500,
    ),
)
```

- `END_SENSITIVITY_LOW` — wait longer before treating silence as end-of-speech (helps hesitant speakers)
- `START_SENSITIVITY_HIGH` — detect user speech quickly to interrupt agent
- `prefix_padding_ms: 100` — capture extra audio at start of utterance (avoid missing first word)
- `silence_duration_ms: 500` — require 500ms silence to mark end-of-speech (default ~200ms)

These are Gemini API parameters, independent of the Silero VAD used for lazy session startup. Both systems coexist: Silero detects speech to trigger session creation; Gemini VAD handles turn-taking within the session. Review Silero VAD settings for consistency — if Silero is more aggressive than Gemini, it could trigger a new session start while a user is still mid-utterance.

### Changes

- `main.py`: Pass `realtime_input_config` to `google.realtime.RealtimeModel()` constructor

---

## 5. thinkingLevel

### Problem

Default `thinkingLevel` is unspecified (model-dependent behavior). For demo quality, image descriptions and multi-step emergency handling benefit from explicit reasoning.

### Design

Set thinking config to `LOW` in RealtimeModel:

```python
thinking_config=types.ThinkingConfig(
    thinking_level=types.ThinkingLevel.LOW,
)
```

This adds a small latency cost but improves response quality for:
- Camera scene descriptions (safety-first, spatial reasoning)
- Multi-step emergency escalation logic
- Complex tool selection (which contact to call, which escalation level)

Not using `MEDIUM` or `HIGH` — latency too high for real-time voice. Easy to revert if too slow.

### Changes

- `main.py`: Add `thinking_config` to `google.realtime.RealtimeModel()` constructor

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
| Session resumption | Medium — native API supported, handle may not survive config changes | Graceful fallback: fresh session if handle invalid |
| VAD tuning | Low — params verified as supported by plugin | Fallback: use defaults if model rejects |
| thinkingLevel | Low — config only | Easy to revert to minimal if too slow |
