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
    # ``make_voice_call`` explicitly opts out of the family-call video
    # default, so the backend still receives ``call_type=voice``.
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
async def test_router_family_call_defaults_to_video_when_call_type_missing():
    """Family calls must be video by default — the user/product spec requires
    the receiving family member to see the user. Missing/unknown ``call_type``
    must not silently fall back to voice."""
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
            "room_name": "call-vid",
            "token": "caller-token",
            "call_type": params.get("call_type", "video"),
            "contact_id": "f1",
            "contact_name": "Me",
            "contact_role": "family",
            "answer_timeout_seconds": 30,
        }

    backend.tool_call = tool_call

    # 1. AI doesn't send call_type at all → must default to video.
    result = await router.call("call_family_contact", {"contact_name": "Me"})
    assert result["status"] == "answered"
    assert backend.calls[0] == (
        "tool_call",
        "call_family_contact",
        "u1",
        {"contact_name": "Me", "call_type": "video"},
    )

    # 2. AI sends an unknown value → must still default to video.
    backend.calls.clear()
    result = await router.call(
        "call_family_contact",
        {"contact_name": "Me", "call_type": "something-weird"},
    )
    assert backend.calls[0][3]["call_type"] == "video"

    # 3. AI explicitly asks for voice → must respect it.
    backend.calls.clear()
    result = await router.call(
        "call_family_contact",
        {"contact_name": "Me", "call_type": "voice"},
    )
    assert backend.calls[0][3]["call_type"] == "voice"


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
            "call_type": "video",
            "contact_id": "v1",
            "contact_name": "Tình nguyện viên",
            "contact_role": "volunteer",
            "answer_timeout_seconds": 12,
        }

    backend.tool_call_with_timeout = tool_call_with_timeout

    result = await router.call("call_support_contact", {})

    assert result["status"] == "answered"
    # Support calls are always forced to video — the volunteer needs to see.
    assert backend.calls == [
        ("tool_call_with_timeout", "call_support_contact", "u1", {"contact_name": "", "call_type": "video"}, 45.0)
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

    assert result["status"] == "error"
    assert result["code"] == "CAMERA_CONSENT_REQUIRED"
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

    assert result["status"] == "error"
    assert result["code"] == "DEVICE_UNAVAILABLE"
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


# ─────────────────────────────────────────────────────────────────────
# In-app walking navigation tests
# ─────────────────────────────────────────────────────────────────────


class FakeNavigationClient:
    """Pretend Mapbox client: returns a 2-step route near (10,20)."""
    is_configured = True

    async def geocode(self, query, *, proximity=None):
        return {
            "status": "ok",
            "lat": 10.1001,
            "lng": 20.2001,
            "name": query,
        }

    async def walking_route(self, origin, destination):
        from gemini_bridge.mapbox_navigation import (
            GeoPoint, NavigationStep, NavigationRoute,
        )
        route = NavigationRoute(
            steps=[
                NavigationStep(
                    index=0,
                    name="Nguyen Hue",
                    instruction_text="Head north on Nguyen Hue",
                    voice_text="Walk north on Nguyen Hue for 100 meters",
                    maneuver="depart",
                    modifier=None,
                    distance_m=100.0,
                    duration_s=80.0,
                    location=GeoPoint(lat=origin.lat + 0.0005, lng=origin.lng),
                ),
                NavigationStep(
                    index=1,
                    name="Le Loi",
                    instruction_text="Turn right onto Le Loi",
                    voice_text="Turn right onto Le Loi",
                    maneuver="turn",
                    modifier="right",
                    distance_m=50.0,
                    duration_s=40.0,
                    location=destination,
                ),
            ],
            total_distance_m=150.0,
            total_duration_s=120.0,
            geometry=[
                GeoPoint(lat=origin.lat, lng=origin.lng),
                GeoPoint(lat=origin.lat + 0.0003, lng=origin.lng),
                destination,
            ],
            destination_name="Bệnh viện",
            destination_point=destination,
        )
        return {"status": "ok", "route": route}


@pytest.mark.anyio
async def test_router_start_walking_navigation_creates_session():
    backend = FakeBackend()
    transport = FakeTransport(
        result={"status": "ok", "data": {"lat": 10.0, "lng": 20.0}}
    )
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1", "user_identity": "phone1"},
        navigation_client=FakeNavigationClient(),
    )

    result = await router.call("start_walking_navigation", {"destination": "Bệnh viện Chợ Rẫy"})

    assert result["status"] == "ok"
    assert result["destination_name"] == "Bệnh viện Chợ Rẫy"
    assert result["total_steps"] == 2
    assert result["first_step_name"] == "Nguyen Hue"
    assert "navigation_state" in router.user_context
    assert router.user_context["navigation_state"] is not None


@pytest.mark.anyio
async def test_router_start_walking_navigation_errors_when_token_missing():
    class Unconfigured:
        is_configured = False
        async def geocode(self, *a, **k): return {"status": "error"}
        async def walking_route(self, *a, **k): return {"status": "error"}

    backend = FakeBackend()
    transport = FakeTransport(
        result={"status": "ok", "data": {"lat": 10.0, "lng": 20.0}}
    )
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1"},
        navigation_client=Unconfigured(),
    )
    result = await router.call("start_walking_navigation", {"destination": "X"})
    assert result["status"] == "error"
    assert "chỉ đường chưa được cấu hình" in result["error"]


@pytest.mark.anyio
async def test_router_announce_navigation_step_without_active_session_errors():
    backend = FakeBackend()
    transport = FakeTransport()
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1"},
        navigation_client=FakeNavigationClient(),
    )
    result = await router.call("announce_navigation_step",
                               {"current_lat": 10.0, "current_lng": 20.0})
    assert result["status"] == "error"
    assert "chưa có cuốc chỉ đường" in result["error"]


@pytest.mark.anyio
async def test_router_full_navigation_lifecycle():
    backend = FakeBackend()
    transport = FakeTransport(
        result={"status": "ok", "data": {"lat": 10.0, "lng": 20.0}}
    )
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1", "user_identity": "phone1"},
        navigation_client=FakeNavigationClient(),
    )

    # 1. Start
    start = await router.call("start_walking_navigation",
                              {"destination": "Bệnh viện"})
    assert start["status"] == "ok"

    # 2. Announce on-route
    step = await router.call("announce_navigation_step",
                             {"current_lat": 10.0, "current_lng": 20.0})
    assert step["status"] == "ok"
    assert step["is_deviated"] is False
    assert step["should_alert_deviation"] is False
    assert step["progress_pct"] >= 0

    # 3. Stop
    stop = await router.call("stop_navigation", {"outcome": "completed"})
    assert stop["status"] == "ok"
    assert stop["was_active"] is True
    assert stop["outcome"] == "completed"
    assert router.user_context["navigation_state"] is None


@pytest.mark.anyio
async def test_router_stop_navigation_when_idle_returns_was_active_false():
    backend = FakeBackend()
    transport = FakeTransport()
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1"},
        navigation_client=FakeNavigationClient(),
    )
    result = await router.call("stop_navigation", {"outcome": "completed"})
    assert result["status"] == "ok"
    assert result["was_active"] is False


@pytest.mark.anyio
async def test_router_announce_navigation_step_detects_deviation():
    """User đi xa khỏi route > 50m → is_deviated=True + should_alert_deviation=True (lần đầu)."""
    backend = FakeBackend()
    transport = FakeTransport(
        result={"status": "ok", "data": {"lat": 10.0, "lng": 20.0}}
    )
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1", "user_identity": "phone1"},
        navigation_client=FakeNavigationClient(),
    )

    await router.call("start_walking_navigation", {"destination": "Bệnh viện"})

    # 1.0000, 20.0000 → lệch ~11km từ route, is_deviated=True ngay
    step = await router.call(
        "announce_navigation_step",
        {"current_lat": 1.0, "current_lng": 1.0},
    )
    assert step["status"] == "ok"
    assert step["is_deviated"] is True
    assert step["deviation_m"] > 1000
    assert step["should_alert_deviation"] is True  # first alert, no cooldown

    # Gọi lại ngay → cooldown nên should_alert_deviation=False
    step2 = await router.call(
        "announce_navigation_step",
        {"current_lat": 1.0, "current_lng": 1.0},
    )
    assert step2["should_alert_deviation"] is False
    assert step2["deviation_count"] == 1


@pytest.mark.anyio
async def test_router_start_walking_navigation_sends_set_camera_fps_command():
    """Verify _start_walking_navigation auto-bumps camera FPS to 600ms."""
    backend = FakeBackend()
    transport = FakeTransport(
        result={"status": "ok", "data": {"lat": 10.0, "lng": 20.0}}
    )
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1", "user_identity": "phone1"},
        navigation_client=FakeNavigationClient(),
    )

    await router.call("start_walking_navigation", {"destination": "Bệnh viện"})

    # Lấy tất cả device requests đã gửi
    device_names = [e[0].payload.get("name") for e in transport.tool_requests]
    assert "set_camera_fps" in device_names, (
        f"expected set_camera_fps, got {device_names}"
    )
    # Verify interval_ms=600
    fps_event = next(
        e for e in transport.tool_requests
        if e[0].payload.get("name") == "set_camera_fps"
    )
    assert fps_event[0].payload["params"]["interval_ms"] == 600


@pytest.mark.anyio
async def test_router_stop_navigation_restores_camera_fps_to_1s():
    """Verify _stop_navigation sends set_camera_fps với interval_ms=1000."""
    backend = FakeBackend()
    transport = FakeTransport(
        result={"status": "ok", "data": {"lat": 10.0, "lng": 20.0}}
    )
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1", "user_identity": "phone1"},
        navigation_client=FakeNavigationClient(),
    )

    await router.call("start_walking_navigation", {"destination": "X"})
    await router.call("stop_navigation", {"outcome": "cancelled"})

    fps_events = [
        e for e in transport.tool_requests
        if e[0].payload.get("name") == "set_camera_fps"
    ]
    # Có ít nhất 2 events: start (600ms) + stop (1000ms)
    intervals = [e[0].payload["params"]["interval_ms"] for e in fps_events]
    assert 600 in intervals
    assert 1000 in intervals


@pytest.mark.anyio
async def test_router_start_walking_navigation_refuses_too_far_destination():
    """Nếu quãng đường > NAV_MAX_DISTANCE_M (2000m), từ chối + đề xuất phương án khác."""
    class FarAwayClient(FakeNavigationClient):
        async def walking_route(self, origin, destination):
            from gemini_bridge.mapbox_navigation import (
                NavigationStep, NavigationRoute,
            )
            # Route 5km — quá xa
            route = NavigationRoute(
                steps=[
                    NavigationStep(
                        index=0, name="Xa", instruction_text="Walk 5km",
                        voice_text="Walk 5km", maneuver="depart", modifier=None,
                        distance_m=5000.0, duration_s=4000.0,
                        location=destination,
                    ),
                ],
                total_distance_m=5000.0,
                total_duration_s=4000.0,
                geometry=[origin, destination],
                destination_name="Bệnh viện xa",
                destination_point=destination,
            )
            return {"status": "ok", "route": route}

    backend = FakeBackend()
    transport = FakeTransport(
        result={"status": "ok", "data": {"lat": 10.0, "lng": 20.0}}
    )
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1", "user_identity": "phone1"},
        navigation_client=FarAwayClient(),
    )

    result = await router.call("start_walking_navigation",
                              {"destination": "Bệnh viện xa"})

    assert result["status"] == "refused_too_far"
    assert result["total_distance_m"] == 5000.0
    assert result["max_distance_m"] == 2000.0
    assert "xe ôm" in result["hint_for_agent"]
    # KHÔNG gọi SHOW_NAVIGATION_SCREEN
    shown = [e for e in transport.tool_requests
             if e[0].payload.get("name") == "show_navigation_screen"]
    assert shown == []
    # KHÔNG set nav state
    assert router.user_context.get("navigation_state") is None


@pytest.mark.anyio
async def test_router_start_walking_navigation_sends_show_navigation_screen():
    """Verify _start_walking_navigation gửi show_navigation_screen device command
    với đầy đủ destination + step info."""
    backend = FakeBackend()
    transport = FakeTransport(
        result={"status": "ok", "data": {"lat": 10.0, "lng": 20.0}}
    )
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1", "user_identity": "phone1"},
        navigation_client=FakeNavigationClient(),
    )

    await router.call("start_walking_navigation", {"destination": "Bệnh viện"})

    show_events = [
        e for e in transport.tool_requests
        if e[0].payload.get("name") == "show_navigation_screen"
    ]
    assert len(show_events) == 1
    params = show_events[0][0].payload["params"]
    assert params["destination_name"] == "Bệnh viện"
    assert params["total_distance_m"] > 0
    assert params["total_steps"] == 2
    assert params["first_step_name"] == "Nguyen Hue"


@pytest.mark.anyio
async def test_router_stop_navigation_sends_hide_navigation_screen():
    """Verify _stop_navigation gửi hide_navigation_screen device command."""
    backend = FakeBackend()
    transport = FakeTransport(
        result={"status": "ok", "data": {"lat": 10.0, "lng": 20.0}}
    )
    router = ToolRouter(
        backend=backend,
        device_channel=TransportDeviceCommandChannel(transport=transport),
        user_context={"user_id": "u1", "user_identity": "phone1"},
        navigation_client=FakeNavigationClient(),
    )

    await router.call("start_walking_navigation", {"destination": "X"})
    await router.call("stop_navigation", {"outcome": "completed"})

    hide_events = [
        e for e in transport.tool_requests
        if e[0].payload.get("name") == "hide_navigation_screen"
    ]
    assert len(hide_events) == 1
    assert hide_events[0][0].payload["params"]["outcome"] == "completed"
