"""Single source of truth for all tool, command, and protocol constants.

Every string that crosses the Python ↔ Flutter ↔ Gemini boundary is defined
here exactly once.  Other modules import from this file instead of hardcoding
string literals.

Wire values MUST NOT be changed without a coordinated migration across all
three layers (Gemini schemas, Python backend, Flutter app).
"""

from __future__ import annotations

import logging
from enum import StrEnum
from typing import Any

logger = logging.getLogger("aisee-agent")

# ─────────────────────────────────────────────────────────────────────
# 1. Gemini tool names — must match schemas.py FunctionDeclarations
# ─────────────────────────────────────────────────────────────────────

class GeminiTool(StrEnum):
    """Tool names exposed to Gemini Live API via FunctionDeclarations."""

    # Communication
    SEND_CHAT_MESSAGE = "send_chat_message"
    READ_UNREAD_MESSAGES = "read_unread_messages"
    MAKE_VOICE_CALL = "make_voice_call"
    MAKE_VIDEO_CALL = "make_video_call"
    CALL_FAMILY_CONTACT = "call_family_contact"
    CALL_SUPPORT_CONTACT = "call_support_contact"
    LIST_FAMILY_CONTACTS = "list_family_contacts"

    # Camera
    ACTIVATE_CAMERA = "activate_camera"
    DEACTIVATE_CAMERA = "deactivate_camera"

    # Health
    SET_MEDICATION_REMINDER = "set_medication_reminder"
    LIST_MEDICATION_REMINDERS = "list_medication_reminders"
    MARK_MEDICATION_TAKEN = "mark_medication_taken"
    MARK_MEDICATION_SKIPPED = "mark_medication_skipped"
    LOG_HEALTH_SYMPTOM = "log_health_symptom"
    LOG_HEALTH_METRIC = "log_health_metric"
    SET_APPOINTMENT_REMINDER = "set_appointment_reminder"

    # Emergency
    EMERGENCY_ALERT = "emergency_alert"
    ESCALATE_TO_FAMILY = "escalate_to_family"
    ESCALATE_TO_VOLUNTEER = "escalate_to_volunteer"

    # Navigation
    NAVIGATE_TO = "navigate_to"

    # Search
    WEB_SEARCH = "web_search"

    # Music
    PLAY_MUSIC = "play_music"
    CONTROL_MUSIC = "control_music"

    # Location
    GET_LOCATION = "get_location"
    GET_CURRENT_LOCATION = "get_current_location"
    UPDATE_LOCATION = "update_location"

    # Call control
    ANSWER_INCOMING_CALL = "answer_incoming_call"
    DECLINE_INCOMING_CALL = "decline_incoming_call"

    # Rating
    RATE_VOLUNTEER = "rate_volunteer"


# ─────────────────────────────────────────────────────────────────────
# 2. Device command names — sent via WS tool_request to Flutter
#    These are the wire values Flutter's DeviceCommandHandler expects.
# ─────────────────────────────────────────────────────────────────────

class DeviceCommand(StrEnum):
    """Command names sent from Python agent to Flutter via WS tool_request."""

    ENABLE_CAMERA = "enable_camera"
    DISABLE_CAMERA = "disable_camera"
    START_CALL = "start_call"
    SCHEDULE_REMINDER = "schedule_reminder"
    GET_LOCATION = "get_location"
    OPEN_MAPS = "open_maps"
    ANSWER_INCOMING_CALL = "answer_incoming_call"
    DECLINE_INCOMING_CALL = "decline_incoming_call"
    PLAY_AUDIO = "play_audio"
    CONTROL_AUDIO = "control_audio"


# Audio control sub-commands (sent inside CONTROL_AUDIO params)
class AudioControlAction(StrEnum):
    """Sub-commands for the control_audio device command."""

    PAUSE = "pause"
    RESUME = "resume"
    PLAY = "play"
    STOP = "stop"
    NEXT = "next"
    PREVIOUS = "previous"
    VOLUME_UP = "volume_up"
    VOLUME_DOWN = "volume_down"


# ─────────────────────────────────────────────────────────────────────
# 3. WS message types — wire protocol between Python ↔ Flutter
# ─────────────────────────────────────────────────────────────────────

class WsInboundType(StrEnum):
    """Message types Flutter sends to Python agent via WS."""

    TEXT = "text"
    CONTROL = "control"
    AUDIO_START = "audio_start"
    AUDIO_STOP = "audio_stop"
    INTERRUPT = "interrupt"
    SESSION_INIT = "session_init"
    MEDIA_MODE = "media_mode"
    TOOL_RESULT = "tool_result"
    NSFW_DETECTED = "nsfw_detected"
    PING = "ping"


class WsOutboundType(StrEnum):
    """Message types Python agent sends to Flutter via WS."""

    READY = "ready"
    AGENT_STATE = "agent_state"
    TOOL_REQUEST = "tool_request"
    DEVICE_COMMAND = "device_command"
    ASSISTANT_AUDIO_INTERRUPTED = "assistant_audio_interrupted"
    PING = "ping"
    PONG = "pong"
    ERROR = "error"


# ─────────────────────────────────────────────────────────────────────
# 4. Control actions — sent inside WS "control" messages
# ─────────────────────────────────────────────────────────────────────

class ControlAction(StrEnum):
    """Control action values inside WS control message payload."""

    WAKE = "wake"
    WAKE_GREETING = "wake_greeting"
    INCOMING_CALL = "incoming_call"
    INCOMING_CALL_CLEARED = "incoming_call_cleared"
    INTERRUPT = "interrupt"
    NSFW_DETECTED = "nsfw_detected"


# ─────────────────────────────────────────────────────────────────────
# 5. Agent states — published to Flutter and LiveKit
# ─────────────────────────────────────────────────────────────────────

class AgentState(StrEnum):
    """Agent lifecycle states visible to Flutter."""

    INITIALIZING = "initializing"
    IDLE = "idle"
    LISTENING = "listening"
    THINKING = "thinking"
    SPEAKING = "speaking"
    DISCONNECTED = "disconnected"
    ERROR = "error"


# ─────────────────────────────────────────────────────────────────────
# 6. Media modes — WS media_mode negotiation
# ─────────────────────────────────────────────────────────────────────

class MediaMode(StrEnum):
    """Media mode values for WS media_mode messages.

    Note: Flutter sends 'audio_pcm'/'camera_jpeg', Python normalizes
    to 'audio'/'camera' internally.
    """

    AUDIO = "audio"
    CAMERA = "camera"


# Flutter-facing wire values (not normalized)
MEDIA_MODE_WIRE_AUDIO = "audio_pcm"
MEDIA_MODE_WIRE_CAMERA = "camera_jpeg"

# Normalization map: Flutter wire value -> internal value
MEDIA_MODE_NORMALIZE: dict[str, str] = {
    "audio_pcm": MediaMode.AUDIO,
    "audio": MediaMode.AUDIO,
    "camera_jpeg": MediaMode.CAMERA,
    "camera": MediaMode.CAMERA,
}


# ─────────────────────────────────────────────────────────────────────
# 7. Tool error codes — structured error responses
# ─────────────────────────────────────────────────────────────────────

class ToolErrorCode(StrEnum):
    """Error codes for structured tool_result error responses."""

    PERMISSION_DENIED = "PERMISSION_DENIED"
    CONTACT_NOT_FOUND = "CONTACT_NOT_FOUND"
    NETWORK_ERROR = "NETWORK_ERROR"
    DEVICE_UNAVAILABLE = "DEVICE_UNAVAILABLE"
    SAFETY_BLOCKED = "SAFETY_BLOCKED"
    TIMEOUT = "TIMEOUT"
    DUPLICATE_REQUEST = "DUPLICATE_REQUEST"
    CANCELLED = "CANCELLED"
    UNKNOWN_TOOL = "UNKNOWN_TOOL"
    UNKNOWN_COMMAND = "UNKNOWN_COMMAND"
    UNKNOWN = "UNKNOWN"
    CAMERA_CONSENT_REQUIRED = "CAMERA_CONSENT_REQUIRED"
    CAMERA_CONSENT_INVALID = "CAMERA_CONSENT_INVALID"
    QUOTA_EXCEEDED = "QUOTA_EXCEEDED"


# ─────────────────────────────────────────────────────────────────────
# 8. Gemini tool -> device command mapping
# ─────────────────────────────────────────────────────────────────────

# Tools that require a device command to be sent to Flutter.
# Value is the DeviceCommand name.
GEMINI_TOOL_TO_DEVICE_COMMAND: dict[str, str] = {
    GeminiTool.ACTIVATE_CAMERA: DeviceCommand.ENABLE_CAMERA,
    GeminiTool.DEACTIVATE_CAMERA: DeviceCommand.DISABLE_CAMERA,
    GeminiTool.NAVIGATE_TO: DeviceCommand.OPEN_MAPS,
    GeminiTool.GET_LOCATION: DeviceCommand.GET_LOCATION,
    GeminiTool.GET_CURRENT_LOCATION: DeviceCommand.GET_LOCATION,
    GeminiTool.ANSWER_INCOMING_CALL: DeviceCommand.ANSWER_INCOMING_CALL,
    GeminiTool.DECLINE_INCOMING_CALL: DeviceCommand.DECLINE_INCOMING_CALL,
}

# Tools that are handled entirely via Go backend HTTP API (no device command).
BACKEND_ONLY_TOOLS: frozenset[str] = frozenset({
    GeminiTool.SEND_CHAT_MESSAGE,
    GeminiTool.READ_UNREAD_MESSAGES,
    GeminiTool.LIST_FAMILY_CONTACTS,
    GeminiTool.LIST_MEDICATION_REMINDERS,
    GeminiTool.MARK_MEDICATION_TAKEN,
    GeminiTool.MARK_MEDICATION_SKIPPED,
    GeminiTool.LOG_HEALTH_SYMPTOM,
    GeminiTool.LOG_HEALTH_METRIC,
    GeminiTool.PLAY_MUSIC,
    GeminiTool.RATE_VOLUNTEER,
})

# Tools with custom backend timeout (seconds). Empty = use default.
BACKEND_TOOL_TIMEOUTS: dict[str, float] = {}

# Device commands that should pause the assistant microphone.
AUDIO_INTERRUPT_COMMANDS: frozenset[str] = frozenset({
    DeviceCommand.PLAY_AUDIO,
    DeviceCommand.START_CALL,
})


# ─────────────────────────────────────────────────────────────────────
# 9. Structured result builders
# ─────────────────────────────────────────────────────────────────────

def tool_success(
    *,
    status: str = "ok",
    message: str = "",
    data: dict[str, Any] | None = None,
) -> dict[str, Any]:
    """Build a structured success result for tool_result."""
    result: dict[str, Any] = {"status": status}
    if message:
        result["message"] = message
    if data:
        result.update(data)
    return result


def tool_error(
    code: str | ToolErrorCode,
    message: str,
    *,
    recoverable: bool = True,
) -> dict[str, Any]:
    """Build a structured error result for tool_result."""
    return {
        "status": "error",
        "error": message,
        "code": str(code),
        "recoverable": recoverable,
    }


def tool_timeout(tool_name: str, timeout_seconds: float) -> dict[str, Any]:
    """Build a structured timeout result for tool_result."""
    return tool_error(
        ToolErrorCode.TIMEOUT,
        f"device command '{tool_name}' timed out after {timeout_seconds:.0f}s",
        recoverable=True,
    )


def tool_unknown(name: str) -> dict[str, Any]:
    """Build a structured unknown tool result."""
    return tool_error(
        ToolErrorCode.UNKNOWN_TOOL,
        f"unknown tool: {name}",
        recoverable=False,
    )


def tool_cancelled(tool_name: str) -> dict[str, Any]:
    """Build a structured cancelled result."""
    return tool_error(
        ToolErrorCode.CANCELLED,
        f"tool '{tool_name}' was cancelled",
        recoverable=True,
    )


# ─────────────────────────────────────────────────────────────────────
# 10. Validation
# ─────────────────────────────────────────────────────────────────────

def validate_tool_consistency() -> list[str]:
    """Check that schemas.py tool names match tool_router.py handlers.

    Returns a list of error descriptions. Empty list = all consistent.
    Should be called once at startup or in tests.
    """
    errors: list[str] = []

    try:
        from schemas import tool_names as schema_tool_names
    except ImportError:
        errors.append("Cannot import schemas.tool_names for validation")
        return errors

    schema_set = set(schema_tool_names())
    gemini_set = {t.value for t in GeminiTool}

    missing_in_enum = schema_set - gemini_set
    if missing_in_enum:
        errors.append(
            f"tools in schemas.py but missing from GeminiTool enum: {missing_in_enum}"
        )
    # Tools in GeminiTool enum but not in schemas.py are OK — they may be
    # internal-only (e.g., play_music, control_music are backend-only).
    missing_in_schema = gemini_set - schema_set
    if missing_in_schema:
        logger.info(
            "GeminiTool enum has tools not exposed to Gemini (schemas.py): %s — this is OK if they have handlers",
            missing_in_schema,
        )

    handled_tools = BACKEND_ONLY_TOOLS | set(GEMINI_TOOL_TO_DEVICE_COMMAND.keys())
    custom_handler_tools = {
        GeminiTool.MAKE_VOICE_CALL,
        GeminiTool.MAKE_VIDEO_CALL,
        GeminiTool.CALL_FAMILY_CONTACT,
        GeminiTool.CALL_SUPPORT_CONTACT,
        GeminiTool.ACTIVATE_CAMERA,
        GeminiTool.DEACTIVATE_CAMERA,
        GeminiTool.SET_MEDICATION_REMINDER,
        GeminiTool.SET_APPOINTMENT_REMINDER,
        GeminiTool.EMERGENCY_ALERT,
        GeminiTool.ESCALATE_TO_FAMILY,
        GeminiTool.ESCALATE_TO_VOLUNTEER,
        GeminiTool.CONTROL_MUSIC,
        GeminiTool.NAVIGATE_TO,
        GeminiTool.WEB_SEARCH,
        GeminiTool.GET_LOCATION,
        GeminiTool.GET_CURRENT_LOCATION,
        GeminiTool.UPDATE_LOCATION,
        GeminiTool.ANSWER_INCOMING_CALL,
        GeminiTool.DECLINE_INCOMING_CALL,
    }
    all_handled = handled_tools | custom_handler_tools
    unhandled = gemini_set - {t.value for t in all_handled}
    if unhandled:
        errors.append(
            f"GeminiTool values with no handler in tool_router: {unhandled}"
        )

    device_set = {d.value for d in DeviceCommand}
    for gemini_name, device_name in GEMINI_TOOL_TO_DEVICE_COMMAND.items():
        if device_name not in device_set:
            errors.append(
                f"GEMINI_TOOL_TO_DEVICE_COMMAND[{gemini_name!r}] = {device_name!r} "
                f"not in DeviceCommand enum"
            )

    return errors
