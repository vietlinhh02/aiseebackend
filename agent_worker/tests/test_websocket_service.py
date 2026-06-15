import asyncio
import json
from contextlib import asynccontextmanager
from types import SimpleNamespace

import pytest
from google.genai import errors as genai_errors

from gemini_bridge.core import GeminiLiveSettings, WAKE_GREETING_TEXT
from gemini_bridge.tool_router import ToolRouter
from gemini_bridge.transport import TransportDeviceCommandChannel
from gemini_bridge.websocket_service import GeminiBridgeWebSocketService


_CLOSE = object()


class FakeWebSocket:
    def __init__(self):
        self.inbound = asyncio.Queue()
        self.sent = []
        self.closed = False

    def __aiter__(self):
        return self

    async def __anext__(self):
        item = await self.inbound.get()
        if item is _CLOSE:
            raise StopAsyncIteration
        return item

    async def send(self, data):
        self.sent.append(data)

    async def close(self):
        self.closed = True

    async def wait_closed(self):
        return None

    async def push_json(self, payload):
        await self.inbound.put(json.dumps(payload))

    async def push_binary(self, payload: bytes):
        await self.inbound.put(payload)

    async def finish(self):
        await self.inbound.put(_CLOSE)


class FakeBackend:
    def __init__(self):
        self.calls = []

    async def tool_call(self, name, user_id, args):
        raise AssertionError(f"unexpected backend tool call: {name}")

    async def tool_call_with_timeout(self, name, user_id, args, timeout=None):
        raise AssertionError(f"unexpected backend tool call with timeout: {name}")

    async def check_quota(self, user_id, quota_name, amount):
        return None

    async def record_usage(
        self, user_id, quota_name, amount, unit, source_type, source_id, metadata
    ):
        return None

    async def start_usage_session(
        self,
        user_id,
        usage_type,
        unit,
        source,
        source_id,
        started_at,
        metadata=None,
    ):
        self.calls.append(
            (
                "start_usage_session",
                user_id,
                usage_type,
                unit,
                source,
                source_id,
                started_at,
                metadata or {},
            )
        )
        return {"status": "started"}

    async def commit_usage_session(
        self,
        user_id,
        usage_type,
        unit,
        source,
        source_id,
        started_at,
        ended_at,
        metadata=None,
    ):
        self.calls.append(
            (
                "commit_usage_session",
                user_id,
                usage_type,
                unit,
                source,
                source_id,
                started_at,
                ended_at,
                metadata or {},
            )
        )
        return {"status": "committed"}

    async def record_ai_cost(self, **kwargs):
        self.calls.append(("record_ai_cost", kwargs))
        return {"status": "recorded"}


class FakeGeminiSession:
    def __init__(self, messages=None):
        self.messages = list(messages or [])
        self.realtime_inputs = []
        self.tool_responses = []

    async def send_realtime_input(self, *, audio=None, video=None, text=None):
        self.realtime_inputs.append({"audio": audio, "video": video, "text": text})

    async def send_tool_response(self, *, function_responses):
        self.tool_responses.extend(function_responses)

    async def receive(self):
        messages = list(self.messages)
        self.messages.clear()
        for message in messages:
            yield message


def build_settings():
    return GeminiLiveSettings(
        api_key="test-key",
        model="gemini-live",
        voice="Aoede",
        thinking_level="low",
        input_sample_rate=16000,
    )


def build_service(session, backend=None):
    backend = backend or FakeBackend()

    @asynccontextmanager
    async def connect(_settings, _session_context):
        yield session

    return GeminiBridgeWebSocketService(
        settings=build_settings(),
        gemini_session_factory=connect,
        tool_router_factory=lambda transport, session_context: ToolRouter(
            backend=backend,
            user_context={"user_id": session_context.get("user_id") or "user-1"},
            device_channel=TransportDeviceCommandChannel(transport),
        ),
    )


async def wait_for_tool_request(websocket: FakeWebSocket):
    for _ in range(50):
        for raw in websocket.sent:
            if not isinstance(raw, str):
                continue
            message = json.loads(raw)
            if message.get("type") == "tool_request":
                return message
        await asyncio.sleep(0.01)
    raise AssertionError("timed out waiting for tool_request")


@pytest.mark.anyio
async def test_websocket_service_routes_text_media_and_tool_results():
    message_tool = SimpleNamespace(
        server_content=None,
        tool_call=SimpleNamespace(
            function_calls=[SimpleNamespace(id="call-1", name="deactivate_camera", args={})]
        ),
    )
    message_audio = SimpleNamespace(
        server_content=SimpleNamespace(
            interrupted=False,
            turn_complete=True,
            model_turn=SimpleNamespace(
                parts=[
                    SimpleNamespace(
                        inline_data=SimpleNamespace(data=b"\x03\x04", mime_type="audio/pcm")
                    )
                ]
            ),
        ),
        tool_call=None,
    )
    gemini_session = FakeGeminiSession(messages=[message_tool, message_audio])
    websocket = FakeWebSocket()
    service = build_service(gemini_session)

    task = asyncio.create_task(service.handle(websocket))

    await websocket.push_json(
        {
            "type": "session_init",
            "payload": {"session_id": "ai-1", "user_id": "user-1", "media_mode": "audio_pcm"},
        }
    )
    await websocket.push_json({"type": "text", "text": "xin chào"})
    await websocket.push_json({"type": "audio_start"})
    await websocket.push_json({"type": "media_mode", "mode": "camera"})
    await websocket.push_binary(b"\xff\xd8\xff")
    await websocket.push_json({"type": "media_mode", "mode": "audio"})
    await websocket.push_binary(b"\x01\x02")
    await websocket.push_json({"type": "audio_stop"})

    tool_request = await wait_for_tool_request(websocket)
    assert tool_request["name"] == "disable_camera"
    assert tool_request["params"] == {}

    await websocket.push_json(
        {
            "type": "tool_result",
            "request_id": tool_request["request_id"],
            "ok": True,
            "result": {"status": "ok", "method": "disable_camera"},
        }
    )
    await websocket.finish()
    await task

    assert gemini_session.realtime_inputs[0]["text"] == "xin chào"
    assert gemini_session.realtime_inputs[1]["video"].data == b"\xff\xd8\xff"
    assert gemini_session.realtime_inputs[1]["video"].mime_type == "image/jpeg"
    assert gemini_session.realtime_inputs[2]["audio"].data == b"\x01\x02"
    assert gemini_session.realtime_inputs[2]["audio"].mime_type == "audio/pcm;rate=16000"
    assert gemini_session.tool_responses[0].response == {
        "status": "ok",
        "method": "disable_camera",
    }

    sent_json = [json.loads(raw) for raw in websocket.sent if isinstance(raw, str)]
    sent_types = [message["type"] for message in sent_json]
    assert sent_types[0:2] == ["ready", "agent_state"]
    assert "tool_request" in sent_types
    assert not any(message.get("code") == "unsupported_message_type" for message in sent_json)
    assert any(message == {"type": "agent_state", "state": "speaking"} for message in sent_json)
    assert sent_json[-1] == {"type": "agent_state", "state": "disconnected"}
    assert [raw for raw in websocket.sent if isinstance(raw, bytes)] == [b"\x03\x04"]


@pytest.mark.anyio
async def test_websocket_service_control_wake_greeting_is_forwarded():
    gemini_session = FakeGeminiSession()
    websocket = FakeWebSocket()
    service = build_service(gemini_session)

    task = asyncio.create_task(service.handle(websocket))
    await websocket.push_json(
        {
            "type": "session_init",
            "payload": {"session_id": "ai-1", "user_id": "user-1", "media_mode": "audio_pcm"},
        }
    )
    await websocket.push_json({"type": "control", "action": "wake_greeting"})
    await websocket.finish()
    await task

    assert gemini_session.realtime_inputs == [
        {"audio": None, "video": None, "text": WAKE_GREETING_TEXT}
    ]
    sent_json = [json.loads(raw) for raw in websocket.sent if isinstance(raw, str)]
    assert {"type": "agent_state", "state": "thinking"} in sent_json


@pytest.mark.anyio
async def test_websocket_service_tracks_ai_minutes_only_while_audio_stream_active():
    backend = FakeBackend()
    gemini_session = FakeGeminiSession()
    websocket = FakeWebSocket()
    service = build_service(gemini_session, backend=backend)

    task = asyncio.create_task(service.handle(websocket))
    await websocket.push_json(
        {
            "type": "session_init",
            "payload": {"session_id": "ai-usage-1", "user_id": "user-1", "media_mode": "audio_pcm"},
        }
    )
    await websocket.push_json({"type": "audio_start"})
    await websocket.push_binary(b"\x01\x02")
    await websocket.push_json({"type": "audio_stop"})
    await websocket.finish()
    await task

    usage_calls = [
        call
        for call in backend.calls
        if call[0] in {"start_usage_session", "commit_usage_session"}
    ]
    assert [call[0] for call in usage_calls] == [
        "start_usage_session",
        "commit_usage_session",
    ]
    start = usage_calls[0]
    commit = usage_calls[1]
    assert start[1:6] == (
        "user-1",
        "ai_minutes",
        "minutes",
        "assistant_ws",
        "ai-usage-1:audio:1",
    )
    assert start[7]["engine"] == "websocket"
    assert start[7]["websocket_session_id"] == "ai-usage-1"
    assert start[7]["audio_session_index"] == 1
    assert commit[1:6] == start[1:6]
    assert commit[6] == start[6]
    assert commit[7] >= commit[6]
    cost_calls = [call for call in backend.calls if call[0] == "record_ai_cost"]
    assert len(cost_calls) == 1
    assert cost_calls[0][1]["pricing_version"]


@pytest.mark.anyio
async def test_websocket_service_uses_distinct_usage_source_ids_for_repeated_wakes():
    backend = FakeBackend()
    gemini_session = FakeGeminiSession()
    websocket = FakeWebSocket()
    service = build_service(gemini_session, backend=backend)

    task = asyncio.create_task(service.handle(websocket))
    await websocket.push_json(
        {
            "type": "session_init",
            "payload": {"session_id": "ai-repeat", "user_id": "user-1", "media_mode": "audio_pcm"},
        }
    )
    await websocket.push_json({"type": "audio_start"})
    await websocket.push_json({"type": "audio_stop"})
    await websocket.push_json({"type": "audio_start"})
    await websocket.push_json({"type": "audio_stop"})
    await websocket.finish()
    await task

    source_ids = [
        call[5]
        for call in backend.calls
        if call[0] in {"start_usage_session", "commit_usage_session"}
    ]
    assert source_ids == [
        "ai-repeat:audio:1",
        "ai-repeat:audio:1",
        "ai-repeat:audio:2",
        "ai-repeat:audio:2",
    ]


@pytest.mark.anyio
async def test_websocket_service_direct_interrupt_frame_is_forwarded():
    gemini_session = FakeGeminiSession()
    websocket = FakeWebSocket()
    service = build_service(gemini_session)

    task = asyncio.create_task(service.handle(websocket))
    await websocket.push_json(
        {
            "type": "session_init",
            "payload": {"session_id": "ai-1", "user_id": "user-1", "media_mode": "audio_pcm"},
        }
    )
    await websocket.push_json({"type": "interrupt"})
    await websocket.finish()
    await task

    sent_json = [json.loads(raw) for raw in websocket.sent if isinstance(raw, str)]
    assert {"type": "agent_state", "state": "idle"} in sent_json


@pytest.mark.anyio
async def test_websocket_service_forwards_gemini_audio_interruption():
    message = SimpleNamespace(
        server_content=SimpleNamespace(
            interrupted=True,
            turn_complete=True,
            model_turn=None,
        ),
        tool_call=None,
    )
    gemini_session = FakeGeminiSession(messages=[message])
    websocket = FakeWebSocket()
    service = build_service(gemini_session)

    task = asyncio.create_task(service.handle(websocket))
    await websocket.push_json(
        {
            "type": "session_init",
            "payload": {"session_id": "ai-1", "user_id": "user-1", "media_mode": "audio_pcm"},
        }
    )
    await websocket.finish()
    await task

    sent_json = [json.loads(raw) for raw in websocket.sent if isinstance(raw, str)]
    assert {"type": "assistant_audio_interrupted"} in sent_json


@pytest.mark.anyio
async def test_websocket_service_reports_gemini_quota_connect_error():
    @asynccontextmanager
    async def connect(_settings, _session_context):
        raise genai_errors.APIError(
            1011,
            "You exceeded your current quota, please check your plan and billing details.",
            None,
        )
        yield

    websocket = FakeWebSocket()
    service = GeminiBridgeWebSocketService(
        settings=build_settings(),
        gemini_session_factory=connect,
        tool_router_factory=lambda transport, session_context: ToolRouter(
            backend=FakeBackend(),
            user_context={"user_id": session_context.get("user_id") or "user-1"},
            device_channel=TransportDeviceCommandChannel(transport),
        ),
    )

    await websocket.push_json(
        {
            "type": "session_init",
            "payload": {"session_id": "ai-1", "user_id": "user-1", "media_mode": "audio_pcm"},
        }
    )
    await service.handle(websocket)

    sent_json = [json.loads(raw) for raw in websocket.sent if isinstance(raw, str)]
    assert {"type": "agent_state", "state": "error"} in sent_json
    assert {
        "type": "error",
        "code": "gemini_quota_exceeded",
        "message": (
            "Gemini Live API quota exceeded. "
            "Check the Gemini API key billing/quota or switch to a key/model with Live API quota."
        ),
    } in sent_json
    assert sent_json[-1] == {"type": "agent_state", "state": "disconnected"}
    assert websocket.closed is True


# ── serve() resource caps ─────────────────────────────────────────────


class _CountingConnection:
    """Stand-in for ``websockets.asyncio.server.ServerConnection``."""

    def __init__(self) -> None:
        self.responded: tuple[int, str] | None = None

    def respond(self, status: int, text: str):
        self.responded = (status, text)
        return SimpleNamespace(status_code=status, body=text.encode("utf-8"))


@pytest.mark.anyio
async def test_process_request_rejects_when_at_capacity(monkeypatch):
    """process_request returns 503 once _active_ws_handlers >= max_clients."""
    import gemini_bridge.websocket_service as ws_module

    monkeypatch.setattr(ws_module, "_active_ws_handlers", 5)
    process_request = ws_module._build_process_request(max_clients=5)

    conn = _CountingConnection()
    result = await process_request(conn, request=None)

    assert result is not None
    assert conn.responded == (503, "Server at capacity, please retry")


@pytest.mark.anyio
async def test_process_request_allows_when_under_capacity(monkeypatch):
    """process_request returns None when below the cap so the handshake proceeds."""
    import gemini_bridge.websocket_service as ws_module

    monkeypatch.setattr(ws_module, "_active_ws_handlers", 3)
    process_request = ws_module._build_process_request(max_clients=5)

    conn = _CountingConnection()
    result = await process_request(conn, request=None)

    assert result is None
    assert conn.responded is None


@pytest.mark.anyio
async def test_handle_with_count_increments_and_decrements(monkeypatch):
    """_handle_with_count tracks the global counter via try/finally."""
    import gemini_bridge.websocket_service as ws_module

    monkeypatch.setattr(ws_module, "_active_ws_handlers", 0)
    entered = asyncio.Event()
    release = asyncio.Event()

    class _DummyService:
        async def handle(self, websocket):  # noqa: ARG002
            entered.set()
            await release.wait()

    service = _DummyService()
    task = asyncio.create_task(
        ws_module.GeminiBridgeWebSocketService._handle_with_count(
            service, websocket=object()
        )
    )

    await entered.wait()
    assert ws_module._active_ws_handlers == 1

    release.set()
    await task
    assert ws_module._active_ws_handlers == 0


@pytest.mark.anyio
async def test_handle_with_count_decrements_on_exception(monkeypatch):
    """Counter must be decremented even if handle() raises."""
    import gemini_bridge.websocket_service as ws_module

    monkeypatch.setattr(ws_module, "_active_ws_handlers", 0)

    class _BoomService:
        async def handle(self, websocket):  # noqa: ARG002
            raise RuntimeError("simulated failure")

    service = _BoomService()
    with pytest.raises(RuntimeError, match="simulated failure"):
        await ws_module.GeminiBridgeWebSocketService._handle_with_count(
            service, websocket=object()
        )
    assert ws_module._active_ws_handlers == 0
