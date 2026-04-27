import json
import logging
import asyncio

import pytest

from gemini_bridge.transport import TransportDeviceCommandChannel
from gemini_bridge.tool_router import ToolRouter


class FakeBackend:
    def __init__(self):
        self.calls = []

    async def tool_call(self, tool, user_id, params):
        self.calls.append(("tool_call", tool, user_id, params))
        return {"status": "ok", "tool": tool}

    async def tool_call_with_timeout(self, tool, user_id, params, timeout):
        self.calls.append(("tool_call_with_timeout", tool, user_id, params, timeout))
        return {"status": "ok", "tool": tool}

    async def check_quota(self, user_id, usage_type, quantity):
        self.calls.append(("check_quota", user_id, usage_type, quantity))
        return {"allowed": True}

    async def record_usage(self, user_id, usage_type, quantity, unit, source, source_id, metadata):
        self.calls.append(("record_usage", user_id, usage_type, quantity, unit, source, metadata))
        return {"status": "recorded"}


class FakeParticipant:
    def __init__(self):
        self.rpc_calls = []

    async def perform_rpc(self, **kwargs):
        self.rpc_calls.append(kwargs)
        method = kwargs["method"]
        if method == "get_location":
            return json.dumps({"lat": 10.1, "lng": 20.2})
        return json.dumps({"status": "ok", "method": method})


class FakeTransport:
    def __init__(self, *, result=None, block=False):
        self.result = result
        self.block = block
        self.tool_requests = []

    async def send_tool_request(self, event, timeout=None):
        self.tool_requests.append((event, timeout))
        if self.block:
            await asyncio.sleep(3600)
        return self.result or {"status": "ok"}


class FakeSearchClient:
    def __init__(self):
        self.calls = []

    async def search(self, query, *, max_results=5):
        self.calls.append((query, max_results))
        return {
            "status": "ok",
            "query": query,
            "results": [
                {
                    "title": "Result",
                    "url": "https://example.test/result",
                    "text": "Search result text",
                }
            ],
        }


@pytest.mark.anyio
async def test_router_dispatches_backend_only_tool():
    backend = FakeBackend()
    participant = FakeParticipant()
    router = ToolRouter(
        backend=backend,
        local_participant=participant,
        user_context={"user_id": "u1", "user_identity": "phone1"},
    )

    result = await router.call("send_chat_message", {"contact_name": "Nam", "message": "Xin chao"})

    assert result == {"status": "ok", "tool": "send_chat_message"}
    assert backend.calls == [
        ("tool_call", "send_chat_message", "u1", {"contact_name": "Nam", "message": "Xin chao"})
    ]
    assert participant.rpc_calls == []


@pytest.mark.anyio
async def test_router_dispatches_rpc_only_tool():
    backend = FakeBackend()
    participant = FakeParticipant()
    router = ToolRouter(
        backend=backend,
        local_participant=participant,
        user_context={"user_id": "u1", "user_identity": "phone1"},
    )

    result = await router.call("deactivate_camera", {})

    assert result == {"status": "ok", "method": "disable_camera"}
    assert participant.rpc_calls[0]["destination_identity"] == "phone1"
    assert participant.rpc_calls[0]["method"] == "disable_camera"
    assert backend.calls == []


@pytest.mark.anyio
async def test_router_logs_rpc_tool_destination(caplog):
    backend = FakeBackend()
    participant = FakeParticipant()
    router = ToolRouter(
        backend=backend,
        local_participant=participant,
        user_context={"user_id": "u1", "user_identity": "phone1"},
    )

    with caplog.at_level(logging.INFO, logger="aisee-agent"):
        result = await router.call("deactivate_camera", {})

    assert result == {"status": "ok", "method": "disable_camera"}
    assert "tool_call start name=deactivate_camera" in caplog.text
    assert "livekit rpc start method=disable_camera destination=phone1" in caplog.text
    assert "tool_call ok name=deactivate_camera" in caplog.text


@pytest.mark.anyio
async def test_router_dispatches_hybrid_tool_and_app_action():
    backend = FakeBackend()
    participant = FakeParticipant()
    router = ToolRouter(
        backend=backend,
        local_participant=participant,
        user_context={"user_id": "u1", "user_identity": "phone1"},
    )

    async def tool_call(tool, user_id, params):
        return {
            "status": "ok",
            "schedule_action": {"params": {"title": "Thuoc", "when": "07:00"}},
        }

    backend.tool_call = tool_call

    result = await router.call(
        "set_medication_reminder",
        {"medicine_name": "A", "dosage": "1 vien", "times": ["07:00"], "frequency": "daily"},
    )

    assert result == {"status": "ok"}
    assert participant.rpc_calls[0]["method"] == "schedule_reminder"
    assert json.loads(participant.rpc_calls[0]["payload"]) == {"title": "Thuoc", "when": "07:00"}


@pytest.mark.anyio
async def test_router_dispatches_device_tool_over_transport_channel():
    backend = FakeBackend()
    transport = FakeTransport(result={"status": "ok", "method": "disable_camera"})
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1", "user_identity": "phone1"},
    )

    result = await router.call("deactivate_camera", {})

    assert result == {"status": "ok", "method": "disable_camera"}
    event, timeout = transport.tool_requests[0]
    assert event.type == "tool_request"
    assert event.payload == {"name": "disable_camera", "params": {}}
    assert timeout == 10.0
    assert backend.calls == []


@pytest.mark.anyio
async def test_router_returns_error_when_device_tool_times_out():
    backend = FakeBackend()
    transport = FakeTransport(block=True)
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1", "user_identity": "phone1"},
    )

    result = await router.call("deactivate_camera", {})

    assert result["status"] == "error"
    assert "timed out" in result["error"]


@pytest.mark.anyio
async def test_router_play_music_is_temporarily_disabled_without_device_request():
    backend = FakeBackend()

    async def tool_call_with_timeout(tool, user_id, params, timeout):
        raise AssertionError("play_music should not call backend while disabled")

    backend.tool_call_with_timeout = tool_call_with_timeout
    transport = FakeTransport(result={"status": "ok"})
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1", "user_identity": "phone1"},
    )

    result = await router.call("play_music", {"query": "song"})

    assert result["status"] == "disabled"
    assert "temporarily disabled" in result["message"]
    assert transport.tool_requests == []
    assert backend.calls == []


@pytest.mark.anyio
async def test_router_control_music_is_temporarily_disabled_without_device_request():
    backend = FakeBackend()
    transport = FakeTransport(result={"status": "ok"})
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1", "user_identity": "phone1"},
    )

    result = await router.call("control_music", {"action": "stop"})

    assert result["status"] == "disabled"
    assert transport.tool_requests == []


@pytest.mark.anyio
async def test_router_web_search_uses_search_client():
    backend = FakeBackend()
    search_client = FakeSearchClient()
    router = ToolRouter(
        backend=backend,
        search_client=search_client,
        user_context={"user_id": "u1"},
    )

    result = await router.call("web_search", {"query": "tin mới hôm nay", "max_results": 3})

    assert result["status"] == "ok"
    assert result["results"][0]["url"] == "https://example.test/result"
    assert search_client.calls == [("tin mới hôm nay", 3)]
    assert backend.calls == []
