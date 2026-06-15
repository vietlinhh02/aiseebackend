"""AiSee agent with tool definitions — Phase 1-3."""

import json
import logging
import uuid

from livekit import rtc
from livekit.agents import Agent, function_tool

from backend import BackendClient
from camera_activation_policy import validate_camera_activation_consent
from prompts import select_prompt

logger = logging.getLogger("aisee-agent")


class AiSeeAgent(Agent):
    def __init__(
        self,
        backend: BackendClient,
        local_participant: rtc.LocalParticipant,
        user_context: dict,
        locale: str = 'vi',
        wake_up: bool = False,
    ):
        self._backend = backend
        self._lp = local_participant
        self._uctx = user_context
        super().__init__(
            instructions=select_prompt(locale, wake_up, user_context),
        )

    def _uid(self) -> str:
        return self._uctx.get('user_id', '')

    def _user_identity(self) -> str:
        return self._uctx.get(
            'user_identity', self._uid(),
        )

    # --- Phase 1: Communication + Camera ---

    @function_tool()
    async def send_chat_message(
        self, contact_name: str, message: str,
    ) -> dict:
        """Send a text message to a family member."""
        return await self._backend.tool_call(
            "send_chat_message", self._uid(),
            {"contact_name": contact_name, "message": message},
        )

    @function_tool()
    async def read_unread_messages(
        self, contact_name: str = "",
    ) -> dict:
        """Read unread messages. Optionally filter by contact name."""
        params = {}
        if contact_name:
            params["contact_name"] = contact_name
        return await self._backend.tool_call(
            "read_unread_messages", self._uid(), params,
        )

    @function_tool()
    async def list_family_contacts(self) -> dict:
        """Check realtime family connections and list linked family contacts."""
        return await self._backend.tool_call(
            "list_family_contacts", self._uid(), {},
        )

    @function_tool()
    async def call_family_contact(
        self, contact_name: str, call_type: str = "video",
    ) -> dict:
        """Call a family contact by name or relationship. Defaults to a
        video call so family members can see the user (mirrors the
        support-contact behaviour). Pass ``call_type="voice"`` explicitly to
        request an audio-only call."""
        return await self._backend.tool_call(
            "call_family_contact", self._uid(),
            {"contact_name": contact_name, "call_type": call_type},
        )

    @function_tool()
    async def call_support_contact(
        self, contact_name: str = "", call_type: str = "video",
    ) -> dict:
        """Call a support contact: named family first, otherwise volunteer support."""
        return await self._backend.tool_call_with_timeout(
            "call_support_contact", self._uid(),
            {"contact_name": contact_name, "call_type": call_type},
            timeout=45.0,
        )

    @function_tool()
    async def activate_camera(
        self,
        reason: str,
        consent_phrase: str,
        consent_source: str,
    ) -> dict:
        """Enable camera to see through user's phone."""
        allowed, detail = validate_camera_activation_consent(
            consent_source=consent_source,
            consent_phrase=consent_phrase,
        )
        if not allowed:
            return {
                "status": "blocked",
                "error": detail,
                "message": (
                    "Camera only opens after the user explicitly asks to see "
                    "through camera or clearly agrees."
                ),
            }
        source_id = f"camera-{uuid.uuid4()}"
        try:
            try:
                await self._backend.check_quota(
                    self._uid(), "camera_activations", 1,
                )
            except Exception as exc:
                logger.warning("camera quota check failed (proceeding anyway): %s", exc)
            response = await self._lp.perform_rpc(
                destination_identity=self._user_identity(),
                method='enable_camera',
                payload='{}',
            )
            result = json.loads(response)
            try:
                await self._backend.record_usage(
                    self._uid(),
                    "camera_activations",
                    1,
                    "activations",
                    "agent",
                    source_id,
                    {
                        "reason": reason,
                        "consent_source": consent_source,
                        "consent_phrase": consent_phrase,
                    },
                )
            except Exception as exc:
                logger.warning("camera record_usage failed (camera still active): %s", exc)
            return result
        except Exception as exc:
            return {"status": "error", "error": str(exc)}

    @function_tool()
    async def deactivate_camera(self) -> dict:
        """Disable camera, return to audio-only."""
        try:
            response = await self._lp.perform_rpc(
                destination_identity=self._user_identity(),
                method='disable_camera',
                payload='{}',
            )
            return json.loads(response)
        except Exception as exc:
            return {"status": "error", "error": str(exc)}

    # --- Phase 2a: Health ---

    @function_tool()
    async def set_medication_reminder(
        self,
        medicine_name: str,
        dosage: str,
        times: list[str],
        frequency: str = "daily",
    ) -> dict:
        """Đặt nhắc nhở uống thuốc. times là danh sách giờ (VD: ["07:00", "21:00"])."""
        result = await self._backend.tool_call(
            "set_medication_reminder", self._uid(),
            {"medicine_name": medicine_name, "dosage": dosage,
             "times": times, "frequency": frequency},
        )
        reminder_action = result.pop("schedule_action", None)
        if reminder_action and isinstance(reminder_action, dict):
            try:
                await self._lp.perform_rpc(
                    destination_identity=self._user_identity(),
                    method='schedule_reminder',
                    payload=json.dumps(
                        reminder_action.get("params", {}),
                    ),
                )
            except Exception as exc:
                logger.warning(
                    "schedule_reminder RPC failed: %s", exc,
                )
        return result

    @function_tool()
    async def list_medication_reminders(self) -> dict:
        """Liệt kê thuốc hôm nay và trạng thái đã uống chưa."""
        return await self._backend.tool_call(
            "list_medication_reminders", self._uid(), {},
        )

    @function_tool()
    async def mark_medication_taken(
        self, medicine_name: str,
    ) -> dict:
        """Xác nhận đã uống thuốc."""
        return await self._backend.tool_call(
            "mark_medication_taken", self._uid(),
            {"medicine_name": medicine_name},
        )

    @function_tool()
    async def mark_medication_skipped(
        self, medicine_name: str,
    ) -> dict:
        """Xác nhận bỏ qua không uống thuốc hôm nay."""
        return await self._backend.tool_call(
            "mark_medication_skipped", self._uid(),
            {"medicine_name": medicine_name},
        )

    @function_tool()
    async def log_health_symptom(
        self,
        symptom: str,
        severity: str = "",
        notes: str = "",
    ) -> dict:
        """Ghi nhận triệu chứng sức khỏe (VD: đau đầu, chóng mặt, mệt mỏi)."""
        params = {"symptom": symptom}
        if severity:
            params["severity"] = severity
        if notes:
            params["notes"] = notes
        return await self._backend.tool_call(
            "log_health_symptom", self._uid(), params,
        )

    @function_tool()
    async def log_health_metric(
        self,
        metric_type: str,
        value: str,
        unit: str = "",
    ) -> dict:
        """Ghi nhận chỉ số sức khỏe (VD: huyết áp 140/90, nhiệt độ 37.5)."""
        params = {"metric_type": metric_type, "value": value}
        if unit:
            params["unit"] = unit
        return await self._backend.tool_call(
            "log_health_metric", self._uid(), params,
        )

    @function_tool()
    async def set_appointment_reminder(
        self,
        title: str,
        datetime: str,
        location: str = "",
        notes: str = "",
    ) -> dict:
        """Đặt nhắc nhở lịch hẹn bác sĩ. datetime dạng YYYY-MM-DD HH:MM."""
        result = await self._backend.tool_call(
            "set_appointment_reminder", self._uid(),
            {"title": title, "datetime": datetime,
             "location": location, "notes": notes},
        )
        reminder_action = result.pop("schedule_action", None)
        if reminder_action and isinstance(reminder_action, dict):
            try:
                await self._lp.perform_rpc(
                    destination_identity=self._user_identity(),
                    method='schedule_reminder',
                    payload=json.dumps(
                        reminder_action.get("params", {}),
                    ),
                )
            except Exception as exc:
                logger.warning(
                    "schedule_reminder RPC failed: %s", exc,
                )
        return result

    # --- Phase 2b: Emergency ---

    @function_tool()
    async def emergency_alert(self) -> dict:
        """SOS khẩn cấp — gửi thông báo đến tất cả người thân kèm vị trí GPS."""
        try:
            response = await self._lp.perform_rpc(
                destination_identity=self._user_identity(),
                method='get_location',
                payload='{}',
                response_timeout=15.0,
            )
            location = json.loads(response)
            lat = location.get("lat", 0)
            lng = location.get("lng", 0)
        except Exception:
            logger.warning(
                "get_location RPC failed, using unknown location",
            )
            lat, lng = 0, 0
        return await self._backend.tool_call_with_timeout(
            "emergency_alert", self._uid(),
            {"lat": lat, "lng": lng},
            timeout=20.0,
        )

    @function_tool()
    async def escalate_to_family(
        self,
        contact_name: str = "",
        skip: list[str] | None = None,
    ) -> dict:
        """Gọi người thân hỗ trợ khi AI không giúp được. Nếu không ai trả lời, gọi lại với skip để thử người khác."""
        params: dict = {}
        if contact_name:
            params["contact_name"] = contact_name
        if skip:
            params["skip"] = skip
        return await self._backend.tool_call_with_timeout(
            "escalate_to_family", self._uid(), params,
            timeout=40.0,
        )

    # --- Phase 2c: Music ---

    @function_tool()
    async def play_music(self, query: str) -> dict:
        """Tìm và phát nhạc theo yêu cầu."""
        return await self._backend.tool_call(
            "play_music", self._uid(),
            {"query": query},
        )

    @function_tool()
    async def control_music(self, action: str) -> dict:
        """Điều khiển nhạc đang phát: pause, resume, stop, next, previous, volume_up, volume_down."""
        return {
            "status": "disabled",
            "message": "music playback control is temporarily disabled",
        }

    # --- Phase 2d: Navigation ---

    @function_tool()
    async def navigate_to(self, destination: str) -> dict:
        """Mở bản đồ và chỉ đường đến địa điểm."""
        source_id = f"navigation-{uuid.uuid4()}"
        try:
            await self._backend.check_quota(
                self._uid(), "navigation_sessions", 1,
            )
            response = await self._lp.perform_rpc(
                destination_identity=self._user_identity(),
                method='open_maps',
                payload=json.dumps(
                    {"destination": destination},
                ),
            )
            result = json.loads(response)
            await self._backend.record_usage(
                self._uid(),
                "navigation_sessions",
                1,
                "sessions",
                "agent",
                source_id,
                {"destination": destination},
            )
            return result
        except Exception as exc:
            return {"status": "error", "error": str(exc)}

    # --- Phase 3: Volunteer + Location ---

    @function_tool()
    async def rate_volunteer(
        self,
        volunteer_id: str,
        match_id: str,
        score: int,
    ) -> dict:
        """Đánh giá tình nguyện viên sau cuộc gọi (1-5 sao)."""
        return await self._backend.tool_call(
            "rate_volunteer", self._uid(),
            {"volunteer_id": volunteer_id,
             "match_id": match_id, "score": score},
        )

    @function_tool()
    async def get_location(self) -> dict:
        """Lấy vị trí hiện tại — trả về địa chỉ đọc được và tọa độ GPS."""
        try:
            response = await self._lp.perform_rpc(
                destination_identity=self._user_identity(),
                method='get_location',
                payload='{}',
                response_timeout=15.0,
            )
            return json.loads(response)
        except Exception as exc:
            return {"status": "error", "error": str(exc)}

    @function_tool()
    async def update_location(self) -> dict:
        """Cập nhật vị trí hiện tại của người dùng lên hệ thống."""
        try:
            response = await self._lp.perform_rpc(
                destination_identity=self._user_identity(),
                method='get_location',
                payload='{}',
                response_timeout=15.0,
            )
            location = json.loads(response)
            lat = location.get("lat", 0)
            lng = location.get("lng", 0)
        except Exception as exc:
            return {"status": "error", "error": str(exc)}
        return await self._backend.tool_call(
            "update_location", self._uid(),
            {"lat": lat, "lng": lng},
        )
