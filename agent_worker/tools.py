"""AiSee agent with tool definitions — Phase 1-3."""

import json
import logging

from livekit import rtc
from livekit.agents import Agent, function_tool

from backend import BackendClient
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
    async def make_voice_call(self, contact_name: str) -> dict:
        """Call a family member by name or relationship (voice only)."""
        return await self._backend.tool_call(
            "make_voice_call", self._uid(),
            {"contact_name": contact_name},
        )

    @function_tool()
    async def make_video_call(self, contact_name: str) -> dict:
        """Video call a family member by name or relationship."""
        return await self._backend.tool_call(
            "make_video_call", self._uid(),
            {"contact_name": contact_name},
        )

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
    async def activate_camera(
        self, reason: str = "User requested",
    ) -> dict:
        """Enable camera to see through user's phone."""
        try:
            response = await self._lp.perform_rpc(
                destination_identity=self._user_identity(),
                method='enable_camera',
                payload='{}',
            )
            return json.loads(response)
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
        """Dat nhac nho uong thuoc. times la danh sach gio (VD: ["07:00", "21:00"])."""
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
        """Liet ke thuoc hom nay va trang thai da uong chua."""
        return await self._backend.tool_call(
            "list_medication_reminders", self._uid(), {},
        )

    @function_tool()
    async def mark_medication_taken(
        self, medicine_name: str,
    ) -> dict:
        """Xac nhan da uong thuoc."""
        return await self._backend.tool_call(
            "mark_medication_taken", self._uid(),
            {"medicine_name": medicine_name},
        )

    @function_tool()
    async def mark_medication_skipped(
        self, medicine_name: str,
    ) -> dict:
        """Xac nhan bo qua khong uong thuoc hom nay."""
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
        """Ghi nhan trieu chung suc khoe (VD: dau dau, chong mat, met moi)."""
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
        """Ghi nhan chi so suc khoe (VD: huyet ap 140/90, nhiet do 37.5)."""
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
        """Dat nhac nho lich hen bac si. datetime dang YYYY-MM-DD HH:MM."""
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
        """SOS khan cap — gui thong bao den tat ca nguoi than kem vi tri GPS."""
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
        """Goi nguoi than ho tro khi AI khong giup duoc. Neu khong ai tra loi, goi lai voi skip de thu nguoi khac."""
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
        """Tim va phat nhac theo yeu cau."""
        result = await self._backend.tool_call(
            "play_music", self._uid(), {"query": query},
        )
        play_action = result.pop("play_action", None)
        if play_action and isinstance(play_action, dict):
            try:
                await self._lp.perform_rpc(
                    destination_identity=self._user_identity(),
                    method='play_audio',
                    payload=json.dumps(
                        play_action.get("params", {}),
                    ),
                )
            except Exception as exc:
                logger.warning(
                    "play_audio RPC failed: %s", exc,
                )
        return result

    @function_tool()
    async def control_music(self, action: str) -> dict:
        """Dieu khien nhac dang phat: pause, resume, stop."""
        try:
            response = await self._lp.perform_rpc(
                destination_identity=self._user_identity(),
                method='control_audio',
                payload=json.dumps({"command": action}),
            )
            return json.loads(response)
        except Exception as exc:
            return {"status": "error", "error": str(exc)}

    # --- Phase 2d: Navigation ---

    @function_tool()
    async def navigate_to(self, destination: str) -> dict:
        """Mo ban do va chi duong den dia diem."""
        try:
            response = await self._lp.perform_rpc(
                destination_identity=self._user_identity(),
                method='open_maps',
                payload=json.dumps(
                    {"destination": destination},
                ),
            )
            return json.loads(response)
        except Exception as exc:
            return {"status": "error", "error": str(exc)}

    # --- Phase 3: Volunteer + Location ---

    @function_tool()
    async def escalate_to_volunteer(self) -> dict:
        """Ket noi voi tinh nguyen vien khi gia dinh khong tra loi. Tra ve match_id de theo doi."""
        return await self._backend.tool_call(
            "escalate_to_volunteer", self._uid(), {},
        )

    @function_tool()
    async def rate_volunteer(
        self,
        volunteer_id: str,
        match_id: str,
        score: int,
    ) -> dict:
        """Danh gia tinh nguyen vien sau cuoc goi (1-5 sao)."""
        return await self._backend.tool_call(
            "rate_volunteer", self._uid(),
            {"volunteer_id": volunteer_id,
             "match_id": match_id, "score": score},
        )

    @function_tool()
    async def get_current_location(self) -> dict:
        """Lay vi tri hien tai — tra ve dia chi doc duoc va toa do GPS."""
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
        """Cap nhat vi tri hien tai cua nguoi dung len he thong."""
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
