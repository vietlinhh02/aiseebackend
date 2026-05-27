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

    result = await router.call("send_chat_message", {"contact_name": "Nam", "message": "Xin chào"})

    assert result == {"status": "ok", "tool": "send_chat_message"}
    assert backend.calls == [
        ("tool_call", "send_chat_message", "u1", {"contact_name": "Nam", "message": "Xin chào"})
    ]
    assert participant.rpc_calls == []


@pytest.mark.anyio
async def test_router_dispatches_family_status_tool():
    backend = FakeBackend()
    participant = FakeParticipant()
    router = ToolRouter(
        backend=backend,
        local_participant=participant,
        user_context={"user_id": "u1", "user_identity": "phone1"},
    )

    result = await router.call("list_family_contacts", {})

    assert result == {"status": "ok", "tool": "list_family_contacts"}
    assert backend.calls == [("tool_call", "list_family_contacts", "u1", {})]
    assert participant.rpc_calls == []


@pytest.mark.anyio
async def test_router_family_call_alias_prepares_and_starts_call():
    backend = FakeBackend()
    transport = FakeTransport(result={"status": "answered"})
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1", "user_identity": "phone1"},
    )

    async def tool_call(tool, user_id, params):
        backend.calls.append(("tool_call", tool, user_id, params))
        return {
            "status": "prepared",
            "mode": "family",
            "room_name": "call-1",
            "token": "caller-token",
            "call_type": "voice",
            "contact_id": "f1",
            "contact_name": "Me",
            "contact_role": "family",
            "answer_timeout_seconds": 30,
        }

    backend.tool_call = tool_call

    result = await router.call("make_voice_call", {"contact_name": "Me"})

    assert result["status"] == "answered"
    assert backend.calls == [
        ("tool_call", "call_family_contact", "u1", {"contact_name": "Me", "call_type": "voice"})
    ]
    event, timeout = transport.tool_requests[0]
    assert event.payload["name"] == "start_call"
    assert event.payload["params"]["room_name"] == "call-1"
    assert timeout == 40.0


@pytest.mark.anyio
async def test_router_family_no_answer_preserves_fallback_contacts():
    backend = FakeBackend()
    transport = FakeTransport(result={"status": "no_answer"})
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1", "user_identity": "phone1"},
    )

    async def tool_call(tool, user_id, params):
        backend.calls.append(("tool_call", tool, user_id, params))
        return {
            "status": "prepared",
            "mode": "family",
            "room_name": "call-2",
            "token": "caller-token",
            "call_type": "voice",
            "contact_id": "f1",
            "contact_name": "Me",
            "contact_role": "family",
            "answer_timeout_seconds": 30,
            "fallback_contacts": [
                {"id": "f2", "display_name": "Chi Lan", "relationship": "Chi"}
            ],
        }

    backend.tool_call = tool_call

    result = await router.call("call_family_contact", {"contact_name": "Me"})

    assert result["status"] == "no_answer"
    assert result["fallback_contacts"] == [
        {"id": "f2", "display_name": "Chi Lan", "relationship": "Chi"}
    ]


@pytest.mark.anyio
async def test_router_support_call_alias_uses_timeout_backend_call():
    backend = FakeBackend()
    transport = FakeTransport(result={"status": "answered"})
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1", "user_identity": "phone1"},
    )

    async def tool_call_with_timeout(tool, user_id, params, timeout):
        backend.calls.append(("tool_call_with_timeout", tool, user_id, params, timeout))
        return {
            "status": "prepared",
            "mode": "volunteer",
            "room_name": "vol-1",
            "token": "caller-token",
            "call_type": "voice",
            "contact_id": "v1",
            "contact_name": "Tình nguyện viên",
            "contact_role": "volunteer",
            "answer_timeout_seconds": 12,
        }

    backend.tool_call_with_timeout = tool_call_with_timeout

    result = await router.call("call_support_contact", {})

    assert result["status"] == "answered"
    assert backend.calls == [
        ("tool_call_with_timeout", "call_support_contact", "u1", {"contact_name": "", "call_type": "voice"}, 45.0)
    ]
    event, timeout = transport.tool_requests[0]
    assert event.payload["name"] == "start_call"
    assert event.payload["params"]["contact_role"] == "volunteer"
    assert timeout == 22.0


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
async def test_router_blocks_camera_without_explicit_consent():
    backend = FakeBackend()
    participant = FakeParticipant()
    router = ToolRouter(
        backend=backend,
        local_participant=participant,
        user_context={"user_id": "u1", "user_identity": "phone1"},
    )

    result = await router.call("activate_camera", {"reason": "Đọc nhãn thuốc"})

    assert result["status"] == "blocked"
    assert result["error"] == "camera_consent_required"
    assert participant.rpc_calls == []
    assert backend.calls == []

@pytest.mark.anyio
async def test_router_allows_camera_after_explicit_confirmation():
    backend = FakeBackend()
    participant = FakeParticipant()
    router = ToolRouter(
        backend=backend,
        local_participant=participant,
        user_context={"user_id": "u1", "user_identity": "phone1"},
    )

    result = await router.call(
        "activate_camera",
        {
            "reason": "Đọc nhãn thuốc",
            "consent_source": "explicit_confirmation",
            "consent_phrase": "uh mở đi",
        },
    )

    assert result == {"status": "ok", "method": "enable_camera"}
    assert participant.rpc_calls[0]["method"] == "enable_camera"
    assert backend.calls[0] == ("check_quota", "u1", "camera_activations", 1)
    assert backend.calls[1][0] == "record_usage"
    assert backend.calls[1][-1] == {
        "reason": "Đọc nhãn thuốc",
        "consent_source": "explicit_confirmation",
        "consent_phrase": "uh mở đi",
    }

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
        {"medicine_name": "A", "dosage": "1 viên", "times": ["07:00"], "frequency": "daily"},
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
    assert timeout == 5.0
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
async def test_router_play_music_routes_to_backend():
    backend = FakeBackend()
    transport = FakeTransport(result={"status": "ok"})
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1", "user_identity": "phone1"},
    )

    result = await router.call("play_music", {"query": "song"})

    assert result["status"] == "ok"
    assert len(backend.calls) == 1
    assert backend.calls[0] == (
        "tool_call",
        "play_music",
        "u1",
        {"query": "song"},
    )


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


@pytest.mark.anyio
async def test_router_escalate_to_family_starts_prepared_call():
    backend = FakeBackend()
    transport = FakeTransport(result={"status": "answered"})
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1", "user_identity": "phone1"},
    )

    async def tool_call(tool, user_id, params):
        backend.calls.append(("tool_call", tool, user_id, params))
        return {
            "status": "prepared",
            "mode": "family",
            "room_name": "esc-room",
            "token": "caller-token",
            "contact_id": "f1",
            "contact_name": "Mom",
            "contact_role": "family",
            "answer_timeout_seconds": 30,
        }

    backend.tool_call = tool_call

    result = await router.call(
        "escalate_to_family",
        {"contact_name": "Mom", "skip": ["Dad", ""]},
    )

    assert result["status"] == "answered"
    assert backend.calls == [
        (
            "tool_call",
            "escalate_to_family",
            "u1",
            {"contact_name": "Mom", "skip": ["Dad"]},
        )
    ]
    event, _ = transport.tool_requests[0]
    assert event.payload["name"] == "start_call"
    assert event.payload["params"]["room_name"] == "esc-room"
    assert event.payload["params"]["contact_name"] == "Mom"


@pytest.mark.anyio
async def test_router_escalate_to_family_no_contacts_skips_device_call():
    backend = FakeBackend()
    transport = FakeTransport(result={"status": "should_not_be_called"})
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1"},
    )

    async def tool_call(tool, user_id, params):
        backend.calls.append(("tool_call", tool, user_id, params))
        return {"status": "no_contacts"}

    backend.tool_call = tool_call

    result = await router.call("escalate_to_family", {})

    assert result == {"status": "no_contacts"}
    assert transport.tool_requests == []


@pytest.mark.anyio
async def test_router_answer_incoming_call_dispatches_to_device():
    backend = FakeBackend()
    transport = FakeTransport(result={"status": "answered", "room_name": "call-1"})
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1"},
    )

    result = await router.call("answer_incoming_call", {})

    assert result == {"status": "answered", "room_name": "call-1"}
    assert len(transport.tool_requests) == 1
    event, _ = transport.tool_requests[0]
    assert event.payload["name"] == "answer_incoming_call"


@pytest.mark.anyio
async def test_router_decline_incoming_call_dispatches_to_device():
    backend = FakeBackend()
    transport = FakeTransport(result={"status": "declined"})
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1"},
    )

    result = await router.call("decline_incoming_call", {})

    assert result == {"status": "declined"}
    assert len(transport.tool_requests) == 1
    event, _ = transport.tool_requests[0]
    assert event.payload["name"] == "decline_incoming_call"
