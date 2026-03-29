# LiveKit Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace custom data channel protocol with RPC, add agent state publishing/UI, user attributes with i18n prompt, and animated orb avatar.

**Architecture:** Cross-repo changes spanning Go server (token grant), Python agent worker (state publishing, RPC, prompt), and Flutter app (state listener, RPC handlers, orb widget, attribute publishing). Deploy order: Go → agent + Flutter simultaneous.

**Tech Stack:** Go 1.26 + Chi, Python 3.12 + livekit-agents >=1.0.0, Flutter + livekit_client ^2.7.0

**Spec:** `docs/superpowers/specs/2026-03-29-livekit-optimization-design.md`

---

## File Map

### Go Server (`aiseebackend/`)

| File | Action | Responsibility |
|------|--------|---------------|
| `pkg/livekit/client.go:23-35` | Modify | Add `CanUpdateOwnMetadata` to token grant |

### Python Agent Worker (`aiseebackend/agent_worker/`)

| File | Action | Responsibility |
|------|--------|---------------|
| `main.py` | Modify | State publishing, user attribute listener, prompt locale selection, remove DataChannelBridge |
| `tools.py` | Modify | Replace bridge calls with `perform_rpc`, remove `_call_and_forward`, add explicit two-step |
| `prompts.py` | Create | Vietnamese + English system prompts, locale selector |
| `data_channel.py` | Delete | Replaced by RPC |

### Flutter App (`aiseeapp/lib/`)

| File | Action | Responsibility |
|------|--------|---------------|
| `models/agent_state.dart` | Create | AgentState enum with fromString, canListen, isConnected helpers |
| `widgets/assistant_orb.dart` | Create | Animated orb widget (CustomPainter + AnimationController) |
| `services/aisee_livekit_voice_session.dart` | Modify | AgentState notifier, 7 RPC method registrations, user attribute publishing, remove all data channel dispatch code |
| `screens/user_home_screen.dart` | Modify | Pass user data to connect(), receive agentState, remove _liveKitDataLine |
| `screens/user_assistant_tab.dart` | Modify | Receive agentState, replace waveform with orb, show state text |
| `widgets/assistant_voice_waveform.dart` | Delete | Replaced by orb |
| `utils/livekit_payload_display.dart` | Delete | No more raw data channel payloads |

---

## Task 1: Go Server — Token Grant Update

**Files:**
- Modify: `aiseebackend/pkg/livekit/client.go:23-35`

- [ ] **Step 1: Update CreateToken to add CanUpdateOwnMetadata**

```go
func (c *Client) CreateToken(roomName, identity, name string) (string, error) {
	at := auth.NewAccessToken(c.apiKey, c.apiSecret)
	canUpdate := true
	grant := &auth.VideoGrant{
		RoomJoin:             true,
		Room:                 roomName,
		CanUpdateOwnMetadata: &canUpdate,
	}
	at.SetVideoGrant(grant).
		SetIdentity(identity).
		SetName(name).
		SetValidFor(time.Hour)

	return at.ToJWT()
}
```

- [ ] **Step 2: Verify build**

Run: `cd aiseebackend && go build ./...`
Expected: Build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
cd aiseebackend
git add pkg/livekit/client.go
git commit -m "feat: add CanUpdateOwnMetadata to LiveKit token grant"
```

---

## Task 2: Flutter — AgentState Enum

**Files:**
- Create: `aiseeapp/lib/models/agent_state.dart`

- [ ] **Step 1: Create AgentState enum**

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

  factory AgentState.fromString(String? s) => switch (s) {
    'initializing' => AgentState.initializing,
    'idle' => AgentState.idle,
    'listening' => AgentState.listening,
    'thinking' => AgentState.thinking,
    'speaking' => AgentState.speaking,
    'disconnected' => AgentState.disconnected,
    'failed' => AgentState.failed,
    _ => AgentState.connecting,
  };

  bool get canListen =>
      this == listening || this == thinking || this == speaking;

  bool get isConnected =>
      this == listening || this == thinking || this == speaking;

  bool get isPending =>
      this == connecting || this == initializing || this == idle;

  bool get isFinished => this == disconnected || this == failed;

  String get displayTextVi => switch (this) {
    connecting || initializing => 'Đang kết nối...',
    idle => 'Sẵn sàng',
    listening => 'Đang nghe...',
    thinking => 'Đang xử lý...',
    speaking => 'Đang trả lời...',
    disconnected => 'Đã ngắt kết nối',
    failed => 'Lỗi kết nối',
  };
}
```

- [ ] **Step 2: Verify no analysis errors**

Run: `cd aiseeapp && dart analyze lib/models/agent_state.dart`
Expected: No issues found.

- [ ] **Step 3: Commit**

```bash
cd aiseeapp
git add lib/models/agent_state.dart
git commit -m "feat: add AgentState enum with display helpers"
```

---

## Task 3: Flutter — Agent State Listener in Voice Session

**Files:**
- Modify: `aiseeapp/lib/services/aisee_livekit_voice_session.dart`

- [ ] **Step 1: Add agentState notifier**

Add import at top:
```dart
import '../models/agent_state.dart';
```

Add field in class alongside existing notifiers (`assistantWaveEnergy`, etc.):
```dart
final ValueNotifier<AgentState> agentState =
    ValueNotifier<AgentState>(AgentState.connecting);
```

- [ ] **Step 2: Add ParticipantAttributesChanged listener**

In the `connect()` method, inside the room listener chain (after `TrackUnmutedEvent`), add:

```dart
..on<ParticipantAttributesChanged>((ParticipantAttributesChanged event) {
  if (event.participant is! RemoteParticipant) return;
  final String? raw = event.participant.attributes['lk.agent.state'];
  if (raw == null) return;
  final AgentState next = AgentState.fromString(raw);
  _log('AgentState', 'Agent state: $raw -> $next');
  agentState.value = next;
})
```

- [ ] **Step 3: Reset agentState in disconnect()**

In `disconnect()`, before existing cleanup, add:
```dart
agentState.value = AgentState.disconnected;
```

- [ ] **Step 4: Verify no analysis errors**

Run: `cd aiseeapp && dart analyze lib/services/aisee_livekit_voice_session.dart`
Expected: No issues found.

- [ ] **Step 5: Commit**

```bash
cd aiseeapp
git add lib/services/aisee_livekit_voice_session.dart
git commit -m "feat: listen for agent state via participant attributes"
```

---

## Task 4: Python Agent — State Publishing

**Files:**
- Modify: `aiseebackend/agent_worker/main.py`

- [ ] **Step 1: Add _publish_state helper**

After the `GREETING_SUFFIX` constant (line 135), add:

```python
async def _publish_state(local_participant, state: str):
    """Publish agent state as participant attribute for frontend UI."""
    try:
        await local_participant.set_attributes({'lk.agent.state': state})
        logger.info("published state: %s", state)
    except Exception as exc:
        logger.warning("failed to publish state %s: %s", state, exc)
```

- [ ] **Step 2: Publish `initializing` after connect**

After `await ctx.connect()` (line 158), add:
```python
await _publish_state(local, 'initializing')
```

- [ ] **Step 3: Publish `idle` after duplicate check passes**

After the duplicate check loop (after line 187), add:
```python
await _publish_state(local, 'idle')
```

- [ ] **Step 4: Publish `idle` in _stop_session (before aclose)**

In `_stop_session()`, as first line before `await active_session.aclose()`:
```python
await _publish_state(local, 'idle')
```
(`local` is captured from the enclosing `entrypoint` scope, already used elsewhere in `_stop_session`.)

- [ ] **Step 5: Publish `disconnected` before exit**

Before the `await _stop_session()` cleanup at line 362, add:
```python
await _publish_state(local, 'disconnected')
```

- [ ] **Step 6: Verify syntax**

Run: `cd aiseebackend/agent_worker && python -m py_compile main.py`
Expected: No output (clean compile).

- [ ] **Step 7: Commit**

```bash
cd aiseebackend
git add agent_worker/main.py
git commit -m "feat: publish agent state as participant attribute"
```

---

## Task 5: Python Agent — User Attributes Listener

**Files:**
- Modify: `aiseebackend/agent_worker/main.py`

- [ ] **Step 1: Add user_context dict and attribute reader**

After `logger.info("resolved user_id=%s", user_id)` (line 225), replace the static `user_id` with a mutable context dict:

```python
user_context = {
    'user_id': user_id,
    'user_name': '',
    'locale': 'vi',
    'accessibility': 'low_vision',
    'has_family_link': 'false',
}

def _read_user_attributes(participant):
    """Read user attributes from participant into user_context."""
    for key in ('user_name', 'user_id', 'locale', 'accessibility', 'has_family_link'):
        val = participant.attributes.get(key)
        if val:
            user_context[key] = val
    logger.info("user_context updated: %s", user_context)

# Read from existing participants (user may have connected first)
for p in ctx.room.remote_participants.values():
    if p.kind != rtc.ParticipantKind.PARTICIPANT_KIND_AGENT:
        _read_user_attributes(p)

@ctx.room.on("participant_attributes_changed")
def _on_attributes(changed_attributes, participant):
    if participant.kind == rtc.ParticipantKind.PARTICIPANT_KIND_AGENT:
        return
    _read_user_attributes(participant)
```

- [ ] **Step 2: Verify syntax**

Run: `cd aiseebackend/agent_worker && python -m py_compile main.py`
Expected: No output (clean compile). Note: `init_tools` call is NOT changed yet — that happens in Task 7 when both sides update atomically.

- [ ] **Step 3: Commit**

```bash
cd aiseebackend
git add agent_worker/main.py
git commit -m "feat: read user attributes from participant for locale/context"
```

---

## Task 6: Python Agent — Locale-Specific Prompts

**Files:**
- Create: `aiseebackend/agent_worker/prompts.py`
- Modify: `aiseebackend/agent_worker/main.py`

- [ ] **Step 1: Create prompts.py with Vietnamese + English system prompts**

Move the existing `SYSTEM_PROMPT`, `WAKEUP_SUFFIX`, `GREETING_SUFFIX` from `main.py` into `prompts.py`. Keep Vietnamese as-is. Add English translation:

```python
"""System prompts for AiSee agent — locale-specific variants."""

SYSTEM_PROMPT_VI = """\
BẠN LÀ AISEE — TRỢ LÝ GIỌNG NÓI CHO NGƯỜI SUY GIẢM THỊ LỰC TẠI VIỆT NAM.
LUÔN TRẢ LỜI BẰNG TIẾNG VIỆT. KHÔNG NGOẠI LỆ.
... (exact copy of current SYSTEM_PROMPT from main.py lines 24-123)
"""

SYSTEM_PROMPT_EN = """\
YOU ARE AISEE — A VOICE ASSISTANT FOR PEOPLE WITH LOW VISION.
ALWAYS RESPOND IN ENGLISH. NO EXCEPTIONS.

=== PERSONALITY ===
- Warm, confident, clear. Speak at a moderate pace, easy to understand.
- Use "I" for yourself, address the user as "you".
- NO pity. NO excessive apologies. NO flattery.
- Speak like a trustworthy friend, not a machine.

... (full English translation of all sections — identical rules, different language)
"""

WAKEUP_SUFFIX_VI = """

[Bạn vừa được kích hoạt lại. Nói ngắn gọn: "Tôi đang nghe." \
rồi chờ người dùng nói.]
"""

WAKEUP_SUFFIX_EN = """

[You just woke up. Say briefly: "I'm listening." \
then wait for the user to speak.]
"""

GREETING_SUFFIX_VI = """

Lời chào — khi bắt đầu phiên:
"Xin chào, tôi là AiSee. Tôi có thể giúp gì cho bạn?"
"""

GREETING_SUFFIX_EN = """

Greeting — when starting a session:
"Hello, I'm AiSee. How can I help you?"
"""


def select_prompt(locale: str, wake_up: bool) -> str:
    """Select system prompt based on locale and wake_up state."""
    if locale.startswith('vi'):
        base = SYSTEM_PROMPT_VI
        suffix = WAKEUP_SUFFIX_VI if wake_up else GREETING_SUFFIX_VI
    else:
        base = SYSTEM_PROMPT_EN
        suffix = WAKEUP_SUFFIX_EN if wake_up else GREETING_SUFFIX_EN
    return base + suffix
```

Note: The English prompt must be a **faithful translation** preserving all behavior rules (confirmation requirements, emergency escalation levels, safety guidelines, number-spelling conventions adapted to English). Do not shorten or simplify — translate 1:1.

- [ ] **Step 2: Update main.py to use prompts.py**

Remove `SYSTEM_PROMPT`, `WAKEUP_SUFFIX`, `GREETING_SUFFIX` constants from main.py.

Add import:
```python
from prompts import select_prompt
```

Update `AiSeeAgent.__init__`:
```python
class AiSeeAgent(Agent):
    def __init__(self, tools, locale: str = 'vi', wake_up: bool = False):
        super().__init__(
            instructions=select_prompt(locale, wake_up),
            tools=tools,
        )
```

Update `_start_session` to pass locale from `user_context`:
```python
agent = AiSeeAgent(
    tools=ALL_TOOLS,
    locale=user_context.get('locale', 'vi'),
    wake_up=wake_up,
)
```

- [ ] **Step 3: Verify syntax for both files**

Run: `cd aiseebackend/agent_worker && python -m py_compile prompts.py && python -m py_compile main.py`
Expected: No output (clean compile).

- [ ] **Step 4: Commit**

```bash
cd aiseebackend
git add agent_worker/prompts.py agent_worker/main.py
git commit -m "feat: locale-specific system prompts (vi + en)"
```

---

## Task 7: Python Agent — RPC Migration (Replace DataChannelBridge)

**Files:**
- Modify: `aiseebackend/agent_worker/tools.py`
- Modify: `aiseebackend/agent_worker/main.py`
- Delete: `aiseebackend/agent_worker/data_channel.py`

- [ ] **Step 1: Update tools.py — new init_tools signature**

Replace the globals and `init_tools` function:

```python
import json
from livekit import rtc

_backend: BackendClient | None = None
_local_participant: rtc.LocalParticipant | None = None
_user_context: dict | None = None


def init_tools(
    backend: BackendClient,
    local_participant: rtc.LocalParticipant,
    user_context: dict,
):
    """Initialize tool dependencies. Called from main.py."""
    global _backend, _local_participant, _user_context
    _backend = backend
    _local_participant = local_participant
    _user_context = user_context


def _user_identity() -> str:
    """LiveKit participant identity for RPC targeting (read live from user_context)."""
    return _user_context.get('user_identity', _user_context.get('user_id', ''))
```

Remove `from data_channel import DataChannelBridge, DataChannelTimeoutError` import.

**Important:** `_user_identity()` is now a function, not a cached string. It reads `user_context['user_identity']` which is set from `participant.identity` in `main.py` (see Task 7 Step 5). This is the LiveKit identity string, NOT the user UUID.

- [ ] **Step 2: Replace data channel tools with perform_rpc**

Replace `activate_camera`:
```python
@function_tool()
async def activate_camera(reason: str = "User requested") -> dict:
    """Enable camera to see through user's phone."""
    try:
        response = await _local_participant.perform_rpc(
            destination_identity=_user_identity(),
            method='enable_camera',
            payload='{}',
        )
        return json.loads(response)
    except Exception as exc:
        return {"status": "error", "error": str(exc)}
```

Replace `deactivate_camera`:
```python
@function_tool()
async def deactivate_camera() -> dict:
    """Disable camera, return to audio-only."""
    try:
        response = await _local_participant.perform_rpc(
            destination_identity=_user_identity(),
            method='disable_camera',
            payload='{}',
        )
        return json.loads(response)
    except Exception as exc:
        return {"status": "error", "error": str(exc)}
```

Replace `control_music`:
```python
@function_tool()
async def control_music(action: str) -> dict:
    """Dieu khien nhac dang phat: pause, resume, stop."""
    try:
        response = await _local_participant.perform_rpc(
            destination_identity=_user_identity(),
            method='control_audio',
            payload=json.dumps({"command": action}),
        )
        return json.loads(response)
    except Exception as exc:
        return {"status": "error", "error": str(exc)}
```

Replace `navigate_to`:
```python
@function_tool()
async def navigate_to(destination: str) -> dict:
    """Mo ban do va chi duong den dia diem."""
    try:
        response = await _local_participant.perform_rpc(
            destination_identity=_user_identity(),
            method='open_maps',
            payload=json.dumps({"destination": destination}),
        )
        return json.loads(response)
    except Exception as exc:
        return {"status": "error", "error": str(exc)}
```

Replace `get_current_location` (15s timeout):
```python
@function_tool()
async def get_current_location() -> dict:
    """Lay vi tri hien tai — tra ve dia chi doc duoc va toa do GPS."""
    try:
        response = await _local_participant.perform_rpc(
            destination_identity=_user_identity(),
            method='get_location',
            payload='{}',
            response_timeout=15.0,
        )
        return json.loads(response)
    except Exception as exc:
        return {"status": "error", "error": str(exc)}
```

Replace `update_location`:
```python
@function_tool()
async def update_location() -> dict:
    """Cap nhat vi tri hien tai cua nguoi dung len he thong."""
    try:
        response = await _local_participant.perform_rpc(
            destination_identity=_user_identity(),
            method='get_location',
            payload='{}',
            response_timeout=15.0,
        )
        location = json.loads(response)
        lat = location.get("lat", 0)
        lng = location.get("lng", 0)
    except Exception as exc:
        return {"status": "error", "error": str(exc)}
    return await _backend.tool_call(
        "update_location", _user_context.get('user_id', ''),
        {"lat": lat, "lng": lng},
    )
```

Replace `emergency_alert` (15s GPS timeout):
```python
@function_tool()
async def emergency_alert() -> dict:
    """SOS khan cap — gui thong bao den tat ca nguoi than kem vi tri GPS."""
    try:
        response = await _local_participant.perform_rpc(
            destination_identity=_user_identity(),
            method='get_location',
            payload='{}',
            response_timeout=15.0,
        )
        location = json.loads(response)
        lat = location.get("lat", 0)
        lng = location.get("lng", 0)
    except Exception:
        logger.warning("get_location RPC failed, using unknown location")
        lat, lng = 0, 0
    return await _backend.tool_call_with_timeout(
        "emergency_alert",
        _user_context.get('user_id', ''),
        {"lat": lat, "lng": lng},
        timeout=20.0,
    )
```

- [ ] **Step 3: Replace _call_and_forward with explicit two-step**

Delete the `_call_and_forward` helper entirely. Replace each tool that used it:

`play_music`:
```python
@function_tool()
async def play_music(query: str) -> dict:
    """Tim va phat nhac theo yeu cau."""
    result = await _backend.tool_call(
        "play_music", _user_context.get('user_id', ''), {"query": query},
    )
    play_action = result.pop("play_audio_action", None)
    if play_action and isinstance(play_action, dict):
        try:
            await _local_participant.perform_rpc(
                destination_identity=_user_identity(),
                method='play_audio',
                payload=json.dumps(play_action.get("params", {})),
            )
        except Exception as exc:
            logger.warning("play_audio RPC failed: %s", exc)
    return result
```

`set_medication_reminder`:
```python
@function_tool()
async def set_medication_reminder(
    medicine_name: str,
    dosage: str,
    times: list[str],
    frequency: str = "daily",
) -> dict:
    """Dat nhac nho uong thuoc."""
    result = await _backend.tool_call(
        "set_medication_reminder",
        _user_context.get('user_id', ''),
        {"medicine_name": medicine_name, "dosage": dosage,
         "times": times, "frequency": frequency},
    )
    reminder_action = result.pop("schedule_reminder_action", None)
    if reminder_action and isinstance(reminder_action, dict):
        try:
            await _local_participant.perform_rpc(
                destination_identity=_user_identity(),
                method='schedule_reminder',
                payload=json.dumps(reminder_action.get("params", {})),
            )
        except Exception as exc:
            logger.warning("schedule_reminder RPC failed: %s", exc)
    return result
```

`set_appointment_reminder`:
```python
@function_tool()
async def set_appointment_reminder(
    title: str,
    datetime: str,
    location: str = "",
    notes: str = "",
) -> dict:
    """Dat nhac nho lich hen bac si. datetime dang YYYY-MM-DD HH:MM."""
    result = await _backend.tool_call(
        "set_appointment_reminder",
        _user_context.get('user_id', ''),
        {"title": title, "datetime": datetime,
         "location": location, "notes": notes},
    )
    reminder_action = result.pop("schedule_reminder_action", None)
    if reminder_action and isinstance(reminder_action, dict):
        try:
            await _local_participant.perform_rpc(
                destination_identity=_user_identity(),
                method='schedule_reminder',
                payload=json.dumps(reminder_action.get("params", {})),
            )
        except Exception as exc:
            logger.warning("schedule_reminder RPC failed: %s", exc)
    return result
```

- [ ] **Step 4: Update all backend tool calls to use user_context**

Replace every `_user_id` reference with `_user_context.get('user_id', '')` in the remaining backend tools (`make_voice_call`, `make_video_call`, `send_chat_message`, `read_unread_messages`, `list_medication_reminders`, `mark_medication_taken`, `mark_medication_skipped`, `log_health_symptom`, `log_health_metric`, `escalate_to_family`, `escalate_to_volunteer`, `rate_volunteer`).

- [ ] **Step 5: Update main.py — remove DataChannelBridge + update init_tools**

Remove:
- `from data_channel import DataChannelBridge` import
- `bridge = DataChannelBridge()` instantiation
- `@ctx.room.on("data_received")` handler and `on_data` function
- `bridge` parameter from `init_tools()` call

Add user_identity resolution (after `user_context` dict setup, before `init_tools`):

```python
# Resolve LiveKit participant identity for RPC targeting.
# This is the identity string from the token, which may differ from user_id UUID.
user_identity = ""
for p in ctx.room.remote_participants.values():
    if p.kind != rtc.ParticipantKind.PARTICIPANT_KIND_AGENT:
        user_identity = p.identity
        break
if not user_identity:
    user_identity = user_id
user_context['user_identity'] = user_identity
logger.info("resolved user_identity=%s (for RPC)", user_identity)
```

Also listen for new participants joining to update identity:
```python
@ctx.room.on("participant_connected")
def _on_user_identity_update(participant):
    if participant.kind != rtc.ParticipantKind.PARTICIPANT_KIND_AGENT:
        user_context['user_identity'] = participant.identity
        logger.info("updated user_identity=%s", participant.identity)
```

Update `init_tools` call (this is the atomic change — both `tools.py` new signature and `main.py` new call happen in the same task):
```python
init_tools(
    backend=backend,
    local_participant=ctx.room.local_participant,
    user_context=user_context,
)
```

- [ ] **Step 6: Delete data_channel.py**

```bash
git rm aiseebackend/agent_worker/data_channel.py
```

- [ ] **Step 7: Verify syntax**

Run: `cd aiseebackend/agent_worker && python -m py_compile main.py && python -m py_compile tools.py`
Expected: No output (clean compile).

- [ ] **Step 8: Commit**

```bash
cd aiseebackend
git add -A agent_worker/
git commit -m "feat: replace DataChannelBridge with LiveKit RPC for all app tools"
```

---

## Task 8: Flutter — RPC Method Registration

**Files:**
- Modify: `aiseeapp/lib/services/aisee_livekit_voice_session.dart`

- [ ] **Step 1: Add RPC method registrations**

Add a new method `_registerRpcMethods()` in the class:

```dart
void _registerRpcMethods() {
  final Room? room = _room;
  if (room == null) return;

  room.registerRpcMethod('enable_camera', (RpcInvocationData data) async {
    _log('RPC', 'enable_camera called');
    if (!kIsWeb) {
      await _ensurePermission(Permission.camera, nameVi: 'camera');
    }
    final CameraCaptureOptions camOpts = await _agentCameraCaptureOptions();
    await _room?.localParticipant?.setCameraEnabled(
      true,
      cameraCaptureOptions: camOpts,
    );
    syncLocalCameraPreviewTrack();
    return jsonEncode({'status': 'ok'});
  });

  room.registerRpcMethod('disable_camera', (RpcInvocationData data) async {
    _log('RPC', 'disable_camera called');
    await _room?.localParticipant?.setCameraEnabled(false);
    syncLocalCameraPreviewTrack();
    return jsonEncode({'status': 'ok'});
  });

  room.registerRpcMethod('play_audio', (RpcInvocationData data) async {
    final Map<String, dynamic> params =
        jsonDecode(data.payload) as Map<String, dynamic>;
    final String url = _paramString(params, const ['url', 'audio_url', 'src']);
    final String titleRaw = _paramString(params, const ['title', 'name']);
    _log('RPC', 'play_audio url=${url.length > 60 ? '${url.substring(0, 60)}...' : url}');
    if (url.isEmpty) {
      throw RpcError(2001, 'Missing url parameter');
    }
    final AudioPlayer player = await _musicPlayerOrCreate();
    await player.setUrl(url);
    _setNowPlayingTitle(titleRaw.isEmpty ? null : titleRaw);
    await player.play();
    return jsonEncode({'status': 'ok'});
  });

  room.registerRpcMethod('control_audio', (RpcInvocationData data) async {
    final Map<String, dynamic> params =
        jsonDecode(data.payload) as Map<String, dynamic>;
    final String cmd = _paramString(params, const ['command', 'action', 'op'])
        .toLowerCase();
    _log('RPC', 'control_audio cmd=$cmd');
    final AudioPlayer? p = _musicPlayer;
    if (p == null) {
      return jsonEncode({'status': 'ok'});
    }
    switch (cmd) {
      case 'pause':
        await p.pause();
      case 'resume' || 'play':
        await p.play();
      case 'stop':
        await p.stop();
        _setNowPlayingTitle(null);
      default:
        throw RpcError(2002, 'Unsupported command: $cmd');
    }
    return jsonEncode({'status': 'ok'});
  });

  room.registerRpcMethod('get_location', (RpcInvocationData data) async {
    _log('RPC', 'get_location called');
    final AiseeLocationResult loc =
        await AiseeLocationService.getCurrentLocation();
    return jsonEncode({
      'status': 'ok',
      'lat': loc.lat,
      'lng': loc.lng,
      'address': loc.address,
    });
  });

  room.registerRpcMethod('open_maps', (RpcInvocationData data) async {
    final Map<String, dynamic> params =
        jsonDecode(data.payload) as Map<String, dynamic>;
    final String dest = _paramString(params, const ['destination', 'dest', 'query', 'q']);
    final String latStr = _paramString(params, const ['lat', 'latitude']);
    final String lngStr = _paramString(params, const ['lng', 'longitude']);
    _log('RPC', 'open_maps dest="$dest" lat=$latStr lng=$lngStr');

    Uri? mapUri;
    if (dest.isNotEmpty) {
      if (!kIsWeb && Platform.isIOS) {
        mapUri = Uri.parse('https://maps.apple.com/?daddr=${Uri.encodeComponent(dest)}');
      } else {
        mapUri = Uri.parse('google.navigation:q=${Uri.encodeComponent(dest)}');
      }
    } else if (latStr.isNotEmpty && lngStr.isNotEmpty) {
      if (!kIsWeb && Platform.isIOS) {
        mapUri = Uri.parse('https://maps.apple.com/?daddr=$latStr,$lngStr');
      } else {
        mapUri = Uri.parse('google.navigation:q=$latStr,$lngStr');
      }
    }

    if (mapUri == null) {
      throw RpcError(2003, 'Missing destination or lat/lng');
    }
    final bool launched = await launchUrl(mapUri, mode: LaunchMode.externalApplication);
    if (!launched) {
      throw RpcError(2004, 'Cannot open maps');
    }
    return jsonEncode({'status': 'ok'});
  });

  room.registerRpcMethod('schedule_reminder', (RpcInvocationData data) async {
    final Map<String, dynamic> params =
        jsonDecode(data.payload) as Map<String, dynamic>;
    final String typeStr = _paramString(params, const ['type', 'reminder_type']);
    final String name = _paramString(params, const ['name', 'title']);
    final int baseId = DateTime.now().millisecondsSinceEpoch % 100000;
    _log('RPC', 'schedule_reminder type=$typeStr name=$name');

    if (typeStr == 'medication') {
      final List<String> times =
          (params['times'] as List<dynamic>?)?.cast<String>() ?? <String>[];
      final String freq = _paramString(params, const ['frequency', 'freq']);
      final AiseeReminder reminder = AiseeReminder(
        id: baseId,
        type: AiseeReminderType.medication,
        title: name,
        times: times,
        frequency: freq.isEmpty ? 'daily' : freq,
        createdAt: DateTime.now(),
      );
      await AiseeReminderService.instance.scheduleReminder(reminder);
    } else if (typeStr == 'appointment') {
      final String dt = _paramString(params, const ['datetime', 'date_time', 'time']);
      final AiseeReminder reminder = AiseeReminder(
        id: baseId,
        type: AiseeReminderType.appointment,
        title: name,
        datetime: dt,
        createdAt: DateTime.now(),
      );
      await AiseeReminderService.instance.scheduleReminder(reminder);
    }
    return jsonEncode({'status': 'ok'});
  });

  _log('RPC', '7 methods registered');
}
```

- [ ] **Step 2: Call _registerRpcMethods in connect()**

In `connect()`, after `await room.connect(creds.url, creds.token)` and before `await room.localParticipant?.setMicrophoneEnabled(true)`:

```dart
_registerRpcMethods();
```

- [ ] **Step 3: Remove old data channel code**

Delete from the class:
- `_unwrapPayload()` method
- `_pickRequestId()` method
- `_asParamMap()` method
- `_sendToolActionAck()` method
- `_handleToolActionPayload()` method (entire ~280 line switch block)
- `_onDataPayload()` method
- `DataReceivedEvent` handler from room listener
- `onDataUtf8` parameter from `connect()` method signature

**Keep** (used by RPC handlers):
- `_pickString()`, `_paramString()` — extract fields from RPC payloads
- `_musicPlayerOrCreate()`, `_setNowPlayingTitle()` — music handlers
- `_ensurePermission()`, `_agentCameraCaptureOptions()` — camera handler
- `syncLocalCameraPreviewTrack()` — camera handler

- [ ] **Step 4: Verify no analysis errors**

Run: `cd aiseeapp && dart analyze lib/services/aisee_livekit_voice_session.dart`
Expected: No issues found.

- [ ] **Step 5: Commit**

```bash
cd aiseeapp
git add lib/services/aisee_livekit_voice_session.dart
git commit -m "feat: register 7 RPC methods, remove data channel dispatch"
```

---

## Task 9: Flutter — User Attributes Publishing

**Files:**
- Modify: `aiseeapp/lib/services/aisee_livekit_voice_session.dart`
- Modify: `aiseeapp/lib/screens/user_home_screen.dart`

- [ ] **Step 1: Update connect() signature**

Change `connect()` to accept user data:

```dart
Future<void> connect({
  required String userName,
  required String userId,
  required bool hasFamilyLink,
}) async {
```

- [ ] **Step 2: Publish attributes after RPC registration**

In `connect()`, after `_registerRpcMethods()` and before mic enable:

```dart
await room.localParticipant?.setAttributes(<String, String>{
  'user_name': userName,
  'user_id': userId,
  'locale': Platform.localeName,
  'accessibility': 'low_vision',
  'has_family_link': hasFamilyLink ? 'true' : 'false',
});
_log('Connect', 'User attributes published: locale=${Platform.localeName}');
```

Ensure `dart:io` `Platform` import is present (already imported for `Platform.isIOS`).

- [ ] **Step 3: Update user_home_screen.dart to pass user data**

In `_startLiveKitVoice()`, update the connect call:

```dart
await _liveKitVoice.connect(
  userName: _displayName,
  userId: widget.aiseeProfile?.id ?? '',
  hasFamilyLink: false, // AiseeUserProfile has no hasFamilyLink field yet
);
```

Note: `AiseeUserProfile` currently has fields: `id`, `email`, `displayName`, `role`, `isVolunteer`, `avatarUrl`, `createdAt`. It does NOT have `hasFamilyLink`. Hardcode to `false` for now. When the backend API adds this field, add it to the model and wire it through.

Remove the old `onDataUtf8` callback and `_liveKitDataLine` state:
- Delete `String? _liveKitDataLine;` field
- Delete `import '../utils/livekit_payload_display.dart';`
- Remove `liveKitDataLine: _liveKitDataLine,` from `UserAssistantTab` constructor
- Remove `_liveKitDataLine` from `setState` calls

- [ ] **Step 4: Verify no analysis errors**

Run: `cd aiseeapp && dart analyze lib/services/ lib/screens/user_home_screen.dart`
Expected: No issues found (or only warnings from other files).

- [ ] **Step 5: Commit**

```bash
cd aiseeapp
git add lib/services/aisee_livekit_voice_session.dart lib/screens/user_home_screen.dart
git commit -m "feat: publish user attributes on room join"
```

---

## Task 10: Flutter — Animated Orb Widget

**Files:**
- Create: `aiseeapp/lib/widgets/assistant_orb.dart`

- [ ] **Step 1: Create AssistantOrb widget**

```dart
import 'dart:math' as math;

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';

import '../models/agent_state.dart';
import '../theme/style_tokens.dart';

class AssistantOrb extends StatefulWidget {
  const AssistantOrb({
    super.key,
    required this.agentState,
    required this.voiceEnergy,
  });

  final ValueListenable<AgentState> agentState;
  final ValueListenable<double> voiceEnergy;

  @override
  State<AssistantOrb> createState() => _AssistantOrbState();
}

class _AssistantOrbState extends State<AssistantOrb>
    with TickerProviderStateMixin {
  late final AnimationController _pulse = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 2000),
  )..repeat();

  late final AnimationController _ripple = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1500),
  );

  void _onStateChanged() {
    if (!mounted) return;
    final AgentState s = widget.agentState.value;
    if (s == AgentState.thinking) {
      if (!_ripple.isAnimating) _ripple.repeat();
    } else {
      _ripple.stop();
      _ripple.reset();
    }

    _pulse.duration = switch (s) {
      AgentState.listening => const Duration(milliseconds: 1000),
      AgentState.connecting || AgentState.initializing =>
        const Duration(milliseconds: 3000),
      _ => const Duration(milliseconds: 2000),
    };
  }

  @override
  void initState() {
    super.initState();
    widget.agentState.addListener(_onStateChanged);
    widget.voiceEnergy.addListener(_markDirty);
    _onStateChanged();
  }

  void _markDirty() {
    if (mounted) setState(() {});
  }

  @override
  void dispose() {
    widget.agentState.removeListener(_onStateChanged);
    widget.voiceEnergy.removeListener(_markDirty);
    _pulse.dispose();
    _ripple.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Semantics(
      label: widget.agentState.value.displayTextVi,
      child: RepaintBoundary(
        child: AnimatedBuilder(
          animation: Listenable.merge([_pulse, _ripple, widget.voiceEnergy]),
          builder: (BuildContext context, Widget? child) {
            return CustomPaint(
              size: const Size(160, 160),
              painter: _OrbPainter(
                state: widget.agentState.value,
                energy: widget.voiceEnergy.value.clamp(0.0, 1.0),
                pulseT: _pulse.value,
                rippleT: _ripple.value,
              ),
            );
          },
        ),
      ),
    );
  }
}

class _OrbPainter extends CustomPainter {
  _OrbPainter({
    required this.state,
    required this.energy,
    required this.pulseT,
    required this.rippleT,
  });

  final AgentState state;
  final double energy;
  final double pulseT;
  final double rippleT;

  static const Color _chartreuse = Color(0xFF6DD210);
  static const Color _gray = Color(0xFF5D5D5D);

  @override
  void paint(Canvas canvas, Size size) {
    final Offset center = size.center(Offset.zero);
    final double baseRadius = size.shortestSide / 2 * 0.6;

    // Scale based on state
    final double stateScale = switch (state) {
      AgentState.speaking => 1.0 + energy * 0.3,
      AgentState.listening => 1.0 + 0.05 * math.sin(pulseT * 2 * math.pi),
      AgentState.thinking => 1.0 + 0.02 * math.sin(rippleT * 2 * math.pi),
      AgentState.disconnected || AgentState.failed => 0.95,
      _ => 1.0 + 0.03 * math.sin(pulseT * 2 * math.pi),
    };

    final double radius = baseRadius * stateScale;

    // Color based on state
    final double opacity = switch (state) {
      AgentState.speaking => 1.0,
      AgentState.listening => 0.7,
      AgentState.idle => 0.5,
      AgentState.thinking => 0.7 + 0.3 * math.sin(rippleT * 2 * math.pi),
      AgentState.connecting || AgentState.initializing =>
        0.3 + 0.2 * math.sin(pulseT * 2 * math.pi),
      AgentState.disconnected || AgentState.failed => 0.3,
    };

    final Color orbColor = state == AgentState.disconnected ||
            state == AgentState.failed
        ? _gray.withValues(alpha: opacity)
        : _chartreuse.withValues(alpha: opacity);

    // Glow
    final Paint glowPaint = Paint()
      ..color = orbColor.withValues(alpha: opacity * 0.3)
      ..maskFilter = const MaskFilter.blur(BlurStyle.normal, 30);
    canvas.drawCircle(center, radius * 1.3, glowPaint);

    // Main orb — radial gradient
    final Paint orbPaint = Paint()
      ..shader = RadialGradient(
        colors: [
          orbColor,
          orbColor.withValues(alpha: opacity * 0.4),
        ],
      ).createShader(
        Rect.fromCircle(center: center, radius: radius),
      );
    canvas.drawCircle(center, radius, orbPaint);

    // Thinking ripple
    if (state == AgentState.thinking && rippleT > 0) {
      final double rippleRadius = radius * (1.0 + rippleT * 0.5);
      final double rippleAlpha = (1.0 - rippleT) * 0.4;
      final Paint ripplePaint = Paint()
        ..color = _chartreuse.withValues(alpha: rippleAlpha)
        ..style = PaintingStyle.stroke
        ..strokeWidth = 2;
      canvas.drawCircle(center, rippleRadius, ripplePaint);
    }
  }

  @override
  bool shouldRepaint(_OrbPainter old) =>
      old.state != state ||
      (old.energy - energy).abs() > 0.02 ||
      (old.pulseT - pulseT).abs() > 0.01 ||
      (old.rippleT - rippleT).abs() > 0.01;
}
```

- [ ] **Step 2: Verify no analysis errors**

Run: `cd aiseeapp && dart analyze lib/widgets/assistant_orb.dart`
Expected: No issues found.

- [ ] **Step 3: Commit**

```bash
cd aiseeapp
git add lib/widgets/assistant_orb.dart
git commit -m "feat: add animated orb widget for agent state visualization"
```

---

## Task 11: Flutter — Wire Orb + Agent State into UI

**Files:**
- Modify: `aiseeapp/lib/screens/user_assistant_tab.dart`
- Modify: `aiseeapp/lib/screens/user_home_screen.dart`

- [ ] **Step 1: Update UserAssistantTab to use agentState + orb**

Replace imports — remove waveform, add orb + agent state:
```dart
import '../models/agent_state.dart';
import '../widgets/assistant_orb.dart';
```
Remove: `import '../widgets/assistant_voice_waveform.dart';`

Update constructor: replace `assistantMusicActive` with `agentState`:
```dart
const UserAssistantTab({
  super.key,
  required this.aiseeLoading,
  required this.aiseeError,
  required this.liveKitConnecting,
  required this.liveKitError,
  required this.liveKitConnected,
  this.nowPlayingTitle,
  this.assistantWaveEnergy,
  this.agentState,
  this.localCameraPreviewTrack,
  required this.onRetryLiveKit,
});
```

Remove: `liveKitDataLine` and `assistantMusicActive` fields.
Add: `final ValueListenable<AgentState>? agentState;`

- [ ] **Step 2: Replace waveform with orb in _AssistantContentColumn**

In `_AssistantContentColumn`, replace the waveform section:

Old:
```dart
if (liveKitConnected &&
    assistantWaveEnergy != null &&
    assistantMusicActive != null) ...[
  const SizedBox(height: 20),
  AssistantVoiceWaveform(
    voiceEnergy: assistantWaveEnergy!,
    musicActive: assistantMusicActive!,
  ),
],
```

New:
```dart
if (liveKitConnected &&
    assistantWaveEnergy != null &&
    agentState != null) ...[
  const SizedBox(height: 20),
  AssistantOrb(
    agentState: agentState!,
    voiceEnergy: assistantWaveEnergy!,
  ),
  const SizedBox(height: 12),
  ValueListenableBuilder<AgentState>(
    valueListenable: agentState!,
    builder: (BuildContext context, AgentState state, _) {
      return Text(
        state.displayTextVi,
        textAlign: TextAlign.center,
        style: base.copyWith(fontSize: 14),
      );
    },
  ),
],
```

Replace the static connection text with agentState-driven text:
```dart
if (agentState != null)
  ValueListenableBuilder<AgentState>(
    valueListenable: agentState!,
    builder: (BuildContext context, AgentState state, _) {
      return Text(
        state.displayTextVi,
        textAlign: TextAlign.center,
        style: base,
      );
    },
  )
else
  Text(
    liveKitConnected
        ? 'Đã kết nối trợ lý giọng nói.'
        : 'Đang chờ kết nối…',
    textAlign: TextAlign.center,
    style: base,
  ),
```

Remove the `_AssistantDataNotice` widget and `liveKitDataLine` references entirely.

- [ ] **Step 3: Update _AssistantContentColumn and _AssistantLiveBody constructors**

Thread `agentState` through both internal widgets. Remove `assistantMusicActive` and `liveKitDataLine` fields from both.

- [ ] **Step 4: Update user_home_screen.dart**

Pass `agentState` instead of `assistantMusicActive` and `liveKitDataLine`:

```dart
UserAssistantTab(
  aiseeLoading: widget.aiseeLoading,
  aiseeError: widget.aiseeError,
  liveKitConnecting: _liveKitConnecting,
  liveKitError: _liveKitError,
  liveKitConnected: _liveKitConnected,
  nowPlayingTitle: _liveKitVoice.nowPlayingTitle,
  assistantWaveEnergy: _liveKitVoice.assistantWaveEnergy,
  agentState: _liveKitVoice.agentState,
  localCameraPreviewTrack: _liveKitVoice.localCameraPreviewTrack,
  onRetryLiveKit: _startLiveKitVoice,
),
```

- [ ] **Step 5: Verify no analysis errors**

Run: `cd aiseeapp && dart analyze lib/screens/user_assistant_tab.dart lib/screens/user_home_screen.dart`
Expected: No issues found.

- [ ] **Step 6: Commit**

```bash
cd aiseeapp
git add lib/screens/user_assistant_tab.dart lib/screens/user_home_screen.dart
git commit -m "feat: wire orb + agent state into assistant UI"
```

---

## Task 12: Cleanup — Delete Old Files

**Files:**
- Delete: `aiseeapp/lib/widgets/assistant_voice_waveform.dart`
- Delete: `aiseeapp/lib/utils/livekit_payload_display.dart`

- [ ] **Step 1: Verify no remaining imports**

Run:
```bash
cd aiseeapp && grep -r "assistant_voice_waveform" lib/
cd aiseeapp && grep -r "livekit_payload_display" lib/
```
Expected: No matches (if matches found, remove the imports first).

- [ ] **Step 2: Delete files**

```bash
cd aiseeapp
git rm lib/widgets/assistant_voice_waveform.dart
git rm lib/utils/livekit_payload_display.dart
```

- [ ] **Step 3: Verify build**

Run: `cd aiseeapp && flutter analyze`
Expected: No errors related to deleted files.

- [ ] **Step 4: Commit**

```bash
cd aiseeapp
git add -A lib/widgets/assistant_voice_waveform.dart lib/utils/livekit_payload_display.dart
git commit -m "chore: delete waveform and payload display (replaced by orb + RPC)"
```

---

## Task 13: Integration Verification

- [ ] **Step 1: Build Flutter app**

Run: `cd aiseeapp && flutter build apk --debug`
Expected: BUILD SUCCESSFUL.

- [ ] **Step 2: Build Go server**

Run: `cd aiseebackend && go build ./...`
Expected: Build succeeds.

- [ ] **Step 3: Verify Python agent syntax**

Run: `cd aiseebackend/agent_worker && python -m py_compile main.py && python -m py_compile tools.py && python -m py_compile prompts.py`
Expected: No output (all clean).

- [ ] **Step 4: Manual integration test checklist**

Start all 3 services and verify:
- [ ] App connects to LiveKit room
- [ ] Agent state shows "Đang nghe..." when user speaks
- [ ] Agent state shows "Đang xử lý..." during tool calls
- [ ] Agent state shows "Đang trả lời..." when agent speaks
- [ ] Orb pulses and glows correctly per state
- [ ] "Bật camera" RPC works (enable_camera)
- [ ] "Phát nhạc" RPC works (play_music → play_audio)
- [ ] "Vị trí" RPC works (get_location with 15s timeout)
- [ ] "Mở bản đồ" RPC works (open_maps)
- [ ] "Đặt nhắc thuốc" works (set_medication_reminder → schedule_reminder)
- [ ] User name appears in agent greeting
- [ ] Locale attribute received by agent

- [ ] **Step 5: Final commit**

```bash
cd aiseebackend
git add -A
git commit -m "chore: integration verification complete"
```
