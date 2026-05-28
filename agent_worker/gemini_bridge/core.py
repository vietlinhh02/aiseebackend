"""LiveKit-free Gemini Live session core and reusable session helpers."""

from __future__ import annotations

import asyncio
import inspect
import logging
from dataclasses import dataclass

from google import genai
from google.genai import errors as genai_errors
from google.genai import types

from .nsfw_filter import NsfwFilter, logger as nsfw_logger
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

=== TOOL CALL RULES ===
- Gọi tool ngay lập tức, không nói gì trước khi gọi tool.
- KHÔNG bao giờ nói "để tôi cập nhật", "tôi sẽ kiểm tra", "let me check" — gọi tool im lặng.
- Camera: chỉ mở khi user nói rõ "mở camera"/"bật camera"/"xem giúp" HOẶC đã đồng ý rõ ràng.
- Nếu mơ hồ ("cái này là gì", "đọc cái này"): hỏi 1 câu "Bạn muốn mở camera để xem không?"
- Tắt camera: gọi deactivate_camera ngay khi user nói tắt/tắt đi/dừng/thôi.
- web_search: chỉ khi cần thông tin mới (tin tức, thời tiết, giá cả, sự kiện). Không cho kiến thức cố định.
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
                activity_handling=types.ActivityHandling.NO_INTERRUPTION,
                automatic_activity_detection=types.AutomaticActivityDetection(
                    disabled=False,
                    silence_duration_ms=200,
                    prefix_padding_ms=100,
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
    _tool_call_in_progress: bool = False

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

    @property
    def tool_call_in_progress(self) -> bool:
        return self._tool_call_in_progress

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
                # Gemini respond audio → clear tool call flag, chuyển sang SPEAKING
                self._tool_call_in_progress = False
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
            self._tool_call_in_progress = True
            await self.state_publisher.publish(AgentState.THINKING)
        responses = await asyncio.gather(
            *(self._function_response_for_call(function_call) for function_call in function_calls)
        )
        if responses:
            await session.send_tool_response(function_responses=responses)
            logger.info("gemini tool_response sent count=%d", len(responses))
            # Giữ THINKING — không publish IDLE ở đây

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
    nsfw_filter: NsfwFilter | None = None
    backend: object | None = None
    user_id: str = ""
    session_id: str = ""

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
            state_publisher.on_audio_input()
            if event.pcm:
                await gemini_session.send_realtime_input(audio=self.settings.audio_blob(event.pcm))
            return
        if isinstance(event, InboundVideoEvent):
            await state_publisher.publish(AgentState.LISTENING)
            if event.jpeg:
                if self.nsfw_filter is not None and not self.nsfw_filter.is_safe(event.jpeg):
                    nsfw_logger.warning("nsfw frame blocked at agent layer")
                    return
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
                        # Chỉ publish IDLE khi KHÔNG có tool call đang chạy
                        if not handler.tool_call_in_progress:
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
        if event.action == "incoming_call":
            await state_publisher.publish(AgentState.THINKING)
            await announce_incoming_call(gemini_session, event.payload)
            return
        if event.action == "incoming_call_cleared":
            return
        if event.action == "interrupt":
            await state_publisher.publish(AgentState.IDLE)
            return
        if event.action == "nsfw_detected":
            await state_publisher.publish(AgentState.THINKING)
            await send_nsfw_warning(gemini_session)
            if self.backend is not None and self.user_id:
                try:
                    await self.backend.record_tracking_event(
                        event_type="camera_blocked_nsfw",
                        category="safety_care",
                        user_id=self.user_id,
                        metadata={
                            "session_id": self.session_id,
                            "source": "client_tflite",
                        },
                        severity="warning",
                        source="agent_worker",
                        source_id=self.session_id,
                    )
                except Exception:
                    logger.warning("failed to record nsfw tracking event", exc_info=True)
            return
        logger.info("ignoring unsupported control action=%s", event.action)


NSFW_WARNING_TEXT = (
    "Camera đã tạm tắt vì phát hiện nội dung không phù hợp. "
    "Hãy thông báo ngắn gọn cho người dùng biết camera đã tạm dừng "
    "và đề nghị hướng camera vào nội dung phù hợp hơn."
)


async def send_wake_greeting(session) -> None:
    await session.send_realtime_input(text=WAKE_GREETING_TEXT)


async def send_nsfw_warning(session) -> None:
    await session.send_realtime_input(text=NSFW_WARNING_TEXT)


async def announce_incoming_call(session, payload: dict) -> None:
    caller = str(payload.get("caller_name") or "").strip() or "ai đó"
    call_type = str(payload.get("call_type") or "voice").strip().lower()
    call_label = "video" if call_type == "video" else "thoại"
    text = (
        f"Có cuộc gọi {call_label} đến từ {caller}. "
        "Hãy thông báo cho người dùng và hỏi xem họ có muốn nghe không. "
        "Nếu người dùng đồng ý nghe, hãy gọi tool answer_incoming_call. "
        "Nếu người dùng từ chối, hãy gọi tool decline_incoming_call. "
        "Không được tự ý bật máy nếu người dùng chưa trả lời."
    )
    await session.send_realtime_input(text=text)


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
