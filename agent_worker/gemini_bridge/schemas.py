"""Gemini function declarations for AiSee tools."""

from __future__ import annotations

from dataclasses import dataclass, field

from google.genai import types


@dataclass(frozen=True, slots=True)
class ParameterSpec:
    type: types.Type
    description: str
    items: "ParameterSpec | None" = None


@dataclass(frozen=True, slots=True)
class ToolSpec:
    name: str
    description: str
    parameters: dict[str, ParameterSpec] = field(default_factory=dict)
    required: list[str] = field(default_factory=list)


STRING = types.Type.STRING
INTEGER = types.Type.INTEGER
ARRAY = types.Type.ARRAY

TOOL_SPECS = (
    ToolSpec(
        "make_voice_call",
        "Goi dien thoai cho nguoi than theo ten hoac vai tro.",
        {"contact_name": ParameterSpec(STRING, "Ten hoac quan he cua nguoi can goi.")},
        ["contact_name"],
    ),
    ToolSpec(
        "make_video_call",
        "Goi video cho nguoi than theo ten hoac vai tro.",
        {"contact_name": ParameterSpec(STRING, "Ten hoac quan he cua nguoi can goi.")},
        ["contact_name"],
    ),
    ToolSpec(
        "send_chat_message",
        "Gui tin nhan van ban cho nguoi than.",
        {
            "contact_name": ParameterSpec(STRING, "Ten hoac quan he nguoi nhan."),
            "message": ParameterSpec(STRING, "Noi dung tin nhan."),
        },
        ["contact_name", "message"],
    ),
    ToolSpec(
        "read_unread_messages",
        "Doc tin nhan chua doc, co the loc theo lien he.",
        {"contact_name": ParameterSpec(STRING, "Ten lien he tuy chon.")},
    ),
    ToolSpec(
        "activate_camera",
        "Bat/mo camera dien thoai khi nguoi dung noi bat camera, mo camera, xem giup, "
        "nhin xung quanh, mo mat, hoac can AI mo ta moi truong.",
        {"reason": ParameterSpec(STRING, "Ly do can bat camera.")},
    ),
    ToolSpec(
        "deactivate_camera",
        "Tat/dung/dong camera khi nguoi dung noi tat camera, tat di, dung camera, "
        "thoi nhin, hoac quay ve che do am thanh.",
    ),
    ToolSpec(
        "set_medication_reminder",
        "Dat nhac nho uong thuoc.",
        {
            "medicine_name": ParameterSpec(STRING, "Ten thuoc."),
            "dosage": ParameterSpec(STRING, "Lieu dung."),
            "times": ParameterSpec(ARRAY, "Danh sach gio dang HH:MM.", ParameterSpec(STRING, "Gio")),
            "frequency": ParameterSpec(STRING, "Tan suat, mac dinh daily."),
        },
        ["medicine_name", "dosage", "times"],
    ),
    ToolSpec("list_medication_reminders", "Liet ke nhac nho thuoc va trang thai hom nay."),
    ToolSpec(
        "mark_medication_taken",
        "Danh dau da uong thuoc.",
        {"medicine_name": ParameterSpec(STRING, "Ten thuoc.")},
        ["medicine_name"],
    ),
    ToolSpec(
        "mark_medication_skipped",
        "Danh dau bo qua thuoc hom nay.",
        {"medicine_name": ParameterSpec(STRING, "Ten thuoc.")},
        ["medicine_name"],
    ),
    ToolSpec(
        "log_health_symptom",
        "Ghi nhan trieu chung suc khoe.",
        {
            "symptom": ParameterSpec(STRING, "Trieu chung."),
            "severity": ParameterSpec(STRING, "Muc do nghiem trong tuy chon."),
            "notes": ParameterSpec(STRING, "Ghi chu bo sung."),
        },
        ["symptom"],
    ),
    ToolSpec(
        "log_health_metric",
        "Ghi nhan chi so suc khoe.",
        {
            "metric_type": ParameterSpec(STRING, "Loai chi so."),
            "value": ParameterSpec(STRING, "Gia tri."),
            "unit": ParameterSpec(STRING, "Don vi tuy chon."),
        },
        ["metric_type", "value"],
    ),
    ToolSpec(
        "set_appointment_reminder",
        "Dat nhac nho lich hen.",
        {
            "title": ParameterSpec(STRING, "Tieu de lich hen."),
            "datetime": ParameterSpec(STRING, "Thoi gian dang YYYY-MM-DD HH:MM."),
            "location": ParameterSpec(STRING, "Dia diem tuy chon."),
            "notes": ParameterSpec(STRING, "Ghi chu tuy chon."),
        },
        ["title", "datetime"],
    ),
    ToolSpec("emergency_alert", "Gui canh bao SOS kem vi tri neu co."),
    ToolSpec(
        "escalate_to_family",
        "Goi nguoi than ho tro, co the bo qua nhung nguoi da thu.",
        {
            "contact_name": ParameterSpec(STRING, "Lien he uu tien tuy chon."),
            "skip": ParameterSpec(ARRAY, "Danh sach lien he bo qua.", ParameterSpec(STRING, "Lien he")),
        },
    ),
    ToolSpec(
        "navigate_to",
        "Mo ban do va chi duong den dia diem.",
        {"destination": ParameterSpec(STRING, "Diem den.")},
        ["destination"],
    ),
    ToolSpec(
        "web_search",
        "Tim kiem web bang Exa khi can thong tin moi, tin tuc, thoi tiet, gia ca, su kien, hoac du lieu hien tai.",
        {
            "query": ParameterSpec(STRING, "Cau truy van can tim tren web."),
            "max_results": ParameterSpec(INTEGER, "So ket qua toi da, mac dinh 3, toi da 5."),
        },
        ["query"],
    ),
    ToolSpec("escalate_to_volunteer", "Ket noi tinh nguyen vien khi gia dinh khong tra loi."),
    ToolSpec(
        "rate_volunteer",
        "Danh gia tinh nguyen vien sau cuoc goi.",
        {
            "volunteer_id": ParameterSpec(STRING, "ID tinh nguyen vien."),
            "match_id": ParameterSpec(STRING, "ID phien ket noi."),
            "score": ParameterSpec(INTEGER, "Diem 1 den 5."),
        },
        ["volunteer_id", "match_id", "score"],
    ),
    ToolSpec("get_location", "Lay vi tri hien tai tu dien thoai."),
    ToolSpec("get_current_location", "Lay vi tri hien tai tu dien thoai."),
    ToolSpec("update_location", "Cap nhat vi tri hien tai len he thong."),
)


def tool_names() -> list[str]:
    return [spec.name for spec in TOOL_SPECS]


def build_gemini_tools(*, enable_google_search: bool = True) -> list[types.Tool]:
    declarations = [
        types.FunctionDeclaration(
            name=spec.name,
            description=spec.description,
            parameters=_object_schema(spec.parameters, spec.required),
        )
        for spec in TOOL_SPECS
    ]
    tools = []
    if enable_google_search:
        tools.append(types.Tool(google_search=types.GoogleSearch()))
    tools.append(types.Tool(function_declarations=declarations))
    return tools


def _object_schema(properties: dict[str, ParameterSpec], required: list[str]) -> types.Schema:
    return types.Schema(
        type=types.Type.OBJECT,
        properties={name: _schema(param) for name, param in properties.items()},
        required=required or None,
    )


def _schema(param: ParameterSpec) -> types.Schema:
    return types.Schema(
        type=param.type,
        description=param.description,
        items=_schema(param.items) if param.items else None,
    )
