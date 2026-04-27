import asyncio
import logging
from types import SimpleNamespace

import pytest

from gemini_bridge.session import GeminiEventHandler
from gemini_bridge.session import GeminiLiveSettings
from gemini_bridge.session import GoogleSDKBridge
from gemini_bridge.session import cancel_track_tasks
from gemini_bridge.session import send_wake_greeting


class FakeAudioPublisher:
    def __init__(self):
        self.audio = []
        self.cleared = False

    async def publish_pcm(self, data):
        self.audio.append(data)

    def clear(self):
        self.cleared = True


class FakeRouter:
    def __init__(self):
        self.calls = []

    async def call(self, name, args):
        self.calls.append((name, args))
        return {"status": "ok"}


class FakeSession:
    def __init__(self):
        self.responses = []
        self.realtime_text = []

    async def send_tool_response(self, *, function_responses):
        self.responses.extend(function_responses)

    async def send_realtime_input(self, *, text=None, **kwargs):
        self.realtime_text.append(text)


class StopReceiveLoop(Exception):
    pass


@pytest.mark.anyio
async def test_event_handler_publishes_all_audio_parts():
    audio = FakeAudioPublisher()
    handler = GeminiEventHandler(tool_router=FakeRouter(), audio_publisher=audio)
    part1 = SimpleNamespace(inline_data=SimpleNamespace(data=b"a", mime_type="audio/pcm"))
    part2 = SimpleNamespace(inline_data=SimpleNamespace(data=b"b", mime_type="audio/pcm"))
    message = SimpleNamespace(
        server_content=SimpleNamespace(
            interrupted=False,
            model_turn=SimpleNamespace(parts=[part1, part2]),
        ),
        tool_call=None,
    )

    await handler.handle(FakeSession(), message)

    assert audio.audio == [b"a", b"b"]


@pytest.mark.anyio
async def test_event_handler_routes_tool_calls_synchronously():
    router = FakeRouter()
    session = FakeSession()
    handler = GeminiEventHandler(tool_router=router, audio_publisher=FakeAudioPublisher())
    function_call = SimpleNamespace(id="call-1", name="deactivate_camera", args={})
    message = SimpleNamespace(
        server_content=None,
        tool_call=SimpleNamespace(function_calls=[function_call]),
    )

    await handler.handle(session, message)

    assert router.calls == [("deactivate_camera", {})]
    assert session.responses[0].id == "call-1"
    assert session.responses[0].name == "deactivate_camera"
    assert session.responses[0].response == {"status": "ok"}


@pytest.mark.anyio
async def test_event_handler_logs_tool_call_names(caplog):
    router = FakeRouter()
    session = FakeSession()
    handler = GeminiEventHandler(tool_router=router, audio_publisher=FakeAudioPublisher())
    function_call = SimpleNamespace(id="call-1", name="activate_camera", args={"reason": "test"})
    message = SimpleNamespace(
        server_content=None,
        tool_call=SimpleNamespace(function_calls=[function_call]),
    )

    with caplog.at_level(logging.INFO, logger="aisee-agent"):
        await handler.handle(session, message)

    assert "gemini tool_call received count=1 names=activate_camera" in caplog.text


@pytest.mark.anyio
async def test_event_handler_starts_batch_tool_calls_concurrently():
    first_started = asyncio.Event()
    second_started = asyncio.Event()
    release_first = asyncio.Event()

    class BlockingRouter:
        async def call(self, name, args):
            if name == "first_tool":
                first_started.set()
                await release_first.wait()
                return {"status": "ok", "name": name}
            second_started.set()
            return {"status": "ok", "name": name}

    session = FakeSession()
    handler = GeminiEventHandler(
        tool_router=BlockingRouter(),
        audio_publisher=FakeAudioPublisher(),
    )
    message = SimpleNamespace(
        server_content=None,
        tool_call=SimpleNamespace(
            function_calls=[
                SimpleNamespace(id="call-1", name="first_tool", args={}),
                SimpleNamespace(id="call-2", name="second_tool", args={}),
            ],
        ),
    )

    task = asyncio.create_task(handler.handle(session, message))
    await asyncio.wait_for(first_started.wait(), timeout=1)
    try:
        await asyncio.wait_for(second_started.wait(), timeout=0.05)
    finally:
        release_first.set()
        await task

    assert [response.id for response in session.responses] == ["call-1", "call-2"]


@pytest.mark.anyio
async def test_event_handler_returns_error_response_when_one_tool_raises():
    class PartiallyFailingRouter:
        async def call(self, name, args):
            if name == "failing_tool":
                raise RuntimeError("tool exploded")
            return {"status": "ok"}

    session = FakeSession()
    handler = GeminiEventHandler(
        tool_router=PartiallyFailingRouter(),
        audio_publisher=FakeAudioPublisher(),
    )
    message = SimpleNamespace(
        server_content=None,
        tool_call=SimpleNamespace(
            function_calls=[
                SimpleNamespace(id="call-1", name="failing_tool", args={}),
                SimpleNamespace(id="call-2", name="healthy_tool", args={}),
            ],
        ),
    )

    await handler.handle(session, message)

    assert [response.id for response in session.responses] == ["call-1", "call-2"]
    assert session.responses[0].response == {
        "status": "error",
        "error": "tool exploded",
    }
    assert session.responses[1].response == {"status": "ok"}


@pytest.mark.anyio
async def test_cancel_track_tasks_tolerates_self_removing_callbacks():
    async def wait_forever():
        try:
            await asyncio.sleep(60)
        finally:
            return None

    task = asyncio.create_task(wait_forever())
    tasks = {task}
    task.add_done_callback(tasks.discard)

    await cancel_track_tasks(tasks)

    assert tasks == set()


@pytest.mark.anyio
async def test_send_wake_greeting_uses_realtime_input_text():
    session = FakeSession()

    await send_wake_greeting(session)

    assert session.realtime_text == [
        "Người dùng vừa đánh thức bạn. Hãy chào ngắn gọn bằng tiếng Việt và hỏi họ cần hỗ trợ gì."
    ]


@pytest.mark.anyio
async def test_receive_loop_keeps_session_open_across_turns():
    class FakeMultiTurnSession:
        def __init__(self):
            self.receive_calls = 0

        async def receive(self):
            self.receive_calls += 1
            if self.receive_calls == 1:
                yield SimpleNamespace(
                    server_content=SimpleNamespace(turn_complete=True, model_turn=None),
                    tool_call=None,
                )
                return
            raise StopReceiveLoop

    published_states = []

    class Local:
        async def set_attributes(self, attrs):
            published_states.append(attrs["lk.agent.state"])

    bridge = GoogleSDKBridge(ctx=None, logger=SimpleNamespace(info=lambda *a, **k: None, warning=lambda *a, **k: None))
    session = FakeMultiTurnSession()

    with pytest.raises(StopReceiveLoop):
        await bridge._receive_loop(
            session,
            GeminiEventHandler(tool_router=FakeRouter(), audio_publisher=FakeAudioPublisher()),
            Local(),
        )

    assert session.receive_calls == 2
    assert published_states == ["idle"]


def test_live_config_prompt_includes_exact_camera_tool_rules():
    settings = GeminiLiveSettings(
        api_key="x",
        model="m",
        voice="v",
        thinking_level="low",
        input_sample_rate=16000,
    )

    config = settings.build_live_config_with_prompt("base prompt")

    assert "activate_camera" in config.system_instruction
    assert "deactivate_camera" in config.system_instruction
    assert "tắt đi" in config.system_instruction
