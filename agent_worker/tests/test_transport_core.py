from types import SimpleNamespace

import pytest
from google.genai import errors as genai_errors

from gemini_bridge.core import GeminiLiveSettings, GeminiTransportSession
from gemini_bridge.state import AgentState
from gemini_bridge.transport import InboundAudioEvent, ToolRequestEvent, TransportStatePublisher


class FakeTransport:
    def __init__(self, inbound_events=None):
        self.inbound_events = list(inbound_events or [])
        self.ready = 0
        self.states = []
        self.audio = []
        self.audio_interrupts = 0
        self.tool_requests = []

    async def emit_ready(self):
        self.ready += 1

    async def emit_state(self, state: AgentState):
        self.states.append(state)

    async def emit_audio(self, pcm: bytes):
        self.audio.append(pcm)

    async def emit_audio_interrupted(self):
        self.audio_interrupts += 1

    async def send_tool_request(self, event: ToolRequestEvent, timeout=None):
        self.tool_requests.append((event, timeout))
        return {"status": "ok"}

    async def receive(self):
        for event in self.inbound_events:
            yield event


class FakeRouter:
    def __init__(self):
        self.calls = []

    async def call(self, name, args):
        self.calls.append((name, args))
        return {"status": "ok"}


class FakeGeminiSession:
    def __init__(self, messages=None):
        self.messages = list(messages or [])
        self.realtime_inputs = []
        self.tool_responses = []

    async def send_realtime_input(self, *, audio=None, video=None, text=None):
        self.realtime_inputs.append(
            {
                "audio": audio,
                "video": video,
                "text": text,
            }
        )

    async def send_tool_response(self, *, function_responses):
        self.tool_responses.extend(function_responses)

    async def receive(self):
        messages = list(self.messages)
        self.messages.clear()
        for message in messages:
            yield message


class FakeNormallyClosedGeminiSession(FakeGeminiSession):
    async def receive(self):
        raise genai_errors.APIError(1000, None, None)
        yield


def build_settings():
    return GeminiLiveSettings(
        api_key="test-key",
        model="gemini-live",
        voice="Aoede",
        thinking_level="low",
        input_sample_rate=16000,
    )


@pytest.mark.anyio
async def test_transport_session_sends_inbound_pcm_to_gemini_audio_blob():
    transport = FakeTransport(inbound_events=[InboundAudioEvent(pcm=b"\x01\x02")])
    session = FakeGeminiSession()
    live_session = GeminiTransportSession(
        settings=build_settings(),
        transport=transport,
        tool_router=FakeRouter(),
    )

    await live_session.run(session)

    assert transport.ready == 1
    assert transport.states[:2] == [AgentState.IDLE, AgentState.LISTENING]
    audio_blob = session.realtime_inputs[0]["audio"]
    assert audio_blob.data == b"\x01\x02"
    assert audio_blob.mime_type == "audio/pcm;rate=16000"


@pytest.mark.anyio
async def test_transport_session_emits_outbound_gemini_audio_to_transport():
    transport = FakeTransport()
    message = SimpleNamespace(
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
    session = FakeGeminiSession(messages=[message])
    live_session = GeminiTransportSession(
        settings=build_settings(),
        transport=transport,
        tool_router=FakeRouter(),
    )

    await live_session.run(session)

    assert transport.audio == [b"\x03\x04"]
    assert AgentState.SPEAKING in transport.states
    assert transport.states[-1] == AgentState.IDLE


@pytest.mark.anyio
async def test_transport_session_emits_audio_interrupted_when_user_barges_in():
    transport = FakeTransport()
    message = SimpleNamespace(
        server_content=SimpleNamespace(
            interrupted=True,
            turn_complete=True,
            model_turn=None,
        ),
        tool_call=None,
    )
    session = FakeGeminiSession(messages=[message])
    live_session = GeminiTransportSession(
        settings=build_settings(),
        transport=transport,
        tool_router=FakeRouter(),
    )

    await live_session.run(session)

    assert transport.audio_interrupts == 1
    assert transport.audio == []
    assert transport.states[-1] == AgentState.IDLE


@pytest.mark.anyio
async def test_transport_session_treats_live_1000_close_as_normal_shutdown():
    transport = FakeTransport()
    session = FakeNormallyClosedGeminiSession()
    live_session = GeminiTransportSession(
        settings=build_settings(),
        transport=transport,
        tool_router=FakeRouter(),
    )

    await live_session.run(session)

    assert transport.ready == 1
    assert transport.states == [AgentState.IDLE]


@pytest.mark.anyio
async def test_state_publisher_holds_speaking_over_mic_listening_chunks():
    transport = FakeTransport()
    publisher = TransportStatePublisher(
        transport=transport,
        speaking_hold_seconds=60,
    )

    await publisher.publish(AgentState.IDLE)
    await publisher.publish(AgentState.SPEAKING)
    await publisher.publish(AgentState.LISTENING)
    await publisher.publish(AgentState.LISTENING)
    await publisher.publish(AgentState.IDLE)

    assert transport.states == [
        AgentState.IDLE,
        AgentState.SPEAKING,
        AgentState.IDLE,
    ]
