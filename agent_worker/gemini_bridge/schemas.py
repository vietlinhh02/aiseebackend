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
        "Gửi tin nhắn văn bản cho người thân. "
        "Chỉ gọi khi user cung cấp CẢ TÊN/VAI TRÒ và CẢ NỘI DUNG tin nhắn. "
        "Nếu thiếu 1 trong 2: hỏi thêm.",
        {
            "contact_name": ParameterSpec(STRING, "Tên hoặc quan hệ người nhận."),
            "message": ParameterSpec(STRING, "Nội dung tin nhắn."),
        },
        ["contact_name", "message"],
    ),
    ToolSpec(
        "read_unread_messages",
        "Đọc tin nhắn chưa đọc. Gọi ngay khi user muốn đọc tin, không cần xác nhận. "
        "Có thể lọc theo tên liên hệ nếu user chỉ định.",
        {"contact_name": ParameterSpec(STRING, "Tên liên hệ tùy chọn.")},
    ),
    ToolSpec(
        "call_family_contact",
        "Gọi điện/video cho người nhà theo tên hoặc vai trò (mẹ, anh, chị, con...). "
        "Chỉ gọi khi user nói RÕ TÊN hoặc VAI TRÒ. "
        "Nếu user chỉ nói 'gọi điện' mà không nói tên: hỏi 'Bạn muốn gọi cho ai?'. "
        "Mở cuộc gọi trên máy người dùng và cho kết quả bắt máy hoặc hết giờ.",
        {
            "contact_name": ParameterSpec(STRING, "Tên hoặc quan hệ của người nhà cần gọi."),
            "call_type": ParameterSpec(STRING, "Loại cuộc gọi: voice hoặc video. Mặc định voice."),
        },
        ["contact_name"],
    ),
    ToolSpec(
        "call_support_contact",
        "Gọi người hỗ trợ/người giúp/tình nguyện viên. "
        "Nếu user nói tên người nhà cụ thể → gọi người nhà đó. "
        "Nếu user nói chung chung 'người hỗ trợ', 'tình nguyện viên', 'ai đó giúp' "
        "→ tìm volunteer từ backend. "
        "Dùng khi user nói 'gọi người giúp', 'cần ai đó hỗ trợ', 'gọi tình nguyện viên'.",
        {
            "contact_name": ParameterSpec(STRING, "Tên người nhà tùy chọn. Để trống để tìm volunteer."),
            "call_type": ParameterSpec(STRING, "Loại cuộc gọi: voice hoặc video. Mặc định voice."),
        },
    ),
    ToolSpec(
        "list_family_contacts",
        "Kiểm tra realtime xem người dùng đã liên kết gia đình chưa và liệt kê các liên hệ hiện có. "
        "Gọi khi user hỏi 'có ai liên kết chưa', 'có những ai', hoặc trước khi gọi người thân "
        "để biết danh sách hiện có.",
    ),
    ToolSpec(
        "activate_camera",
        "Bật camera để AI nhìn qua máy ảnh người dùng. "
        "CHỈ GỌI khi: (1) user nói rõ 'mở camera', 'bật camera', 'xem giúp', 'nhìn giúp' "
        "-> consent_source=explicit_request; HOẶC (2) user đã đồng ý rõ ràng sau khi được hỏi "
        "-> consent_source=explicit_confirmation. "
        "KHÔNG GỌI khi user nói mơ hồ như 'cái này là gì', 'đọc cái này'.",
        {
            "reason": ParameterSpec(STRING, "Lý do cần bật camera."),
            "consent_phrase": ParameterSpec(
                STRING,
                "Cụm từ user vừa nói để cho phép mở camera, giữ gần nguyên văn.",
            ),
            "consent_source": ParameterSpec(
                STRING,
                "explicit_request nếu user tự nói mở/xem; explicit_confirmation nếu user đồng ý sau hỏi.",
            ),
        },
        ["reason", "consent_phrase", "consent_source"],
    ),
    ToolSpec(
        "deactivate_camera",
        "Tắt camera, quay về chế độ âm thanh. "
        "Gọi ngay khi user nói 'tắt camera', 'thôi', 'dừng', 'được rồi', "
        "hoặc khi đã mô tả xong hình ảnh.",
    ),
    ToolSpec(
        "set_medication_reminder",
        "Đặt nhắc nhở uống thuốc. Chỉ hỏi thêm nếu THIẾU tên thuốc, liều, hoặc giờ. "
        "Nếu user nói đầy đủ 'nhắc uống Amlodipine 5mg lúc 8h' -> gọi ngay.",
        {
            "medicine_name": ParameterSpec(STRING, "Tên thuốc."),
            "dosage": ParameterSpec(STRING, "Liều dùng."),
            "times": ParameterSpec(ARRAY, "Danh sách giờ dạng HH:MM.", ParameterSpec(STRING, "Giờ")),
            "frequency": ParameterSpec(STRING, "Tần suất, mặc định daily."),
        },
        ["medicine_name", "dosage", "times"],
    ),
    ToolSpec(
        "list_medication_reminders",
        "Liệt kê nhắc nhở thuốc và trạng thái hôm nay. Gọi ngay khi user hỏi 'hôm nay uống thuốc gì', "
        "'có nhắc nhở nào không', 'thuốc hôm nay'.",
    ),
    ToolSpec(
        "mark_medication_taken",
        "Đánh dấu đã uống thuốc. Gọi ngay khi user nói 'đã uống', 'uống rồi', 'đánh dấu đã uống'.",
        {"medicine_name": ParameterSpec(STRING, "Tên thuốc.")},
        ["medicine_name"],
    ),
    ToolSpec(
        "mark_medication_skipped",
        "Đánh dấu bỏ qua thuốc hôm nay. Gọi ngay khi user nói 'bỏ qua', 'không uống', 'skip'.",
        {"medicine_name": ParameterSpec(STRING, "Tên thuốc.")},
        ["medicine_name"],
    ),
    ToolSpec(
        "log_health_symptom",
        "Ghi nhận triệu chứng sức khỏe (đau đầu, chóng mặt, mệt mỏi...). "
        "Gọi khi user mô tả triệu chứng.",
        {
            "symptom": ParameterSpec(STRING, "Triệu chứng."),
            "severity": ParameterSpec(STRING, "Mức độ nghiêm trọng tùy chọn."),
            "notes": ParameterSpec(STRING, "Ghi chú bổ sung."),
        },
        ["symptom"],
    ),
    ToolSpec(
        "log_health_metric",
        "Ghi nhận chỉ số sức khỏe (huyết áp, nhiệt độ, nhịp tim...). "
        "Gọi khi user nói chỉ số cụ thể.",
        {
            "metric_type": ParameterSpec(STRING, "Loại chỉ số."),
            "value": ParameterSpec(STRING, "Giá trị."),
            "unit": ParameterSpec(STRING, "Đơn vị tùy chọn."),
        },
        ["metric_type", "value"],
    ),
    ToolSpec(
        "set_appointment_reminder",
        "Đặt nhắc nhở lịch hẹn bác sĩ. Chỉ hỏi nếu THIẾU thời gian hoặc tiêu đề.",
        {
            "title": ParameterSpec(STRING, "Tiêu đề lịch hẹn."),
            "datetime": ParameterSpec(STRING, "Thời gian dạng YYYY-MM-DD HH:MM."),
            "location": ParameterSpec(STRING, "Địa điểm tùy chọn."),
            "notes": ParameterSpec(STRING, "Ghi chú tùy chọn."),
        },
        ["title", "datetime"],
    ),
    ToolSpec(
        "emergency_alert",
        "Gửi cảnh báo SOS kèm vị trí GPS đến TẤT CẢ người thân. "
        "Chỉ gọi khi có tình huống nguy hiểm thực sự (ngã, đau, cứu). "
        "KHÔNG cần xác nhận khi user rõ ràng nguy hiểm.",
    ),
    ToolSpec(
        "escalate_to_family",
        "Gọi người thân hỗ trợ khi AI không giúp được hoặc cần nói chuyện trực tiếp. "
        "Dùng skip để bỏ qua người đã thử. "
        "Dùng khi: user hoảng sợ, AI không hiểu sau 2 lần, hoặc user yêu cầu.",
        {
            "contact_name": ParameterSpec(STRING, "Liên hệ ưu tiên tùy chọn."),
            "skip": ParameterSpec(ARRAY, "Danh sách liên hệ bỏ qua.", ParameterSpec(STRING, "Liên hệ")),
        },
    ),
    ToolSpec(
        "navigate_to",
        "Mở bản đồ và chỉ đường đến địa điểm. Gọi khi user nói 'dẫn tới...', 'chỉ đường...', 'mở bản đồ'.",
        {"destination": ParameterSpec(STRING, "Điểm đến.")},
        ["destination"],
    ),
    ToolSpec(
        "web_search",
        "Tìm kiếm web khi cần thông tin MỚI: tin tức, thời tiết, giá cả, sự kiện, dữ liệu hiện tại. "
        "KHÔNG dùng cho kiến thức cố định, trò chuyện thường, hoặc yêu cầu đã đủ dữ liệu.",
        {
            "query": ParameterSpec(STRING, "Câu truy vấn cần tìm trên web."),
            "max_results": ParameterSpec(INTEGER, "Số kết quả tối đa, mặc định 3."),
        },
        ["query"],
    ),
    ToolSpec(
        "rate_volunteer",
        "Đánh giá tình nguyện viên sau cuộc gọi (1-5 sao). "
        "Gọi sau khi cuộc gọi volunteer kết thúc và user đã cho điểm.",
        {
            "volunteer_id": ParameterSpec(STRING, "ID tình nguyện viên."),
            "match_id": ParameterSpec(STRING, "ID phiên kết nối."),
            "score": ParameterSpec(INTEGER, "Điểm 1 đến 5."),
        },
        ["volunteer_id", "match_id", "score"],
    ),
    ToolSpec(
        "get_location",
        "Lấy vị trí hiện tại từ điện thoại người dùng. "
        "Gọi khi user hỏi 'tôi đang ở đâu', 'vị trí hiện tại'.",
    ),
    ToolSpec(
        "update_location",
        "Cập nhật vị trí hiện tại lên hệ thống để người thân có thể theo dõi. "
        "Gọi khi user yêu cầu cập nhật vị trí.",
    ),
    ToolSpec(
        "answer_incoming_call",
        "Bắt máy khi có cuộc gọi đến đang đổ chuông. "
        "CHỈ GỌI khi user đồng ý: 'nghe', 'bắt máy', 'ok'.",
    ),
    ToolSpec(
        "decline_incoming_call",
        "Từ chối cuộc gọi đến. "
        "CHỈ GỌI khi user nói 'không nghe', 'từ chối', 'tắt'.",
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
