import json

import httpx
import pytest

from backend import BackendClient


@pytest.mark.anyio
async def test_tool_call_uses_typed_payload_and_internal_header():
    captured = {}

    async def handler(request: httpx.Request) -> httpx.Response:
        captured["path"] = request.url.path
        captured["headers"] = dict(request.headers)
        captured["json"] = json.loads(request.content.decode())
        return httpx.Response(200, json={"status": "ok"})

    client = httpx.AsyncClient(
        transport=httpx.MockTransport(handler),
        base_url="http://backend/api/internal",
    )
    backend = BackendClient(
        "http://backend/api/internal",
        "secret",
        client=client,
    )

    try:
        result = await backend.tool_call(
            "make_voice_call",
            "user-1",
            {"contact_name": "Lan"},
        )
    finally:
        await client.aclose()

    assert result == {"status": "ok"}
    assert captured["path"] == "/api/internal/tool-call"
    assert captured["headers"]["x-internal-key"] == "secret"
    assert captured["json"] == {
        "tool": "make_voice_call",
        "user_id": "user-1",
        "params": {"contact_name": "Lan"},
    }


@pytest.mark.anyio
async def test_record_tracking_event_uses_shared_post_json():
    captured = {}

    async def handler(request: httpx.Request) -> httpx.Response:
        captured["path"] = request.url.path
        captured["json"] = json.loads(request.content.decode())
        return httpx.Response(200, json={"event_id": "evt-1"})

    client = httpx.AsyncClient(
        transport=httpx.MockTransport(handler),
        base_url="http://backend/api/internal",
    )
    backend = BackendClient(
        "http://backend/api/internal",
        "secret",
        client=client,
    )

    try:
        result = await backend.record_tracking_event(
            "ai_session_started",
            "ai_ops",
            user_id="user-1",
            metadata={"session_id": "s1"},
        )
    finally:
        await client.aclose()

    assert result == {"event_id": "evt-1"}
    assert captured["path"] == "/api/internal/tracking/events"
    assert captured["json"] == {
        "event_type": "ai_session_started",
        "category": "ai_ops",
        "user_id": "user-1",
        "severity": "",
        "source": "agent_worker",
        "source_id": "unknown",
        "metadata": {"session_id": "s1"},
    }
