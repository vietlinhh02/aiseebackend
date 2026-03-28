"""AiSee agent tool definitions — Phase 1 + Phase 2a + Phase 2b + Phase 2c."""

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


# --- Helper for tools that include *_action fields ---


async def _call_and_forward(tool_name: str, params: dict) -> dict:
    """Call backend and forward any *_action fields to app via data channel."""
    result = await _backend.tool_call(tool_name, _user_id, params)
    for key in list(result.keys()):
        if key.endswith("_action") and isinstance(result[key], dict):
            action = result.pop(key)
            try:
                await _bridge.send_action(
                    _local_participant,
                    action["action"],
                    action["params"],
                )
            except (DataChannelTimeoutError, Exception) as exc:
                logger.debug("%s forwarding failed: %s", key, exc)
    return result


# --- Health tools (Phase 2a) ---


@function_tool()
async def set_medication_reminder(
    medicine_name: str,
    dosage: str,
    times: list[str],
    frequency: str = "daily",
) -> dict:
    """Dat nhac nho uong thuoc. times la danh sach gio (VD: ["07:00", "21:00"])."""
    return await _call_and_forward(
        "set_medication_reminder",
        {
            "medicine_name": medicine_name,
            "dosage": dosage,
            "times": times,
            "frequency": frequency,
        },
    )


@function_tool()
async def list_medication_reminders() -> dict:
    """Liet ke thuoc hom nay va trang thai da uong chua."""
    return await _backend.tool_call(
        "list_medication_reminders", _user_id, {},
    )


@function_tool()
async def mark_medication_taken(medicine_name: str) -> dict:
    """Xac nhan da uong thuoc."""
    return await _backend.tool_call(
        "mark_medication_taken",
        _user_id,
        {"medicine_name": medicine_name},
    )


@function_tool()
async def mark_medication_skipped(medicine_name: str) -> dict:
    """Xac nhan bo qua khong uong thuoc hom nay."""
    return await _backend.tool_call(
        "mark_medication_skipped",
        _user_id,
        {"medicine_name": medicine_name},
    )


@function_tool()
async def log_health_symptom(
    symptom: str,
    severity: str = "",
    notes: str = "",
) -> dict:
    """Ghi nhan trieu chung suc khoe (VD: dau dau, chong mat, met moi)."""
    params = {"symptom": symptom}
    if severity:
        params["severity"] = severity
    if notes:
        params["notes"] = notes
    return await _backend.tool_call(
        "log_health_symptom", _user_id, params,
    )


@function_tool()
async def log_health_metric(
    metric_type: str,
    value: str,
    unit: str = "",
) -> dict:
    """Ghi nhan chi so suc khoe (VD: huyet ap 140/90, nhiet do 37.5)."""
    params = {"metric_type": metric_type, "value": value}
    if unit:
        params["unit"] = unit
    return await _backend.tool_call(
        "log_health_metric", _user_id, params,
    )


@function_tool()
async def set_appointment_reminder(
    title: str,
    datetime: str,
    location: str = "",
    notes: str = "",
) -> dict:
    """Dat nhac nho lich hen bac si. datetime dang YYYY-MM-DD HH:MM."""
    return await _call_and_forward(
        "set_appointment_reminder",
        {
            "title": title,
            "datetime": datetime,
            "location": location,
            "notes": notes,
        },
    )


# --- Emergency tools (Phase 2b) ---


@function_tool()
async def emergency_alert() -> dict:
    """SOS khan cap — gui thong bao den tat ca nguoi than kem vi tri GPS."""
    try:
        location = await _bridge.send_action(
            _local_participant, "get_location", {},
        )
        lat = location.get("lat", 0)
        lng = location.get("lng", 0)
    except DataChannelTimeoutError:
        logger.warning("get_location timed out, using unknown location")
        lat, lng = 0, 0
    return await _backend.tool_call_with_timeout(
        "emergency_alert",
        _user_id,
        {"lat": lat, "lng": lng},
        timeout=20.0,
    )


@function_tool()
async def escalate_to_family(
    contact_name: str = "",
    skip: list[str] | None = None,
) -> dict:
    """Goi nguoi than ho tro khi AI khong giup duoc. Neu khong ai tra loi, goi lai voi skip de thu nguoi khac."""
    params: dict = {}
    if contact_name:
        params["contact_name"] = contact_name
    if skip:
        params["skip"] = skip
    return await _backend.tool_call_with_timeout(
        "escalate_to_family",
        _user_id,
        params,
        timeout=40.0,
    )


# --- Music tools (Phase 2c) ---


@function_tool()
async def play_music(query: str) -> dict:
    """Tim va phat nhac theo yeu cau (VD: 'nhac Trinh Cong Son', 'bai hat Diem Xua')."""
    return await _call_and_forward(
        "play_music",
        {"query": query},
    )


@function_tool()
async def control_music(action: str) -> dict:
    """Dieu khien nhac dang phat: pause, resume, stop."""
    try:
        result = await _bridge.send_action(
            _local_participant,
            "control_audio",
            {"command": action},
        )
        return {"status": result.get("status", "ok")}
    except DataChannelTimeoutError:
        return {
            "status": "error",
            "error": "App did not respond.",
        }


ALL_TOOLS = [
    # Phase 1 — Communication + Camera
    make_voice_call,
    make_video_call,
    send_chat_message,
    read_unread_messages,
    activate_camera,
    deactivate_camera,
    # Phase 2a — Health
    set_medication_reminder,
    list_medication_reminders,
    mark_medication_taken,
    mark_medication_skipped,
    log_health_symptom,
    log_health_metric,
    set_appointment_reminder,
    # Phase 2b — Emergency
    emergency_alert,
    escalate_to_family,
    # Phase 2c — Music
    play_music,
    control_music,
]
