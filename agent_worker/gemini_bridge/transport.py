"""Transport abstractions for Gemini Live sessions without LiveKit."""

from __future__ import annotations

import asyncio
import time
import uuid
from dataclasses import dataclass
from typing import Any, AsyncIterator, Protocol

from .state import AgentState


def _get_running_loop() -> asyncio.AbstractEventLoop | None:
    try:
        return asyncio.get_running_loop()
    except RuntimeError:
        return None


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
    thinking_delay_seconds: float = 0.3
    _current_state: AgentState | None = None
    _speaking_hold_until: float = 0.0
    _last_audio_at: float = 0.0
    _thinking_timer: object | None = None

    async def publish(self, state: AgentState) -> None:
        now = time.monotonic()
        if state == AgentState.SPEAKING:
            self._speaking_hold_until = now + self.speaking_hold_seconds
            self._cancel_thinking_timer()
        elif state == AgentState.LISTENING and now < self._speaking_hold_until:
            return

        if self._current_state == state:
            return
        self._current_state = state
        await self.transport.emit_state(state)

    def on_audio_input(self) -> None:
        """Gọi khi nhận audio input từ user — reset thinking timer."""
        self._last_audio_at = time.monotonic()
        if self._current_state == AgentState.LISTENING:
            self._schedule_thinking()

    def _schedule_thinking(self) -> None:
        self._cancel_thinking_timer()
        loop = _get_running_loop()
        if loop is None:
            return
        self._thinking_timer = loop.call_later(
            self.thinking_delay_seconds,
            lambda: asyncio.ensure_future(self._maybe_publish_thinking()),
        )

    def _cancel_thinking_timer(self) -> None:
        timer = self._thinking_timer
        if timer is not None:
            timer.cancel()
            self._thinking_timer = None

    async def _maybe_publish_thinking(self) -> None:
        self._thinking_timer = None
        elapsed = time.monotonic() - self._last_audio_at
        if elapsed >= self.thinking_delay_seconds and self._current_state == AgentState.LISTENING:
            self._current_state = AgentState.THINKING
            await self.transport.emit_state(AgentState.THINKING)


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
