# LiveKit Optimization: Agent State, User Attributes, RPC, Animated Orb

**Date:** 2026-03-29
**Scope:** Backend agent worker (Python) + Frontend Flutter app + Go server (minor)
**Goal:** Optimize LiveKit integration using official SDK features — replace custom data channel protocol with RPC, add agent state publishing, user attributes, and animated orb avatar.

---

## 1. Agent State Publishing

### Problem

Frontend has no visibility into what the agent is doing. UI shows only "Connected" or "Connecting" — user does not know when to speak or when to wait.

### Solution

Two-layer approach:

1. **AgentSession auto-publishes `lk.agent.state`** — when the inner `active_session` is running (Gemini live), the LiveKit agents framework automatically publishes `listening`, `thinking`, `speaking` states.
2. **Agent manually publishes `lk.agent.state`** during periods when no AgentSession is active (idle timeout, pre-first-speech, shutdown) — using `local_participant.set_attributes()`.

This avoids fighting the framework while covering the gaps.

### Agent States

| State | When | Source |
|-------|------|--------|
| `initializing` | Agent joins room, before first speech | Manual publish |
| `idle` | No active AgentSession (before first speech, after idle timeout) | Manual publish |
| `listening` | VAD detects user speech, AgentSession active | Framework auto |
| `thinking` | LLM processing or tool call in progress | Framework auto |
| `speaking` | TTS audio playing | Framework auto |
| `disconnected` | Session ended cleanly | Manual publish (before room disconnect) |

### Backend Changes (agent_worker/main.py)

```python
async def _publish_state(local_participant, state: str):
    await local_participant.set_attributes({'lk.agent.state': state})
```

Publish manually at:
- `entrypoint()` after connect → `initializing`
- After duplicate check passes → `idle`
- `_stop_session()` first line (before `aclose()`) → `idle`
- Before `disconnect_event`/`duplicate_event` cleanup → `disconnected`

Framework handles `listening`, `thinking`, `speaking` automatically when `active_session` is running.

### Frontend Changes (Flutter)

**aisee_livekit_voice_session.dart:**
- Add `ValueNotifier<AgentState> agentState = ValueNotifier(AgentState.connecting)`

**New enum `AgentState`** (in a models file or inline):

```dart
enum AgentState {
  connecting,
  initializing,
  idle,
  listening,
  thinking,
  speaking,
  disconnected,
  failed;

  static AgentState fromString(String? s) => switch (s) {
    'initializing' => initializing,
    'idle' => idle,
    'listening' => listening,
    'thinking' => thinking,
    'speaking' => speaking,
    'disconnected' => disconnected,
    _ => connecting,
  };

  bool get canListen => this == listening || this == thinking || this == speaking;
  bool get isConnected => this == listening || this == thinking || this == speaking;
  bool get isPending => this == connecting || this == initializing || this == idle;
  bool get isFinished => this == disconnected || this == failed;
}
```

In room listener:

```dart
..on<ParticipantAttributesChanged>((event) {
  if (event.participant is RemoteParticipant) {
    final raw = event.participant.attributes['lk.agent.state'];
    agentState.value = AgentState.fromString(raw);
  }
})
```

**user_assistant_tab.dart:**
- Receive `ValueNotifier<AgentState> agentState`
- Display: `idle` → "San sang", `listening` → "Dang nghe...", `thinking` → "Dang xu ly...", `speaking` → "Dang tra loi..."

### Go Server Changes

None.

---

## 2. User Attributes + Locale-driven Prompt

### Problem

Agent has no context about the user when session starts. Must ask or hardcode assumptions. System prompt is Vietnamese-only, blocking internationalization.

### Solution

Frontend publishes user attributes after joining room. Agent listens for `participant_attributes_changed` event to read them (handles race condition where agent connects before user sets attributes). System prompt uses locale-specific variants.

### Attributes

| Key | Value | Source |
|-----|-------|--------|
| `user_name` | Display name | Google profile / AiseeProfile |
| `user_id` | UUID | Session store |
| `locale` | e.g. `vi_VN`, `en_US` | `Platform.localeName` |
| `accessibility` | `low_vision` | Hardcoded (current user type) |
| `has_family_link` | `true` / `false` | Profile API |

### Go Server Changes (pkg/livekit/client.go)

Add `CanUpdateOwnMetadata` to video grant in `CreateToken()`:

```go
canUpdate := true
grant := &auth.VideoGrant{
    RoomJoin:             true,
    Room:                 roomName,
    CanUpdateOwnMetadata: &canUpdate,
}
```

### Frontend Changes (Flutter)

**aisee_livekit_voice_session.dart:**
- `connect()` receives additional params: `userName`, `userId`, `hasFamilyLink`
- Ordering after `room.connect()`:
  1. Register RPC methods (so agent can call them immediately)
  2. Publish user attributes
  3. Enable microphone

```dart
await room.localParticipant?.setAttributes({
  'user_name': userName,
  'user_id': userId,
  'locale': Platform.localeName,
  'accessibility': 'low_vision',
  'has_family_link': hasFamilyLink ? 'true' : 'false',
});
```

**user_home_screen.dart:**
- Pass user data to `connect()` call
- Remove `_liveKitDataLine` state, `liveKitPayloadDisplayText` import, `onDataUtf8` callback

### Backend Changes (agent_worker/main.py)

**Listen for attributes (handles race condition):**

```python
user_context = {'user_id': user_id, 'locale': 'vi', 'user_name': ''}

@ctx.room.on("participant_attributes_changed")
def _on_user_attributes(changed_attributes, participant):
    if participant.kind == rtc.ParticipantKind.PARTICIPANT_KIND_AGENT:
        return
    for key in ('user_name', 'user_id', 'locale', 'accessibility', 'has_family_link'):
        val = participant.attributes.get(key)
        if val:
            user_context[key] = val
    logger.info("user attributes updated: %s", user_context)
```

Also read attributes once at startup from existing remote participants (covers case where user connected first).

**System prompt refactor — locale-specific variants:**

Keep the current Vietnamese prompt as-is for `vi` locales (preserves cultural nuances like number-reading conventions). Add an English translation for `en` locales. Select based on `user_context['locale']`:

```python
def _select_prompt(locale: str, wake_up: bool) -> str:
    base = SYSTEM_PROMPT_VI if locale.startswith('vi') else SYSTEM_PROMPT_EN
    suffix = WAKEUP_SUFFIX if wake_up else GREETING_SUFFIX
    return base + suffix
```

Both prompt variants contain identical behavior rules, just in different languages. This preserves natural phrasing per locale rather than relying on LLM translation.

---

## 3. RPC for App Tools

### Problem

Custom `tool_action` / `tool_action_ack` JSON protocol over raw data packets. No built-in timeout, no standard error codes, ~280 lines of manual dispatch/parse/ack code on frontend, ~112 lines of `DataChannelBridge` on agent.

### Solution

Replace with LiveKit RPC. Agent calls `perform_rpc()`, frontend registers handlers via `registerRpcMethod()`. Built-in timeout, error codes, structured error handling.

### RPC Methods (7 total)

| Method | Payload (JSON) | Success Response (JSON) | Timeout |
|--------|---------------|------------------------|---------|
| `enable_camera` | `{}` | `{"status":"ok"}` | 10s (default) |
| `disable_camera` | `{}` | `{"status":"ok"}` | 10s |
| `play_audio` | `{"url":"...","title":"..."}` | `{"status":"ok"}` | 10s |
| `control_audio` | `{"command":"pause\|resume\|stop"}` | `{"status":"ok"}` | 10s |
| `get_location` | `{}` | `{"lat":..,"lng":..,"address":".."}` | **15s** (cold GPS fix) |
| `open_maps` | `{"destination":".."}` or `{"lat":..,"lng":..}` | `{"status":"ok"}` | 10s |
| `schedule_reminder` | `{"type":"medication\|appointment","title":"...","times":[...],"datetime":"..."}` | `{"status":"ok"}` | 10s |

### `_call_and_forward` Migration

Three tools use backend HTTP + forward-to-app pattern:

| Tool | Current flow | New flow |
|------|-------------|----------|
| `play_music` | Backend returns `play_audio_action` → `send_fire_and_forget` | Backend returns audio URL → agent calls `perform_rpc('play_audio', ...)` |
| `set_medication_reminder` | Backend returns `schedule_reminder_action` → `send_fire_and_forget` | Backend returns result → agent calls `perform_rpc('schedule_reminder', ...)` |
| `set_appointment_reminder` | Same as above | Same as above |

The `_call_and_forward` helper is replaced by explicit two-step logic in each tool:

```python
@function_tool()
async def play_music(query: str) -> dict:
    result = await _backend.tool_call("play_music", _user_id, {"query": query})
    play_action = result.pop("play_audio_action", None)
    if play_action:
        await _local_participant.perform_rpc(
            destination_identity=_user_identity,
            method="play_audio",
            payload=json.dumps(play_action["params"]),
        )
    return result
```

### `control_music` — was fire-and-forget, now RPC

Current code uses `send_fire_and_forget` for `control_audio`. With RPC, the agent will wait for a response. This is acceptable because:
- Flutter handler returns immediately (no async work beyond `player.pause()`)
- Default 10s timeout is more than enough
- Agent gets confirmation the command succeeded (better than blind fire-and-forget)

### Backend Changes (agent_worker)

**Delete:** `data_channel.py` entirely.

**tools.py:**
- Replace all `bridge.send_action()` → `_local_participant.perform_rpc()`
- Replace all `bridge.send_fire_and_forget()` → `_local_participant.perform_rpc()`
- Replace `_call_and_forward()` with explicit two-step logic per tool
- `init_tools()` receives `user_identity` (string) instead of bridge
- For `get_location` and `emergency_alert`: set `response_timeout=15.0`

```python
response = await _local_participant.perform_rpc(
    destination_identity=_user_identity,
    method='get_location',
    payload='{}',
    response_timeout=15.0,
)
```

**main.py:**
- Remove `DataChannelBridge` import, instantiation, and `data_received` handler
- Remove `on_data` room event handler
- Pass `user_identity` to `init_tools()` instead of bridge

### Frontend Changes (Flutter)

**aisee_livekit_voice_session.dart:**

Register 7 methods **after** `room.connect()` succeeds (ensures room state is ready, handlers can access `_room?.localParticipant`):

```dart
room.registerRpcMethod('enable_camera', (data) async {
    await _handleEnableCamera();
    return jsonEncode({'status': 'ok'});
});
room.registerRpcMethod('schedule_reminder', (data) async {
    final params = jsonDecode(data.payload) as Map<String, dynamic>;
    await _handleScheduleReminder(params);
    return jsonEncode({'status': 'ok'});
});
// ... same pattern for other 5 methods
```

Each handler has null-safety guards for `_room`, `_musicPlayer`, etc.

**Delete:**
- `_handleToolActionPayload()` — entire dispatch switch
- `_sendToolActionAck()`
- `_unwrapPayload()`, `_pickRequestId()`, `_onDataPayload()`
- `onDataUtf8` callback parameter from `connect()`
- `DataReceivedEvent` listener

**Keep:** Individual handler logic (camera enable, audio play, etc.) extracted into standalone `_handle*` methods.

**Also delete:**
- `livekit_payload_display.dart` (no more raw payloads to display)

### Go Server Changes

None.

---

## 4. Animated Orb Avatar

### Problem

Current UI shows only waveform bars. No visual representation of the assistant. User has limited feedback about agent presence.

### Solution

Replace `AssistantVoiceWaveform` with an animated orb (`CustomPainter` + `AnimationController`). Orb reacts to agent state and audio energy. Chartreuse accent on dark background. No external dependencies.

### Orb Behavior by State

| Agent State | Animation | Color | Scale |
|-------------|-----------|-------|-------|
| `connecting` / `initializing` | Slow rotate + fade pulse | Chartreuse 30%-50% | 1.0 |
| `idle` | Gentle breathe (2s cycle) | Chartreuse 50% | 1.0 |
| `listening` | Faster pulse (1s cycle), react to user mic | Chartreuse 70% | 1.0-1.05 |
| `thinking` | Ripple outward effect | Chartreuse-White gradient | 1.0-1.02 |
| `speaking` | Scale with `assistantWaveEnergy` (0-1 = 1.0-1.3) | Chartreuse 100% | 1.0-1.3 |
| `disconnected` / `failed` | Fade out | Gray `#5D5D5D` | 0.95 |

### Design Details

- **Glow:** Outer shadow blur 20-40px, same color as orb, creates glow on `#060913` background
- **Size:** ~120-160dp diameter, centered in assistant tab
- **Gradient:** Radial gradient from center (bright) to edge (darker)

### Implementation

**New widget: `AssistantOrb` (StatefulWidget)**
- Input: `ValueNotifier<AgentState> agentState`, `ValueNotifier<double> assistantWaveEnergy`
- 2 `AnimationController`: pulse (continuous) + energy scale (driven by notifier)
- `CustomPainter` draws: filled circle with radial gradient + outer glow shadow
- `shouldRepaint()` compares previous energy + state values — skip repaint when both unchanged
- Wrapped in `RepaintBoundary` for paint isolation

**Performance:**
- `shouldRepaint` on CustomPainter compares `oldEnergy`, `oldState` fields
- No `BackdropFilter` or expensive blur
- Only gradient + box shadow (GPU-friendly)

### Files Changed

- **New:** `lib/widgets/assistant_orb.dart`
- **Modified:** `user_assistant_tab.dart` — replace `AssistantVoiceWaveform` with `AssistantOrb`
- **Delete:** `assistant_voice_waveform.dart` (replaced, not deprecated)

### Frontend Only

No backend or Go server changes.

---

## Summary of Changes by Component

### Agent Worker (Python)

| File | Action |
|------|--------|
| `main.py` | Add `_publish_state()` for idle/init/disconnected, listen for user attributes, refactor prompt to locale-specific variants, remove DataChannelBridge + data_received handler |
| `tools.py` | Replace `bridge.send_action()`/`send_fire_and_forget()` with `perform_rpc()`, replace `_call_and_forward()` with explicit two-step, add `schedule_reminder` RPC, use `user_identity` param |
| `data_channel.py` | **Delete entirely** |
| `config.py` | No changes |
| `backend.py` | No changes |

### Go Server

| File | Action |
|------|--------|
| `pkg/livekit/client.go` | Add `CanUpdateOwnMetadata: &boolTrue` to token grant (pointer to bool) |

### Flutter App

| File | Action |
|------|--------|
| `aisee_livekit_voice_session.dart` | Add `AgentState` enum + notifier, register 7 RPC methods after connect, publish user attributes, remove all data channel dispatch/ack code, remove `onDataUtf8` param |
| `user_home_screen.dart` | Pass user data to connect(), receive agentState, remove `_liveKitDataLine` state + `liveKitPayloadDisplayText` import |
| `user_assistant_tab.dart` | Receive agentState, replace waveform with orb, show state text, remove `liveKitDataLine` prop |
| `assistant_orb.dart` | **New** — animated orb widget |
| `assistant_voice_waveform.dart` | **Delete** (replaced by orb) |
| `livekit_payload_display.dart` | **Delete** (no more raw data channel payloads) |

---

## Dependencies

No new dependencies required. All features use existing `livekit_client ^2.7.0` (Flutter) and `livekit-agents >=1.0.0` (Python) capabilities.

---

## Migration Notes

### Deploy Order

1. **Go server** — token grant update (`CanUpdateOwnMetadata`). Must deploy first; Flutter app needs this permission before it can set attributes.
2. **Agent worker + Flutter app** — simultaneous deploy. RPC protocol is incompatible with old data channel protocol.

### Coordinated Deploy Risk

Old Flutter app (data channel) + new agent (RPC) = broken. Mitigation options:
- **Forced app update** — require minimum app version before connecting
- **Feature flag via user attribute** — agent checks `user.attributes.get('rpc_version')`, falls back to data channel if absent. Adds complexity; only needed if gradual rollout is required.

Recommended: **forced app update** for simplicity. AiSee is early-stage; all users can update simultaneously.

### Backward Compatibility

None maintained. Old data channel protocol is removed entirely per project principle "Replace, don't deprecate."
