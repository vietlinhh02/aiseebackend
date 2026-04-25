"""Generate AiSee Data Analysis Report in DOCX format."""

from docx import Document
from docx.shared import Inches, Pt, Cm, RGBColor
from docx.enum.text import WD_ALIGN_PARAGRAPH
from docx.enum.table import WD_TABLE_ALIGNMENT
from docx.enum.section import WD_ORIENT
import datetime


def add_heading_styled(doc, text, level=1):
    h = doc.add_heading(text, level=level)
    for run in h.runs:
        run.font.color.rgb = RGBColor(0x1A, 0x32, 0x63)
    return h


def add_table(doc, headers, rows):
    table = doc.add_table(rows=1 + len(rows), cols=len(headers))
    table.style = "Light Grid Accent 1"
    table.alignment = WD_TABLE_ALIGNMENT.CENTER
    for i, h in enumerate(headers):
        cell = table.rows[0].cells[i]
        cell.text = h
        for p in cell.paragraphs:
            for run in p.runs:
                run.bold = True
                run.font.size = Pt(9)
    for r_idx, row in enumerate(rows):
        for c_idx, val in enumerate(row):
            cell = table.rows[r_idx + 1].cells[c_idx]
            cell.text = str(val)
            for p in cell.paragraphs:
                for run in p.runs:
                    run.font.size = Pt(9)
    return table


def build_report():
    doc = Document()

    # Page setup
    for section in doc.sections:
        section.top_margin = Cm(2)
        section.bottom_margin = Cm(2)
        section.left_margin = Cm(2.5)
        section.right_margin = Cm(2.5)

    style = doc.styles["Normal"]
    style.font.name = "Calibri"
    style.font.size = Pt(11)

    # ========== TITLE PAGE ==========
    for _ in range(6):
        doc.add_paragraph()

    title = doc.add_paragraph()
    title.alignment = WD_ALIGN_PARAGRAPH.CENTER
    run = title.add_run("BÁO CÁO PHÂN TÍCH DỮ LIỆU TOÀN DIỆN")
    run.bold = True
    run.font.size = Pt(28)
    run.font.color.rgb = RGBColor(0x1A, 0x32, 0x63)

    subtitle = doc.add_paragraph()
    subtitle.alignment = WD_ALIGN_PARAGRAPH.CENTER
    run = subtitle.add_run("ỨNG DỤNG AISEE — TRỢ LÝ AI CHO NGƯỜI KHIẾM THỊ")
    run.font.size = Pt(16)
    run.font.color.rgb = RGBColor(0x4A, 0x6F, 0xA5)

    doc.add_paragraph()
    meta = doc.add_paragraph()
    meta.alignment = WD_ALIGN_PARAGRAPH.CENTER
    run = meta.add_run(
        f"Ngày lập: {datetime.date.today().strftime('%d/%m/%Y')}\n"
        "Đội ngũ: Team Đắng — VentureX S3 / VinUni ELAB\n"
        "Phiên bản: v1.0.0 (MVP)\n"
        "Người phân tích: Data Analysis Report"
    )
    run.font.size = Pt(12)
    run.font.color.rgb = RGBColor(0x66, 0x66, 0x66)

    doc.add_page_break()

    # ========== TABLE OF CONTENTS ==========
    add_heading_styled(doc, "MỤC LỤC", 1)
    toc_items = [
        "1. Tóm tắt điều hành (Executive Summary)",
        "2. Tổng quan sản phẩm",
        "3. Phân tích đối tượng người dùng",
        "4. Phân tích kiến trúc hệ thống & dữ liệu",
        "5. Mô hình dữ liệu & luồng dữ liệu",
        "6. Phân tích tính năng & mức độ hoàn thiện",
        "7. Phân tích trải nghiệm người dùng (UX)",
        "8. Phân tích khả năng tiếp cận (Accessibility)",
        "9. Phân tích hiệu suất & khả năng mở rộng",
        "10. Phân tích bảo mật & quyền riêng tư",
        "11. Phân tích mô hình kinh doanh & monetization",
        "12. Phân tích cạnh tranh",
        "13. Phân tích chi phí vận hành",
        "14. KPIs & metrics đề xuất",
        "15. Phân tích rủi ro",
        "16. Đề xuất & lộ trình phát triển",
        "17. Kết luận",
    ]
    for item in toc_items:
        p = doc.add_paragraph(item)
        p.paragraph_format.space_after = Pt(4)

    doc.add_page_break()

    # ========== 1. EXECUTIVE SUMMARY ==========
    add_heading_styled(doc, "1. Tóm tắt điều hành (Executive Summary)", 1)

    doc.add_paragraph(
        "AiSee là ứng dụng trợ lý AI dành cho người khiếm thị tại Việt Nam, "
        "được phát triển bởi đội ngũ Team Đắng trong khuôn khổ VentureX Season 3 / VinUni ELAB. "
        "Ứng dụng sử dụng công nghệ voice-first (ưu tiên giọng nói) kết hợp với AI Gemini Live "
        "để hỗ trợ người dùng trong các hoạt động hàng ngày: giao tiếp, quản lý sức khỏe, "
        "điều hướng, giải trí và kết nối với gia đình/tình nguyện viên."
    )

    add_heading_styled(doc, "Các phát hiện chính", 2)
    findings = [
        "Sản phẩm MVP hoàn thiện với 20 công cụ AI, hệ thống real-time voice, "
        "chat, gọi điện, và dashboard gia đình.",
        "Kiến trúc 4 thành phần (App - Backend - Agent Worker - LiveKit) "
        "được thiết kế tối ưu cho tiết kiệm pin và băng thông.",
        "Chưa có hệ thống analytics/tracking — đây là lỗ hổng lớn nhất "
        "cần được bổ sung ngay.",
        "Mô hình monetization chưa rõ ràng — chỉ có quảng cáo tùy chọn cho "
        "vai trò gia đình, chưa có subscription hay payment integration.",
        "Chi phí vận hành ước tính $60-130/tháng cho MVP, có thể tăng nhanh "
        "khi scale lên 1000+ users do chi phí Gemini API và LiveKit.",
        "Accessibility là điểm mạnh cốt lõi — voice-first design, wake-word detection, "
        "haptic feedback, semantic labels.",
        "Thị trường tiềm năng: ~2 triệu người khiếm thị tại Việt Nam, "
        "chưa có đối thủ trực tiếp với giải pháp tương tự.",
    ]
    for f in findings:
        doc.add_paragraph(f, style="List Bullet")

    doc.add_page_break()

    # ========== 2. TỔNG QUAN SẢN PHẨM ==========
    add_heading_styled(doc, "2. Tổng quan sản phẩm", 1)

    add_heading_styled(doc, "2.1 Mô tả sản phẩm", 2)
    doc.add_paragraph(
        "AiSee (\"AI See\" — AI nhìn giúp bạn) là ứng dụng di động cross-platform "
        "(iOS/Android) cung cấp trợ lý AI thông minh cho người khiếm thị. "
        "Slogan: \"Không chỉ nhìn thay bạn — hành động cùng bạn.\""
    )

    add_heading_styled(doc, "2.2 Thông tin kỹ thuật cơ bản", 2)
    add_table(doc, ["Thuộc tính", "Giá trị"], [
        ["Tên ứng dụng", "AiSee"],
        ["Phiên bản", "1.0.0+1"],
        ["Framework", "Flutter 3.11.3+ (Dart)"],
        ["Backend", "Go 1.26.1 (Chi router)"],
        ["AI Engine", "Google Gemini 3.1 Flash Live"],
        ["Real-time", "LiveKit (WebRTC SFU)"],
        ["Database", "PostgreSQL 16 + Redis 7"],
        ["Storage", "MinIO (S3-compatible)"],
        ["Push", "APNs (iOS) + FCM (Android)"],
        ["Domain", "data.visme.tech"],
    ])

    add_heading_styled(doc, "2.3 Hệ thống vai trò", 2)
    doc.add_paragraph(
        "Ứng dụng có 2 vai trò chính, mỗi vai trò có giao diện và tính năng riêng:"
    )
    add_table(doc, ["Vai trò", "Đối tượng", "Tính năng chính", "Chi phí"], [
        ["User", "Người khiếm thị", "Trợ lý AI voice, nhắc thuốc, gọi điện, SOS", "Miễn phí"],
        ["Family", "Gia đình/người chăm sóc", "Dashboard giám sát, chat, quản lý thuốc", "Có thể có quảng cáo"],
    ])

    doc.add_page_break()

    # ========== 3. PHÂN TÍCH ĐỐI TƯỢNG NGƯỜI DÙNG ==========
    add_heading_styled(doc, "3. Phân tích đối tượng người dùng", 1)

    add_heading_styled(doc, "3.1 Persona chính: Người khiếm thị", 2)
    add_table(doc, ["Đặc điểm", "Chi tiết"], [
        ["Nhân khẩu học", "Mọi lứa tuổi, tập trung người trưởng thành tại Việt Nam"],
        ["Số lượng tiềm năng", "~2 triệu người khiếm thị tại VN (WHO, 2023)"],
        ["Nhu cầu chính", "Hỗ trợ sinh hoạt hàng ngày, giao tiếp, di chuyển"],
        ["Thách thức", "Khó sử dụng smartphone thông thường, phụ thuộc người thân"],
        ["Hành vi kỹ thuật số", "Sử dụng VoiceOver/TalkBack, ưu tiên tương tác bằng giọng nói"],
        ["Ngôn ngữ", "Tiếng Việt (100% UI tiếng Việt)"],
    ])

    add_heading_styled(doc, "3.2 Persona phụ: Gia đình/Người chăm sóc", 2)
    add_table(doc, ["Đặc điểm", "Chi tiết"], [
        ["Nhân khẩu học", "Con cái, vợ/chồng, người chăm sóc chuyên nghiệp"],
        ["Nhu cầu chính", "Giám sát sức khỏe, biết vị trí, nhận cảnh báo khẩn cấp"],
        ["Động lực", "An tâm về người thân, theo dõi tuân thủ thuốc"],
        ["Hành vi", "Kiểm tra dashboard định kỳ, phản hồi nhanh khi có SOS"],
    ])

    add_heading_styled(doc, "3.3 Persona mở rộng: Tình nguyện viên", 2)
    doc.add_paragraph(
        "Hệ thống hỗ trợ đăng ký tình nguyện viên với tracking đánh giá (1-5 sao), "
        "trạng thái available/unavailable, và matching tự động khi có emergency escalation. "
        "Tuy nhiên tính năng này còn ở giai đoạn sớm."
    )

    add_heading_styled(doc, "3.4 Phân tích User Journey", 2)
    doc.add_paragraph("Luồng chính của người dùng khiếm thị:")
    steps = [
        "Đăng nhập bằng Google → Chọn vai trò 'User' → Cấp quyền (mic, camera, location, notification)",
        "Vào màn hình chính → Tab 'Trợ lý' luôn lắng nghe wake-word 'cần giúp'",
        "Nói 'cần giúp' → Micro kích hoạt → Nói yêu cầu (ví dụ: 'gọi cho mẹ')",
        "Agent xử lý → Thực hiện công cụ (tạo phòng gọi, gửi push) → Phản hồi bằng giọng nói",
        "Nhận nhắc thuốc định kỳ → Nói 'đã uống' → Ghi nhận vào hệ thống",
        "Tình huống khẩn cấp → Nói 'cần giúp khẩn cấp' → Gửi SOS đến toàn bộ gia đình",
    ]
    for i, s in enumerate(steps, 1):
        doc.add_paragraph(f"Bước {i}: {s}", style="List Number")

    doc.add_page_break()

    # ========== 4. KIẾN TRÚC HỆ THỐNG ==========
    add_heading_styled(doc, "4. Phân tích kiến trúc hệ thống & dữ liệu", 1)

    add_heading_styled(doc, "4.1 Kiến trúc tổng quan", 2)
    doc.add_paragraph(
        "Hệ thống gồm 4 thành phần chính kết nối qua một LiveKit room duy nhất cho mỗi user, "
        "tối ưu cho tiết kiệm pin và giảm độ phức tạp:"
    )
    add_table(doc, ["Thành phần", "Công nghệ", "Vai trò"], [
        ["Flutter App", "Flutter 3.11.3 / Dart", "Giao diện người dùng, wake-word detection, LiveKit client"],
        ["Go Backend", "Go 1.26.1 / Chi router", "Business logic, API, auth, push notifications, DB"],
        ["Agent Worker", "Python / google-genai", "Chạy Gemini Live API, xử lý voice, routing tool calls"],
        ["LiveKit Server", "LiveKit Cloud (WebRTC SFU)", "Quản lý audio/video tracks, data channel, room management"],
    ])

    add_heading_styled(doc, "4.2 Luồng dữ liệu chính", 2)
    flows = [
        "Voice Input: User → Mic → LiveKit Audio Track → Agent Worker → Gemini Live API",
        "Voice Output: Gemini → Agent Worker → LiveKit Audio Track → User Speaker",
        "Tool Call: Gemini → Agent Worker → HTTP POST /api/internal/tool-call → Go Backend → DB/Redis",
        "Chat: App → WebSocket /ws/chat → Go Backend → DB + Push notification → Family App",
        "File Upload: App → POST /upload → Go Backend → MinIO → Signed URL returned",
        "Push: Go Backend → APNs (iOS VoIP cho calls, standard cho rest) + FCM (Android)",
    ]
    for f in flows:
        doc.add_paragraph(f, style="List Bullet")

    add_heading_styled(doc, "4.3 Infrastructure Stack", 2)
    add_table(doc, ["Service", "Mục đích", "Cấu hình"], [
        ["PostgreSQL 16", "Primary datastore", "10 tables, UUID PKs, JSONB settings"],
        ["Redis 7", "Cache & queue", "Reminder scheduling, session state, volunteer matching"],
        ["MinIO", "Object storage (S3)", "Bucket 'aisee', voice messages, avatars"],
        ["LiveKit Cloud", "WebRTC SFU", "wss://hehe-2ask256r.livekit.cloud"],
        ["Docker Compose", "Orchestration", "5 services (PG, Redis, MinIO, Backend, Agent)"],
    ])

    doc.add_page_break()

    # ========== 5. MÔ HÌNH DỮ LIỆU ==========
    add_heading_styled(doc, "5. Mô hình dữ liệu & luồng dữ liệu", 1)

    add_heading_styled(doc, "5.1 Database Schema (10 bảng)", 2)
    add_table(doc, ["Bảng", "Mục đích", "Chiến lược", "Dữ liệu nhạy cảm"], [
        ["users", "Thông tin người dùng", "UUID PK, JSONB settings", "Email, tên, vị trí GPS, push tokens"],
        ["family_links", "Quan hệ gia đình", "Soft-delete", "Quan hệ giữa users"],
        ["messages", "Tin nhắn text/voice/image", "Append-only, cursor pagination", "Nội dung tin nhắn, URL voice/image"],
        ["medications", "Định nghĩa thuốc", "Soft-delete, active flag", "Tên thuốc, liều lượng, lịch uống"],
        ["medication_logs", "Lịch sử uống thuốc", "Append-only", "Tuân thủ thuốc theo thời gian"],
        ["health_symptoms", "Triệu chứng sức khỏe", "Append-only", "Mô tả triệu chứng, mức độ"],
        ["health_metrics", "Chỉ số sức khỏe", "Append-only", "Huyết áp, đường huyết, cân nặng"],
        ["appointments", "Lịch hẹn khám", "Soft-delete", "Thời gian, địa điểm khám"],
        ["volunteers", "Hồ sơ tình nguyện viên", "1:1 with users", "Trạng thái, đánh giá"],
        ["audit_logs", "Log tool calls từ AI", "Append-only, never deleted", "Toàn bộ tương tác AI-user"],
    ])

    add_heading_styled(doc, "5.2 Phân tích dữ liệu thu thập", 2)
    doc.add_paragraph(
        "AiSee thu thập các loại dữ liệu sau, phân loại theo mức độ nhạy cảm:"
    )

    add_heading_styled(doc, "Dữ liệu nhạy cảm cao (PII + Health)", 3)
    items = [
        "Thông tin cá nhân: Email, tên hiển thị, avatar (qua Google OAuth)",
        "Dữ liệu sức khỏe: Thuốc, liều lượng, triệu chứng, chỉ số sinh hiệu",
        "Vị trí GPS: Tọa độ lat/lng, địa chỉ reverse-geocoded",
        "Tin nhắn: Nội dung text, file voice, hình ảnh",
        "Audit logs: Toàn bộ lệnh AI thực hiện (bao gồm nội dung yêu cầu)",
    ]
    for item in items:
        doc.add_paragraph(item, style="List Bullet")

    add_heading_styled(doc, "Dữ liệu kỹ thuật", 3)
    items = [
        "FCM/APNs tokens cho push notifications",
        "JWT tokens (access + refresh)",
        "LiveKit room tokens",
        "Session state (Redis, ephemeral)",
    ]
    for item in items:
        doc.add_paragraph(item, style="List Bullet")

    add_heading_styled(doc, "5.3 Phân tích lỗ hổng dữ liệu", 2)
    doc.add_paragraph(
        "Các loại dữ liệu quan trọng CHƯA được thu thập nhưng cần thiết cho data analytics:"
    )
    add_table(doc, ["Dữ liệu thiếu", "Tại sao cần", "Ưu tiên"], [
        ["Session duration", "Đo engagement, thời gian sử dụng trung bình", "Cao"],
        ["Feature usage events", "Biết tính năng nào được dùng nhiều/ít", "Cao"],
        ["Error/crash logs", "Theo dõi stability, debugging", "Cao"],
        ["Onboarding completion rate", "Đo conversion funnel", "Cao"],
        ["Wake-word success rate", "Đo chất lượng voice recognition", "Trung bình"],
        ["Tool call success/failure rate", "Đo reliability của AI tools", "Trung bình"],
        ["Response latency (AI)", "Đo trải nghiệm người dùng", "Trung bình"],
        ["Retention metrics (D1/D7/D30)", "Đo sticky factor", "Cao"],
        ["DAU/MAU", "Đo active user base", "Cao"],
        ["Churn indicators", "Dự đoán người dùng rời bỏ", "Trung bình"],
        ["NPS/satisfaction scores", "Đo mức độ hài lòng", "Thấp (MVP)"],
        ["Demographic data", "Phân khúc người dùng", "Thấp"],
    ])

    doc.add_page_break()

    # ========== 6. PHÂN TÍCH TÍNH NĂNG ==========
    add_heading_styled(doc, "6. Phân tích tính năng & mức độ hoàn thiện", 1)

    add_heading_styled(doc, "6.1 Ma trận tính năng", 2)
    add_table(doc, ["Tính năng", "Trạng thái", "Hoàn thiện", "Ghi chú"], [
        ["Google OAuth login", "Hoàn thành", "95%", "Chưa có fallback cho non-Google users"],
        ["Voice Assistant (Gemini Live)", "Hoàn thành", "90%", "Session 15 phút, cần reconnect"],
        ["Wake-word detection", "Hoàn thành", "85%", "Ngưỡng 0.88, false positive cần kiểm tra"],
        ["Text/Voice/Image chat", "Hoàn thành", "90%", "WebSocket real-time, pagination"],
        ["Voice/Video calls", "Hoàn thành", "85%", "LiveKit, VoIP push trên iOS"],
        ["Medication reminders", "Hoàn thành", "90%", "Local + push notifications"],
        ["Health tracking", "Hoàn thành", "80%", "Symptoms, metrics, summary"],
        ["Family dashboard", "Hoàn thành", "85%", "Health, meds, location, audit"],
        ["Family linking", "Hoàn thành", "90%", "Invite code system"],
        ["Emergency SOS", "Hoàn thành", "80%", "60s cooldown, push to all family"],
        ["Location sharing", "Hoàn thành", "75%", "Mapbox static preview"],
        ["Music/Radio", "Hoàn thành", "70%", "yt-dlp search, VOV radio stations"],
        ["Volunteer matching", "Beta", "60%", "Async matching, rating system"],
        ["Navigation", "Cơ bản", "50%", "Chỉ mở Google Maps external"],
        ["Quảng cáo", "Chưa triển khai", "0%", "Chỉ mention trong role selection"],
        ["Subscription/Payment", "Chưa triển khai", "0%", "Không có payment integration"],
        ["Analytics tracking", "Chưa triển khai", "0%", "Không có SDK analytics nào"],
        ["Admin panel", "Chưa triển khai", "0%", "Cần cho quản trị hệ thống"],
        ["i18n (đa ngôn ngữ)", "Chưa triển khai", "0%", "Hardcoded tiếng Việt"],
        ["Offline mode", "Chưa triển khai", "0%", "Cần internet cho mọi tính năng"],
    ])

    add_heading_styled(doc, "6.2 20 AI Tools phân tích", 2)
    add_table(doc, ["#", "Tool", "Nhóm", "Độ phức tạp", "Phụ thuộc"], [
        ["1", "make_voice_call", "Communication", "Cao", "LiveKit, APNs/FCM"],
        ["2", "make_video_call", "Communication", "Cao", "LiveKit, APNs/FCM, Camera"],
        ["3", "send_chat_message", "Communication", "Trung bình", "DB, Push"],
        ["4", "read_unread_messages", "Communication", "Thấp", "DB"],
        ["5", "activate_camera", "Device Control", "Trung bình", "LiveKit RPC"],
        ["6", "deactivate_camera", "Device Control", "Thấp", "LiveKit RPC"],
        ["7", "set_medication_reminder", "Health", "Trung bình", "DB, Redis, Push"],
        ["8", "list_medication_reminders", "Health", "Thấp", "DB"],
        ["9", "mark_medication_taken", "Health", "Thấp", "DB"],
        ["10", "get_health_summary", "Health", "Trung bình", "DB (multiple queries)"],
        ["11", "emergency_alert", "Emergency", "Cao", "Push to all family, 60s cooldown"],
        ["12", "escalate_emergency", "Emergency", "Cao", "Volunteer matching, Redis"],
        ["13", "start_volunteer_match", "Volunteer", "Cao", "Redis queue, async"],
        ["14", "get_match_status", "Volunteer", "Thấp", "Redis poll"],
        ["15", "rate_volunteer", "Volunteer", "Thấp", "DB"],
        ["16", "get_location", "Location", "Thấp", "DB (cached GPS)"],
        ["17", "play_music", "Entertainment", "Trung bình", "yt-dlp, YouTube"],
        ["18", "navigate_to", "Location", "Thấp", "Mapbox URL"],
        ["19", "get_time", "Utility", "Thấp", "System clock"],
        ["20", "get_weather", "Utility", "N/A", "Placeholder (chưa triển khai)"],
    ])

    doc.add_page_break()

    # ========== 7. UX ANALYSIS ==========
    add_heading_styled(doc, "7. Phân tích trải nghiệm người dùng (UX)", 1)

    add_heading_styled(doc, "7.1 Điểm mạnh UX", 2)
    items = [
        "Voice-first design: Người dùng có thể tương tác hoàn toàn bằng giọng nói, "
        "không cần nhìn màn hình",
        "Wake-word 'cần giúp': Kích hoạt tự nhiên, không cần tìm nút bấm",
        "Onboarding đơn giản: Google Sign-In → Chọn vai trò → Cấp quyền → Dùng ngay",
        "Haptic feedback: Rung khi chuyển tab, phản hồi xúc giác cho người khiếm thị",
        "Semantic labels: Screen reader có thể đọc mọi phần tử UI",
        "Font lớn (Plus Jakarta Sans): Dễ đọc cho người suy giảm thị lực",
        "Màu sắc tương phản cao: Navy #1A3263 trên nền sáng #F6F6F6",
    ]
    for item in items:
        doc.add_paragraph(item, style="List Bullet")

    add_heading_styled(doc, "7.2 Điểm yếu UX cần cải thiện", 2)
    add_table(doc, ["Vấn đề", "Mô tả", "Đề xuất"], [
        ["Session timeout 15 phút", "Gemini Live giới hạn 15 phút/session, user phải reconnect",
         "Auto-reconnect seamless, thông báo trước khi hết session"],
        ["Chỉ Google login", "Người dùng không có Google account không thể dùng",
         "Thêm phone number login (phổ biến ở VN)"],
        ["Không có offline mode", "Mất internet = không dùng được gì",
         "Cache reminders, basic TTS offline"],
        ["Video chỉ 2 phút", "Camera session rất ngắn do giới hạn Gemini",
         "Cần thông báo rõ cho user, auto-deactivate gracefully"],
        ["Navigation hạn chế", "Chỉ mở Google Maps external, không có turn-by-turn",
         "Tích hợp deeper navigation hoặc audio directions"],
        ["Không có tutorial/hướng dẫn", "User mới không biết nói gì với AI",
         "Thêm voice onboarding tutorial, gợi ý câu lệnh"],
    ])

    doc.add_page_break()

    # ========== 8. ACCESSIBILITY ==========
    add_heading_styled(doc, "8. Phân tích khả năng tiếp cận (Accessibility)", 1)

    add_heading_styled(doc, "8.1 Đánh giá WCAG 2.1 cho ứng dụng mobile", 2)
    add_table(doc, ["Tiêu chí WCAG", "Trạng thái", "Chi tiết"], [
        ["1.1 Text Alternatives", "Đạt", "Semantic labels cho screen reader"],
        ["1.3 Adaptable", "Đạt", "Voice-first, không phụ thuộc layout phức tạp"],
        ["1.4 Distinguishable", "Đạt", "Contrast ratio cao (navy/white), font lớn"],
        ["2.1 Keyboard Accessible", "N/A", "Mobile app, dùng gestures + voice"],
        ["2.4 Navigable", "Một phần", "3 tabs đơn giản, nhưng thiếu skip navigation"],
        ["3.1 Readable", "Đạt", "Tiếng Việt 100%, ngôn ngữ đơn giản"],
        ["3.2 Predictable", "Đạt", "Navigation nhất quán, ít thay đổi context"],
        ["4.1 Compatible", "Một phần", "Cần kiểm tra VoiceOver/TalkBack thực tế"],
    ])

    add_heading_styled(doc, "8.2 Accessibility features đặc biệt", 2)
    items = [
        "Wake-word detection on-device (Davoice engine): Không cần internet để kích hoạt",
        "Haptic feedback (HapticFeedback.selectionClick): Xác nhận hành động bằng rung",
        "SystemSound feedback: Âm thanh khi chuyển tab",
        "Agent state announcement: Thông báo trạng thái AI (đang nghe, đang nghĩ, đang nói)",
        "Large tap targets: Bottom navigation dạng pill, dễ chạm",
        "Minimal visual dependency: Có thể dùng app mà không cần nhìn màn hình",
    ]
    for item in items:
        doc.add_paragraph(item, style="List Bullet")

    doc.add_page_break()

    # ========== 9. PERFORMANCE ==========
    add_heading_styled(doc, "9. Phân tích hiệu suất & khả năng mở rộng", 1)

    add_heading_styled(doc, "9.1 Hiệu suất hiện tại", 2)
    add_table(doc, ["Metric", "Giá trị ước tính", "Đánh giá"], [
        ["App startup time", "2-4s (Flutter cold start)", "Chấp nhận được"],
        ["Voice latency (end-to-end)", "500ms-2s (Gemini + LiveKit)", "Tốt cho voice AI"],
        ["Chat message delivery", "<500ms (WebSocket)", "Tốt"],
        ["Push notification", "1-3s (APNs/FCM)", "Chuẩn ngành"],
        ["API response time", "50-200ms (Go backend)", "Tốt"],
        ["Wake-word detection", "<100ms (on-device TFLite)", "Rất tốt"],
        ["LiveKit connection", "1-3s initial connect", "Chấp nhận được"],
        ["Battery consumption", "Cao (always-listening mic + LiveKit)", "Cần tối ưu"],
    ])

    add_heading_styled(doc, "9.2 Khả năng mở rộng (Scalability)", 2)
    add_table(doc, ["Users đồng thời", "Infrastructure cần", "Chi phí ước tính/tháng"], [
        ["1-50", "1 VPS 2vCPU/4GB + LiveKit Cloud free tier", "$30-60"],
        ["50-200", "1 VPS 4vCPU/8GB + LiveKit Cloud starter", "$60-130"],
        ["200-1000", "2 VPS + managed PG + LiveKit Growth", "$300-600"],
        ["1000-5000", "Kubernetes cluster + managed services", "$1,000-3,000"],
        ["5000+", "Multi-region, auto-scaling, CDN", "$5,000+"],
    ])

    add_heading_styled(doc, "9.3 Bottlenecks tiềm năng", 2)
    items = [
        "Gemini Live API: 15 phút/session, rate limits chưa rõ cho scale lớn",
        "LiveKit concurrent rooms: Mỗi user = 1 room luôn mở, tốn tài nguyên server",
        "yt-dlp (music): Chạy subprocess trên server, không scale tốt",
        "PostgreSQL single instance: Cần read replicas khi >1000 users",
        "MinIO single node: Cần cluster mode cho high availability",
        "Wake-word + foreground service: Tốn pin đáng kể trên mobile",
    ]
    for item in items:
        doc.add_paragraph(item, style="List Bullet")

    doc.add_page_break()

    # ========== 10. SECURITY ==========
    add_heading_styled(doc, "10. Phân tích bảo mật & quyền riêng tư", 1)

    add_heading_styled(doc, "10.1 Đánh giá bảo mật", 2)
    add_table(doc, ["Khía cạnh", "Triển khai", "Đánh giá"], [
        ["Authentication", "Google OAuth2 → JWT (HS256, 1h access, 30d refresh)", "Tốt"],
        ["Token storage", "flutter_secure_storage (Keychain/Keystore)", "Tốt"],
        ["API transport", "HTTPS/WSS (TLS)", "Tốt"],
        ["Internal API auth", "Shared secret (constant-time comparison)", "Chấp nhận được"],
        ["Input validation", "Cơ bản trong handlers", "Cần cải thiện"],
        ["Rate limiting", "Chỉ có emergency 60s cooldown", "Yếu — cần rate limit toàn API"],
        ["SQL injection", "pgx parameterized queries", "Tốt"],
        ["CORS", "Cấu hình trong middleware", "Cần kiểm tra"],
        ["Data encryption at rest", "Không (PostgreSQL default)", "Cần cải thiện"],
        ["Audit logging", "Có (audit_logs table, append-only)", "Tốt"],
    ])

    add_heading_styled(doc, "10.2 Rủi ro quyền riêng tư", 2)
    add_table(doc, ["Rủi ro", "Mức độ", "Giải pháp đề xuất"], [
        ["Dữ liệu sức khỏe (HIPAA-adjacent)", "Cao",
         "Mã hóa at-rest, access control chặt, data retention policy"],
        ["Vị trí GPS liên tục", "Cao",
         "Chỉ thu thập khi cần, xóa sau 30 ngày, consent rõ ràng"],
        ["Voice recordings trên MinIO", "Cao",
         "Mã hóa, auto-delete sau period, không dùng cho training"],
        ["Audit logs chứa nội dung tương tác", "Trung bình",
         "Anonymize sau 90 ngày, access control"],
        ["Google OAuth dependency", "Trung bình",
         "Dữ liệu Google profile (email, avatar) — cần privacy policy"],
        ["API keys trong config files", "Trung bình",
         "Mapbox, Davoice license trong app_config.env — cần obfuscate"],
    ])

    add_heading_styled(doc, "10.3 Compliance considerations", 2)
    doc.add_paragraph(
        "Với việc thu thập dữ liệu sức khỏe và vị trí của người khuyết tật, "
        "AiSee cần tuân thủ:"
    )
    items = [
        "Luật An ninh mạng Việt Nam 2018: Lưu trữ dữ liệu trong nước, báo cáo khi yêu cầu",
        "Nghị định 13/2023/NĐ-CP (Bảo vệ dữ liệu cá nhân): Consent rõ ràng, quyền xóa dữ liệu",
        "GDPR (nếu mở rộng quốc tế): Right to be forgotten, data portability",
        "Luật Người khuyết tật 2010: Đảm bảo tiếp cận công nghệ cho người khuyết tật",
    ]
    for item in items:
        doc.add_paragraph(item, style="List Bullet")

    doc.add_page_break()

    # ========== 11. MONETIZATION ==========
    add_heading_styled(doc, "11. Phân tích mô hình kinh doanh & monetization", 1)

    add_heading_styled(doc, "11.1 Mô hình hiện tại", 2)
    doc.add_paragraph(
        "Hiện tại AiSee KHÔNG có nguồn thu. Mô hình miễn phí hoàn toàn cho user, "
        "quảng cáo tùy chọn cho family member (chưa triển khai). "
        "Không có payment integration nào trong codebase."
    )

    add_heading_styled(doc, "11.2 Phân tích các mô hình monetization khả thi", 2)
    add_table(doc,
        ["Mô hình", "Mô tả", "Ưu điểm", "Nhược điểm", "Khả thi"],
        [
            ["Freemium", "Free basic + Premium features (unlimited sessions, priority support)",
             "Phổ biến, dễ hiểu", "Khó xác định ranh giới free/premium cho accessibility app", "Cao"],
            ["B2B/Institutional", "Bán cho bệnh viện, trung tâm khiếm thị, tổ chức NGO",
             "Revenue ổn định, volume lớn", "Chu kỳ sales dài, cần team BD", "Cao"],
            ["Government grants", "Tài trợ từ chính phủ, quỹ khuyết tật",
             "Không ảnh hưởng UX", "Không bền vững dài hạn", "Trung bình"],
            ["Subscription gia đình", "$2-5/tháng cho family dashboard premium",
             "Recurring revenue", "Thị trường VN nhạy giá", "Trung bình"],
            ["Ads (family role)", "Quảng cáo cho family members",
             "Không ảnh hưởng user chính", "Revenue thấp ở VN (~$0.5-1 CPM)", "Thấp"],
            ["Data insights (anonymized)", "Bán insights sức khỏe cho pharma/research",
             "High value data", "Ethical concerns, privacy risks", "Thấp"],
        ]
    )

    add_heading_styled(doc, "11.3 Ước tính Revenue tiềm năng", 2)
    add_table(doc, ["Kịch bản", "Users", "ARPU/tháng", "MRR", "ARR"], [
        ["Conservative (Year 1)", "500 users, 200 families", "$0.5 (ads only)", "$100", "$1,200"],
        ["Moderate (Year 2)", "2,000 users, 1,000 families",
         "$2 (freemium + B2B)", "$2,000", "$24,000"],
        ["Optimistic (Year 3)", "10,000 users, 5,000 families",
         "$3 (multi-stream)", "$15,000", "$180,000"],
    ])

    doc.add_page_break()

    # ========== 12. COMPETITIVE ANALYSIS ==========
    add_heading_styled(doc, "12. Phân tích cạnh tranh", 1)

    add_table(doc,
        ["Sản phẩm", "Đối tượng", "Công nghệ", "Giá", "Điểm mạnh", "Điểm yếu so với AiSee"],
        [
            ["Be My Eyes", "Người khiếm thị toàn cầu",
             "Video call + GPT-4 (Be My AI)", "Miễn phí",
             "Cộng đồng tình nguyện viên lớn, AI mạnh",
             "Không có tiếng Việt native, không health tracking"],
            ["Seeing AI (Microsoft)", "Người khiếm thị",
             "Computer vision, OCR, scene description", "Miễn phí (iOS only)",
             "Nhận diện object/text tốt",
             "Chỉ iOS, không có voice assistant, không health"],
            ["Sullivan+", "Người khiếm thị Hàn Quốc",
             "AI scene description", "Miễn phí",
             "Nhẹ, nhanh",
             "Limited features, không có ecosystem gia đình"],
            ["Google Lookout", "Người khiếm thị",
             "ML on-device", "Miễn phí",
             "Google ecosystem, on-device",
             "Chỉ Android, không conversation, không health"],
            ["AiSee", "Người khiếm thị VN",
             "Gemini Live + LiveKit + Flutter", "Miễn phí (MVP)",
             "Tiếng Việt native, voice-first, health+family ecosystem",
             "Mới, chưa có user base, phụ thuộc Google"],
        ]
    )

    add_heading_styled(doc, "12.1 Lợi thế cạnh tranh của AiSee", 2)
    items = [
        "Duy nhất tại Việt Nam: Không có đối thủ trực tiếp với giải pháp tương tự cho người khiếm thị VN",
        "Voice-first tiếng Việt: Tối ưu cho ngữ cảnh Việt Nam, không cần dịch",
        "Ecosystem toàn diện: AI assistant + Health tracking + Family dashboard + Emergency = all-in-one",
        "Real-time voice (không phải chat): LiveKit WebRTC cho trải nghiệm conversation thực sự",
        "Wake-word on-device: Không cần internet để kích hoạt, tiết kiệm pin hơn always-on mic",
    ]
    for item in items:
        doc.add_paragraph(item, style="List Bullet")

    doc.add_page_break()

    # ========== 13. CHI PHÍ VẬN HÀNH ==========
    add_heading_styled(doc, "13. Phân tích chi phí vận hành", 1)

    add_heading_styled(doc, "13.1 Chi phí MVP (0-200 users)", 2)
    add_table(doc, ["Hạng mục", "Service", "Chi phí/tháng (USD)", "Ghi chú"], [
        ["Server (VPS)", "Hetzner/DigitalOcean 4vCPU/8GB", "$24-48", "Backend + Agent + DB"],
        ["LiveKit", "LiveKit Cloud (free tier → starter)", "$0-50", "Free: 5000 mins/month"],
        ["Gemini API", "Google AI (free tier → pay-as-you-go)", "$0-30", "Free: 15 RPM, limited"],
        ["Domain + SSL", "Cloudflare", "$0-10", "Free tier đủ"],
        ["Push notifications", "APNs + FCM", "$0", "Miễn phí"],
        ["MinIO storage", "Self-hosted trên VPS", "$0", "Included trong VPS"],
        ["Monitoring", "UptimeRobot + basic logging", "$0-7", "Free tier"],
        ["", "TỔNG CỘNG", "$24-145", ""],
    ])

    add_heading_styled(doc, "13.2 Chi phí Scale (1000+ users)", 2)
    add_table(doc, ["Hạng mục", "Chi phí/tháng (USD)", "Ghi chú"], [
        ["Compute (2-3 VPS)", "$100-200", "Separate DB, backend, agent"],
        ["Managed PostgreSQL", "$30-80", "Supabase/Neon/DO Managed"],
        ["LiveKit Growth plan", "$100-300", "Based on concurrent rooms"],
        ["Gemini API", "$100-500", "~$0.075/min audio, scales with usage"],
        ["CDN + Storage", "$20-50", "MinIO cluster or S3"],
        ["Monitoring + Logging", "$20-50", "Datadog/Grafana Cloud"],
        ["TỔNG CỘNG", "$370-1,180", ""],
    ])

    doc.add_page_break()

    # ========== 14. KPIs ==========
    add_heading_styled(doc, "14. KPIs & metrics đề xuất", 1)

    add_heading_styled(doc, "14.1 North Star Metric", 2)
    doc.add_paragraph(
        "Đề xuất North Star Metric: \"Số phút tương tác AI thành công mỗi tuần\" "
        "— phản ánh giá trị cốt lõi (AI hỗ trợ người khiếm thị thực sự hữu ích)."
    )

    add_heading_styled(doc, "14.2 KPIs theo nhóm", 2)
    add_table(doc, ["Nhóm", "KPI", "Mục tiêu (6 tháng)", "Cách đo"], [
        ["Acquisition", "Số user đăng ký mới/tuần", "50 users/tuần", "DB: users.created_at"],
        ["Acquisition", "Onboarding completion rate", ">80%", "Event tracking (cần thêm)"],
        ["Activation", "% user dùng AI assistant trong 24h đầu", ">60%",
         "audit_logs first tool call"],
        ["Activation", "% user thêm ít nhất 1 family member", ">40%", "DB: family_links"],
        ["Engagement", "DAU/MAU ratio", ">30%", "Session tracking (cần thêm)"],
        ["Engagement", "Số tool calls/user/ngày", ">5", "DB: audit_logs"],
        ["Engagement", "Thời gian voice session trung bình", ">3 phút", "LiveKit analytics"],
        ["Retention", "D1 retention", ">50%", "Session tracking (cần thêm)"],
        ["Retention", "D7 retention", ">30%", "Session tracking (cần thêm)"],
        ["Retention", "D30 retention", ">15%", "Session tracking (cần thêm)"],
        ["Health", "Medication adherence rate", ">70%", "DB: medication_logs vs medications"],
        ["Health", "% reminders responded to", ">60%", "medication_logs / scheduled reminders"],
        ["Quality", "AI tool call success rate", ">95%", "audit_logs success/fail"],
        ["Quality", "Crash-free sessions", ">99%", "Crashlytics (cần thêm)"],
        ["Quality", "Voice latency P95", "<2s", "Custom metric (cần thêm)"],
        ["Revenue", "MRR", "Phụ thuộc model", "Payment system (cần thêm)"],
    ])

    doc.add_page_break()

    # ========== 15. RISK ANALYSIS ==========
    add_heading_styled(doc, "15. Phân tích rủi ro", 1)

    add_table(doc,
        ["Rủi ro", "Xác suất", "Tác động", "Mức độ", "Giải pháp giảm thiểu"],
        [
            ["Google Gemini API thay đổi giá/điều khoản", "Trung bình", "Cao", "Cao",
             "Thiết kế abstract AI layer, sẵn sàng switch sang OpenAI/Claude"],
            ["LiveKit Cloud tăng giá hoặc ngừng free tier", "Thấp", "Cao", "Trung bình",
             "Self-host LiveKit server, đã có Docker setup"],
            ["Không đạt product-market fit", "Trung bình", "Cao", "Cao",
             "User research sớm, pivot nhanh dựa trên data"],
            ["Dữ liệu sức khỏe bị leak", "Thấp", "Rất cao", "Cao",
             "Encryption at-rest, access audit, penetration testing"],
            ["Không có revenue sau 12 tháng", "Trung bình", "Cao", "Cao",
             "Đa dạng hóa revenue: B2B + grants + freemium"],
            ["Gemini Live session limits (15 min)", "Cao", "Trung bình", "Trung bình",
             "Session resumption đã implement, cần seamless UX"],
            ["Battery drain từ always-listening", "Cao", "Trung bình", "Trung bình",
             "Tối ưu wake-word engine, smart listening schedules"],
            ["Quy định dữ liệu cá nhân thắt chặt", "Trung bình", "Trung bình", "Trung bình",
             "Privacy-by-design, data minimization, retention policies"],
            ["Team size nhỏ, bus factor cao", "Cao", "Trung bình", "Trung bình",
             "Documentation tốt (đã có), CI/CD automation"],
            ["False positive wake-word", "Trung bình", "Thấp", "Thấp",
             "Tune threshold (hiện 0.88), user feedback loop"],
        ]
    )

    doc.add_page_break()

    # ========== 16. ĐỀ XUẤT ==========
    add_heading_styled(doc, "16. Đề xuất & lộ trình phát triển", 1)

    add_heading_styled(doc, "16.1 Ưu tiên ngay (Sprint 1-2, tuần 1-4)", 2)
    add_table(doc, ["#", "Đề xuất", "Lý do", "Effort"], [
        ["1", "Tích hợp analytics SDK (Firebase Analytics hoặc PostHog)",
         "Không thể ra quyết định data-driven nếu không có data", "Medium"],
        ["2", "Thêm event tracking cho tất cả user actions",
         "Cần biết tính năng nào được dùng, funnel conversion", "Medium"],
        ["3", "Implement rate limiting toàn API",
         "Bảo mật cơ bản, chống abuse", "Small"],
        ["4", "Thêm crash reporting (Firebase Crashlytics)",
         "Biết app crash ở đâu, bao nhiêu % users affected", "Small"],
        ["5", "Privacy policy + Terms of Service",
         "Bắt buộc cho App Store/Play Store và compliance VN", "Small"],
    ])

    add_heading_styled(doc, "16.2 Ngắn hạn (Sprint 3-6, tháng 2-3)", 2)
    add_table(doc, ["#", "Đề xuất", "Lý do", "Effort"], [
        ["1", "Phone number login (OTP)",
         "Phổ biến hơn Google ở VN, đặc biệt cho người lớn tuổi", "Large"],
        ["2", "Offline medication reminders",
         "Critical feature không nên phụ thuộc internet", "Medium"],
        ["3", "Voice onboarding tutorial",
         "Hướng dẫn user mới biết nói gì với AI", "Medium"],
        ["4", "Admin panel cơ bản",
         "Quản lý users, xem analytics, moderate content", "Large"],
        ["5", "Data encryption at rest",
         "Bảo vệ dữ liệu sức khỏe nhạy cảm", "Medium"],
        ["6", "Retention analysis dashboard",
         "Theo dõi cohort retention, churn prediction", "Medium"],
    ])

    add_heading_styled(doc, "16.3 Trung hạn (tháng 4-6)", 2)
    add_table(doc, ["#", "Đề xuất", "Lý do", "Effort"], [
        ["1", "Payment integration (MoMo/ZaloPay)",
         "Monetization cho thị trường VN", "Large"],
        ["2", "B2B partnership program",
         "Revenue từ bệnh viện, trung tâm khiếm thị", "Large"],
        ["3", "Multi-language support (English)",
         "Mở rộng ra ASEAN, đối tác quốc tế", "Large"],
        ["4", "AI model fallback (Claude/GPT)",
         "Giảm phụ thuộc vào Google Gemini", "Large"],
        ["5", "Kubernetes migration",
         "Chuẩn bị cho scale 1000+ users", "Large"],
    ])

    doc.add_page_break()

    # ========== 17. KẾT LUẬN ==========
    add_heading_styled(doc, "17. Kết luận", 1)

    doc.add_paragraph(
        "AiSee là một sản phẩm MVP ấn tượng với kiến trúc chắc chắn, tập trung đúng "
        "vào đối tượng mục tiêu (người khiếm thị tại Việt Nam) và có tiềm năng thị trường "
        "đáng kể (~2 triệu người). Sản phẩm nổi bật nhờ:"
    )

    items = [
        "Kiến trúc voice-first thực sự, không chỉ thêm voice vào app thông thường",
        "Ecosystem toàn diện: AI + Health + Family + Emergency trong một ứng dụng",
        "Công nghệ tiên tiến: Gemini Live real-time voice, LiveKit WebRTC, on-device wake-word",
        "Documentation xuất sắc (12 phần, rất chi tiết) — hiếm thấy ở dự án student/startup",
    ]
    for item in items:
        doc.add_paragraph(item, style="List Bullet")

    doc.add_paragraph("")
    doc.add_paragraph(
        "Tuy nhiên, để chuyển từ MVP sang sản phẩm bền vững, AiSee cần giải quyết ngay "
        "3 vấn đề cốt lõi:"
    )

    items = [
        "Analytics gap: Không có data → không thể ra quyết định → không thể improve. "
        "Đây là ưu tiên #1.",
        "Monetization: Cần xác định rõ mô hình kinh doanh (đề xuất: B2B + Freemium family) "
        "trước khi hết runway.",
        "Privacy & Compliance: Dữ liệu sức khỏe + vị trí + khuyết tật = sensitivity rất cao. "
        "Cần privacy policy, encryption, và data retention policy ngay.",
    ]
    for i, item in enumerate(items, 1):
        doc.add_paragraph(f"{i}. {item}")

    doc.add_paragraph("")
    doc.add_paragraph(
        "Với nền tảng kỹ thuật vững chắc và thị trường chưa có đối thủ tại Việt Nam, "
        "AiSee có cơ hội lớn để trở thành ứng dụng hỗ trợ người khiếm thị hàng đầu "
        "khu vực nếu thực hiện đúng lộ trình data-driven và monetization trong 6 tháng tới."
    )

    # ========== FOOTER ==========
    doc.add_paragraph("")
    doc.add_paragraph("— Hết báo cáo —").alignment = WD_ALIGN_PARAGRAPH.CENTER

    # Save
    output_path = "/home/eddiesngu/Desktop/Dang/aiseebackend/docs/2026-04-06-aisee-data-analysis-report.docx"
    doc.save(output_path)
    return output_path


if __name__ == "__main__":
    path = build_report()
    print(f"Report saved to: {path}")
