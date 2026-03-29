# LiveKit Optimization: Agent State, User Attributes, RPC, Animated Orb

**Date:** 2026-03-29
**Scope:** Backend agent worker (Python) + Frontend Flutter app + Go server (minor)
**Goal:** Optimize LiveKit integration using official SDK features — replace custom data channel protocol with RPC, add agent state publishing, user attributes, and animated orb avatar.

---

## 1. Agent State Publishing

### Problem

Frontend has no visibility into what the agent is doing. UI shows only "Connected" or "Connecting" — user does not know when to speak or when to wait.

### Solution

Agent publishes `lk.agent.state` participant attribute at each lifecycle transition. Frontend listens for attribute changes and updates UI accordingly.

### Agent States

| State | When | Published at |
|-------|------|-------------|
| `initializing` | Agent joins room, setting up | `entrypoint()` start |
| `idle` | Waiting for user input | After greeting, after TTS finishes |
| `listening` | VAD detects user speech | `_on_vad` speech start |
| `thinking` | Processing tool call or LLM generating | Tool call start, LLM inference start |
| `speaking` | TTS audio playing | TTS audio start |
| `disconnected` | Session ended cleanly | Session shutdown |

### Backend Changes (agent_worker/main.py)

Add helper:

```python
async def _publish_state(participant: LocalParticipant, state: str):
    await participant.set_attributes({'lk.agent.state': state})
```

Call at each transition point in the session lifecycle.

### Frontend Changes (Flutter)

**aisee_livekit_voice_session.dart:**
- Add `ValueNotifier<String> agentState = ValueNotifier('connecting')`
- In room listener, add:

```dart
..on<ParticipantAttributesChanged>((event) {
  if (event.participant is RemoteParticipant) {
    final state = event.participant.attributes['lk.agent.state'];
    if (state != null) agentState.value = state;
  }
})
```

**user_assistant_tab.dart:**
- Receive `agentState` ValueNotifier
- Display localized text: `idle` = "San sang", `listening` = "Dang nghe...", `thinking` = "Dang xu ly...", `speaking` = "Dang tra loi..."

### Go Server Changes

None.

---

## 2. User Attributes + Locale-driven Prompt

### Problem

Agent has no context about the user when session starts. Must ask or hardcode assumptions. System prompt is Vietnamese-only, blocking internationalization.

### Solution

Frontend publishes user attributes after joining room. Agent reads them immediately. System prompt refactored to English base with locale-driven response language.

### Attributes

| Key | Value | Source |
|-----|-------|--------|
| `user_name` | Display name | Google profile / AiseeProfile |
| `user_id` | UUID | Session store |
| `locale` | e.g. `vi_VN`, `en_US` | `Platform.localeName` |
| `accessibility` | `low_vision` | Hardcoded (current user type) |
| `has_family_link` | `true` / `false` | Profile API |

### Go Server Changes (pkg/livekit/client.go)

Add `canUpdateOwnMetadata: true` to video grant in `CreateToken()`.

### Frontend Changes (Flutter)

**aisee_livekit_voice_session.dart:**
- `connect()` receives additional params: `userName`, `userId`, `hasFamilyLink`
- After `room.connect()` succeeds:

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

### Backend Changes (agent_worker/main.py)

- Read `participant.attributes` when user joins room
- Use `user_name` in greeting
- Use `user_id` for backend API calls
- Use `locale` for language selection

**System prompt refactor:**
- Rewrite system prompt in English
- Append locale instruction: `"Always respond in the user's language. User locale: {locale}. If locale starts with 'vi', speak Vietnamese. If 'en', speak English."`
- Keep all behavior rules, only change authoring language
- Gemini TTS voice "Aoede" supports multilingual

---

## 3. RPC for 6 App Tools

### Problem

Custom `tool_action` / `tool_action_ack` JSON protocol over raw data packets. No built-in timeout, no standard error codes, ~280 lines of manual dispatch/parse/ack code on frontend, ~112 lines of `DataChannelBridge` on agent.

### Solution

Replace with LiveKit RPC. Agent calls `perform_rpc()`, frontend registers handlers via `registerRpcMethod()`. Built-in 10s timeout, error codes, retry.

### RPC Methods

| Method | Payload (JSON) | Success Response (JSON) | Error |
|--------|---------------|------------------------|-------|
| `enable_camera` | `{}` | `{"status":"ok"}` | `RpcError(code, msg)` |
| `disable_camera` | `{}` | `{"status":"ok"}` | `RpcError(code, msg)` |
| `play_audio` | `{"url":"...","title":"..."}` | `{"status":"ok"}` | `RpcError(code, msg)` |
| `control_audio` | `{"command":"pause\|resume\|stop"}` | `{"status":"ok"}` | `RpcError(code, msg)` |
| `get_location` | `{}` | `{"lat":..,"lng":..,"address":".."}` | `RpcError(code, msg)` |
| `open_maps` | `{"destination":".."}` or `{"lat":..,"lng":..}` | `{"status":"ok"}` | `RpcError(code, msg)` |

### Backend Changes (agent_worker)

**Delete:** `data_channel.py` entirely.

**tools.py:** Replace `bridge.send_action()` calls with:

```python
response = await local_participant.perform_rpc(
    destination_identity=user_identity,
    method='enable_camera',
    payload=json.dumps(params),
)
```

Agent catches `RpcError` and informs user verbally.

**main.py:** Remove `DataChannelBridge` instantiation and data received handler for ack messages.

### Frontend Changes (Flutter)

**aisee_livekit_voice_session.dart:**

Register 6 methods before `room.connect()`:

```dart
room.registerRpcMethod('enable_camera', (data) async {
    await _handleEnableCamera();
    return jsonEncode({'status': 'ok'});
});
// ... same pattern for other 5 methods
```

**Delete:**
- `_handleToolActionPayload()` (~280 lines of dispatch logic)
- `_sendToolActionAck()`
- `_unwrapPayload()`, `_pickRequestId()`, `_onDataPayload()`
- `onDataUtf8` callback parameter from `connect()`
- `DataReceivedEvent` listener (unless needed for other purposes)

**Keep:** Individual handler logic (camera enable, audio play, etc.) — just extract into standalone methods called by RPC handlers.

### Go Server Changes

None.

---

## 4. Animated Orb Avatar

### Problem

Current UI shows only waveform bars. No visual representation of the assistant. User has limited feedback about agent presence.

### Solution

Replace `AssistantVoiceWaveform` with an animated orb (`CustomPainter` + `AnimationController`). Orb reacts to agent state and audio energy. Chartreuse accent on dark background. No external dependencies (no Lottie/Rive needed).

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
- Input: `ValueNotifier<String> agentState`, `ValueNotifier<double> assistantWaveEnergy`
- 2 `AnimationController`: pulse (continuous) + energy scale (driven by notifier)
- `CustomPainter` draws: filled circle with radial gradient + outer glow shadow
- Wrapped in `RepaintBoundary` for paint isolation

**Performance:**
- Skip repaint when energy delta < 0.02
- No `BackdropFilter` or expensive blur
- Only gradient + box shadow (GPU-friendly)

### Files Changed

- **New:** `lib/widgets/assistant_orb.dart`
- **Modified:** `user_assistant_tab.dart` — replace `AssistantVoiceWaveform` with `AssistantOrb`
- **Deprecate:** `assistant_voice_waveform.dart` — delete after orb is stable

### Frontend Only

No backend or Go server changes.

---

## Summary of Changes by Component

### Agent Worker (Python)

| File | Action |
|------|--------|
| `main.py` | Add `_publish_state()`, read user attributes, refactor system prompt to English + locale |
| `tools.py` | Replace `bridge.send_action()` with `perform_rpc()` for 6 app tools |
| `data_channel.py` | **Delete entirely** |
| `config.py` | No changes |
| `backend.py` | No changes |

### Go Server

| File | Action |
|------|--------|
| `pkg/livekit/client.go` | Add `canUpdateOwnMetadata: true` to token grant |

### Flutter App

| File | Action |
|------|--------|
| `aisee_livekit_voice_session.dart` | Add agentState notifier, register 6 RPC methods, publish user attributes, remove data channel dispatch code |
| `user_home_screen.dart` | Pass user data to connect(), receive agentState |
| `user_assistant_tab.dart` | Receive agentState, replace waveform with orb, show state text |
| `assistant_orb.dart` | **New** — animated orb widget |
| `assistant_voice_waveform.dart` | **Delete** |
| `livekit_payload_display.dart` | **Delete** (no more raw data channel payloads to display) |

---

## Dependencies

No new dependencies required for any component. All features use existing `livekit_client ^2.7.0` (Flutter) and `livekit-agents >=1.0.0` (Python) capabilities.

---

## Migration Notes

- RPC change requires **coordinated deploy**: agent worker and Flutter app must update together. Old data channel protocol and new RPC are incompatible.
- User attributes require token grant update (Go server) deployed before Flutter app update.
- Agent state and orb are additive — can deploy independently.

**Recommended deploy order:**
1. Go server (token grant update)
2. Agent worker + Flutter app (simultaneous — RPC + state + attributes + orb)
