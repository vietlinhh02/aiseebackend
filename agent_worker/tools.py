"""AiSee agent tool definitions — Phase 1-3."""

import json
import logging

from livekit import rtc
from livekit.agents import function_tool

from backend import BackendClient

logger = logging.getLogger("aisee-agent")

_backend: BackendClient | None = None
_local_participant: rtc.LocalParticipant | None = None
_user_context: dict | None = None


def init_tools(
    backend: BackendClient,
    local_participant: rtc.LocalParticipant,
    user_context: dict,
):
    """Initialize tool dependencies. Called from main.py."""
    global _backend, _local_participant, _user_context
    _backend = backend
    _local_participant = local_participant
    _user_context = user_context


def _user_identity() -> str:
    """LiveKit participant identity for RPC targeting."""
    return _user_context.get(
        'user_identity', _user_context.get('user_id', ''),
    )


def _uid() -> str:
    """User ID for backend API calls."""
    return _user_context.get('user_id', '')


# --- Backend tools (HTTP -> Go backend) ---


@function_tool()
async def make_voice_call(contact_name: str) -> dict:
    """Call a family member by name or relationship (voice only)."""
    return await _backend.tool_call(
        "make_voice_call", _uid(), {"contact_name": contact_name},
    )


@function_tool()
async def make_video_call(contact_name: str) -> dict:
    """Video call a family member by name or relationship."""
    return await _backend.tool_call(
        "make_video_call", _uid(), {"contact_name": contact_name},
    )


@function_tool()
async def send_chat_message(contact_name: str, message: str) -> dict:
    """Send a text message to a family member."""
    return await _backend.tool_call(
        "send_chat_message",
        _uid(),
        {"contact_name": contact_name, "message": message},
    )


@function_tool()
async def read_unread_messages(contact_name: str = "") -> dict:
    """Read unread messages. Optionally filter by contact name."""
    params = {}
    if contact_name:
        params["contact_name"] = contact_name
    return await _backend.tool_call(
        "read_unread_messages", _uid(), params,
    )


# --- RPC tools (LiveKit -> App) ---


@function_tool()
async def activate_camera(reason: str = "User requested") -> dict:
    """Enable camera to see through user's phone."""
    try:
        response = await _local_participant.perform_rpc(
            destination_identity=_user_identity(),
            method='enable_camera',
            payload='{}',
        )
        return json.loads(response)
    except Exception as exc:
        return {"status": "error", "error": str(exc)}


@function_tool()
async def deactivate_camera() -> dict:
    """Disable camera, return to audio-only."""
    try:
        response = await _local_participant.perform_rpc(
            destination_identity=_user_identity(),
            method='disable_camera',
            payload='{}',
        )
        return json.loads(response)
    except Exception as exc:
        return {"status": "error", "error": str(exc)}


# --- Health tools (Phase 2a) ---


@function_tool()
async def set_medication_reminder(
    medicine_name: str,
    dosage: str,
    times: list[str],
    frequency: str = "daily",
) -> dict:
    """Dat nhac nho uong thuoc. times la danh sach gio (VD: ["07:00", "21:00"])."""
    result = await _backend.tool_call(
        "set_medication_reminder", _uid(),
        {"medicine_name": medicine_name, "dosage": dosage,
         "times": times, "frequency": frequency},
    )
    reminder_action = result.pop("schedule_reminder_action", None)
    if reminder_action and isinstance(reminder_action, dict):
        try:
            await _local_participant.perform_rpc(
                destination_identity=_user_identity(),
                method='schedule_reminder',
                payload=json.dumps(
                    reminder_action.get("params", {}),
                ),
            )
        except Exception as exc:
            logger.warning("schedule_reminder RPC failed: %s", exc)
    return result


@function_tool()
async def list_medication_reminders() -> dict:
    """Liet ke thuoc hom nay va trang thai da uong chua."""
    return await _backend.tool_call(
        "list_medication_reminders", _uid(), {},
    )


@function_tool()
async def mark_medication_taken(medicine_name: str) -> dict:
    """Xac nhan da uong thuoc."""
    return await _backend.tool_call(
        "mark_medication_taken",
        _uid(),
        {"medicine_name": medicine_name},
    )


@function_tool()
async def mark_medication_skipped(medicine_name: str) -> dict:
    """Xac nhan bo qua khong uong thuoc hom nay."""
    return await _backend.tool_call(
        "mark_medication_skipped",
        _uid(),
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
        "log_health_symptom", _uid(), params,
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
        "log_health_metric", _uid(), params,
    )


@function_tool()
async def set_appointment_reminder(
    title: str,
    datetime: str,
    location: str = "",
    notes: str = "",
) -> dict:
    """Dat nhac nho lich hen bac si. datetime dang YYYY-MM-DD HH:MM."""
    result = await _backend.tool_call(
        "set_appointment_reminder", _uid(),
        {"title": title, "datetime": datetime,
         "location": location, "notes": notes},
    )
    reminder_action = result.pop("schedule_reminder_action", None)
    if reminder_action and isinstance(reminder_action, dict):
        try:
            await _local_participant.perform_rpc(
                destination_identity=_user_identity(),
                method='schedule_reminder',
                payload=json.dumps(
                    reminder_action.get("params", {}),
                ),
            )
        except Exception as exc:
            logger.warning("schedule_reminder RPC failed: %s", exc)
    return result


# --- Emergency tools (Phase 2b) ---


@function_tool()
async def emergency_alert() -> dict:
    """SOS khan cap — gui thong bao den tat ca nguoi than kem vi tri GPS."""
    try:
        response = await _local_participant.perform_rpc(
            destination_identity=_user_identity(),
            method='get_location',
            payload='{}',
            response_timeout=15.0,
        )
        location = json.loads(response)
        lat = location.get("lat", 0)
        lng = location.get("lng", 0)
    except Exception:
        logger.warning("get_location RPC failed, using unknown location")
        lat, lng = 0, 0
    return await _backend.tool_call_with_timeout(
        "emergency_alert", _uid(), {"lat": lat, "lng": lng},
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
        _uid(),
        params,
        timeout=40.0,
    )


# --- Music tools (Phase 2c) ---


@function_tool()
async def play_music(query: str) -> dict:
    """Tim va phat nhac theo yeu cau."""
    result = await _backend.tool_call(
        "play_music", _uid(), {"query": query},
    )
    play_action = result.pop("play_audio_action", None)
    if play_action and isinstance(play_action, dict):
        try:
            await _local_participant.perform_rpc(
                destination_identity=_user_identity(),
                method='play_audio',
                payload=json.dumps(
                    play_action.get("params", {}),
                ),
            )
        except Exception as exc:
            logger.warning("play_audio RPC failed: %s", exc)
    return result


@function_tool()
async def control_music(action: str) -> dict:
    """Dieu khien nhac dang phat: pause, resume, stop."""
    try:
        response = await _local_participant.perform_rpc(
            destination_identity=_user_identity(),
            method='control_audio',
            payload=json.dumps({"command": action}),
        )
        return json.loads(response)
    except Exception as exc:
        return {"status": "error", "error": str(exc)}


# --- Navigation tools (Phase 2d) ---


@function_tool()
async def navigate_to(destination: str) -> dict:
    """Mo ban do va chi duong den dia diem."""
    try:
        response = await _local_participant.perform_rpc(
            destination_identity=_user_identity(),
            method='open_maps',
            payload=json.dumps({"destination": destination}),
        )
        return json.loads(response)
    except Exception as exc:
        return {"status": "error", "error": str(exc)}


# --- Volunteer & Location tools (Phase 3) ---


@function_tool()
async def escalate_to_volunteer() -> dict:
    """Ket noi voi tinh nguyen vien khi gia dinh khong tra loi. Tra ve match_id de theo doi."""
    return await _backend.tool_call(
        "escalate_to_volunteer", _uid(), {},
    )


@function_tool()
async def rate_volunteer(
    volunteer_id: str,
    match_id: str,
    score: int,
) -> dict:
    """Danh gia tinh nguyen vien sau cuoc goi (1-5 sao)."""
    return await _backend.tool_call(
        "rate_volunteer",
        _uid(),
        {
            "volunteer_id": volunteer_id,
            "match_id": match_id,
            "score": score,
        },
    )


@function_tool()
async def get_current_location() -> dict:
    """Lay vi tri hien tai — tra ve dia chi doc duoc va toa do GPS."""
    try:
        response = await _local_participant.perform_rpc(
            destination_identity=_user_identity(),
            method='get_location',
            payload='{}',
            response_timeout=15.0,
        )
        return json.loads(response)
    except Exception as exc:
        return {"status": "error", "error": str(exc)}


@function_tool()
async def update_location() -> dict:
    """Cap nhat vi tri hien tai cua nguoi dung len he thong."""
    try:
        response = await _local_participant.perform_rpc(
            destination_identity=_user_identity(),
            method='get_location',
            payload='{}',
            response_timeout=15.0,
        )
        location = json.loads(response)
        lat = location.get("lat", 0)
        lng = location.get("lng", 0)
    except Exception as exc:
        return {"status": "error", "error": str(exc)}
    return await _backend.tool_call(
        "update_location", _uid(), {"lat": lat, "lng": lng},
    )


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
    # Phase 2d — Navigation
    navigate_to,
    # Phase 3 — Volunteer + Location
    escalate_to_volunteer,
    rate_volunteer,
    get_current_location,
    update_location,
]
