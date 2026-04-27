"""Transport abstractions for Gemini Live sessions without LiveKit."""

from __future__ import annotations

import asyncio
import time
import uuid
from dataclasses import dataclass
from typing import Any, AsyncIterator, Protocol

from .state import AgentState


@dataclass(frozen=True, slots=True)
class InboundTextEvent:
    text: str


@dataclass(frozen=True, slots=True)
class InboundAudioEvent:
    pcm: bytes


@dataclass(frozen=True, slots=True)
class InboundVideoEvent:
    jpeg: bytes


@dataclass(frozen=True, slots=True)
class InboundControlEvent:
    action: str
    payload: dict[str, Any]


InboundTransportEvent = (
    InboundTextEvent | InboundAudioEvent | InboundVideoEvent | InboundControlEvent
)


@dataclass(frozen=True, slots=True)
class ToolRequestEvent:
    request_id: str
    name: str
    params: dict[str, Any]
    type: str = "tool_request"

    @property
    def payload(self) -> dict[str, Any]:
        return {
            "name": self.name,
            "params": self.params,
        }


class GeminiTransport(Protocol):
    async def emit_ready(self) -> None: ...

    async def emit_state(self, state: AgentState) -> None: ...

    async def emit_audio(self, pcm: bytes) -> None: ...

    async def emit_audio_interrupted(self) -> None: ...

    async def send_tool_request(
        self,
        event: ToolRequestEvent,
        timeout: float | None = None,
    ) -> dict: ...

    def receive(self) -> AsyncIterator[InboundTransportEvent]: ...


class DeviceCommandChannel(Protocol):
    async def request(
        self,
        command: str,
        payload: dict[str, Any],
        timeout: float | None = None,
    ) -> dict: ...


@dataclass(slots=True)
class TransportAudioSink:
    transport: GeminiTransport

    async def publish_pcm(self, pcm: bytes) -> None:
        await self.transport.emit_audio(pcm)

    def clear(self):
        return self.transport.emit_audio_interrupted()


@dataclass(slots=True)
class TransportStatePublisher:
    transport: GeminiTransport
    speaking_hold_seconds: float = 1.25
    _current_state: AgentState | None = None
    _speaking_hold_until: float = 0.0

    async def publish(self, state: AgentState) -> None:
        now = time.monotonic()
        if state == AgentState.SPEAKING:
            self._speaking_hold_until = now + self.speaking_hold_seconds
        elif state == AgentState.LISTENING and now < self._speaking_hold_until:
            return

        if self._current_state == state:
            return
        self._current_state = state
        await self.transport.emit_state(state)


@dataclass(slots=True)
class TransportDeviceCommandChannel:
    transport: GeminiTransport

    async def request(
        self,
        command: str,
        payload: dict[str, Any],
        timeout: float | None = None,
    ) -> dict:
        event = ToolRequestEvent(
            request_id=f"tool-{uuid.uuid4()}",
            name=command,
            params=payload,
        )
        request = self.transport.send_tool_request(event, timeout=timeout)
        if timeout is None:
            return await request
        try:
            return await asyncio.wait_for(request, timeout=timeout)
        except TimeoutError as exc:
            raise TimeoutError(f"device tool {command} timed out after {timeout:.1f}s") from exc
