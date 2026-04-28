"""LiveKit-free Gemini Live session core and reusable session helpers."""

from __future__ import annotations

import asyncio
import inspect
import logging
from dataclasses import dataclass

from google import genai
from google.genai import errors as genai_errors
from google.genai import types

from .schemas import build_gemini_tools
from .state import AgentState
from .tool_router import ToolRouter
from .transport import (
    GeminiTransport,
    InboundAudioEvent,
    InboundControlEvent,
    InboundTextEvent,
    InboundVideoEvent,
    TransportAudioSink,
    TransportStatePublisher,
)

WAKE_GREETING_TEXT = (
    "Người dùng vừa đánh thức bạn. Hãy chào ngắn gọn bằng tiếng Việt và hỏi họ cần hỗ trợ gì."
)
PENDING_TASK_DRAIN_SECONDS = 0.25

TOOL_ROUTING_INSTRUCTIONS = """

=== TOOL ROUTING RULES ===
- Khi người dùng yêu cầu bật camera, mở camera, nhìn xung quanh, xem giúp,
  hoặc mô tả môi trường: gọi tool activate_camera trước khi trả lời.
- Khi người dùng yêu cầu tắt camera, dừng camera, thôi nhìn, dừng quay,
  hoặc nói "tắt đi" ngay sau khi camera đang bật: gọi tool deactivate_camera
  trước khi trả lời.
- Không chỉ nói rằng bạn sẽ bật hoặc tắt camera. Phải gọi đúng tool trước,
  rồi báo kết quả ngắn gọn.
- For English camera commands, call activate_camera for turn on/open/see/look,
  and deactivate_camera for turn off/stop/close camera.
- Khi người dùng hỏi tin tức, thông tin hiện tại, thời tiết, giá cả, sự kiện,
  hoặc yêu cầu tìm trên mạng: gọi tool web_search trước khi trả lời.
- Không gọi web_search cho kiến thức ổn định, trò chuyện thường, điều khiển
  thiết bị, camera, nhạc, tin nhắn, lịch, thuốc, hoặc yêu cầu đã đủ dữ liệu.
- Khi gọi web_search, dùng truy vấn ngắn gọn và chỉ yêu cầu nhiều hơn 3 kết quả
  nếu người dùng thật sự cần so sánh nhiều nguồn.
"""

logger = logging.getLogger("aisee-agent")


@dataclass(frozen=True, slots=True)
class GeminiLiveSettings:
    api_key: str
    model: str
    voice: str
    thinking_level: str
    input_sample_rate: int
    google_search_enabled: bool = False

    @classmethod
    def from_env(cls) -> "GeminiLiveSettings":
        from config import (
            GEMINI_API_KEY,
            GEMINI_AUDIO_SAMPLE_RATE,
            GEMINI_GOOGLE_SEARCH_ENABLED,
            GEMINI_MODEL,
            GEMINI_THINKING_LEVEL,
            GEMINI_VOICE,
        )

        return cls(
            api_key=GEMINI_API_KEY,
            model=GEMINI_MODEL,
            voice=GEMINI_VOICE,
            thinking_level=GEMINI_THINKING_LEVEL,
            input_sample_rate=GEMINI_AUDIO_SAMPLE_RATE,
            google_search_enabled=GEMINI_GOOGLE_SEARCH_ENABLED,
        )

    def client(self) -> genai.Client:
        return genai.Client(api_key=self.api_key)

    def build_live_config(
        self,
        *,
        enable_google_search: bool | None = None,
    ) -> types.LiveConnectConfig:
        if enable_google_search is None:
            enable_google_search = self.google_search_enabled
        return types.LiveConnectConfig(
            response_modalities=[types.Modality.AUDIO],
            speech_config=types.SpeechConfig(
                voice_config=types.VoiceConfig(
                    prebuilt_voice_config=types.PrebuiltVoiceConfig(
                        voice_name=self.voice,
                    ),
                ),
            ),
            thinking_config=types.ThinkingConfig(
                thinking_level=self._thinking_level(),
            ),
            realtime_input_config=types.RealtimeInputConfig(
                activity_handling=types.ActivityHandling.START_OF_ACTIVITY_INTERRUPTS,
                automatic_activity_detection=types.AutomaticActivityDetection(
                    disabled=False,
                    silence_duration_ms=400,
                ),
            ),
            tools=build_gemini_tools(enable_google_search=enable_google_search),
        )

    def build_live_config_with_prompt(
        self,
        instructions: str,
        *,
        enable_google_search: bool | None = None,
    ) -> types.LiveConnectConfig:
        config = self.build_live_config(enable_google_search=enable_google_search)
        config.system_instruction = instructions + TOOL_ROUTING_INSTRUCTIONS
        return config

    def audio_blob(self, pcm: bytes) -> types.Blob:
        return types.Blob(
            data=pcm,
            mime_type=f"audio/pcm;rate={self.input_sample_rate}",
        )

    def video_blob(self, jpeg: bytes) -> types.Blob:
        return types.Blob(data=jpeg, mime_type="image/jpeg")

    def _thinking_level(self) -> types.ThinkingLevel:
        normalized = self.thinking_level.strip().upper()
        try:
            return types.ThinkingLevel[normalized]
        except KeyError:
            allowed = ", ".join(
                level.name.lower()
                for level in types.ThinkingLevel
                if level is not types.ThinkingLevel.THINKING_LEVEL_UNSPECIFIED
            )
            raise ValueError(
                f"Unsupported GEMINI_THINKING_LEVEL={self.thinking_level!r}; "
                f"expected one of: {allowed}"
            ) from None


@dataclass(slots=True)
class GeminiEventHandler:
    tool_router: ToolRouter
    audio_publisher: object
    state_publisher: object | None = None

    async def handle(self, session, message) -> None:
        server_content = getattr(message, "server_content", None)
        if server_content is not None:
            if getattr(server_content, "interrupted", False):
                clear_result = self.audio_publisher.clear()
                if inspect.isawaitable(clear_result):
                    await clear_result
            await self._handle_server_content(server_content)

        tool_call = getattr(message, "tool_call", None)
        if tool_call is not None:
            await self._handle_tool_call(session, tool_call)

    async def _handle_server_content(self, server_content) -> None:
        model_turn = getattr(server_content, "model_turn", None)
        parts = getattr(model_turn, "parts", None) or []
        for part in parts:
            inline_data = getattr(part, "inline_data", None)
            if inline_data is None:
                continue
            data = getattr(inline_data, "data", None)
            mime_type = getattr(inline_data, "mime_type", "") or ""
            if data and mime_type.startswith("audio/"):
                if self.state_publisher is not None:
                    await self.state_publisher.publish(AgentState.SPEAKING)
                await self.audio_publisher.publish_pcm(data)

    async def _handle_tool_call(self, session, tool_call) -> None:
        function_calls = list(getattr(tool_call, "function_calls", []) or [])
        names = ",".join(getattr(call, "name", "") for call in function_calls)
        logger.info(
            "gemini tool_call received count=%d names=%s",
            len(function_calls),
            names,
        )
        if self.state_publisher is not None and function_calls:
            await self.state_publisher.publish(AgentState.THINKING)
        responses = await asyncio.gather(
            *(self._function_response_for_call(function_call) for function_call in function_calls)
        )
        if responses:
            await session.send_tool_response(function_responses=responses)
            logger.info("gemini tool_response sent count=%d", len(responses))

    async def _function_response_for_call(self, function_call) -> types.FunctionResponse:
        call_id = getattr(function_call, "id", None)
        name = str(getattr(function_call, "name", "") or "")
        args = _function_call_args(function_call)
        logger.info(
            "gemini function_call start id=%s name=%s arg_keys=%s",
            call_id,
            name,
            sorted(args.keys()),
        )
        try:
            if not name:
                result = {"status": "error", "error": "missing function call name"}
            else:
                result = await self.tool_router.call(name, args)
            result = _function_response_payload(result)
        except Exception as exc:
            logger.warning(
                "gemini function_call failed id=%s name=%s error=%s",
                call_id,
                name,
                exc,
            )
            result = {"status": "error", "error": str(exc)}
        logger.info(
            "gemini function_call result id=%s name=%s status=%s",
            call_id,
            name,
            result.get("status"),
        )
        return types.FunctionResponse(
            id=call_id,
            name=name,
            response=result,
        )


@dataclass(slots=True)
class GeminiTransportSession:
    settings: GeminiLiveSettings
    transport: GeminiTransport
    tool_router: ToolRouter

    async def run(self, gemini_session) -> None:
        state_publisher = TransportStatePublisher(transport=self.transport)
        handler = GeminiEventHandler(
            tool_router=self.tool_router,
            audio_publisher=TransportAudioSink(transport=self.transport),
            state_publisher=state_publisher,
        )
        await self.transport.emit_ready()
        await state_publisher.publish(AgentState.IDLE)
        inbound_task = asyncio.create_task(
            self._forward_inbound_loop(gemini_session, state_publisher)
        )
        receive_task = asyncio.create_task(
            self._receive_loop(gemini_session, handler, state_publisher)
        )
        done, pending = await asyncio.wait(
            {inbound_task, receive_task},
            return_when=asyncio.FIRST_COMPLETED,
        )
        for task in done:
            task.result()
        if pending:
            drained, pending = await asyncio.wait(
                pending,
                timeout=PENDING_TASK_DRAIN_SECONDS,
            )
            for task in drained:
                task.result()
        for task in pending:
            task.cancel()
        for task in pending:
            try:
                await task
            except asyncio.CancelledError:
                pass

    async def _forward_inbound_loop(self, gemini_session, state_publisher) -> None:
        async for event in self.transport.receive():
            await self._handle_inbound_event(gemini_session, event, state_publisher)

    async def _handle_inbound_event(self, gemini_session, event, state_publisher) -> None:
        if isinstance(event, InboundAudioEvent):
            await state_publisher.publish(AgentState.LISTENING)
            if event.pcm:
                await gemini_session.send_realtime_input(audio=self.settings.audio_blob(event.pcm))
            return
        if isinstance(event, InboundVideoEvent):
            await state_publisher.publish(AgentState.LISTENING)
            if event.jpeg:
                await gemini_session.send_realtime_input(video=self.settings.video_blob(event.jpeg))
            return
        if isinstance(event, InboundTextEvent):
            await state_publisher.publish(AgentState.LISTENING)
            if event.text:
                await gemini_session.send_realtime_input(text=event.text)
            return
        if isinstance(event, InboundControlEvent):
            await self._handle_control_event(gemini_session, event, state_publisher)
            return
        raise TypeError(f"unsupported inbound transport event: {type(event)!r}")

    async def _receive_loop(
        self, gemini_session, handler: GeminiEventHandler, state_publisher
    ) -> None:
        while True:
            saw_message = False
            try:
                async for message in gemini_session.receive():
                    saw_message = True
                    await handler.handle(gemini_session, message)
                    server_content = getattr(message, "server_content", None)
                    if server_content and getattr(server_content, "turn_complete", False):
                        await state_publisher.publish(AgentState.IDLE)
            except genai_errors.APIError as exc:
                if _is_normal_live_close(exc):
                    return
                raise
            if not saw_message:
                return

    async def _handle_control_event(self, gemini_session, event, state_publisher) -> None:
        if event.action in {"wake", "wake_greeting"}:
            await state_publisher.publish(AgentState.THINKING)
            await send_wake_greeting(gemini_session)
            return
        if event.action == "interrupt":
            await state_publisher.publish(AgentState.IDLE)
            return
        logger.info("ignoring unsupported control action=%s", event.action)


async def send_wake_greeting(session) -> None:
    await session.send_realtime_input(text=WAKE_GREETING_TEXT)


def _function_call_args(function_call) -> dict:
    raw_args = getattr(function_call, "args", {}) or {}
    if isinstance(raw_args, dict):
        return dict(raw_args)
    try:
        return dict(raw_args)
    except (TypeError, ValueError):
        return {}


def _function_response_payload(result) -> dict:
    if isinstance(result, dict):
        return result
    return {"status": "ok", "result": result}


def _is_normal_live_close(exc: genai_errors.APIError) -> bool:
    status_code = getattr(exc, "status_code", None)
    if status_code == 1000:
        return True
    return str(exc).strip().startswith("1000")
