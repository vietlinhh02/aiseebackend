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
        "send_chat_message",
        "Gui tin nhan van ban cho nguoi than. "
        "Chi goi khi user cung cap CA TEN/VAI TRO va CA NOI DUNG tin nhan. "
        "Neu thieu 1 trong 2: hoi them.",
        {
            "contact_name": ParameterSpec(STRING, "Ten hoac quan he nguoi nhan."),
            "message": ParameterSpec(STRING, "Noi dung tin nhan."),
        },
        ["contact_name", "message"],
    ),
    ToolSpec(
        "read_unread_messages",
        "Doc tin nhan chua doc. Goi ngay khi user muon doc tin, khong can xac nhan. "
        "Co the loc theo ten lien he neu user chi dinh.",
        {"contact_name": ParameterSpec(STRING, "Ten lien he tuy chon.")},
    ),
    ToolSpec(
        "call_family_contact",
        "Goi dien/video cho nguoi nha theo ten hoac vai tro (me, anh, chi, con...). "
        "Chi goi khi user noi R TEN hoac VAI TRO. "
        "Neu user chi noi 'goi dien' ma khong noi ten: hoi 'Ban muon goi cho ai?'. "
        "Mo cuoc goi tren may nguoi dung va cho ket qua bat may hoac het gio.",
        {
            "contact_name": ParameterSpec(STRING, "Ten hoac quan he cua nguoi nha can goi."),
            "call_type": ParameterSpec(STRING, "Loai cuoc goi: voice hoac video. Mac dinh voice."),
        },
        ["contact_name"],
    ),
    ToolSpec(
        "call_support_contact",
        "Goi nguoi ho tro/nguoi giup/tinh nguyen vien. "
        "Neu user noi ten nguoi nha cu the → goi nguoi nha do. "
        "Neu user noi chung chung 'nguoi ho tro', 'tinh nguyen vien', 'ai do giup' "
        "→ tim volunteer tu backend. "
        "Dung khi user noi 'goi nguoi giup', 'can ai do ho tro', 'goi tinh nguyen vien'.",
        {
            "contact_name": ParameterSpec(STRING, "Ten nguoi nha tuy chon. Bo trong de tim volunteer."),
            "call_type": ParameterSpec(STRING, "Loai cuoc goi: voice hoac video. Mac dinh voice."),
        },
    ),
    ToolSpec(
        "list_family_contacts",
        "Kiem tra realtime xem nguoi dung da lien ket gia dinh chua va liet ke cac lien he. "
        "Goi khi user hoi 'co ai lien ket chua', 'co nhung ai', hoac truoc khi goi nguoi than "
        "de biet danh sach hien co.",
    ),
    ToolSpec(
        "activate_camera",
        "Bat camera de AI nhin qua may anh nguoi dung. "
        "CHI GOI khi: (1) user noi ro 'mo camera', 'bat camera', 'xem giup', 'nhin giup' "
        "→ consent_source=explicit_request; HOAC (2) user da dong y ro rang sau khi duoc hoi "
        "→ consent_source=explicit_confirmation. "
        "KHONG GOI khi user noi mo ho nhu 'cai nay la gi', 'doc cai nay'.",
        {
            "reason": ParameterSpec(STRING, "Ly do can bat camera."),
            "consent_phrase": ParameterSpec(
                STRING,
                "Cum tu user vua noi de cho phep mo camera, giu gan nguyen van.",
            ),
            "consent_source": ParameterSpec(
                STRING,
                "explicit_request neu user tu noi mo/xem; explicit_confirmation neu user dong y sau hoi.",
            ),
        },
        ["reason", "consent_phrase", "consent_source"],
    ),
    ToolSpec(
        "deactivate_camera",
        "Tat camera, quay ve che do am thanh. "
        "Goi ngay khi user noi 'tat camera', 'thoi', 'dung', 'duoc roi', "
        "hoac khi da mo ta xong hinh anh.",
    ),
    ToolSpec(
        "set_medication_reminder",
        "Dat nhac nho uong thuoc. Chi hoi them neu THIEU ten thuoc, lieu, hoac gio. "
        "Neu user noi day du 'nhac uong Amlodipine 5mg luc 8h' → goi ngay.",
        {
            "medicine_name": ParameterSpec(STRING, "Ten thuoc."),
            "dosage": ParameterSpec(STRING, "Lieu dung."),
            "times": ParameterSpec(ARRAY, "Danh sach gio dang HH:MM.", ParameterSpec(STRING, "Gio")),
            "frequency": ParameterSpec(STRING, "Tan suat, mac dinh daily."),
        },
        ["medicine_name", "dosage", "times"],
    ),
    ToolSpec(
        "list_medication_reminders",
        "Liet ke nhac nho thuoc va trang thai hom nay. Goi ngay khi user hoi 'hom nay uong thuoc gi', "
        "'co nhac nho nao khong', 'thuoc hom nay'.",
    ),
    ToolSpec(
        "mark_medication_taken",
        "Danh dau da uong thuoc. Goi ngay khi user noi 'da uong', 'uong roi', 'danh dau da uong'.",
        {"medicine_name": ParameterSpec(STRING, "Ten thuoc.")},
        ["medicine_name"],
    ),
    ToolSpec(
        "mark_medication_skipped",
        "Danh dau bo qua thuoc hom nay. Goi ngay khi user noi 'bo qua', 'khong uong', 'skip'.",
        {"medicine_name": ParameterSpec(STRING, "Ten thuoc.")},
        ["medicine_name"],
    ),
    ToolSpec(
        "log_health_symptom",
        "Ghi nhan trieu chung suc khoe (dau dau, chong mat, met moi...). "
        "Goi khi user mo ta trieu chung.",
        {
            "symptom": ParameterSpec(STRING, "Trieu chung."),
            "severity": ParameterSpec(STRING, "Muc do nghiem trong tuy chon."),
            "notes": ParameterSpec(STRING, "Ghi chu bo sung."),
        },
        ["symptom"],
    ),
    ToolSpec(
        "log_health_metric",
        "Ghi nhan chi so suc khoe (huyet ap, nhiet do, nhip tim...). "
        "Goi khi user noi chi so cu the.",
        {
            "metric_type": ParameterSpec(STRING, "Loai chi so."),
            "value": ParameterSpec(STRING, "Gia tri."),
            "unit": ParameterSpec(STRING, "Don vi tuy chon."),
        },
        ["metric_type", "value"],
    ),
    ToolSpec(
        "set_appointment_reminder",
        "Dat nhac nho lich hen bac si. Chi hoi neu THIEU thoi gian hoac tieu de.",
        {
            "title": ParameterSpec(STRING, "Tieu de lich hen."),
            "datetime": ParameterSpec(STRING, "Thoi gian dang YYYY-MM-DD HH:MM."),
            "location": ParameterSpec(STRING, "Dia diem tuy chon."),
            "notes": ParameterSpec(STRING, "Ghi chu tuy chon."),
        },
        ["title", "datetime"],
    ),
    ToolSpec(
        "emergency_alert",
        "Gui canh bao SOS kem vi tri GPS den TAT CA nguoi than. "
        "Chi goi khi co tinh huong nguy hiem thuc su (ngã, đau, cứu). "
        "KHONG can xac nhan khi user ro rang nguy hiem.",
    ),
    ToolSpec(
        "escalate_to_family",
        "Goi nguoi than ho tro khi AI khong giup duoc hoac can noi chuyen truc tiep. "
        "Dung skip de bo qua nguoi da thu. "
        "Dung khi: user hoang so, AI khong hieu sau 2 lan, hoac user yeu cau.",
        {
            "contact_name": ParameterSpec(STRING, "Lien he uu tien tuy chon."),
            "skip": ParameterSpec(ARRAY, "Danh sach lien he bo qua.", ParameterSpec(STRING, "Lien he")),
        },
    ),
    ToolSpec(
        "navigate_to",
        "Mo ban do va chi duong den dia diem. Goi khi user noi 'dan toi...', 'chi duong...', 'mo ban do'.",
        {"destination": ParameterSpec(STRING, "Diem den.")},
        ["destination"],
    ),
    ToolSpec(
        "web_search",
        "Tim kiem web khi can thong tin MOI: tin tuc, thoi tiet, gia ca, su kien, du lieu hien tai. "
        "KHONG dung cho kien thuc co dinh, tro chuyen thuong, hoac yeu da du du lieu.",
        {
            "query": ParameterSpec(STRING, "Cau truy van can tim tren web."),
            "max_results": ParameterSpec(INTEGER, "So ket qua toi da, mac dinh 3."),
        },
        ["query"],
    ),
    ToolSpec(
        "rate_volunteer",
        "Danh gia tinh nguyen vien sau cuoc goi (1-5 sao). "
        "Goi sau khi cuoc goi volunteer ket thuc va user da cho diem.",
        {
            "volunteer_id": ParameterSpec(STRING, "ID tinh nguyen vien."),
            "match_id": ParameterSpec(STRING, "ID phien ket noi."),
            "score": ParameterSpec(INTEGER, "Diem 1 den 5."),
        },
        ["volunteer_id", "match_id", "score"],
    ),
    ToolSpec(
        "get_location",
        "Lay vi tri hien tai tu dien thoai nguoi dung. "
        "Goi khi user hoi 'toi dang o dau', 'vi tri hien tai'.",
    ),
    ToolSpec(
        "update_location",
        "Cap nhat vi tri hien tai len he thong de nguoi than co the theo doi. "
        "Goi khi user yeu cau cap nhat vi tri.",
    ),
    ToolSpec(
        "answer_incoming_call",
        "Bat may khi co cuoc goi den dang do chuong. "
        "CHI GOI khi user dong y: 'nghe', 'bat may', 'ok'.",
    ),
    ToolSpec(
        "decline_incoming_call",
        "Tu choi cuoc goi den. "
        "CHI GOI khi user noi 'khong nghe', 'tu choi', 'tat'.",
    ),
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
