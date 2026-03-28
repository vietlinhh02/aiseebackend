"""AiSee agent tool definitions — Phase 1."""

import logging

from livekit.agents import function_tool

from backend import BackendClient
from data_channel import DataChannelBridge, DataChannelTimeoutError

logger = logging.getLogger("aisee-agent")

# Global instances injected by main.py
_backend: BackendClient | None = None
_bridge: DataChannelBridge | None = None
_local_participant = None
_user_id: str = ""


def init_tools(
    backend: BackendClient,
    bridge: DataChannelBridge,
    local_participant,
    user_id: str,
):
    """Initialize tool dependencies. Called from main.py."""
    global _backend, _bridge, _local_participant, _user_id
    _backend = backend
    _bridge = bridge
    _local_participant = local_participant
    _user_id = user_id


# --- Backend tools (HTTP -> Go backend) ---


@function_tool()
async def make_voice_call(contact_name: str) -> dict:
    """Call a family member by name or relationship (voice only)."""
    return await _backend.tool_call(
        "make_voice_call", _user_id, {"contact_name": contact_name},
    )


@function_tool()
async def make_video_call(contact_name: str) -> dict:
    """Video call a family member by name or relationship."""
    return await _backend.tool_call(
        "make_video_call", _user_id, {"contact_name": contact_name},
    )


@function_tool()
async def send_chat_message(contact_name: str, message: str) -> dict:
    """Send a text message to a family member."""
    return await _backend.tool_call(
        "send_chat_message",
        _user_id,
        {"contact_name": contact_name, "message": message},
    )


@function_tool()
async def read_unread_messages(contact_name: str = "") -> dict:
    """Read unread messages. Optionally filter by contact name."""
    params = {}
    if contact_name:
        params["contact_name"] = contact_name
    return await _backend.tool_call(
        "read_unread_messages", _user_id, params,
    )


# --- Data channel tools (LiveKit -> App) ---


@function_tool()
async def activate_camera(reason: str = "User requested") -> dict:
    """Enable camera to see through user's phone."""
    try:
        result = await _bridge.send_action(
            _local_participant, "enable_camera", {},
        )
        if result["status"] == "ok":
            return {"status": "camera_enabled"}
        return result
    except DataChannelTimeoutError:
        return {
            "status": "error",
            "error": "App did not respond. Camera may not be available.",
        }


@function_tool()
async def deactivate_camera() -> dict:
    """Disable camera, return to audio-only."""
    try:
        result = await _bridge.send_action(
            _local_participant, "disable_camera", {},
        )
        if result["status"] == "ok":
            return {"status": "camera_disabled"}
        return result
    except DataChannelTimeoutError:
        return {
            "status": "error",
            "error": "App did not respond.",
        }


ALL_TOOLS = [
    make_voice_call,
    make_video_call,
    send_chat_message,
    read_unread_messages,
    activate_camera,
    deactivate_camera,
]
